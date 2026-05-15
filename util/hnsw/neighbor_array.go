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
	"math"
	"sort"
)

// NeighborArray encodes the neighbors of a node and their mutual scores
// in the HNSW graph as a pair of growable parallel arrays. Nodes are
// arranged in the sorted order of their scores in descending order
// (if scoresDescOrder is true), or in the ascending order of their
// scores (if scoresDescOrder is false).
//
// This is a port of org.apache.lucene.util.hnsw.NeighborArray
// (Lucene 10.4.0). The Go port preserves the public contract and
// observable behavior of the Java original. Notable differences:
//
//   - The Java implementation relies on hppc's MaxSizedIntArrayList /
//     MaxSizedFloatArrayList for backing storage with a growth ceiling
//     and a RamUsageEstimator-aware listener. The Go port uses plain
//     slices with pre-allocation matched to the same initial capacity
//     hint (maxSize / 8, clamped) and grows linearly up to maxSize.
//   - Java's package-private assertions (which fire only with the
//     -ea JVM flag and yield AssertionError) are reified as panics in
//     Go. They guard programmer-error invariants — calling AddInOrder
//     after AddOutOfOrder, or AddInOrder with a score that breaks the
//     sort order. Tests use recover to verify the contract, matching
//     the Java tests that use expectThrows(AssertionError.class, ...).
//   - Hard capacity violations (size == maxSize) panic with a message
//     mirroring Java's IllegalStateException("No growth is allowed").
//
// NeighborArray is not safe for concurrent use; the comment on
// AddAndEnsureDiversity in the Java source describes the locking
// requirement at the caller boundary.
//
// lucene.internal.
type NeighborArray struct {
	scoresDescOrder bool
	size            int
	maxSize         int
	scores          []float32
	nodes           []int
	sortedNodeSize  int
}

// NewNeighborArray builds an empty NeighborArray bounded by maxSize.
// When descOrder is true, AddInOrder expects strictly non-increasing
// scores; otherwise it expects strictly non-decreasing scores. Mirrors
// the Java constructor NeighborArray(int maxSize, boolean descOrder).
func NewNeighborArray(maxSize int, descOrder bool) *NeighborArray {
	initial := maxSize / 8
	if initial < 0 {
		initial = 0
	}
	if initial > maxSize {
		initial = maxSize
	}
	return &NeighborArray{
		scoresDescOrder: descOrder,
		maxSize:         maxSize,
		nodes:           make([]int, 0, initial),
		scores:          make([]float32, 0, initial),
	}
}

// AddInOrder adds a new node to the NeighborArray. The new node must
// follow the established sort order (non-increasing if descOrder is
// true, non-decreasing otherwise). This cannot be called after
// AddOutOfOrder; doing so panics, mirroring the Java assertion.
func (n *NeighborArray) AddInOrder(newNode int, newScore float32) {
	if n.size != n.sortedNodeSize {
		panic("cannot call AddInOrder after AddOutOfOrder")
	}
	if n.size == n.maxSize {
		panic("No growth is allowed")
	}
	if n.size > 0 {
		previousScore := n.scores[n.size-1]
		var ok bool
		if n.scoresDescOrder {
			ok = previousScore >= newScore
		} else {
			ok = previousScore <= newScore
		}
		if !ok {
			panic(fmt.Sprintf(
				"Nodes are added in the incorrect order! Comparing %v to %v",
				newScore, n.scores))
		}
	}
	n.nodes = append(n.nodes, newNode)
	n.scores = append(n.scores, newScore)
	n.size++
	n.sortedNodeSize++
}

// AddOutOfOrder appends a node/score pair without enforcing sort
// order. The caller is expected to invoke Sort or InsertSorted later
// to restore the ordering invariant. Mirrors Java's addOutOfOrder.
func (n *NeighborArray) AddOutOfOrder(newNode int, newScore float32) {
	if n.size == n.maxSize {
		panic("No growth is allowed")
	}
	n.nodes = append(n.nodes, newNode)
	n.scores = append(n.scores, newScore)
	n.size++
}

