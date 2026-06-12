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
//
// Source: lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene90/
//         Lucene90HnswVectorsWriter.java (Lucene 10.4.0)
//
// Test-only backward-compat writer that produces .vec / .vem / .vex files in the
// Lucene 9.0 HNSW vector format (v0). The production
// Lucene90HnswVectorsFormat#fieldsWriter in Lucene 10.4.0 throws
// UnsupportedOperationException; this writer exists solely so Gocene can emit
// fixture segments that Java Lucene 10.4.0's backward-codecs reader can verify
// with CheckIndex.
//
// Wire-format parity:
//
//   - .vec carries the little-endian float32 vector data, aligned to a float
//     boundary per field. Opens with a CodecUtil index header and closes with a
//     CodecUtil footer.
//   - .vex carries the flat (single-level) HNSW graph. For each node:
//     int32 neighbor count, then delta-encoded vint neighbor ordinals
//     (initial delta subtracts -1). Opens with a CodecUtil index header and
//     closes with a CodecUtil footer.
//   - .vem carries one record per field: int32 field number, int32 similarity
//     ordinal, vlong .vec offset, vlong .vec length, vlong .vex offset,
//     vlong .vex length, int32 dimension, int32 count, then count * vint docID,
//     then count * delta-vlong graph offset. Terminated by int32 sentinel -1
//     and the CodecUtil footer.
//
// Deviations from the Java reference:
//
//  1. The merge paths (WriteField) and index-sort path (sortMap in Flush) are
//     out of scope; they return explicit errors.
//  2. Vectors are buffered in memory and the graph is built from the in-memory
//     slice directly using util/hnsw.HnswGraphBuilder, without the temporary
//     file indirection the Java writer uses. The on-disk result is byte-for-byte
//     identical because the vector data and graph layout match the format spec.
//  3. Only FLOAT32 fields are supported; BYTE fields return an explicit error,
//     matching the Java UnsupportedOperationException.

package lucene90

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// Wire-level constants for Lucene90 HNSW v0 format.
const (
	lucene90HnswMetaCodecName  = "Lucene90HnswVectorsFormatMeta"
	lucene90HnswDataCodecName  = "Lucene90HnswVectorsFormatData"
	lucene90HnswIndexCodecName = "Lucene90HnswVectorsFormatIndex"
	lucene90HnswMetaExtension  = "vem"
	lucene90HnswDataExtension  = "vec"
	lucene90HnswIndexExtension = "vex"
	lucene90HnswVersionStart   int32 = 0
	lucene90HnswVersionCurrent int32 = 0
	lucene90HnswDefaultMaxConn       = 16
	lucene90HnswDefaultBeamWidth     = 100
)

// lucene90HnswSimilarityOrdinals fixes the on-disk ordinal ->
// VectorSimilarityFunction mapping. The order is part of the wire format and
// must not change without a version bump.
var lucene90HnswSimilarityOrdinals = []index.VectorSimilarityFunction{
	index.VectorSimilarityFunctionEuclidean,
	index.VectorSimilarityFunctionDotProduct,
	index.VectorSimilarityFunctionCosine,
	index.VectorSimilarityFunctionMaximumInnerProduct,
}

func similarityToOrd(f index.VectorSimilarityFunction) (int32, error) {
	for i, v := range lucene90HnswSimilarityOrdinals {
		if v == f {
			return int32(i), nil
		}
	}
	return 0, fmt.Errorf("lucene90 hnsw: invalid similarity function: %v", f)
}

// Lucene90HnswVectorsWriter writes vector values and HNSW graphs in the
// Lucene 9.0 backward-compatible format (.vec / .vem / .vex).
// It is the Go port of
// org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsWriter.
type Lucene90HnswVectorsWriter struct {
	state       *codecs.SegmentWriteState
	maxConn     int
	beamWidth   int
	meta        store.IndexOutput
	vectorData  store.IndexOutput
	vectorIndex store.IndexOutput
	fields      []*lucene90HnswFieldWriter
	finished    bool
	closed      bool
}

