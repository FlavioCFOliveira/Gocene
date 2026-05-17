// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndriScorer is the abstract base for IndriAndScorer and related scorers.
// It tracks the underlying clause scorers, the boost and the smoothing-score
// computation hook used by Indri scoring.
//
// Mirrors org.apache.lucene.search.IndriScorer.
type IndriScorer struct {
	weight Weight
	boost  float32
	subs   []Scorer
}

// NewIndriScorer constructs an IndriScorer wired to weight, boost and clause
// scorers.
func NewIndriScorer(weight Weight, boost float32, subs []Scorer) *IndriScorer {
	return &IndriScorer{weight: weight, boost: boost, subs: subs}
}

// Boost returns the configured boost.
func (s *IndriScorer) Boost() float32 { return s.boost }

// SubScorers returns the wrapped clause scorers.
func (s *IndriScorer) SubScorers() []Scorer { return s.subs }

// SmoothingScore returns a fallback log-probability for a document that does
// not contain the term. Concrete subclasses override this hook.
func (s *IndriScorer) SmoothingScore(docID int) float32 { return 0 }
