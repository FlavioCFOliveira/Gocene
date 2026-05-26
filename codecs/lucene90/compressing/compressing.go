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

// Package compressing hosts the Lucene90 compressing stored-fields and
// term-vectors codecs. It is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.
//
// # Wire format
//
// The package produces three files per segment:
//
//   - .fdt  — field data, IndexHeader(formatName, VERSION_CURRENT) + chunks +
//     IndexFooter. Each chunk: docBase(VInt) + numDocs<<2|flags(VInt) +
//     numStoredFields per doc (StoredFieldsInts) + lengths per doc
//     (StoredFieldsInts) + compressed payload.
//
//   - .fdm  — metadata, IndexHeader("Lucene90FieldsIndexMeta", VERSION_CURRENT)
//
//   - chunkSize(VInt) + numDocs(Int) + blockShift(Int) + (totalChunks+1)(Int)
//
//   - DirectMonotonicWriter(docs) data offsets/lengths
//
//   - DirectMonotonicWriter(startPointers) data offsets/lengths
//
//   - maxPointer(Long) + numChunks(VLong) + numDirtyChunks(VLong)
//
//   - numDirtyDocs(VLong) + IndexFooter.
//
//   - .fdx  — index, IndexHeader("Lucene90FieldsIndexIdx", VERSION_CURRENT) +
//     DirectMonotonic data blobs + IndexFooter.
//
// # Field encoding in .fdt chunks
//
// Each stored field is prefixed with a VLong of (fieldNumber << TYPE_BITS | type)
// where fieldNumber is the 0-based per-document sequential ID assigned by the
// writer in place of the FieldInfo.number that Lucene uses. Numeric values use
// ZInt/TLong/ZFloat/ZDouble encodings. Strings and binary blobs are prefixed
// with a VInt length.
//
// The reader resolves field names from the embedded field number via the
// FieldInfos passed at construction time (identical to Lucene's approach).
// When FieldInfos is nil the field name defaults to the empty string.
//
// For segments whose fields appear in the same order across every document and
// whose FieldInfo numbers are assigned 0, 1, 2, … in that order (the common
// case for newly created segments), Gocene's sequential per-document IDs
// match Lucene's FieldInfo.number values and the .fdt bytes are byte-identical.
//
// # Divergences from Lucene 10.4.0
//
//   - Gocene's StoredFieldsWriter.WriteField receives an IndexableField with
//     no FieldInfo, so sequential 0-based field IDs are used instead of the
//     FieldInfo.number values that Lucene stamps into infoAndBits. For segments
//     where FieldInfo numbers are dense 0-based integers matching the write
//     order, the output is byte-identical to Lucene's.
//
//   - The FieldsIndexWriter does not use CreateTempOutput (not part of
//     Gocene's Directory interface). Doc-count and start-pointer streams are
//     buffered in memory via ByteBuffersDataOutput and flushed into the .fdx
//     at finish time.
//
//   - Merge (bulk chunk copy) is not implemented; the writer always flushes
//     document-by-document. Bulk merge is an optimisation and does not affect
//     correctness.
package compressing

