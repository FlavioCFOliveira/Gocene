// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// MultiTermQueryConstantScoreWrapper wraps a MultiTermQuery with constant score.
// This is the Go port of Lucene's org.apache.lucene.search.MultiTermQueryConstantScoreWrapper.
type MultiTermQueryConstantScoreWrapper struct {
	BaseQuery
	query *MultiTermQuery
}

// NewMultiTermQueryConstantScoreWrapper creates a new wrapper.
func NewMultiTermQueryConstantScoreWrapper(query *MultiTermQuery) *MultiTermQueryConstantScoreWrapper {
	return &MultiTermQueryConstantScoreWrapper{
		query: query,
	}
}

// GetQuery returns the wrapped query.
func (w *MultiTermQueryConstantScoreWrapper) GetQuery() *MultiTermQuery {
	return w.query
}

// GetField returns the field for this query.
func (w *MultiTermQueryConstantScoreWrapper) GetField() string {
	return w.query.GetField()
}

// Rewrite rewrites this query to a constant-score wrapper around the
// rewritten inner multi-term query.
//
// Mirrors org.apache.lucene.search.MultiTermQueryConstantScoreWrapper.rewrite
// in Lucene 10.4.0. The inner MultiTermQuery is first rewritten (giving the
// concrete TermQuery / BooleanQuery / ... structure that matches the
// indexed terms) and then wrapped in a ConstantScoreQuery so that all
// matching documents receive an identical score equal to the query boost.
func (w *MultiTermQueryConstantScoreWrapper) Rewrite(searcher *IndexSearcher) (Query, error) {
	if w.query == nil {
		return nil, nil
	}
	inner, err := w.query.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		inner = w.query
	}
	if _, ok := inner.(*ConstantScoreQuery); ok {
		return inner, nil
	}
	return NewConstantScoreQuery(inner), nil
}

// CreateWeight creates a Weight for this query.
//
// Mirrors MultiTermQueryConstantScoreWrapper.createWeight(IndexSearcher,
// ScoreMode, float) of Apache Lucene 10.5.0, which returns
// `new RewritingWeight(query, boost, scoreMode, searcher) { ... }`.
// RewritingWeight extends ConstantScoreWeight and is constructed with the
// boost as its constant score, so the Weight handed back here is a
// ConstantScoreWeight over this query.
func (w *MultiTermQueryConstantScoreWrapper) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewConstantScoreWeight(w, boost, nil, nil), nil
}

// Equals checks if this query equals another.
func (w *MultiTermQueryConstantScoreWrapper) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*MultiTermQueryConstantScoreWrapper); ok {
		return w.query.Equals(o.query)
	}
	return false
}

// HashCode returns a hash code for this query.
func (w *MultiTermQueryConstantScoreWrapper) HashCode() int {
	return w.query.HashCode()
}

// String mirrors AbstractMultiTermQueryConstantScoreWrapper.toString, whose
// body is `return "ConstantScore(" + query.toString(field) + ")";`.
//
// Java reaches the concrete MultiTermQuery subclass through the abstract
// Query.toString(String) declaration. Gocene's Query interface does not carry
// that member (see the note in query.go), so the call is routed through
// queryToString, which dispatches on whatever rendering the concrete query
// actually declares.
func (w *MultiTermQueryConstantScoreWrapper) String(field string) string {
	return "ConstantScore(" + queryToString(w.query, field) + ")"
}

// Ensure MultiTermQueryConstantScoreWrapper implements Query
var _ Query = (*MultiTermQueryConstantScoreWrapper)(nil)
