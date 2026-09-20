// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene50/compressing/Lucene50CompressingStoredFieldsReader.java

package compressing

import (
	"errors"
	"fmt"
	"io"
	"math"

	bccompressing "github.com/FlavioCFOliveira/Gocene/backward_codecs/compressing"
	bstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	// FieldsExtension is the extension of the stored fields file.
	//
	// Mirrors {@code public static final String FIELDS_EXTENSION = "fdt"}.
	FieldsExtension = "fdt"
	// IndexExtension is the extension of the stored fields index.
	//
	// Mirrors {@code public static final String INDEX_EXTENSION = "fdx"}.
	IndexExtension = "fdx"
	// MetaExtension is the extension of the stored fields meta.
	//
	// Mirrors {@code public static final String META_EXTENSION = "fdm"}.
	MetaExtension = "fdm"
	// IndexCodecName is the codec name for the index.
	//
	// Mirrors {@code public static final String INDEX_CODEC_NAME = "Lucene85FieldsIndex"}.
	IndexCodecName = "Lucene85FieldsIndex"
)

// The stored-field type flags, mirroring the package-private constants of
// Lucene50CompressingStoredFieldsReader.
const (
	storedTypeString        = 0x00
	storedTypeByteArr       = 0x01
	storedTypeNumericInt    = 0x02
	storedTypeNumericFloat  = 0x03
	storedTypeNumericLong   = 0x04
	storedTypeNumericDouble = 0x05
)

// storedTypeBits and storedTypeMask mirror
//
//	static final int TYPE_BITS = PackedInts.bitsRequired(NUMERIC_DOUBLE);
//	static final int TYPE_MASK = (int) PackedInts.maxValue(TYPE_BITS);
//
// PackedInts.bitsRequired(5) is 3 and PackedInts.maxValue(3) is 7.
const (
	storedTypeBits = 3
	storedTypeMask = 7
)

// The format versions, mirroring the package-private constants.
const (
	storedFieldsVersionStart = 1
	// storedFieldsVersionOffHeapIndex is the version from which the fields
	// index moved off-heap.
	storedFieldsVersionOffHeapIndex = 2
	// storedFieldsVersionMeta is the version where all metadata were moved to
	// the meta file.
	storedFieldsVersionMeta = 3
	// storedFieldsVersionNumChunks is the version where numChunks is
	// explicitly recorded in the meta file and a dirty chunk bit is recorded
	// in each chunk.
	storedFieldsVersionNumChunks = 4
	storedFieldsVersionCurrent   = storedFieldsVersionNumChunks
	storedFieldsMetaVersionStart = 0
)

// The timestamp-compression constants, mirroring the package-private fields.
const (
	storedSecond         = int64(1000)
	storedHour           = 60 * 60 * storedSecond
	storedDay            = 24 * storedHour
	storedSecondEncoding = 0x40
	storedHourEncoding   = 0x80
	storedDayEncoding    = 0xC0
)

// ErrAlreadyClosed renders org.apache.lucene.store.AlreadyClosedException,
// which ensureOpen throws.
var ErrAlreadyClosed = errors.New("this FieldsReader is closed")

// Lucene50CompressingStoredFieldsReader is the StoredFieldsReader
// implementation for Lucene50CompressingStoredFieldsFormat.
//
// Mirrors {@code public final class Lucene50CompressingStoredFieldsReader
// extends StoredFieldsReader} (Lucene 10.5.0, @lucene.experimental).
type Lucene50CompressingStoredFieldsReader struct {
	version           int
	fieldInfos        *index.FieldInfos
	indexReader       FieldsIndex
	maxPointer        int64
	fieldsStream      store.IndexInput
	chunkSize         int
	packedIntsVersion int
	compressionMode   bccompressing.CompressionMode
	decompressor      bccompressing.Decompressor
	numDocs           int
	merging           bool
	state             *storedFieldsBlockState
	closed            bool
}

// cloneReader reproduces the private copy constructor
// {@code Lucene50CompressingStoredFieldsReader(Lucene50CompressingStoredFieldsReader reader,
// boolean merging)}.
func cloneReader(reader *Lucene50CompressingStoredFieldsReader, merging bool) *Lucene50CompressingStoredFieldsReader {
	r := &Lucene50CompressingStoredFieldsReader{
		version:           reader.version,
		fieldInfos:        reader.fieldInfos,
		fieldsStream:      reader.fieldsStream.Clone(),
		indexReader:       reader.indexReader.Clone(),
		maxPointer:        reader.maxPointer,
		chunkSize:         reader.chunkSize,
		packedIntsVersion: reader.packedIntsVersion,
		compressionMode:   reader.compressionMode,
		decompressor:      reader.decompressor.Clone(),
		numDocs:           reader.numDocs,
		merging:           merging,
		closed:            false,
	}
	r.state = newStoredFieldsBlockState(r)
	return r
}

