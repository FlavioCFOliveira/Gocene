package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BaseGlobalOrdinalScorer is the base for scorers that use global ordinals.
// It mirrors Lucene's org.apache.lucene.search.join.BaseGlobalOrdinalScorer.
type BaseGlobalOrdinalScorer struct {
	values        index.SortedDocValues
	approximation DocIdSetIterator
	boost         float32
	score         float32
	// createTwoPhaseIterator is the Go equivalent of the abstract method in Java.
	createTwoPhaseIterator func(approximation DocIdSetIterator) *TwoPhaseIterator
}

// NewBaseGlobalOrdinalScorer creates a new BaseGlobalOrdinalScorer.
func NewBaseGlobalOrdinalScorer(values index.SortedDocValues, approximation DocIdSetIterator, boost float32, createTPI func(DocIdSetIterator) *TwoPhaseIterator) *BaseGlobalOrdinalScorer {
	return &BaseGlobalOrdinalScorer{
		values:                 values,
		approximation:          approximation,
		boost:                  boost,
		createTwoPhaseIterator: createTPI,
	}
}

// Score returns the current document's score.
func (s *BaseGlobalOrdinalScorer) Score() (float32, error) {
	return s.score * s.boost, nil
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *BaseGlobalOrdinalScorer) GetMaxScore(upTo int) (float32, error) {
	// mirrors Float.POSITIVE_INFINITY
	return 3.402823466e+38, nil
}

// DocID returns the current document ID.
func (s *BaseGlobalOrdinalScorer) DocID() int {
	return s.approximation.DocID()
}

// Iterator returns a DocIdSetIterator over matching documents.
func (s *BaseGlobalOrdinalScorer) Iterator() DocIdSetIterator {
	return s.TwoPhaseIterator().AsDocIdSetIterator()
}

// TwoPhaseIterator returns a TwoPhaseIterator view of this Scorer.
func (s *BaseGlobalOrdinalScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.createTwoPhaseIterator(s.approximation)
}

// AdvanceShallow is not overridden, uses DefaultAdvanceShallow.
func (s *BaseGlobalOrdinalScorer) AdvanceShallow(target int) (int, error) {
	return DefaultAdvanceShallow(target)
}

// NextDocsAndScores is not overridden, uses DefaultNextDocsAndScores.
func (s *BaseGlobalOrdinalScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}
