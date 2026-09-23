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
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// FieldsExtension is the extension for the .fdt data file. Mirrors the
	// public constant Lucene90CompressingStoredFieldsWriter.FIELDS_EXTENSION.
	FieldsExtension = "fdt"

	// indexExtension is the extension for the .fdx index file.
	indexExtension = "fdx"

	// metaExtension is the extension for the .fdm metadata file.
	metaExtension = "fdm"

	// indexCodecName is the codec name for the index files.
	// Matches Lucene90CompressingStoredFieldsWriter.INDEX_CODEC_NAME.
	indexCodecName = "Lucene90FieldsIndex"

	// versionStart is VERSION_START in the Java reference.
	versionStart = int32(1)

	// versionCurrent is VERSION_CURRENT in the Java reference.
	versionCurrent = versionStart

	// metaVersionStart is META_VERSION_START in the Java reference.
	metaVersionStart = int32(0)

	// Field type codes – must match Java statics exactly.
	typeString        = int64(0x00)
	typeByteArray     = int64(0x01)
	typeNumericInt    = int64(0x02)
	typeNumericFloat  = int64(0x03)
	typeNumericLong   = int64(0x04)
	typeNumericDouble = int64(0x05)

	// typeBits is PackedInts.bitsRequired(NUMERIC_DOUBLE) = 3.
	// All type constants fit in 3 bits.
	typeBits = int64(3)

	// typeMask masks off the type bits from infoAndBits.
	typeMask = int64((1 << typeBits) - 1)

	// Timestamp compression constants for TLong encoding.
	tlongSecond         = int64(1000)
	tlongHour           = 60 * 60 * tlongSecond
	tlongDay            = 24 * tlongHour
	tlongSecondEncoding = 0x40
	tlongHourEncoding   = 0x80
	tlongDayEncoding    = 0xC0
)

// negativezeroFloat32 is the bit pattern for -0f, used in ZFloat encoding.
// Mirrors NEGATIVE_ZERO_FLOAT = Float.floatToIntBits(-0f). The Go constant
// expression -0.0 is +0, so the negative zero is built with math.Copysign.
var negativezeroFloat32 = math.Float32bits(float32(math.Copysign(0, -1)))

// negativezeroFloat64 is the bit pattern for -0d, used in ZDouble encoding.
// Mirrors NEGATIVE_ZERO_DOUBLE = Double.doubleToLongBits(-0d); see
// negativezeroFloat32 for why math.Copysign is used.
var negativezeroFloat64 = math.Float64bits(math.Copysign(0, -1))

// ---------------------------------------------------------------------------
// Lucene90CompressingStoredFieldsFormat – the StoredFieldsFormat
// ---------------------------------------------------------------------------

// Lucene90CompressingStoredFieldsWriter is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsWriter.
type Lucene90CompressingStoredFieldsWriter struct {
	segment         string
	si              *index.SegmentInfo
	indexWriter     *FieldsIndexWriter
	metaStream      store.IndexOutput
	fieldsStream    store.IndexOutput
	compressor      compressing.Compressor
	compressionMode compressing.CompressionMode
	chunkSize       int
	maxDocsPerChunk int

	// bufferedDocs holds the uncompressed bytes for the current chunk.
	bufferedDocs *store.ByteBuffersDataOutput
	// numStoredFields[i] = number of stored fields in the i-th buffered document.
	numStoredFields []int32
	// endOffsets[i] = end offset (in bufferedDocs) of the i-th buffered document.
	endOffsets []int32

	docBase              int32 // first docID of the current chunk
	numBufferedDocs      int   // number of docs buffered in the current chunk
	numStoredFieldsInDoc int32 // fields written into the current document

	numChunks      int64
	numDirtyChunks int64
	numDirtyDocs   int64

	closed bool
}

