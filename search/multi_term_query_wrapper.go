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

// Rewrite rewrites this query to a simpler form.
func (w *MultiTermQueryConstantScoreWrapper) Rewrite(reader IndexReader) (Query, error) {
	// For now, just return the wrapped query
	// In a full implementation, this would rewrite to a ConstantScoreQuery
	return w.query, nil
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