import (
	"errors"
	"fmt"
	"math"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// ---------------------------------------------------------------------------
// Wire-format constants – must match the Java reference exactly.
// ---------------------------------------------------------------------------

const (
	// fieldsExtension is the extension for the .fdt data file.
	fieldsExtension = "fdt"

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
var negativezeroFloat32 = math.Float32bits(-0.0)

// negativezeroFloat64 is the bit pattern for -0d, used in ZDouble encoding.
var negativezeroFloat64 = math.Float64bits(-0.0)

// ---------------------------------------------------------------------------
// Lucene90CompressingStoredFieldsFormat – the StoredFieldsFormat
// ---------------------------------------------------------------------------

// Lucene90CompressingStoredFieldsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsFormat.
//
// The on-disk chunk layout (LZ4/Deflate compressed sub-blocks with a
// monotonic per-block index) is fully implemented. The type is
// configurable so callers such as Lucene90StoredFieldsFormat can
// supply the BEST_SPEED / BEST_COMPRESSION preset parameters.
type Lucene90CompressingStoredFieldsFormat struct {
	formatName      string
	compressionMode compressing.CompressionMode
	chunkSize       int
	maxDocsPerChunk int
	blockShift      int
}

// NewLucene90CompressingStoredFieldsFormat builds a
// Lucene90CompressingStoredFieldsFormat with zero-valued tuning
// parameters. The constructor is preserved for backwards compatibility
// with the original Sprint 48 stub; prefer
// NewLucene90CompressingStoredFieldsFormatWithOptions when the preset
// values are known.
func NewLucene90CompressingStoredFieldsFormat() *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{}
}

// NewLucene90CompressingStoredFieldsFormatWithOptions builds a
// Lucene90CompressingStoredFieldsFormat configured for a specific mode.
//
// It is the Go counterpart of the 5-arg Java constructor
// Lucene90CompressingStoredFieldsFormat(String, CompressionMode, int,
// int, int).
func NewLucene90CompressingStoredFieldsFormatWithOptions(
	formatName string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{
		formatName:      formatName,
		compressionMode: compressionMode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		blockShift:      blockShift,
	}
}

// FormatName returns the on-disk format tag (one of
// "Lucene90StoredFieldsFastData" / "Lucene90StoredFieldsHighData").
func (f *Lucene90CompressingStoredFieldsFormat) FormatName() string { return f.formatName }

// CompressionMode returns the configured CompressionMode singleton.
func (f *Lucene90CompressingStoredFieldsFormat) CompressionMode() compressing.CompressionMode {
	return f.compressionMode
}

// ChunkSize returns the target uncompressed chunk size in bytes.
func (f *Lucene90CompressingStoredFieldsFormat) ChunkSize() int { return f.chunkSize }

// MaxDocsPerChunk returns the cap on documents per stored-fields chunk.
func (f *Lucene90CompressingStoredFieldsFormat) MaxDocsPerChunk() int { return f.maxDocsPerChunk }

// BlockShift returns the per-fields-index block shift (one block per
// 1 << blockShift chunks).
func (f *Lucene90CompressingStoredFieldsFormat) BlockShift() int { return f.blockShift }

// Name implements [gcodecs.StoredFieldsFormat].
func (f *Lucene90CompressingStoredFieldsFormat) Name() string {
	return "Lucene90CompressingStoredFieldsFormat"
}

// FieldsReader opens the stored-fields reader for the given segment.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsReader(
	dir store.Directory,
	si *index.SegmentInfo,
	fn *index.FieldInfos,
	ctx store.IOContext,
) (gcodecs.StoredFieldsReader, error) {
	if f.formatName == "" {
		return nil, fmt.Errorf(
			"lucene90/compressing: FieldsReader called on zero-value format; use NewLucene90CompressingStoredFieldsFormatWithOptions",
		)
	}
	return newLucene90CompressingStoredFieldsReader(dir, si, fn, ctx, f.formatName, f.compressionMode)
}

// FieldsWriter opens the stored-fields writer for the given segment.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsWriter(
	dir store.Directory,
	si *index.SegmentInfo,
	ctx store.IOContext,
) (gcodecs.StoredFieldsWriter, error) {
	if f.formatName == "" {
		return nil, fmt.Errorf(
			"lucene90/compressing: FieldsWriter called on zero-value format; use NewLucene90CompressingStoredFieldsFormatWithOptions",
		)
	}
	return newLucene90CompressingStoredFieldsWriter(
		dir, si, ctx, f.formatName, f.compressionMode, f.chunkSize, f.maxDocsPerChunk, f.blockShift,
	)
}

// Compile-time guarantee.
var _ gcodecs.StoredFieldsFormat = (*Lucene90CompressingStoredFieldsFormat)(nil)

// ---------------------------------------------------------------------------
// in-memory fields index builder (replaces Lucene's FieldsIndexWriter)
// ---------------------------------------------------------------------------

// fieldsIndexBuilder accumulates per-chunk (numDocs, startPointer) pairs in
// memory. At finish() time it emits the .fdm metadata and .fdx index data
// using DirectMonotonicWriter — byte-identical to Lucene's FieldsIndexWriter
// output, without requiring CreateTempOutput.
type fieldsIndexBuilder struct {
	dir        store.Directory
	name       string // segment name
	suffix     string // segment suffix (usually empty)
	codecName  string // "Lucene90FieldsIndex"
	id         []byte // segment ID (16 bytes)
	blockShift int
	ctx        store.IOContext

	// accumulated per-chunk data
	chunkDocCounts   []int32 // number of docs in each chunk
	chunkStartPtrs   []int64 // fieldsStream file pointer at chunk start
	totalDocs        int32
	totalChunks      int
	previousStartPtr int64
}

func newFieldsIndexBuilder(
	dir store.Directory,
	name, suffix, codecName string,
	id []byte,
	blockShift int,
	ctx store.IOContext,
) *fieldsIndexBuilder {
	return &fieldsIndexBuilder{
		dir:        dir,
		name:       name,
		suffix:     suffix,
		codecName:  codecName,
		id:         id,
		blockShift: blockShift,
		ctx:        ctx,
	}
}

