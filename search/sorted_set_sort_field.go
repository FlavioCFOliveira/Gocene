// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SortedSetSortField sorts by values from a SortedSetDocValues field.
// This is the Go port of Lucene's org.apache.lucene.search.SortedSetSortField.
type SortedSetSortField struct {
	*SortField
	selector SortSelector
}

// NewSortedSetSortField creates a new SortedSetSortField.
func NewSortedSetSortField(field string, reverse bool, selector SortSelector) *SortedSetSortField {
	sf := NewSortField(field, SortFieldTypeString)
	sf.Reverse = reverse
	return &SortedSetSortField{
		SortField: sf,
		selector:  selector,
	}
}

// GetSelector returns the sort selector.
func (s *SortedSetSortField) GetSelector() SortSelector {
	return s.selector
}
