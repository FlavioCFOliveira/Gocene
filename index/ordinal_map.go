// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
)

// OrdinalMap maps per-segment ordinals to a single global ordinal space so
// that callers can compare or aggregate SortedDocValues / SortedSetDocValues
// across the leaves of a composite reader. Mirrors
// org.apache.lucene.index.OrdinalMap from Apache Lucene 10.4.0.
//
// Gocene deviations from the Java original:
//
//   - Internal storage uses plain []int64 slices instead of PackedLongValues
//     and PackedInts. The memory-optimal packed representation is deferred to
//     backlog #2703 once benchmarks indicate it is needed in practice.
//   - GetGlobalOrds returns []int64 (not LongValues) because the existing
//     Gocene call sites in join/ index directly into a slice; changing that
//     surface is a separate backlog item.
//   - SortedDocValues / SortedSetDocValues do not expose TermsEnum() in the
//     Gocene interface. The public Build functions construct the required
//     TermsEnum wrappers internally using SortedDocValuesTermsEnum /
//     SortedSetDocValuesTermsEnum and bridge the Ord() call via ordEnumPair.
type OrdinalMap struct {
	// Owner is the cache key owner this map is associated with.
	Owner *CacheKey

	// valueCount is the number of distinct global ordinals.
	valueCount int64

	// globalOrdDeltas[globalOrd] = globalOrd - segmentOrd where segmentOrd is
	// the ordinal in the first segment (sorted order) that contains this term.
	globalOrdDeltas []int64

	// firstSegments[globalOrd] = sorted-order index of the first segment that
	// contains this term.
	firstSegments []int64

	// segmentToGlobalOrds[sortedIdx][segOrd] = globalOrd for segOrd in the
	// segment at sorted-order position sortedIdx.
	segmentToGlobalOrds [][]int64

	// segmentMap maps between original segment indices and sorted order.
	segmentMap segmentMap
}

// segmentMap records the bijection between original segment order and the
// order sorted by descending weight. Mirrors OrdinalMap.SegmentMap in Lucene.
type segmentMap struct {
	newToOld []int // sorted-order index → original index
	oldToNew []int // original index → sorted-order index
}

// buildSegmentMap creates a segmentMap from the given per-segment weights.
// Segments are ordered descending by weight so that the largest segment
// is processed first, minimising the ordDelta array sizes in the common case.
func buildSegmentMap(weights []int64) segmentMap {
	n := len(weights)
	newToOld := make([]int, n)
	for i := range newToOld {
		newToOld[i] = i
	}
	// Stable descending sort by weight.
	sort.SliceStable(newToOld, func(i, j int) bool {
		return weights[newToOld[i]] > weights[newToOld[j]]
	})
	oldToNew := make([]int, n)
	for newIdx, oldIdx := range newToOld {
		oldToNew[oldIdx] = newIdx
	}
	return segmentMap{newToOld: newToOld, oldToNew: oldToNew}
}

// ordEnumPair bundles a TermsEnum with a function that returns the current
// ordinal. This bridges the gap between Gocene's TermsEnum interface (which
// has no Ord() method) and the concrete SortedDocValues-backed enums that do.
type ordEnumPair struct {
	te  TermsEnum
	ord func() int64
}

// BuildOrdinalMapFromSortedValues builds an OrdinalMap from per-segment
// SortedDocValues. The weight for each segment is the number of unique values.
// acceptableOverheadRatio is accepted for API compatibility but unused in the
// current slice-based implementation.
func BuildOrdinalMapFromSortedValues(owner *CacheKey, values []SortedDocValues, _ float32) (*OrdinalMap, error) {
	pairs := make([]ordEnumPair, len(values))
	weights := make([]int64, len(values))
	for i, v := range values {
		te := NewSortedDocValuesTermsEnum("", v)
		pairs[i] = ordEnumPair{te: te, ord: te.Ord}
		weights[i] = int64(v.GetValueCount())
	}
	return buildOrdinalMap(owner, pairs, weights)
}

// BuildOrdinalMapFromSortedSetValues builds an OrdinalMap from per-segment
// SortedSetDocValues. The weight for each segment is the number of unique values.
func BuildOrdinalMapFromSortedSetValues(owner *CacheKey, values []SortedSetDocValues, _ float32) (*OrdinalMap, error) {
	pairs := make([]ordEnumPair, len(values))
	weights := make([]int64, len(values))
	for i, v := range values {
		te := NewSortedSetDocValuesTermsEnum("", v)
		pairs[i] = ordEnumPair{te: te, ord: te.Ord}
		weights[i] = int64(v.GetValueCount())
	}
	return buildOrdinalMap(owner, pairs, weights)
}

