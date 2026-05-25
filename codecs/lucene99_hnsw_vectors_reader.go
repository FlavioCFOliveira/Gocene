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
//  1. No FlatVectorsReader / .vec file. The Gocene writer does not write .vec
//     (see writer deviation 1). GetFloatVectorValues and GetByteVectorValues
//     return errors; Search is not yet implemented.
//  2. QuantizedVectorsReader interface not implemented (no ScalarQuantizer
//     support in this sprint).
type Lucene99HnswVectorsReader struct {
	fieldInfos  *index.FieldInfos
	fields      map[int]*lucene99HnswFieldEntry // keyed by field number
	vectorIndex store.IndexInput                // open .vex file
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

// CheckIntegrity verifies the checksum of the .vex file.
func (r *Lucene99HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("hnsw99 reader: closed")
	}
	_, err := ChecksumEntireFile(r.vectorIndex)
	return err
}

// Close releases the .vex file handle.
func (r *Lucene99HnswVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.vectorIndex != nil {
		return r.vectorIndex.Close()
	}
	return nil
}

// GetFloatVectorValues returns the float vectors for the named field.
// Not yet implemented (requires Lucene99FlatVectorsReader / .vec file).
func (r *Lucene99HnswVectorsReader) GetFloatVectorValues(_ string) (FloatVectorValues, error) {
	return nil, errors.New("hnsw99 reader: GetFloatVectorValues not implemented (no .vec file in this sprint)")
}

// GetByteVectorValues returns the byte vectors for the named field.
// Not yet implemented (requires Lucene99FlatVectorsReader / .vec file).
func (r *Lucene99HnswVectorsReader) GetByteVectorValues(_ string) (ByteVectorValues, error) {
	return nil, errors.New("hnsw99 reader: GetByteVectorValues not implemented (no .vec file in this sprint)")
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

// SearchFloat performs approximate nearest-neighbour search against float32 query vector.
// Requires FlatVectorsReader (not yet ported); returns an error if called.
//
// knnCollector and acceptDocs are typed as any to avoid an import cycle with
// codecs/hnsw; concrete types will be restored once the search package
// integration sprint lands.
func (r *Lucene99HnswVectorsReader) SearchFloat(_ string, _ []float32, _ any, _ util.Bits) error {
	return errors.New("hnsw99 reader: SearchFloat not implemented (no FlatVectorsReader in this sprint)")
}

// SearchByte performs approximate nearest-neighbour search against byte query vector.
// Requires FlatVectorsReader (not yet ported); returns an error if called.
func (r *Lucene99HnswVectorsReader) SearchByte(_ string, _ []byte, _ any, _ util.Bits) error {
	return errors.New("hnsw99 reader: SearchByte not implemented (no FlatVectorsReader in this sprint)")
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

	offset := g.graphLevelNodeOffsets.Get(int64(targetIndex) + g.graphLevelNodeIndexOffsets[level])
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
