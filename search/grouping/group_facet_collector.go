// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GroupFacetCollector is the base class for computing grouped facets.
//
// Mirrors the abstract class
// org.apache.lucene.search.grouping.GroupFacetCollector, which extends
// SimpleCollector. Its two abstract members, collect(int) — inherited from
// SimpleCollector — and createSegmentResult(), are supplied by the concrete
// subclass: the first as a method of its own, the second through the
// CreateSegmentResult hook this base dispatches to, which renders Java's
// virtual call.
//
// lucene.experimental
type GroupFacetCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	groupField     string
	facetField     string
	facetPrefix    *util.BytesRef
	segmentResults []SegmentResult

	segmentFacetCounts []int
	segmentTotalCount  int
	startFacetOrd      int
	endFacetOrd        int

	// CreateSegmentResult renders the protected abstract method
	// `SegmentResult createSegmentResult() throws IOException`; the concrete
	// subclass installs its implementation here.
	CreateSegmentResult func() (SegmentResult, error)
}

// NewGroupFacetCollector mirrors the protected constructor
// GroupFacetCollector(String groupField, String facetField, BytesRef facetPrefix).
func NewGroupFacetCollector(groupField, facetField string, facetPrefix *util.BytesRef) *GroupFacetCollector {
	// Outer is left for the concrete subclass to install, since collect(int)
	// is abstract at this level.
	return &GroupFacetCollector{
		groupField:     groupField,
		facetField:     facetField,
		facetPrefix:    facetPrefix,
		segmentResults: make([]SegmentResult, 0),
	}
}

// MergeSegmentResults returns grouped facet results that were computed over
// zero or more segments. Grouped facet counts are merged from zero or more
// segment results.
//
// size is the total number of facets to include, typically offset + limit.
// minCount is the minimum count a facet entry should have to be included in
// the grouped facet result. orderByCount says whether to sort the facet
// entries by facet entry count; when false the facets are sorted
// lexicographically in ascending order.
//
// Mirrors GroupedFacetResult mergeSegmentResults(int, int, boolean).
func (c *GroupFacetCollector) MergeSegmentResults(size, minCount int, orderByCount bool) (*GroupedFacetResult, error) {
	totalCount := 0
	missingCount := 0
	segments, err := newSegmentResultPriorityQueue(len(c.segmentResults))
	if err != nil {
		return nil, err
	}
	for _, segmentResult := range c.segmentResults {
		missingCount += segmentResult.Missing()
		if segmentResult.MergePos() >= segmentResult.MaxTermPos() {
			continue
		}
		totalCount += segmentResult.Total()
		segments.Add(segmentResult)
	}

	facetResult := NewGroupedFacetResult(size, minCount, orderByCount, totalCount, missingCount)
	for segments.Size() > 0 {
		segmentResult := segments.Top()
		currentFacetValue := util.BytesRefDeepCopyOf(segmentResult.MergeTerm())
		count := 0

		for {
			count += segmentResult.Counts()[segmentResult.MergePos()]
			segmentResult.SetMergePos(segmentResult.MergePos() + 1)
			if segmentResult.MergePos() < segmentResult.MaxTermPos() {
				if err := segmentResult.NextTerm(); err != nil {
					return nil, err
				}
				// Java's PriorityQueue.updateTop() returns the new top;
				// Gocene's returns nothing, so the value is read back.
				segments.UpdateTop()
				segmentResult = segments.Top()
			} else {
				segments.Pop()
				if segments.Size() == 0 {
					segmentResult = nil
					break
				}
				segmentResult = segments.Top()
			}
			if !bytesRefEquals(currentFacetValue, segmentResult.MergeTerm()) {
				break
			}
		}
		facetResult.AddFacetCount(currentFacetValue, count)
	}
	return facetResult, nil
}

// Finish mirrors GroupFacetCollector.finish().
func (c *GroupFacetCollector) Finish() error {
	segmentResult, err := c.CreateSegmentResult()
	if err != nil {
		return err
	}
	c.segmentResults = append(c.segmentResults, segmentResult)
	c.segmentFacetCounts = nil
	return nil
}