func newLucene90CompressingStoredFieldsWriter(
	dir store.Directory,
	si *index.SegmentInfo,
	ctx store.IOContext,
	formatName string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) (*Lucene90CompressingStoredFieldsWriter, error) {
	segment := si.Name()
	suffix := "" // Lucene90 uses empty suffix

	var metaStream, fieldsStream store.IndexOutput
	var err error

	// Allocate meta stream (.fdm), wrapped in a checksum tracker so
	// WriteFooter can record the CRC32 at close time (Lucene's Java
	// FSDirectory wraps outputs in BufferedIndexOutput/FSIndexOutput which
	// implement checksumming; Gocene's FS directories do not, so we add
	// the wrapper here — byte-identical output, just adds CRC bookkeeping).
	metaName := store.SegmentFileName(segment, suffix, metaExtension)
	{
		raw, err := dir.CreateOutput(metaName, ctx)
		if err != nil {
			return nil, fmt.Errorf("lucene90/compressing: create %s: %w", metaName, err)
		}
		metaStream = store.NewChecksumIndexOutput(raw)
	}

	success := false
	defer func() {
		if !success {
			_ = metaStream.Close()
			if fieldsStream != nil {
				_ = fieldsStream.Close()
			}
		}
	}()

	if err := gcodecs.WriteIndexHeader(
		metaStream,
		indexCodecName+"Meta",
		versionCurrent,
		si.GetID(),
		suffix,
	); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: write fdm header: %w", err)
	}

	// Allocate fields stream (.fdt), also checksum-wrapped.
	fdtName := store.SegmentFileName(segment, suffix, FieldsExtension)
	{
		raw, err := dir.CreateOutput(fdtName, ctx)
		if err != nil {
			return nil, fmt.Errorf("lucene90/compressing: create %s: %w", fdtName, err)
		}
		fieldsStream = store.NewChecksumIndexOutput(raw)
	}

	if err := gcodecs.WriteIndexHeader(
		fieldsStream,
		formatName,
		versionCurrent,
		si.GetID(),
		suffix,
	); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: write fdt header: %w", err)
	}

	// Write chunkSize to meta stream (read back by reader).
	if err := metaStream.WriteVInt(int32(chunkSize)); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: write chunkSize: %w", err)
	}

	indexWriter, err := NewFieldsIndexWriter(
		dir, segment, suffix, indexExtension, indexCodecName, si.GetID(), blockShift, ctx,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: create fields index writer: %w", err)
	}

	success = true
	return &Lucene90CompressingStoredFieldsWriter{
		segment:         segment,
		si:              si,
		indexWriter:     indexWriter,
		metaStream:      metaStream,
		fieldsStream:    fieldsStream,
		compressor:      compressionMode.NewCompressor(),
		compressionMode: compressionMode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		bufferedDocs:    store.NewByteBuffersDataOutput(),
		numStoredFields: make([]int32, 16),
		endOffsets:      make([]int32, 16),
	}, nil
}

// StartDocument begins accumulating a new document.
func (w *Lucene90CompressingStoredFieldsWriter) StartDocument() error {
	return nil // no-op in Java too
}

// FinishDocument closes the current document and potentially flushes a chunk.
func (w *Lucene90CompressingStoredFieldsWriter) FinishDocument() error {
	if w.numBufferedDocs >= len(w.numStoredFields) {
		newLen := oversizeInt(w.numBufferedDocs + 1)
		nsf := make([]int32, newLen)
		copy(nsf, w.numStoredFields)
		w.numStoredFields = nsf
		eo := make([]int32, newLen)
		copy(eo, w.endOffsets)
		w.endOffsets = eo
	}
	w.numStoredFields[w.numBufferedDocs] = w.numStoredFieldsInDoc
	w.numStoredFieldsInDoc = 0
	w.endOffsets[w.numBufferedDocs] = int32(w.bufferedDocs.Size())
	w.numBufferedDocs++
	if w.triggerFlush() {
		return w.flush(false)
	}
	return nil
}

// storedValueField is the interface implemented by document types that
// expose a typed StoredValue. document.StoredField satisfies this interface
// via its StoredValue() method.
type storedValueField interface {
	StoredValue() *document.StoredValue
}