// NewLucene90HnswVectorsWriter creates a new writer bound to state.
func NewLucene90HnswVectorsWriter(state *codecs.SegmentWriteState, maxConn, beamWidth int) (*Lucene90HnswVectorsWriter, error) {
	if state == nil {
		return nil, errors.New("lucene90 hnsw: nil SegmentWriteState")
	}
	if state.SegmentInfo == nil {
		return nil, errors.New("lucene90 hnsw: nil SegmentInfo")
	}
	if state.Directory == nil {
		return nil, errors.New("lucene90 hnsw: nil Directory")
	}
	if maxConn <= 0 {
		maxConn = lucene90HnswDefaultMaxConn
	}
	if beamWidth <= 0 {
		beamWidth = lucene90HnswDefaultBeamWidth
	}

	metaName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene90HnswMetaExtension)
	dataName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene90HnswDataExtension)
	indexName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene90HnswIndexExtension)

	rawMeta, err := state.Directory.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("lucene90 hnsw: create meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexOutput(rawMeta)

	w := &Lucene90HnswVectorsWriter{
		state:     state,
		maxConn:   maxConn,
		beamWidth: beamWidth,
		meta:      meta,
	}

	rawData, err := state.Directory.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		return nil, fmt.Errorf("lucene90 hnsw: create data %q: %w", dataName, err)
	}
	w.vectorData = store.NewChecksumIndexOutput(rawData)

	rawIndex, err := state.Directory.CreateOutput(indexName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		_ = rawData.Close()
		return nil, fmt.Errorf("lucene90 hnsw: create index %q: %w", indexName, err)
	}
	w.vectorIndex = store.NewChecksumIndexOutput(rawIndex)

	id := state.SegmentInfo.GetID()
	if err := codecs.WriteIndexHeader(meta, lucene90HnswMetaCodecName, lucene90HnswVersionCurrent, id, state.SegmentSuffix); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene90 hnsw: write meta header: %w", err)
	}
	if err := codecs.WriteIndexHeader(w.vectorData, lucene90HnswDataCodecName, lucene90HnswVersionCurrent, id, state.SegmentSuffix); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene90 hnsw: write data header: %w", err)
	}
	if err := codecs.WriteIndexHeader(w.vectorIndex, lucene90HnswIndexCodecName, lucene90HnswVersionCurrent, id, state.SegmentSuffix); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene90 hnsw: write index header: %w", err)
	}

	return w, nil
}

// lucene90HnswFieldWriter accumulates per-document float32 vectors for a
// single field.
type lucene90HnswFieldWriter struct {
	fieldInfo *index.FieldInfo
	docIDs    []int
	vectors   [][]float32
	lastDocID int
	finished  bool
}

// AddValue records a FLOAT32 vector for docID.
func (fw *lucene90HnswFieldWriter) AddValue(docID int, vectorValue any) error {
	if fw.finished {
		return errors.New("lucene90 hnsw: field writer already finished")
	}
	vec, ok := vectorValue.([]float32)
	if !ok {
		return fmt.Errorf("lucene90 hnsw: field %q expects []float32, got %T", fw.fieldInfo.Name(), vectorValue)
	}
	if len(vec) != fw.fieldInfo.VectorDimension() {
		return fmt.Errorf("lucene90 hnsw: field %q vector dim mismatch: got %d, want %d", fw.fieldInfo.Name(), len(vec), fw.fieldInfo.VectorDimension())
	}
	if n := len(fw.docIDs); n > 0 && fw.docIDs[n-1] >= docID {
		return fmt.Errorf("lucene90 hnsw: field %q out-of-order docID %d (last %d)", fw.fieldInfo.Name(), docID, fw.docIDs[n-1])
	}
	cp := make([]float32, len(vec))
	copy(cp, vec)
	fw.docIDs = append(fw.docIDs, docID)
	fw.vectors = append(fw.vectors, cp)
	fw.lastDocID = docID
	return nil
}

// RamBytesUsed estimates the field's in-memory footprint.
func (fw *lucene90HnswFieldWriter) RamBytesUsed() int64 {
	const docIDBytes = 8
	const floatBytes = 4
	return int64(len(fw.docIDs))*docIDBytes + int64(len(fw.vectors))*int64(fw.fieldInfo.VectorDimension())*floatBytes
}

// Finish marks the field as complete.
func (fw *lucene90HnswFieldWriter) Finish() error {
	fw.finished = true
	return nil
}

// AddField registers a new FLOAT32 vector field. Byte vectors are rejected
// with an explicit error, matching the Java reference.
func (w *Lucene90HnswVectorsWriter) AddField(fieldInfo *index.FieldInfo) (codecs.KnnFieldVectorsWriter, error) {
	if w.closed {
		return nil, errors.New("lucene90 hnsw: writer is closed")
	}
	if w.finished {
		return nil, errors.New("lucene90 hnsw: writer already finished")
	}
	if fieldInfo == nil {
		return nil, errors.New("lucene90 hnsw: AddField: nil FieldInfo")
	}
	if fieldInfo.VectorDimension() <= 0 {
		return nil, fmt.Errorf("lucene90 hnsw: field %q has non-positive dimension %d", fieldInfo.Name(), fieldInfo.VectorDimension())
	}
	if fieldInfo.VectorEncoding() != index.VectorEncodingFloat32 {
		return nil, fmt.Errorf("lucene90 hnsw: field %q encoding %v not supported (only FLOAT32)", fieldInfo.Name(), fieldInfo.VectorEncoding())
	}
	for _, existing := range w.fields {
		if existing.fieldInfo.Name() == fieldInfo.Name() {
			return nil, fmt.Errorf("lucene90 hnsw: duplicate field %q", fieldInfo.Name())
		}
	}
	fw := &lucene90HnswFieldWriter{
		fieldInfo: fieldInfo,
		lastDocID: -1,
	}
	w.fields = append(w.fields, fw)
	return fw, nil
}

