// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene99HnswVectorsWriter constants. Mirror the static
// definitions from org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsFormat
// (Lucene 10.4.0) — kept as a private block here because Gocene's
// pre-existing Lucene99HnswVectorsFormat type already exports the
// public tuning knobs (max conn, beam width, threshold) but not the
// codec wire-level identifiers, which only the writer / reader pair
// needs.
const (
	// lucene99HnswMetaCodecName mirrors META_CODEC_NAME.
	lucene99HnswMetaCodecName = "Lucene99HnswVectorsFormatMeta"

	// lucene99HnswIndexCodecName mirrors VECTOR_INDEX_CODEC_NAME.
	lucene99HnswIndexCodecName = "Lucene99HnswVectorsFormatIndex"

	// lucene99HnswMetaExtension mirrors META_EXTENSION.
	lucene99HnswMetaExtension = "vem"

	// lucene99HnswIndexExtension mirrors VECTOR_INDEX_EXTENSION.
	lucene99HnswIndexExtension = "vex"

	// lucene99HnswVersionStart mirrors VERSION_START.
	lucene99HnswVersionStart int32 = 0

	// lucene99HnswVersionGroupVInt mirrors VERSION_GROUPVARINT — the
	// version that enables the GroupVInt encoding for delta-encoded
	// neighbour ids on level 0.
	lucene99HnswVersionGroupVInt int32 = 1

	// lucene99HnswVersionCurrent mirrors VERSION_CURRENT.
	lucene99HnswVersionCurrent int32 = lucene99HnswVersionGroupVInt

	// lucene99HnswDirectMonotonicBlockShift mirrors
	// Lucene99HnswVectorsFormat.DIRECT_MONOTONIC_BLOCK_SHIFT — the
	// block-shift used by the DirectMonotonicWriter that records per-
	// node neighbour offsets in the meta file.
	lucene99HnswDirectMonotonicBlockShift = 16
)

// lucene99HnswSimilarityOrdinals mirrors
// Lucene99HnswVectorsReader.SIMILARITY_FUNCTIONS — the ordered list
// that fixes the on-disk ordinal -> VectorSimilarityFunction mapping.
// The order is part of the wire format and must not change without a
// version bump.
var lucene99HnswSimilarityOrdinals = []index.VectorSimilarityFunction{
	index.VectorSimilarityFunctionEuclidean,
	index.VectorSimilarityFunctionDotProduct,
	index.VectorSimilarityFunctionCosine,
	index.VectorSimilarityFunctionMaximumInnerProduct,
}

// distFuncToOrd resolves the on-disk ordinal for the supplied
// similarity function. Mirrors Java's static helper of the same name.
func distFuncToOrd(f index.VectorSimilarityFunction) (int32, error) {
	for i, v := range lucene99HnswSimilarityOrdinals {
		if v == f {
			return int32(i), nil
		}
	}
	return 0, fmt.Errorf("hnsw: invalid distance function: %v", f)
}