// SetScorer mirrors GroupFacetCollector.setScorer(Scorable), whose body is
// empty.
func (c *GroupFacetCollector) SetScorer(scorer search.Scorable) error {
	return nil
}

// ScoreMode mirrors GroupFacetCollector.scoreMode().
func (c *GroupFacetCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// GroupedFacetResult is the grouped facet result, containing grouped facet
// entries, total count and total missing count.
//
// Mirrors the nested public static class
// GroupFacetCollector.GroupedFacetResult.
type GroupedFacetResult struct {
	maxSize           int
	facetEntries      *treeSet[*FacetEntry]
	totalMissingCount int
	totalCount        int

	currentMin int
}

// NewGroupedFacetResult mirrors
// GroupedFacetResult(int size, int minCount, boolean orderByCount, int totalCount, int totalMissingCount).
func NewGroupedFacetResult(size, minCount int, orderByCount bool, totalCount, totalMissingCount int) *GroupedFacetResult {
	var cmp func(a, b *FacetEntry) int
	if orderByCount {
		cmp = func(a, b *FacetEntry) int {
			c := b.Count - a.Count // Highest count first!
			if c != 0 {
				return c
			}
			return a.Value.BytesRefCompareTo(b.Value)
		}
	} else {
		cmp = func(a, b *FacetEntry) int {
			return a.Value.BytesRefCompareTo(b.Value)
		}
	}
	return &GroupedFacetResult{
		facetEntries:      newTreeSet(cmp),
		totalMissingCount: totalMissingCount,
		totalCount:        totalCount,
		maxSize:           size,
		currentMin:        minCount,
	}
}

// AddFacetCount mirrors void addFacetCount(BytesRef facetValue, int count).
func (r *GroupedFacetResult) AddFacetCount(facetValue *util.BytesRef, count int) {
	if count < r.currentMin {
		return
	}

	facetEntry := NewFacetEntry(facetValue, count)
	if r.facetEntries.size() == r.maxSize {
		if _, ok := r.facetEntries.higher(facetEntry); !ok {
			return
		}
		r.facetEntries.pollLast()
	}
	r.facetEntries.add(facetEntry)

	if r.facetEntries.size() == r.maxSize {
		r.currentMin = r.facetEntries.last().Count
	}
}

// GetFacetEntries returns a list of facet entries to be rendered based on the
// specified offset and limit. The facet entries are retrieved from the facet
// entries collected during merging.
//
// Mirrors List<FacetEntry> getFacetEntries(int offset, int limit).
func (r *GroupedFacetResult) GetFacetEntries(offset, limit int) []*FacetEntry {
	if offset >= r.facetEntries.size() {
		return []*FacetEntry{}
	}

	capacity := r.facetEntries.size() - offset
	if limit < capacity {
		capacity = limit
	}
	entries := make([]*FacetEntry, 0, capacity)

	skipped := 0
	included := 0
	for _, facetEntry := range r.facetEntries.values() {
		if skipped < offset {
			skipped++
			continue
		}
		if included >= limit {
			break
		}
		included++
		entries = append(entries, facetEntry)
	}
	return entries
}

// GetTotalCount returns the sum of all facet entries counts.
//
// Mirrors int getTotalCount().
func (r *GroupedFacetResult) GetTotalCount() int {
	return r.totalCount
}

// GetTotalMissingCount returns the number of groups that didn't have a facet
// value.
//
// Mirrors int getTotalMissingCount().
func (r *GroupedFacetResult) GetTotalMissingCount() int {
	return r.totalMissingCount
}

// FacetEntry represents a facet entry with a value and a count.
//
// Mirrors the nested record GroupFacetCollector.FacetEntry(BytesRef value,
// int count). A Java record component is both a field and an accessor of the
// same name, which Go forbids, so the components are rendered as exported
// fields.
type FacetEntry struct {
	// Value is the facet value.
	Value *util.BytesRef
	// Count is the number of groups carrying the value.
	Count int
}

// NewFacetEntry mirrors the canonical constructor of the record FacetEntry.
func NewFacetEntry(value *util.BytesRef, count int) *FacetEntry {
	return &FacetEntry{Value: value, Count: count}
}

// String mirrors FacetEntry.toString().
func (e *FacetEntry) String() string {
	return fmt.Sprintf("FacetEntry{value=%s, count=%d}", e.Value.Utf8ToString(), e.Count)
}

// SegmentResult contains the local grouped segment counts for a particular
// segment. Each SegmentResult must be added together.
//
// Mirrors the nested protected abstract static class
// GroupFacetCollector.SegmentResult. Java's protected fields are reachable
// through an interface in Go only as methods, so they are rendered as
// accessors.
type SegmentResult interface {
	// Counts renders the protected final field int[] counts.
	Counts() []int

	// Total renders the protected final field int total.
	Total() int

	// Missing renders the protected final field int missing.
	Missing() int

	// MaxTermPos renders the protected final field int maxTermPos.
	MaxTermPos() int

	// MergeTerm renders the protected field BytesRef mergeTerm.
	MergeTerm() *util.BytesRef

	// MergePos renders the protected field int mergePos.
	MergePos() int

	// SetMergePos writes the protected field int mergePos.
	SetMergePos(pos int)

	// NextTerm goes to next term in this SegmentResult in order to retrieve
	// the grouped facet counts.
	NextTerm() error
}

// BaseSegmentResult carries the state of the abstract class
// GroupFacetCollector.SegmentResult.
type BaseSegmentResult struct {
	counts     []int
	total      int
	missing    int
	maxTermPos int

	mergeTerm *util.BytesRef
	mergePos  int
}

// NewBaseSegmentResult mirrors protected SegmentResult(int[] counts, int
// total, int missing, int maxTermPos).
func NewBaseSegmentResult(counts []int, total, missing, maxTermPos int) BaseSegmentResult {
	return BaseSegmentResult{counts: counts, total: total, missing: missing, maxTermPos: maxTermPos}
}

// Counts renders the protected final field counts.
func (s *BaseSegmentResult) Counts() []int { return s.counts }

// Total renders the protected final field total.
func (s *BaseSegmentResult) Total() int { return s.total }

// Missing renders the protected final field missing.
func (s *BaseSegmentResult) Missing() int { return s.missing }

// MaxTermPos renders the protected final field maxTermPos.
func (s *BaseSegmentResult) MaxTermPos() int { return s.maxTermPos }

// MergeTerm renders the protected field mergeTerm.
func (s *BaseSegmentResult) MergeTerm() *util.BytesRef { return s.mergeTerm }

// MergePos renders the protected field mergePos.
func (s *BaseSegmentResult) MergePos() int { return s.mergePos }

// SetMergePos writes the protected field mergePos.
func (s *BaseSegmentResult) SetMergePos(pos int) { s.mergePos = pos }

// segmentResultPriorityQueue mirrors the private static class
// GroupFacetCollector.SegmentResultPriorityQueue.
type segmentResultPriorityQueue struct {
	*util.PriorityQueue[SegmentResult]
}

// newSegmentResultPriorityQueue mirrors SegmentResultPriorityQueue(int maxSize).
func newSegmentResultPriorityQueue(maxSize int) (*segmentResultPriorityQueue, error) {
	pq, err := util.NewPriorityQueue(maxSize, func(a, b SegmentResult) bool {
		return a.MergeTerm().BytesRefCompareTo(b.MergeTerm()) < 0
	})
	if err != nil {
		return nil, err
	}
	return &segmentResultPriorityQueue{PriorityQueue: pq}, nil
}

// bytesRefEquals mirrors BytesRef.equals(Object), which compares the
// [offset, offset+length) windows byte by byte and treats null as unequal to
// any value.
func bytesRefEquals(a, b *util.BytesRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.BytesEqualsRange(b)
}