// Flush serialises every buffered field. Mirrors Java's flush(int maxDoc,
// Sorter.DocMap). The index-sort path is out of scope.
func (w *Lucene90HnswVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	if w.closed {
		return errors.New("lucene90 hnsw: writer is closed")
	}
	if w.finished {
		return errors.New("lucene90 hnsw: writer already finished")
	}
	if sortMap != nil {
		return errors.New("lucene90 hnsw: index-sort (sortMap) not supported yet")
	}
	for _, fw := range w.fields {
		if err := w.flushField(fw); err != nil {
			return err
		}
	}
	return nil
}

// flushField builds the graph and writes the vector data, graph, and meta for
// a single field.
func (w *Lucene90HnswVectorsWriter) flushField(fw *lucene90HnswFieldWriter) error {
	if !fw.finished {
		fw.finished = true
	}

	// Align vector data to float boundary (4 bytes).
	vectorDataOffset, err := store.AlignFilePointer(w.vectorData, 4)
	if err != nil {
		return fmt.Errorf("lucene90 hnsw: align vector data: %w", err)
	}

	if err := w.writeVectorData(w.vectorData, fw); err != nil {
		return err
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset

	// Build graph from in-memory vectors.
	graph, err := w.buildGraph(fw)
	if err != nil {
		return err
	}

	vectorIndexOffset := w.vectorIndex.GetFilePointer()
	offsets, err := w.writeGraph(w.vectorIndex, vectorIndexOffset, graph, len(fw.docIDs))
	if err != nil {
		return err
	}
	vectorIndexLength := w.vectorIndex.GetFilePointer() - vectorIndexOffset

	return w.writeMeta(fw.fieldInfo, vectorDataOffset, vectorDataLength, vectorIndexOffset, vectorIndexLength, fw.docIDs, offsets)
}

// writeVectorData writes the little-endian float32 vectors for the field to
// out. Mirrors Java's private writeVectorData.
func (w *Lucene90HnswVectorsWriter) writeVectorData(out store.IndexOutput, fw *lucene90HnswFieldWriter) error {
	if len(fw.vectors) == 0 {
		return nil
	}
	dim := fw.fieldInfo.VectorDimension()
	scratch := make([]byte, dim*4)
	for i, vec := range fw.vectors {
		if len(vec) != dim {
			return fmt.Errorf("lucene90 hnsw: vector %d dim mismatch: got %d, want %d", i, len(vec), dim)
		}
		for j, f := range vec {
			binary.LittleEndian.PutUint32(scratch[j*4:], math.Float32bits(f))
		}
		if err := out.WriteBytes(scratch); err != nil {
			return err
		}
	}
	return nil
}

// buildGraph constructs an HNSW graph from the accumulated vectors using the
// util/hnsw package. Only level 0 is serialized by the Lucene90 format.
func (w *Lucene90HnswVectorsWriter) buildGraph(fw *lucene90HnswFieldWriter) (*utilhnsw.OnHeapHnswGraph, error) {
	if len(fw.vectors) == 0 {
		return nil, nil
	}
	mv := &memFloat32VectorValues{vecs: fw.vectors}
	supplier, err := newMemFloat32ScorerSupplier(mv, fw.fieldInfo.VectorSimilarityFunction())
	if err != nil {
		return nil, fmt.Errorf("lucene90 hnsw: scorer supplier: %w", err)
	}
	builder, err := utilhnsw.NewHnswGraphBuilderWithGraphSize(
		supplier, w.maxConn, w.beamWidth, utilhnsw.RandSeed, len(fw.vectors),
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90 hnsw: graph builder: %w", err)
	}
	graph, err := builder.Build(len(fw.vectors))
	if err != nil {
		return nil, fmt.Errorf("lucene90 hnsw: graph build: %w", err)
	}
	return graph, nil
}

// writeGraph serialises the flat (level 0) neighbour lists for graph into out.
// Returns the per-node byte offsets that the meta writer records.
// Mirrors Java's private writeGraph.
func (w *Lucene90HnswVectorsWriter) writeGraph(out store.IndexOutput, graphDataOffset int64, graph *utilhnsw.OnHeapHnswGraph, numNodes int) ([]int64, error) {
	if graph == nil || numNodes == 0 {
		return nil, nil
	}
	offsets := make([]int64, numNodes)
	for ord := 0; ord < numNodes; ord++ {
		offsets[ord] = out.GetFilePointer() - graphDataOffset
		neighbors := graph.GetNeighbors(0, ord)
		size := neighbors.Size()
		nodes := neighbors.Nodes()
		sort.Ints(nodes[:size])

		if err := out.WriteInt(int32(size)); err != nil {
			return nil, err
		}
		lastNode := -1
		for i := 0; i < size; i++ {
			node := nodes[i]
			if node <= lastNode {
				return nil, fmt.Errorf("lucene90 hnsw: nodes out of order at ord=%d: %d <= %d", ord, node, lastNode)
			}
			if node >= numNodes {
				return nil, fmt.Errorf("lucene90 hnsw: node too large at ord=%d: %d >= %d", ord, node, numNodes)
			}
			if err := store.WriteVInt(out, int32(node-lastNode)); err != nil {
				return nil, err
			}
			lastNode = node
		}
	}
	return offsets, nil
}

// writeMeta emits one field record on the meta file. Mirrors Java's private
// writeMeta + writeGraphOffsets.
func (w *Lucene90HnswVectorsWriter) writeMeta(
	fieldInfo *index.FieldInfo,
	vectorDataOffset, vectorDataLength int64,
	vectorIndexOffset, vectorIndexLength int64,
	docIDs []int,
	offsets []int64,
) error {
	simOrd, err := similarityToOrd(fieldInfo.VectorSimilarityFunction())
	if err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(fieldInfo.Number())); err != nil {
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
	if err := store.WriteVLong(w.meta, vectorIndexOffset); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorIndexLength); err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(fieldInfo.VectorDimension())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(len(docIDs))); err != nil {
		return err
	}
	for _, docID := range docIDs {
		if err := store.WriteVInt(w.meta, int32(docID)); err != nil {
			return err
		}
	}
	last := int64(0)
	for _, off := range offsets {
		if err := store.WriteVLong(w.meta, off-last); err != nil {
			return err
		}
		last = off
	}
	return nil
}

