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

	// parentFilter is the filter identifying parent documents
	parentFilter search.Query
}

// NewToChildBlockJoinQuery creates a new ToChildBlockJoinQuery.
// Parameters:
//   - parentQuery: the query to match parent documents
//   - parentFilter: the filter identifying parent documents
func NewToChildBlockJoinQuery(parentQuery search.Query, parentFilter search.Query) *ToChildBlockJoinQuery {
	return &ToChildBlockJoinQuery{
		parentQuery:  parentQuery,
		parentFilter: parentFilter,
	}
}

// GetParentQuery returns the parent query.
func (q *ToChildBlockJoinQuery) GetParentQuery() search.Query {
	return q.parentQuery
}

// GetParentFilter returns the parent filter.
func (q *ToChildBlockJoinQuery) GetParentFilter() search.Query {
	return q.parentFilter
}

// Rewrite rewrites this query.
func (q *ToChildBlockJoinQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewrittenParent, err := q.parentQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	rewrittenFilter, err := q.parentFilter.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	if rewrittenParent != q.parentQuery || rewrittenFilter != q.parentFilter {
		return NewToChildBlockJoinQuery(rewrittenParent, rewrittenFilter), nil
	}

	return q, nil
}

// Clone creates a copy of this query.
func (q *ToChildBlockJoinQuery) Clone() search.Query {
	return NewToChildBlockJoinQuery(
		q.parentQuery.Clone(),
		q.parentFilter.Clone(),
	)
}

// Equals checks if this query equals another.
func (q *ToChildBlockJoinQuery) Equals(other search.Query) bool {
	if o, ok := other.(*ToChildBlockJoinQuery); ok {
		return q.parentQuery.Equals(o.parentQuery) &&
			q.parentFilter.Equals(o.parentFilter)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *ToChildBlockJoinQuery) HashCode() int {
	return 31*q.parentQuery.HashCode() + q.parentFilter.HashCode()
}

// CreateWeight creates a Weight for this query.
func (q *ToChildBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// Create the parent query weight
	parentWeight, err := q.parentQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent weight: %w", err)
	}

	// Create the parent filter weight (doesn't need scores for filtering)
	filterWeight, err := q.parentFilter.CreateWeight(searcher, false, boost)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter weight: %w", err)
	}

	// Create and return the BlockJoinWeight
	// For ToChildBlockJoinQuery, we use the parent query to identify children
	// whose parents match, so we pass the parentWeight as child and filter as parent
	return NewBlockJoinWeight(q, parentWeight, filterWeight, None), nil
}

// String returns a string representation of this query.
func (q *ToChildBlockJoinQuery) String() string {
	return fmt.Sprintf("ToChildBlockJoinQuery(parent=%v, filter=%v)",
		q.parentQuery, q.parentFilter)
}
