// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongValueFacetCounts counts facets for long value fields.
// This is the Go port of Lucene's org.apache.lucene.facet.LongValueFacetCounts.
type LongValueFacetCounts struct {
	// field is the field name for the long values
	field string

	// counts maps long values to their occurrence counts
	counts map[int64]int

	// totalCount is the total number of documents with long values
	totalCount int

	// topN is the number of top values to track
	topN int
}

// NewLongValueFacetCounts creates a new LongValueFacetCounts for the specified field.
func NewLongValueFacetCounts(field string) *LongValueFacetCounts {
	return &LongValueFacetCounts{
		field:  field,
		counts: make(map[int64]int),
		topN:   10, // default top N
	}
}

// NewLongValueFacetCountsWithTopN creates a new LongValueFacetCounts with a custom topN.
func NewLongValueFacetCountsWithTopN(field string, topN int) *LongValueFacetCounts {
	lvfc := NewLongValueFacetCounts(field)
	lvfc.topN = topN
	return lvfc
}

// Accumulate accumulates a value from a document.
func (lvfc *LongValueFacetCounts) Accumulate(value int64) {
	lvfc.counts[value]++
	lvfc.totalCount++
}

// AccumulateWithCount accumulates a value with a specific count.
func (lvfc *LongValueFacetCounts) AccumulateWithCount(value int64, count int) {
	lvfc.counts[value] += count
	lvfc.totalCount += count
}

// GetCount returns the count for a specific value.
func (lvfc *LongValueFacetCounts) GetCount(value int64) int {
	return lvfc.counts[value]
}

// GetTotalCount returns the total count of all values.
func (lvfc *LongValueFacetCounts) GetTotalCount() int {
	return lvfc.totalCount
}

// GetUniqueValueCount returns the number of unique values.
func (lvfc *LongValueFacetCounts) GetUniqueValueCount() int {
	return len(lvfc.counts)
}

// GetTopValues returns the top N values by count.
func (lvfc *LongValueFacetCounts) GetTopValues(n int) []LongValueCount {
	if n <= 0 {
		n = lvfc.topN
	}

	// Convert map to slice
	var values []LongValueCount
	for value, count := range lvfc.counts {
		values = append(values, LongValueCount{
			Value: value,
			Count: count,
		})
	}

	// Sort by count (descending), then by value (ascending)
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count != values[j].Count {
			return values[i].Count > values[j].Count
		}
		return values[i].Value < values[j].Value
	})

	// Return top N
	if len(values) > n {
		return values[:n]
	}
	return values
}

// GetAllValues returns all values sorted by count.
func (lvfc *LongValueFacetCounts) GetAllValues() []LongValueCount {
	return lvfc.GetTopValues(len(lvfc.counts))
}

// GetValuesInRange returns values within the specified range.
func (lvfc *LongValueFacetCounts) GetValuesInRange(min, max int64) []LongValueCount {
	var values []LongValueCount
	for value, count := range lvfc.counts {
		if value >= min && value <= max {
			values = append(values, LongValueCount{
				Value: value,
				Count: count,
			})
		}
	}

	// Sort by count (descending)
	sort.Slice(values, func(i, j int) bool {
		return values[i].Count > values[j].Count
	})

	return values
}

// Reset clears all accumulated counts.
func (lvfc *LongValueFacetCounts) Reset() {
	lvfc.counts = make(map[int64]int)
	lvfc.totalCount = 0
}

// Merge merges another LongValueFacetCounts into this one.
func (lvfc *LongValueFacetCounts) Merge(other *LongValueFacetCounts) error {
	if other == nil {
		return fmt.Errorf("cannot merge nil LongValueFacetCounts")
	}
	if lvfc.field != other.field {
		return fmt.Errorf("cannot merge LongValueFacetCounts with different fields: %s vs %s", lvfc.field, other.field)
	}

	for value, count := range other.counts {
		lvfc.counts[value] += count
		lvfc.totalCount += count
	}

	return nil
}

// String returns a string representation of this facet counts.
func (lvfc *LongValueFacetCounts) String() string {
	return fmt.Sprintf("LongValueFacetCounts(field=%s, uniqueValues=%d, totalCount=%d)",
		lvfc.field, len(lvfc.counts), lvfc.totalCount)
}

// LongValueCount represents a value and its count.
type LongValueCount struct {
	Value int64
	Count int
}

// LongValueFacetCollector collects long value facets from search results.
type LongValueFacetCollector struct {
	field  string
	counts *LongValueFacetCounts
	reader *index.IndexReader
}

// NewLongValueFacetCollector creates a new LongValueFacetCollector.
func NewLongValueFacetCollector(field string, reader *index.IndexReader) *LongValueFacetCollector {
	return &LongValueFacetCollector{
		field:  field,
		counts: NewLongValueFacetCounts(field),
		reader: reader,
	}
}

// Collect collects facets from a document.
func (c *LongValueFacetCollector) Collect(docID int) error {
	// In a real implementation, this would read the long value from the document
	// For now, this is a placeholder
	return nil
}

// GetFacetCounts returns the accumulated facet counts.
func (c *LongValueFacetCollector) GetFacetCounts() *LongValueFacetCounts {
	return c.counts
}

// LongValueFacetQuery creates a query for matching documents with specific long values.
type LongValueFacetQuery struct {
	*search.TermRangeQuery
	field  string
	values []int64
}

// NewLongValueFacetQuery creates a query for matching documents with the specified long values.
func NewLongValueFacetQuery(field string, values ...int64) *LongValueFacetQuery {
	// Create a term range query that encompasses all values
	// In a real implementation, this would be optimized for the specific values
	return &LongValueFacetQuery{
		field:  field,
		values: values,
	}
}

// GetValues returns the values in this query.
func (q *LongValueFacetQuery) GetValues() []int64 {
	return q.values
}
