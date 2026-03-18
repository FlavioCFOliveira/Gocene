// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SortedNumericSortField sorts by numeric values from a SortedNumericDocValues field.
// This is the Go port of Lucene's org.apache.lucene.search.SortedNumericSortField.
type SortedNumericSortField struct {
	*SortField
	selector SortSelector
}

// SortSelector determines which value to use from a multi-valued field.
type SortSelector int

const (
	// SortMin selects the minimum value
	SortMin SortSelector = iota
	// SortMax selects the maximum value
	SortMax
	// SortSum selects the sum of values
	SortSum
	// SortAvg selects the average of values
	SortAvg
)

// NewSortedNumericSortField creates a new SortedNumericSortField.
func NewSortedNumericSortField(field string, sortType SortFieldType, reverse bool, selector SortSelector) *SortedNumericSortField {
	sf := NewSortField(field, sortType)
	sf.Reverse = reverse
	return &SortedNumericSortField{
		SortField: sf,
		selector:  selector,
	}
}

// GetSelector returns the sort selector.
func (s *SortedNumericSortField) GetSelector() SortSelector {
	return s.selector
}
