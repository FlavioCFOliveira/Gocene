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
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// UnknownMaxConn is the sentinel returned by HnswGraph.MaxConn when the
// implementation cannot report the maximum number of connections per node.
// Mirrors org.apache.lucene.util.hnsw.HnswGraph#UNKNOWN_MAX_CONN.
const UnknownMaxConn = -1

// HnswGraph is the abstract API of a Hierarchical Navigable Small World graph,
// providing efficient approximate nearest neighbor search for high dimensional
// vectors. Port of org.apache.lucene.util.hnsw.HnswGraph (Lucene 10.4.0).
//
// Java's abstract class becomes a Go interface here: concrete implementations
// such as OnHeapHnswGraph must implement every method. MaxNodeID is part of
// the interface because non-contiguous-ordinal implementations (e.g.
// OnHeapHnswGraph constructed with a larger capacity than the current
// size) need to override Java's default behaviour of Size() - 1. The other
// default method on the Java reference, getSortedNodes, is exposed as a
// free function [GetSortedNodes] so it may be reused across implementations
// without forcing a concrete embedding type.
//
// The nomenclature mirrors the original paper "Efficient and robust approximate
// nearest neighbor search using Hierarchical Navigable Small World graphs"
// (https://arxiv.org/abs/1603.09320) with one adjustment: HnswGraphBuilder's
// beamWidth has the same role as the paper's efConst — the number of nearest
// neighbor candidates tracked while inserting a new node — while MaxConn
// corresponds to the paper's M, bounding how many of those candidates become
// neighbors of the new node.
//
// Implementations may be searched by multiple goroutines concurrently, but
// updates are not safe for concurrent use. A traversal is performed by calling
// SeekLevel(level, target) and then repeatedly calling NextNeighbor until it
// returns util.NO_MORE_DOCS; calling NextNeighbor again without an intervening
// SeekLevel is illegal.
type HnswGraph interface {
	// SeekLevel moves the pointer to exactly the given level's target node.
	// After SeekLevel returns, callers invoke NextNeighbor to retrieve the
	// successive (ordered) connected node ordinals on that level. target must
	// be a valid node ordinal on the requested level.
	//
	// Mirrors HnswGraph#seek(int, int) in the Java reference.
	SeekLevel(level, target int) error

	// Size returns the number of nodes in the graph.
	Size() int

	// NextNeighbor returns the next neighbor node ordinal in the iteration
	// established by the most recent SeekLevel call, or util.NO_MORE_DOCS
	// when the neighbor list is exhausted. It is illegal to call
	// NextNeighbor again after it has returned util.NO_MORE_DOCS without
	// first calling SeekLevel to reset the iterator.
	NextNeighbor() (int, error)

	// NumLevels returns the number of levels in the graph.
	NumLevels() (int, error)

	// MaxConn returns M, the maximum number of connections per node, or
	// [UnknownMaxConn] when the implementation does not expose this value.
	MaxConn() int

	// EntryNode returns the graph's entry point on the top level.
	EntryNode() (int, error)

	// GetNodesOnLevel returns an iterator over the node ordinals present on
	// the requested level. The returned iterator yields nodes in NO
	// particular order; implementations are free to return any ordering.
	GetNodesOnLevel(level int) (NodesIterator, error)

	// NeighborCount returns the number of neighbors currently positioned by
	// the most recent SeekLevel — i.e. how many ordinals NextNeighbor will
	// yield before returning util.NO_MORE_DOCS, not counting any already
	// consumed.
	NeighborCount() int

	// MaxNodeID returns the maximum node id, inclusive. For
	// implementations that expose contiguous node ordinals this is
	// Size()-1; implementations that allocate slot space ahead of the
	// current Size (e.g. [OnHeapHnswGraph] when constructed with an
	// explicit numNodes) must override this so callers sizing buffers
	// to MaxNodeID()+1 do not underestimate the ordinal range.
	//
	// Mirrors Java's HnswGraph#maxNodeId default method, which serves
	// as a default for non-contiguous implementations to override.
	MaxNodeID() int
}

// MaxNodeID returns the maximum node id, inclusive, by dispatching to the
// graph's own [HnswGraph.MaxNodeID] method.
//
// The free function is preserved as a thin wrapper so call sites that
// existed before MaxNodeID was promoted to a method on the interface do
// not need to change. New code should prefer the method form directly.
func MaxNodeID(g HnswGraph) int {
	return g.MaxNodeID()
}