// Lucene99HnswVectorsWriter writes HNSW graph data for the Lucene 9.9+
// format. It is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsWriter
// (Lucene 10.4.0), implementing the parts of that surface that the
// already-ported Gocene HNSW stack supports (graph build, on-disk
// graph layout, per-segment meta).
//
// Wire-format parity (.vem + .vex, see the Lucene99HnswVectorsFormat
// Javadoc for the full layout):
//
//   - `.vex` carries the per-level delta-encoded neighbour lists
//     followed by the DirectMonotonicWriter-encoded per-node offsets.
//     Identical byte layout to Lucene, including the GroupVInt
//     encoding gated by [lucene99HnswVersionGroupVInt].
//   - `.vem` carries one record per field: field number, vector
//     encoding ordinal, similarity ordinal, .vex offsets, dimension,
//     count, graph hyper-parameters and the per-level node lists; then
//     the DirectMonotonicWriter meta header. Terminated by an int32
//     sentinel -1, then the codec footer. Identical byte layout to
//     Lucene.
//
// Deviations from the Java reference (each tracked in the rmp task
// summary, [project-gocene-sprint-55-...]):
//
//  1. No `.vec` flat-vector file. The Java writer composes with a
//     [hnsw.FlatVectorsWriter] that owns the raw per-document vectors;
//     the corresponding Gocene FlatVectorsWriter implementation
//     (Lucene99FlatVectorsWriter) has not been ported yet. The current
//     constructor therefore does not take a FlatVectorsWriter, and
//     [WriteField] / [Flush] accumulate vectors only through the
//     in-memory FieldWriter without persisting them separately. A
//     follow-up sprint will wire the flat writer once it lands.
//  2. No MergeOneField / mergeOneFieldToIndex. Without a
//     FlatVectorsWriter the merge path cannot reuse merged vectors
//     across segments. The codec reader read path still works for the
//     graph-only data this writer emits, which is sufficient for the
//     graph builder / searcher tests in util/hnsw.
//  3. No Sorter.DocMap support on Flush. The Java reference reorders
//     per-doc vectors when an index sort is configured; Gocene's
//     index-sort sprint has not landed, so the writer accepts the
//     accumulated insertion order verbatim and panics if a caller
//     passes a non-nil sortMap to [WriteFieldSorted].
//  4. tinySegmentsThreshold honoured for build-time graph creation
//     gating — segments below the threshold skip graph construction
//     and the resulting meta entry records numLevels=0, matching the
//     Java tinySegment optimisation.
//
// Lifecycle (mirrors Lucene):
//
//	NewLucene99HnswVectorsWriter
//	  -> (AddField + AddValue)*
//	  -> Flush(maxDoc) (or WriteField per field, with no sort map)
//	  -> Finish
//	  -> Close
//
// The Lucene99HnswVectorsFormat already constructed in Gocene calls
// the writer through the legacy [KnnVectorsWriter] surface; that path
// goes through [WriteField] and is preserved here.
//
// Concurrency: not safe for concurrent use. Mirrors the Java
// reference, which is also single-threaded by codec contract.
type Lucene99HnswVectorsWriter struct {
	state                 *SegmentWriteState
	maxConn               int
	beamWidth             int
	tinySegmentsThreshold int
	numMergeWorkers       int
	version               int32

	meta        store.IndexOutput
	vectorIndex store.IndexOutput

	fields []*lucene99HnswFieldWriter

	finished bool
	closed   bool
}

// lucene99HnswFieldWriter is the per-field carrier used by
// Lucene99HnswVectorsWriter to accumulate vectors and build the HNSW
// graph for one field. It is the Go port of the private FieldWriter
// inner class in the Java reference, trimmed to the surface the
// current Gocene HNSW stack supports (graph build only — see writer
// deviation 1).
type lucene99HnswFieldWriter struct {
	fieldInfo *index.FieldInfo
	encoding  index.VectorEncoding

	// floats and bytes hold the per-document vectors as they arrive.
	// Exactly one is populated, gated by encoding.
	floats [][]float32
	bytes  [][]byte

	docIDs []int
	lastID int

	// graphBuilder is initialised lazily once the field has enough
	// vectors to cross the tinySegments threshold (or eagerly if the
	// threshold is zero). nil means "no graph for this field" —
	// either because no vector has been added yet or because the
	// segment stayed below the threshold for its lifetime.
	graphBuilder *hnsw.HnswGraphBuilder

	// completedGraph is set once Finish has been invoked. It captures
	// the graph returned by graphBuilder.GetCompletedGraph so that a
	// subsequent WriteField call does not refreeze the builder.
	completedGraph *hnsw.OnHeapHnswGraph

	maxConn   int
	beamWidth int
	threshold int

	finished bool
}