// writeIndex records the chunk just written to fieldsStream.
func (b *fieldsIndexBuilder) writeIndex(numDocs int, startPointer int64) {
	b.chunkDocCounts = append(b.chunkDocCounts, int32(numDocs))
	b.chunkStartPtrs = append(b.chunkStartPtrs, startPointer)
	b.totalDocs += int32(numDocs)
	b.totalChunks++
	b.previousStartPtr = startPointer
}

// finish writes the .fdm and .fdx files. It mirrors FieldsIndexWriter.finish()
// in the Java reference but reads doc counts and file pointers from the
// in-memory slices instead of temp files.
func (b *fieldsIndexBuilder) finish(numDocs int, maxPointer int64, metaOut store.IndexOutput) error {
	if int(b.totalDocs) != numDocs {
		return fmt.Errorf(
			"lucene90/compressing: fieldsIndexBuilder: expected %d docs, accumulated %d",
			numDocs, b.totalDocs,
		)
	}

	// Open the .fdx data output, wrapped in a checksum tracker so that
	// WriteFooter can record the CRC32 (mirrors the Java FSDirectory
	// behaviour that wraps every output stream automatically).
	fdxName := index.SegmentFileName(b.name, b.suffix, indexExtension)
	var fdxOut store.IndexOutput
	{
		raw, err := b.dir.CreateOutput(fdxName, b.ctx)
		if err != nil {
			return fmt.Errorf("lucene90/compressing: create %s: %w", fdxName, err)
		}
		fdxOut = store.NewChecksumIndexOutput(raw)
	}
	var fdxCloseErr error
	defer func() {
		if fdxCloseErr == nil {
			fdxCloseErr = fdxOut.Close()
		} else {
			_ = fdxOut.Close()
		}
	}()

	if err := gcodecs.WriteIndexHeader(fdxOut, b.codecName+"Idx", 0, b.id, b.suffix); err != nil {
		return fmt.Errorf("lucene90/compressing: write fdx header: %w", err)
	}

	// The monotonic sequences have totalChunks+1 entries: the (n+1)-th entry
	// for docs is the total doc count; for start pointers it is maxPointer.
	n := int64(b.totalChunks + 1)

	// --- docs monotonic sequence ---
	// Emit to metaOut (position info) and fdxOut (packed data).
	metaOut.WriteInt(int32(numDocs)) //nolint:errcheck // errors checked below via footer
	metaOut.WriteInt(int32(b.blockShift))
	metaOut.WriteInt(int32(b.totalChunks + 1))
	metaOut.WriteLong(fdxOut.GetFilePointer())

	docsWriter, err := packed.NewDirectMonotonicWriter(metaOut, fdxOut, n, b.blockShift)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: create docs monotonic writer: %w", err)
	}
	var cumDocs int64
	if err := docsWriter.Add(cumDocs); err != nil {
		return err
	}
	for _, cnt := range b.chunkDocCounts {
		cumDocs += int64(cnt)
		if err := docsWriter.Add(cumDocs); err != nil {
			return err
		}
	}
	if err := docsWriter.Finish(); err != nil {
		return err
	}

	// --- startPointers monotonic sequence ---
	metaOut.WriteLong(fdxOut.GetFilePointer())

	ptrsWriter, err := packed.NewDirectMonotonicWriter(metaOut, fdxOut, n, b.blockShift)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: create ptrs monotonic writer: %w", err)
	}
	var cumPtr int64
	for _, sp := range b.chunkStartPtrs {
		if err := ptrsWriter.Add(sp); err != nil {
			return err
		}
		cumPtr = sp
	}
	// Sentinel: maxPointer after the last chunk.
	_ = cumPtr
	if err := ptrsWriter.Add(maxPointer); err != nil {
		return err
	}
	if err := ptrsWriter.Finish(); err != nil {
		return err
	}

	metaOut.WriteLong(fdxOut.GetFilePointer())
	metaOut.WriteLong(maxPointer)

	// Write footer on .fdx.
	if err := gcodecs.WriteFooter(fdxOut); err != nil {
		return fmt.Errorf("lucene90/compressing: write fdx footer: %w", err)
	}
	fdxCloseErr = fdxOut.Close()
	return fdxCloseErr
}

// ---------------------------------------------------------------------------
// Lucene90CompressingStoredFieldsWriter
// ---------------------------------------------------------------------------

