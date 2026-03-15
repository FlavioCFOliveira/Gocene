// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TwoPhaseIterator provides a two-phase matching approach for queries.
// This is the Go port of Lucene's org.apache.lucene.search.TwoPhaseIterator.
//
// Two-phase matching allows queries to:
// 1. Use a fast approximation to identify candidate documents
// 2. Use a slower confirmation phase to verify actual matches
//
// This is particularly useful for complex queries like phrase queries where
// the approximation can quickly find documents containing all terms, and the
// confirmation phase verifies the exact phrase positions.
type TwoPhaseIterator struct {
	// approximation is the fast iterator that produces candidate documents
	approximation DocIdSetIterator

	// matchesFunc returns true if the current document is an actual match
	matchesFunc func() (bool, error)
}

// NewTwoPhaseIterator creates a new TwoPhaseIterator.
//
// Parameters:
//   - approximation: A DocIdSetIterator that provides candidate documents
//   - matchesFunc: A function that returns true if the current document matches
//
// The matchesFunc will only be called when the approximation iterator is
// positioned on a document (i.e., DocID() != -1 and != NO_MORE_DOCS).
func NewTwoPhaseIterator(approximation DocIdSetIterator, matchesFunc func() (bool, error)) *TwoPhaseIterator {
	return &TwoPhaseIterator{
		approximation: approximation,
		matchesFunc:   matchesFunc,
	}
}

// Approximation returns the approximation iterator.
// This iterator produces candidate documents that may or may not match.
func (tpi *TwoPhaseIterator) Approximation() DocIdSetIterator {
	return tpi.approximation
}

// Matches returns true if the current document is an actual match.
// This performs the second phase of two-phase matching.
// Should only be called when the approximation is positioned on a document.
func (tpi *TwoPhaseIterator) Matches() (bool, error) {
	return tpi.matchesFunc()
}

// DocID returns the current document ID from the approximation.
func (tpi *TwoPhaseIterator) DocID() int {
	return tpi.approximation.DocID()
}

// TwoPhaseIteratorAsDocIdSetIterator wraps a TwoPhaseIterator as a DocIdSetIterator.
// This allows two-phase matching to be used transparently where a regular
// DocIdSetIterator is expected.
type TwoPhaseIteratorAsDocIdSetIterator struct {
	twoPhase *TwoPhaseIterator
}

// NewTwoPhaseIteratorAsDocIdSetIterator creates a new DocIdSetIterator
// that wraps a TwoPhaseIterator.
func NewTwoPhaseIteratorAsDocIdSetIterator(twoPhase *TwoPhaseIterator) DocIdSetIterator {
	return &TwoPhaseIteratorAsDocIdSetIterator{
		twoPhase: twoPhase,
	}
}

// DocID returns the current document ID.
func (it *TwoPhaseIteratorAsDocIdSetIterator) DocID() int {
	return it.twoPhase.DocID()
}

// NextDoc advances to the next matching document.
// This iterates through the approximation and checks Matches() for each candidate.
func (it *TwoPhaseIteratorAsDocIdSetIterator) NextDoc() (int, error) {
	for {
		doc, err := it.twoPhase.approximation.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		matches, err := it.twoPhase.Matches()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if matches {
			return doc, nil
		}
	}
}

// Advance advances to the first document at or beyond the target that matches.
func (it *TwoPhaseIteratorAsDocIdSetIterator) Advance(target int) (int, error) {
	doc, err := it.twoPhase.approximation.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}

	for {
		matches, err := it.twoPhase.Matches()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if matches {
			return doc, nil
		}
		doc, err = it.twoPhase.approximation.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
	}
}

// Cost returns an estimate of the cost.
// The cost is based on the approximation's cost.
func (it *TwoPhaseIteratorAsDocIdSetIterator) Cost() int64 {
	return it.twoPhase.approximation.Cost()
}

// DocIDRunEnd returns the end of the current run.
// Since matches are sparse, we return the current doc + 1.
func (it *TwoPhaseIteratorAsDocIdSetIterator) DocIDRunEnd() int {
	return it.twoPhase.approximation.DocID() + 1
}

// Ensure TwoPhaseIteratorAsDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*TwoPhaseIteratorAsDocIdSetIterator)(nil)

// AsDocIdSetIterator returns this TwoPhaseIterator as a DocIdSetIterator.
// This is a convenience method that wraps the two-phase iterator.
func (tpi *TwoPhaseIterator) AsDocIdSetIterator() DocIdSetIterator {
	return NewTwoPhaseIteratorAsDocIdSetIterator(tpi)
}

// TwoPhaseIteratorScorer wraps a TwoPhaseIterator as a Scorer.
// This allows two-phase matching to be used in scoring contexts.
type TwoPhaseIteratorScorer struct {
	*BaseScorer
	twoPhase *TwoPhaseIterator
	doc      int
}