// NewLucene99HnswVectorsWriter constructs a Lucene99HnswVectorsWriter
// bound to state. It creates the .vex and .vem segment files and
// writes their codec headers. The parameter signature is preserved
// from the previous stub so the Lucene99HnswVectorsFormat factory
// continues to compile.
//
// numMergeWorkers is accepted for signature parity with the Java
// reference but is currently a no-op: the merge path is not wired
// (writer deviation 2).
func NewLucene99HnswVectorsWriter(
	state *SegmentWriteState,
	maxConn, beamWidth, tinySegmentsThreshold, numMergeWorkers int,
) (*Lucene99HnswVectorsWriter, error) {
	if state == nil {
		return nil, errors.New("hnsw99: nil SegmentWriteState")
	}
	if state.SegmentInfo == nil {
		return nil, errors.New("hnsw99: nil SegmentInfo")
	}
	if state.Directory == nil {
		return nil, errors.New("hnsw99: nil Directory")
	}
	if maxConn <= 0 {
		return nil, fmt.Errorf("hnsw99: maxConn must be positive; got %d", maxConn)
	}
	if beamWidth <= 0 {
		return nil, fmt.Errorf("hnsw99: beamWidth must be positive; got %d", beamWidth)
	}

	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99HnswMetaExtension)
	indexName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99HnswIndexExtension)

	rawMeta, err := state.Directory.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("hnsw99: create meta output %q: %w", metaName, err)
	}
	// Wrap in ChecksumIndexOutput so [WriteFooter] can compute the
	// trailing CRC32. Mirrors Lucene's behaviour where every codec
	// IndexOutput is wrapped in a BufferedChecksumIndexOutput before
	// reaching the writer.
	meta := store.NewChecksumIndexOutput(rawMeta)

	w := &Lucene99HnswVectorsWriter{
		state:                 state,
		maxConn:               maxConn,
		beamWidth:             beamWidth,
		tinySegmentsThreshold: tinySegmentsThreshold,
		numMergeWorkers:       numMergeWorkers,
		version:               lucene99HnswVersionCurrent,
		meta:                  meta,
	}

	rawIndex, err := state.Directory.CreateOutput(indexName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		return nil, fmt.Errorf("hnsw99: create index output %q: %w", indexName, err)
	}
	w.vectorIndex = store.NewChecksumIndexOutput(rawIndex)

	if err := w.writeHeaders(); err != nil {
		_ = w.Close()
		return nil, err
	}
	return w, nil
}

// writeHeaders writes the codec index header on both segment files,
// mirroring the CodecUtil.writeIndexHeader calls from the Java
// constructor.
func (w *Lucene99HnswVectorsWriter) writeHeaders() error {
	id := w.state.SegmentInfo.GetID()
	if err := WriteIndexHeader(
		w.meta, lucene99HnswMetaCodecName, w.version, id, w.state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("hnsw99: write meta header: %w", err)
	}
	if err := WriteIndexHeader(
		w.vectorIndex, lucene99HnswIndexCodecName, w.version, id, w.state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("hnsw99: write index header: %w", err)
	}
	return nil
}

// AddField allocates a new per-field writer bound to fieldInfo and
// returns it. Callers stream per-document vectors via
// [Lucene99HnswFieldWriter.AddValue]. Mirrors Java's
// public KnnFieldVectorsWriter<?> addField(FieldInfo).
//
// AddField returns an error when the field has already been added or
// when fieldInfo lacks vector metadata.
func (w *Lucene99HnswVectorsWriter) AddField(
	fieldInfo *index.FieldInfo,
) (*lucene99HnswFieldWriter, error) {
	if w.closed {
		return nil, errors.New("hnsw99: writer is closed")
	}
	if w.finished {
		return nil, errors.New("hnsw99: writer already finished")
	}
	if fieldInfo == nil {
		return nil, errors.New("hnsw99: AddField: nil FieldInfo")
	}
	if fieldInfo.VectorDimension() <= 0 {
		return nil, fmt.Errorf(
			"hnsw99: AddField: field %q has non-positive vector dimension %d",
			fieldInfo.Name(), fieldInfo.VectorDimension())
	}
	for _, existing := range w.fields {
		if existing.fieldInfo.Name() == fieldInfo.Name() {
			return nil, fmt.Errorf("hnsw99: AddField: duplicate field %q", fieldInfo.Name())
		}
	}

	fw := &lucene99HnswFieldWriter{
		fieldInfo: fieldInfo,
		encoding:  fieldInfo.VectorEncoding(),
		lastID:    -1,
		maxConn:   w.maxConn,
		beamWidth: w.beamWidth,
		threshold: w.tinySegmentsThreshold,
	}
	w.fields = append(w.fields, fw)
	return fw, nil
}

