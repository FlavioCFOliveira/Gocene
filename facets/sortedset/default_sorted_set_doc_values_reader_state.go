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
	field    string
	size     int
	dimRanges map[string][2]int
	dims      []string
	dimTrees  map[string]*DimTree
}

// NewDefaultSortedSetDocValuesReaderState builds a state object from a
// pre-computed dimension-to-ordinal-range mapping.
// dimRanges values are [start, end) where end is exclusive.
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
		dimTrees:  make(map[string]*DimTree),
	}
}

// NewDefaultSortedSetDocValuesReaderStateWithDimTrees builds a state object that
// also supports hierarchical dimensions via DimTree entries.
func NewDefaultSortedSetDocValuesReaderStateWithDimTrees(
	field string, size int,
	dimRanges map[string][2]int,
	dimTrees map[string]*DimTree,
) *DefaultSortedSetDocValuesReaderState {
	s := NewDefaultSortedSetDocValuesReaderState(field, size, dimRanges)
	for k, v := range dimTrees {
		s.dimTrees[k] = v
	}
	return s
}

// GetField returns the field name.
func (s *DefaultSortedSetDocValuesReaderState) GetField() string { return s.field }

// GetSize returns the total ordinal count.
func (s *DefaultSortedSetDocValuesReaderState) GetSize() int { return s.size }

// GetOrdRange returns the [start, end) ordinal range for dim (end exclusive)
// for backward-compatible callers. Returns (-1, -1) when dim is unknown.
func (s *DefaultSortedSetDocValuesReaderState) GetOrdRange(dim string) (int, int) {
	r, ok := s.dimRanges[dim]
	if !ok {
		return -1, -1
	}
	return r[0], r[1]
}

// GetOrdRangeFor returns the inclusive OrdRange for dim (end = exclusive-1),
// or nil when dim is unknown. Mirrors the Java getOrdRange return type.
func (s *DefaultSortedSetDocValuesReaderState) GetOrdRangeFor(dim string) *OrdRange {
	r, ok := s.dimRanges[dim]
	if !ok {
		return nil
	}
	// Java OrdRange is inclusive on both ends; stored values are [start, end)
	// so end-inclusive = r[1]-1.
	or := NewOrdRange(r[0], r[1]-1)
	return &or
}

// GetPrefixToOrdRange returns the full mapping of dim name to inclusive OrdRange.
func (s *DefaultSortedSetDocValuesReaderState) GetPrefixToOrdRange() map[string]OrdRange {
	out := make(map[string]OrdRange, len(s.dimRanges))
	for dim, r := range s.dimRanges {
		out[dim] = NewOrdRange(r[0], r[1]-1)
	}
	return out
}

// GetDimTree returns the DimTree for the given hierarchical dimension, or nil.
func (s *DefaultSortedSetDocValuesReaderState) GetDimTree(dim string) *DimTree {
	return s.dimTrees[dim]
}

// GetDims returns the dimensions in sorted order.
func (s *DefaultSortedSetDocValuesReaderState) GetDims() []string {
	out := make([]string, len(s.dims))
	copy(out, s.dims)
	return out
}

var _ SortedSetDocValuesReaderState = (*DefaultSortedSetDocValuesReaderState)(nil)