// Lucene90CompressingStoredFieldsWriter is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsWriter.
type Lucene90CompressingStoredFieldsWriter struct {
	segment         string
	si              *index.SegmentInfo
	indexBuilder    *fieldsIndexBuilder
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
	metaName := index.SegmentFileName(segment, suffix, metaExtension)
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
		metaVersionStart,
		si.GetID(),
		suffix,
	); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: write fdm header: %w", err)
	}

	// Allocate fields stream (.fdt), also checksum-wrapped.
	fdtName := index.SegmentFileName(segment, suffix, fieldsExtension)
	{
		raw, err := dir.CreateOutput(fdtName, ctx)
		if err != nil {
			return nil, fmt.Errorf("lucene90/compressing: create %s: %w", fdtName, err)
		}
		fieldsStream = store.NewChecksumIndexOutput(raw)
	}
	_ = err

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
	if err := store.WriteVInt(metaStream, int32(chunkSize)); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: write chunkSize: %w", err)
	}

	indexBuilder := newFieldsIndexBuilder(dir, segment, suffix, indexCodecName, si.GetID(), blockShift, ctx)

	success = true
	return &Lucene90CompressingStoredFieldsWriter{
		segment:         segment,
		si:              si,
		indexBuilder:    indexBuilder,
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

// WriteField serializes one stored field from an IndexableField.
//
// Type dispatch uses StoredValue() when the field implements storedValueField
// (e.g. document.StoredField), which gives the exact type tag without
// ambiguity. Fields that do not implement the interface fall back to
// NumericValue() then StringValue().
//
// The wire encoding mirrors Lucene's per-type writeField methods, using a
// sequential 0-based field ID in place of the FieldInfo.number that Lucene
// uses (see package divergence note).
func (w *Lucene90CompressingStoredFieldsWriter) WriteField(field document.IndexableField) error {
	w.numStoredFieldsInDoc++

	// Assign a sequential field ID for this document. In Lucene the field
	// number comes from FieldInfo; here we use (numStoredFieldsInDoc - 1)
	// as a monotonically increasing identifier within the document.
	fieldSeq := int64(w.numStoredFieldsInDoc - 1)

	// Prefer StoredValue() for exact type tagging — this avoids the
	// ambiguity where document.stringValue.Binary() returns []byte(string)
	// and would be mis-classified as a binary field.
	if svf, ok := field.(storedValueField); ok {
		if sv := svf.StoredValue(); sv != nil {
			return w.writeStoredValue(fieldSeq, sv)
		}
	}

	// Fallback for fields that do not implement storedValueField: use
	// NumericValue() first, then StringValue(). BinaryValue() is NOT
	// checked here because document.stringValue.Binary() is non-nil for
	// any non-empty string, which would cause mis-classification.
	if nv := field.NumericValue(); nv != nil {
		return w.writeNumericValue(fieldSeq, nv)
	}
	// String field (sv may be "" for an empty stored string).
	infoAndBits := (fieldSeq << typeBits) | typeString
	if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
		return err
	}
	return w.bufferedDocs.WriteString(field.StringValue())
}

// writeStoredValue encodes a StoredValue into the buffered document stream.
func (w *Lucene90CompressingStoredFieldsWriter) writeStoredValue(fieldSeq int64, sv *document.StoredValue) error {
	switch sv.GetType() {
	case document.StoredValueTypeBinary:
		bv := sv.GetBinaryValue()
		infoAndBits := (fieldSeq << typeBits) | typeByteArray
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		if err := w.bufferedDocs.WriteVInt(int32(len(bv))); err != nil {
			return err
		}
		return w.bufferedDocs.WriteBytes(bv)
	case document.StoredValueTypeString:
		infoAndBits := (fieldSeq << typeBits) | typeString
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return w.bufferedDocs.WriteString(sv.GetStringValue())
	case document.StoredValueTypeInteger:
		infoAndBits := (fieldSeq << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, sv.GetIntValue())
	case document.StoredValueTypeLong:
		infoAndBits := (fieldSeq << typeBits) | typeNumericLong
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeTLong(w.bufferedDocs, sv.GetLongValue())
	case document.StoredValueTypeFloat:
		infoAndBits := (fieldSeq << typeBits) | typeNumericFloat
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZFloat(w.bufferedDocs, sv.GetFloatValue())
	case document.StoredValueTypeDouble:
		infoAndBits := (fieldSeq << typeBits) | typeNumericDouble
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZDouble(w.bufferedDocs, sv.GetDoubleValue())
	default:
		return fmt.Errorf("lucene90/compressing: unsupported StoredValue type %v", sv.GetType())
	}
}

