// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene99FlatVectorsFormat wire-level constants. Mirror the static
// definitions in
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsFormat
// (Lucene 10.4.0). The flat format owns the raw per-document vectors
// (the `.vec` data file and the `.vemf` metadata file); the HNSW format
// composes a flat writer/reader to persist the vectors that back its
// graph.
const (
	// lucene99FlatMetaCodecName mirrors META_CODEC_NAME.
	lucene99FlatMetaCodecName = "Lucene99FlatVectorsFormatMeta"

	// lucene99FlatDataCodecName mirrors VECTOR_DATA_CODEC_NAME.
	lucene99FlatDataCodecName = "Lucene99FlatVectorsFormatData"

	// lucene99FlatMetaExtension mirrors META_EXTENSION.
	lucene99FlatMetaExtension = "vemf"

	// lucene99FlatDataExtension mirrors VECTOR_DATA_EXTENSION.
	lucene99FlatDataExtension = "vec"

	// lucene99FlatVersionStart mirrors VERSION_START.
	lucene99FlatVersionStart int32 = 0

	// lucene99FlatVersionCurrent mirrors VERSION_CURRENT.
	lucene99FlatVersionCurrent int32 = lucene99FlatVersionStart

	// lucene99FlatDirectMonotonicBlockShift mirrors
	// DIRECT_MONOTONIC_BLOCK_SHIFT (used only by the sparse OrdToDoc
	// mapping, which is deferred — see [errFlatSparseUnsupported]).
	lucene99FlatDirectMonotonicBlockShift = 16

	// floatBytes is the wire width of a FLOAT32 sample.
	floatBytes = 4

	// flatFloatAlignment is the .vec alignment used for FLOAT32 fields.
	// Lucene aligns float vectors to 64 bytes for Arm Neoverse machines.
	flatFloatAlignment = 64
)

// errFlatSparseUnsupported is returned when a flat vector field is sparse
// (some documents in the segment lack a value for the field). The dense
// case — every document has a vector — needs no IndexedDISI and is fully
// supported; the sparse case requires IndexedDISI.writeBitSet, whose port
// is tracked by rmp #4755.
var errFlatSparseUnsupported = errors.New(
	"lucene99 flat vectors: sparse vector fields (some docs lack a value) " +
		"are not supported yet; the dense case (every doc has a vector) is " +
		"supported. Sparse support requires the IndexedDISI port tracked by rmp #4755")

// Lucene99FlatVectorsWriter writes raw vector values to the `.vec` data
// file and per-field metadata to the `.vemf` file. It is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsWriter
// (Lucene 10.4.0), restricted to the dense and empty cases (rmp #4731).
//
// Wire-format parity (.vec + .vemf, see the Lucene99FlatVectorsFormat
// Javadoc for the full layout):
//
//   - `.vec` carries each field's vectors ordered by document ordinal and
//     dimension. FLOAT32 samples are written little-endian; BYTE samples
//     are written verbatim. Each field's block is preceded by an alignment
//     pad (64 bytes for FLOAT32, 4 for BYTE). Identical byte layout to
//     Lucene.
//   - `.vemf` carries one record per field: field number, encoding ordinal,
//     similarity ordinal, .vec offset/length, dimension, count, then the
//     OrdToDoc/DocsWithField sentinel block. Terminated by an int32
//     sentinel -1, then the codec footer. Identical byte layout to Lucene
//     for the dense and empty cases.
//
// Deviation from the Java reference (rmp #4731):
//
//  1. Only dense (count == maxDoc) and empty (count == 0) fields are
//     supported. A sparse field surfaces [errFlatSparseUnsupported] from
//     [Lucene99FlatVectorsWriter.Flush]; the sparse IndexedDISI path is
//     tracked by rmp #4755. The merge path (mergeOneField /
//     mergeOneFieldToIndex) and the index-sort path (writeSortingField)
//     are likewise out of scope for rmp #4731.
//
// Concurrency: not safe for concurrent use. Mirrors the Java reference.
type Lucene99FlatVectorsWriter struct {
	state *SegmentWriteState

	meta       store.IndexOutput
	vectorData store.IndexOutput

	fields []*lucene99FlatFieldWriter

	finished bool
	closed   bool
}

// lucene99FlatFieldWriter accumulates the per-document vectors for one
// field. It is the Go port of the private FieldWriter inner class in the
// Java reference, trimmed to the dense/empty surface rmp #4731 supports.
type lucene99FlatFieldWriter struct {
	fieldInfo *index.FieldInfo
	encoding  index.VectorEncoding
	dim       int

	// floats and bytes hold the per-document vectors. Exactly one is
	// populated, gated by encoding.
	floats [][]float32
	bytes  [][]byte

	// docIDs records the docID associated with each accumulated vector,
	// in insertion order. Used to detect the dense-vs-sparse case.
	docIDs []int
	lastID int

	finished bool
}