// WriteField is the single-reader merge entrypoint. Not supported yet.
func (w *Lucene90HnswVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader codecs.KnnVectorsReader) error {
	_ = fieldInfo
	_ = reader
	return errors.New("lucene90 hnsw: WriteField (merge path) not supported yet")
}

// RamBytesUsed reports the in-memory footprint of every per-field buffer.
func (w *Lucene90HnswVectorsWriter) RamBytesUsed() int64 {
	var total int64
	for _, fw := range w.fields {
		total += fw.RamBytesUsed()
	}
	return total
}

// Finish writes the end-of-fields sentinel (-1) and CodecUtil footers on all
// three segment files. Mirrors Java's public void finish().
func (w *Lucene90HnswVectorsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene90 hnsw: writer is closed")
	}
	if w.finished {
		return errors.New("lucene90 hnsw: already finished")
	}
	w.finished = true

	if w.meta != nil {
		if err := w.meta.WriteInt(-1); err != nil {
			return fmt.Errorf("lucene90 hnsw: write meta sentinel: %w", err)
		}
		if err := codecs.WriteFooter(w.meta); err != nil {
			return fmt.Errorf("lucene90 hnsw: write meta footer: %w", err)
		}
	}
	if w.vectorData != nil {
		if err := codecs.WriteFooter(w.vectorData); err != nil {
			return fmt.Errorf("lucene90 hnsw: write data footer: %w", err)
		}
	}
	if w.vectorIndex != nil {
		if err := codecs.WriteFooter(w.vectorIndex); err != nil {
			return fmt.Errorf("lucene90 hnsw: write index footer: %w", err)
		}
	}
	return nil
}

// Close releases the segment outputs. Idempotent.
func (w *Lucene90HnswVectorsWriter) Close() error {
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
	if w.vectorIndex != nil {
		if err := w.vectorIndex.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.vectorIndex = nil
	}
	return firstErr
}

// Compile-time interface checks.
var (
	_ codecs.KnnVectorsWriter      = (*Lucene90HnswVectorsWriter)(nil)
	_ codecs.KnnFieldVectorsWriter = (*lucene90HnswFieldWriter)(nil)
)

