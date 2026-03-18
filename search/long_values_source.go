// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LongValuesSource provides long values for use in queries and sorting.
// This is the Go port of Lucene's org.apache.lucene.search.LongValuesSource.
type LongValuesSource struct {
	field string
}

// NewLongValuesSource creates a new LongValuesSource.
func NewLongValuesSource(field string) *LongValuesSource {
	return &LongValuesSource{field: field}
}

// GetValues returns the long values for the given context.
func (s *LongValuesSource) GetValues(context interface{}) ([]int64, error) {
	// For now, return empty values
	// In a full implementation, this would read from DocValues
	return nil, nil
}

// GetSortField returns a SortField for sorting by these values.
func (s *LongValuesSource) GetSortField(reverse bool) *SortField {
	sf := NewSortField(s.field, SortFieldTypeLong)
	sf.Reverse = reverse
	return sf
}

// GetRangeQuery returns a query that matches documents within a range.
func (s *LongValuesSource) GetRangeQuery(lower, upper int64) Query {
	// For now, return a match all query
	// In a full implementation, this would create a range query
	return NewMatchAllDocsQuery()
}