// NewLucene50CompressingStoredFieldsReader is the sole public constructor.
//
// Mirrors {@code public Lucene50CompressingStoredFieldsReader(Directory d,
// SegmentInfo si, String segmentSuffix, FieldInfos fn, IOContext context,
// String formatName, CompressionMode compressionMode)}.
func NewLucene50CompressingStoredFieldsReader(
	d store.Directory,
	si *spi.SegmentInfo,
	segmentSuffix string,
	fn *index.FieldInfos,
	context store.IOContext,
	formatName string,
	compressionMode bccompressing.CompressionMode,
) (*Lucene50CompressingStoredFieldsReader, error) {
	r := &Lucene50CompressingStoredFieldsReader{
		compressionMode: compressionMode,
		fieldInfos:      fn,
		numDocs:         si.DocCount(),
		merging:         false,
	}
	segment := si.Name()

	fieldsStreamFN := store.SegmentFileName(segment, segmentSuffix, FieldsExtension)
	// Open the data file
	fieldsStream, err := bstore.OpenInput(d, fieldsStreamFN, context)
	if err != nil {
		return nil, err
	}
	r.fieldsStream = fieldsStream
	success := false
	defer func() {
		if !success {
			_ = r.Close()
		}
	}()

	version, err := codecs.CheckIndexHeader(
		fieldsStream, formatName, storedFieldsVersionStart, storedFieldsVersionCurrent,
		si.GetID(), segmentSuffix)
	if err != nil {
		return nil, err
	}
	r.version = int(version)

	var metaIn *bstore.EndiannessReverserChecksumIndexInput
	if r.version >= storedFieldsVersionOffHeapIndex {
		metaStreamFN := store.SegmentFileName(segment, segmentSuffix, MetaExtension)
		metaIn, err = bstore.OpenChecksumInput(d, metaStreamFN, spi.IOContextReadOnce)
		if err != nil {
			return nil, err
		}
		if _, err := codecs.CheckIndexHeader(
			metaIn, IndexCodecName+"Meta", storedFieldsMetaVersionStart, int32(r.version),
			si.GetID(), segmentSuffix,
		); err != nil {
			_ = metaIn.Close()
			return nil, err
		}
	}
	if r.version >= storedFieldsVersionMeta {
		chunkSize, err := metaIn.ReadVInt()
		if err != nil {
			_ = metaIn.Close()
			return nil, err
		}
		r.chunkSize = int(chunkSize)
		packedIntsVersion, err := metaIn.ReadVInt()
		if err != nil {
			_ = metaIn.Close()
			return nil, err
		}
		r.packedIntsVersion = int(packedIntsVersion)
	} else {
		chunkSize, err := fieldsStream.ReadVInt()
		if err != nil {
			return nil, err
		}
		r.chunkSize = int(chunkSize)
		packedIntsVersion, err := fieldsStream.ReadVInt()
		if err != nil {
			return nil, err
		}
		r.packedIntsVersion = int(packedIntsVersion)
	}

	r.decompressor = compressionMode.NewDecompressor()
	r.state = newStoredFieldsBlockState(r)

	// NOTE: data file is too costly to verify checksum against all the bytes
	// on open, but for now we at least verify proper structure of the checksum
	// footer: which looks for FOOTER_MAGIC + algorithmID. This is cheap and
	// can detect some forms of corruption such as file truncation.
	if _, err := codecs.RetrieveChecksum(fieldsStream); err != nil {
		return nil, err
	}

	maxPointer := int64(-1)
	var indexReader FieldsIndex

	if r.version < storedFieldsVersionOffHeapIndex {
		// Load the index into memory
		indexName := store.SegmentFileName(segment, segmentSuffix, IndexExtension)
		indexStream, err := bstore.OpenChecksumInput(d, indexName, context)
		if err != nil {
			return nil, err
		}
		// assert formatName.endsWith("Data");
		codecNameIdx := formatName[:len(formatName)-len("Data")] + "Index"
		version2, priorE := codecs.CheckIndexHeader(
			indexStream, codecNameIdx, storedFieldsVersionStart, storedFieldsVersionCurrent,
			si.GetID(), segmentSuffix)
		if priorE == nil && int(version2) != r.version {
			priorE = fmt.Errorf("Version mismatch between stored fields index and data: %d != %d",
				version2, r.version)
		}
		if priorE == nil {
			legacy, err := NewLegacyFieldsIndexReader(indexStream, si)
			if err != nil {
				priorE = err
			} else {
				indexReader = legacy
				maxPointer, priorE = indexStream.ReadVLong()
			}
		}
		if footerErr := checkStoredFieldsFooter(indexStream); priorE == nil && footerErr != nil {
			priorE = footerErr
		}
		_ = indexStream.Close()
		if priorE != nil {
			if metaIn != nil {
				_ = metaIn.Close()
			}
			return nil, priorE
		}
	} else {
		fieldsIndex, err := newFieldsIndexReader(
			d, si.Name(), segmentSuffix, IndexExtension, IndexCodecName, si.GetID(), metaIn)
		if err != nil {
			_ = metaIn.Close()
			return nil, err
		}
		indexReader = fieldsIndex
		maxPointer = fieldsIndex.GetMaxPointer()
	}

	r.maxPointer = maxPointer
	r.indexReader = indexReader

	if r.version >= storedFieldsVersionNumChunks {
		// discard num_chunks
		if _, err := metaIn.ReadVLong(); err != nil {
			_ = metaIn.Close()
			return nil, err
		}
	}
	if r.version >= storedFieldsVersionMeta {
		// consume dirty chunks/docs stats we wrote
		if _, err := metaIn.ReadVLong(); err != nil {
			_ = metaIn.Close()
			return nil, err
		}
		if _, err := metaIn.ReadVLong(); err != nil {
			_ = metaIn.Close()
			return nil, err
		}
	}

	if metaIn != nil {
		if err := checkStoredFieldsFooter(metaIn); err != nil {
			_ = metaIn.Close()
			return nil, err
		}
		if err := metaIn.Close(); err != nil {
			return nil, err
		}
	}

	success = true
	return r, nil
}

