// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MatchesIterator iterates over match positions and offsets within a single
// document and field.
//
// Mirrors org.apache.lucene.search.MatchesIterator.
type MatchesIterator interface {
	// Next advances to the next match. Returns false when the iterator is
	// exhausted.
	Next() (bool, error)
	// StartPosition returns the start position of the current match, or -1 if
	// positions are unavailable.
	StartPosition() int
	// EndPosition returns the end position of the current match, or -1 if
	// positions are unavailable.
	EndPosition() int
	// StartOffset returns the start character offset of the current match, or
	// -1 if offsets are unavailable.
	StartOffset() (int, error)
	// EndOffset returns the end character offset of the current match, or -1
	// if offsets are unavailable.
	EndOffset() (int, error)
	// GetSubMatches returns an iterator over the term-level sub-matches inside
	// the current match, or nil if there are none.
	GetSubMatches() (MatchesIterator, error)
	// GetQuery returns the Query responsible for the current match.
	GetQuery() Query
}