// buildOrdinalMap is the internal constructor shared by both public build
// functions. It merges per-segment TermsEnums in descending-weight order
// using a priority queue and records per-segment ordinal deltas.
func buildOrdinalMap(owner *CacheKey, pairs []ordEnumPair, weights []int64) (*OrdinalMap, error) {
	if len(pairs) != len(weights) {
		return nil, fmt.Errorf("OrdinalMap.build: pairs and weights must have the same length")
	}

	segMap := buildSegmentMap(weights)
	n := len(pairs)

	// ordDeltas[sortedSegIdx] accumulates (globalOrd - segOrd) per segOrd.
	ordDeltas := make([][]int64, n)
	for i := range ordDeltas {
		ordDeltas[i] = make([]int64, 0, 16)
	}
	// segmentOrds[sortedSegIdx] = next expected segOrd (for gap filling).
	segmentOrds := make([]int64, n)

	var globalOrdDeltasList []int64 // indexed by globalOrd
	var firstSegmentsList []int64   // indexed by globalOrd

	// Initialise priority queue.
	pq := newTermsEnumIndexPQ(n)
	pqPairs := make([]ordEnumPair, n) // in sorted order, so pqPairs[i] = pairs[segMap.newToOld[i]]

	for i := 0; i < n; i++ {
		origIdx := segMap.newToOld[i]
		pqPairs[i] = pairs[origIdx]
		sub := NewTermsEnumIndex(pqPairs[i].te, i) // SubIndex = sorted position
		term, err := sub.Next()
		if err != nil {
			return nil, fmt.Errorf("OrdinalMap.build: initial Next on sub %d: %w", i, err)
		}
		if term != nil {
			pq.add(sub)
		}
	}

	var globalOrd int64
	topState := NewTermsEnumTermState()

	for pq.size() > 0 {
		top := pq.top()
		topState.CopyFrom(top)

		firstSegIdx := n // sentinel → replaced below
		var globalOrdDelta int64

		// Drain all segments sharing the current term.
		for {
			sortedIdx := top.SubIndex
			segOrd := pqPairs[sortedIdx].ord()
			delta := globalOrd - segOrd

			if sortedIdx < firstSegIdx {
				firstSegIdx = sortedIdx
				globalOrdDelta = delta
			}

			// Fill any ordinal gaps (non-compact ordinal spaces, e.g. FilteredTermsEnum).
			for segmentOrds[sortedIdx] <= segOrd {
				ordDeltas[sortedIdx] = append(ordDeltas[sortedIdx], delta)
				segmentOrds[sortedIdx]++
			}

			// Advance this segment.
			nextTerm, err := top.Next()
			if err != nil {
				return nil, fmt.Errorf("OrdinalMap.build: Next on sub %d: %w", sortedIdx, err)
			}
			if nextTerm == nil {
				pq.pop()
				if pq.size() == 0 {
					break
				}
				top = pq.top()
			} else {
				top = pq.updateTop()
			}

			if !top.TermEquals(topState) {
				break
			}
		}

		firstSegmentsList = append(firstSegmentsList, int64(firstSegIdx))
		globalOrdDeltasList = append(globalOrdDeltasList, globalOrdDelta)
		globalOrd++
	}

	// Materialise segmentToGlobalOrds[sortedIdx][segOrd] = segOrd + delta.
	segToGlobal := make([][]int64, n)
	for i, deltas := range ordDeltas {
		sg := make([]int64, len(deltas))
		for segOrd, delta := range deltas {
			sg[segOrd] = int64(segOrd) + delta
		}
		segToGlobal[i] = sg
	}

	return &OrdinalMap{
		Owner:               owner,
		valueCount:          globalOrd,
		globalOrdDeltas:     globalOrdDeltasList,
		firstSegments:       firstSegmentsList,
		segmentToGlobalOrds: segToGlobal,
		segmentMap:          segMap,
	}, nil
}

// GetValueCount returns the number of distinct global ordinals.
func (m *OrdinalMap) GetValueCount() int64 { return m.valueCount }