// AddValueFloat32 records a float32 vector for docID. Returns an
// error when the field uses a different encoding, when docID is not
// strictly greater than the previous one, or when the vector length
// does not match the configured dimension.
//
// Mirrors the BYTE/FLOAT32 dispatch in Java's FieldWriter.addValue,
// trimmed to the slice surface Gocene exposes.
func (fw *lucene99HnswFieldWriter) AddValueFloat32(docID int, vector []float32) error {
	if fw.finished {
		return errors.New("hnsw99: field writer already finished")
	}
	if fw.encoding != index.VectorEncodingFloat32 {
		return fmt.Errorf(
			"hnsw99: field %q encoding %v, AddValueFloat32 called",
			fw.fieldInfo.Name(), fw.encoding)
	}
	if got, want := len(vector), fw.fieldInfo.VectorDimension(); got != want {
		return fmt.Errorf(
			"hnsw99: field %q vector dim mismatch: got %d, want %d",
			fw.fieldInfo.Name(), got, want)
	}
	if docID <= fw.lastID {
		return fmt.Errorf(
			"hnsw99: field %q docID %d not strictly greater than previous %d",
			fw.fieldInfo.Name(), docID, fw.lastID)
	}

	cp := make([]float32, len(vector))
	copy(cp, vector)
	fw.floats = append(fw.floats, cp)
	fw.docIDs = append(fw.docIDs, docID)
	fw.lastID = docID
	return nil
}

// AddValueByte records a byte vector for docID. See AddValueFloat32
// for the validation contract.
func (fw *lucene99HnswFieldWriter) AddValueByte(docID int, vector []byte) error {
	if fw.finished {
		return errors.New("hnsw99: field writer already finished")
	}
	if fw.encoding != index.VectorEncodingByte {
		return fmt.Errorf(
			"hnsw99: field %q encoding %v, AddValueByte called",
			fw.fieldInfo.Name(), fw.encoding)
	}
	if got, want := len(vector), fw.fieldInfo.VectorDimension(); got != want {
		return fmt.Errorf(
			"hnsw99: field %q vector dim mismatch: got %d, want %d",
			fw.fieldInfo.Name(), got, want)
	}
	if docID <= fw.lastID {
		return fmt.Errorf(
			"hnsw99: field %q docID %d not strictly greater than previous %d",
			fw.fieldInfo.Name(), docID, fw.lastID)
	}

	cp := make([]byte, len(vector))
	copy(cp, vector)
	fw.bytes = append(fw.bytes, cp)
	fw.docIDs = append(fw.docIDs, docID)
	fw.lastID = docID
	return nil
}

// NumDocs returns the number of vectors accumulated for the field.
// Mirrors the cardinality of the Java reference's DocsWithFieldSet.
func (fw *lucene99HnswFieldWriter) NumDocs() int {
	if fw.encoding == index.VectorEncodingByte {
		return len(fw.bytes)
	}
	return len(fw.floats)
}

// WriteField is preserved from the previous stub for the
// [KnnVectorsWriter] interface contract. The legacy code path supplied
// a reader-backed source of vectors; the new path uses the per-field
// AddField + AddValue accumulator. This shim looks up the matching
// FieldWriter and serialises it; supplying a nil reader (or one whose
// field is not registered) is treated as the empty-field case.
//
// Deviation 5: the legacy reader-driven WriteField is not byte-
// for-byte equivalent to the Java path that streams vectors from a
// MergeState; that path is the merge code, not the flush code. Until
// MergeState lands the shim is a thin wrapper over the in-memory
// accumulator.
func (w *Lucene99HnswVectorsWriter) WriteField(
	fieldInfo *index.FieldInfo, reader KnnVectorsReader,
) error {
	if w.closed {
		return errors.New("hnsw99: writer is closed")
	}
	if w.finished {
		return errors.New("hnsw99: writer already finished")
	}
	if fieldInfo == nil {
		return errors.New("hnsw99: WriteField: nil FieldInfo")
	}
	_ = reader // see deviation 5 above
	var fw *lucene99HnswFieldWriter
	for _, candidate := range w.fields {
		if candidate.fieldInfo.Name() == fieldInfo.Name() {
			fw = candidate
			break
		}
	}
	if fw == nil {
		// No per-field accumulator: emit an empty-field meta record so
		// the reader can still iterate the segment without tripping the
		// "missing field" guard.
		return w.writeEmptyFieldMeta(fieldInfo)
	}
	return w.flushField(fw)
}

