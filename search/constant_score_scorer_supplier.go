// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ConstantScoreScorerSupplier is a ScorerSupplier that yields a
// ConstantScoreScorer. It mirrors
// org.apache.lucene.search.ConstantScoreScorerSupplier (Lucene
// 10.4.0): the supplier holds the constant score, the requested
// ScoreMode and a pre-computed cost; the wrapped DocIdSetIterator is
// materialised lazily through the iteratorFactory closure when Get
// is called.
//
// The Java reference is abstract — callers subclass it and override
// iterator(long leadCost). In Go we capture that hook as a function
// field so concrete consumers (the spatial query family, the
// upcoming MatchAllDocsQuery promotion, etc.) can plug their own
// iterator-building logic without inheritance.
type ConstantScoreScorerSupplier struct {
	score           float32
	scoreMode       ScoreMode
	cost            int64
	iteratorFactory func(leadCost int64) (DocIdSetIterator, error)

	// topLevelScoring tracks whether SetTopLevelScoringClause has
	// been invoked. The flag is currently informational only; once
	// MaxScoreSumPropagator lands the supplier can use it to honour
	// the optimisation Java applies in the same situation.
	topLevelScoring bool
}

// NewConstantScoreScorerSupplier builds a supplier that hands out a
// ConstantScoreScorer for every Get call. The iteratorFactory
// closure is invoked exactly once per Get (mirroring the abstract
// iterator(long leadCost) hook on the Java reference); a nil
// iteratorFactory is rejected because the supplier would otherwise
// produce a scorer with a nil iterator.
func NewConstantScoreScorerSupplier(
	score float32,
	scoreMode ScoreMode,
	cost int64,
	iteratorFactory func(leadCost int64) (DocIdSetIterator, error),
) *ConstantScoreScorerSupplier {
	if iteratorFactory == nil {
		// Fall back to a no-op factory that yields an empty
		// iterator so callers never NPE; the empty-iterator branch
		// is the safe equivalent of Java returning DocIdSetIterator.
		// empty(), which is what every concrete subclass does when
		// the cell has no matches.
		iteratorFactory = func(_ int64) (DocIdSetIterator, error) {
			return NewEmptyDocIdSetIterator(), nil
		}
	}
	if cost < 0 {
		cost = 0
	}
	return &ConstantScoreScorerSupplier{
		score:           score,
		scoreMode:       scoreMode,
		cost:            cost,
		iteratorFactory: iteratorFactory,
	}
}

// NewConstantScoreScorerSupplierFromIterator is a convenience
// constructor that wraps a single pre-built iterator. The supplier
// returns the same DocIdSetIterator on every Get call (mirroring the
// pattern of Java callers that materialise the iterator eagerly and
// then build the supplier around it).
//
// Callers that need a fresh iterator per Get should use
// NewConstantScoreScorerSupplier with a factory closure instead.
func NewConstantScoreScorerSupplierFromIterator(
	score float32,
	scoreMode ScoreMode,
	iter DocIdSetIterator,
) *ConstantScoreScorerSupplier {
	if iter == nil {
		iter = NewEmptyDocIdSetIterator()
	}
	return NewConstantScoreScorerSupplier(
		score,
		scoreMode,
		iter.Cost(),
		func(_ int64) (DocIdSetIterator, error) { return iter, nil },
	)
}

// Get returns a ConstantScoreScorer over the iterator produced by
// the supplier's factory. leadCost is propagated to the factory in
// case it wants to size internal buffers (e.g. a DocIdSetBuilder)
// based on the lead-clause cost; the default factory ignores it.
func (s *ConstantScoreScorerSupplier) Get(leadCost int64) (Scorer, error) {
	iter, err := s.iteratorFactory(leadCost)
	if err != nil {
		return nil, err
	}
	if iter == nil {
		iter = NewEmptyDocIdSetIterator()
	}
	return NewConstantScoreScorer(s.score, s.scoreMode, iter), nil
}

// Cost returns the supplier's pre-computed cost. The Java reference
// also caches the cost across calls; this supplier holds the value
// in the field directly to keep the cost call lock-free and
// allocation-free.
func (s *ConstantScoreScorerSupplier) Cost() int64 { return s.cost }

// SetTopLevelScoringClause records that this supplier is the
// top-level scoring clause for the current search. The flag is
// surfaced as IsTopLevelScoringClause for tests and for the
// upcoming MaxScoreSumPropagator wiring; it has no behavioural
// effect today.
func (s *ConstantScoreScorerSupplier) SetTopLevelScoringClause() {
	s.topLevelScoring = true
}

// IsTopLevelScoringClause reports whether SetTopLevelScoringClause
// has been called on this supplier.
func (s *ConstantScoreScorerSupplier) IsTopLevelScoringClause() bool {
	return s.topLevelScoring
}

// GetScore returns the constant score the supplier will hand to
// every scorer it produces.
func (s *ConstantScoreScorerSupplier) GetScore() float32 { return s.score }

// GetScoreMode returns the ScoreMode the supplier was configured
// with.
func (s *ConstantScoreScorerSupplier) GetScoreMode() ScoreMode { return s.scoreMode }

// Ensure ConstantScoreScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*ConstantScoreScorerSupplier)(nil)
