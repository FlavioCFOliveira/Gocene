// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
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

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene99FlatVectorsFormat wire-level constants. Mirror the static
// definitions in
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsFormat
// (Apache Lucene 10.5.0). The flat format owns the raw per-document vectors
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
	// Lucene99FlatVectorsFormat.DIRECT_MONOTONIC_BLOCK_SHIFT — the
	// block-shift used by the DirectMonotonicWriter that records the
	// sparse ord->doc mapping.
	lucene99FlatDirectMonotonicBlockShift = 16

	// floatBytes is the wire width of a FLOAT32 sample (Float.BYTES).
	floatBytes = 4

	// flatFloatAlignment is the .vec alignment used for FLOAT32 fields.
	// Lucene aligns float vectors to 64 bytes for Arm Neoverse machines.
	flatFloatAlignment = 64
)

// Lucene99FlatVectorsWriter writes vector values to index segments. It is
// the Go port of org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsWriter
// (Apache Lucene 10.5.0).
//
// Wire-format parity (.vec + .vemf, see the Lucene99FlatVectorsFormat
// Javadoc for the full layout):
//
//   - `.vec` carries each field's vectors ordered by document ordinal and
//     dimension. FLOAT32 samples are written little-endian; BYTE samples
//     are written verbatim. Each field's block is preceded by an alignment
//     pad (64 bytes for FLOAT32, 4 for BYTE). For sparse fields the per-doc
//     vectors are followed by the IndexedDISI doc-id set and the
//     DirectMonotonicWriter ord->doc data, appended to `.vec`.
//   - `.vemf` carries one record per field: field number, encoding ordinal,
//     similarity ordinal, .vec offset/length, dimension, count, then the
//     OrdToDoc/DocsWithField block (see [writeFlatOrdToDocStoredMeta]).
//     Terminated by an int32 sentinel -1, then the codec footer.
//
// Concurrency: not safe for concurrent use. Mirrors the Java reference.
type Lucene99FlatVectorsWriter struct {
	*hnsw.BaseFlatVectorsWriter

	segmentWriteState *SegmentWriteState
	meta              store.IndexOutput
	vectorData        store.IndexOutput

	// fieldWriterFactory mirrors the IOFunction<FieldInfo,
	// FlatFieldVectorsWriter<?>> strategy factory. The wildcard
	// FlatFieldVectorsWriter<?> is rendered as the non-generic
	// KnnFieldVectorsWriter; the writer asserts the typed
	// hnsw.FlatFieldVectorsWriter by the field's vector encoding.
	fieldWriterFactory func(fieldInfo *index.FieldInfo) (KnnFieldVectorsWriter, error)

	fields   []lucene99FlatFieldData
	finished bool
}

// lucene99FlatFieldData mirrors the private record
// FieldData(FlatFieldVectorsWriter<?> fieldWriter, FieldInfo fieldInfo).
type lucene99FlatFieldData struct {
	fieldWriter KnnFieldVectorsWriter
	fieldInfo   *index.FieldInfo
}

// lucene99FlatVectorsWriterShallowRamBytesUsed mirrors SHALLOW_RAM_BYTES_USED.
var lucene99FlatVectorsWriterShallowRamBytesUsed = util.ShallowSizeOf(Lucene99FlatVectorsWriter{})

// NewLucene99FlatVectorsWriter constructs a writer that uses the default
// factory to build per-field vector storage: per-field writers that store
// vector data as a list of on-heap slices, one per vector. Mirrors
// Lucene99FlatVectorsWriter(SegmentWriteState, FlatVectorsScorer).
func NewLucene99FlatVectorsWriter(state *SegmentWriteState, scorer hnsw.FlatVectorsScorer) (*Lucene99FlatVectorsWriter, error) {
	return NewLucene99FlatVectorsWriterWithStrategy(state, scorer, newLucene99FlatDefaultFieldWriter)
}

