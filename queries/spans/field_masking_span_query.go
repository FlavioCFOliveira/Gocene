// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/FieldMaskingSpanQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashFieldMaskingSpanQuery seeds FieldMaskingSpanQuery.hashCode() in
// place of Java's classHash().
const classHashFieldMaskingSpanQuery = 0x466d_5351 // "FmSQ"

// FieldMaskingSpanQuery is a wrapper that allows SpanQuery objects to
// participate in composite single-field SpanQueries by 'lying' about their
// search field. That is, the masked SpanQuery will function as normal, but
// GetField simply hands back the value supplied in the constructor.
//
// This can be used to support queries like SpanNearQuery or SpanOrQuery across
// different fields, which is not ordinarily permitted.
//
// Note: as GetField returns the masked field, scoring will be done using the
// Similarity and collection statistics of the field name supplied, but with the
// term statistics of the real field. This may lead to errors, poor performance,
// and unexpected scoring behaviour.
//
// Mirrors org.apache.lucene.queries.spans.FieldMaskingSpanQuery (final).
type FieldMaskingSpanQuery struct {
	search.BaseQuery
	maskedQuery SpanQuery
	field       string
}

// NewFieldMaskingSpanQuery constructs a FieldMaskingSpanQuery presenting
// maskedQuery under maskedField.
//
// Mirrors FieldMaskingSpanQuery(SpanQuery, String).
func NewFieldMaskingSpanQuery(maskedQuery SpanQuery, maskedField string) (*FieldMaskingSpanQuery, error) {
	if maskedQuery == nil {
		return nil, fmt.Errorf("FieldMaskingSpanQuery: maskedQuery must not be nil")
	}
	return &FieldMaskingSpanQuery{maskedQuery: maskedQuery, field: maskedField}, nil
}

// GetField returns the masked field.
func (q *FieldMaskingSpanQuery) GetField() string { return q.field }

// GetMaskedQuery returns the wrapped SpanQuery.
func (q *FieldMaskingSpanQuery) GetMaskedQuery() SpanQuery { return q.maskedQuery }

// Note: getBoost and setBoost are not proxied to the maskedQuery — this is done
// to be more consistent with things like SpanFirstQuery.

// CreateWeight mirrors FieldMaskingSpanQuery.createWeight(IndexSearcher,
// ScoreMode, float), whose covariant return is a SpanWeight.
func (q *FieldMaskingSpanQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// FieldMaskingSpanQuery.createWeight: the masked query's weight, unchanged.
func (q *FieldMaskingSpanQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	return q.maskedQuery.CreateSpanWeight(searcher, scoreMode, boost)
}

// Rewrite mirrors FieldMaskingSpanQuery.rewrite(IndexSearcher).
func (q *FieldMaskingSpanQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewrittenQuery, err := q.maskedQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	rewritten, ok := rewrittenQuery.(SpanQuery)
	if !ok {
		return nil, fmt.Errorf("FieldMaskingSpanQuery.Rewrite: masked query rewrote to non-SpanQuery %T", rewrittenQuery)
	}
	if rewritten != q.maskedQuery {
		return NewFieldMaskingSpanQuery(rewritten, q.field)
	}
	return q, nil
}

// Visit mirrors FieldMaskingSpanQuery.visit(QueryVisitor).
func (q *FieldMaskingSpanQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitSpanQuery(q.maskedQuery, visitor.GetSubVisitor(search.MUST, q))
	}
}

// ToString mirrors FieldMaskingSpanQuery.toString(String field).
func (q *FieldMaskingSpanQuery) ToString(field string) string {
	return fmt.Sprintf("mask(%s) as %s", spanQueryToString(q.maskedQuery, field), q.field)
}

// Equals mirrors FieldMaskingSpanQuery.equals(Object).
func (q *FieldMaskingSpanQuery) Equals(other spi.Query) bool {
	o, ok := other.(*FieldMaskingSpanQuery)
	if !ok {
		return false
	}
	return q.GetField() == o.GetField() && q.GetMaskedQuery().Equals(o.GetMaskedQuery())
}

// HashCode mirrors FieldMaskingSpanQuery.hashCode().
func (q *FieldMaskingSpanQuery) HashCode() int {
	return classHashFieldMaskingSpanQuery ^ q.GetMaskedQuery().HashCode() ^ javaStringHashCode(q.GetField())
}

var _ SpanQuery = (*FieldMaskingSpanQuery)(nil)

// String renders Query.toString(), whose Java body is toString("").
func (q *FieldMaskingSpanQuery) String() string { return q.ToString("") }
