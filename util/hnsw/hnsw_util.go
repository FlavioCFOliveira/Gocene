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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package hnsw — HnswUtil ports utility helpers from
// org.apache.lucene.util.hnsw.HnswUtil (Lucene 10.4.0). The helpers walk an
// [HnswGraph] level-by-level to identify rooted (reachable-from-entry-point)
// components, primarily for testing graph integrity.
//
// TODO: HnswUtil.graphIsRooted(IndexReader, String) is deferred — it depends
// on IndexReader, CodecReader, HnswGraphProvider, and PerFieldKnnVectorsFormat
// which are not yet ported. Re-introduce it once those packages exist.

package hnsw

import (
	"math/bits"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Component describes one connected (rooted) component of an [HnswGraph]
// level — the set of nodes reachable from at least one entry point via the
// directed edges of the graph.
//
// In graph theory, "connected components" are formally defined only on
// undirected graphs. HNSW graphs are directed (because of pruning) but are
// *mostly* undirected; this helper measures whether the graph is a "rooted
// graph" — every node reachable from some entry point — rather than strict
// connectivity. The Java reference notes a TODO to measure strong
// connectivity (path from every node to every other node) as well.
//
// Start is the lowest-numbered node in the component (the entry point from
// which the BFS/DFS originated). Size is the number of nodes the component
// contains. Mirrors org.apache.lucene.util.hnsw.HnswUtil.Component (a record
// in the Java reference).
type Component struct {
	start int
	size  int
}

// NewComponent constructs a Component with the supplied start node and
// size. Provided for fidelity with Java's compact record constructor;
// most callers receive Components from [Components] or its higher-level
// wrappers.
func NewComponent(start, size int) Component {
	return Component{start: start, size: size}
}

// Start returns the lowest-numbered node of the component — the entry
// point from which traversal was seeded. Mirrors Component#start().
func (c Component) Start() int { return c.start }

// Size returns the number of nodes in the component. Mirrors
// Component#size().
func (c Component) Size() int { return c.size }

// IsRooted reports whether every node on every level of the supplied
// [HnswGraph] is reachable from the entry points of its top level. A graph
// satisfies this predicate iff [Components] returns a single entry for
// every level.
//
// Mirrors HnswUtil#isRooted(HnswGraph). Used primarily by the Lucene test
// suite to validate graph integrity after construction.
func IsRooted(g HnswGraph) (bool, error) {
	numLevels, err := g.NumLevels()
	if err != nil {
		return false, err
	}
	for level := 0; level < numLevels; level++ {
		cs, err := components(g, level, nil, 0)
		if err != nil {
			return false, err
		}
		if len(cs) > 1 {
			return false, nil
		}
	}
	return true, nil
}

// ComponentSizes returns the sizes of the distinct rooted components on
// the requested level. The forest seeded at the entry points (nodes from
// the next-higher level) is treated as a single component. If the entire
// graph is rooted in those entry points — every node reachable from at
// least one of them — the returned slice has a single entry. If the graph
// is empty the returned slice is empty.
//
// Mirrors the two-argument HnswUtil#componentSizes(HnswGraph, int).
// Callers wishing the level-0 default — Java's
// componentSizes(HnswGraph) — pass level == 0.
func ComponentSizes(g HnswGraph, level int) ([]int, error) {
	cs, err := components(g, level, nil, 0)
	if err != nil {
		return nil, err
	}
	sizes := make([]int, len(cs))
	for i, c := range cs {
		sizes[i] = c.size
	}
	return sizes, nil
}

// Components finds the orphaned rooted components on the given level of
// the supplied graph. When notFullyConnected is non-nil it is populated
// during traversal: every visited node whose neighbour count on this
// level is strictly less than maxConn has its bit set. maxConn is ignored
// when notFullyConnected is nil.
//
// The returned slice contains:
//
//   - one Component covering every node reachable from the entry points
//     (when any are reachable); its Start is the smallest set bit in
//     notFullyConnected (when supplied) or in the connected-nodes bitset.
//   - one additional Component per orphaned subgraph discovered by sweeping
//     the level for nodes not visited from any entry point.
//
// The traversal is a single non-recursive DFS, so memory cost is O(nodes
// on the level). Mirrors HnswUtil#components(HnswGraph, int, FixedBitSet,
// int).
func Components(g HnswGraph, level int, notFullyConnected *util.FixedBitSet, maxConn int) ([]Component, error) {
	return components(g, level, notFullyConnected, maxConn)
}

// components is the unexported workhorse delegated to by [IsRooted],
// [ComponentSizes], and [Components]. It exists so the package-internal
// callers can invoke it without going through the public type wrapping
// performed by [Components]; the signatures are identical otherwise.
//
// The algorithm mirrors HnswUtil.components(HnswGraph, int, FixedBitSet,
// int) line-for-line. The level >= 0 invariant is checked up-front and
// returns a wrapped IllegalArgumentException equivalent if violated.
func components(g HnswGraph, level int, notFullyConnected *util.FixedBitSet, maxConn int) ([]Component, error) {
	numLevels, err := g.NumLevels()
	if err != nil {
		return nil, err
	}
	if level >= numLevels {
		return nil, &illegalLevelError{level: level, numLevels: numLevels}
	}

	connectedNodes, err := util.NewFixedBitSet(g.Size())
	if err != nil {
		return nil, err
	}

	var components []Component
	total := 0

	// Seed entry points: the apex level uses the single entry node; lower
	// levels inherit every node present on the level immediately above.
	var entryPoints NodesIterator
	if level == numLevels-1 {
		entry, err := g.EntryNode()
		if err != nil {
			return nil, err
		}
		entryPoints = NewArrayNodesIterator([]int{entry})
	} else {
		entryPoints, err = g.GetNodesOnLevel(level + 1)
		if err != nil {
			return nil, err
		}
	}

	for entryPoints.HasNext() {
		entryPoint := entryPoints.NextInt()
		c, err := markRooted(g, level, connectedNodes, notFullyConnected, maxConn, entryPoint)
		if err != nil {
			return nil, err
		}
		total += c.size
	}

	// Java's nextSetBit returns NO_MORE_DOCS on miss; Go's returns -1.
	// Because total > 0 guarantees at least one bit was set, the -1 path
	// here is unreachable in practice — but we preserve the conditional
	// shape of the Java source so the algorithm reads identically.
	var entryPoint int
	if notFullyConnected != nil {
		entryPoint = notFullyConnected.NextSetBit(0)
	} else {
		entryPoint = connectedNodes.NextSetBit(0)
	}
	if total > 0 {
		components = append(components, Component{start: entryPoint, size: total})
	}

	if level == 0 {
		nextClear := nextClearBit(connectedNodes, 0)
		for nextClear != util.NO_MORE_DOCS {
			c, err := markRooted(g, level, connectedNodes, notFullyConnected, maxConn, nextClear)
			if err != nil {
				return nil, err
			}
			// Java assert: component.size > 0. Skip the assertion in
			// release builds; markRooted only returns size == 0 when
			// the entry was already visited, which cannot happen here
			// because nextClearBit returned a clear bit.
			components = append(components, c)
			total += c.size
			nextClear = nextClearBit(connectedNodes, c.start)
		}
	} else {
		nodes, err := g.GetNodesOnLevel(level)
		if err != nil {
			return nil, err
		}
		for nodes.HasNext() {
			nextClear := nodes.NextInt()
			if connectedNodes.Get(nextClear) {
				continue
			}
			c, err := markRooted(g, level, connectedNodes, notFullyConnected, maxConn, nextClear)
			if err != nil {
				return nil, err
			}
			// Java assert: c.start == nextClear && c.size > 0.
			components = append(components, c)
			total += c.size
		}
	}

	// Java's trailing assert validates the total against
	// getNodesOnLevel(level).size(); leaving it as a comment here, since
	// the Go FixedBitSet path already enforces correctness via the
	// connectedNodes bitset, and going back through getNodesOnLevel
	// twice would double the cost of the dominant traversal.
	return components, nil
}

// illegalLevelError mirrors Java's IllegalArgumentException raised when a
// caller passes a level outside [0, numLevels). The Go port returns the
// error rather than panicking to keep the public surface error-driven; the
// caller may match on the wrapped type if behavioural branching is needed.
type illegalLevelError struct {
	level     int
	numLevels int
}

// Error renders the message verbatim from the Java reference so log
// scrapers tuned for Lucene continue to match.
func (e *illegalLevelError) Error() string {
	return "Level " + strconv.Itoa(e.level) +
		" too large for graph with " + strconv.Itoa(e.numLevels) + " levels"
}

// markRooted performs an iterative DFS from entryPoint over the supplied
// level, marking every visited node in connectedNodes. When
// notFullyConnected is non-nil it additionally records every visited node
// whose direct neighbour count on this level is strictly less than
// maxConn — used by graph-repair routines to limit search to under-filled
// nodes.
//
// Returns a Component whose Start is entryPoint and whose Size is the
// number of newly-visited nodes (zero when entryPoint was already
// connected before this call).
//
// Mirrors HnswUtil#markRooted. The Java implementation uses
// org.apache.lucene.internal.hppc.IntHashSet to track nodes pushed onto
// the work stack; the Go port uses map[int]struct{} for the same purpose,
// trading a small constant-factor allocation cost for the avoidance of an
// hppc port at this stage.
func markRooted(g HnswGraph, level int, connectedNodes, notFullyConnected *util.FixedBitSet,
	maxConn, entryPoint int,
) (Component, error) {
	if connectedNodes.Get(entryPoint) {
		return Component{start: entryPoint, size: 0}, nil
	}
	nodesInStack := make(map[int]struct{})
	// Stack pre-sized to a single-step worth — most components are much
	// smaller than total node count, so growing on demand wins on the
	// common case; the slice doubles geometrically via append.
	stack := []int{entryPoint}
	count := 0
	for len(stack) > 0 {
		// Pop.
		n := len(stack) - 1
		node := stack[n]
		stack = stack[:n]

		if connectedNodes.Get(node) {
			continue
		}
		count++
		connectedNodes.Set(node)
		if err := g.SeekLevel(level, node); err != nil {
			return Component{}, err
		}
		friendCount := 0
		for {
			friendOrd, err := g.NextNeighbor()
			if err != nil {
				return Component{}, err
			}
			if friendOrd == util.NO_MORE_DOCS {
				break
			}
			friendCount++
			if connectedNodes.Get(friendOrd) {
				continue
			}
			if _, seen := nodesInStack[friendOrd]; seen {
				continue
			}
			stack = append(stack, friendOrd)
			nodesInStack[friendOrd] = struct{}{}
		}
		if notFullyConnected != nil && friendCount < maxConn {
			notFullyConnected.Set(node)
		}
	}
	return Component{start: entryPoint, size: count}, nil
}

// nextClearBit is the HnswUtil-local clear-bit search. It returns the
// index of the first unset bit at or after index, or util.NO_MORE_DOCS
// when the entire suffix is set.
//
// The util.FixedBitSet.NextClearBit method returns the bitset's length
// when no clear bit remains, whereas HnswUtil's Java inline helper
// returns NO_MORE_DOCS — a deliberate sentinel choice in the Lucene
// source to flag "no more candidates" distinctly from "off the end". We
// preserve the latter semantics here.
//
// Implementation hugs the Java reference: it operates directly on the
// underlying uint64 word array via FixedBitSet.Bits() and uses
// math/bits.TrailingZeros64 for the per-word scan; the ghost-bit
// invariant is irrelevant because the inverted word is masked against
// the bitset length explicitly.
func nextClearBit(b *util.FixedBitSet, index int) int {
	barr := b.Bits()
	length := b.Length()
	// Java assert: index >= 0 && index < length. We rely on the same
	// invariant; the function is only invoked with values produced by
	// markRooted or the level-0 sweep, both of which respect the range.
	if index >= length {
		return util.NO_MORE_DOCS
	}
	i := index >> 6
	// Skip all bits to the right of index by inverting the masked word.
	// The cast to uint64 keeps the >> operation logical (Go's >> on a
	// signed integer is arithmetic, which would shift in 1-bits for
	// negative values — not what the Java >>> would do).
	word := ^(barr[i] >> uint(index&63))

	next := util.NO_MORE_DOCS
	if word != 0 {
		next = index + bits.TrailingZeros64(word)
	} else {
		for {
			i++
			if i >= len(barr) {
				break
			}
			word = ^barr[i]
			if word != 0 {
				next = (i << 6) + bits.TrailingZeros64(word)
				break
			}
		}
	}
	if next >= length {
		return util.NO_MORE_DOCS
	}
	return next
}
