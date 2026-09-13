package join

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToParentBlockJoinQuery is a query that matches parent documents
// based on child document criteria. It treats documents as blocks
// where the last document in each block is the parent.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinQuery.
type ToParentBlockJoinQuery struct {
	// originalChildQuery stores the original child query before rewrite
	originalChildQuery search.Query

	// childQuery is the query to match child documents
	childQuery search.Query

	// parentsFilter identifies parent documents using a BitSetProducer
	parentsFilter BitSetProducer

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode
}

// NewToParentBlockJoinQuery creates a new ToParentBlockJoinQuery.
// Parameters:
//   - childQuery: the query to match child documents
//   - parentsFilter: the BitSetProducer identifying parent documents
//   - scoreMode: how to combine scores from child documents
func NewToParentBlockJoinQuery(childQuery search.Query, parentsFilter BitSetProducer, scoreMode ScoreMode) *ToParentBlockJoinQuery {
	return &ToParentBlockJoinQuery{
		originalChildQuery: childQuery,
		childQuery:         childQuery,
		parentsFilter:      parentsFilter,
		scoreMode:          scoreMode,
	}
}

// GetChildQuery returns the child query.
func (q *ToParentBlockJoinQuery) GetChildQuery() search.Query {
	return q.childQuery
}

// GetOriginalChildQuery returns the original child query before any rewrites.
func (q *ToParentBlockJoinQuery) GetOriginalChildQuery() search.Query {
	return q.originalChildQuery
}

// GetParentsFilter returns the BitSetProducer that identifies parent documents.
func (q *ToParentBlockJoinQuery) GetParentsFilter() BitSetProducer {
	return q.parentsFilter
}

// GetScoreMode returns the score mode.
func (q *ToParentBlockJoinQuery) GetScoreMode() ScoreMode {
	return q.scoreMode
}

// Rewrite rewrites this query.
func (q *ToParentBlockJoinQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewrittenChild, err := q.childQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}

	if rewrittenChild != q.childQuery {
		return NewToParentBlockJoinQuery(rewrittenChild, q.parentsFilter, q.scoreMode), nil
	}

	return q, nil
}

// Equals checks if this query equals another.
func (q *ToParentBlockJoinQuery) Equals(other spi.Query) bool {
	if o, ok := other.(*ToParentBlockJoinQuery); ok {
		return q.childQuery.Equals(o.childQuery) &&
			q.parentsFilter == o.parentsFilter &&
			q.scoreMode == o.scoreMode
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *ToParentBlockJoinQuery) HashCode() int {
	// Use the child query hash code and score mode
	// The parentsFilter is an interface, so we use a constant contribution
	return 31*(31*q.childQuery.HashCode()+17) + int(q.scoreMode)
}

// CreateWeight mirrors
// ToParentBlockJoinQuery.createWeight(IndexSearcher, org.apache.lucene.search.ScoreMode, float).
//
// weightScoreMode is the search-level ScoreMode; q.scoreMode is the join's
// child-aggregation ScoreMode.
func (q *ToParentBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, weightScoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	childScoreMode := None
	if weightScoreMode.NeedsScores() {
		childScoreMode = q.scoreMode
	}

	var childWeight search.Weight
	var err error
	if childScoreMode == None {
		// We do not need to compute a score for the child query, so wrap it
		// in a constant-score query that can early-terminate when the minimum
		// score is greater than 0 and the total hit count is not requested.
		var rewritten search.Query
		rewritten, err = searcher.Rewrite(search.NewConstantScoreQuery(q.childQuery))
		if err != nil {
			return nil, fmt.Errorf("failed to rewrite child query: %w", err)
		}
		childWeight, err = rewritten.CreateWeight(searcher, weightScoreMode, 0)
	} else {
		// If the score is needed and the score mode is not Max, force the
		// collection mode to COMPLETE because the child query cannot skip
		// non-competitive documents. weightScoreMode.NeedsScores() is always
		// true here, but the check is kept to make the logic clearer.
		childWeightScoreMode := weightScoreMode
		if weightScoreMode.NeedsScores() && childScoreMode != Max {
			childWeightScoreMode = search.ScoreModeComplete
		}
		childWeight, err = q.childQuery.CreateWeight(searcher, childWeightScoreMode, boost)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create child weight: %w", err)
	}

	return NewToParentBlockJoinWeight(q, childWeight, q.parentsFilter, childScoreMode, boost), nil
}

// String returns a string representation of this query.
func (q *ToParentBlockJoinQuery) String() string {
	return fmt.Sprintf("ToParentBlockJoinQuery(child=%v, scoreMode=%s)",
		q.childQuery, q.scoreMode)
}