// WriteField serializes one stored field of the current document.
//
// Java declares one writeField overload per value type, each of them
// computing the record header as
//
//	final long infoAndBits = (((long) info.number) << TYPE_BITS) | TYPE;
//
// (Lucene90CompressingStoredFieldsWriter.java:271-326). Go has no
// overloading, so the value arrives behind spi.IndexableField and the type
// tag is recovered from its StoredValue(), which gives the exact tag
// without ambiguity. Fields that do not expose a StoredValue fall back to
// NumericValue() then StringValue().
//
// The field number is info.Number() — exactly Java's info.number. It is
// NOT a per-document sequence: the two coincide only when a document
// stores every field of the segment, in ascending field order.
func (w *Lucene90CompressingStoredFieldsWriter) WriteField(info *spi.FieldInfo, field spi.IndexableField) error {
	if info == nil {
		return errors.New("lucene90/compressing: WriteField requires a non-nil FieldInfo")
	}
	w.numStoredFieldsInDoc++

	fieldNumber := int64(info.Number())

	// Prefer StoredValue() for exact type tagging — this avoids the
	// ambiguity where document.stringValue.Binary() returns []byte(string)
	// and would be mis-classified as a binary field.
	if svf, ok := field.(storedValueField); ok {
		if sv := svf.StoredValue(); sv != nil {
			return w.writeStoredValue(fieldNumber, sv)
		}
	}

	// Fallback for fields that do not implement storedValueField: use
	// NumericValue() first, then StringValue(). BinaryValue() is NOT
	// checked here because document.stringValue.Binary() is non-nil for
	// any non-empty string, which would cause mis-classification.
	if nv := field.NumericValue(); nv != nil {
		return w.writeNumericValue(fieldNumber, nv)
	}
	// String field (the value may be "" for an empty stored string).
	infoAndBits := (fieldNumber << typeBits) | typeString
	if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
		return err
	}
	return w.bufferedDocs.WriteString(field.StringValue())
}

// writeStoredValue encodes a StoredValue into the buffered document stream.
func (w *Lucene90CompressingStoredFieldsWriter) writeStoredValue(fieldNumber int64, sv *document.StoredValue) error {
	switch sv.Type() {
	case document.StoredValueTypeBinary:
		bv := sv.BinaryValue()
		infoAndBits := (fieldNumber << typeBits) | typeByteArray
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		if err := w.bufferedDocs.WriteVInt(int32(len(bv))); err != nil {
			return err
		}
		return w.bufferedDocs.WriteBytes(bv, 0, len(bv))
	case document.StoredValueTypeString:
		infoAndBits := (fieldNumber << typeBits) | typeString
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return w.bufferedDocs.WriteString(sv.StringValue())
	case document.StoredValueTypeInteger:
		infoAndBits := (fieldNumber << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, sv.IntValue())
	case document.StoredValueTypeLong:
		infoAndBits := (fieldNumber << typeBits) | typeNumericLong
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeTLong(w.bufferedDocs, sv.LongValue())
	case document.StoredValueTypeFloat:
		infoAndBits := (fieldNumber << typeBits) | typeNumericFloat
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZFloat(w.bufferedDocs, sv.FloatValue())
	case document.StoredValueTypeDouble:
		infoAndBits := (fieldNumber << typeBits) | typeNumericDouble
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZDouble(w.bufferedDocs, sv.DoubleValue())
	default:
		return fmt.Errorf("lucene90/compressing: unsupported StoredValue type %v", sv.Type())
	}
}

// writeNumericValue is the fallback path for fields that do not implement
// storedValueField but do return a NumericValue().
func (w *Lucene90CompressingStoredFieldsWriter) writeNumericValue(fieldNumber int64, nv interface{}) error {
	switch v := nv.(type) {
	case int:
		infoAndBits := (fieldNumber << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, int32(v))
	case int32:
		infoAndBits := (fieldNumber << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, v)
	case int64:
		infoAndBits := (fieldNumber << typeBits) | typeNumericLong
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeTLong(w.bufferedDocs, v)
	case float32:
		infoAndBits := (fieldNumber << typeBits) | typeNumericFloat
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZFloat(w.bufferedDocs, v)
	case float64:
		infoAndBits := (fieldNumber << typeBits) | typeNumericDouble
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZDouble(w.bufferedDocs, v)
	default:
		// Unknown numeric type — serialize as string.
		infoAndBits := (fieldNumber << typeBits) | typeString
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return w.bufferedDocs.WriteString(fmt.Sprintf("%v", v))
	}
}

func (w *Lucene90CompressingStoredFieldsWriter) triggerFlush() bool {
	return w.bufferedDocs.Size() >= int64(w.chunkSize) ||
		w.numBufferedDocs >= w.maxDocsPerChunk
}

