// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"
)

// Range represents a range with a label.
type Range struct {
	Label        string
	Min          interface{}
	Max          interface{}
	MinInclusive bool
	MaxInclusive bool
}

// NewRange creates a new Range.
func NewRange(label string, min, max interface{}) Range {
	return Range{
		Label:        label,
		Min:          min,
		Max:          max,
		MinInclusive: true,
		MaxInclusive: true,
	}
}

// NewRangeExclusive creates a new Range with exclusive bounds.
func NewRangeExclusive(label string, min, max interface{}) Range {
	return Range{
		Label:        label,
		Min:          min,
		Max:          max,
		MinInclusive: false,
		MaxInclusive: false,
	}
}

// Contains checks if a value is within this range.
func (r Range) Contains(value interface{}) bool {
	// Type-specific comparisons
	switch v := value.(type) {
	case int64:
		return r.containsInt64(v)
	case float64:
		return r.containsFloat64(v)
	case int:
		return r.containsInt64(int64(v))
	default:
		return false
	}
}

func (r Range) containsInt64(value int64) bool {
	minVal, minOk := r.Min.(int64)
	maxVal, maxOk := r.Max.(int64)

	if minOk {
		if r.MinInclusive {
			if value < minVal {
				return false
			}
		} else {
			if value <= minVal {
				return false
			}
		}
	}

	if maxOk {
		if r.MaxInclusive {
			if value > maxVal {
				return false
			}
		} else {
			if value >= maxVal {
				return false
			}
		}
	}

	return true
}

func (r Range) containsFloat64(value float64) bool {
	minVal, minOk := r.Min.(float64)
	maxVal, maxOk := r.Max.(float64)

	if minOk {
		if r.MinInclusive {
			if value < minVal {
				return false
			}
		} else {
			if value <= minVal {
				return false
			}
		}
	}

	if maxOk {
		if r.MaxInclusive {
			if value > maxVal {
				return false
			}
		} else {
			if value >= maxVal {
				return false
			}
		}
	}

	return true
}

// String returns a string representation of this range.
func (r Range) String() string {
	minStr := "*"
	if r.Min != nil {
		minStr = fmt.Sprintf("%v", r.Min)
	}
	maxStr := "*"
	if r.Max != nil {
		maxStr = fmt.Sprintf("%v", r.Max)
	}

	minBracket := "["
	if !r.MinInclusive {
		minBracket = "{"
	}
	maxBracket := "]"
	if !r.MaxInclusive {
		maxBracket = "}"
	}

	return fmt.Sprintf("%s: %s%s TO %s%s", r.Label, minBracket, minStr, maxStr, maxBracket)
}

// RangeFacetCounts counts facets for range-based fields.
// This is the Go port of Lucene's org.apache.lucene.facet.RangeFacetCounts.
type RangeFacetCounts struct {
	// field is the field name
	field string

	// ranges are the defined ranges
	ranges []Range

	// counts maps range index to count
	counts []int

	// totalCount is the total number of documents
	totalCount int
}

// NewRangeFacetCounts creates a new RangeFacetCounts for the specified field and ranges.
func NewRangeFacetCounts(field string, ranges ...Range) *RangeFacetCounts {
	return &RangeFacetCounts{
		field:  field,
		ranges: ranges,
		counts: make([]int, len(ranges)),
	}
}

// Accumulate accumulates a value from a document.
func (rfc *RangeFacetCounts) Accumulate(value interface{}) {
	for i, r := range rfc.ranges {
		if r.Contains(value) {
			rfc.counts[i]++
		}
	}
	rfc.totalCount++
}

// GetCount returns the count for a specific range.
func (rfc *RangeFacetCounts) GetCount(rangeIndex int) int {
	if rangeIndex < 0 || rangeIndex >= len(rfc.counts) {
		return 0
	}
	return rfc.counts[rangeIndex]
}

