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

package hnsw

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// onHeapInitSize mirrors Java's OnHeapHnswGraph.INIT_SIZE, the initial
// capacity of the per-node graph slice when no upper bound is supplied.
const onHeapInitSize = 128

// entryNodeState is the immutable pair (node ordinal, top level) used by
// OnHeapHnswGraph to represent the current graph entry point. Mirrors
// Java's private record EntryNode(int node, int level).
//
// Stored behind atomic.Pointer and replaced wholesale so updates remain
// race-free without touching the unrelated per-node neighbour slots.
type entryNodeState struct {
	node  int
	level int
}

// OnHeapHnswGraph is an HnswGraph whose nodes and connections are kept
// entirely in memory. It is the construction-time representation of the
// graph; HnswGraphBuilder writes into it before the graph is flushed to
// disk via the codec writer.
//
// Port of org.apache.lucene.util.hnsw.OnHeapHnswGraph (Lucene 10.4.0).
// The Java reference is a final class extending HnswGraph and implementing
// Accountable; the Go port satisfies the HnswGraph interface and exposes
// RamBytesUsed for util.Accountable parity.
//
// Concurrency contract (mirrors the Java reference):
//   - The graph may be read from many goroutines concurrently.
//   - Entry-node updates (TrySetNewEntryNode, TryPromoteNewEntryNode)
//     use atomic compare-and-swap and are safe for concurrent callers.
//   - Per-node neighbour mutations (AddNode + the returned NeighborArray)
//     are not safe for concurrent writes. Callers must coordinate write
//     access either by partitioning ordinals across producers (as
//     HnswGraphBuilder does) or by external locking.
//   - When numNodes is -1 the underlying graph slice may be reallocated
//     on growth; concurrent searches in that mode are unsafe and the
//     Java reference notes the same restriction.
//
// lucene.internal.
type OnHeapHnswGraph struct {
	// entryNode is an atomic.Pointer so trySet / tryPromote can CAS the
	// entire (node, level) pair atomically. Initial value mirrors Java's
	// new EntryNode(-1, 1).
	entryNode atomic.Pointer[entryNodeState]

	// graph is the two-dimensional storage: graph[nodeID][level] holds
	// the NeighborArray of node `nodeID` on `level`. Mirrors Java's
	// NeighborArray[][] graph field. The outer slice may be reallocated
	// when noGrowth == false; in that mode searches are not concurrent
	// with construction (matches Java's documented contract).
	graph [][]*NeighborArray

	// levelToNodes caches, per level, the list of node IDs present on
	// that level. Built lazily on the first non-zero GetNodesOnLevel
	// call after the graph stops growing, and rebuilt only when size()
	// has changed since the last freeze. Mirrors Java's lazy
	// levelToNodes[] cache.
	levelToNodes [][]int
	// lastFreezeSize records the size at which levelToNodes was last
	// generated; equality with size() lets us skip rebuilds when the
	// graph is unchanged.
	lastFreezeSize int

	// size, nonZeroLevelSize, maxNodeID are atomic counters so trySet /
	// addNode interactions stay race-free. maxNodeID starts at -1 to
	// match Java's AtomicInteger(-1).
	size             atomic.Int32
	nonZeroLevelSize atomic.Int32
	maxNodeID        atomic.Int32

	// nsize, nsize0 are the configured NeighborArray capacities for
	// non-zero levels and level 0 respectively. Derived once at
	// construction from M.
	nsize  int
	nsize0 int

	// noGrowth is true when the constructor was given an explicit
	// numNodes upper bound; in that mode addNode panics on out-of-range
	// ordinals and maxNodeID() returns graph.length - 1 directly
	// (the contract Java relies on for concurrent searches during
	// construction).
	noGrowth bool

	// upto / cur capture the seek-iterator state used by SeekLevel /
	// NextNeighbor / NeighborCount. Not thread-safe; the iterator state
	// is intrinsic to the receiver, matching Java's HnswGraph contract
	// (one cursor per graph instance).
	upto int
	cur  *NeighborArray
}

// shallowHnswGraphRAMBytes is the constant overhead reported by the
// graph itself, mirroring Java's RAM_BYTES_USED =
// RamUsageEstimator.shallowSizeOfInstance(OnHeapHnswGraph.class). The
// figure is a best-effort approximation of the receiver's own memory
// footprint and excludes the variable-size graph slice and per-node
// NeighborArrays.
var shallowHnswGraphRAMBytes = int64(unsafe.Sizeof(OnHeapHnswGraph{}))

