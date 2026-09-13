package search

import "math"

// ScorerSupplier is a supplier of Scorer. This allows to get an estimate of the cost
// before building the Scorer.
type ScorerSupplier interface {
	// Get the Scorer. This may not return nil and must be called at most once.
	//
	// leadCost: Cost of the scorer that will be used in order to lead iteration.
	// This can be interpreted as an upper bound of the number of times that
	// DocIdSetIterator.NextDoc, DocIdSetIterator.Advance and TwoPhaseIterator.Matches
	// will be called. Under doubt, pass math.MaxInt64, which will produce a Scorer
	// that has good iteration capabilities.
	Get(leadCost int64) (Scorer, error)

	// Cost gets an estimate of the Scorer that would be returned by Get.
	// This may be a costly operation, so it should only be called if necessary.
	Cost() int64

	// SetTopLevelScoringClause informs this ScorerSupplier that its returned scorers
	// produce scores that get passed to the collector, as opposed to partial scores
	// that then need to get combined (e.g. summed up).
	SetTopLevelScoringClause() error

	// BulkScorer gets a scorer that is optimized for bulk-scoring.
	BulkScorer() (BulkScorer, error)
}

// BaseScorerSupplier carries the concrete members of the abstract class
// org.apache.lucene.search.ScorerSupplier (Lucene 10.5.0).
//
// Go has no class inheritance, so a type that ports a ScorerSupplier subclass
// embeds BaseScorerSupplier and overrides only what the Java subclass
// overrides. Java's get(long) and cost() are abstract and are therefore not
// provided here: the embedder must supply them.
type BaseScorerSupplier struct{}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Java is empty.
func (s *BaseScorerSupplier) SetTopLevelScoringClause() error {
	return nil
}

// DefaultScorerSupplierBulkScorer is the body of ScorerSupplier.bulkScorer():
// new DefaultBulkScorer(get(Long.MAX_VALUE)).
//
// It is a free function rather than a method on BaseScorerSupplier because the
// Java body dispatches back to the abstract get(long), which an embedded Go
// struct cannot reach. This mirrors the idiom scorer.go already uses for
// Scorer's self-dispatching default (DefaultNextDocsAndScores).
func DefaultScorerSupplierBulkScorer(s ScorerSupplier) (BulkScorer, error) {
	scorer, err := s.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return NewDefaultBulkScorer(scorer), nil
}
