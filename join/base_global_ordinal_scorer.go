// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BaseGlobalOrdinalScorer is the abstract base for scorers that iterate over
// documents using a two-phase approach: a fast approximation pass followed by
// an ordinal-based match confirmation.
//
// Mirrors org.apache.lucene.search.join.BaseGlobalOrdinalScorer.
//
// Concrete subtypes must provide createTwoPhaseIterator.
type BaseGlobalOrdinalScorer struct {
	values        index.SortedDocValues
	approximation search.DocIdSetIterator
	boost         float32

	// score is set by the concrete subtype during two-phase matching.
	score float32

	// twoPhase is constructed lazily the first time Iterator() or
	// TwoPhaseIterator() is called.
	twoPhase *search.TwoPhaseIterator

	// createTwoPhase is the factory injected by the concrete subtype.
	createTwoPhase func(approx search.DocIdSetIterator) *search.TwoPhaseIterator
}

// newBaseGlobalOrdinalScorer initialises the base scorer.
func newBaseGlobalOrdinalScorer(
	values index.SortedDocValues,
	approximation search.DocIdSetIterator,
	boost float32,
	createTwoPhase func(approx search.DocIdSetIterator) *search.TwoPhaseIterator,
) *BaseGlobalOrdinalScorer {
	return &BaseGlobalOrdinalScorer{
		values:         values,
		approximation:  approximation,
		boost:          boost,
		createTwoPhase: createTwoPhase,
	}
}

// Score implements search.Scorer.
func (s *BaseGlobalOrdinalScorer) Score() float32 { return s.score * s.boost }

// GetMaxScore implements search.Scorer.
func (s *BaseGlobalOrdinalScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }

// DocID implements search.DocIdSetIterator via the approximation.
func (s *BaseGlobalOrdinalScorer) DocID() int {
	if s.approximation == nil {
		return search.NO_MORE_DOCS
	}
	return s.approximation.DocID()
}

// TwoPhaseIterator returns the two-phase iterator for this scorer,
// constructing it once on first call.
func (s *BaseGlobalOrdinalScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	if s.twoPhase == nil && s.createTwoPhase != nil {
		s.twoPhase = s.createTwoPhase(s.approximation)
	}
	return s.twoPhase
}

// NextDoc implements search.DocIdSetIterator.
// Advances through the two-phase wrapper to the next matching document.
func (s *BaseGlobalOrdinalScorer) NextDoc() (int, error) {
	tpi := s.TwoPhaseIterator()
	if tpi == nil {
		if s.approximation == nil {
			return search.NO_MORE_DOCS, nil
		}
		return s.approximation.NextDoc()
	}
	return nextDocTwoPhase(tpi)
}

// Advance implements search.DocIdSetIterator.
func (s *BaseGlobalOrdinalScorer) Advance(target int) (int, error) {
	tpi := s.TwoPhaseIterator()
	if tpi == nil {
		if s.approximation == nil {
			return search.NO_MORE_DOCS, nil
		}
		return s.approximation.Advance(target)
	}
	return advanceTwoPhase(tpi, target)
}

// Cost implements search.DocIdSetIterator.
func (s *BaseGlobalOrdinalScorer) Cost() int64 {
	if s.approximation == nil {
		return 0
	}
	return s.approximation.Cost()
}

// DocIDRunEnd implements search.DocIdSetIterator.
func (s *BaseGlobalOrdinalScorer) DocIDRunEnd() int {
	return s.DocID() + 1
}

// nextDocTwoPhase advances a TwoPhaseIterator to the next matching document.
func nextDocTwoPhase(tpi *search.TwoPhaseIterator) (int, error) {
	approx := tpi.Approximation()
	for {
		doc, err := approx.NextDoc()
		if err != nil || doc == search.NO_MORE_DOCS {
			return doc, err
		}
		ok, err := tpi.Matches()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if ok {
			return doc, nil
		}
	}
}

// advanceTwoPhase advances a TwoPhaseIterator to the given target or beyond,
// returning the first matching document.
func advanceTwoPhase(tpi *search.TwoPhaseIterator, target int) (int, error) {
	approx := tpi.Approximation()
	doc, err := approx.Advance(target)
	if err != nil || doc == search.NO_MORE_DOCS {
		return doc, err
	}
	for {
		ok, err := tpi.Matches()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if ok {
			return doc, nil
		}
		doc, err = approx.NextDoc()
		if err != nil || doc == search.NO_MORE_DOCS {
			return doc, err
		}
	}
}

// interface compliance — BaseGlobalOrdinalScorer itself satisfies search.Scorer.
var _ search.Scorer = (*BaseGlobalOrdinalScorer)(nil)