// NewOnHeapHnswGraph constructs an OnHeapHnswGraph configured for M
// connections per node and an optional upper bound on node count.
//
// numNodes == -1 enables auto-growth: the graph starts with capacity
// onHeapInitSize and reallocates on demand. In that mode, concurrent
// search-while-build is not safe (the Java reference enforces the same
// restriction by panicking on slice growth when noGrowth is set).
//
// A non-negative numNodes locks the graph at that capacity. AddNode
// panics if a caller attempts to insert a node ordinal >= numNodes.
//
// Mirrors Java's package-private constructor OnHeapHnswGraph(int M, int
// numNodes).
func NewOnHeapHnswGraph(m, numNodes int) *OnHeapHnswGraph {
	g := &OnHeapHnswGraph{
		nsize:    m + 1,
		nsize0:   m*2 + 1,
		noGrowth: numNodes != -1,
	}
	g.entryNode.Store(&entryNodeState{node: -1, level: 1})
	if !g.noGrowth {
		numNodes = onHeapInitSize
	}
	g.graph = make([][]*NeighborArray, numNodes)
	g.maxNodeID.Store(-1)
	return g
}

// GetNeighbors returns the NeighborArray connected to the given node on
// the supplied level. Mirrors Java's public NeighborArray
// getNeighbors(int level, int node).
//
// Panics with a descriptive message when:
//   - node is out of bounds for the current graph slice;
//   - the node has not been added at the requested level;
//   - the slot for (node, level) is nil.
//
// The Java original uses assert statements for the same three checks;
// the Go port converts them to panics so callers can recover in tests
// (mirroring how AssertionError is asserted in the Lucene test suite).
func (g *OnHeapHnswGraph) GetNeighbors(level, node int) *NeighborArray {
	if node >= len(g.graph) {
		panic(fmt.Sprintf("hnsw: node %d out of range (graph length=%d)", node, len(g.graph)))
	}
	if g.graph[node] == nil {
		panic(fmt.Sprintf("hnsw: node %d not added (graph[%d] is nil)", node, node))
	}
	if level >= len(g.graph[node]) {
		panic(fmt.Sprintf(
			"hnsw: level=%d, node has only %d levels", level, len(g.graph[node])))
	}
	if g.graph[node][level] == nil {
		panic(fmt.Sprintf("hnsw: node=%d, level=%d (neighbor array nil)", node, level))
	}
	return g.graph[node][level]
}

// Size returns the number of nodes that have at least one level slot
// allocated — equivalent to Java's size().
func (g *OnHeapHnswGraph) Size() int {
	return int(g.size.Load())
}

// MaxNodeID returns the maximum node ordinal, inclusive. When the
// constructor was passed a non-negative numNodes the value is fixed at
// graph.length - 1 so concurrent searchers can return without paying for
// an atomic read. Otherwise the live atomic counter is returned.
//
// Mirrors Java's @Override int maxNodeId().
func (g *OnHeapHnswGraph) MaxNodeID() int {
	if g.noGrowth {
		return len(g.graph) - 1
	}
	return int(g.maxNodeID.Load())
}

// AddNode adds the given node ordinal on the supplied level. Nodes may
// be inserted out of order, provided every preceding ordinal is added
// eventually (Java's contract verbatim).
//
// Invariants enforced (mirror the Java assert statements):
//   - When the graph was constructed with noGrowth == true and node is
//     past the bound, AddNode panics with the IllegalStateException
//     message Java emits.
//   - Existing nodes must be inserted top-level first; if graph[node]
//     already exists at a smaller level it is grown via util.GrowExact
//     to fit the new top.
//
// Level 0 entries are sized nsize0 (== 2M+1); non-zero levels are sized
// nsize (== M+1). Both differ in capacity but use the same
// descending-score-order NeighborArray (the second constructor argument
// is true to match Java's `new NeighborArray(..., true, listener)`).
func (g *OnHeapHnswGraph) AddNode(level, node int) {
	if node >= len(g.graph) {
		if g.noGrowth {
			panic("hnsw: the graph does not expect to grow when an initial size is given")
		}
		g.graph = growNeighborArraySlice(g.graph, node+1)
	}

	if g.graph[node] != nil && len(g.graph[node]) < level {
		// Java's assert: "node must be inserted from the top level".
		// Java asserts >= level (not > level); the difference matters
		// because a freshly added node with top-level N has slice
		// length N+1, so the next AddNode at level N is legal but at
		// level N+1 it is not (since the top has already passed).
		panic(fmt.Sprintf(
			"hnsw: node %d must be inserted from the top level (current top %d, requested %d)",
			node, len(g.graph[node])-1, level))
	}

	if g.graph[node] == nil {
		g.graph[node] = make([]*NeighborArray, level+1)
		g.size.Add(1)
	} else if len(g.graph[node]) <= level {
		g.graph[node] = util.GrowExact(g.graph[node], level+1)
	}

	if level == 0 {
		g.graph[node][level] = NewNeighborArray(g.nsize0, true)
	} else {
		g.graph[node][level] = NewNeighborArray(g.nsize, true)
		g.nonZeroLevelSize.Add(1)
	}

	// Equivalent to Java's AtomicInteger.accumulateAndGet(node, Math::max).
	for {
		cur := g.maxNodeID.Load()
		if int32(node) <= cur {
			break
		}
		if g.maxNodeID.CompareAndSwap(cur, int32(node)) {
			break
		}
	}
}

