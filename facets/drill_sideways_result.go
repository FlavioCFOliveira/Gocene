// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// DrillSidewaysResult represents a single facet result within a drill-sideways operation.
// This holds the count and label information for a specific dimension.
type DrillSidewaysResult struct {
	// Dim is the dimension name
	Dim string

	// Path is the hierarchical path (if applicable)
	Path []string

	// Value is the count/value for this result
	Value int64

	// ChildCount is the number of child facets
	ChildCount int

	// LabelValues contains the top children
	LabelValues []*LabelAndValue
}

// NewDrillSidewaysResult creates a new DrillSidewaysResult.
func NewDrillSidewaysResult(dim string) *DrillSidewaysResult {
	return &DrillSidewaysResult{
		Dim:         dim,
		Path:        make([]string, 0),
		LabelValues: make([]*LabelAndValue, 0),
	}
}

// NewDrillSidewaysResultWithPath creates a new DrillSidewaysResult with a path.
func NewDrillSidewaysResultWithPath(dim string, path []string) *DrillSidewaysResult {
	result := NewDrillSidewaysResult(dim)
	result.Path = append(result.Path, path...)
	return result
}

// AddLabelValue adds a label value to this result.
func (dsr *DrillSidewaysResult) AddLabelValue(lv *LabelAndValue) {
	dsr.LabelValues = append(dsr.LabelValues, lv)
}

// GetTotalCount returns the total count of all label values.
func (dsr *DrillSidewaysResult) GetTotalCount() int64 {
	var total int64
	for _, lv := range dsr.LabelValues {
		total += lv.Value
	}
	return total
}

// Size returns the number of label values.
func (dsr *DrillSidewaysResult) Size() int {
	return len(dsr.LabelValues)
}

// IsEmpty returns true if there are no label values.
func (dsr *DrillSidewaysResult) IsEmpty() bool {
	return len(dsr.LabelValues) == 0
}

// SortByValue sorts the label values by value (descending).
func (dsr *DrillSidewaysResult) SortByValue() {
	sort.Slice(dsr.LabelValues, func(i, j int) bool {
		return dsr.LabelValues[i].Value > dsr.LabelValues[j].Value
	})
}

// SortByLabel sorts the label values by label (ascending).
func (dsr *DrillSidewaysResult) SortByLabel() {
	sort.Slice(dsr.LabelValues, func(i, j int) bool {
		return dsr.LabelValues[i].Label < dsr.LabelValues[j].Label
	})
}

// GetTopN returns the top N label values.
func (dsr *DrillSidewaysResult) GetTopN(n int) []*LabelAndValue {
	if n >= len(dsr.LabelValues) {
		result := make([]*LabelAndValue, len(dsr.LabelValues))
		copy(result, dsr.LabelValues)
		return result
	}
	result := make([]*LabelAndValue, n)
	copy(result, dsr.LabelValues[:n])
	return result
}

// String returns a string representation of this result.
func (dsr *DrillSidewaysResult) String() string {
	return fmt.Sprintf("DrillSidewaysResult{dim=%s, value=%d, childCount=%d, labels=%d}",
		dsr.Dim, dsr.Value, dsr.ChildCount, len(dsr.LabelValues))
}

// DrillSidewaysResults holds all results from a drill-sideways search.
// This is a convenience wrapper around a collection of DrillSidewaysResult.
type DrillSidewaysResults struct {
	// Results contains the results by dimension
	Results map[string]*DrillSidewaysResult

	// TotalHits is the total number of hits
	TotalHits int64
}

// NewDrillSidewaysResults creates a new DrillSidewaysResults.
func NewDrillSidewaysResults() *DrillSidewaysResults {
	return &DrillSidewaysResults{
		Results: make(map[string]*DrillSidewaysResult),
	}
}

// AddResult adds a result for a dimension.
func (dsrs *DrillSidewaysResults) AddResult(dim string, result *DrillSidewaysResult) {
	dsrs.Results[dim] = result
}

// GetResult returns the result for a dimension.
func (dsrs *DrillSidewaysResults) GetResult(dim string) *DrillSidewaysResult {
	return dsrs.Results[dim]
}