// flush emits one compressed chunk to .fdt and records it in the index builder.
// force=true is used for the final (possibly incomplete) chunk.
func (w *Lucene90CompressingStoredFieldsWriter) flush(force bool) error {
	w.numChunks++
	if force {
		w.numDirtyChunks++
		w.numDirtyDocs += int64(w.numBufferedDocs)
	}

	// Record chunk start pointer BEFORE writing.
	if err := w.indexWriter.WriteIndex(w.numBufferedDocs, w.fieldsStream.GetFilePointer()); err != nil {
		return err
	}

	// Compute lengths from cumulative end offsets.
	lengths := make([]int32, w.numBufferedDocs)
	copy(lengths, w.endOffsets[:w.numBufferedDocs])
	for i := w.numBufferedDocs - 1; i > 0; i-- {
		lengths[i] = lengths[i] - lengths[i-1]
	}

	sliced := w.bufferedDocs.Size() >= 2*int64(w.chunkSize)
	dirtyChunk := force

	// Write chunk header.
	if err := w.writeChunkHeader(
		int(w.docBase),
		w.numBufferedDocs,
		w.numStoredFields[:w.numBufferedDocs],
		lengths,
		sliced,
		dirtyChunk,
	); err != nil {
		return err
	}

	// Compress buffered docs.
	// Build a ByteBuffersDataInput from the buffered bytes so the Compressor
	// can read them. ToArrayCopy gives us a fresh slice; wrapping it in
	// NewByteBuffersDataInput gives us the required interface type.
	rawBytes := w.bufferedDocs.ToArrayCopy()
	bbdi := store.NewByteBuffersDataInput(rawBytes)

	if sliced {
		capacity := int(bbdi.Length())
		for compressed := 0; compressed < capacity; compressed += w.chunkSize {
			l := w.chunkSize
			if capacity-compressed < l {
				l = capacity - compressed
			}
			slice, err := bbdi.Slice(int64(compressed), int64(l))
			if err != nil {
				return fmt.Errorf("lucene90/compressing: slice BBDI: %w", err)
			}
			if err := w.compressor.Compress(slice, w.fieldsStream); err != nil {
				return fmt.Errorf("lucene90/compressing: compress slice: %w", err)
			}
		}
	} else {
		if err := w.compressor.Compress(bbdi, w.fieldsStream); err != nil {
			return fmt.Errorf("lucene90/compressing: compress chunk: %w", err)
		}
	}

	// Reset for next chunk.
	w.docBase += int32(w.numBufferedDocs)
	w.numBufferedDocs = 0
	w.bufferedDocs.Reset()
	return nil
}

// writeChunkHeader writes the chunk header to .fdt.
//
// Format: docBase(VInt) + (numDocs<<2|flags)(VInt) + numStoredFields + lengths.
//
// numStoredFields and lengths follow the Java saveInts contract: when
// numBufferedDocs == 1 they are written as a bare VInt (no header byte);
// when numBufferedDocs > 1 they are encoded by StoredFieldsInts.writeInts.
// This matches the Java reference's private saveInts(int[], int, DataOutput).
func (w *Lucene90CompressingStoredFieldsWriter) writeChunkHeader(
	docBase, numBufferedDocs int,
	numStoredFields, lengths []int32,
	sliced, dirtyChunk bool,
) error {
	if err := w.fieldsStream.WriteVInt(int32(docBase)); err != nil {
		return err
	}
	slicedBit := int32(0)
	if sliced {
		slicedBit = 1
	}
	dirtyBit := int32(0)
	if dirtyChunk {
		dirtyBit = 2
	}
	code := int32(numBufferedDocs<<2) | dirtyBit | slicedBit
	if err := w.fieldsStream.WriteVInt(code); err != nil {
		return err
	}
	if err := saveInts(numStoredFields, numBufferedDocs, w.fieldsStream); err != nil {
		return err
	}
	return saveInts(lengths, numBufferedDocs, w.fieldsStream)
}