// writeNumericValue is the fallback path for fields that do not implement
// storedValueField but do return a NumericValue().
func (w *Lucene90CompressingStoredFieldsWriter) writeNumericValue(fieldSeq int64, nv interface{}) error {
	switch v := nv.(type) {
	case int:
		infoAndBits := (fieldSeq << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, int32(v))
	case int32:
		infoAndBits := (fieldSeq << typeBits) | typeNumericInt
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZInt(w.bufferedDocs, v)
	case int64:
		infoAndBits := (fieldSeq << typeBits) | typeNumericLong
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeTLong(w.bufferedDocs, v)
	case float32:
		infoAndBits := (fieldSeq << typeBits) | typeNumericFloat
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZFloat(w.bufferedDocs, v)
	case float64:
		infoAndBits := (fieldSeq << typeBits) | typeNumericDouble
		if err := w.bufferedDocs.WriteVLong(infoAndBits); err != nil {
			return err
		}
		return writeZDouble(w.bufferedDocs, v)
	default:
		// Unknown numeric type — serialize as string.
		infoAndBits := (fieldSeq << typeBits) | typeString
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
	w.indexBuilder.writeIndex(w.numBufferedDocs, w.fieldsStream.GetFilePointer())

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
	if err := store.WriteVInt(w.fieldsStream, int32(docBase)); err != nil {
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
	if err := store.WriteVInt(w.fieldsStream, code); err != nil {
		return err
	}
	if err := saveInts(numStoredFields, numBufferedDocs, w.fieldsStream); err != nil {
		return err
	}
	return saveInts(lengths, numBufferedDocs, w.fieldsStream)
}

// saveInts mirrors the Java reference's private saveInts(int[], int, DataOutput):
// for length==1 it writes a single VInt; for length>1 it delegates to
// StoredFieldsInts.writeInts (called WriteStoredFieldsInts in Gocene).
func saveInts(values []int32, length int, out store.DataOutput) error {
	if length == 1 {
		return store.WriteVInt(out, values[0])
	}
	return gcodecs.WriteStoredFieldsInts(values, 0, length, out)
}

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
	return gcodecs.ReadStoredFieldsInts(in, count, values, offset)
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

	// Let the index builder write .fdx and contribute to .fdm.
	if err := w.indexBuilder.finish(numDocs, maxPointer, w.metaStream); err != nil {
		return err
	}

	// Write dirty-chunk stats and footer to .fdm.
	if err := store.WriteVLong(w.metaStream, w.numChunks); err != nil {
		return err
	}
	if err := store.WriteVLong(w.metaStream, w.numDirtyChunks); err != nil {
		return err
	}
	if err := store.WriteVLong(w.metaStream, w.numDirtyDocs); err != nil {
		return err
	}
	if err := gcodecs.WriteFooter(w.metaStream); err != nil {
		return err
	}

	// Write footer to .fdt.
	return gcodecs.WriteFooter(w.fieldsStream)
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
	indexReader  *luceneFieldsIndexReader
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
	fdtName := index.SegmentFileName(segment, suffix, fieldsExtension)
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

	_, err = gcodecs.CheckIndexHeader(
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
	fdmName := index.SegmentFileName(segment, suffix, metaExtension)
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
		metaIn, indexCodecName+"Meta", metaVersionStart, metaVersionStart, si.GetID(), suffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: check fdm header: %w", err)
	}

	chunkSize, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read chunkSize: %w", err)
	}

	// Read fields index from .fdm + .fdx.
	indexReader, err := newLuceneFieldsIndexReader(dir, si.Name(), suffix, indexExtension, indexCodecName, si.GetID(), metaIn, ctx)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read fields index: %w", err)
	}

	numChunks, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numChunks: %w", err)
	}
	numDirtyChunks, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numDirtyChunks: %w", err)
	}
	numDirtyDocs, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: read numDirtyDocs: %w", err)
	}

	if _, err := gcodecs.CheckFooter(metaIn); err != nil {
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
		maxPointer:      indexReader.maxPointer,
	}, nil
}