// NewLucene99FlatVectorsWriter constructs a flat vectors writer bound to
// state. It creates the `.vec` and `.vemf` segment files and writes their
// codec headers. Mirrors the Java constructor
// Lucene99FlatVectorsWriter(SegmentWriteState, FlatVectorsScorer); the
// scorer parameter is omitted because Gocene resolves the search-time
// scorer at read time (see [Lucene99FlatVectorsReader]).
func NewLucene99FlatVectorsWriter(state *SegmentWriteState) (*Lucene99FlatVectorsWriter, error) {
	if state == nil {
		return nil, errors.New("lucene99 flat: nil SegmentWriteState")
	}
	if state.SegmentInfo == nil {
		return nil, errors.New("lucene99 flat: nil SegmentInfo")
	}
	if state.Directory == nil {
		return nil, errors.New("lucene99 flat: nil Directory")
	}

	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatMetaExtension)
	dataName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatDataExtension)

	rawMeta, err := state.Directory.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: create meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexOutput(rawMeta)

	w := &Lucene99FlatVectorsWriter{state: state, meta: meta}

	rawData, err := state.Directory.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		return nil, fmt.Errorf("lucene99 flat: create data %q: %w", dataName, err)
	}
	w.vectorData = store.NewChecksumIndexOutput(rawData)

	id := state.SegmentInfo.GetID()
	if err := WriteIndexHeader(
		w.meta, lucene99FlatMetaCodecName, lucene99FlatVersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene99 flat: write meta header: %w", err)
	}
	if err := WriteIndexHeader(
		w.vectorData, lucene99FlatDataCodecName, lucene99FlatVersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene99 flat: write data header: %w", err)
	}
	return w, nil
}

// AddField allocates a per-field accumulator. Mirrors Java's
// addField(FieldInfo).
func (w *Lucene99FlatVectorsWriter) AddField(fieldInfo *index.FieldInfo) (*lucene99FlatFieldWriter, error) {
	if w.closed {
		return nil, errors.New("lucene99 flat: writer is closed")
	}
	if w.finished {
		return nil, errors.New("lucene99 flat: writer already finished")
	}
	if fieldInfo == nil {
		return nil, errors.New("lucene99 flat: AddField: nil FieldInfo")
	}
	if fieldInfo.VectorDimension() <= 0 {
		return nil, fmt.Errorf(
			"lucene99 flat: AddField: field %q has non-positive vector dimension %d",
			fieldInfo.Name(), fieldInfo.VectorDimension())
	}
	fw := &lucene99FlatFieldWriter{
		fieldInfo: fieldInfo,
		encoding:  fieldInfo.VectorEncoding(),
		dim:       fieldInfo.VectorDimension(),
		lastID:    -1,
	}
	w.fields = append(w.fields, fw)
	return fw, nil
}

// addValueFloat32 records a float32 vector for docID. Mirrors the
// FLOAT32 branch of FieldWriter.addValue.
func (fw *lucene99FlatFieldWriter) addValueFloat32(docID int, vector []float32) error {
	if fw.finished {
		return errors.New("lucene99 flat: field writer already finished")
	}
	if fw.encoding != index.VectorEncodingFloat32 {
		return fmt.Errorf("lucene99 flat: field %q encoding %v, float value added",
			fw.fieldInfo.Name(), fw.encoding)
	}
	if got := len(vector); got != fw.dim {
		return fmt.Errorf("lucene99 flat: field %q vector dim mismatch: got %d, want %d",
			fw.fieldInfo.Name(), got, fw.dim)
	}
	if docID == fw.lastID {
		return fmt.Errorf(
			"lucene99 flat: field %q appears more than once in document %d "+
				"(only one value is allowed per field)", fw.fieldInfo.Name(), docID)
	}
	if docID < fw.lastID {
		return fmt.Errorf("lucene99 flat: field %q docID %d < previous %d",
			fw.fieldInfo.Name(), docID, fw.lastID)
	}
	cp := make([]float32, len(vector))
	copy(cp, vector)
	fw.floats = append(fw.floats, cp)
	fw.docIDs = append(fw.docIDs, docID)
	fw.lastID = docID
	return nil
}

