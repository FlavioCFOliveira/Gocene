// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// PhraseMatcher abstracts the per-document matching loop used by both exact
// and sloppy phrase scorers.
//
// Mirrors org.apache.lucene.search.PhraseMatcher. The usage pattern is:
//
//  1. caller advances the approximation iterator to a candidate doc
//  2. caller invokes Reset
//  3. caller invokes NextMatch in a loop until it returns false
type PhraseMatcher interface {
	// Approximation returns the iterator that lists every doc containing at
	// least one of the phrase's terms.
	Approximation() DocIdSetIterator
	// ImpactsApproximation returns the impacts-aware variant of the
	// approximation iterator, or nil if no impacts are available.
	ImpactsApproximation() DocIdSetIterator
	// MaxFreq returns an upper bound on the number of matches in the current
	// document.
	MaxFreq() (int, error)
	// Reset prepares the matcher for iteration over the current document.
	Reset() error
	// NextMatch advances to the next match in the current document and
	// returns whether one was found.
	NextMatch() (bool, error)
	// StartPosition returns the start position of the current match.
	StartPosition() int
	// EndPosition returns the end position of the current match.
	EndPosition() int
	// StartOffset returns the start character offset of the current match.
	StartOffset() (int, error)
	// EndOffset returns the end character offset of the current match.
	EndOffset() (int, error)
	// SloppyWeight returns the per-match scoring weight under the current
	// slop configuration (exact matchers may return 1.0).
	SloppyWeight() float32
	// MatchCost returns an estimate of the cost of producing all matches in a
	// document.
	MatchCost() float32
}