// NewLucene99FlatVectorsWriterWithStrategy constructs a writer that uses
// strategyFactory to build per-field vector storage. The factory is
// consulted on every AddField call; merges write directly to the new segment
// through MergeOneFlatVectorField and do not go through the strategy.
// Mirrors Lucene99FlatVectorsWriter(SegmentWriteState, FlatVectorsScorer,
// IOFunction<FieldInfo, FlatFieldVectorsWriter<?>>).
func NewLucene99FlatVectorsWriterWithStrategy(
	state *SegmentWriteState,
	scorer hnsw.FlatVectorsScorer,
	strategyFactory func(fieldInfo *index.FieldInfo) (KnnFieldVectorsWriter, error),
) (*Lucene99FlatVectorsWriter, error) {
	w := &Lucene99FlatVectorsWriter{
		BaseFlatVectorsWriter: hnsw.NewBaseFlatVectorsWriter(scorer),
		segmentWriteState:     state,
		fieldWriterFactory:    strategyFactory,
	}
	metaFileName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatMetaExtension)
	vectorDataFileName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatDataExtension)

	if err := w.openOutputs(state, metaFileName, vectorDataFileName); err != nil {
		// Java: IOUtils.closeWhileHandlingException(this).
		w.closeWhileHandlingException()
		return nil, err
	}
	return w, nil
}

// openOutputs creates the meta and vector data outputs and writes their
// codec index headers, the body of the Java constructor's try block.
func (w *Lucene99FlatVectorsWriter) openOutputs(state *SegmentWriteState, metaFileName, vectorDataFileName string) error {
	rawMeta, err := state.Directory.CreateOutput(metaFileName, state.Context)
	if err != nil {
		return err
	}
	w.meta = store.NewChecksumIndexOutput(rawMeta)
	rawVectorData, err := state.Directory.CreateOutput(vectorDataFileName, state.Context)
	if err != nil {
		return err
	}
	w.vectorData = store.NewChecksumIndexOutput(rawVectorData)

	id := state.SegmentInfo.GetID()
	if err := WriteIndexHeader(
		w.meta, lucene99FlatMetaCodecName, lucene99FlatVersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		return err
	}
	return WriteIndexHeader(
		w.vectorData, lucene99FlatDataCodecName, lucene99FlatVersionCurrent, id, state.SegmentSuffix,
	)
}

// AddField builds the per-field writer through the strategy factory and
// records it. Mirrors addField(FieldInfo).
func (w *Lucene99FlatVectorsWriter) AddField(fieldInfo *index.FieldInfo) (KnnFieldVectorsWriter, error) {
	newFieldWriter, err := w.fieldWriterFactory(fieldInfo)
	if err != nil {
		return nil, err
	}
	w.fields = append(w.fields, lucene99FlatFieldData{fieldWriter: newFieldWriter, fieldInfo: fieldInfo})
	return newFieldWriter, nil
}

// Flush writes every field, sorted through sortMap when it is non-nil, and
// finishes each field writer. Mirrors flush(int, Sorter.DocMap).
func (w *Lucene99FlatVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	for _, field := range w.fields {
		if sortMap == nil {
			if err := w.writeField(field.fieldWriter, field.fieldInfo, maxDoc); err != nil {
				return err
			}
		} else {
			if err := w.writeSortingField(field.fieldWriter, field.fieldInfo, maxDoc, sortMap); err != nil {
				return err
			}
		}
		if err := field.fieldWriter.Finish(); err != nil {
			return err
		}
	}
	return nil
}

// Finish writes the end-of-fields marker and the codec footers. Mirrors
// finish(), which throws IllegalStateException when called twice.
func (w *Lucene99FlatVectorsWriter) Finish() error {
	if w.finished {
		return errors.New("already finished")
	}
	w.finished = true
	if w.meta != nil {
		// write end of fields marker
		if err := w.meta.WriteInt(-1); err != nil {
			return err
		}
		if err := WriteFooter(w.meta); err != nil {
			return err
		}
	}
	if w.vectorData != nil {
		if err := WriteFooter(w.vectorData); err != nil {
			return err
		}
	}
	return nil
}

// RamBytesUsed mirrors ramBytesUsed(): the shallow size of the writer plus
// the footprint of every field writer.
func (w *Lucene99FlatVectorsWriter) RamBytesUsed() int64 {
	total := lucene99FlatVectorsWriterShallowRamBytesUsed
	for _, field := range w.fields {
		total += field.fieldWriter.RamBytesUsed()
	}
	return total
}

// alignLucene99FlatOutput mirrors the private static alignOutput(IndexOutput,
// VectorEncoding): BYTE vectors are aligned to Float.BYTES and FLOAT32
// vectors to 64 bytes, the optimal alignment for Arm Neoverse machines.
func alignLucene99FlatOutput(output store.IndexOutput, encoding index.VectorEncoding) (int64, error) {
	switch encoding {
	case index.VectorEncodingByte:
		return store.AlignFilePointer(output, floatBytes)
	case index.VectorEncodingFloat32:
		return store.AlignFilePointer(output, flatFloatAlignment)
	}
	return 0, fmt.Errorf("unknown vector encoding: %v", encoding)
}