// addValueByte records a byte vector for docID. Mirrors the BYTE branch
// of FieldWriter.addValue.
func (fw *lucene99FlatFieldWriter) addValueByte(docID int, vector []byte) error {
	if fw.finished {
		return errors.New("lucene99 flat: field writer already finished")
	}
	if fw.encoding != index.VectorEncodingByte {
		return fmt.Errorf("lucene99 flat: field %q encoding %v, byte value added",
			fw.fieldInfo.Name(), fw.encoding)
	}
	if got := len(vector); got != fw.dim {
		return fmt.Errorf("lucene99 flat: field %q vector dim mismatch: got %d, want %d",
			fw.fieldInfo.Name(), got, fw.dim)
	}
	if docID == fw.lastID {
		return fmt.Errorf(
			"lucene99 flat: field %q appears more than once in document %d "+
				"(only one value is allowed per field)", fw.fieldInfo.Name(), docID)
	}
	if docID < fw.lastID {
		return fmt.Errorf("lucene99 flat: field %q docID %d < previous %d",
			fw.fieldInfo.Name(), docID, fw.lastID)
	}
	cp := make([]byte, len(vector))
	copy(cp, vector)
	fw.bytes = append(fw.bytes, cp)
	fw.docIDs = append(fw.docIDs, docID)
	fw.lastID = docID
	return nil
}

// numDocs returns the number of vectors accumulated for the field.
func (fw *lucene99FlatFieldWriter) numDocs() int {
	if fw.encoding == index.VectorEncodingByte {
		return len(fw.bytes)
	}
	return len(fw.floats)
}

// ramBytesUsed estimates the in-memory footprint of the accumulated
// vectors for the field.
func (fw *lucene99FlatFieldWriter) ramBytesUsed() int64 {
	const docIDBytes = 8
	switch fw.encoding {
	case index.VectorEncodingFloat32:
		return int64(len(fw.docIDs))*docIDBytes + int64(len(fw.floats))*int64(fw.dim)*floatBytes
	case index.VectorEncodingByte:
		return int64(len(fw.docIDs))*docIDBytes + int64(len(fw.bytes))*int64(fw.dim)
	default:
		return 0
	}
}

// Flush serialises every field accumulated so far, mirroring Java's
// flush(int maxDoc, Sorter.DocMap sortMap). sortMap support
// (writeSortingField) is out of scope for rmp #4731; a non-nil sortMap
// returns an error rather than silently ignoring the requested ordering.
func (w *Lucene99FlatVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	if w.closed {
		return errors.New("lucene99 flat: writer is closed")
	}
	if w.finished {
		return errors.New("lucene99 flat: writer already finished")
	}
	if sortMap != nil {
		return errors.New("lucene99 flat: index-sort (sortMap) not supported yet (rmp #4731 scope)")
	}
	for _, fw := range w.fields {
		if err := w.writeField(fw, maxDoc); err != nil {
			return err
		}
		fw.finished = true
	}
	return nil
}

// writeField writes one field's vectors to `.vec` and its record to
// `.vemf`. Mirrors Java's private writeField(FieldWriter, int maxDoc).
func (w *Lucene99FlatVectorsWriter) writeField(fw *lucene99FlatFieldWriter, maxDoc int) error {
	vectorDataOffset, err := w.alignData(fw.encoding)
	if err != nil {
		return err
	}
	switch fw.encoding {
	case index.VectorEncodingFloat32:
		if err := w.writeFloat32Vectors(fw); err != nil {
			return err
		}
	case index.VectorEncodingByte:
		if err := w.writeByteVectors(fw); err != nil {
			return err
		}
	default:
		return fmt.Errorf("lucene99 flat: field %q unsupported encoding %v",
			fw.fieldInfo.Name(), fw.encoding)
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset
	return w.writeMeta(fw, maxDoc, vectorDataOffset, vectorDataLength)
}

// alignData advances the .vec file pointer to the encoding-specific
// alignment boundary and returns the aligned offset. Mirrors Java's
// alignOutput(IndexOutput, VectorEncoding).
func (w *Lucene99FlatVectorsWriter) alignData(encoding index.VectorEncoding) (int64, error) {
	alignment := floatBytes
	if encoding == index.VectorEncodingFloat32 {
		alignment = flatFloatAlignment
	}
	return store.AlignFilePointer(w.vectorData, alignment)
}

// writeFloat32Vectors writes each float32 vector little-endian. Mirrors
// Java's writeFloat32Vectors.
func (w *Lucene99FlatVectorsWriter) writeFloat32Vectors(fw *lucene99FlatFieldWriter) error {
	buf := make([]byte, fw.dim*floatBytes)
	for _, v := range fw.floats {
		for i, f := range v {
			binary.LittleEndian.PutUint32(buf[i*floatBytes:], math.Float32bits(f))
		}
		if err := w.vectorData.WriteBytes(buf); err != nil {
			return err
		}
	}
	return nil
}

// writeByteVectors writes each byte vector verbatim. Mirrors Java's
// writeByteVectors.
func (w *Lucene99FlatVectorsWriter) writeByteVectors(fw *lucene99FlatFieldWriter) error {
	for _, v := range fw.bytes {
		if err := w.vectorData.WriteBytes(v); err != nil {
			return err
		}
	}
	return nil
}

// writeMeta writes one field record on the meta file, mirroring Java's
// private writeMeta. Layout (matches the Lucene99FlatVectorsFormat
// Javadoc verbatim):
//
//	int32  field number
//	int32  vector encoding ordinal
//	int32  similarity ordinal
//	vlong  .vec offset
//	vlong  .vec length
//	vint   dimension
//	int32  count (docs with a value)
//	then OrdToDocDISIReaderConfiguration.writeStoredMeta (see
//	[writeFlatOrdToDocStoredMeta]).
func (w *Lucene99FlatVectorsWriter) writeMeta(
	fw *lucene99FlatFieldWriter, maxDoc int, vectorDataOffset, vectorDataLength int64,
) error {
	simOrd, err := distFuncToOrd(fw.fieldInfo.VectorSimilarityFunction())
	if err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(fw.fieldInfo.Number())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(vectorEncodingOrdinal(fw.encoding)); err != nil {
		return err
	}
	if err := w.meta.WriteInt(simOrd); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorDataOffset); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorDataLength); err != nil {
		return err
	}
	if err := store.WriteVInt(w.meta, int32(fw.dim)); err != nil {
		return err
	}
	count := fw.numDocs()
	if err := w.meta.WriteInt(int32(count)); err != nil {
		return err
	}
	return writeFlatOrdToDocStoredMeta(w.meta, count, maxDoc, fw.docIDs)
}

