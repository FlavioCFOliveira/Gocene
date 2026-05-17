// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SortedSetSelectorType picks one value from a multi-valued sorted-set
// document-values field for sorting.
//
// Mirrors org.apache.lucene.search.SortedSetSelector.Type. The MIDDLE_MIN /
// MIDDLE_MAX variants resolve to the lower (resp. higher) of the two centre
// values when the cardinality is even.
type SortedSetSelectorType int

const (
	// SortedSetSelectorMin picks the smallest value in the set.
	SortedSetSelectorMin SortedSetSelectorType = iota
	// SortedSetSelectorMax picks the largest value in the set.
	SortedSetSelectorMax
	// SortedSetSelectorMiddleMin picks the middle value, biasing low for
	// even cardinality.
	SortedSetSelectorMiddleMin
	// SortedSetSelectorMiddleMax picks the middle value, biasing high for
	// even cardinality.
	SortedSetSelectorMiddleMax
)

// String returns the canonical name of the selector type.
func (t SortedSetSelectorType) String() string {
	switch t {
	case SortedSetSelectorMin:
		return "MIN"
	case SortedSetSelectorMax:
		return "MAX"
	case SortedSetSelectorMiddleMin:
		return "MIDDLE_MIN"
	case SortedSetSelectorMiddleMax:
		return "MIDDLE_MAX"
	default:
		return "UNKNOWN"
	}
}
