// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// PhraseMatcher is the base interface for exact and sloppy phrase matching.
// Mirrors org.apache.lucene.search.PhraseMatcher.
type PhraseMatcher interface {
	// Approximation returns an approximation that only matches documents that have all terms.
	Approximation() DocIdSetIterator

	// ImpactsApproximation returns an approximation that is aware of impacts.
	ImpactsApproximation() *ImpactsDISI

	// MaxFreq returns an upper bound on the number of possible matches on this document.
	MaxFreq() (float32, error)

	// ResetPositions is called after approximation has been advanced to load positions for matching.
	ResetPositions() error

	// NextMatch finds the next match on the current document, returning false if there are none.
	NextMatch() (bool, error)

	// SloppyWeight returns the slop-adjusted weight of the current match.
	SloppyWeight() float32

	// StartPosition returns the start position of the current match.
	StartPosition() int

	// EndPosition returns the end position of the current match.
	EndPosition() int

	// StartOffset returns the start offset of the current match.
	StartOffset() (int, error)

	// EndOffset returns the end offset of the current match.
	EndOffset() (int, error)

	// GetMatchCost returns an estimate of the average cost of finding all matches on a document.
	GetMatchCost() float32
}
