// Package composite implements
// org.apache.lucene.spatial.composite: the composite spatial strategy.
package composite

import "github.com/FlavioCFOliveira/Gocene/search"

// CompositeSpatialStrategy combines an index-side strategy with a verify-side
// strategy. Mirrors
// org.apache.lucene.spatial.composite.CompositeSpatialStrategy.
type CompositeSpatialStrategy struct {
	IndexStrategy  any
	VerifyStrategy any
	Field          string
}

// NewCompositeSpatialStrategy builds the strategy.
func NewCompositeSpatialStrategy(field string, indexStrat, verifyStrat any) *CompositeSpatialStrategy {
	return &CompositeSpatialStrategy{Field: field, IndexStrategy: indexStrat, VerifyStrategy: verifyStrat}
}

// CompositeVerifyQuery wraps an inner query with a verification predicate.
// Mirrors org.apache.lucene.spatial.composite.CompositeVerifyQuery.
type CompositeVerifyQuery struct {
	Inner  search.Query
	Verify func(docID int) (bool, error)
}

// NewCompositeVerifyQuery builds the query.
func NewCompositeVerifyQuery(inner search.Query, verify func(docID int) (bool, error)) *CompositeVerifyQuery {
	return &CompositeVerifyQuery{Inner: inner, Verify: verify}
}

// IntersectsRPTVerifyQuery is the RPT-based intersect verify query. Mirrors
// org.apache.lucene.spatial.composite.IntersectsRPTVerifyQuery.
type IntersectsRPTVerifyQuery struct {
	*CompositeVerifyQuery
}

// NewIntersectsRPTVerifyQuery builds the query.
func NewIntersectsRPTVerifyQuery(inner search.Query, verify func(docID int) (bool, error)) *IntersectsRPTVerifyQuery {
	return &IntersectsRPTVerifyQuery{CompositeVerifyQuery: NewCompositeVerifyQuery(inner, verify)}
}