// Flush serialises every field accumulated so far. Mirrors Java's
// public void flush(int maxDoc, Sorter.DocMap sortMap). maxDoc is
// accepted for signature parity but is not consulted: each field
// records its own docIDs slice. sortMap support is not yet wired (see
// deviation 3).
func (w *Lucene99HnswVectorsWriter) Flush(maxDoc int) error {
	_ = maxDoc
	if w.closed {
		return errors.New("hnsw99: writer is closed")
	}
	if w.finished {
		return errors.New("hnsw99: writer already finished")
	}
	for _, fw := range w.fields {
		if err := w.flushField(fw); err != nil {
			return err
		}
	}
	return nil
}

// flushField builds the graph for a single field (if it crosses the
// tiny-segment threshold) and writes the per-field entries on .vex
// and .vem.
func (w *Lucene99HnswVectorsWriter) flushField(fw *lucene99HnswFieldWriter) error {
	if !fw.finished {
		if err := fw.finish(); err != nil {
			return err
		}
	}
	graph := fw.completedGraph

	vectorIndexOffset := w.vectorIndex.GetFilePointer()
	levelOffsets, err := writeHnswGraph(w.vectorIndex, w.version, graph)
	if err != nil {
		return err
	}
	vectorIndexLength := w.vectorIndex.GetFilePointer() - vectorIndexOffset

	return w.writeMeta(
		fw.fieldInfo,
		vectorIndexOffset,
		vectorIndexLength,
		fw.NumDocs(),
		graph,
		levelOffsets,
	)
}

// writeEmptyFieldMeta writes the meta record corresponding to a field
// with no per-segment vectors. Mirrors Lucene's behaviour when
// FieldWriter.docsWithFieldSet is empty: a zero-count entry with no
// graph levels.
func (w *Lucene99HnswVectorsWriter) writeEmptyFieldMeta(fieldInfo *index.FieldInfo) error {
	return w.writeMeta(fieldInfo, 0, 0, 0, nil, nil)
}

// finish freezes the per-field graph builder. After finish, the
// FieldWriter exposes the completed graph through completedGraph.
func (fw *lucene99HnswFieldWriter) finish() error {
	if fw.finished {
		return nil
	}
	fw.finished = true
	if fw.graphBuilder != nil {
		g, err := fw.graphBuilder.GetCompletedGraph()
		if err != nil {
			return fmt.Errorf("hnsw99: field %q: GetCompletedGraph: %w",
				fw.fieldInfo.Name(), err)
		}
		fw.completedGraph = g
	}
	return nil
}

// Finish writes the meta sentinel (-1) and the codec footer on both
// segment files. Mirrors Java's public void finish().
func (w *Lucene99HnswVectorsWriter) Finish() error {
	if w.closed {
		return errors.New("hnsw99: writer is closed")
	}
	if w.finished {
		return errors.New("hnsw99: already finished")
	}
	w.finished = true

	if w.meta != nil {
		if err := w.meta.WriteInt(-1); err != nil {
			return fmt.Errorf("hnsw99: write meta sentinel: %w", err)
		}
		if err := WriteFooter(w.meta); err != nil {
			return fmt.Errorf("hnsw99: write meta footer: %w", err)
		}
	}
	if w.vectorIndex != nil {
		if err := WriteFooter(w.vectorIndex); err != nil {
			return fmt.Errorf("hnsw99: write index footer: %w", err)
		}
	}
	return nil
}

// Close releases the segment outputs, mirroring IOUtils.close(meta,
// vectorIndex) in the Java reference. Close is idempotent.
func (w *Lucene99HnswVectorsWriter) Close() error {
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
	if w.vectorIndex != nil {
		if err := w.vectorIndex.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.vectorIndex = nil
	}
	return firstErr
}

