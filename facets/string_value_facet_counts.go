// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "sort"

// StringValueFacetCounts aggregates document counts per term ordinal for a
// SortedSet/Sorted DocValues field exposed via a StringDocValuesReaderState.
// Mirrors org.apache.lucene.facet.StringValueFacetCounts.
type StringValueFacetCounts struct {
	state  *StringDocValuesReaderState
	counts []int
	total  int
}

// NewStringValueFacetCounts builds a fresh aggregator over the supplied
// reader state. Counts start at zero; callers feed observed ordinals via
// IncrementOrd.
func NewStringValueFacetCounts(state *StringDocValuesReaderState) *StringValueFacetCounts {
	return &StringValueFacetCounts{
		state:  state,
		counts: make([]int, state.UniqueOrds),
	}
}

// IncrementOrd increases the count for ordinal ord by one. Out-of-range
// ordinals are silently ignored.
func (s *StringValueFacetCounts) IncrementOrd(ord int) {
	if ord < 0 || ord >= len(s.counts) {
		return
	}
	s.counts[ord]++
	s.total++
}

// GetTotalCount returns the sum of all per-ordinal counts.
func (s *StringValueFacetCounts) GetTotalCount() int { return s.total }

// CountForOrd returns the count recorded for ordinal ord.
func (s *StringValueFacetCounts) CountForOrd(ord int) int {
	if ord < 0 || ord >= len(s.counts) {
		return 0
	}
	return s.counts[ord]
}

// GetTopChildren returns the top N (label, count) pairs sorted by descending
// count.
func (s *StringValueFacetCounts) GetTopChildren(topN int) []*LabelAndValue {
	type ordCount struct {
		ord   int
		count int
	}
	entries := make([]ordCount, 0, len(s.counts))
	for i, c := range s.counts {
		if c > 0 {
			entries = append(entries, ordCount{i, c})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return s.state.TermForOrd(entries[i].ord) < s.state.TermForOrd(entries[j].ord)
	})
	if topN > 0 && len(entries) > topN {
		entries = entries[:topN]
	}
	out := make([]*LabelAndValue, len(entries))
	for i, e := range entries {
		out[i] = NewLabelAndValue(s.state.TermForOrd(e.ord), int64(e.count))
	}
	return out
}