// VisitDocument decodes the stored fields for docID and dispatches each
// field to visitor.
func (r *Lucene90CompressingStoredFieldsReader) VisitDocument(docID int, visitor gcodecs.StoredFieldVisitor) error {
	if r.closed {
		return errors.New("lucene90/compressing: reader is closed")
	}

	// Find the chunk that contains docID.
	startPointer, err := r.indexReader.getStartPointer(docID)
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
		infoAndBits, err := store.ReadVLong(docData)
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
			if err := docData.ReadBytes(b); err != nil {
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
	if err := r.indexReader.close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Compile-time guarantee.
var _ gcodecs.StoredFieldsReader = (*Lucene90CompressingStoredFieldsReader)(nil)

// ---------------------------------------------------------------------------
// luceneFieldsIndexReader – reads .fdm + .fdx monotonic arrays
// ---------------------------------------------------------------------------

// luceneFieldsIndexReader reads the fields index produced by
// fieldsIndexBuilder.finish(). It holds the DirectMonotonicReaders for
// doc-IDs and start-pointers, and answers getStartPointer(docID) queries.
type luceneFieldsIndexReader struct {
	maxDoc        int
	blockShift    int
	numChunks     int
	docsMeta      *packed.DirectMonotonicMeta
	ptrsMeta      *packed.DirectMonotonicMeta
	indexInput    store.IndexInput
	docsStart     int64
	docsEnd       int64
	ptrsStart     int64
	ptrsEnd       int64
	maxPointer    int64
	docs          *packed.DirectMonotonicReader
	startPointers *packed.DirectMonotonicReader
}

func newLuceneFieldsIndexReader(
	dir store.Directory,
	name, suffix, extension, codecName string,
	id []byte,
	metaIn store.IndexInput,
	ctx store.IOContext,
) (*luceneFieldsIndexReader, error) {
	maxDoc, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	blockShift, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	numChunks, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	docsStart, err := metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	docsMeta, err := packed.LoadDirectMonotonicMeta(metaIn, int64(numChunks), int(blockShift))
	if err != nil {
		return nil, err
	}
	docsEnd, err := metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	ptrsStart := docsEnd
	ptrsMeta, err := packed.LoadDirectMonotonicMeta(metaIn, int64(numChunks), int(blockShift))
	if err != nil {
		return nil, err
	}
	ptrsEnd, err := metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	maxPointer, err := metaIn.ReadLong()
	if err != nil {
		return nil, err
	}

	// Open .fdx.
	fdxName := index.SegmentFileName(name, suffix, extension)
	indexInput, err := dir.OpenInput(fdxName, ctx)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: open %s: %w", fdxName, err)
	}
	success := false
	defer func() {
		if !success {
			_ = indexInput.Close()
		}
	}()

	_, err = gcodecs.CheckIndexHeader(indexInput, codecName+"Idx", 0, 0, id, suffix)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: check fdx header: %w", err)
	}
	if _, err := gcodecs.RetrieveChecksum(indexInput); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: retrieve fdx checksum: %w", err)
	}

	// Build DirectMonotonicReaders for docs and start pointers.
	// indexInput.Slice returns a store.IndexInput which may implement
	// packed.RandomAccessInput. We type-assert first; if the concrete
	// implementation does not satisfy the interface (e.g. the file-backed
	// slice does not expose random-access reads) we fall back to reading the
	// entire slice into a byte slice and wrapping it in ByteArrayRandomAccess.
	docsRA, err := sliceToRandomAccess(indexInput, "docs", docsStart, docsEnd-docsStart)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: slice docs: %w", err)
	}
	docs, err := packed.NewDirectMonotonicReader(docsMeta, docsRA)
	if err != nil {
		return nil, err
	}

	ptrsRA, err := sliceToRandomAccess(indexInput, "ptrs", ptrsStart, ptrsEnd-ptrsStart)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: slice ptrs: %w", err)
	}
	startPointers, err := packed.NewDirectMonotonicReader(ptrsMeta, ptrsRA)
	if err != nil {
		return nil, err
	}

	success = true
	return &luceneFieldsIndexReader{
		maxDoc:        int(maxDoc),
		blockShift:    int(blockShift),
		numChunks:     int(numChunks) - 1, // numChunks stored is totalChunks+1
		docsMeta:      docsMeta,
		ptrsMeta:      ptrsMeta,
		indexInput:    indexInput,
		docsStart:     docsStart,
		docsEnd:       docsEnd,
		ptrsStart:     ptrsStart,
		ptrsEnd:       ptrsEnd,
		maxPointer:    maxPointer,
		docs:          docs,
		startPointers: startPointers,
	}, nil
}

// getStartPointer returns the start file pointer of the chunk containing docID.
func (r *luceneFieldsIndexReader) getStartPointer(docID int) (int64, error) {
	if docID < 0 || docID >= r.maxDoc {
		return 0, fmt.Errorf("lucene90/compressing: docID %d out of range [0, %d)", docID, r.maxDoc)
	}
	// Binary search in docs monotonic array for the chunk index.
	blockIdx, err := r.docs.BinarySearch(0, int64(r.numChunks), int64(docID))
	if err != nil {
		return 0, err
	}
	if blockIdx < 0 {
		blockIdx = -2 - blockIdx
	}
	return r.startPointers.Get(blockIdx), nil
}