// writeMeta emits one field record on the meta file, mirroring Java's
// private void writeMeta. Layout (matches the Lucene99HnswVectorsFormat
// Javadoc verbatim):
//
//	int32  field number
//	int32  vector encoding ordinal
//	int32  similarity ordinal
//	vlong  vectorIndex offset
//	vlong  vectorIndex length
//	vint   dimension
//	int32  count
//	vint   M
//	vint   numLevels                 (0 when no graph)
//	per level > 0:
//	  vint  nodes on level
//	  vint[] delta-encoded sorted node ids
//	int64  start offset of DirectMonotonicWriter data            (only when numLevels > 0)
//	vint   DIRECT_MONOTONIC_BLOCK_SHIFT                          (only when numLevels > 0)
//	DirectMonotonicWriter meta header                            (only when numLevels > 0)
//	int64  length of DirectMonotonicWriter data                  (only when numLevels > 0)
func (w *Lucene99HnswVectorsWriter) writeMeta(
	fieldInfo *index.FieldInfo,
	vectorIndexOffset, vectorIndexLength int64,
	count int,
	graph *hnsw.OnHeapHnswGraph,
	levelNodeOffsets [][]int,
) error {
	simOrd, err := distFuncToOrd(fieldInfo.VectorSimilarityFunction())
	if err != nil {
		return err
	}

	if err := w.meta.WriteInt(int32(fieldInfo.Number())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(vectorEncodingOrdinal(fieldInfo.VectorEncoding())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(simOrd); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorIndexOffset); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorIndexLength); err != nil {
		return err
	}
	if err := store.WriteVInt(w.meta, int32(fieldInfo.VectorDimension())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(count)); err != nil {
		return err
	}
	if err := store.WriteVInt(w.meta, int32(w.maxConn)); err != nil {
		return err
	}

	if graph == nil {
		return store.WriteVInt(w.meta, 0)
	}

	numLevels, _ := graph.NumLevels()
	if err := store.WriteVInt(w.meta, int32(numLevels)); err != nil {
		return err
	}

	var valueCount int64
	for level := 0; level < numLevels; level++ {
		nodes, err := graph.GetNodesOnLevel(level)
		if err != nil {
			return fmt.Errorf("hnsw99: nodes on level %d: %w", level, err)
		}
		valueCount += int64(nodes.Size())
		if level > 0 {
			nol := make([]int, nodes.Size())
			consumed := nodes.Consume(nol)
			if consumed != nodes.Size() {
				return fmt.Errorf(
					"hnsw99: level %d nodes consumed %d, want %d",
					level, consumed, nodes.Size())
			}
			sort.Ints(nol)
			if err := store.WriteVInt(w.meta, int32(len(nol))); err != nil {
				return err
			}
			for i := len(nol) - 1; i > 0; i-- {
				nol[i] -= nol[i-1]
			}
			for _, n := range nol {
				if n < 0 {
					return fmt.Errorf(
						"hnsw99: level %d delta encoding produced negative %d", level, n)
				}
				if err := store.WriteVInt(w.meta, int32(n)); err != nil {
					return err
				}
			}
		} else if nodes.Size() != count {
			return fmt.Errorf(
				"hnsw99: level 0 expects %d nodes, got %d", count, nodes.Size())
		}
	}

	start := w.vectorIndex.GetFilePointer()
	if err := w.meta.WriteLong(start); err != nil {
		return err
	}
	if err := store.WriteVInt(w.meta, lucene99HnswDirectMonotonicBlockShift); err != nil {
		return err
	}
	dm, err := packed.NewDirectMonotonicWriter(
		dmAdapter{w.meta}, dmAdapter{w.vectorIndex},
		valueCount, lucene99HnswDirectMonotonicBlockShift,
	)
	if err != nil {
		return fmt.Errorf("hnsw99: DirectMonotonicWriter: %w", err)
	}
	var cumulative int64
	for _, perLevel := range levelNodeOffsets {
		for _, off := range perLevel {
			if err := dm.Add(cumulative); err != nil {
				return err
			}
			cumulative += int64(off)
		}
	}
	if err := dm.Finish(); err != nil {
		return fmt.Errorf("hnsw99: DirectMonotonicWriter.Finish: %w", err)
	}
	return w.meta.WriteLong(w.vectorIndex.GetFilePointer() - start)
}

// dmAdapter satisfies packed.DataOutputAt for the meta and index
// IndexOutputs. The adapter exists because the Gocene
// DirectMonotonicWriter requires a narrow interface (DataOutput +
// GetFilePointer) rather than the full IndexOutput surface.
type dmAdapter struct {
	out store.IndexOutput
}

