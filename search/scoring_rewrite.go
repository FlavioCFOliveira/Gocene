// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ScoringRewrite rewrites multi-term queries preserving scoring information.
// This is the Go port of Lucene's ScoringRewrite.
type ScoringRewrite struct {
	field string
}

// NewScoringRewrite creates a new ScoringRewrite.
func NewScoringRewrite(field string) *ScoringRewrite {
	return &ScoringRewrite{field: field}
}

// Rewrite rewrites the query preserving scoring.
func (r *ScoringRewrite) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	// For now, return the query as-is
	// In a full implementation, this would create a BooleanQuery with SHOULD clauses
	// and preserve term statistics for scoring
	return query, nil
}

// GetField returns the field for this rewrite method.
func (r *ScoringRewrite) GetField() string {
	return r.field
}