func (r *luceneFieldsIndexReader) close() error {
	return r.indexInput.Close()
}

// ---------------------------------------------------------------------------
// sliceToRandomAccess helper
// ---------------------------------------------------------------------------

// sliceToRandomAccess slices indexInput at [offset, offset+length) and
// returns the result as a packed.RandomAccessInput. It first attempts a
// type assertion (most concrete IndexInput implementations satisfy
// store.RandomAccessInput, which has the same methods); if that fails it
// reads all bytes into memory and wraps them in a ByteArrayRandomAccessInput.
func sliceToRandomAccess(in store.IndexInput, desc string, offset, length int64) (packed.RandomAccessInput, error) {
	if length == 0 {
		return store.NewByteArrayRandomAccessInput(nil), nil
	}
	sub, err := in.Slice(desc, offset, length)
	if err != nil {
		return nil, err
	}
	if ra, ok := sub.(packed.RandomAccessInput); ok {
		return ra, nil
	}
	// Fall back: read everything into memory.
	buf := make([]byte, length)
	if err := sub.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("sliceToRandomAccess: read %d bytes at %d: %w", length, offset, err)
	}
	return store.NewByteArrayRandomAccessInput(buf), nil
}

// ---------------------------------------------------------------------------
// Numeric encodings (mirrors Java static helpers)
// ---------------------------------------------------------------------------

// writeZInt writes a 32-bit integer using zigzag+VInt encoding.
func writeZInt(out store.DataOutput, v int32) error {
	return store.WriteVInt(out, int32((v<<1)^(v>>31)))
}

// readZInt reads a zigzag+VInt encoded int32.
func readZInt(in store.DataInput) (int32, error) {
	v, err := store.ReadVInt(in)
	if err != nil {
		return 0, err
	}
	return int32(uint32(v)>>1) ^ -(v & 1), nil
}

// writeZFloat mirrors Lucene's Lucene90CompressingStoredFieldsWriter.writeZFloat.
//
// All multi-byte writes are done byte-by-byte in big-endian order to match
// Lucene's DataOutput contract and be independent of the underlying
// DataOutput implementation's endianness (ByteBuffersDataOutput is BE;
// ByteArrayDataOutput is LE — using explicit bytes avoids the mismatch).
func writeZFloat(out store.DataOutput, f float32) error {
	intVal := int32(f)
	floatBits := math.Float32bits(f)

	if f == float32(intVal) && intVal >= -1 && intVal <= 0x7D && floatBits != negativezeroFloat32 {
		// Small integer [-1..125]: single byte.
		return out.WriteByte(byte(0x80 | (1 + intVal)))
	} else if (floatBits >> 31) == 0 {
		// Positive float: 4 bytes big-endian.
		if err := out.WriteByte(byte(floatBits >> 24)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(floatBits >> 16)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(floatBits >> 8)); err != nil {
			return err
		}
		return out.WriteByte(byte(floatBits))
	}
	// Negative float: 0xFF header + 4 bytes big-endian.
	if err := out.WriteByte(0xFF); err != nil {
		return err
	}
	if err := out.WriteByte(byte(floatBits >> 24)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(floatBits >> 16)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(floatBits >> 8)); err != nil {
		return err
	}
	return out.WriteByte(byte(floatBits))
}

// readZFloat mirrors Lucene's Lucene90CompressingStoredFieldsReader.readZFloat.
//
// All multi-byte reads use ReadByte to be endian-independent.
func readZFloat(in store.DataInput) (float32, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	if b == 0xFF {
		// Negative float: 4 bytes big-endian.
		b1, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b2, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b3, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b4, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		bits := uint32(b1)<<24 | uint32(b2)<<16 | uint32(b3)<<8 | uint32(b4)
		return math.Float32frombits(bits), nil
	} else if (b & 0x80) != 0 {
		// Small integer.
		return float32(int32(b&0x7F) - 1), nil
	}
	// Positive float: b is top byte, then 3 more bytes.
	b2, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	bits := uint32(b)<<24 | uint32(b2)<<16 | uint32(b3)<<8 | uint32(b4)
	return math.Float32frombits(bits), nil
}

