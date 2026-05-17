// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SortedNumericSelectorType picks one value from a multi-valued numeric
// document-values field for sorting.
//
// Mirrors org.apache.lucene.search.SortedNumericSelector.Type.
type SortedNumericSelectorType int

const (
	// SortedNumericSelectorMin picks the smallest value in the set.
	SortedNumericSelectorMin SortedNumericSelectorType = iota
	// SortedNumericSelectorMax picks the largest value in the set.
	SortedNumericSelectorMax
)

// String returns the canonical name of the selector type.
func (t SortedNumericSelectorType) String() string {
	switch t {
	case SortedNumericSelectorMin:
		return "MIN"
	case SortedNumericSelectorMax:
		return "MAX"
	default:
		return "UNKNOWN"
	}
}
