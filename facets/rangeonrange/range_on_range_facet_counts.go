// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package rangeonrange

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// RangeOnRangeFacetCounts is the abstract base for facet counting over
// BinaryRange doc-values fields. Concrete subtypes (LongRangeOnRangeFacetCounts,
// DoubleRangeOnRangeFacetCounts) embed this struct and call Count to populate
// the per-range counters before exposing FacetResult accessors.
//
// Mirrors org.apache.lucene.facet.rangeonrange.RangeOnRangeFacetCounts.
type RangeOnRangeFacetCounts struct {
	// Field is the BinaryRange doc-values field being counted.
	Field string

	// Labels are the range labels in user-specified order.
	Labels []string

	// Counts are the per-range hit counts (same length as Labels).
	Counts []int

	// TotCount is the total number of matching documents (after subtracting
	// documents with no valid range).
	TotCount int
}

// NewRangeOnRangeFacetCounts initialises the base struct for the given field
// and labels. Counts are zeroed; the caller (concrete subtype) is responsible
// for populating them via Count.
func NewRangeOnRangeFacetCounts(field string, labels []string) *RangeOnRangeFacetCounts {
	return &RangeOnRangeFacetCounts{
		Field:  field,
		Labels: labels,
		Counts: make([]int, len(labels)),
	}
}

// validateDimAndPath ensures the caller supplied the correct dimension name and
// an empty path (range facets do not support paths). Mirrors the private
// validateDimAndPathForGetChildren in the Java reference.
func (r *RangeOnRangeFacetCounts) validateDimAndPath(dim string, path []string) error {
	if dim != r.Field {
		return fmt.Errorf("invalid dim %q; should be %q", dim, r.Field)
	}
	if len(path) != 0 {
		return fmt.Errorf("path.length should be 0")
	}
	return nil
}

// GetAllChildren returns every range bucket as a LabelAndValue, preserving
// user-specified order. Mirrors RangeOnRangeFacetCounts.getAllChildren.
func (r *RangeOnRangeFacetCounts) GetAllChildren(dim string, path ...string) (*facets.FacetResult, error) {
	if err := r.validateDimAndPath(dim, path); err != nil {
		return nil, err
	}
	lv := make([]*facets.LabelAndValue, len(r.Counts))
	for i, c := range r.Counts {
		lv[i] = facets.NewLabelAndValue(r.Labels[i], int64(c))
	}
	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(r.TotCount)
	result.ChildCount = len(lv)
	for _, l := range lv {
		result.AddLabelValue(l)
	}
	return result, nil
}

// rangeEntry is used internally by GetTopChildren.
type rangeEntry struct {
	label string
	count int
}

// GetTopChildren returns the top-N range buckets sorted by count descending,
// ties broken by label descending (Java uses label.compareTo which is lex
// ascending for less-than, so label desc here matches the Java heap ordering).
// Mirrors RangeOnRangeFacetCounts.getTopChildren.
func (r *RangeOnRangeFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	if err := r.validateDimAndPath(dim, path); err != nil {
		return nil, err
	}

	var entries []rangeEntry
	for i, c := range r.Counts {
		if c != 0 {
			entries = append(entries, rangeEntry{label: r.Labels[i], count: c})
		}
	}

	childCount := len(entries)

	// Sort: count desc; on tie, label asc (mirrors Java PriorityQueue lessThan
	// which keeps the min-heap with count asc + label desc, so the top-N
	// kept are the highest counts; ties are broken by label asc after pop).
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].label < entries[j].label
	})
	if len(entries) > topN {
		entries = entries[:topN]
	}

	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(r.TotCount)
	result.ChildCount = childCount
	for _, e := range entries {
		result.AddLabelValue(facets.NewLabelAndValue(e.label, int64(e.count)))
	}
	return result, nil
}

// GetSpecificValue is not supported for range-on-range facets.
// Mirrors RangeOnRangeFacetCounts.getSpecificValue throwing UnsupportedOperationException.
func (r *RangeOnRangeFacetCounts) GetSpecificValue(_ string, _ ...string) (int64, error) {
	return 0, fmt.Errorf("getSpecificValue not supported for RangeOnRangeFacetCounts")
}

// GetAllDims returns a single-element slice containing the top-N result for
// the field dimension. Mirrors RangeOnRangeFacetCounts.getAllDims.
func (r *RangeOnRangeFacetCounts) GetAllDims(topN int) ([]*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	fr, err := r.GetTopChildren(topN, r.Field)
	if err != nil {
		return nil, err
	}
	return []*facets.FacetResult{fr}, nil
}
