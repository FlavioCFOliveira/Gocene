// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version 2.0
//   (the "License"); you may not use this file except in compliance with
//   the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package codecs

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene99HnswVectorsReader reads HNSW vector data for the Lucene 9.9+ format.
// It parses per-field metadata from the .vem file and reads the off-heap HNSW
// graph from the .vex file.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsReader (Lucene 10.4.0).
//
// Deviations from the Java reference:
//  1. Resolved by rmp #4731: the reader now composes a
//     [Lucene99FlatVectorsReader] that reads the raw per-document vectors
//     from .vec / .vemf. GetFloatVectorValues / GetByteVectorValues and the
//     search entry points are backed by it for the dense case; the sparse
//     case surfaces the flat reader's typed error (rmp #4755).
//  2. QuantizedVectorsReader interface not implemented (no ScalarQuantizer
//     support in this sprint).
type Lucene99HnswVectorsReader struct {
	fieldInfos  *index.FieldInfos
	fields      map[int]*lucene99HnswFieldEntry // keyed by field number
	vectorIndex store.IndexInput                // open .vex file
	flatReader  *Lucene99FlatVectorsReader      // reads .vec / .vemf
	version     int32
	closed      bool
}

// lucene99HnswFieldEntry mirrors the Java FieldEntry record.
type lucene99HnswFieldEntry struct {
	similarityFunction index.VectorSimilarityFunction
	vectorEncoding     index.VectorEncoding
	vectorIndexOffset  int64
	vectorIndexLength  int64
	M                  int
	numLevels          int
	dimension          int
	size               int
	nodesByLevel       [][]int // nil for level 0 (all nodes implicit); populated for levels 1+
	offsetsMeta        *packed.DirectMonotonicMeta
	offsetsOffset      int64
	offsetsBlockShift  int
	offsetsLength      int64
}