// GetCountByLabel returns the count for a range with the specified label.
func (rfc *RangeFacetCounts) GetCountByLabel(label string) int {
	for i, r := range rfc.ranges {
		if r.Label == label {
			return rfc.counts[i]
		}
	}
	return 0
}

// GetTotalCount returns the total count of all documents.
func (rfc *RangeFacetCounts) GetTotalCount() int {
	return rfc.totalCount
}

// GetRangeCount returns the number of ranges.
func (rfc *RangeFacetCounts) GetRangeCount() int {
	return len(rfc.ranges)
}

// GetRange returns the range at the specified index.
func (rfc *RangeFacetCounts) GetRange(index int) (Range, error) {
	if index < 0 || index >= len(rfc.ranges) {
		return Range{}, fmt.Errorf("range index %d out of bounds", index)
	}
	return rfc.ranges[index], nil
}

// GetRangeResults returns all range results sorted by count.
func (rfc *RangeFacetCounts) GetRangeResults() []RangeResult {
	results := make([]RangeResult, len(rfc.ranges))
	for i, r := range rfc.ranges {
		results[i] = RangeResult{
			Range: r,
			Count: rfc.counts[i],
		}
	}

	// Sort by count (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	return results
}

// GetAllResults returns all range results in range order.
func (rfc *RangeFacetCounts) GetAllResults() []RangeResult {
	results := make([]RangeResult, len(rfc.ranges))
	for i, r := range rfc.ranges {
		results[i] = RangeResult{
			Range: r,
			Count: rfc.counts[i],
		}
	}
	return results
}

// Reset clears all accumulated counts.
func (rfc *RangeFacetCounts) Reset() {
	rfc.counts = make([]int, len(rfc.ranges))
	rfc.totalCount = 0
}

// String returns a string representation of this facet counts.
func (rfc *RangeFacetCounts) String() string {
	return fmt.Sprintf("RangeFacetCounts(field=%s, ranges=%d, totalCount=%d)",
		rfc.field, len(rfc.ranges), rfc.totalCount)
}

// RangeResult represents a range and its count.
type RangeResult struct {
	Range Range
	Count int
}

// RangeFacetCollector collects range facets from search results.
type RangeFacetCollector struct {
	field  string
	ranges []Range
	counts *RangeFacetCounts
}

// NewRangeFacetCollector creates a new RangeFacetCollector.
func NewRangeFacetCollector(field string, ranges ...Range) *RangeFacetCollector {
	return &RangeFacetCollector{
		field:  field,
		ranges: ranges,
		counts: NewRangeFacetCounts(field, ranges...),
	}
}

// Collect collects a value from a document.
func (c *RangeFacetCollector) Collect(value interface{}) {
	c.counts.Accumulate(value)
}

// GetFacetCounts returns the accumulated facet counts.
func (c *RangeFacetCollector) GetFacetCounts() *RangeFacetCounts {
	return c.counts
}

// Common range presets

// GetPriceRanges returns common price ranges.
func GetPriceRanges() []Range {
	return []Range{
		NewRange("Under $10", nil, 10.0),
		NewRange("$10 - $50", 10.0, 50.0),
		NewRange("$50 - $100", 50.0, 100.0),
		NewRange("$100 - $500", 100.0, 500.0),
		NewRange("Over $500", 500.0, nil),
	}
}

// GetDateRanges returns common date ranges (in days ago).
func GetDateRanges() []Range {
	return []Range{
		NewRange("Today", 0, 1),
		NewRange("Last 7 days", 1, 7),
		NewRange("Last 30 days", 7, 30),
		NewRange("Last 90 days", 30, 90),
		NewRange("Last year", 90, 365),
		NewRange("Older", 365, nil),
	}
}

// GetSizeRanges returns common size ranges.
func GetSizeRanges() []Range {
	return []Range{
		NewRange("Small", nil, 1024),
		NewRange("Medium", 1024, 1048576),
		NewRange("Large", 1048576, 1073741824),
		NewRange("Extra Large", 1073741824, nil),
	}
}