// NewTwoPhaseIteratorScorer creates a new Scorer that wraps a TwoPhaseIterator.
func NewTwoPhaseIteratorScorer(twoPhase *TwoPhaseIterator, weight Weight) *TwoPhaseIteratorScorer {
	return &TwoPhaseIteratorScorer{
		BaseScorer: NewBaseScorer(weight),
		twoPhase:   twoPhase,
		doc:        -1,
	}
}

// DocID returns the current document ID.
func (s *TwoPhaseIteratorScorer) DocID() int {
	return s.twoPhase.DocID()
}

// NextDoc advances to the next matching document.
func (s *TwoPhaseIteratorScorer) NextDoc() (int, error) {
	for {
		doc, err := s.twoPhase.approximation.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc == NO_MORE_DOCS {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		matches, err := s.twoPhase.Matches()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if matches {
			s.doc = doc
			return doc, nil
		}
	}
}

// Advance advances to the target document.
func (s *TwoPhaseIteratorScorer) Advance(target int) (int, error) {
	doc, err := s.twoPhase.approximation.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}

	for {
		matches, err := s.twoPhase.Matches()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if matches {
			s.doc = doc
			return doc, nil
		}
		doc, err = s.twoPhase.approximation.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc == NO_MORE_DOCS {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
	}
}

// Cost returns the estimated cost.
func (s *TwoPhaseIteratorScorer) Cost() int64 {
	return s.twoPhase.approximation.Cost()
}

// DocIDRunEnd returns the end of the current run.
func (s *TwoPhaseIteratorScorer) DocIDRunEnd() int {
	return s.twoPhase.approximation.DocIDRunEnd()
}

// Score returns the score for the current document.
// For simplicity, returns 1.0 for matches.
func (s *TwoPhaseIteratorScorer) Score() float32 {
	return 1.0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *TwoPhaseIteratorScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Matches returns true if the current document matches.
func (s *TwoPhaseIteratorScorer) Matches() (bool, error) {
	return s.twoPhase.Matches()
}

// Ensure TwoPhaseIteratorScorer implements Scorer
var _ Scorer = (*TwoPhaseIteratorScorer)(nil)

// ConjunctionTwoPhaseIterator provides a two-phase iterator for conjunctions (AND queries).
// It combines multiple two-phase iterators and only matches when all match.
type ConjunctionTwoPhaseIterator struct {
	approximation DocIdSetIterator
	matches       []func() (bool, error)
}

// NewConjunctionTwoPhaseIterator creates a two-phase iterator for conjunctions.
// The approximation should be the conjunction of all sub-iterators' approximations.
// The matches slice contains the match functions for each sub-iterator.
func NewConjunctionTwoPhaseIterator(approximation DocIdSetIterator, matches []func() (bool, error)) *TwoPhaseIterator {
	return NewTwoPhaseIterator(approximation, func() (bool, error) {
		for _, match := range matches {
			m, err := match()
			if err != nil {
				return false, err
			}
			if !m {
				return false, nil
			}
		}
		return true, nil
	})
}

// DisjunctionTwoPhaseIterator provides a two-phase iterator for disjunctions (OR queries).
// It combines multiple two-phase iterators and matches when any match.
type DisjunctionTwoPhaseIterator struct {
	approximation DocIdSetIterator
	matches       []func() (bool, error)
}

// NewDisjunctionTwoPhaseIterator creates a two-phase iterator for disjunctions.
// The approximation should be the disjunction of all sub-iterators' approximations.
// The matches slice contains the match functions for each sub-iterator.
func NewDisjunctionTwoPhaseIterator(approximation DocIdSetIterator, matches []func() (bool, error)) *TwoPhaseIterator {
	return NewTwoPhaseIterator(approximation, func() (bool, error) {
		for _, match := range matches {
			m, err := match()
			if err != nil {
				return false, err
			}
			if m {
				return true, nil
			}
		}
		return false, nil
	})
}

// HasTwoPhaseIterator checks if a DocIdSetIterator supports two-phase iteration.
// Returns the TwoPhaseIterator if available, or nil if not.
func HasTwoPhaseIterator(iterator DocIdSetIterator) *TwoPhaseIterator {
	// Check if the iterator is a TwoPhaseIterator wrapper
	if tpi, ok := iterator.(*TwoPhaseIteratorAsDocIdSetIterator); ok {
		return tpi.twoPhase
	}
	// Check if the iterator is a TwoPhaseIteratorScorer
	if tps, ok := iterator.(*TwoPhaseIteratorScorer); ok {
		return tps.twoPhase
	}
	return nil
}

// AsTwoPhaseIterator returns a TwoPhaseIterator view of the given iterator, or nil if not available.
func AsTwoPhaseIterator(iterator DocIdSetIterator) *TwoPhaseIterator {
	return HasTwoPhaseIterator(iterator)
}