// NewLucene99HnswVectorsReader creates a new HNSW vectors reader.
// It reads and validates the .vem header and per-field entries, then opens
// the .vex graph index file.
//
// Mirrors Lucene99HnswVectorsReader(SegmentReadState, FlatVectorsReader).
// The flatVectorsReader parameter is omitted because the Gocene writer does
// not emit .vec files yet.
func NewLucene99HnswVectorsReader(state *SegmentReadState) (*Lucene99HnswVectorsReader, error) {
	r := &Lucene99HnswVectorsReader{
		fieldInfos: state.FieldInfos,
		fields:     make(map[int]*lucene99HnswFieldEntry),
	}

	// --- read .vem metadata ---
	metaName := state.SegmentInfo.Name()
	if state.SegmentSuffix != "" {
		metaName += "_" + state.SegmentSuffix
	}
	metaName += "." + lucene99HnswMetaExtension

	metaRaw, err := state.Directory.OpenInput(metaName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("hnsw99 reader: open meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexInput(metaRaw)

	var readErr error
	func() {
		id := state.SegmentInfo.GetID()
		versionMeta, e := CheckIndexHeader(
			meta, lucene99HnswMetaCodecName,
			lucene99HnswVersionStart, lucene99HnswVersionCurrent,
			id, state.SegmentSuffix,
		)
		if e != nil {
			readErr = e
			return
		}
		r.version = int32(versionMeta)
		readErr = r.readFields(meta)
	}()

	_, footerErr := CheckFooter(meta)
	_ = metaRaw.Close()
	if readErr != nil {
		return nil, readErr
	}
	if footerErr != nil {
		return nil, fmt.Errorf("hnsw99 reader: meta footer %q: %w", metaName, footerErr)
	}

	// --- open .vex graph index ---
	idxName := state.SegmentInfo.Name()
	if state.SegmentSuffix != "" {
		idxName += "_" + state.SegmentSuffix
	}
	idxName += "." + lucene99HnswIndexExtension

	vectorIndex, err := state.Directory.OpenInput(idxName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("hnsw99 reader: open index %q: %w", idxName, err)
	}
	id := state.SegmentInfo.GetID()
	versionIdx, err := CheckIndexHeader(
		vectorIndex, lucene99HnswIndexCodecName,
		lucene99HnswVersionStart, lucene99HnswVersionCurrent,
		id, state.SegmentSuffix,
	)
	if err != nil {
		_ = vectorIndex.Close()
		return nil, fmt.Errorf("hnsw99 reader: index header %q: %w", idxName, err)
	}
	if int32(versionIdx) != r.version {
		_ = vectorIndex.Close()
		return nil, fmt.Errorf("hnsw99 reader: meta version %d != index version %d",
			r.version, versionIdx)
	}
	r.vectorIndex = vectorIndex

	// Open the composed flat reader for the raw vectors (.vec / .vemf),
	// mirroring the FlatVectorsReader the Java Lucene99HnswVectorsReader
	// delegates to.
	flat, err := NewLucene99FlatVectorsReader(state)
	if err != nil {
		_ = vectorIndex.Close()
		return nil, fmt.Errorf("hnsw99 reader: open flat reader: %w", err)
	}
	r.flatReader = flat
	return r, nil
}

// readFields parses all per-field entries from the meta input until the -1 sentinel.
func (r *Lucene99HnswVectorsReader) readFields(meta store.DataInput) error {
	for {
		fieldNum, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("hnsw99 reader: reading field number: %w", err)
		}
		if fieldNum == -1 {
			break
		}
		info := r.fieldInfos.GetByNumber(int(fieldNum))
		if info == nil {
			return fmt.Errorf("hnsw99 reader: invalid field number %d", fieldNum)
		}
		entry, err := r.readFieldEntry(meta, info)
		if err != nil {
			return fmt.Errorf("hnsw99 reader: field %d: %w", fieldNum, err)
		}
		r.fields[int(fieldNum)] = entry
	}
	return nil
}

// readFieldEntry parses one FieldEntry from the meta stream.
// Mirrors Lucene99HnswVectorsReader.readField + FieldEntry.create.
func (r *Lucene99HnswVectorsReader) readFieldEntry(meta store.DataInput, info *index.FieldInfo) (*lucene99HnswFieldEntry, error) {
	// vector encoding ordinal (BYTE=0, FLOAT32=1)
	encOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	enc := index.VectorEncoding(encOrd)

	// similarity function ordinal
	simOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	if int(simOrd) < 0 || int(simOrd) >= len(lucene99HnswSimilarityOrdinals) {
		return nil, fmt.Errorf("invalid similarity ordinal: %d", simOrd)
	}
	sim := lucene99HnswSimilarityOrdinals[simOrd]

	vectorIndexOffset, err := store.ReadVLong(meta)
	if err != nil {
		return nil, err
	}
	vectorIndexLength, err := store.ReadVLong(meta)
	if err != nil {
		return nil, err
	}
	dimV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	size, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	mV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	numLevelsV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	numLevels := int(numLevelsV)

	nodesByLevel := make([][]int, numLevels)
	var numberOfOffsets int64
	for level := 0; level < numLevels; level++ {
		if level > 0 {
			numNodesV, e := store.ReadVInt(meta)
			if e != nil {
				return nil, e
			}
			numNodes := int(numNodesV)
			numberOfOffsets += int64(numNodes)
			nodes := make([]int, numNodes)
			if numNodes > 0 {
				first, e2 := store.ReadVInt(meta)
				if e2 != nil {
					return nil, e2
				}
				nodes[0] = int(first)
				for i := 1; i < numNodes; i++ {
					delta, e3 := store.ReadVInt(meta)
					if e3 != nil {
						return nil, e3
					}
					nodes[i] = nodes[i-1] + int(delta)
				}
			}
			nodesByLevel[level] = nodes
		} else {
			// level 0: all size nodes are implicit; nodesByLevel[0] stays nil
			numberOfOffsets += int64(size)
		}
	}

	var offsetsOffset int64
	var offsetsBlockShift int
	var offsetsMeta *packed.DirectMonotonicMeta
	var offsetsLength int64

	if numberOfOffsets > 0 {
		offsetsOffset, err = meta.ReadLong()
		if err != nil {
			return nil, err
		}
		bsV, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		offsetsBlockShift = int(bsV)
		offsetsMeta, err = packed.LoadDirectMonotonicMeta(meta, numberOfOffsets, offsetsBlockShift)
		if err != nil {
			return nil, err
		}
		offsetsLength, err = meta.ReadLong()
		if err != nil {
			return nil, err
		}
	}

	return &lucene99HnswFieldEntry{
		similarityFunction: sim,
		vectorEncoding:     enc,
		vectorIndexOffset:  vectorIndexOffset,
		vectorIndexLength:  vectorIndexLength,
		M:                  int(mV),
		numLevels:          numLevels,
		dimension:          int(dimV),
		size:               int(size),
		nodesByLevel:       nodesByLevel,
		offsetsMeta:        offsetsMeta,
		offsetsOffset:      offsetsOffset,
		offsetsBlockShift:  offsetsBlockShift,
		offsetsLength:      offsetsLength,
	}, nil
}

// CheckIntegrity verifies the checksums of the .vex and .vec files.
func (r *Lucene99HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("hnsw99 reader: closed")
	}
	if r.flatReader != nil {
		if err := r.flatReader.CheckIntegrity(); err != nil {
			return err
		}
	}
	_, err := ChecksumEntireFile(r.vectorIndex)
	return err
}