// ensureOpen reproduces
// {@code if (closed) throw new AlreadyClosedException("this FieldsReader is closed");}.
func (r *Lucene50CompressingStoredFieldsReader) ensureOpen() error {
	if r.closed {
		return ErrAlreadyClosed
	}
	return nil
}

// Close closes the underlying IndexInputs.
//
// Mirrors {@code if (!closed) { IOUtils.close(indexReader, fieldsStream); closed = true; }}.
func (r *Lucene50CompressingStoredFieldsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var first error
	if r.indexReader != nil {
		if err := r.indexReader.Close(); err != nil {
			first = err
		}
	}
	if r.fieldsStream != nil {
		if err := r.fieldsStream.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// readStoredField reproduces the private static
// {@code readField(DataInput in, StoredFieldVisitor visitor, FieldInfo info, int bits)}.
//
// Java's BYTE_ARR branch hands the visitor a StoredFieldDataInput; Gocene's
// spi.StoredFieldVisitor.BinaryField takes the bytes themselves, so the run is
// read first — the same rendering codecs/lucene90/compressing already uses.
func readStoredField(in store.DataInput, visitor spi.StoredFieldVisitor, info *index.FieldInfo, bits int) error {
	switch bits & storedTypeMask {
	case storedTypeByteArr:
		length, err := in.ReadVInt()
		if err != nil {
			return err
		}
		b := make([]byte, length)
		if err := in.ReadBytes(b, 0, len(b)); err != nil {
			return err
		}
		return visitor.BinaryField(info, b)
	case storedTypeString:
		s, err := in.ReadString()
		if err != nil {
			return err
		}
		return visitor.StringField(info, s)
	case storedTypeNumericInt:
		v, err := in.ReadZInt()
		if err != nil {
			return err
		}
		return visitor.IntField(info, int(v))
	case storedTypeNumericFloat:
		v, err := readStoredZFloat(in)
		if err != nil {
			return err
		}
		return visitor.FloatField(info, v)
	case storedTypeNumericLong:
		v, err := readStoredTLong(in)
		if err != nil {
			return err
		}
		return visitor.LongField(info, v)
	case storedTypeNumericDouble:
		v, err := readStoredZDouble(in)
		if err != nil {
			return err
		}
		return visitor.DoubleField(info, v)
	default:
		return fmt.Errorf("Unknown type flag: %x", bits)
	}
}

// skipStoredField reproduces the private static
// {@code skipField(DataInput in, int bits)}.
func skipStoredField(in store.DataInput, bits int) error {
	switch bits & storedTypeMask {
	case storedTypeByteArr, storedTypeString:
		length, err := in.ReadVInt()
		if err != nil {
			return err
		}
		return in.SkipBytes(int64(length))
	case storedTypeNumericInt:
		_, err := in.ReadZInt()
		return err
	case storedTypeNumericFloat:
		_, err := readStoredZFloat(in)
		return err
	case storedTypeNumericLong:
		_, err := readStoredTLong(in)
		return err
	case storedTypeNumericDouble:
		_, err := readStoredZDouble(in)
		return err
	default:
		return fmt.Errorf("Unknown type flag: %x", bits)
	}
}

// readStoredZFloat reads a float in a variable-length format. It reads between
// one and five bytes; small integral values typically take fewer bytes.
//
// Mirrors {@code static float readZFloat(DataInput in)}.
func readStoredZFloat(in store.DataInput) (float32, error) {
	raw, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b := int(raw) & 0xFF
	switch {
	case b == 0xFF:
		// negative value
		bits, err := in.ReadInt()
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(uint32(bits)), nil
	case b&0x80 != 0:
		// small integer [-1..125]
		return float32((b & 0x7f) - 1), nil
	default:
		// positive float
		s, err := in.ReadShort()
		if err != nil {
			return 0, err
		}
		lo, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		bits := int32(b)<<24 | int32(uint16(s))<<8 | int32(lo)&0xFF
		return math.Float32frombits(uint32(bits)), nil
	}
}

// readStoredZDouble reads a double in a variable-length format. It reads
// between one and nine bytes; small integral values typically take fewer
// bytes.
//
// Mirrors {@code static double readZDouble(DataInput in)}.
func readStoredZDouble(in store.DataInput) (float64, error) {
	raw, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b := int(raw) & 0xFF
	switch {
	case b == 0xFF:
		// negative value
		bits, err := in.ReadLong()
		if err != nil {
			return 0, err
		}
		return math.Float64frombits(uint64(bits)), nil
	case b == 0xFE:
		// float
		bits, err := in.ReadInt()
		if err != nil {
			return 0, err
		}
		return float64(math.Float32frombits(uint32(bits))), nil
	case b&0x80 != 0:
		// small integer [-1..124]
		return float64((b & 0x7f) - 1), nil
	default:
		// positive double
		i, err := in.ReadInt()
		if err != nil {
			return 0, err
		}
		s, err := in.ReadShort()
		if err != nil {
			return 0, err
		}
		lo, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		bits := int64(b)<<56 | int64(uint32(i))<<24 | int64(uint16(s))<<8 | int64(lo)&0xFF
		return math.Float64frombits(uint64(bits)), nil
	}
}

// readStoredTLong reads a long in a variable-length format. It reads between
// one and nine bytes; small values typically take fewer bytes.
//
// Mirrors {@code static long readTLong(DataInput in)}.
func readStoredTLong(in store.DataInput) (int64, error) {
	raw, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	header := int(raw) & 0xFF

	bits := int64(header & 0x1F)
	if header&0x20 != 0 {
		// continuation bit
		more, err := in.ReadVLong()
		if err != nil {
			return 0, err
		}
		bits |= more << 5
	}

	l := util.ZigZagDecodeInt64(bits)

	switch header & storedDayEncoding {
	case storedSecondEncoding:
		l *= storedSecond
	case storedHourEncoding:
		l *= storedHour
	case storedDayEncoding:
		l *= storedDay
	case 0:
		// uncompressed
	default:
		return 0, fmt.Errorf("lucene50/compressing: invalid TLong header %x", header)
	}

	return l, nil
}

// serializedDocument is a serialized document; its input has to be decoded in
// order to get an actual Document.
//
// Mirrors the package-private nested class
// {@code static class SerializedDocument}.
type serializedDocument struct {
	// in is the serialized data.
	in store.DataInput
	// length is the number of bytes on which the document is encoded.
	length int
	// numStoredFields is the number of stored fields.
	numStoredFields int
}

// storedFieldsBlockState keeps state about the current block of documents.
//
// Mirrors the private inner class {@code class BlockState}.
type storedFieldsBlockState struct {
	// owner is the Go stand-in for the implicit outer instance the Java inner
	// class reaches through Lucene50CompressingStoredFieldsReader.this.
	owner *Lucene50CompressingStoredFieldsReader

	docBase   int
	chunkDocs int

	// sliced records whether the block has been sliced, which happens for
	// large documents.
	sliced bool

	offsets         []int64
	numStoredFields []int64

	// startPointer is the start pointer at which the compressed documents can
	// be read.
	startPointer int64

	spare *util.BytesRef
	bytes *util.BytesRef
}

// newStoredFieldsBlockState reproduces
//
//	BlockState() {
//	  if (merging) { spare = new BytesRef(); bytes = new BytesRef(); }
//	  else { spare = bytes = null; }
//	}
func newStoredFieldsBlockState(owner *Lucene50CompressingStoredFieldsReader) *storedFieldsBlockState {
	s := &storedFieldsBlockState{
		owner:           owner,
		offsets:         make([]int64, 0),
		numStoredFields: make([]int64, 0),
	}
	if owner.merging {
		s.spare = util.NewBytesRefEmpty()
		s.bytes = util.NewBytesRefEmpty()
	}
	return s
}

// contains reproduces
// {@code return docID >= docBase && docID < docBase + chunkDocs;}.
func (s *storedFieldsBlockState) contains(docID int) bool {
	return docID >= s.docBase && docID < s.docBase+s.chunkDocs
}

// reset resets this block so that it stores state for the block that contains
// the given doc id.
//
// Mirrors {@code void reset(int docID)}, including the failure handling that
// zeroes chunkDocs so a corrupted block is never reused.
func (s *storedFieldsBlockState) reset(docID int) error {
	err := s.doReset(docID)
	if err != nil {
		// if the read failed, set chunkDocs to 0 so that it does not contain
		// any docs anymore and is not reused. This should help get consistent
		// exceptions when trying to get several documents which are in the
		// same corrupted block since it will force the header to be decoded
		// again
		s.chunkDocs = 0
	}
	return err
}

// doReset reproduces {@code private void doReset(int docID)}.
func (s *storedFieldsBlockState) doReset(docID int) error {
	r := s.owner
	docBase, err := r.fieldsStream.ReadVInt()
	if err != nil {
		return err
	}
	s.docBase = int(docBase)
	token, err := r.fieldsStream.ReadVInt()
	if err != nil {
		return err
	}
	if r.version >= storedFieldsVersionNumChunks {
		s.chunkDocs = int(uint32(token) >> 2)
	} else {
		s.chunkDocs = int(uint32(token) >> 1)
	}
	if !s.contains(docID) || s.docBase+s.chunkDocs > r.numDocs {
		return fmt.Errorf("Corrupted: docID=%d, docBase=%d, chunkDocs=%d, numDocs=%d",
			docID, s.docBase, s.chunkDocs, r.numDocs)
	}

	s.sliced = (token & 1) != 0

	s.offsets = growInt64Array(s.offsets, s.chunkDocs+1)
	s.numStoredFields = growInt64Array(s.numStoredFields, s.chunkDocs)

	if s.chunkDocs == 1 {
		v, err := r.fieldsStream.ReadVInt()
		if err != nil {
			return err
		}
		s.numStoredFields[0] = int64(v)
		v, err = r.fieldsStream.ReadVInt()
		if err != nil {
			return err
		}
		s.offsets[1] = int64(v)
	} else {
		// Number of stored fields per document
		bitsPerStoredFields, err := r.fieldsStream.ReadVInt()
		if err != nil {
			return err
		}
		switch {
		case bitsPerStoredFields == 0:
			v, err := r.fieldsStream.ReadVInt()
			if err != nil {
				return err
			}
			for i := 0; i < s.chunkDocs; i++ {
				s.numStoredFields[i] = int64(v)
			}
		case bitsPerStoredFields > 31:
			return fmt.Errorf("bitsPerStoredFields=%d", bitsPerStoredFields)
		default:
			it, err := packed.GetReaderIteratorNoHeader(
				r.fieldsStream, packed.FormatPacked, r.packedIntsVersion,
				s.chunkDocs, int(bitsPerStoredFields), 1024)
			if err != nil {
				return err
			}
			for i := 0; i < s.chunkDocs; {
				next, err := it.NextN(math.MaxInt32)
				if err != nil {
					return err
				}
				copy(s.numStoredFields[i:i+next.Length], next.Longs[next.Offset:next.Offset+next.Length])
				i += next.Length
			}
		}

		// The stream encodes the length of each document and we decode it into
		// a list of monotonically increasing offsets
		bitsPerLength, err := r.fieldsStream.ReadVInt()
		if err != nil {
			return err
		}
		switch {
		case bitsPerLength == 0:
			length, err := r.fieldsStream.ReadVInt()
			if err != nil {
				return err
			}
			for i := 0; i < s.chunkDocs; i++ {
				s.offsets[1+i] = int64(1+i) * int64(length)
			}
		case bitsPerStoredFields > 31:
			return fmt.Errorf("bitsPerLength=%d", bitsPerLength)
		default:
			it, err := packed.GetReaderIteratorNoHeader(
				r.fieldsStream, packed.FormatPacked, r.packedIntsVersion,
				s.chunkDocs, int(bitsPerLength), 1024)
			if err != nil {
				return err
			}
			for i := 0; i < s.chunkDocs; {
				next, err := it.NextN(math.MaxInt32)
				if err != nil {
					return err
				}
				copy(s.offsets[i+1:i+1+next.Length], next.Longs[next.Offset:next.Offset+next.Length])
				i += next.Length
			}
			for i := 0; i < s.chunkDocs; i++ {
				s.offsets[i+1] += s.offsets[i]
			}
		}

		// Additional validation: only the empty document has a serialized
		// length of 0
		for i := 0; i < s.chunkDocs; i++ {
			length := s.offsets[i+1] - s.offsets[i]
			storedFields := s.numStoredFields[i]
			if (length == 0) != (storedFields == 0) {
				return fmt.Errorf("length=%d, numStoredFields=%d", length, storedFields)
			}
		}
	}

	s.startPointer = r.fieldsStream.GetFilePointer()

	if r.merging {
		totalLength := int(s.offsets[s.chunkDocs])
		// decompress eagerly
		if s.sliced {
			s.bytes.Offset, s.bytes.Length = 0, 0
			for decompressed := 0; decompressed < totalLength; {
				toDecompress := totalLength - decompressed
				if toDecompress > r.chunkSize {
					toDecompress = r.chunkSize
				}
				if err := r.decompressor.Decompress(
					r.fieldsStream, toDecompress, 0, toDecompress, s.spare,
				); err != nil {
					return err
				}
				s.bytes.Bytes = growByteArray(s.bytes.Bytes, s.bytes.Length+s.spare.Length)
				copy(s.bytes.Bytes[s.bytes.Length:s.bytes.Length+s.spare.Length],
					s.spare.Bytes[s.spare.Offset:s.spare.Offset+s.spare.Length])
				s.bytes.Length += s.spare.Length
				decompressed += toDecompress
			}
		} else {
			if err := r.decompressor.Decompress(
				r.fieldsStream, totalLength, 0, totalLength, s.bytes,
			); err != nil {
				return err
			}
		}
		if s.bytes.Length != totalLength {
			return fmt.Errorf("Corrupted: expected chunk size = %d, got %d", totalLength, s.bytes.Length)
		}
	}
	return nil
}

// document gets the serialized representation of the given docID. This docID
// has to be contained in the current block.
//
// Mirrors {@code SerializedDocument document(int docID)}.
func (s *storedFieldsBlockState) document(docID int) (*serializedDocument, error) {
	if !s.contains(docID) {
		return nil, fmt.Errorf("lucene50/compressing: docID %d is not in the current block", docID)
	}
	r := s.owner

	idx := docID - s.docBase
	offset := int(s.offsets[idx])
	length := int(s.offsets[idx+1]) - offset
	totalLength := int(s.offsets[s.chunkDocs])
	numStoredFields := int(s.numStoredFields[idx])

	var bytesRef *util.BytesRef
	if r.merging {
		bytesRef = s.bytes
	} else {
		bytesRef = util.NewBytesRefEmpty()
	}

	var documentInput store.DataInput
	switch {
	case length == 0:
		// empty
		documentInput = store.NewByteArrayDataInput(nil)
	case r.merging:
		// already decompressed
		documentInput = store.NewByteArrayDataInputWithOffset(
			bytesRef.Bytes, bytesRef.Offset+offset, length)
	case s.sliced:
		if err := r.fieldsStream.SetPosition(s.startPointer); err != nil {
			return nil, err
		}
		first := length
		if first > r.chunkSize-offset {
			first = r.chunkSize - offset
		}
		if err := r.decompressor.Decompress(
			r.fieldsStream, r.chunkSize, offset, first, bytesRef,
		); err != nil {
			return nil, err
		}
		documentInput = newSlicedStoredFieldsDataInput(r, bytesRef, length)
	default:
		if err := r.fieldsStream.SetPosition(s.startPointer); err != nil {
			return nil, err
		}
		if err := r.decompressor.Decompress(
			r.fieldsStream, totalLength, offset, length, bytesRef,
		); err != nil {
			return nil, err
		}
		documentInput = store.NewByteArrayDataInputWithOffset(bytesRef.Bytes, bytesRef.Offset, bytesRef.Length)
	}

	return &serializedDocument{
		in:              bstore.NewEndiannessReverserDataInput(documentInput),
		length:          length,
		numStoredFields: numStoredFields,
	}, nil
}

// slicedStoredFieldsDataInput is the anonymous DataInput that
// BlockState.document returns for a sliced (large) document: it refills the
// BytesRef from the fields stream one chunk at a time.
type slicedStoredFieldsDataInput struct {
	// BaseDataInput carries the concrete members org.apache.lucene.store.DataInput
	// supplies on top of readByte / readBytes / skipBytes, which are the three
	// the anonymous Java class overrides. Its Core is this input.
	spi.BaseDataInput

	owner        *Lucene50CompressingStoredFieldsReader
	bytes        *util.BytesRef
	length       int
	decompressed int
}

// newSlicedStoredFieldsDataInput reproduces the anonymous class's field
// initialiser {@code int decompressed = bytes.length;}.
func newSlicedStoredFieldsDataInput(
	owner *Lucene50CompressingStoredFieldsReader, bytesRef *util.BytesRef, length int,
) *slicedStoredFieldsDataInput {
	d := &slicedStoredFieldsDataInput{
		owner:        owner,
		bytes:        bytesRef,
		length:       length,
		decompressed: bytesRef.Length,
	}
	d.Core = d
	return d
}

// fillBuffer reproduces {@code void fillBuffer()}.
func (d *slicedStoredFieldsDataInput) fillBuffer() error {
	if d.decompressed == d.length {
		return io.EOF
	}
	toDecompress := d.length - d.decompressed
	if toDecompress > d.owner.chunkSize {
		toDecompress = d.owner.chunkSize
	}
	if err := d.owner.decompressor.Decompress(
		d.owner.fieldsStream, toDecompress, 0, toDecompress, d.bytes,
	); err != nil {
		return err
	}
	d.decompressed += toDecompress
	return nil
}

// ReadByte reproduces
//
//	if (bytes.length == 0) fillBuffer();
//	--bytes.length;
//	return bytes.bytes[bytes.offset++];
func (d *slicedStoredFieldsDataInput) ReadByte() (byte, error) {
	if d.bytes.Length == 0 {
		if err := d.fillBuffer(); err != nil {
			return 0, err
		}
	}
	d.bytes.Length--
	b := d.bytes.Bytes[d.bytes.Offset]
	d.bytes.Offset++
	return b, nil
}

// ReadBytes reproduces {@code public void readBytes(byte[] b, int offset, int len)}.
func (d *slicedStoredFieldsDataInput) ReadBytes(b []byte, offset, length int) error {
	for length > d.bytes.Length {
		copy(b[offset:offset+d.bytes.Length], d.bytes.Bytes[d.bytes.Offset:d.bytes.Offset+d.bytes.Length])
		length -= d.bytes.Length
		offset += d.bytes.Length
		if err := d.fillBuffer(); err != nil {
			return err
		}
	}
	copy(b[offset:offset+length], d.bytes.Bytes[d.bytes.Offset:d.bytes.Offset+length])
	d.bytes.Offset += length
	d.bytes.Length -= length
	return nil
}

// SkipBytes reproduces {@code public void skipBytes(long numBytes)}.
func (d *slicedStoredFieldsDataInput) SkipBytes(numBytes int64) error {
	if numBytes < 0 {
		return fmt.Errorf("numBytes must be >= 0, got %d", numBytes)
	}
	for numBytes > int64(d.bytes.Length) {
		numBytes -= int64(d.bytes.Length)
		if err := d.fillBuffer(); err != nil {
			return err
		}
	}
	d.bytes.Offset += int(numBytes)
	d.bytes.Length -= int(numBytes)
	return nil
}

var _ store.DataInput = (*slicedStoredFieldsDataInput)(nil)

// checkStoredFieldsFooter renders
// {@code CodecUtil.checkFooter(ChecksumIndexInput)} against the
// endianness-reversing checksum input this reader opens. codecs.CheckFooter
// is typed to the concrete *store.ChecksumIndexInput, which the legacy
// wrapper is not, so the body is spelled out here — the same rendering
// backward_codecs/lucene80 and lucene60 already use.
func checkStoredFieldsFooter(in *bstore.EndiannessReverserChecksumIndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	expected := int64(codecs.FooterLength())
	if remaining < expected {
		return fmt.Errorf("misplaced codec footer (file truncated?): remaining=%d, expected=%d",
			remaining, expected)
	}
	if remaining > expected {
		return fmt.Errorf("misplaced codec footer (file extended?): remaining=%d, expected=%d",
			remaining, expected)
	}
	magic, err := in.ReadInt()
	if err != nil {
		return err
	}
	if magic != codecs.FOOTER_MAGIC {
		return fmt.Errorf("codec footer mismatch: actual footer=%x vs expected footer=%x",
			magic, codecs.FOOTER_MAGIC)
	}
	algorithmID, err := in.ReadInt()
	if err != nil {
		return err
	}
	if algorithmID != 0 {
		return fmt.Errorf("codec footer mismatch: unknown algorithmID: %d", algorithmID)
	}
	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := in.ReadLong()
	if err != nil {
		return err
	}
	if expectedChecksum != actualChecksum {
		return fmt.Errorf("checksum failed (hardware problem?) : expected=%x actual=%x",
			expectedChecksum, actualChecksum)
	}
	return nil
}

// serializedDocumentFor reproduces
//
//	SerializedDocument serializedDocument(int docID) throws IOException {
//	  if (state.contains(docID) == false) {
//	    fieldsStream.seek(indexReader.getStartPointer(docID));
//	    state.reset(docID);
//	  }
//	  return state.document(docID);
//	}
func (r *Lucene50CompressingStoredFieldsReader) serializedDocumentFor(docID int) (*serializedDocument, error) {
	if !r.state.contains(docID) {
		startPointer, err := r.indexReader.GetStartPointer(docID)
		if err != nil {
			return nil, err
		}
		if err := r.fieldsStream.SetPosition(startPointer); err != nil {
			return nil, err
		}
		if err := r.state.reset(docID); err != nil {
			return nil, err
		}
	}
	return r.state.document(docID)
}

// VisitDocument reproduces
// {@code public void document(int docID, StoredFieldVisitor visitor)}.
//
// Gocene's spi.StoredFieldsReader names the member VisitDocument; the body is
// Java's document(int, StoredFieldVisitor) statement for statement.
func (r *Lucene50CompressingStoredFieldsReader) VisitDocument(docID int, visitor spi.StoredFieldVisitor) error {
	doc, err := r.serializedDocumentFor(docID)
	if err != nil {
		return err
	}

	for fieldIDX := 0; fieldIDX < doc.numStoredFields; fieldIDX++ {
		infoAndBits, err := doc.in.ReadVLong()
		if err != nil {
			return err
		}
		fieldNumber := int(uint64(infoAndBits) >> storedTypeBits)
		var fieldInfo *index.FieldInfo
		if r.fieldInfos != nil {
			fieldInfo = r.fieldInfos.GetByNumber(fieldNumber)
		}

		bits := int(infoAndBits & storedTypeMask)

		status, err := visitor.NeedsField(fieldInfo)
		if err != nil {
			return err
		}
		switch status {
		case spi.StoredFieldVisitorStatusYes:
			if err := readStoredField(doc.in, visitor, fieldInfo, bits); err != nil {
				return err
			}
		case spi.StoredFieldVisitorStatusNo:
			if fieldIDX == doc.numStoredFields-1 {
				// don't skipField on last field value; treat like STOP
				return nil
			}
			if err := skipStoredField(doc.in, bits); err != nil {
				return err
			}
		case spi.StoredFieldVisitorStatusStop:
			return nil
		}
	}
	return nil
}

// CloneReader reproduces
//
//	public StoredFieldsReader clone() {
//	  ensureOpen();
//	  return new Lucene50CompressingStoredFieldsReader(this, false);
//	}
//
// spi.StoredFieldsReader does not declare clone(); the member is kept on the
// concrete type under a Go-legal name, because Clone would collide with
// nothing but reads better spelled out.
func (r *Lucene50CompressingStoredFieldsReader) CloneReader() (spi.StoredFieldsReader, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}
	return cloneReader(r, false), nil
}

// GetMergeInstance reproduces
//
//	public StoredFieldsReader getMergeInstance() {
//	  ensureOpen();
//	  return new Lucene50CompressingStoredFieldsReader(this, true);
//	}
func (r *Lucene50CompressingStoredFieldsReader) GetMergeInstance() (spi.StoredFieldsReader, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}
	return cloneReader(r, true), nil
}

