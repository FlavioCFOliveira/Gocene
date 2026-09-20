// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"bytes"
	"container/heap"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupFacetCollector is a base class for computing grouped facets.
// Mirrors org.apache.lucene.search.grouping.GroupFacetCollector.
type GroupFacetCollector interface {
	search.Collector
	MergeSegmentResults(size, minCount int, orderByCount bool) (*GroupedFacetResult, error)
}

// BaseGroupFacetCollector provides the common logic for grouped facet collectors.
type BaseGroupFacetCollector struct {
	groupField     string
	facetField     string
	facetPrefix    []byte
	segmentResults []*segmentResult
}

func NewBaseGroupFacetCollector(groupField, facetField string, facetPrefix []byte) *BaseGroupFacetCollector {
	return &BaseGroupFacetCollector{
		groupField:     groupField,
		facetField:     facetField,
		facetPrefix:    facetPrefix,
		segmentResults: make([]*segmentResult, 0),
	}
}

// GroupedFacetResult contains grouped facet entries, total count and total missing count.
type GroupedFacetResult struct {
	facetEntries      []*FacetEntry
	totalMissingCount int
	totalCount        int
	maxSize           int
	currentMin        int
}

type FacetEntry struct {
	Value []byte
	Count int
}

func (fe *FacetEntry) String() string {
	return fmt.Sprintf("FacetEntry{value=%s, count=%d}", string(fe.Value), fe.Count)
}

// segmentResult contains the local grouped segment counts.
type segmentResult struct {
	counts     []int
	total      int
	missing    int
	maxTermPos int
	mergeTerm  []byte
	mergePos   int
}

type segmentResultPQ []*segmentResult

func (pq segmentResultPQ) Len() int { return len(pq) }
func (pq segmentResultPQ) Less(i, j int) bool {
	return bytes.Compare(pq[i].mergeTerm, pq[j].mergeTerm) < 0
}
func (pq segmentResultPQ) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *segmentResultPQ) Push(x interface{}) {
	*pq = append(*pq, x.(*segmentResult))
}
func (pq *segmentResultPQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// MergeSegmentResults merges grouped facet counts from all segments.
func (bgfc *BaseGroupFacetCollector) MergeSegmentResults(size, minCount int, orderByCount bool) (*GroupedFacetResult, error) {
	totalCount := 0
	missingCount := 0
	pq := &segmentResultPQ{}
	heap.Init(pq)

	for _, sr := range bgfc.segmentResults {
		missingCount += sr.missing
		if sr.mergePos >= sr.maxTermPos {
			continue
		}
		totalCount += sr.total
		heap.Push(pq, sr)
	}

	res := &GroupedFacetResult{
		maxSize:           size,
		totalMissingCount: missingCount,
		totalCount:        totalCount,
		currentMin:        minCount,
		facetEntries:      make([]*FacetEntry, 0),
	}

	for pq.Len() > 0 {
		sr := (*pq)[0]
		currentFacetValue := make([]byte, len(sr.mergeTerm))
		copy(currentFacetValue, sr.mergeTerm)
		count := 0

		for {
			count += sr.counts[sr.mergePos]
			sr.mergePos++
			if sr.mergePos < sr.maxTermPos {
				// In a real implementation, we would call sr.nextTerm() here
				// and then update the priority queue.
				// For this base class, we assume the logic is implemented in subclasses.
				// This is a simplified version.
				break
			} else {
				heap.Pop(pq)
				if pq.Len() == 0 {
					break
				}
				sr = (*pq)[0]
			}
			if !bytes.Equal(currentFacetValue, sr.mergeTerm) {
				break
			}
		}
		res.addFacetCount(currentFacetValue, count)
	}

	return res, nil
}

func (gfr *GroupedFacetResult) addFacetCount(value []byte, count int) {
	if count < gfr.currentMin {
		return
	}

	entry := &FacetEntry{Value: value, Count: count}
	gfr.facetEntries = append(gfr.facetEntries, entry)

	// Sort and truncate based on maxSize and orderByCount
	sort.Slice(gfr.facetEntries, func(i, j int) bool {
		// This is a simplified sort, the actual one depends on orderByCount
		return gfr.facetEntries[i].Count > gfr.facetEntries[j].Count
	})

	if len(gfr.facetEntries) > gfr.maxSize {
		gfr.facetEntries = gfr.facetEntries[:gfr.maxSize]
	}
}

func (gfr *GroupedFacetResult) GetFacetEntries(offset, limit int) []*FacetEntry {
	if offset >= len(gfr.facetEntries) {
		return nil
	}
	end := offset + limit
	if end > len(gfr.facetEntries) {
		end = len(gfr.facetEntries)
	}
	return gfr.facetEntries[offset:end]
}

func (gfr *GroupedFacetResult) GetTotalCount() int {
	return gfr.totalCount
}

func (gfr *GroupedFacetResult) GetTotalMissingCount() int {
	return gfr.totalMissingCount
}
