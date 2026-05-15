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

// Package hnsw — UpdateGraphsUtils ports utilities used while merging
// segments that contain HNSW graphs. Port of
// org.apache.lucene.util.hnsw.UpdateGraphsUtils (Lucene 10.4.0).

package hnsw

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ComputeJoinSet picks a set of graph nodes that "best cover" the graph —
// a reminiscent variant of the edge cover problem in which nodes (rather
// than edges) are chosen, and a coverage counter is incremented at each of
// the picked node's neighbours. The returned set is used during HNSW
// segment merging to seed the larger graph from the smaller one.
//
// Mirrors UpdateGraphsUtils#computeJoinSet (Lucene 10.4.0) line-for-line.
// The Java reference returns an org.apache.lucene.internal.hppc.IntHashSet;
// the Go port returns map[int]struct{} for idiomatic O(1) Contains and to
// avoid an hppc port. The returned map is owned by the caller.
//
// The algorithm at a glance:
//
//  1. For every node v, compute a target coverage k = max(2, ceil(deg/4))
//     and an initial gain k + deg. Push (-gain, v) into a min-heap so that
//     popping yields the highest-gain node first.
//  2. Pop nodes one by one. If the popped node is "stale" (some neighbour
//     of an already-chosen node touched its counter), recompute its gain
//     and re-push if still positive. Otherwise add it to the join set,
//     credit its neighbours' counters, and mark neighbours stale where
//     appropriate (including a one-hop fan-out so neighbours at distance
//     two are also reconsidered).
//  3. Stop once the cumulative gain crosses the target gExit, or the heap
//     drains.
//
// An empty graph returns an empty join set without consulting the heap;
// LongHeap requires a positive capacity, and the Java reference would
// throw on size == 0, so the early return is a safe Go-side guard.
//
// Returns an error only when the underlying graph's SeekLevel or
// NextNeighbor surface one.
func ComputeJoinSet(graph HnswGraph) (map[int]struct{}, error) {
	size := graph.Size()
	j := make(map[int]struct{})
	if size == 0 {
		return j, nil
	}

	// LongHeap of size `size`. Java's LongHeap defaults to min-heap; the
	// encoder negates the gain so that the popped entries come out in
	// descending order of gain — the priority desired by the algorithm.
	heap := util.NewLongHeapMin(size)
	stale := make([]bool, size)
	counts := make([]int, size)
	var gExit int64
	for v := 0; v < size; v++ {
		if err := graph.SeekLevel(0, v); err != nil {
			return nil, err
		}
		degree := graph.NeighborCount()
		k := coverageTarget(degree)
		gExit += int64(k)
		gain := k + degree
		heap.Push(encode(gain, v))
	}

	var gTot int64
	for gTot < gExit && heap.Size() > 0 {
		el := heap.Pop()
		gain := decodeValue1(el)
		v := decodeValue2(el)
		if err := graph.SeekLevel(0, v); err != nil {
			return nil, err
		}
		degree := graph.NeighborCount()
		// Materialise the neighbour list up-front: the inner loop may
		// re-seek the graph (to fan out to neighbours-of-neighbours),
		// which would clobber the iterator if we tried to stream the
		// outer neighbour list lazily.
		ns := make([]int, 0, degree)
		for {
			u, err := graph.NextNeighbor()
			if err != nil {
				return nil, err
			}
			if u == util.NO_MORE_DOCS {
				break
			}
			ns = append(ns, u)
		}
		k := coverageTarget(degree)
		if stale[v] {
			// Stale: recompute the gain using the latest counts and
			// the current join set. If still positive, re-push.
			newGain := k - counts[v]
			if newGain < 0 {
				newGain = 0
			}
			for _, u := range ns {
				if _, picked := j[u]; counts[u] < k && !picked {
					newGain++
				}
			}
			if newGain > 0 {
				heap.Push(encode(newGain, v))
				stale[v] = false
			}
			continue
		}

		// Fresh pop: commit v to the join set and credit neighbours.
		j[v] = struct{}{}
		gTot += int64(gain)
		markNeighboursStale := counts[v] < k
		for _, u := range ns {
			if markNeighboursStale {
				stale[u] = true
			}
			if counts[u] < (k - 1) {
				// Two-hop staleness: u still needs more credit than
				// the single increment below would provide, so its
				// neighbours must be re-evaluated as well.
				if err := graph.SeekLevel(0, u); err != nil {
					return nil, err
				}
				for {
					uu, err := graph.NextNeighbor()
					if err != nil {
						return nil, err
					}
					if uu == util.NO_MORE_DOCS {
						break
					}
					stale[uu] = true
				}
			}
			counts[u]++
		}
	}
	return j, nil
}

// coverageTarget returns the per-node coverage target k as defined by the
// Java reference: 2 when the degree is small, otherwise ceil(degree/4).
// Java's Math.ceilDiv(degree, 4) for non-negative degree is equivalent to
// the integer expression (degree + 3) / 4.
func coverageTarget(degree int) int {
	if degree < 9 {
		return 2
	}
	return (degree + 3) / 4
}

// encode packs (value1, value2) into a single int64 with value1 in the
// upper 32 bits (negated, so that a min-heap orders by descending value1)
// and value2 in the lower 32 bits (zero-extended into the 32-bit window).
//
// Mirrors UpdateGraphsUtils.encode(int, int). The bit layout is:
//
//	bits 63..32 : -value1 (sign-extended)
//	bits 31..0  : value2 & 0xFFFFFFFF
//
// Because LongHeapMin is a min-heap, the negation in the upper half causes
// Pop() to yield the entry with the largest original value1 first — the
// priority queue semantics required by ComputeJoinSet.
func encode(value1, value2 int) int64 {
	return (int64(-value1) << 32) | (int64(value2) & 0xFFFFFFFF)
}

// decodeValue1 returns the value1 originally encoded via [encode]. The
// Java reference is `(int) -(encoded >> 32)`. Go's arithmetic right
// shift on a signed int64 reproduces Java's `>>` exactly; the
// intermediate int32 conversion mirrors Java's 32-bit `int` truncation
// before widening to platform-int. For the gains produced by
// [ComputeJoinSet] (always non-negative and well within int32 range)
// the truncation is a no-op, but the cast preserves identical semantics
// for any future caller that encodes large value1.
func decodeValue1(encoded int64) int {
	return int(int32(-(encoded >> 32)))
}

// decodeValue2 returns the value2 originally encoded via [encode]. The
// Java reference is `(int) (encoded & 0xFFFFFFFFL)`. The mask keeps only
// the lower 32 bits; the `(int)` cast then re-interprets the masked
// pattern as a 32-bit signed int — meaning the round-trip preserves
// negative value2 inputs as well as non-negative ones. Gocene's
// ComputeJoinSet only encodes non-negative node ordinals, but the helper
// preserves Java's general-purpose semantics for future callers.
func decodeValue2(encoded int64) int {
	return int(int32(encoded & 0xFFFFFFFF))
}