// SeekLevel positions the iterator cursor on (level, target). Mirrors
// Java's void seek(int level, int targetNode). The returned error is
// always nil for an in-memory graph; the signature preserves the
// HnswGraph interface contract used by codec-backed implementations.
func (g *OnHeapHnswGraph) SeekLevel(level, target int) error {
	g.cur = g.GetNeighbors(level, target)
	g.upto = -1
	return nil
}

// NeighborCount returns the size of the most recently seeked
// NeighborArray. Returns 0 if no seek has occurred; Java would NPE in
// that scenario, but the Go port prefers a defensive zero so callers
// without a prior seek see a well-defined empty iteration.
func (g *OnHeapHnswGraph) NeighborCount() int {
	if g.cur == nil {
		return 0
	}
	return g.cur.Size()
}

// NextNeighbor returns the next neighbour ordinal in the active seek
// iteration, or util.NO_MORE_DOCS when the cursor is exhausted. The
// returned error is always nil for OnHeapHnswGraph; the signature
// matches the HnswGraph interface used by codec-backed implementations.
func (g *OnHeapHnswGraph) NextNeighbor() (int, error) {
	if g.cur == nil {
		return util.NO_MORE_DOCS, nil
	}
	g.upto++
	if g.upto < g.cur.Size() {
		return g.cur.Nodes()[g.upto], nil
	}
	return util.NO_MORE_DOCS, nil
}

// NumLevels returns the current number of levels — entryNode.level + 1.
// The returned error is always nil; preserves the HnswGraph interface
// shape.
func (g *OnHeapHnswGraph) NumLevels() (int, error) {
	return g.entryNode.Load().level + 1, nil
}

// NodeExistAtLevel reports whether the given node has a NeighborArray on
// the supplied level. Mirrors Java's boolean nodeExistAtLevel(int level,
// int node) — used by HnswGraphBuilder to decide whether to add or just
// connect.
func (g *OnHeapHnswGraph) NodeExistAtLevel(level, node int) bool {
	if node >= len(g.graph) || g.graph[node] == nil {
		return false
	}
	return level < len(g.graph[node])
}

// EntryNode returns the current entry-point node ordinal (the node id
// pinned at the top of the graph). The error return is always nil.
func (g *OnHeapHnswGraph) EntryNode() (int, error) {
	return g.entryNode.Load().node, nil
}

// MaxConn returns M, the maximum number of connections per node on the
// non-zero levels. nsize is M + 1 by construction.
func (g *OnHeapHnswGraph) MaxConn() int {
	return g.nsize - 1
}

// TrySetNewEntryNode atomically sets the graph's entry node when none
// has been recorded yet. Returns true on success, false if another
// goroutine has already published an entry node.
//
// Mirrors Java's boolean trySetNewEntryNode(int node, int level).
func (g *OnHeapHnswGraph) TrySetNewEntryNode(node, level int) bool {
	cur := g.entryNode.Load()
	if cur.node == -1 {
		return g.entryNode.CompareAndSwap(cur, &entryNodeState{node: node, level: level})
	}
	return false
}

// TryPromoteNewEntryNode atomically promotes node to the entry point on
// the supplied level, provided the current entry level matches
// expectOldLevel. The caller specifies expectOldLevel so the CAS
// rejects updates that races with concurrent promotions to an
// unexpected level.
//
// Returns true on a successful promotion. Returns false when the
// current entry-node level differs from expectOldLevel, even if level
// would still be an improvement; the caller is expected to retry with a
// freshly observed level.
//
// level must be strictly greater than expectOldLevel (Java assert).
func (g *OnHeapHnswGraph) TryPromoteNewEntryNode(node, level, expectOldLevel int) bool {
	if level <= expectOldLevel {
		panic(fmt.Sprintf(
			"hnsw: TryPromoteNewEntryNode expects level (%d) > expectOldLevel (%d)",
			level, expectOldLevel))
	}
	cur := g.entryNode.Load()
	if cur.level == expectOldLevel {
		return g.entryNode.CompareAndSwap(cur, &entryNodeState{node: node, level: level})
	}
	return false
}