// AddAndEnsureDiversity extends AddOutOfOrder by removing the
// least-diverse node when the array is full after insertion. nodeID
// is the owner of this NeighborArray; the scorer is repositioned to
// that ordinal before the diversity check.
//
// In multi-threading environments this method must be locked by the
// caller because it interacts with the scorer; the other add methods
// are single-producer.
//
// Mirrors Java's addAndEnsureDiversity.
func (n *NeighborArray) AddAndEnsureDiversity(
	newNode int, newScore float32, nodeID int, scorer UpdateableRandomVectorScorer,
) error {
	n.AddOutOfOrder(newNode, newScore)
	if n.size < n.maxSize {
		return nil
	}
	if err := scorer.SetScoringOrdinal(nodeID); err != nil {
		return err
	}
	worst, err := n.findWorstNonDiverse(scorer)
	if err != nil {
		return err
	}
	n.RemoveIndex(worst)
	return nil
}

// Sort orders the array by score, returning the sorted (post-sort)
// indexes of previously unsorted nodes — the "unchecked" indexes —
// in ascending order. Returns nil if the array was already fully
// sorted. The scorer is invoked to materialize scores for entries
// whose stored score is NaN; it may be nil when no NaN entries are
// present. Mirrors Java's package-private sort.
func (n *NeighborArray) Sort(scorer RandomVectorScorer) ([]int, error) {
	if n.size == n.sortedNodeSize {
		return nil, nil
	}
	uncheckedIndexes := make([]int, n.size-n.sortedNodeSize)
	count := 0
	for n.sortedNodeSize != n.size {
		ip, err := n.insertSortedInternal(scorer)
		if err != nil {
			return nil, err
		}
		uncheckedIndexes[count] = ip
		for i := 0; i < count; i++ {
			if uncheckedIndexes[i] >= uncheckedIndexes[count] {
				uncheckedIndexes[i]++
			}
		}
		count++
	}
	sort.Ints(uncheckedIndexes)
	return uncheckedIndexes, nil
}

// insertSortedInternal moves the first unsorted node into its sorted
// position. Returns the insertion point. The caller must ensure
// sortedNodeSize < size.
func (n *NeighborArray) insertSortedInternal(scorer RandomVectorScorer) (int, error) {
	if n.sortedNodeSize >= n.size {
		panic("Call this method only when there's an unsorted node")
	}
	tmpNode := n.nodes[n.sortedNodeSize]
	tmpScore := n.scores[n.sortedNodeSize]

	if isNaN32(tmpScore) {
		if scorer == nil {
			return 0, fmt.Errorf("hnsw: NeighborArray.Sort needs a scorer to evaluate NaN score at index %d", n.sortedNodeSize)
		}
		s, err := scorer.Score(tmpNode)
		if err != nil {
			return 0, err
		}
		tmpScore = s
	}

	var insertionPoint int
	if n.scoresDescOrder {
		insertionPoint = n.descSortFindRightMostInsertionPoint(tmpScore, n.sortedNodeSize)
	} else {
		insertionPoint = n.ascSortFindRightMostInsertionPoint(tmpScore, n.sortedNodeSize)
	}

	// Shift [insertionPoint, sortedNodeSize) one slot to the right.
	// This is the System.arraycopy equivalent. We are guaranteed
	// capacity because insertionPoint <= sortedNodeSize <= size-1 and
	// the source/destination overlap is handled correctly by copy on
	// matching slices when copying right-to-left would be needed —
	// but Go's copy is left-to-right, so we use the standard idiom:
	// copy with explicit slicing.
	copy(n.nodes[insertionPoint+1:n.sortedNodeSize+1], n.nodes[insertionPoint:n.sortedNodeSize])
	copy(n.scores[insertionPoint+1:n.sortedNodeSize+1], n.scores[insertionPoint:n.sortedNodeSize])
	n.nodes[insertionPoint] = tmpNode
	n.scores[insertionPoint] = tmpScore
	n.sortedNodeSize++
	return insertionPoint, nil
}

