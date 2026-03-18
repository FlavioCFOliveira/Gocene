// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DoubleValuesSource provides double values for use in queries and sorting.
// This is the Go port of Lucene's org.apache.lucene.search.DoubleValuesSource.
type DoubleValuesSource struct {
	field string
}

// NewDoubleValuesSource creates a new DoubleValuesSource.
func NewDoubleValuesSource(field string) *DoubleValuesSource {
	return &DoubleValuesSource{field: field}
}

// GetValues returns the double values for the given context.
func (s *DoubleValuesSource) GetValues(context interface{}) ([]float64, error) {
	// For now, return empty values
	// In a full implementation, this would read from DocValues
	return nil, nil
}

// GetSortField returns a SortField for sorting by these values.
func (s *DoubleValuesSource) GetSortField(reverse bool) *SortField {
	sf := NewSortField(s.field, SortFieldTypeDouble)
	sf.Reverse = reverse
	return sf
}

// GetRangeQuery returns a query that matches documents within a range.
func (s *DoubleValuesSource) GetRangeQuery(lower, upper float64) Query {
	// For now, return a match all query
	// In a full implementation, this would create a range query
	return NewMatchAllDocsQuery()
}
