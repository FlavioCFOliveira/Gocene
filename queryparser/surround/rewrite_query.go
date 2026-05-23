// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// RewriteQuery is the abstract base for surround rewrite queries. It holds a
// wrapped SrndQuery, a target field name, and a BasicQueryFactory. Concrete
// subtypes (SimpleTermRewriteQuery, DistanceRewriteQuery) implement Rewrite.
//
// Mirrors the package-private abstract class
// org.apache.lucene.queryparser.surround.query.RewriteQuery.
type RewriteQuery struct {
	search.BaseQuery
	srndQuery SrndQuery
	fieldName string
	qf        *BasicQueryFactory
}

// newRewriteQuery initialises the embedded RewriteQuery. Panics if either
// srndQuery or qf is nil — both are required by the Java contract.
func newRewriteQuery(srndQuery SrndQuery, fieldName string, qf *BasicQueryFactory) RewriteQuery {
	if srndQuery == nil {
		panic("surround: srndQuery must not be nil")
	}
	if qf == nil {
		panic("surround: BasicQueryFactory must not be nil")
	}
	return RewriteQuery{
		srndQuery: srndQuery,
		fieldName: fieldName,
		qf:        qf,
	}
}

// GetSrndQuery returns the wrapped SrndQuery.
func (r *RewriteQuery) GetSrndQuery() SrndQuery { return r.srndQuery }

// GetFieldName returns the target field name.
func (r *RewriteQuery) GetFieldName() string { return r.fieldName }

// GetQueryFactory returns the BasicQueryFactory.
func (r *RewriteQuery) GetQueryFactory() *BasicQueryFactory { return r.qf }

// queryStringBase produces the string representation common to all concrete
// subtypes. It mirrors the Java toString(String field) implementation.
func (r *RewriteQuery) queryStringBase(typeName, unusedField string) string {
	fieldNote := ""
	if unusedField != "" {
		fieldNote = "(unused: " + unusedField + ")"
	}
	return fmt.Sprintf("%s%s(%s, %s, maxBasicQueries=%d)",
		typeName, fieldNote, r.fieldName, r.srndQuery, r.qf.GetMaxBasicQueries())
}

// equalsBase checks equality of the common RewriteQuery fields.
func (r *RewriteQuery) equalsBase(other *RewriteQuery) bool {
	return r.fieldName == other.fieldName &&
		r.qf.GetMaxBasicQueries() == other.qf.GetMaxBasicQueries() &&
		fmt.Sprintf("%v", r.srndQuery) == fmt.Sprintf("%v", other.srndQuery)
}

// rewriteQueryHashCode returns a basic combined hash for the common fields.
func rewriteQueryHashCode(fieldName string, maxBasicQueries int) int {
	h := 17
	for _, c := range fieldName {
		h = h*31 + int(c)
	}
	h = h*31 + maxBasicQueries
	return h
}

// SimpleTermRewriteQuery rewrites a SimpleTerm surround node into a Lucene
// Query by delegating to the SimpleTerm's MakeLuceneQueryField. Mirrors
// org.apache.lucene.queryparser.surround.query.SimpleTermRewriteQuery.
type SimpleTermRewriteQuery struct {
	RewriteQuery
	st SimpleTerm
}

// NewSimpleTermRewriteQuery constructs a SimpleTermRewriteQuery.
func NewSimpleTermRewriteQuery(srndQuery SimpleTerm, fieldName string, qf *BasicQueryFactory) *SimpleTermRewriteQuery {
	return &SimpleTermRewriteQuery{
		RewriteQuery: newRewriteQuery(srndQuery, fieldName, qf),
		st:           srndQuery,
	}
}

// Rewrite expands the SimpleTerm against the current BasicQueryFactory state
// and returns the resulting query.
func (q *SimpleTermRewriteQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q.st.MakeLuceneQueryField(q.fieldName, q.qf)
}

// Clone returns a copy of this query.
func (q *SimpleTermRewriteQuery) Clone() search.Query {
	return NewSimpleTermRewriteQuery(q.st, q.fieldName, q.qf)
}

// Equals checks structural equality.
func (q *SimpleTermRewriteQuery) Equals(other search.Query) bool {
	o, ok := other.(*SimpleTermRewriteQuery)
	if !ok {
		return false
	}
	return q.equalsBase(&o.RewriteQuery)
}

// HashCode returns a hash for this query.
func (q *SimpleTermRewriteQuery) HashCode() int {
	return rewriteQueryHashCode(q.fieldName, q.qf.GetMaxBasicQueries())
}

// CreateWeight is not implemented at the surround rewrite layer.
func (q *SimpleTermRewriteQuery) CreateWeight(_ *search.IndexSearcher, _ bool, _ float32) (search.Weight, error) {
	return nil, nil
}

// DistanceRewriteQuery rewrites a DistanceQuery surround node into a Lucene
// SpanNearQuery. Mirrors
// org.apache.lucene.queryparser.surround.query.DistanceRewriteQuery.
type DistanceRewriteQuery struct {
	RewriteQuery
	dq *DistanceQuery
}

// NewDistanceRewriteQuery constructs a DistanceRewriteQuery.
func NewDistanceRewriteQuery(srndQuery *DistanceQuery, fieldName string, qf *BasicQueryFactory) *DistanceRewriteQuery {
	return &DistanceRewriteQuery{
		RewriteQuery: newRewriteQuery(srndQuery, fieldName, qf),
		dq:           srndQuery,
	}
}

// Rewrite delegates to DistanceQuery.MakeLuceneQueryField, producing a
// SpanNearQuery over the named field.
func (q *DistanceRewriteQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q.dq.MakeLuceneQueryField(q.fieldName, q.qf)
}

// Clone returns a copy of this query.
func (q *DistanceRewriteQuery) Clone() search.Query {
	return NewDistanceRewriteQuery(q.dq, q.fieldName, q.qf)
}

// Equals checks structural equality.
func (q *DistanceRewriteQuery) Equals(other search.Query) bool {
	o, ok := other.(*DistanceRewriteQuery)
	if !ok {
		return false
	}
	return q.equalsBase(&o.RewriteQuery)
}

// HashCode returns a hash for this query.
func (q *DistanceRewriteQuery) HashCode() int {
	return rewriteQueryHashCode(q.fieldName, q.qf.GetMaxBasicQueries()) ^ q.dq.GetOpDistance()
}

// CreateWeight is not implemented at the surround rewrite layer.
func (q *DistanceRewriteQuery) CreateWeight(_ *search.IndexSearcher, _ bool, _ float32) (search.Weight, error) {
	return nil, nil
}