// GetSortedNodes returns a NodesIterator over the nodes on the requested
// level, presented in ascending ordinal order. For level 0 the iterator
// is a dense iterator over [0, Size()) (every node is on level 0 by
// invariant of the HNSW construction); for higher levels the nodes returned
// by GetNodesOnLevel are materialised into a slice and sorted.
//
// Mirrors HnswGraph#getSortedNodes in the Java reference. The materialise-
// and-sort cost is bound by the number of nodes on the requested level,
// which on the upper levels of an HNSW graph is logarithmic in Size().
func GetSortedNodes(g HnswGraph, level int) (NodesIterator, error) {
	if level == 0 {
		return NewDenseNodesIterator(g.Size()), nil
	}
	nodesOnLevel, err := g.GetNodesOnLevel(level)
	if err != nil {
		return nil, err
	}
	sortedNodes := make([]int, nodesOnLevel.Size())
	for n := 0; nodesOnLevel.HasNext(); n++ {
		sortedNodes[n] = nodesOnLevel.NextInt()
	}
	sort.Ints(sortedNodes)
	return NewArrayNodesIterator(sortedNodes), nil
}

// emptyHnswGraph is the zero-state HnswGraph implementation returned by
// [Empty]. It reports zero size, zero levels, and an empty neighbour list,
// and SeekLevel is a no-op.
type emptyHnswGraph struct{}

// SeekLevel is a no-op on the empty graph.
func (emptyHnswGraph) SeekLevel(level, target int) error { return nil }

// NextNeighbor returns util.NO_MORE_DOCS on the empty graph.
func (emptyHnswGraph) NextNeighbor() (int, error) { return util.NO_MORE_DOCS, nil }

// Size returns 0 on the empty graph.
func (emptyHnswGraph) Size() int { return 0 }

// NumLevels returns 0 on the empty graph.
func (emptyHnswGraph) NumLevels() (int, error) { return 0, nil }

// EntryNode returns 0 on the empty graph.
func (emptyHnswGraph) EntryNode() (int, error) { return 0, nil }

// NeighborCount returns 0 on the empty graph.
func (emptyHnswGraph) NeighborCount() int { return 0 }

// MaxConn returns [UnknownMaxConn] on the empty graph.
func (emptyHnswGraph) MaxConn() int { return UnknownMaxConn }

// MaxNodeID returns -1 on the empty graph: the contiguous-ordinal
// default is Size() - 1 = -1, which correctly signals "no nodes".
func (emptyHnswGraph) MaxNodeID() int { return -1 }

// GetNodesOnLevel returns the empty NodesIterator regardless of the
// requested level.
func (emptyHnswGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	return emptyDenseNodesIterator, nil
}

// emptyGraphSingleton is the singleton zero-state HnswGraph returned by
// [Empty]. It is a value (not a pointer) so the type-zero check used by
// Java's `g == HnswGraph.EMPTY` translates to identity comparison through
// the interface header.
var emptyGraphSingleton HnswGraph = emptyHnswGraph{}

// Empty returns the singleton zero-state HnswGraph, equivalent to
// HnswGraph.EMPTY in the Java reference. The returned value is safe to
// share across goroutines: every method is read-only and produces constant
// output.
func Empty() HnswGraph { return emptyGraphSingleton }

// NodesIterator iterates the node ordinals on a single graph level and also
// reports the total iteration size up-front. Port of HnswGraph.NodesIterator
// (Lucene 10.4.0), itself implementing PrimitiveIterator.OfInt.
//
// The nodes are NOT guaranteed to be presented in any particular order; use
// [GetSortedNodes] when an ascending order is required.
//
// NodesIterator is not safe for concurrent use; iteration state is local to
// the iterator instance.
type NodesIterator interface {
	// HasNext reports whether further node ordinals remain.
	HasNext() bool

	// NextInt returns the next node ordinal. Calling NextInt after HasNext
	// has returned false panics with "NoSuchElementException", mirroring the
	// Java reference behaviour.
	NextInt() int

	// Size reports the total number of node ordinals to be iterated over,
	// counting any already consumed. Implementations expose this so callers
	// can size buffers ahead of full consumption.
	Size() int

	// Consume drains as many ordinals as fit into dest, returning the count
	// actually written. Consume must be called only while HasNext returns
	// true; calling Consume after exhaustion panics with
	// "NoSuchElementException", mirroring the Java reference.
	Consume(dest []int) int
}

// baseNodesIterator captures the size invariant shared by all NodesIterator
// implementations. It is not exported.
type baseNodesIterator struct {
	size int
}

// Size returns the fixed size reported at construction.
func (b *baseNodesIterator) Size() int { return b.size }

// ArrayNodesIterator is a NodesIterator backed by a caller-supplied slice of
// node ordinals. Port of HnswGraph.ArrayNodesIterator (Lucene 10.4.0).
//
// The slice is referenced, not copied, so callers must not mutate it for
// the lifetime of the iterator. The iterator's reported [Size] may be
// smaller than len(nodes) — Lucene exposes the size override solely for
// back-compatibility with on-disk graphs that record a count distinct from
// the materialised array length.
type ArrayNodesIterator struct {
	baseNodesIterator
	nodes []int
	cur   int
}

// NewArrayNodesIterator constructs an ArrayNodesIterator over the entire
// slice. Equivalent to Java's ArrayNodesIterator(int[]).
func NewArrayNodesIterator(nodes []int) *ArrayNodesIterator {
	return NewArrayNodesIteratorWithSize(nodes, len(nodes))
}