// InsertSorted appends a node out of order and then immediately moves
// it into its sorted position. Exposed for tests, mirroring Java's
// package-private insertSorted.
func (n *NeighborArray) InsertSorted(newNode int, newScore float32) error {
	n.AddOutOfOrder(newNode, newScore)
	_, err := n.insertSortedInternal(nil)
	return err
}

// Size returns the number of stored nodes.
func (n *NeighborArray) Size() int { return n.size }

// MaxSize returns the configured maximum number of nodes.
func (n *NeighborArray) MaxSize() int { return n.maxSize }

// ScoresDescOrder reports whether scores are sorted in descending
// order. Not part of the Java public surface but useful for callers
// that need to interpret the ordering without re-deriving it.
func (n *NeighborArray) ScoresDescOrder() bool { return n.scoresDescOrder }

// Nodes returns the internal slice of node ids. Provided for
// efficient writing of the graph; the caller must treat the slice as
// shared mutable state and read only the first Size() entries.
// Mirrors Java's nodes() which returns the raw buffer.
func (n *NeighborArray) Nodes() []int { return n.nodes }

// GetScore returns the score at index i. The Java method is
// getScores(int); the Go name is GetScore to drop the misleading
// plural and follow Go naming conventions for accessors that return
// a single value.
func (n *NeighborArray) GetScore(i int) float32 { return n.scores[i] }

// Clear resets the array, discarding all entries while retaining the
// underlying capacity. Mirrors Java's clear.
func (n *NeighborArray) Clear() {
	n.size = 0
	n.sortedNodeSize = 0
	n.nodes = n.nodes[:0]
	n.scores = n.scores[:0]
}

// RemoveLast drops the trailing node. Mirrors Java's package-private
// removeLast.
func (n *NeighborArray) RemoveLast() {
	n.nodes = n.nodes[:len(n.nodes)-1]
	n.scores = n.scores[:len(n.scores)-1]
	n.size--
	if n.sortedNodeSize > n.size {
		n.sortedNodeSize = n.size
	}
}

// RemoveIndex removes the entry at idx, shifting subsequent entries
// left. Mirrors Java's package-private removeIndex.
func (n *NeighborArray) RemoveIndex(idx int) {
	if idx == n.size-1 {
		n.RemoveLast()
		return
	}
	copy(n.nodes[idx:n.size-1], n.nodes[idx+1:n.size])
	copy(n.scores[idx:n.size-1], n.scores[idx+1:n.size])
	n.nodes = n.nodes[:n.size-1]
	n.scores = n.scores[:n.size-1]
	if idx < n.sortedNodeSize {
		n.sortedNodeSize--
	}
	n.size--
}

// String implements fmt.Stringer. Mirrors Java's toString format.
func (n *NeighborArray) String() string {
	return fmt.Sprintf("NeighborArray[%d]", n.size)
}

// ascSortFindRightMostInsertionPoint returns the right-most index at
// which newScore can be inserted into the ascending-sorted prefix
// [0, bound) while preserving the sort order (placing duplicates to
// the right of any pre-existing equal scores). Equivalent to Java's
// helper of the same name.
func (n *NeighborArray) ascSortFindRightMostInsertionPoint(newScore float32, bound int) int {
	// Mirror java.util.Arrays.binarySearch(float[], int from, int to,
	// float key). That implementation uses Float.compare semantics,
	// which we reproduce with cmpFloat32 below.
	lo, hi := 0, bound-1
	found := -1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1) // overflow-safe midpoint
		c := cmpFloat32(n.scores[mid], newScore)
		if c < 0 {
			lo = mid + 1
		} else if c > 0 {
			hi = mid - 1
		} else {
			found = mid
			break
		}
	}
	if found >= 0 {
		insertionPoint := found
		for insertionPoint < bound-1 && n.scores[insertionPoint+1] == n.scores[insertionPoint] {
			insertionPoint++
		}
		return insertionPoint + 1
	}
	return lo
}

