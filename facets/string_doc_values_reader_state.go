// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// StringDocValuesReaderState caches the per-field state needed by
// StringValueFacetCounts to enumerate sorted-string DocValues facets
// efficiently. Mirrors org.apache.lucene.facet.StringDocValuesReaderState.
type StringDocValuesReaderState struct {
	Field      string
	UniqueOrds int
	// OrdToTerm caches the ordinal-to-term mapping captured at construction
	// time. Index i holds the term associated with ordinal i.
	OrdToTerm []string
}

// NewStringDocValuesReaderState builds a state object for the supplied field
// and ordinal-to-term mapping.
func NewStringDocValuesReaderState(field string, ordToTerm []string) *StringDocValuesReaderState {
	out := make([]string, len(ordToTerm))
	copy(out, ordToTerm)
	return &StringDocValuesReaderState{
		Field:      field,
		UniqueOrds: len(out),
		OrdToTerm:  out,
	}
}

// TermForOrd returns the term mapped to ord, or "" when the ordinal is
// out of range.
func (s *StringDocValuesReaderState) TermForOrd(ord int) string {
	if ord < 0 || ord >= len(s.OrdToTerm) {
		return ""
	}
	return s.OrdToTerm[ord]
}