// asFlatFieldVectorsWriter renders the Java casts of the wildcard
// FlatFieldVectorsWriter<?> to FlatFieldVectorsWriter<float[]> or
// FlatFieldVectorsWriter<byte[]>. A failed cast, a ClassCastException in
// Java, is returned as an error.
func asFlatFieldVectorsWriter[T float32 | byte](fieldWriter KnnFieldVectorsWriter) (hnsw.FlatFieldVectorsWriter[T], error) {
	typed, ok := fieldWriter.(hnsw.FlatFieldVectorsWriter[T])
	if !ok {
		return nil, fmt.Errorf("ClassCastException: %T cannot be cast to FlatFieldVectorsWriter", fieldWriter)
	}
	return typed, nil
}

// writeField writes one field's vectors and its metadata record. Mirrors
// the private writeField(FlatFieldVectorsWriter<?>, FieldInfo, int).
func (w *Lucene99FlatVectorsWriter) writeField(fieldWriter KnnFieldVectorsWriter, fieldInfo *index.FieldInfo, maxDoc int) error {
	// write vector values
	encoding := fieldInfo.VectorEncoding()
	vectorDataOffset, err := alignLucene99FlatOutput(w.vectorData, encoding)
	if err != nil {
		return err
	}
	var docsWithField *index.DocsWithFieldSet
	switch encoding {
	case index.VectorEncodingByte:
		byteWriter, err := asFlatFieldVectorsWriter[byte](fieldWriter)
		if err != nil {
			return err
		}
		if err := w.writeByteVectors(byteWriter); err != nil {
			return err
		}
		docsWithField = byteWriter.GetDocsWithFieldSet()
	case index.VectorEncodingFloat32:
		floatWriter, err := asFlatFieldVectorsWriter[float32](fieldWriter)
		if err != nil {
			return err
		}
		if err := w.writeFloat32Vectors(floatWriter, fieldInfo); err != nil {
			return err
		}
		docsWithField = floatWriter.GetDocsWithFieldSet()
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset

	return w.writeMeta(fieldInfo, maxDoc, vectorDataOffset, vectorDataLength, docsWithField)
}

// putFloat32sLE renders ByteBuffer.order(LITTLE_ENDIAN).asFloatBuffer().put:
// the values are written little-endian from the start of buffer.
func putFloat32sLE(buffer []byte, values []float32) {
	for i, v := range values {
		binary.LittleEndian.PutUint32(buffer[i*floatBytes:], math.Float32bits(v))
	}
}

// writeFloat32Vectors mirrors the private writeFloat32Vectors.
func (w *Lucene99FlatVectorsWriter) writeFloat32Vectors(fieldWriter hnsw.FlatFieldVectorsWriter[float32], fieldInfo *index.FieldInfo) error {
	buffer := make([]byte, fieldInfo.VectorDimension()*floatBytes)
	for _, v := range fieldWriter.GetVectors() {
		putFloat32sLE(buffer, v)
		if err := w.vectorData.WriteBytes(buffer, 0, len(buffer)); err != nil {
			return err
		}
	}
	return nil
}

// writeByteVectors mirrors the private writeByteVectors.
func (w *Lucene99FlatVectorsWriter) writeByteVectors(fieldWriter hnsw.FlatFieldVectorsWriter[byte]) error {
	for _, vector := range fieldWriter.GetVectors() {
		if err := w.vectorData.WriteBytes(vector, 0, len(vector)); err != nil {
			return err
		}
	}
	return nil
}

// writeSortingField writes one field's vectors in the order of sortMap.
// Mirrors the private writeSortingField(FlatFieldVectorsWriter<?>,
// FieldInfo, int, Sorter.DocMap).
func (w *Lucene99FlatVectorsWriter) writeSortingField(
	fieldWriter KnnFieldVectorsWriter, fieldInfo *index.FieldInfo, maxDoc int, sortMap spi.SorterDocMap,
) error {
	encoding := fieldInfo.VectorEncoding()
	var (
		floatWriter hnsw.FlatFieldVectorsWriter[float32]
		byteWriter  hnsw.FlatFieldVectorsWriter[byte]
		err         error
	)
	var docsWithFieldSet *index.DocsWithFieldSet
	switch encoding {
	case index.VectorEncodingByte:
		if byteWriter, err = asFlatFieldVectorsWriter[byte](fieldWriter); err != nil {
			return err
		}
		docsWithFieldSet = byteWriter.GetDocsWithFieldSet()
	case index.VectorEncodingFloat32:
		if floatWriter, err = asFlatFieldVectorsWriter[float32](fieldWriter); err != nil {
			return err
		}
		docsWithFieldSet = floatWriter.GetDocsWithFieldSet()
	default:
		return fmt.Errorf("unknown vector encoding: %v", encoding)
	}
	ordMap := make([]int, docsWithFieldSet.Cardinality()) // new ord to old ord

	newDocsWithField := index.NewDocsWithFieldSet()
	if err := MapOldOrdToNewOrd(docsWithFieldSet, sortMap, nil, ordMap, newDocsWithField); err != nil {
		return err
	}

	// write vector values
	vectorDataOffset, err := alignLucene99FlatOutput(w.vectorData, encoding)
	if err != nil {
		return err
	}
	switch encoding {
	case index.VectorEncodingByte:
		err = w.writeSortedByteVectors(byteWriter, ordMap)
	case index.VectorEncodingFloat32:
		err = w.writeSortedFloat32Vectors(floatWriter, fieldInfo, ordMap)
	}
	if err != nil {
		return err
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset

	return w.writeMeta(fieldInfo, maxDoc, vectorDataOffset, vectorDataLength, newDocsWithField)
}

// writeSortedFloat32Vectors mirrors the private writeSortedFloat32Vectors.
func (w *Lucene99FlatVectorsWriter) writeSortedFloat32Vectors(
	fieldWriter hnsw.FlatFieldVectorsWriter[float32], fieldInfo *index.FieldInfo, ordMap []int,
) error {
	buffer := make([]byte, fieldInfo.VectorDimension()*floatBytes)
	for _, ordinal := range ordMap {
		vector := fieldWriter.GetVectors()[ordinal]
		putFloat32sLE(buffer, vector)
		if err := w.vectorData.WriteBytes(buffer, 0, len(buffer)); err != nil {
			return err
		}
	}
	return nil
}

// writeSortedByteVectors mirrors the private writeSortedByteVectors.
func (w *Lucene99FlatVectorsWriter) writeSortedByteVectors(fieldWriter hnsw.FlatFieldVectorsWriter[byte], ordMap []int) error {
	for _, ordinal := range ordMap {
		vector := fieldWriter.GetVectors()[ordinal]
		if err := w.vectorData.WriteBytes(vector, 0, len(vector)); err != nil {
			return err
		}
	}
	return nil
}

// MergeOneFlatVectorField writes the merged vectors of fieldInfo directly to
// the new segment. Mirrors mergeOneFlatVectorField(FieldInfo, MergeState):
// since no additional indexing will search them, no temporary file is used.
func (w *Lucene99FlatVectorsWriter) MergeOneFlatVectorField(fieldInfo *index.FieldInfo, mergeState *index.MergeState) error {
	encoding := fieldInfo.VectorEncoding()
	vectorDataOffset, err := alignLucene99FlatOutput(w.vectorData, encoding)
	if err != nil {
		return err
	}
	var docsWithField *index.DocsWithFieldSet
	switch encoding {
	case index.VectorEncodingByte:
		merged, err := MergeByteVectorValues(fieldInfo, mergeState)
		if err != nil {
			return err
		}
		if docsWithField, err = writeLucene99FlatByteVectorData(w.vectorData, merged); err != nil {
			return err
		}
	case index.VectorEncodingFloat32:
		merged, err := MergeFloatVectorValues(fieldInfo, mergeState)
		if err != nil {
			return err
		}
		if docsWithField, err = writeLucene99FlatVectorData(w.vectorData, merged); err != nil {
			return err
		}
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset
	return w.writeMeta(
		fieldInfo,
		w.segmentWriteState.SegmentInfo.MaxDoc(),
		vectorDataOffset,
		vectorDataLength,
		docsWithField)
}

// writeMeta writes one field record on the meta file, mirroring the private
// writeMeta. Layout:
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
	field *index.FieldInfo, maxDoc int, vectorDataOffset, vectorDataLength int64, docsWithField *index.DocsWithFieldSet,
) error {
	simOrd, err := distFuncToOrd(field.VectorSimilarityFunction())
	if err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(vectorEncodingOrdinal(field.VectorEncoding())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(simOrd); err != nil {
		return err
	}
	if err := w.meta.WriteVLong(vectorDataOffset); err != nil {
		return err
	}
	if err := w.meta.WriteVLong(vectorDataLength); err != nil {
		return err
	}
	if err := w.meta.WriteVInt(int32(field.VectorDimension())); err != nil {
		return err
	}

	// write docIDs
	count := docsWithField.Cardinality()
	if err := w.meta.WriteInt(int32(count)); err != nil {
		return err
	}
	return writeFlatOrdToDocStoredMeta(
		lucene99FlatDirectMonotonicBlockShift, w.meta, w.vectorData, count, maxDoc, docsWithField)
}

// writeLucene99FlatByteVectorData writes the byte vector values to the output
// and returns a set of documents that contains vectors. Mirrors the private
// static writeByteVectorData(IndexOutput, ByteVectorValues).
func writeLucene99FlatByteVectorData(output store.IndexOutput, byteVectorValues index.ByteVectorValues) (*index.DocsWithFieldSet, error) {
	docsWithField := index.NewDocsWithFieldSet()
	iter := byteVectorValues.Iterator()
	for {
		docV, err := iter.NextDoc()
		if err != nil {
			return nil, err
		}
		if docV == util.NO_MORE_DOCS {
			break
		}
		// write vector
		binaryValue, err := byteVectorValues.VectorValue(iter.Index())
		if err != nil {
			return nil, err
		}
		if err := output.WriteBytes(binaryValue, 0, len(binaryValue)); err != nil {
			return nil, err
		}
		if err := docsWithField.Add(docV); err != nil {
			return nil, err
		}
	}
	return docsWithField, nil
}

// writeLucene99FlatVectorData writes the vector values to the output and
// returns a set of documents that contains vectors. Mirrors the private
// static writeVectorData(IndexOutput, FloatVectorValues).
func writeLucene99FlatVectorData(output store.IndexOutput, floatVectorValues index.FloatVectorValues) (*index.DocsWithFieldSet, error) {
	docsWithField := index.NewDocsWithFieldSet()
	buffer := make([]byte, floatVectorValues.Dimension()*index.VectorEncodingByteSize(index.VectorEncodingFloat32))
	iter := floatVectorValues.Iterator()
	for {
		docV, err := iter.NextDoc()
		if err != nil {
			return nil, err
		}
		if docV == util.NO_MORE_DOCS {
			break
		}
		// write vector
		value, err := floatVectorValues.VectorValue(iter.Index())
		if err != nil {
			return nil, err
		}
		putFloat32sLE(buffer, value)
		if err := output.WriteBytes(buffer, 0, len(buffer)); err != nil {
			return nil, err
		}
		if err := docsWithField.Add(docV); err != nil {
			return nil, err
		}
	}
	return docsWithField, nil
}

// WriteField is declared by [spi.KnnVectorsWriter], which has no counterpart
// in Apache Lucene 10.5.0: the Java writer merges through
// MergeOneFlatVectorField. The reader-driven entry point is unsupported.
func (w *Lucene99FlatVectorsWriter) WriteField(fieldInfo *index.FieldInfo, _ KnnVectorsReader) error {
	return fmt.Errorf("lucene99 flat: WriteField is not part of Lucene99FlatVectorsWriter (field=%q)", fieldInfo.Name())
}

// Close closes the meta and vector data outputs. Mirrors close(), which is
// IOUtils.close(meta, vectorData): every output is closed and the first
// failure is reported.
func (w *Lucene99FlatVectorsWriter) Close() error {
	var firstErr error
	for _, output := range []store.IndexOutput{w.meta, w.vectorData} {
		if output != nil {
			if err := output.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// closeWhileHandlingException mirrors IOUtils.closeWhileHandlingException
// applied to this writer by the constructor's failure path: every opened
// output is closed and close failures are suppressed.
func (w *Lucene99FlatVectorsWriter) closeWhileHandlingException() {
	for _, output := range []store.IndexOutput{w.meta, w.vectorData} {
		if output != nil {
			util.CloseAllWhileHandlingException(output)
		}
	}
}

// lucene99FlatDefaultFieldWriter is the Go port of the private static
// DefaultFieldWriter<T>: the default FlatFieldVectorsWriter, which stores
// vectors on-heap in a list, copying each value through CopyValue on
// AddValue. The anonymous BYTE and FLOAT32 subclasses created by
// DefaultFieldWriter.create are the byte and float32 instantiations.
type lucene99FlatDefaultFieldWriter[T float32 | byte] struct {
	fieldInfo     *index.FieldInfo
	docsWithField *index.DocsWithFieldSet
	vectors       [][]T
	finished      bool

	lastDocID int
	dim       int
}

// lucene99FlatDefaultFieldWriterShallowRamBytesUsed mirrors
// DefaultFieldWriter.SHALLOW_RAM_BYTES_USED.
var lucene99FlatDefaultFieldWriterShallowRamBytesUsed = util.ShallowSizeOf(lucene99FlatDefaultFieldWriter[float32]{})

// newLucene99FlatDefaultFieldWriter mirrors DefaultFieldWriter.create(FieldInfo).
func newLucene99FlatDefaultFieldWriter(fieldInfo *index.FieldInfo) (KnnFieldVectorsWriter, error) {
	switch fieldInfo.VectorEncoding() {
	case index.VectorEncodingByte:
		return newLucene99FlatDefaultFieldWriterOf[byte](fieldInfo), nil
	case index.VectorEncodingFloat32:
		return newLucene99FlatDefaultFieldWriterOf[float32](fieldInfo), nil
	}
	return nil, fmt.Errorf("unknown vector encoding: %v", fieldInfo.VectorEncoding())
}

// newLucene99FlatDefaultFieldWriterOf mirrors the DefaultFieldWriter(FieldInfo)
// constructor together with the dimension captured by create.
func newLucene99FlatDefaultFieldWriterOf[T float32 | byte](fieldInfo *index.FieldInfo) *lucene99FlatDefaultFieldWriter[T] {
	return &lucene99FlatDefaultFieldWriter[T]{
		fieldInfo:     fieldInfo,
		docsWithField: index.NewDocsWithFieldSet(),
		lastDocID:     -1,
		dim:           fieldInfo.VectorDimension(),
	}
}

// AddValue mirrors DefaultFieldWriter.addValue(int, T).
func (fw *lucene99FlatDefaultFieldWriter[T]) AddValue(docID int, vectorValue any) error {
	if fw.finished {
		return errors.New("already finished, cannot add more values")
	}
	if docID == fw.lastDocID {
		return fmt.Errorf("VectorValuesField \"%s\" appears more than once in this document (only one value is allowed per field)",
			fw.fieldInfo.Name())
	}
	value, ok := vectorValue.([]T)
	if !ok {
		return fmt.Errorf("ClassCastException: %T cannot be cast to the vector type of field \"%s\"", vectorValue, fw.fieldInfo.Name())
	}
	copied := fw.CopyValue(value)
	if err := fw.docsWithField.Add(docID); err != nil {
		return err
	}
	fw.vectors = append(fw.vectors, copied)
	fw.lastDocID = docID
	return nil
}

// CopyValue mirrors the anonymous copyValue overrides of
// DefaultFieldWriter.create: ArrayUtil.copyOfSubArray(value, 0, dim).
func (fw *lucene99FlatDefaultFieldWriter[T]) CopyValue(value []T) []T {
	return util.CopyOfSubArrayGeneric(value, 0, fw.dim)
}

// RamBytesUsed mirrors DefaultFieldWriter.ramBytesUsed().
func (fw *lucene99FlatDefaultFieldWriter[T]) RamBytesUsed() int64 {
	size := lucene99FlatDefaultFieldWriterShallowRamBytesUsed
	if len(fw.vectors) == 0 {
		return size
	}
	return size +
		fw.docsWithField.RamBytesUsed() +
		int64(len(fw.vectors))*int64(util.NumBytesObjectRef+util.NumBytesArrayHeader) +
		int64(len(fw.vectors))*
			int64(fw.fieldInfo.VectorDimension())*
			int64(index.VectorEncodingByteSize(fw.fieldInfo.VectorEncoding()))
}

// GetVectors mirrors DefaultFieldWriter.getVectors().
func (fw *lucene99FlatDefaultFieldWriter[T]) GetVectors() [][]T {
	return fw.vectors
}

// GetDocsWithFieldSet mirrors DefaultFieldWriter.getDocsWithFieldSet().
func (fw *lucene99FlatDefaultFieldWriter[T]) GetDocsWithFieldSet() *index.DocsWithFieldSet {
	return fw.docsWithField
}

// Finish mirrors DefaultFieldWriter.finish().
func (fw *lucene99FlatDefaultFieldWriter[T]) Finish() error {
	if fw.finished {
		return nil
	}
	fw.finished = true
	return nil
}

// IsFinished mirrors DefaultFieldWriter.isFinished().
func (fw *lucene99FlatDefaultFieldWriter[T]) IsFinished() bool {
	return fw.finished
}

// AsKnnVectorValues carries the inherited
// FlatFieldVectorsWriter.asKnnVectorValues(VectorEncoding, int).
func (fw *lucene99FlatDefaultFieldWriter[T]) AsKnnVectorValues(encoding index.VectorEncoding, dim int) (index.KnnVectorValues, error) {
	return hnsw.DefaultAsKnnVectorValues[T](fw, encoding, dim)
}

// writeFlatOrdToDocStoredMeta writes the docsWithField / ordToDoc block,
// mirroring
// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration.writeStoredMeta
// (Apache Lucene 10.5.0) verbatim. The codecs package cannot import
// codecs/lucene95 (that package imports codecs), so the IndexedDISI bit set
// is written by the package-local, byte-identical writeDVBitSet.
//
// For the empty case (count == 0) and the dense case (count == maxDoc) the
// block on meta is a fixed sentinel (long offset, long length=0, short
// jumpTableEntryCount=-1, byte denseRankPower=-1) and nothing is written to
// the data file. For the sparse case the data file receives the IndexedDISI
// bit set of the docIDs that carry a value and the DirectMonotonicWriter
// ord->doc data, and the meta block records their offsets and lengths.
func writeFlatOrdToDocStoredMeta(
	directMonotonicBlockShift int,
	outputMeta, vectorData store.IndexOutput,
	count, maxDoc int,
	docsWithField *index.DocsWithFieldSet,
) error {
	if count == 0 {
		if err := outputMeta.WriteLong(-2); err != nil { // docsWithFieldOffset
			return err
		}
		if err := outputMeta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := outputMeta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		return outputMeta.WriteByte(0xFF) // denseRankPower, (byte) -1
	} else if count == maxDoc {
		if err := outputMeta.WriteLong(-1); err != nil { // docsWithFieldOffset
			return err
		}
		if err := outputMeta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := outputMeta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		return outputMeta.WriteByte(0xFF) // denseRankPower, (byte) -1
	}

	offset := vectorData.GetFilePointer()
	if err := outputMeta.WriteLong(offset); err != nil { // docsWithFieldOffset
		return err
	}
	jumpTableEntryCount, err := writeDVBitSet(docsWithField.Iterator(), vectorData)
	if err != nil {
		return err
	}
	if err := outputMeta.WriteLong(vectorData.GetFilePointer() - offset); err != nil { // docsWithFieldLength
		return err
	}
	if err := outputMeta.WriteShort(jumpTableEntryCount); err != nil {
		return err
	}
	if err := outputMeta.WriteByte(dvDefaultDenseRankPower); err != nil {
		return err
	}

	// write ordToDoc mapping
	start := vectorData.GetFilePointer()
	if err := outputMeta.WriteLong(start); err != nil {
		return err
	}
	if err := outputMeta.WriteVInt(int32(directMonotonicBlockShift)); err != nil {
		return err
	}
	// dense case and empty case do not need to store ordToMap mapping
	ordToDocWriter, err := packed.NewDirectMonotonicWriter(
		newDMAdapter(outputMeta), newDMAdapter(vectorData),
		int64(count), directMonotonicBlockShift,
	)
	if err != nil {
		return err
	}
	iterator := docsWithField.Iterator()
	for {
		doc, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		if err := ordToDocWriter.Add(int64(doc)); err != nil {
			return err
		}
	}
	if err := ordToDocWriter.Finish(); err != nil {
		return err
	}
	return outputMeta.WriteLong(vectorData.GetFilePointer() - start)
}

// Compile-time guards.
var (
	_ hnsw.FlatVectorsWriter               = (*Lucene99FlatVectorsWriter)(nil)
	_ hnsw.FlatFieldVectorsWriter[float32] = (*lucene99FlatDefaultFieldWriter[float32])(nil)
	_ hnsw.FlatFieldVectorsWriter[byte]    = (*lucene99FlatDefaultFieldWriter[byte])(nil)
)