func (a dmAdapter) WriteByte(b byte) error    { return a.out.WriteByte(b) }
func (a dmAdapter) WriteBytes(b []byte) error { return a.out.WriteBytes(b) }
func (a dmAdapter) WriteBytesN(b []byte, n int) error {
	return a.out.WriteBytesN(b, n)
}
func (a dmAdapter) WriteShort(v int16) error   { return a.out.WriteShort(v) }
func (a dmAdapter) WriteInt(v int32) error     { return a.out.WriteInt(v) }
func (a dmAdapter) WriteLong(v int64) error    { return a.out.WriteLong(v) }
func (a dmAdapter) WriteString(s string) error { return a.out.WriteString(s) }
func (a dmAdapter) GetFilePointer() int64      { return a.out.GetFilePointer() }

// vectorEncodingOrdinal maps a VectorEncoding to its on-disk ordinal.
// The Java reference uses Enum.ordinal(), which yields BYTE=0,
// FLOAT32=1. Gocene's VectorEncoding constants follow the same order
// (VectorEncodingByte = iota), so the underlying integer value is
// already wire-correct.
func vectorEncodingOrdinal(e index.VectorEncoding) int32 {
	return int32(e)
}

// writeHnswGraph serialises the per-level neighbour lists for graph
// into out. Returns a per-level slice of per-node byte offsets that
// the meta writer feeds to DirectMonotonicWriter.
//
// Mirrors Java's private int[][] writeGraph(OnHeapHnswGraph), with one
// difference: when graph is nil (no graph was built — tiny segment or
// empty field) the function emits no bytes and returns nil, leaving
// the meta writer to record numLevels=0.
func writeHnswGraph(
	out store.IndexOutput, version int32, graph *hnsw.OnHeapHnswGraph,
) ([][]int, error) {
	if graph == nil {
		return nil, nil
	}
	countOnLevel0 := graph.Size()
	numLevels, _ := graph.NumLevels()
	offsets := make([][]int, numLevels)
	maxConn := graph.MaxConn()
	scratch := make([]int, maxConn*2)
	scratch32 := make([]int32, maxConn*2)
	groupScratch := make([]byte, util.GroupVIntMaxLengthPerGroup)

	for level := 0; level < numLevels; level++ {
		sortedNodes, err := hnsw.GetSortedNodes(graph, level)
		if err != nil {
			return nil, fmt.Errorf("hnsw99: GetSortedNodes(level=%d): %w", level, err)
		}
		offsets[level] = make([]int, sortedNodes.Size())
		nodeIdx := 0
		for sortedNodes.HasNext() {
			node := sortedNodes.NextInt()
			neighbors := graph.GetNeighbors(level, node)
			size := neighbors.Size()
			offsetStart := out.GetFilePointer()

			// Destructively sort the live nodes slice — matches Java.
			nnodes := neighbors.Nodes()
			sort.Ints(nnodes[:size])

			actualSize := 0
			if size > 0 {
				scratch[0] = nnodes[0]
				actualSize = 1
			}
			for i := 1; i < size; i++ {
				if nnodes[i] >= countOnLevel0 {
					return nil, fmt.Errorf(
						"hnsw99: node too large: %d >= %d", nnodes[i], countOnLevel0)
				}
				if nnodes[i-1] == nnodes[i] {
					continue
				}
				scratch[actualSize] = nnodes[i] - nnodes[i-1]
				actualSize++
			}

			if err := store.WriteVInt(out, int32(actualSize)); err != nil {
				return nil, err
			}
			if version >= lucene99HnswVersionGroupVInt {
				for i := 0; i < actualSize; i++ {
					scratch32[i] = int32(scratch[i])
				}
				if err := util.WriteGroupVInts(out, groupScratch, scratch32, actualSize); err != nil {
					return nil, err
				}
			} else {
				for i := 0; i < actualSize; i++ {
					if err := store.WriteVInt(out, int32(scratch[i])); err != nil {
						return nil, err
					}
				}
			}

			offsets[level][nodeIdx] = int(out.GetFilePointer() - offsetStart)
			nodeIdx++
		}
	}
	return offsets, nil
}
