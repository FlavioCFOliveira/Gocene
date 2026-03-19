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
func (q *ToParentBlockJoinQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewrittenChild, err := q.childQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	if rewrittenChild != q.childQuery {
		return NewToParentBlockJoinQuery(rewrittenChild, q.parentsFilter, q.scoreMode), nil
	}

	return q, nil
}

// Clone creates a copy of this query.
func (q *ToParentBlockJoinQuery) Clone() search.Query {
	return NewToParentBlockJoinQuery(
		q.childQuery.Clone(),
		q.parentsFilter,
		q.scoreMode,
	)
}

// Equals checks if this query equals another.
func (q *ToParentBlockJoinQuery) Equals(other search.Query) bool {
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

// CreateWeight creates a Weight for this query.
func (q *ToParentBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// Create the child query weight
	childWeight, err := q.childQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create child weight: %w", err)
	}

	// Create and return the ToParentBlockJoinWeight
	// For ToParentBlockJoinQuery, we need a special weight that handles
	// the child-to-parent relationship using the BitSetProducer
	return NewToParentBlockJoinWeight(q, childWeight, q.parentsFilter, q.scoreMode, boost), nil
}

// String returns a string representation of this query.
func (q *ToParentBlockJoinQuery) String() string {
	return fmt.Sprintf("ToParentBlockJoinQuery(child=%v, scoreMode=%s)",
		q.childQuery, q.scoreMode)
}
