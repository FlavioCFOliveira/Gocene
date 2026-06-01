// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Scorer iterates over documents and scores them.
type Scorer interface {
	DocIdSetIterator
	// Score returns the score of the current document.
	Score() float32
	// GetMaxScore returns the maximum score for documents up to the given doc.
	GetMaxScore(upTo int) float32
	// AdvanceShallow advances to the block of documents that contains target
	// in order to get scoring information about this block. The returned value
	// is an inclusive upper bound of the doc IDs that share the same scoring
	// information (the same block-max upper bound) as target; subsequent
	// GetMaxScore(upTo) calls with upTo in [target, returned] are honoured
	// without re-advancing.
	//
	// This mirrors org.apache.lucene.search.Scorer#advanceShallow(int). The
	// default implementation (see BaseScorer.AdvanceShallow) returns
	// NO_MORE_DOCS, matching Lucene's Scorer.advanceShallow default, which
	// signals that the scorer treats the whole remaining postings list as a
	// single block (no impact-based skipping).
	AdvanceShallow(target int) (int, error)
}

// ScoreErrorReporter is the optional Scorer extension for scorers that can
// detect an error condition while computing a score. Gocene's Scorer.Score
// returns only a float32 (no error), unlike Lucene where Scorer.score() throws
// IOException/IllegalStateException; this interface lets such scorers surface a
// deferred error that the search loop consults after the score is consumed.
//
// It is used to faithfully reproduce the block-join "Child query must not match
// same docs with parent filter" IllegalStateException that Lucene raises from
// ToParentBlockJoinQuery.BlockJoinScorer.scoreChildDocs.
type ScoreErrorReporter interface {
	// ScoreError returns a non-nil error if the most recent Score call detected
	// an invariant violation, or nil otherwise.
	ScoreError() error
}

// MinCompetitiveScorer is the optional Scorer extension that lets a collector
// (or a parent scorer) hint at the minimum score a hit must reach to be
// competitive, enabling non-competitive documents to be skipped. It mirrors
// org.apache.lucene.search.Scorer#setMinCompetitiveScore.
//
// It is modelled as an optional interface rather than a method on Scorer so
// that the many existing Scorer implementations keep compiling unchanged: only
// scorers that participate in TOP_SCORES early termination implement it, and
// callers type-assert before forwarding the hint.
type MinCompetitiveScorer interface {
	// SetMinCompetitiveScore informs the scorer that hits scoring below
	// minScore are not competitive and may be skipped. Implementations that
	// cannot skip should leave it a no-op.
	SetMinCompetitiveScore(minScore float32) error
}

// BaseScorer provides common functionality for scorers.
type BaseScorer struct {
	weight Weight
}

// NewBaseScorer creates a new BaseScorer.
func NewBaseScorer(weight Weight) *BaseScorer {
	return &BaseScorer{weight: weight}
}

// GetWeight returns the weight.
func (s *BaseScorer) GetWeight() Weight {
	return s.weight
}

// Score returns a default score.
func (s *BaseScorer) Score() float32 {
	return 1.0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *BaseScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// AdvanceShallow returns NO_MORE_DOCS, mirroring the default implementation of
// org.apache.lucene.search.Scorer#advanceShallow, which returns
// DocIdSetIterator.NO_MORE_DOCS. A scorer that does not expose per-block impact
// information treats the entire remaining postings list as one block: callers
// learn that GetMaxScore is only meaningful for upTo == NO_MORE_DOCS (a global
// upper bound). Scorers backed by impacts (e.g. TermScorer) override this.
func (s *BaseScorer) AdvanceShallow(target int) (int, error) {
	return NO_MORE_DOCS, nil
}
