package search

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