// writeFlatOrdToDocStoredMeta writes the docsWithField / ordToDoc sentinel
// block, mirroring
// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration.writeStoredMeta.
//
// For the empty case (count == 0) and the dense case (count == maxDoc) the
// block is a fixed sentinel (long offset, long length=0, short
// jumpTableEntryCount=-1, byte denseRankPower=-1) and nothing is written to
// the data file:
//
//	count == 0      -> docsWithFieldOffset = -2 (empty)
//	count == maxDoc -> docsWithFieldOffset = -1 (dense)
//
// The sparse case (0 < count < maxDoc) requires IndexedDISI.writeBitSet and
// a DirectMonotonicWriter ordToDoc mapping, which are out of scope for rmp
// #4731 (tracked by rmp #4755); it returns [errFlatSparseUnsupported].
func writeFlatOrdToDocStoredMeta(meta store.IndexOutput, count, maxDoc int, _ []int) error {
	switch {
	case count == 0:
		// Empty: docsWithFieldOffset = -2.
		if err := meta.WriteLong(-2); err != nil {
			return err
		}
	case count == maxDoc:
		// Dense: docsWithFieldOffset = -1.
		if err := meta.WriteLong(-1); err != nil {
			return err
		}
	default:
		// Sparse: requires IndexedDISI.writeBitSet (rmp #4755).
		return fmt.Errorf("%w (count=%d, maxDoc=%d)", errFlatSparseUnsupported, count, maxDoc)
	}
	if err := meta.WriteLong(0); err != nil { // docsWithFieldLength
		return err
	}
	if err := meta.WriteShort(-1); err != nil { // jumpTableEntryCount
		return err
	}
	return meta.WriteByte(0xFF) // denseRankPower == (byte) -1
}

// Finish writes the end-of-fields sentinel (-1) and the codec footer on
// both segment files. Mirrors Java's finish().
func (w *Lucene99FlatVectorsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene99 flat: writer is closed")
	}
	if w.finished {
		return errors.New("lucene99 flat: already finished")
	}
	w.finished = true
	if w.meta != nil {
		if err := w.meta.WriteInt(-1); err != nil {
			return fmt.Errorf("lucene99 flat: write meta sentinel: %w", err)
		}
		if err := WriteFooter(w.meta); err != nil {
			return fmt.Errorf("lucene99 flat: write meta footer: %w", err)
		}
	}
	if w.vectorData != nil {
		if err := WriteFooter(w.vectorData); err != nil {
			return fmt.Errorf("lucene99 flat: write data footer: %w", err)
		}
	}
	return nil
}

// RamBytesUsed sums the in-memory footprint of every per-field buffer.
func (w *Lucene99FlatVectorsWriter) RamBytesUsed() int64 {
	var total int64
	for _, fw := range w.fields {
		total += fw.ramBytesUsed()
	}
	return total
}

// Close releases the segment outputs. Close is idempotent.
func (w *Lucene99FlatVectorsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	var firstErr error
	if w.meta != nil {
		if err := w.meta.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.meta = nil
	}
	if w.vectorData != nil {
		if err := w.vectorData.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.vectorData = nil
	}
	return firstErr
}