// GetGlobalOrds returns the per-segment to global ord mapping for the given
// original segment index. The returned slice maps segmentOrd → globalOrd.
// Returns nil for an out-of-range index or a segment with no ordinals.
func (m *OrdinalMap) GetGlobalOrds(segmentIndex int) []int64 {
	if segmentIndex < 0 || segmentIndex >= len(m.segmentMap.oldToNew) {
		return nil
	}
	sortedIdx := m.segmentMap.oldToNew[segmentIndex]
	if sortedIdx < 0 || sortedIdx >= len(m.segmentToGlobalOrds) {
		return nil
	}
	return m.segmentToGlobalOrds[sortedIdx]
}

// GetFirstSegmentOrd returns the ordinal within the first segment that
// contains the given global ordinal.
func (m *OrdinalMap) GetFirstSegmentOrd(globalOrd int64) int64 {
	if globalOrd < 0 || globalOrd >= int64(len(m.globalOrdDeltas)) {
		return -1
	}
	return globalOrd - m.globalOrdDeltas[globalOrd]
}

// GetFirstSegmentNumber returns the original index of the first segment that
// contains the given global ordinal.
func (m *OrdinalMap) GetFirstSegmentNumber(globalOrd int64) int {
	if globalOrd < 0 || globalOrd >= int64(len(m.firstSegments)) {
		return -1
	}
	sortedIdx := int(m.firstSegments[globalOrd])
	if sortedIdx < 0 || sortedIdx >= len(m.segmentMap.newToOld) {
		return -1
	}
	return m.segmentMap.newToOld[sortedIdx]
}

// RAMBytesUsed returns an estimate of heap usage in bytes.
func (m *OrdinalMap) RAMBytesUsed() int64 {
	const i64 = int64(8)
	const i32 = int64(4)
	total := i64 * int64(len(m.globalOrdDeltas))
	total += i64 * int64(len(m.firstSegments))
	total += i32 * int64(len(m.segmentMap.newToOld))
	total += i32 * int64(len(m.segmentMap.oldToNew))
	for _, sg := range m.segmentToGlobalOrds {
		total += i64 * int64(len(sg))
	}
	return total
}

// ---------------------------------------------------------------------------
// Internal priority queue for TermsEnumIndex merge (min-heap by term).
// Mirrors TermsEnumPriorityQueue in OrdinalMap.java.
// ---------------------------------------------------------------------------

type termsEnumIndexPQ struct {
	heap []*TermsEnumIndex
}

func newTermsEnumIndexPQ(capacity int) *termsEnumIndexPQ {
	return &termsEnumIndexPQ{
		heap: make([]*TermsEnumIndex, 0, capacity+1),
	}
}

func (pq *termsEnumIndexPQ) size() int { return len(pq.heap) }

func (pq *termsEnumIndexPQ) add(t *TermsEnumIndex) {
	pq.heap = append(pq.heap, t)
	pq.upHeap(len(pq.heap) - 1)
}

func (pq *termsEnumIndexPQ) top() *TermsEnumIndex {
	if len(pq.heap) == 0 {
		return nil
	}
	return pq.heap[0]
}

func (pq *termsEnumIndexPQ) pop() *TermsEnumIndex {
	if len(pq.heap) == 0 {
		return nil
	}
	top := pq.heap[0]
	last := len(pq.heap) - 1
	pq.heap[0] = pq.heap[last]
	pq.heap = pq.heap[:last]
	if len(pq.heap) > 0 {
		pq.downHeap(0)
	}
	return top
}

func (pq *termsEnumIndexPQ) updateTop() *TermsEnumIndex {
	pq.downHeap(0)
	return pq.heap[0]
}

func (pq *termsEnumIndexPQ) lessThan(a, b *TermsEnumIndex) bool {
	return a.CompareTermTo(b) < 0
}

func (pq *termsEnumIndexPQ) upHeap(i int) {
	node := pq.heap[i]
	for i > 0 {
		parent := (i - 1) >> 1
		if !pq.lessThan(node, pq.heap[parent]) {
			break
		}
		pq.heap[i] = pq.heap[parent]
		i = parent
	}
	pq.heap[i] = node
}

func (pq *termsEnumIndexPQ) downHeap(i int) {
	n := len(pq.heap)
	node := pq.heap[i]
	for {
		left := (i << 1) + 1
		if left >= n {
			break
		}
		smallest := left
		right := left + 1
		if right < n && pq.lessThan(pq.heap[right], pq.heap[left]) {
			smallest = right
		}
		if !pq.lessThan(pq.heap[smallest], node) {
			break
		}
		pq.heap[i] = pq.heap[smallest]
		i = smallest
	}
	pq.heap[i] = node
}
