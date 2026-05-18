// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// SearchGroup represents a single group encountered by the first-pass
// grouping collector. Mirrors org.apache.lucene.search.grouping.SearchGroup.
//
// GroupValue is the group's key (string for term-based groupings; for the
// other typed group selectors it is the value cast to interface{}).
// SortValues hold the comparator state used by the first-pass collector to
// order groups for top-N selection.
type SearchGroup[T any] struct {
	GroupValue T
	SortValues []any
}

// NewSearchGroup builds a SearchGroup with the supplied value and sort state.
func NewSearchGroup[T any](value T, sortValues []any) *SearchGroup[T] {
	cloned := make([]any, len(sortValues))
	copy(cloned, sortValues)
	return &SearchGroup[T]{GroupValue: value, SortValues: cloned}
}