// GetNodesOnLevel returns the node IDs present on level. Calling this
// method while the graph is still being built is illegal: a mismatch
// between size() and maxNodeID()+1 triggers an IllegalStateException-
// equivalent error, mirroring Java's guard.
//
// Level 0 always returns a dense iterator over [0, Size()) since every
// node is on level 0 by HNSW construction. Higher levels go through the
// cached levelToNodes table, which is rebuilt only when the graph has
// grown since the last call.
//
// WARN: the level > 0 path iterates every level-0 node to materialise
// the cache; do not invoke during graph construction.
func (g *OnHeapHnswGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	if g.Size() != g.MaxNodeID()+1 {
		return nil, fmt.Errorf(
			"hnsw: graph build not complete, size=%d maxNodeId=%d",
			g.Size(), g.MaxNodeID())
	}
	if level == 0 {
		return NewDenseNodesIterator(g.Size()), nil
	}
	g.generateLevelToNodes()
	return NewCollectionNodesIterator(g.levelToNodes[level]), nil
}

// generateLevelToNodes populates the per-level node cache used by
// GetNodesOnLevel. The Java reference uses IntArrayList; the Go port
// uses []int slices because Gocene has no IntArrayList equivalent yet.
//
// The traversal stops once nonNullNode == size(): in graphs that grew
// out-of-order some intermediate ordinals can remain nil; that early
// exit avoids iterating the unused tail.
func (g *OnHeapHnswGraph) generateLevelToNodes() {
	if g.lastFreezeSize == g.Size() {
		return
	}
	maxLevels, _ := g.NumLevels()
	g.levelToNodes = make([][]int, maxLevels)
	for i := 1; i < maxLevels; i++ {
		g.levelToNodes[i] = make([]int, 0)
	}
	nonNullNode := 0
	size := g.Size()
	for node := 0; node < len(g.graph); node++ {
		if g.graph[node] == nil {
			continue
		}
		nonNullNode++
		for i := 1; i < len(g.graph[node]); i++ {
			g.levelToNodes[i] = append(g.levelToNodes[i], node)
		}
		if nonNullNode == size {
			break
		}
	}
	g.lastFreezeSize = size
}

// RamBytesUsed approximates the on-heap memory footprint of the graph.
// Matches Java's @Override long ramBytesUsed() — the figure is
// best-effort, not thread-safe, and intended for indexing-flush
// estimates rather than runtime accounting.
//
// The computation walks every per-node NeighborArray to sum its node
// and score slice capacities; Java's implementation pre-accumulates the
// same figure through a NeighborArray growth listener. Walking lazily
// avoids touching NeighborArray's hot path, which is the trade-off
// permitted by Java's "not threadsafe" disclaimer.
func (g *OnHeapHnswGraph) RamBytesUsed() int64 {
	total := shallowHnswGraphRAMBytes
	total += util.AlignObjectSize(int64(len(g.graph)) * int64(unsafe.Sizeof((*NeighborArray)(nil))))
	for _, levels := range g.graph {
		if levels == nil {
			continue
		}
		total += util.AlignObjectSize(int64(len(levels)) * int64(unsafe.Sizeof((*NeighborArray)(nil))))
		for _, na := range levels {
			if na == nil {
				continue
			}
			total += util.SizeOfIntSlice(na.Nodes())
			// Mirrors the float[] accounting in NeighborArray —
			// scores are private; approximate using the public Size.
			total += util.SizeOfFloat32Slice(make([]float32, 0, na.Size()))
		}
	}
	return total
}

// String formats the graph for debugging, mirroring Java's
// toString format.
func (g *OnHeapHnswGraph) String() string {
	numLevels, _ := g.NumLevels()
	en, _ := g.EntryNode()
	return fmt.Sprintf("OnHeapHnswGraph(size=%d, numLevels=%d, entryNode=%d)",
		g.Size(), numLevels, en)
}

// growNeighborArraySlice grows the outer graph slice to at least
// minLength, using the same exponential heuristic as Java's
// ArrayUtil.grow. The growth target matches util.Oversize so the
// behaviour is consistent across Gocene's port surface.
//
// Unlike util.GrowExact, this helper permits over-allocation: the
// returned slice's length is at least minLength but may be larger to
// avoid quadratic-time growth as nodes are appended one at a time.
func growNeighborArraySlice(s [][]*NeighborArray, minLength int) [][]*NeighborArray {
	if minLength <= len(s) {
		return s
	}
	newLen := util.Oversize(minLength, int(unsafe.Sizeof((*NeighborArray)(nil))))
	if newLen < minLength {
		newLen = minLength
	}
	out := make([][]*NeighborArray, newLen)
	copy(out, s)
	return out
}
