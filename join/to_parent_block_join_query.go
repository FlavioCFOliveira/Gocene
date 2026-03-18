package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToParentBlockJoinQuery is a query that matches parent documents
// based on child document criteria. It treats documents as blocks
// where the last document in each block is the parent.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinQuery.
type ToParentBlockJoinQuery struct {
	// childQuery is the query to match child documents
	childQuery search.Query

	// parentFilter is the filter identifying parent documents
	parentFilter search.Query

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode
}

// NewToParentBlockJoinQuery creates a new ToParentBlockJoinQuery.
// Parameters:
//   - childQuery: the query to match child documents
//   - parentFilter: the filter identifying parent documents
//   - scoreMode: how to combine scores from child documents
func NewToParentBlockJoinQuery(childQuery search.Query, parentFilter search.Query, scoreMode ScoreMode) *ToParentBlockJoinQuery {
	return &ToParentBlockJoinQuery{
		childQuery:   childQuery,
		parentFilter: parentFilter,
		scoreMode:    scoreMode,
	}
}

// GetChildQuery returns the child query.
func (q *ToParentBlockJoinQuery) GetChildQuery() search.Query {
	return q.childQuery
}

// GetParentFilter returns the parent filter.
func (q *ToParentBlockJoinQuery) GetParentFilter() search.Query {
	return q.parentFilter
}

// GetScoreMode returns the score mode.
func (q *ToParentBlockJoinQuery) GetScoreMode() ScoreMode {
	return q.scoreMode
}

// Rewrite rewrites this query.
func (q *ToParentBlockJoinQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewrittenChild, err := q.childQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	rewrittenParent, err := q.parentFilter.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	if rewrittenChild != q.childQuery || rewrittenParent != q.parentFilter {
		return NewToParentBlockJoinQuery(rewrittenChild, rewrittenParent, q.scoreMode), nil
	}

	return q, nil
}

// Clone creates a copy of this query.
func (q *ToParentBlockJoinQuery) Clone() search.Query {
	return NewToParentBlockJoinQuery(
		q.childQuery.Clone(),
		q.parentFilter.Clone(),
		q.scoreMode,
	)
}

// Equals checks if this query equals another.
func (q *ToParentBlockJoinQuery) Equals(other search.Query) bool {
	if o, ok := other.(*ToParentBlockJoinQuery); ok {
		return q.childQuery.Equals(o.childQuery) &&
			q.parentFilter.Equals(o.parentFilter) &&
			q.scoreMode == o.scoreMode
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *ToParentBlockJoinQuery) HashCode() int {
	return 31*(31*q.childQuery.HashCode()+q.parentFilter.HashCode()) + int(q.scoreMode)
}

// CreateWeight creates a Weight for this query.
func (q *ToParentBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// Create the child query weight
	childWeight, err := q.childQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create child weight: %w", err)
	}

	// Create the parent filter weight (always needs scores for parent matching)
	parentWeight, err := q.parentFilter.CreateWeight(searcher, false, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent weight: %w", err)
	}

	// Create and return the BlockJoinWeight
	return NewBlockJoinWeight(q, childWeight, parentWeight, q.scoreMode), nil
}

// String returns a string representation of this query.
func (q *ToParentBlockJoinQuery) String() string {
	return fmt.Sprintf("ToParentBlockJoinQuery(child=%v, parent=%v, scoreMode=%s)",
		q.childQuery, q.parentFilter, q.scoreMode)
}
