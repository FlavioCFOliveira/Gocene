// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import "sort"

// DefaultSortedSetDocValuesReaderState is the in-memory implementation of
// SortedSetDocValuesReaderState used by tests and callers that already hold
// the (dim -> ordinal range) mapping. Mirrors
// org.apache.lucene.facet.sortedset.DefaultSortedSetDocValuesReaderState.
type DefaultSortedSetDocValuesReaderState struct {
	field      string
	size       int
	dimRanges  map[string][2]int
	dims       []string
}

// NewDefaultSortedSetDocValuesReaderState builds a state object from a
// pre-computed dimension-to-ordinal-range mapping.
func NewDefaultSortedSetDocValuesReaderState(field string, size int, dimRanges map[string][2]int) *DefaultSortedSetDocValuesReaderState {
	cloned := make(map[string][2]int, len(dimRanges))
	dims := make([]string, 0, len(dimRanges))
	for k, v := range dimRanges {
		cloned[k] = v
		dims = append(dims, k)
	}
	sort.Strings(dims)
	return &DefaultSortedSetDocValuesReaderState{
		field:     field,
		size:      size,
		dimRanges: cloned,
		dims:      dims,
	}
}

// GetField returns the field name.
func (s *DefaultSortedSetDocValuesReaderState) GetField() string { return s.field }

// GetSize returns the total ordinal count.
func (s *DefaultSortedSetDocValuesReaderState) GetSize() int { return s.size }

// GetOrdRange returns the [start, end) ordinal range for dim, or (-1, -1).
func (s *DefaultSortedSetDocValuesReaderState) GetOrdRange(dim string) (int, int) {
	r, ok := s.dimRanges[dim]
	if !ok {
		return -1, -1
	}
	return r[0], r[1]
}

// GetDims returns the dimensions in sorted order.
func (s *DefaultSortedSetDocValuesReaderState) GetDims() []string {
	out := make([]string, len(s.dims))
	copy(out, s.dims)
	return out
}

var _ SortedSetDocValuesReaderState = (*DefaultSortedSetDocValuesReaderState)(nil)
