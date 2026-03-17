// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// TopChildrenResult represents the top children results for a facet dimension.
// This is used to hold the results of a TopChildren query on a taxonomy facet.
// This is the Go port of Lucene's FacetResult.TopChildrenResult.
type TopChildrenResult struct {
	// Dim is the dimension/facet field name
	Dim string

	// Path is the path for hierarchical facets
	Path []string

	// Value is the total value/count for this result
	Value int64

	// ChildCount is the number of child facets
	ChildCount int

	// LabelValues contains the top children label values
	LabelValues []*LabelAndValue
}

// NewTopChildrenResult creates a new TopChildrenResult for the given dimension.
func NewTopChildrenResult(dim string) *TopChildrenResult {
	return &TopChildrenResult{
		Dim:         dim,
		Path:        make([]string, 0),
		LabelValues: make([]*LabelAndValue, 0),
	}
}

// NewTopChildrenResultWithPath creates a new TopChildrenResult with a hierarchical path.
func NewTopChildrenResultWithPath(dim string, path []string) *TopChildrenResult {
	result := NewTopChildrenResult(dim)
	result.Path = append(result.Path, path...)
	return result
}

// AddLabelValue adds a LabelAndValue to this TopChildrenResult.
func (tcr *TopChildrenResult) AddLabelValue(lv *LabelAndValue) {
	tcr.LabelValues = append(tcr.LabelValues, lv)
}

// GetTotalCount returns the total count of all label values.
func (tcr *TopChildrenResult) GetTotalCount() int64 {
	var total int64
	for _, lv := range tcr.LabelValues {
		total += lv.Value
	}
	return total
}

// Size returns the number of label values in this result.
func (tcr *TopChildrenResult) Size() int {
	return len(tcr.LabelValues)
}

// IsEmpty returns true if there are no label values.
func (tcr *TopChildrenResult) IsEmpty() bool {
	return len(tcr.LabelValues) == 0
}
