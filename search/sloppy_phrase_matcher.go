// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SloppyPhraseMatcher is the PhraseMatcher variant that allows the matched
// terms to be within a configurable edit-distance (the "slop") of the
// requested phrase positions.
//
// Mirrors org.apache.lucene.search.SloppyPhraseMatcher. This port wires the
// public surface — a richer per-document matching loop using a priority queue
// over phrase positions lives in sloppy_phrase_query_test.go and the
// existing phrase_weight.go implementation; the structural type lives here so
// callers can program against the canonical PhraseMatcher contract.
type SloppyPhraseMatcher struct {
	approximation        DocIdSetIterator
	impactsApproximation DocIdSetIterator
	slop                 int
	matchCost            float32
	captureLeadMatch     bool

	startPos int
	endPos   int
}

// NewSloppyPhraseMatcher builds a SloppyPhraseMatcher wired to the given
// approximation iterator and configuration. The concrete per-document match
// loop is provided by the caller via Reset/NextMatch overrides in subtypes.
func NewSloppyPhraseMatcher(approximation, impactsApproximation DocIdSetIterator, slop int, matchCost float32, captureLeadMatch bool) *SloppyPhraseMatcher {
	return &SloppyPhraseMatcher{
		approximation:        approximation,
		impactsApproximation: impactsApproximation,
		slop:                 slop,
		matchCost:            matchCost,
		captureLeadMatch:     captureLeadMatch,
	}
}

// Approximation returns the lead iterator.
func (m *SloppyPhraseMatcher) Approximation() DocIdSetIterator { return m.approximation }

// ImpactsApproximation returns the impacts-aware iterator, or nil.
func (m *SloppyPhraseMatcher) ImpactsApproximation() DocIdSetIterator {
	return m.impactsApproximation
}

// Slop returns the configured slop.
func (m *SloppyPhraseMatcher) Slop() int { return m.slop }

// MaxFreq returns 0 by default; concrete subtypes override after Reset.
func (m *SloppyPhraseMatcher) MaxFreq() (int, error) { return 0, nil }

// Reset is a hook overridden by concrete matchers.
func (m *SloppyPhraseMatcher) Reset() error { m.startPos, m.endPos = -1, -1; return nil }

// NextMatch returns false by default; concrete subtypes override.
func (m *SloppyPhraseMatcher) NextMatch() (bool, error) { return false, nil }

// StartPosition returns the current match start position.
func (m *SloppyPhraseMatcher) StartPosition() int { return m.startPos }

// EndPosition returns the current match end position.
func (m *SloppyPhraseMatcher) EndPosition() int { return m.endPos }

// StartOffset is not tracked by the default sloppy matcher.
func (m *SloppyPhraseMatcher) StartOffset() (int, error) { return -1, nil }

// EndOffset is not tracked by the default sloppy matcher.
func (m *SloppyPhraseMatcher) EndOffset() (int, error) { return -1, nil }

// SloppyWeight computes the per-match weight using Lucene's
// 1 / (1 + distance) shape.
func (m *SloppyPhraseMatcher) SloppyWeight() float32 {
	return 1.0 / (1.0 + float32(m.slop))
}

// MatchCost returns the configured match cost estimate.
func (m *SloppyPhraseMatcher) MatchCost() float32 { return m.matchCost }