// NewArrayNodesIteratorWithSize constructs an ArrayNodesIterator over nodes
// while overriding the reported size. Mirrors Java's two-argument
// constructor ArrayNodesIterator(int[], int), retained solely for
// back-compatibility with on-disk graphs whose recorded node count may
// differ from len(nodes).
func NewArrayNodesIteratorWithSize(nodes []int, size int) *ArrayNodesIterator {
	return &ArrayNodesIterator{
		baseNodesIterator: baseNodesIterator{size: size},
		nodes:             nodes,
	}
}

// HasNext reports whether further ordinals remain.
func (a *ArrayNodesIterator) HasNext() bool { return a.cur < a.size }

// NextInt returns the next ordinal.
func (a *ArrayNodesIterator) NextInt() int {
	if !a.HasNext() {
		panic("NoSuchElementException")
	}
	v := a.nodes[a.cur]
	a.cur++
	return v
}

// Consume drains up to len(dest) ordinals from the iterator into dest and
// returns the number written.
func (a *ArrayNodesIterator) Consume(dest []int) int {
	if !a.HasNext() {
		panic("NoSuchElementException")
	}
	numToCopy := a.size - a.cur
	if len(dest) < numToCopy {
		numToCopy = len(dest)
	}
	copy(dest[:numToCopy], a.nodes[a.cur:a.cur+numToCopy])
	a.cur += numToCopy
	return numToCopy
}

// DenseNodesIterator is a NodesIterator yielding the dense range [0, size).
// Port of HnswGraph.DenseNodesIterator (Lucene 10.4.0).
type DenseNodesIterator struct {
	baseNodesIterator
	cur int
}

// NewDenseNodesIterator constructs a DenseNodesIterator over the range
// [0, size).
func NewDenseNodesIterator(size int) *DenseNodesIterator {
	return &DenseNodesIterator{baseNodesIterator: baseNodesIterator{size: size}}
}

// emptyDenseNodesIterator is the zero-length DenseNodesIterator shared by
// the empty graph; it is a stateful value but its state is immutable
// because HasNext returns false from the first call.
var emptyDenseNodesIterator NodesIterator = NewDenseNodesIterator(0)

// HasNext reports whether further ordinals remain.
func (d *DenseNodesIterator) HasNext() bool { return d.cur < d.size }

// NextInt returns the next ordinal.
func (d *DenseNodesIterator) NextInt() int {
	if !d.HasNext() {
		panic("NoSuchElementException")
	}
	v := d.cur
	d.cur++
	return v
}

// Consume drains up to len(dest) ordinals from the iterator into dest and
// returns the number written.
func (d *DenseNodesIterator) Consume(dest []int) int {
	if !d.HasNext() {
		panic("NoSuchElementException")
	}
	numToCopy := d.size - d.cur
	if len(dest) < numToCopy {
		numToCopy = len(dest)
	}
	for i := 0; i < numToCopy; i++ {
		dest[i] = d.cur + i
	}
	d.cur += numToCopy
	return numToCopy
}

// CollectionNodesIterator is a NodesIterator backed by an arbitrary slice of
// node ordinals that the iterator iterates element-by-element. Port of
// HnswGraph.CollectionNodesIterator (Lucene 10.4.0), whose Java counterpart
// wraps an IntArrayList; the Go port accepts a slice directly since
// Gocene has no IntArrayList type yet.
//
// Functionally CollectionNodesIterator is equivalent to ArrayNodesIterator
// — both iterate over a slice — but the Java reference treats them as
// separate types for source-level fidelity with the hppc collections, so we
// mirror that distinction here.
type CollectionNodesIterator struct {
	baseNodesIterator
	nodes []int
	cur   int
}

// NewCollectionNodesIterator constructs a CollectionNodesIterator over the
// supplied slice.
func NewCollectionNodesIterator(nodes []int) *CollectionNodesIterator {
	return &CollectionNodesIterator{
		baseNodesIterator: baseNodesIterator{size: len(nodes)},
		nodes:             nodes,
	}
}

// HasNext reports whether further ordinals remain.
func (c *CollectionNodesIterator) HasNext() bool { return c.cur < len(c.nodes) }

// NextInt returns the next ordinal.
func (c *CollectionNodesIterator) NextInt() int {
	if !c.HasNext() {
		panic("NoSuchElementException")
	}
	v := c.nodes[c.cur]
	c.cur++
	return v
}

// Consume drains up to len(dest) ordinals from the iterator into dest and
// returns the number written. Mirrors the Java implementation which copies
// element-by-element rather than via a block copy because IntArrayList is
// not contiguous-array-addressable through the Iterator interface.
func (c *CollectionNodesIterator) Consume(dest []int) int {
	if !c.HasNext() {
		panic("NoSuchElementException")
	}
	destIndex := 0
	for c.HasNext() && destIndex < len(dest) {
		dest[destIndex] = c.NextInt()
		destIndex++
	}
	return destIndex
}