// CheckIntegrity reproduces
//
//	indexReader.checkIntegrity();
//	CodecUtil.checksumEntireFile(fieldsStream);
func (r *Lucene50CompressingStoredFieldsReader) CheckIntegrity() error {
	if err := r.indexReader.CheckIntegrity(); err != nil {
		return err
	}
	_, err := codecs.ChecksumEntireFile(r.fieldsStream)
	return err
}

// String reproduces
//
//	return getClass().getSimpleName() + "(mode=" + compressionMode + ",chunksize=" + chunkSize + ")";
func (r *Lucene50CompressingStoredFieldsReader) String() string {
	return fmt.Sprintf("Lucene50CompressingStoredFieldsReader(mode=%v,chunksize=%d)",
		r.compressionMode, r.chunkSize)
}

// GetChunkSize exposes the chunk size, mirroring the package-private field the
// Java term-vectors reader in the same package reads directly.
func (r *Lucene50CompressingStoredFieldsReader) GetChunkSize() int { return r.chunkSize }

// GetVersion exposes the format version, mirroring the package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetVersion() int { return r.version }

// GetMaxPointer exposes the max pointer, mirroring the package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetMaxPointer() int64 { return r.maxPointer }

// GetIndexReader exposes the fields index, mirroring the package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetIndexReader() FieldsIndex { return r.indexReader }

// GetNumDocs exposes the document count, mirroring the package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetNumDocs() int { return r.numDocs }

// GetFieldsStream exposes the .fdt input, mirroring the package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetFieldsStream() store.IndexInput {
	return r.fieldsStream
}

// GetCompressionMode exposes the compression mode, mirroring the
// package-private field.
func (r *Lucene50CompressingStoredFieldsReader) GetCompressionMode() bccompressing.CompressionMode {
	return r.compressionMode
}

var _ spi.StoredFieldsReader = (*Lucene50CompressingStoredFieldsReader)(nil)

// ─── ArrayUtil.grow renderings ──────────────────────────────────────────────

// growInt64Array renders org.apache.lucene.util.ArrayUtil#grow(long[], int).
func growInt64Array(array []int64, minSize int) []int64 {
	if len(array) < minSize {
		grown := make([]int64, util.Oversize(minSize, 8))
		copy(grown, array)
		return grown
	}
	return array
}

// growByteArray renders org.apache.lucene.util.ArrayUtil#grow(byte[], int).
func growByteArray(array []byte, minSize int) []byte {
	if len(array) < minSize {
		grown := make([]byte, util.Oversize(minSize, 1))
		copy(grown, array)
		return grown
	}
	return array
}