// ---------------------------------------------------------------------------
// In-memory float32 vector values and scorer supplier for HNSW graph building
// ---------------------------------------------------------------------------

// memFloat32VectorValues wraps a [][]float32 slice and implements
// util/hnsw.KnnVectorValues.
type memFloat32VectorValues struct {
	vecs [][]float32
}

func (m *memFloat32VectorValues) Dimension() int       { return len(m.vecs[0]) }
func (m *memFloat32VectorValues) Size() int            { return len(m.vecs) }
func (m *memFloat32VectorValues) OrdToDoc(ord int) int { return ord }
func (m *memFloat32VectorValues) GetAcceptOrds(_ util.Bits) util.Bits {
	return nil // nil == accept all
}
func (m *memFloat32VectorValues) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= len(m.vecs) {
		return nil, errors.New("memFloat32VectorValues: ordinal out of range")
	}
	return m.vecs[ord], nil
}
func (m *memFloat32VectorValues) Iterator() utilhnsw.DocIndexIterator {
	return &seqDocIndexIterator{size: len(m.vecs), cur: -1}
}
func (m *memFloat32VectorValues) CopyFloat() (*memFloat32VectorValues, error) {
	cp := make([][]float32, len(m.vecs))
	for i, v := range m.vecs {
		cp[i] = make([]float32, len(v))
		copy(cp[i], v)
	}
	return &memFloat32VectorValues{vecs: cp}, nil
}

// seqDocIndexIterator is a simple sequential DocIndexIterator.
type seqDocIndexIterator struct {
	size int
	cur  int
}

func (it *seqDocIndexIterator) NextDoc() (int, error) {
	it.cur++
	if it.cur >= it.size {
		it.cur = it.size
		return util.NO_MORE_DOCS, nil
	}
	return it.cur, nil
}
func (it *seqDocIndexIterator) Index() int { return it.cur }

// memFloat32ScorerSupplier implements util/hnsw.RandomVectorScorerSupplier.
type memFloat32ScorerSupplier struct {
	vecs   *memFloat32VectorValues
	target *memFloat32VectorValues
	sim    index.VectorSimilarityFunction
}

func newMemFloat32ScorerSupplier(vecs *memFloat32VectorValues, sim index.VectorSimilarityFunction) (utilhnsw.RandomVectorScorerSupplier, error) {
	tgt, err := vecs.CopyFloat()
	if err != nil {
		return nil, err
	}
	return &memFloat32ScorerSupplier{vecs: vecs, target: tgt, sim: sim}, nil
}

func (s *memFloat32ScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	buf := make([]float32, s.vecs.Dimension())
	base := utilhnsw.NewAbstractUpdateableRandomVectorScorer(s.vecs)
	return &memFloat32Scorer{
		AbstractUpdateableRandomVectorScorer: base,
		supplier:                             s,
		buf:                                  buf,
	}, nil
}

func (s *memFloat32ScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	return newMemFloat32ScorerSupplier(s.vecs, s.sim)
}

// memFloat32Scorer is the per-Scorer instance.
type memFloat32Scorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	supplier *memFloat32ScorerSupplier
	buf      []float32
}

func (s *memFloat32Scorer) SetScoringOrdinal(node int) error {
	v, err := s.supplier.target.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.buf, v)
	return nil
}

func (s *memFloat32Scorer) Score(node int) (float32, error) {
	v, err := s.supplier.vecs.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeFloatSimilarity(s.supplier.sim, s.buf, v), nil
}

func (s *memFloat32Scorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// memComputeFloatSimilarity mirrors the float similarity dispatch in
// codecs/hnsw/default_flat_vector_scorer.go. Duplicated here to avoid a
// cross-package import cycle (backward_codecs/lucene90 -> codecs/hnsw).
func memComputeFloatSimilarity(sim index.VectorSimilarityFunction, a, b []float32) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		var sum float32
		for i := range a {
			d := a[i] - b[i]
			sum += d * d
		}
		return 1.0 / (1.0 + sum)
	case index.VectorSimilarityFunctionDotProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		return (dot + 1.0) / 2.0
	case index.VectorSimilarityFunctionCosine:
		var dot, na, nb float32
		for i := range a {
			dot += a[i] * b[i]
			na += a[i] * a[i]
			nb += b[i] * b[i]
		}
		if na == 0 || nb == 0 {
			return 0
		}
		return (dot/(memSqrt32(na)*memSqrt32(nb)) + 1.0) / 2.0
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		if dot < 0 {
			return 1.0 / (1.0 - dot)
		}
		return dot + 1.0
	default:
		return 0
	}
}

func memSqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }
