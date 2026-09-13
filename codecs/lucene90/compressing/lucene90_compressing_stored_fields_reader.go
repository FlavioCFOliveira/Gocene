// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

package compressing

import (
	"errors"
	"fmt"
	"math"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene90CompressingStoredFieldsReader is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsReader.
type Lucene90CompressingStoredFieldsReader struct {
	chunkSize       int
	compressionMode compressing.CompressionMode
	decompressor    compressing.Decompressor
	numDocs         int
	numChunks       int64
	numDirtyChunks  int64
	numDirtyDocs    int64
	fieldInfos      *index.FieldInfos // used to map field numbers to names

	fieldsStream store.IndexInput
	indexReader  *fieldsIndexReader
	maxPointer   int64

	closed bool
}

func newLucene90CompressingStoredFieldsReader(
	dir store.Directory,
	si *index.SegmentInfo,
	fn *index.FieldInfos,
	ctx store.IOContext,
	formatName string,
	compressionMode compressing.CompressionMode,
) (*Lucene90CompressingStoredFieldsReader, error) {
	segment := si.Name()
	suffix := ""

	// Open .fdt.
	fdtName := store.SegmentFileName(segment, suffix, fieldsExtension)
	fieldsStream, err := dir.OpenInput(fdtName, ctx)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: open %s: %w", fdtName, err)
	}

	success := false
	defer func() {
		if !success {
			_ = fieldsStream.Close()
		}
	}()

	version, err := gcodecs.CheckIndexHeader(
		fieldsStream, formatName, versionStart, versionCurrent, si.GetID(), suffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: check fdt header: %w", err)
	}

	// Retrieve footer checksum without reading the whole file.
	if _, err := gcodecs.RetrieveChecksum(fieldsStream); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: retrieve fdt checksum: %w", err)
	}

	// Open .fdm.
	fdmName := store.SegmentFileName(segment, suffix, metaExtension)
	metaRaw, err := dir.OpenInput(fdmName, ctx)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: open %s: %w", fdmName, err)
	}
	metaIn := store.NewChecksumIndexInput(metaRaw)

	defer func() {
		if !success {
			_ = metaIn.Close()
		}
	}()

	_, err = gcodecs.CheckIndexHeader(
		metaIn, indexCodecName+"Meta", metaVersionStart, version, si.GetID(), suffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: check fdm header: %w", err)
	}

	chunkSize, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read chunkSize: %w", err)
	}

	// Read fields index from .fdm + .fdx.
	indexReader, err := newFieldsIndexReader(
		dir, si.Name(), suffix, indexExtension, indexCodecName, si.GetID(), metaIn, ctx,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read fields index: %w", err)
	}

	numChunks, err := metaIn.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numChunks: %w", err)
	}
	numDirtyChunks, err := metaIn.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numDirtyChunks: %w", err)
	}
	numDirtyDocs, err := metaIn.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numDirtyDocs: %w", err)
	}

	if _, err := store.CheckFooter(metaIn); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: check fdm footer: %w", err)
	}
	_ = metaIn.Close()

	success = true
	return &Lucene90CompressingStoredFieldsReader{
		chunkSize:       int(chunkSize),
		compressionMode: compressionMode,
		decompressor:    compressionMode.NewDecompressor(),
		numDocs:         si.DocCount(),
		numChunks:       numChunks,
		numDirtyChunks:  numDirtyChunks,
		numDirtyDocs:    numDirtyDocs,
		fieldInfos:      fn,
		fieldsStream:    fieldsStream,
		indexReader:     indexReader,
		maxPointer:      indexReader.getMaxPointer(),
	}, nil
}

// VisitDocument decodes the stored fields for docID and dispatches each
// field to visitor.
func (r *Lucene90CompressingStoredFieldsReader) VisitDocument(docID int, visitor gcodecs.StoredFieldVisitor) error {
	if r.closed {
		return errors.New("lucene90/compressing: reader is closed")
	}

	// Find the chunk that contains docID.
	startPointer, err := fieldsIndexStartPointer(r.indexReader, docID)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: lookup docID %d: %w", docID, err)
	}

	if err := r.fieldsStream.SetPosition(startPointer); err != nil {
		return fmt.Errorf("lucene90/compressing: seek to chunk: %w", err)
	}

	// Read chunk header.
	docBase, err := store.ReadVInt(r.fieldsStream)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: read docBase: %w", err)
	}
	token, err := store.ReadVInt(r.fieldsStream)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: read token: %w", err)
	}
	chunkDocs := int(token >> 2)
	sliced := (token & 1) != 0

	if docID < int(docBase) || docID >= int(docBase)+chunkDocs || int(docBase)+chunkDocs > r.numDocs {
		return fmt.Errorf(
			"lucene90/compressing: corrupted chunk: docID=%d docBase=%d chunkDocs=%d numDocs=%d",
			docID, docBase, chunkDocs, r.numDocs,
		)
	}

	// Read numStoredFields per doc and lengths (raw, not cumulative yet).
	// Mirrors the Java saveInts/readInts contract: chunkDocs==1 → bare VInt;
	// chunkDocs>1 → StoredFieldsInts wire format with a leading header byte.
	numStoredFieldsArr := make([]int64, chunkDocs)
	if err := readInts(r.fieldsStream, chunkDocs, numStoredFieldsArr, 0); err != nil {
		return fmt.Errorf("lucene90/compressing: read numStoredFields: %w", err)
	}
	offsets := make([]int64, chunkDocs+1)
	if err := readInts(r.fieldsStream, chunkDocs, offsets, 1); err != nil {
		return fmt.Errorf("lucene90/compressing: read lengths: %w", err)
	}
	// Convert lengths to cumulative offsets.
	for i := 0; i < chunkDocs; i++ {
		offsets[i+1] += offsets[i]
	}

	// Decompress to get the raw bytes for the whole chunk.
	totalLength := int(offsets[chunkDocs])
	idx := docID - int(docBase)
	docOffset := int(offsets[idx])
	docLength := int(offsets[idx+1]) - docOffset
	numFields := int(numStoredFieldsArr[idx])

	if docLength == 0 {
		// Empty document.
		return nil
	}

	var dst util.BytesRef
	startPointerData := r.fieldsStream.GetFilePointer()

	if sliced {
		// Decompress slice-by-slice; we need to reconstruct the full chunk.
		dst.Bytes = make([]byte, totalLength)
		decompressed := 0
		var spare util.BytesRef
		for decompressed < totalLength {
			toDecompress := r.chunkSize
			if totalLength-decompressed < toDecompress {
				toDecompress = totalLength - decompressed
			}
			if err := r.decompressor.Decompress(r.fieldsStream, toDecompress, 0, toDecompress, &spare); err != nil {
				return fmt.Errorf("lucene90/compressing: decompress slice: %w", err)
			}
			copy(dst.Bytes[decompressed:], spare.Bytes[spare.Offset:spare.Offset+spare.Length])
			decompressed += spare.Length
		}
		dst.Offset = docOffset
		dst.Length = docLength
	} else {
		// Single decompression, requesting only the window we need.
		_ = startPointerData
		if err := r.decompressor.Decompress(r.fieldsStream, totalLength, docOffset, docLength, &dst); err != nil {
			return fmt.Errorf("lucene90/compressing: decompress: %w", err)
		}
	}

	// Parse fields from dst.Bytes[dst.Offset : dst.Offset+dst.Length].
	docData := store.NewByteArrayDataInput(dst.Bytes[dst.Offset : dst.Offset+dst.Length])
	for fieldIDX := 0; fieldIDX < numFields; fieldIDX++ {
		infoAndBits, err := docData.ReadVLong()
		if err != nil {
			return fmt.Errorf("lucene90/compressing: read infoAndBits: %w", err)
		}
		// The upper (infoAndBits >> typeBits) bits encode the field number
		// (FieldInfo.number in Lucene; sequential 0-based ID in the Gocene
		// writer). Map it back to a field name via FieldInfos when available.
		fieldNumber := int(infoAndBits >> typeBits)
		fieldName := ""
		if r.fieldInfos != nil {
			if fi := r.fieldInfos.GetByNumber(fieldNumber); fi != nil {
				fieldName = fi.Name()
			}
		}
		bits := int(infoAndBits & typeMask)
		switch bits {
		case int(typeString):
			s, err := store.ReadString(docData)
			if err != nil {
				return err
			}
			visitor.StringField(fieldName, s)
		case int(typeByteArray):
			length, err := store.ReadVInt(docData)
			if err != nil {
				return err
			}
			b := make([]byte, length)
			if err := docData.ReadBytes(b, 0, len(b)); err != nil {
				return err
			}
			visitor.BinaryField(fieldName, b)
		case int(typeNumericInt):
			v, err := readZInt(docData)
			if err != nil {
				return err
			}
			visitor.IntField(fieldName, int(v))
		case int(typeNumericFloat):
			v, err := readZFloat(docData)
			if err != nil {
				return err
			}
			visitor.FloatField(fieldName, v)
		case int(typeNumericLong):
			v, err := readTLong(docData)
			if err != nil {
				return err
			}
			visitor.LongField(fieldName, v)
		case int(typeNumericDouble):
			v, err := readZDouble(docData)
			if err != nil {
				return err
			}
			visitor.DoubleField(fieldName, v)
		default:
			return fmt.Errorf("lucene90/compressing: unknown field type %d", bits)
		}
	}
	return nil
}

// Close releases the underlying IndexInput.
func (r *Lucene90CompressingStoredFieldsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var errs []error
	if err := r.fieldsStream.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := r.indexReader.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// CheckIntegrity walks the stored-field data and validates the checksum
// framing.
//
// Go port of Lucene90CompressingStoredFieldsReader.checkIntegrity
// (Lucene 10.5.0, Lucene90CompressingStoredFieldsReader.java):
//
//	public void checkIntegrity() throws IOException {
//	  indexReader.checkIntegrity();
//	  CodecUtil.checksumEntireFile(fieldsStream);
//	}
func (r *Lucene90CompressingStoredFieldsReader) CheckIntegrity() error {
	if err := r.indexReader.checkIntegrity(); err != nil {
		return err
	}
	_, err := store.ChecksumEntireFile(r.fieldsStream)
	return err
}

// Compile-time guarantee.
var _ gcodecs.StoredFieldsReader = (*Lucene90CompressingStoredFieldsReader)(nil)

// readInts is the read-side counterpart of saveInts. It mirrors the Java
// readInts(DataInput, int, long[], int) helper in
// Lucene90CompressingStoredFieldsReader: for count==1 it reads a bare VInt;
// for count>1 it delegates to StoredFieldsInts.readInts.
func readInts(in store.DataInput, count int, values []int64, offset int) error {
	if count == 1 {
		v, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		values[offset] = int64(v)
		return nil
	}
	return storedFieldsIntsReadInts(in, count, values, offset)
}

// readZInt reads a zigzag+VInt encoded int32.
func readZInt(in store.DataInput) (int32, error) {
	v, err := store.ReadVInt(in)
	if err != nil {
		return 0, err
	}
	return int32(uint32(v)>>1) ^ -(v & 1), nil
}

// readZFloat mirrors Lucene's Lucene90CompressingStoredFieldsReader.readZFloat
// (Lucene90CompressingStoredFieldsReader.java:317-330):
//
//	int b = in.readByte() & 0xFF;
//	if (b == 0xFF) {
//	  return Float.intBitsToFloat(in.readInt());
//	} else if ((b & 0x80) != 0) {
//	  return (b & 0x7f) - 1;
//	} else {
//	  int bits = b << 24 | ((in.readShort() & 0xFFFF) << 8) | (in.readByte() & 0xFF);
//	  return Float.intBitsToFloat(bits);
//	}
//
// DataInput.readShort and DataInput.readInt are LITTLE-endian in Lucene 10.5.0,
// and so are store.DataInput's; this is the exact mirror of writeZFloat.
func readZFloat(in store.DataInput) (float32, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	if b == 0xFF {
		// negative value
		v, err := in.ReadInt()
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(uint32(v)), nil
	} else if (b & 0x80) != 0 {
		// small integer [-1..125]
		return float32(int32(b&0x7F) - 1), nil
	}
	// positive float
	sh, err := in.ReadShort()
	if err != nil {
		return 0, err
	}
	lo, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	bits := uint32(b)<<24 | (uint32(uint16(sh)) << 8) | uint32(lo)
	return math.Float32frombits(bits), nil
}

// readZDouble mirrors Lucene's Lucene90CompressingStoredFieldsReader.readZDouble
// (Lucene90CompressingStoredFieldsReader.java:336-356):
//
//	int b = in.readByte() & 0xFF;
//	if (b == 0xFF) {
//	  return Double.longBitsToDouble(in.readLong());
//	} else if (b == 0xFE) {
//	  return Float.intBitsToFloat(in.readInt());
//	} else if ((b & 0x80) != 0) {
//	  return (b & 0x7f) - 1;
//	} else {
//	  long bits = ((long) b) << 56
//	      | ((in.readInt() & 0xFFFFFFFFL) << 24)
//	      | ((in.readShort() & 0xFFFFL) << 8)
//	      | (in.readByte() & 0xFFL);
//	  return Double.longBitsToDouble(bits);
//	}
func readZDouble(in store.DataInput) (float64, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	if b == 0xFF {
		// negative value
		v, err := in.ReadLong()
		if err != nil {
			return 0, err
		}
		return math.Float64frombits(uint64(v)), nil
	} else if b == 0xFE {
		// float
		v, err := in.ReadInt()
		if err != nil {
			return 0, err
		}
		return float64(math.Float32frombits(uint32(v))), nil
	} else if (b & 0x80) != 0 {
		// small integer [-1..124]
		return float64(int64(b&0x7F) - 1), nil
	}
	// positive double
	mid, err := in.ReadInt()
	if err != nil {
		return 0, err
	}
	sh, err := in.ReadShort()
	if err != nil {
		return 0, err
	}
	lo, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	bits := uint64(b)<<56 |
		(uint64(uint32(mid)) << 24) |
		(uint64(uint16(sh)) << 8) |
		uint64(lo)
	return math.Float64frombits(bits), nil
}

// readTLong mirrors Lucene's Lucene90CompressingStoredFieldsReader.readTLong.
func readTLong(in store.DataInput) (int64, error) {
	headerByte, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	header := int(headerByte)
	bits := uint64(header & 0x1F)
	if (header & 0x20) != 0 {
		upper, err := in.ReadVLong()
		if err != nil {
			return 0, err
		}
		bits |= uint64(upper) << 5
	}
	l := int64(bits>>1) ^ -(int64(bits) & 1)
	switch header & tlongDayEncoding {
	case tlongSecondEncoding:
		l *= tlongSecond
	case tlongHourEncoding:
		l *= tlongHour
	case tlongDayEncoding:
		l *= tlongDay
	}
	return l, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