// writeZDouble mirrors Lucene's Lucene90CompressingStoredFieldsWriter.writeZDouble.
//
// All multi-byte writes are done byte-by-byte in big-endian order (see
// writeZFloat for the endianness rationale).
func writeZDouble(out store.DataOutput, d float64) error {
	intVal := int64(d)
	doubleBits := math.Float64bits(d)

	if d == float64(intVal) && intVal >= -1 && intVal <= 0x7C && doubleBits != negativezeroFloat64 {
		// Small integer [-1..124]: single byte.
		return out.WriteByte(byte(0x80 | (intVal + 1)))
	} else if d == float64(float32(d)) {
		// Double has accurate float32 representation: 0xFE + 4 bytes.
		fb := math.Float32bits(float32(d))
		if err := out.WriteByte(0xFE); err != nil {
			return err
		}
		if err := out.WriteByte(byte(fb >> 24)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(fb >> 16)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(fb >> 8)); err != nil {
			return err
		}
		return out.WriteByte(byte(fb))
	} else if (doubleBits >> 63) == 0 {
		// Positive double: 7 bytes (byte + 4 bytes + 2 bytes + 1 byte = 8 total).
		if err := out.WriteByte(byte(doubleBits >> 56)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 48)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 40)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 32)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 24)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 16)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(doubleBits >> 8)); err != nil {
			return err
		}
		return out.WriteByte(byte(doubleBits))
	}
	// Negative double: 0xFF header + 8 bytes big-endian.
	if err := out.WriteByte(0xFF); err != nil {
		return err
	}
	for shift := 56; shift >= 0; shift -= 8 {
		if err := out.WriteByte(byte(doubleBits >> shift)); err != nil {
			return err
		}
	}
	return nil
}

// readZDouble mirrors Lucene's Lucene90CompressingStoredFieldsReader.readZDouble.
//
// All multi-byte reads use ReadByte to be endian-independent.
func readZDouble(in store.DataInput) (float64, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	if b == 0xFF {
		// Negative double: 8 bytes big-endian.
		var bits uint64
		for i := 0; i < 8; i++ {
			bx, err := in.ReadByte()
			if err != nil {
				return 0, err
			}
			bits = bits<<8 | uint64(bx)
		}
		return math.Float64frombits(bits), nil
	} else if b == 0xFE {
		// Float32 representation: 4 bytes big-endian.
		b1, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b2, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b3, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		b4, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		bits := uint32(b1)<<24 | uint32(b2)<<16 | uint32(b3)<<8 | uint32(b4)
		return float64(math.Float32frombits(bits)), nil
	} else if (b & 0x80) != 0 {
		// Small integer.
		return float64(int64(b&0x7F) - 1), nil
	}
	// Positive double: b is the top byte, followed by 7 more bytes big-endian.
	var bits uint64 = uint64(b)
	for i := 0; i < 7; i++ {
		bx, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		bits = bits<<8 | uint64(bx)
	}
	return math.Float64frombits(bits), nil
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
		return store.WriteVLong(out, int64(upperBits))
	}
	return nil
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
		upper, err := store.ReadVLong(in)
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

// FieldsIndexWriter is the stub type kept for backwards compatibility.
// The real index writer is the unexported fieldsIndexBuilder above.
type FieldsIndexWriter struct{}

// NewFieldsIndexWriter builds a FieldsIndexWriter stub.
func NewFieldsIndexWriter() *FieldsIndexWriter { return &FieldsIndexWriter{} }

// Lucene90CompressingStoredFieldsReader is the concrete reader type.
// (Declared above — this stub alias is no longer needed but kept for
// any external reference that may exist.)

// Lucene90CompressingStoredFieldsWriter is the concrete writer type.
// (Declared above.)

// Lucene90CompressingTermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90CompressingTermVectorsFormat struct{}

// NewLucene90CompressingTermVectorsFormat builds a
// Lucene90CompressingTermVectorsFormat.
func NewLucene90CompressingTermVectorsFormat() *Lucene90CompressingTermVectorsFormat {
	return &Lucene90CompressingTermVectorsFormat{}
}

// Lucene90CompressingTermVectorsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader.
type Lucene90CompressingTermVectorsReader struct{}

// NewLucene90CompressingTermVectorsReader builds a
// Lucene90CompressingTermVectorsReader.
func NewLucene90CompressingTermVectorsReader() *Lucene90CompressingTermVectorsReader {
	return &Lucene90CompressingTermVectorsReader{}
}

// Lucene90CompressingTermVectorsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter.
type Lucene90CompressingTermVectorsWriter struct{}

// NewLucene90CompressingTermVectorsWriter builds a
// Lucene90CompressingTermVectorsWriter.
func NewLucene90CompressingTermVectorsWriter() *Lucene90CompressingTermVectorsWriter {
	return &Lucene90CompressingTermVectorsWriter{}
}