// descSortFindRightMostInsertionPoint mirrors the Java helper of the
// same name, scanning a descending-sorted prefix.
func (n *NeighborArray) descSortFindRightMostInsertionPoint(newScore float32, bound int) int {
	start, end := 0, bound-1
	for start <= end {
		mid := int(uint(start+end) >> 1)
		if n.scores[mid] < newScore {
			end = mid - 1
		} else {
			start = mid + 1
		}
	}
	return start
}

// findWorstNonDiverse finds the first non-diverse neighbour, scanning
// from the most distant neighbours. Returns the index of the worst
// non-diverse neighbour, or size-1 if none is found.
func (n *NeighborArray) findWorstNonDiverse(scorer UpdateableRandomVectorScorer) (int, error) {
	uncheckedIndexes, err := n.Sort(scorer)
	if err != nil {
		return 0, err
	}
	if uncheckedIndexes == nil {
		panic("findWorstNonDiverse invoked with no unchecked nodes")
	}
	uncheckedCursor := len(uncheckedIndexes) - 1
	for i := n.size - 1; i > 0; i-- {
		if uncheckedCursor < 0 {
			break
		}
		if err := scorer.SetScoringOrdinal(n.nodes[i]); err != nil {
			return 0, err
		}
		bad, err := n.isWorstNonDiverse(i, uncheckedIndexes, uncheckedCursor, scorer)
		if err != nil {
			return 0, err
		}
		if bad {
			return i, nil
		}
		if i == uncheckedIndexes[uncheckedCursor] {
			uncheckedCursor--
		}
	}
	return n.size - 1, nil
}

// isWorstNonDiverse reports whether the candidate at candidateIndex
// violates the diversity invariant relative to the unchecked entries.
func (n *NeighborArray) isWorstNonDiverse(
	candidateIndex int, uncheckedIndexes []int, uncheckedCursor int, scorer RandomVectorScorer,
) (bool, error) {
	minAcceptedSimilarity := n.scores[candidateIndex]
	if candidateIndex == uncheckedIndexes[uncheckedCursor] {
		for i := candidateIndex - 1; i >= 0; i-- {
			s, err := scorer.Score(n.nodes[i])
			if err != nil {
				return false, err
			}
			if s >= minAcceptedSimilarity {
				return true, nil
			}
		}
		return false, nil
	}
	if candidateIndex <= uncheckedIndexes[uncheckedCursor] {
		panic("invariant violated: candidateIndex must exceed unchecked cursor")
	}
	for i := uncheckedCursor; i >= 0; i-- {
		s, err := scorer.Score(n.nodes[uncheckedIndexes[i]])
		if err != nil {
			return false, err
		}
		if s >= minAcceptedSimilarity {
			return true, nil
		}
	}
	return false, nil
}

// isNaN32 returns true when f is NaN. math.IsNaN takes float64; this
// helper avoids the conversion and the allocation of using
// math.IsNaN(float64(f)) in hot paths.
func isNaN32(f float32) bool {
	return f != f
}

// cmpFloat32 mirrors java.lang.Float.compare: NaN is greater than any
// other value (including +Inf) and -0.0 is less than +0.0. This
// matches the semantics of java.util.Arrays.binarySearch(float[],…).
func cmpFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	// Handle NaN and -0.0 / +0.0 via raw bit comparison, replicating
	// Float.floatToIntBits's normalization (every NaN -> canonical NaN
	// bits which are greater than +Inf bits).
	abits := canonicalFloat32Bits(a)
	bbits := canonicalFloat32Bits(b)
	if abits < bbits {
		return -1
	}
	if abits > bbits {
		return 1
	}
	return 0
}

// canonicalFloat32Bits collapses every NaN to the canonical positive
// NaN bit pattern used by Float.floatToIntBits, so comparisons obey
// Java's Float.compare ordering.
func canonicalFloat32Bits(f float32) uint32 {
	if isNaN32(f) {
		return 0x7fc00000
	}
	return math.Float32bits(f)
}