// saveInts mirrors the Java reference's private saveInts(int[], int, DataOutput):
// for length==1 it writes a single VInt; for length>1 it delegates to
// StoredFieldsInts.writeInts.
func saveInts(values []int32, length int, out store.DataOutput) error {
	if length == 1 {
		return out.WriteVInt(values[0])
	}
	return storedFieldsIntsWriteInts(values, 0, length, out)
}

// Finish satisfies the spi.StoredFieldsWriter interface. The real
// flush + footer emission happens inside Close (which the indexing
// chain calls immediately afterwards using the segment's DocCount as
// numDocs), so Finish is intentionally a no-op here to avoid
// double-flushing.
func (w *Lucene90CompressingStoredFieldsWriter) Finish(numDocs int) error {
	return nil
}

// finish is called internally by Close to flush remaining docs and write
// the index/meta footers.
func (w *Lucene90CompressingStoredFieldsWriter) finish(numDocs int) error {
	if w.numBufferedDocs > 0 {
		if err := w.flush(true); err != nil {
			return err
		}
	}
	if int(w.docBase) != numDocs {
		return fmt.Errorf(
			"lucene90/compressing: wrote %d docs, finish called with numDocs=%d",
			w.docBase, numDocs,
		)
	}

	maxPointer := w.fieldsStream.GetFilePointer()

	// FieldsIndexWriter writes the .fdx in full and appends its metadata
	// to the .fdm (FieldsIndexWriter#finish).
	if err := w.indexWriter.Finish(numDocs, maxPointer, w.metaStream); err != nil {
		return err
	}

	// Write dirty-chunk stats and footer to .fdm.
	if err := w.metaStream.WriteVLong(w.numChunks); err != nil {
		return err
	}
	if err := w.metaStream.WriteVLong(w.numDirtyChunks); err != nil {
		return err
	}
	if err := w.metaStream.WriteVLong(w.numDirtyDocs); err != nil {
		return err
	}
	if err := store.WriteFooter(w.metaStream); err != nil {
		return err
	}

	// Write footer to .fdt.
	return store.WriteFooter(w.fieldsStream)
}