// Close releases the .vex file handle and the composed flat reader.
func (r *Lucene99HnswVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var firstErr error
	if r.flatReader != nil {
		if err := r.flatReader.Close(); err != nil {
			firstErr = err
		}
		r.flatReader = nil
	}
	if r.vectorIndex != nil {
		if err := r.vectorIndex.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetFloatVectorValues returns the float vectors for the named field,
// reading them from the composed flat reader (.vec). Dense fields only
// (rmp #4731); a sparse field surfaces the flat reader's typed error
// (rmp #4755). Mirrors Lucene99HnswVectorsReader.getFloatVectorValues,
// which forwards to flatVectorsReader.getFloatVectorValues.
func (r *Lucene99HnswVectorsReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	if r.flatReader == nil {
		return nil, errors.New("hnsw99 reader: flat reader not initialised")
	}
	values, err := r.flatReader.floatVectorValues(field)
	if err != nil {
		return nil, err
	}
	return &denseFloatVectorValuesAdapter{values: values, doc: -1}, nil
}

// GetByteVectorValues returns the byte vectors for the named field,
// reading them from the composed flat reader (.vec). Dense fields only
// (rmp #4731). Mirrors Lucene99HnswVectorsReader.getByteVectorValues.
func (r *Lucene99HnswVectorsReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	if r.flatReader == nil {
		return nil, errors.New("hnsw99 reader: flat reader not initialised")
	}
	values, err := r.flatReader.byteVectorValues(field)
	if err != nil {
		return nil, err
	}
	return &denseByteVectorValuesAdapter{values: values, doc: -1}, nil
}

// FloatVectorValues returns the field's float vectors typed as the
// index-package [index.FloatVectorValues] surface (Get(docID)). It is the
// entry point the index layer (LeafReader / SegmentReader) consumes through
// a structural delegate interface; the returned adapter is the same one
// [GetFloatVectorValues] yields, which satisfies both surfaces.
func (r *Lucene99HnswVectorsReader) FloatVectorValues(field string) (index.FloatVectorValues, error) {
	if r.flatReader == nil {
		return nil, errors.New("hnsw99 reader: flat reader not initialised")
	}
	values, err := r.flatReader.floatVectorValues(field)
	if err != nil {
		return nil, err
	}
	return &denseFloatVectorValuesAdapter{values: values, doc: -1}, nil
}

// ByteVectorValues is the byte analogue of [FloatVectorValues].
func (r *Lucene99HnswVectorsReader) ByteVectorValues(field string) (index.ByteVectorValues, error) {
	if r.flatReader == nil {
		return nil, errors.New("hnsw99 reader: flat reader not initialised")
	}
	values, err := r.flatReader.byteVectorValues(field)
	if err != nil {
		return nil, err
	}
	return &denseByteVectorValuesAdapter{values: values, doc: -1}, nil
}

// GetGraph returns the off-heap HNSW graph for the named field.
// Implements codecs/hnsw.HnswGraphProvider.
func (r *Lucene99HnswVectorsReader) GetGraph(field string) (utilhnsw.HnswGraph, error) {
	info := r.fieldInfos.GetByName(field)
	if info == nil {
		return nil, fmt.Errorf("hnsw99 reader: field %q not found", field)
	}
	entry, ok := r.fields[info.Number()]
	if !ok {
		return nil, fmt.Errorf("hnsw99 reader: field %q has no HNSW entry", field)
	}
	if entry.vectorIndexLength == 0 {
		return utilhnsw.Empty(), nil
	}
	return newOffHeapHnswGraph(entry, r.vectorIndex, r.version)
}

// SearchFloat is the legacy any-typed search entry point retained for the
// codecs/hnsw.FlatVectorsReader surface. The concrete search path used by
// the index layer is [SearchNearestFloat]; this overload always returns an
// error directing callers there (knnCollector / acceptDocs cannot be typed
// here without importing search, which would create an import cycle).
func (r *Lucene99HnswVectorsReader) SearchFloat(_ string, _ []float32, _ any, _ util.Bits) error {
	return errors.New("hnsw99 reader: use SearchNearestFloat (the typed search entry point)")
}

// SearchByte is the byte analogue of [SearchFloat]; see [SearchNearestByte].
func (r *Lucene99HnswVectorsReader) SearchByte(_ string, _ []byte, _ any, _ util.Bits) error {
	return errors.New("hnsw99 reader: use SearchNearestByte (the typed search entry point)")
}

// SearchNearestFloat performs approximate (HNSW) or exhaustive
// nearest-neighbour search for the float32 target against field, collecting
// the top k results restricted to acceptDocs (nil accepts all live docs).
//
// It mirrors the body of Lucene99HnswVectorsReader.search(String, float[],
// KnnCollector, AcceptDocs): build a query-vs-node scorer over the flat
// vectors, wrap the collector in an OrdinalTranslatedKnnCollector, then
// either run HnswGraphSearcher.search over the .vex graph or, when k is
// large relative to the graph, score every accepted ordinal exhaustively.
func (r *Lucene99HnswVectorsReader) SearchNearestFloat(
	field string, target []float32, k int, acceptDocs util.Bits,
) (*utilhnsw.TopDocs, error) {
	scorer, err := r.flatReader.randomVectorScorerFloat(field, target)
	if err != nil {
		return nil, err
	}
	return r.search(field, scorer, k, acceptDocs)
}

// SearchNearestByte is the byte analogue of [SearchNearestFloat].
func (r *Lucene99HnswVectorsReader) SearchNearestByte(
	field string, target []byte, k int, acceptDocs util.Bits,
) (*utilhnsw.TopDocs, error) {
	scorer, err := r.flatReader.randomVectorScorerByte(field, target)
	if err != nil {
		return nil, err
	}
	return r.search(field, scorer, k, acceptDocs)
}

// search runs the shared HNSW-or-exhaustive decision for the supplied
// query-vs-node scorer. Mirrors the private search() helper in the Java
// reference. visitedLimit is set to MaxInt (no budget) because the caller
// (the index/search layer) enforces its own visit budget through the
// collector it owns; this internal entry point collects the global top-k
// for one segment.
func (r *Lucene99HnswVectorsReader) search(
	field string, scorer utilhnsw.RandomVectorScorer, k int, acceptDocs util.Bits,
) (*utilhnsw.TopDocs, error) {
	collector := utilhnsw.NewTopKnnCollector(k, int(^uint(0)>>1), nil)
	if err := r.searchCollector(field, scorer, collector, acceptDocs); err != nil {
		return nil, err
	}
	return collector.TopDocs(), nil
}

// searchCollector drives an externally supplied KnnCollector with the shared
// HNSW-or-exhaustive decision. It is the collector-driven core that both the
// internal top-k search and the public collector entry points
// ([SearchNearestFloatCollector] / [SearchNearestByteCollector]) reuse.
//
// The collector observes leaf-local document ids (the ordinal is translated
// through scorer.OrdToDoc by the OrdinalTranslatedKnnCollector wrapper). The
// HNSW-vs-exhaustive branch is decided by collector.K(); the collector is
// responsible for any further per-result filtering or diversification (e.g.
// the join package's DiversifyingNearestChildrenKnnCollector groups by parent
// block).
//
// Mirrors Lucene99HnswVectorsReader.search(String, *, KnnCollector,
// AcceptDocs), which passes the caller-owned collector straight through to
// HnswGraphSearcher / the exhaustive fallback.
func (r *Lucene99HnswVectorsReader) searchCollector(
	field string, scorer utilhnsw.RandomVectorScorer, collector utilhnsw.KnnCollector, acceptDocs util.Bits,
) error {
	info := r.fieldInfos.GetByName(field)
	if info == nil {
		return fmt.Errorf("hnsw99 reader: field %q not found", field)
	}
	entry, ok := r.fields[info.Number()]
	if !ok {
		return fmt.Errorf("hnsw99 reader: field %q has no HNSW entry", field)
	}

	numVectors := scorer.MaxOrd()
	k := collector.K()
	if numVectors == 0 || k == 0 {
		return nil
	}

	translated := utilhnsw.NewOrdinalTranslatedKnnCollector(
		collector, utilhnsw.IntToIntFunc(scorer.OrdToDoc),
	)
	acceptedOrds := scorer.GetAcceptOrds(acceptDocs)

	graphSize := entry.size
	if entry.vectorIndexLength == 0 {
		graphSize = 0
	}

	doHnsw := k < numVectors
	if graphSize == 0 {
		doHnsw = false
	}

	if doHnsw {
		graph, err := r.GetGraph(field)
		if err != nil {
			return err
		}
		return utilhnsw.SearchWithCollector(scorer, translated, graph, acceptedOrds)
	}

	// Exhaustive scan: k >= numVectors (or no graph). Score every accepted
	// ordinal and collect. Mirrors the non-HNSW branch in Lucene's search().
	for ord := 0; ord < numVectors; ord++ {
		if acceptedOrds != nil && !acceptedOrds.Get(ord) {
			continue
		}
		score, err := scorer.Score(ord)
		if err != nil {
			return err
		}
		translated.Collect(ord, score)
		translated.IncVisitedCount(1)
	}
	return nil
}

// SearchNearestFloatCollector runs approximate (HNSW) or exhaustive
// nearest-neighbour search for the float32 target against field, driving the
// caller-supplied collector instead of an internally created TopKnnCollector.
//
// This is the collector-driven entry point that lets callers (e.g. the join
// package's DiversifyingChildren KNN queries) plug a custom KnnCollector — the
// DiversifyingNearestChildrenKnnCollector — into the HNSW traversal so the
// graph search itself diversifies by parent block. Mirrors the body of
// Lucene99HnswVectorsReader.search(String, float[], KnnCollector, AcceptDocs).
func (r *Lucene99HnswVectorsReader) SearchNearestFloatCollector(
	field string, target []float32, collector utilhnsw.KnnCollector, acceptDocs util.Bits,
) error {
	scorer, err := r.flatReader.randomVectorScorerFloat(field, target)
	if err != nil {
		return err
	}
	return r.searchCollector(field, scorer, collector, acceptDocs)
}

// SearchNearestByteCollector is the byte analogue of
// [SearchNearestFloatCollector].
func (r *Lucene99HnswVectorsReader) SearchNearestByteCollector(
	field string, target []byte, collector utilhnsw.KnnCollector, acceptDocs util.Bits,
) error {
	scorer, err := r.flatReader.randomVectorScorerByte(field, target)
	if err != nil {
		return err
	}
	return r.searchCollector(field, scorer, collector, acceptDocs)
}

// ---------------------------------------------------------------------------
// offHeapHnswGraph
// ---------------------------------------------------------------------------

// offHeapHnswGraph implements util/hnsw.HnswGraph by reading neighbour lists
// lazily from the .vex file on demand.
//
// Mirrors Lucene99HnswVectorsReader.OffHeapHnswGraph (Java inner class).
type offHeapHnswGraph struct {
	dataIn                     store.IndexInput // slice over vectorIndexOffset..+Length
	nodesByLevel               [][]int          // nil for level 0
	numLevels                  int
	entryNode                  int
	size                       int
	maxConn                    int
	version                    int32
	graphLevelNodeOffsets      *packed.DirectMonotonicReader
	graphLevelNodeIndexOffsets []int64 // cumulative node counts per level

	// current neighbour iteration state
	arcCount         int
	arcUpTo          int
	currentNeighbors []int
}

func newOffHeapHnswGraph(entry *lucene99HnswFieldEntry, vectorIndex store.IndexInput, version int32) (*offHeapHnswGraph, error) {
	dataIn, err := vectorIndex.Slice("graph-data", entry.vectorIndexOffset, entry.vectorIndexLength)
	if err != nil {
		return nil, fmt.Errorf("hnsw99 offHeap: slice graph-data: %w", err)
	}

	// Load DirectMonotonicReader for per-node offsets.
	addrSlice, err := vectorIndex.Slice("graph-addrs", entry.offsetsOffset, entry.offsetsLength)
	if err != nil {
		return nil, fmt.Errorf("hnsw99 offHeap: slice addrs: %w", err)
	}
	// Convert IndexInput to RandomAccessInput; fall back to reading into memory.
	var addrRA packed.RandomAccessInput
	if ra, ok := addrSlice.(packed.RandomAccessInput); ok {
		addrRA = ra
	} else {
		buf := make([]byte, entry.offsetsLength)
		if e := addrSlice.ReadBytes(buf); e != nil {
			return nil, fmt.Errorf("hnsw99 offHeap: read addrs: %w", e)
		}
		addrRA = newByteArrayRandomAccess(buf)
	}
	dmr, err := packed.NewDirectMonotonicReader(entry.offsetsMeta, addrRA)
	if err != nil {
		return nil, fmt.Errorf("hnsw99 offHeap: DirectMonotonicReader: %w", err)
	}

	// Compute per-level cumulative node index offsets.
	levelIndexOffsets := make([]int64, entry.numLevels)
	levelIndexOffsets[0] = 0
	for i := 1; i < entry.numLevels; i++ {
		var prevCount int
		if entry.nodesByLevel[i-1] == nil {
			prevCount = entry.size
		} else {
			prevCount = len(entry.nodesByLevel[i-1])
		}
		levelIndexOffsets[i] = levelIndexOffsets[i-1] + int64(prevCount)
	}

	entryNode := 0
	if entry.numLevels > 1 {
		entryNode = entry.nodesByLevel[entry.numLevels-1][0]
	}

	return &offHeapHnswGraph{
		dataIn:                     dataIn,
		nodesByLevel:               entry.nodesByLevel,
		numLevels:                  entry.numLevels,
		entryNode:                  entryNode,
		size:                       entry.size,
		maxConn:                    entry.M,
		version:                    version,
		graphLevelNodeOffsets:      dmr,
		graphLevelNodeIndexOffsets: levelIndexOffsets,
		currentNeighbors:           make([]int, entry.M*2),
	}, nil
}

// SeekLevel positions the graph cursor at the neighbour list for (level, targetOrd).
// Mirrors OffHeapHnswGraph.seek(int, int).
func (g *offHeapHnswGraph) SeekLevel(level, targetOrd int) error {
	var targetIndex int
	if level == 0 {
		targetIndex = targetOrd
	} else {
		nodes := g.nodesByLevel[level]
		i := sort.SearchInts(nodes, targetOrd)
		if i >= len(nodes) || nodes[i] != targetOrd {
			return fmt.Errorf("hnsw99 offHeap: node %d not found on level %d", targetOrd, level)
		}
		targetIndex = i
	}

	offset, err := g.graphLevelNodeOffsets.Get(int64(targetIndex) + g.graphLevelNodeIndexOffsets[level])
	if err != nil {
		return fmt.Errorf("hnsw99 offHeap: get node offset: %w", err)
	}
	if err := g.dataIn.SetPosition(offset); err != nil {
		return fmt.Errorf("hnsw99 offHeap: seek to offset %d: %w", offset, err)
	}

	arcCountV, err := store.ReadVInt(g.dataIn)
	if err != nil {
		return fmt.Errorf("hnsw99 offHeap: read arcCount: %w", err)
	}
	g.arcCount = int(arcCountV)

	// Grow buffer if needed.
	if g.arcCount > len(g.currentNeighbors) {
		g.currentNeighbors = make([]int, g.arcCount)
	}

	if g.arcCount > 0 {
		if g.version >= lucene99HnswVersionGroupVInt {
			scratch := make([]int32, g.arcCount)
			if err := util.ReadGroupVInts(g.dataIn, scratch, g.arcCount); err != nil {
				return fmt.Errorf("hnsw99 offHeap: ReadGroupVInts: %w", err)
			}
			g.currentNeighbors[0] = int(scratch[0])
			for i := 1; i < g.arcCount; i++ {
				g.currentNeighbors[i] = g.currentNeighbors[i-1] + int(scratch[i])
			}
		} else {
			first, err := store.ReadVInt(g.dataIn)
			if err != nil {
				return fmt.Errorf("hnsw99 offHeap: read first neighbor: %w", err)
			}
			g.currentNeighbors[0] = int(first)
			for i := 1; i < g.arcCount; i++ {
				delta, err := store.ReadVInt(g.dataIn)
				if err != nil {
					return fmt.Errorf("hnsw99 offHeap: read neighbor delta: %w", err)
				}
				g.currentNeighbors[i] = g.currentNeighbors[i-1] + int(delta)
			}
		}
	}
	g.arcUpTo = 0
	return nil
}

// Size returns the number of nodes (vectors) in the graph.
func (g *offHeapHnswGraph) Size() int { return g.size }

// NextNeighbor returns the next neighbour ordinal, or util.NO_MORE_DOCS when exhausted.
func (g *offHeapHnswGraph) NextNeighbor() (int, error) {
	if g.arcUpTo >= g.arcCount {
		return util.NO_MORE_DOCS, nil
	}
	v := g.currentNeighbors[g.arcUpTo]
	g.arcUpTo++
	return v, nil
}

// NeighborCount returns the total number of neighbours for the last seek.
func (g *offHeapHnswGraph) NeighborCount() int { return g.arcCount }

// NumLevels returns the number of levels in the graph.
func (g *offHeapHnswGraph) NumLevels() (int, error) { return g.numLevels, nil }

// MaxConn returns M, the maximum connections per node.
func (g *offHeapHnswGraph) MaxConn() int { return g.maxConn }

// EntryNode returns the entry node ordinal on the top level.
func (g *offHeapHnswGraph) EntryNode() (int, error) { return g.entryNode, nil }

// MaxNodeID returns size-1 (node ordinals are 0-based contiguous integers).
func (g *offHeapHnswGraph) MaxNodeID() int {
	if g.size == 0 {
		return -1
	}
	return g.size - 1
}

// GetNodesOnLevel returns an iterator over node ordinals on the given level.
func (g *offHeapHnswGraph) GetNodesOnLevel(level int) (utilhnsw.NodesIterator, error) {
	if level == 0 {
		return utilhnsw.NewDenseNodesIterator(g.size), nil
	}
	nodes := g.nodesByLevel[level]
	cp := make([]int, len(nodes))
	copy(cp, nodes)
	return utilhnsw.NewArrayNodesIterator(cp), nil
}

// ---------------------------------------------------------------------------
// byteArrayRandomAccess — tiny RandomAccessInput backed by a []byte.
// Used to promote the fallback addr slice to RandomAccessInput for the
// DirectMonotonicReader constructor.
// ---------------------------------------------------------------------------

type byteArrayRandomAccess struct{ b []byte }

func newByteArrayRandomAccess(b []byte) *byteArrayRandomAccess {
	return &byteArrayRandomAccess{b: b}
}

func (r *byteArrayRandomAccess) ReadByteAt(pos int64) (byte, error) {
	return r.b[pos], nil
}
func (r *byteArrayRandomAccess) ReadShortAt(pos int64) (int16, error) {
	v := int16(r.b[pos])<<8 | int16(r.b[pos+1])
	return v, nil
}
func (r *byteArrayRandomAccess) ReadIntAt(pos int64) (int32, error) {
	p := int(pos)
	v := int32(r.b[p])<<24 | int32(r.b[p+1])<<16 | int32(r.b[p+2])<<8 | int32(r.b[p+3])
	return v, nil
}
func (r *byteArrayRandomAccess) ReadLongAt(pos int64) (int64, error) {
	p := int(pos)
	lo, _ := r.ReadIntAt(int64(p + 4))
	hi, _ := r.ReadIntAt(pos)
	return int64(hi)<<32 | int64(uint32(lo)), nil
}

// ---------------------------------------------------------------------------
// doc-keyed vector-values adapters
//
// The flat reader's values are ordinal-keyed (VectorValue(ord)); the codecs
// [FloatVectorValues] / [ByteVectorValues] interfaces and their index-package
// peers are doc-keyed (Get(docID)/GetVector(docID) + NextDoc/Advance/DocID).
//
// For the dense case ord == doc, so the adapter walks the ordinal space as
// the document space. For the sparse case the adapter drives the value's
// DocIndexIterator (an IndexedDISI), whose Index() yields the ordinal for the
// current docID; Get(docID) returns nil for documents that carry no vector,
// matching the index.FloatVectorValues contract.
//
// Both consumers of the doc-keyed Get(docID) accessor (CheckIndex and the
// KNN graph test) scan docIDs strictly ascending, so Get advances the
// internal cursor forward to the requested docID; a request for an earlier
// docID rebuilds the iterator.
// ---------------------------------------------------------------------------

type denseFloatVectorValuesAdapter struct {
	values flatFloatVectorValues
	doc    int

	// iter / iterDoc / iterOrd drive a single forward cursor over the
	// underlying value's DocIndexIterator. For a dense field the iterator
	// yields ord==doc; for a sparse field it yields the true (set) docIDs and
	// iter.Index() yields the matching ordinal. The cursor backs both the
	// iteration surface (NextDoc/Advance/DocID) and the random-access
	// Get(docID) accessor; iter is created lazily on first use.
	iter    utilhnsw.DocIndexIterator
	iterDoc int
	iterOrd int
}

func (a *denseFloatVectorValuesAdapter) Dimension() int { return a.values.Dimension() }
func (a *denseFloatVectorValuesAdapter) Size() int      { return a.values.Size() }
func (a *denseFloatVectorValuesAdapter) DocID() int     { return a.doc }

// GetVector returns the vector for docID, or nil when docID carries no vector
// (sparse). A fresh copy is returned because the underlying buffer is reused
// across calls and callers of the codecs surface may retain the result.
func (a *denseFloatVectorValuesAdapter) GetVector(docID int) ([]float32, error) {
	ord, ok, err := a.ordForDoc(docID)
	if err != nil || !ok {
		return nil, err
	}
	v, err := a.values.VectorValue(ord)
	if err != nil {
		return nil, err
	}
	out := make([]float32, len(v))
	copy(out, v)
	return out, nil
}

// ordForDoc resolves the ordinal of docID, returning ok=false when docID has
// no vector. It advances (or rebuilds) the internal iterator forward to docID.
func (a *denseFloatVectorValuesAdapter) ordForDoc(docID int) (int, bool, error) {
	if a.iter == nil || a.iterDoc > docID {
		a.iter = a.values.Iterator()
		a.iterDoc, a.iterOrd = -1, -1
	}
	for a.iterDoc < docID {
		d, err := a.iter.NextDoc()
		if err != nil {
			return 0, false, err
		}
		if d == util.NO_MORE_DOCS {
			a.iterDoc = util.NO_MORE_DOCS
			return 0, false, nil
		}
		a.iterDoc = d
		a.iterOrd = a.iter.Index()
	}
	if a.iterDoc == docID {
		return a.iterOrd, true, nil
	}
	return 0, false, nil
}

// Get is the index.FloatVectorValues accessor name; it aliases GetVector so
// the adapter satisfies both the codecs and index FloatVectorValues
// interfaces (they differ only in this method's name).
func (a *denseFloatVectorValuesAdapter) Get(docID int) ([]float32, error) {
	return a.GetVector(docID)
}

// NextDoc advances the iteration cursor to the next document that carries a
// vector and returns its true docID (or NO_MORE_DOCS). It drives the
// underlying value's DocIndexIterator, so it is correct for both the dense
// (ord==doc) and sparse (DISI-backed) layouts. Mirrors the docID-yielding
// contract of KnnVectorValues.iterator() in Lucene 10.4.0.
func (a *denseFloatVectorValuesAdapter) NextDoc() (int, error) {
	return a.Advance(a.doc + 1)
}

// Advance positions the iteration cursor on the first document >= target that
// carries a vector and returns that docID (or NO_MORE_DOCS). It drives the
// underlying DocIndexIterator so sparse documents are skipped, matching the
// index.FloatVectorValues contract.
func (a *denseFloatVectorValuesAdapter) Advance(target int) (int, error) {
	if target < 0 {
		target = 0
	}
	if a.iter == nil || a.iterDoc >= target {
		a.iter = a.values.Iterator()
		a.iterDoc, a.iterOrd = -1, -1
	}
	for a.iterDoc < target {
		d, err := a.iter.NextDoc()
		if err != nil {
			return 0, err
		}
		if d == util.NO_MORE_DOCS {
			a.iterDoc = util.NO_MORE_DOCS
			a.doc = util.NO_MORE_DOCS
			return util.NO_MORE_DOCS, nil
		}
		a.iterDoc = d
		a.iterOrd = a.iter.Index()
	}
	a.doc = a.iterDoc
	return a.doc, nil
}

type denseByteVectorValuesAdapter struct {
	values flatByteVectorValues
	doc    int

	// See denseFloatVectorValuesAdapter: a single forward cursor over the
	// underlying value's DocIndexIterator backs both the iteration surface
	// and the random-access Get(docID) accessor.
	iter    utilhnsw.DocIndexIterator
	iterDoc int
	iterOrd int
}

func (a *denseByteVectorValuesAdapter) Dimension() int { return a.values.Dimension() }
func (a *denseByteVectorValuesAdapter) Size() int      { return a.values.Size() }
func (a *denseByteVectorValuesAdapter) DocID() int     { return a.doc }

func (a *denseByteVectorValuesAdapter) GetVector(docID int) ([]byte, error) {
	ord, ok, err := a.ordForDoc(docID)
	if err != nil || !ok {
		return nil, err
	}
	v, err := a.values.VectorValue(ord)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (a *denseByteVectorValuesAdapter) ordForDoc(docID int) (int, bool, error) {
	if a.iter == nil || a.iterDoc > docID {
		a.iter = a.values.Iterator()
		a.iterDoc, a.iterOrd = -1, -1
	}
	for a.iterDoc < docID {
		d, err := a.iter.NextDoc()
		if err != nil {
			return 0, false, err
		}
		if d == util.NO_MORE_DOCS {
			a.iterDoc = util.NO_MORE_DOCS
			return 0, false, nil
		}
		a.iterDoc = d
		a.iterOrd = a.iter.Index()
	}
	if a.iterDoc == docID {
		return a.iterOrd, true, nil
	}
	return 0, false, nil
}

// Get aliases GetVector so the adapter satisfies index.ByteVectorValues.
func (a *denseByteVectorValuesAdapter) Get(docID int) ([]byte, error) {
	return a.GetVector(docID)
}

// NextDoc advances to the next document carrying a vector and returns its true
// docID (or NO_MORE_DOCS). See denseFloatVectorValuesAdapter.NextDoc.
func (a *denseByteVectorValuesAdapter) NextDoc() (int, error) {
	return a.Advance(a.doc + 1)
}

// Advance positions the cursor on the first document >= target carrying a
// vector. See denseFloatVectorValuesAdapter.Advance.
func (a *denseByteVectorValuesAdapter) Advance(target int) (int, error) {
	if target < 0 {
		target = 0
	}
	if a.iter == nil || a.iterDoc >= target {
		a.iter = a.values.Iterator()
		a.iterDoc, a.iterOrd = -1, -1
	}
	for a.iterDoc < target {
		d, err := a.iter.NextDoc()
		if err != nil {
			return 0, err
		}
		if d == util.NO_MORE_DOCS {
			a.iterDoc = util.NO_MORE_DOCS
			a.doc = util.NO_MORE_DOCS
			return util.NO_MORE_DOCS, nil
		}
		a.iterDoc = d
		a.iterOrd = a.iter.Index()
	}
	a.doc = a.iterDoc
	return a.doc, nil
}

// Compile-time guards that the adapters satisfy both the codecs and index
// vector-value interfaces (they differ only in Get vs GetVector).
var (
	_ FloatVectorValues       = (*denseFloatVectorValuesAdapter)(nil)
	_ ByteVectorValues        = (*denseByteVectorValuesAdapter)(nil)
	_ index.FloatVectorValues = (*denseFloatVectorValuesAdapter)(nil)
	_ index.ByteVectorValues  = (*denseByteVectorValuesAdapter)(nil)
)