// GetDimensions returns all dimension names.
func (dsrs *DrillSidewaysResults) GetDimensions() []string {
	dims := make([]string, 0, len(dsrs.Results))
	for dim := range dsrs.Results {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// IsEmpty returns true if there are no results.
func (dsrs *DrillSidewaysResults) IsEmpty() bool {
	return len(dsrs.Results) == 0
}

// Size returns the number of dimensions with results.
func (dsrs *DrillSidewaysResults) Size() int {
	return len(dsrs.Results)
}

// ToSearchResult converts DrillSidewaysResults to DrillSidewaysSearchResult.
func (dsrs *DrillSidewaysResults) ToSearchResult(hits *search.TopDocs) *DrillSidewaysSearchResult {
	facetResults := make(map[string]*FacetResult)
	for dim, dsr := range dsrs.Results {
		facetResults[dim] = &FacetResult{
			Dim:         dsr.Dim,
			Path:        dsr.Path,
			Value:       dsr.Value,
			ChildCount:  dsr.ChildCount,
			LabelValues: dsr.LabelValues,
		}
	}

	return &DrillSidewaysSearchResult{
		Hits:         hits,
		FacetResults: facetResults,
		HitsCount:    dsrs.TotalHits,
	}
}

// DrillSidewaysResultBuilder helps build DrillSidewaysResult instances.
type DrillSidewaysResultBuilder struct {
	result *DrillSidewaysResult
}

// NewDrillSidewaysResultBuilder creates a new builder.
func NewDrillSidewaysResultBuilder(dim string) *DrillSidewaysResultBuilder {
	return &DrillSidewaysResultBuilder{
		result: NewDrillSidewaysResult(dim),
	}
}

// SetPath sets the path.
func (b *DrillSidewaysResultBuilder) SetPath(path []string) *DrillSidewaysResultBuilder {
	b.result.Path = append([]string{}, path...)
	return b
}

// SetValue sets the value.
func (b *DrillSidewaysResultBuilder) SetValue(value int64) *DrillSidewaysResultBuilder {
	b.result.Value = value
	return b
}

// SetChildCount sets the child count.
func (b *DrillSidewaysResultBuilder) SetChildCount(count int) *DrillSidewaysResultBuilder {
	b.result.ChildCount = count
	return b
}

// AddLabelValue adds a label value.
func (b *DrillSidewaysResultBuilder) AddLabelValue(label string, value int64) *DrillSidewaysResultBuilder {
	b.result.LabelValues = append(b.result.LabelValues, &LabelAndValue{
		Label: label,
		Value: value,
	})
	return b
}

// Build builds and returns the result.
func (b *DrillSidewaysResultBuilder) Build() *DrillSidewaysResult {
	return b.result
}

// DrillSidewaysFacetResult represents a facet result specifically for drill-sideways.
// This extends FacetResult with drill-sideways specific information.
type DrillSidewaysFacetResult struct {
	*FacetResult

	// DrillSidewaysCount is the count when not drilling down on this dimension
	DrillSidewaysCount int64

	// IsDrillDown is true if this dimension is being drilled down
	IsDrillDown bool
}

// NewDrillSidewaysFacetResult creates a new DrillSidewaysFacetResult.
func NewDrillSidewaysFacetResult(dim string) *DrillSidewaysFacetResult {
	return &DrillSidewaysFacetResult{
		FacetResult: NewFacetResult(dim),
	}
}

// SetDrillSidewaysCount sets the drill-sideways count.
func (dsfr *DrillSidewaysFacetResult) SetDrillSidewaysCount(count int64) {
	dsfr.DrillSidewaysCount = count
}

// SetIsDrillDown sets whether this is a drill-down dimension.
func (dsfr *DrillSidewaysFacetResult) SetIsDrillDown(isDrillDown bool) {
	dsfr.IsDrillDown = isDrillDown
}

// GetDrillSidewaysCount returns the drill-sideways count.
func (dsfr *DrillSidewaysFacetResult) GetDrillSidewaysCount() int64 {
	return dsfr.DrillSidewaysCount
}

// GetIsDrillDown returns whether this is a drill-down dimension.
func (dsfr *DrillSidewaysFacetResult) GetIsDrillDown() bool {
	return dsfr.IsDrillDown
}
