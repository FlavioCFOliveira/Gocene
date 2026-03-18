package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToChildBlockJoinQuery is a query that matches child documents
// based on parent document criteria. It treats documents as blocks
// where the last document in each block is the parent.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToChildBlockJoinQuery.
type ToChildBlockJoinQuery struct {
	// parentQuery is the query to match parent documents
	parentQuery search.Query

	// originalParentQuery stores the original parent query before rewrite
	originalParentQuery search.Query

	// parentsFilter identifies parent documents using a BitSetProducer
	parentsFilter BitSetProducer

	// scoreMode determines how parent scores are propagated to children
	scoreMode ScoreMode
}

// NewToChildBlockJoinQuery creates a new ToChildBlockJoinQuery.
// Parameters:
//   - parentQuery: the query to match parent documents
//   - parentsFilter: the BitSetProducer identifying parent documents
//   - scoreMode: how to combine scores from parent documents
func NewToChildBlockJoinQuery(parentQuery search.Query, parentsFilter BitSetProducer, scoreMode ScoreMode) *ToChildBlockJoinQuery {
	return &ToChildBlockJoinQuery{
		parentQuery:         parentQuery,
		originalParentQuery:   parentQuery,
		parentsFilter:       parentsFilter,
		scoreMode:           scoreMode,
	}
}

// GetParentQuery returns the parent query.
func (q *ToChildBlockJoinQuery) GetParentQuery() search.Query {
	return q.parentQuery
}

// GetOriginalParentQuery returns the original parent query before any rewrites.
func (q *ToChildBlockJoinQuery) GetOriginalParentQuery() search.Query {
	return q.originalParentQuery
}

// GetParentsFilter returns the BitSetProducer that identifies parent documents.
func (q *ToChildBlockJoinQuery) GetParentsFilter() BitSetProducer {
	return q.parentsFilter
}

// GetScoreMode returns the score mode.
func (q *ToChildBlockJoinQuery) GetScoreMode() ScoreMode {
	return q.scoreMode
}

// Rewrite rewrites this query.
func (q *ToChildBlockJoinQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewrittenParent, err := q.parentQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	if rewrittenParent != q.parentQuery {
		return NewToChildBlockJoinQuery(rewrittenParent, q.parentsFilter, q.scoreMode), nil
	}

	return q, nil
}

// Clone creates a copy of this query.
func (q *ToChildBlockJoinQuery) Clone() search.Query {
	return NewToChildBlockJoinQuery(
		q.parentQuery.Clone(),
		q.parentsFilter,
		q.scoreMode,
	)
}

// Equals checks if this query equals another.
func (q *ToChildBlockJoinQuery) Equals(other search.Query) bool {
	if o, ok := other.(*ToChildBlockJoinQuery); ok {
		return q.parentQuery.Equals(o.parentQuery) &&
			q.parentsFilter == o.parentsFilter &&
			q.scoreMode == o.scoreMode
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *ToChildBlockJoinQuery) HashCode() int {
	// Use the parent query hash code and score mode
	// The parentsFilter is an interface, so we use a constant contribution
	return 31*(31*q.parentQuery.HashCode()+int(q.scoreMode)) + 17
}

// CreateWeight creates a Weight for this query.
func (q *ToChildBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// Create the parent query weight
	parentWeight, err := q.parentQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent weight: %w", err)
	}

	// Create and return the BlockJoinWeight
	// For ToChildBlockJoinQuery, we need a special weight that handles
	// the child-to-parent relationship using the BitSetProducer
	return NewToChildBlockJoinWeight(q, parentWeight, q.parentsFilter, q.scoreMode, boost), nil
}

// String returns a string representation of this query.
func (q *ToChildBlockJoinQuery) String() string {
	return fmt.Sprintf("ToChildBlockJoinQuery(parent=%v, scoreMode=%v)",
		q.parentQuery, q.scoreMode)
}
