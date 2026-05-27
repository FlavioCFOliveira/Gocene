// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

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
func (w *MultiTermQueryConstantScoreWrapper) Rewrite(reader IndexReader) (Query, error) {
	if w.query == nil {
		return nil, nil
	}
	inner, err := w.query.Rewrite(reader)
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
func (w *MultiTermQueryConstantScoreWrapper) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(w, nil), nil
}

// Clone creates a copy of this query.
func (w *MultiTermQueryConstantScoreWrapper) Clone() Query {
	return NewMultiTermQueryConstantScoreWrapper(w.query)
}

// Equals checks if this query equals another.
func (w *MultiTermQueryConstantScoreWrapper) Equals(other Query) bool {
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

// String returns a string representation of the query.
func (w *MultiTermQueryConstantScoreWrapper) String(field string) string {
	return "ConstantScore(" + w.query.String(field) + ")"
}

// Ensure MultiTermQueryConstantScoreWrapper implements Query
var _ Query = (*MultiTermQueryConstantScoreWrapper)(nil)