// Close finalizes both streams. It calls finish with the segment's doc count.
func (w *Lucene90CompressingStoredFieldsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	numDocs := w.si.DocCount()
	var finishErr error
	if numDocs >= 0 {
		finishErr = w.finish(numDocs)
	}

	var errs []error
	if finishErr != nil {
		errs = append(errs, finishErr)
	}
	if err := w.compressor.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := w.metaStream.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := w.fieldsStream.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Compile-time guarantee.
var _ gcodecs.StoredFieldsWriter = (*Lucene90CompressingStoredFieldsWriter)(nil)

// ---------------------------------------------------------------------------
// Lucene90CompressingStoredFieldsReader
// ---------------------------------------------------------------------------

// writeZInt writes a 32-bit integer using zigzag+VInt encoding.
func writeZInt(out store.DataOutput, v int32) error {
	return out.WriteVInt(int32((v << 1) ^ (v >> 31)))
}

// writeZFloat mirrors Lucene's Lucene90CompressingStoredFieldsWriter.writeZFloat
// (Lucene90CompressingStoredFieldsWriter.java:356-370):
//
//	if (f == intVal && intVal >= -1 && intVal <= 0x7D && floatBits != NEGATIVE_ZERO_FLOAT) {
//	  out.writeByte((byte) (0x80 | (1 + intVal)));
//	} else if ((floatBits >>> 31) == 0) {
//	  out.writeByte((byte) (floatBits >> 24));
//	  out.writeShort((short) (floatBits >>> 8));
//	  out.writeByte((byte) floatBits);
//	} else {
//	  out.writeByte((byte) 0xFF);
//	  out.writeInt(floatBits);
//	}
//
// DataOutput.writeShort and DataOutput.writeInt are LITTLE-endian in Lucene
// 10.5.0 (DataOutput.java:73-89), and so are store.DataOutput's, so the
// multi-byte tail is emitted through them rather than by hand.
func writeZFloat(out store.DataOutput, f float32) error {
	intVal := int32(f)
	floatBits := math.Float32bits(f)

	if f == float32(intVal) && intVal >= -1 && intVal <= 0x7D && floatBits != negativezeroFloat32 {
		// small integer value [-1..125]: single byte
		return out.WriteByte(byte(0x80 | (1 + intVal)))
	} else if (floatBits >> 31) == 0 {
		// other positive floats: 4 bytes
		if err := out.WriteByte(byte(floatBits >> 24)); err != nil {
			return err
		}
		if err := out.WriteShort(int16(floatBits >> 8)); err != nil {
			return err
		}
		return out.WriteByte(byte(floatBits))
	}
	// other negative float: 5 bytes
	if err := out.WriteByte(0xFF); err != nil {
		return err
	}
	return out.WriteInt(int32(floatBits))
}

// writeZDouble mirrors Lucene's Lucene90CompressingStoredFieldsWriter.writeZDouble
// (Lucene90CompressingStoredFieldsWriter.java:388-408):
//
//	if (d == intVal && intVal >= -1 && intVal <= 0x7C && doubleBits != NEGATIVE_ZERO_DOUBLE) {
//	  out.writeByte((byte) (0x80 | (intVal + 1)));
//	  return;
//	} else if (d == (float) d) {
//	  out.writeByte((byte) 0xFE);
//	  out.writeInt(Float.floatToIntBits((float) d));
//	} else if ((doubleBits >>> 63) == 0) {
//	  out.writeByte((byte) (doubleBits >> 56));
//	  out.writeInt((int) (doubleBits >>> 24));
//	  out.writeShort((short) (doubleBits >>> 8));
//	  out.writeByte((byte) (doubleBits));
//	} else {
//	  out.writeByte((byte) 0xFF);
//	  out.writeLong(doubleBits);
//	}
//
// As in writeZFloat, the multi-byte tail goes through the little-endian
// writeShort/writeInt/writeLong of DataOutput.
func writeZDouble(out store.DataOutput, d float64) error {
	intVal := int64(d)
	doubleBits := math.Float64bits(d)

	if d == float64(intVal) && intVal >= -1 && intVal <= 0x7C && doubleBits != negativezeroFloat64 {
		// small integer value [-1..124]: single byte
		return out.WriteByte(byte(0x80 | (intVal + 1)))
	} else if d == float64(float32(d)) {
		// d has an accurate float representation: 5 bytes
		if err := out.WriteByte(0xFE); err != nil {
			return err
		}
		return out.WriteInt(int32(math.Float32bits(float32(d))))
	} else if (doubleBits >> 63) == 0 {
		// other positive doubles: 8 bytes
		if err := out.WriteByte(byte(doubleBits >> 56)); err != nil {
			return err
		}
		if err := out.WriteInt(int32(doubleBits >> 24)); err != nil {
			return err
		}
		if err := out.WriteShort(int16(doubleBits >> 8)); err != nil {
			return err
		}
		return out.WriteByte(byte(doubleBits))
	}
	// other negative doubles: 9 bytes
	if err := out.WriteByte(0xFF); err != nil {
		return err
	}
	return out.WriteLong(int64(doubleBits))
}

// writeTLong mirrors Lucene's Lucene90CompressingStoredFieldsWriter.writeTLong.
func writeTLong(out store.DataOutput, l int64) error {
	var header int
	if l%tlongSecond != 0 {
		header = 0
	} else if l%tlongDay == 0 {
		header = tlongDayEncoding
		l /= tlongDay
	} else if l%tlongHour == 0 {
		header = tlongHourEncoding
		l /= tlongHour
	} else {
		header = tlongSecondEncoding
		l /= tlongSecond
	}
	zigZagL := uint64((l << 1) ^ (l >> 63))
	header |= int(zigZagL & 0x1F)
	upperBits := zigZagL >> 5
	if upperBits != 0 {
		header |= 0x20
	}
	if err := out.WriteByte(byte(header)); err != nil {
		return err
	}
	if upperBits != 0 {
		return out.WriteVLong(int64(upperBits))
	}
	return nil
}

// oversizeInt returns a capacity >= n, growing exponentially to avoid
// repeated allocations. Mirrors ArrayUtil.oversize in Java.
func oversizeInt(n int) int {
	if n < 4 {
		return 4
	}
	extra := n >> 3
	if extra < 3 {
		extra = 3
	}
	return n + extra
}

// ---------------------------------------------------------------------------
// Stub types kept for backwards compatibility with the original Sprint 48 stub.
// ---------------------------------------------------------------------------
