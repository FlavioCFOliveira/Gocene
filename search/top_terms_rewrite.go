// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TopTermsRewrite rewrites multi-term queries using only the top N terms.
// This is the Go port of Lucene's TopTermsRewrite.
type TopTermsRewrite struct {
	size int
}

// NewTopTermsRewrite creates a new TopTermsRewrite.
func NewTopTermsRewrite(size int) *TopTermsRewrite {
	return &TopTermsRewrite{size: size}
}

// SetSize sets the maximum number of terms to use.
func (r *TopTermsRewrite) SetSize(size int) {
	r.size = size
}

// GetSize returns the maximum number of terms to use.
func (r *TopTermsRewrite) GetSize() int {
	return r.size
}

// Rewrite rewrites the query using only the top terms.
func (r *TopTermsRewrite) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	// For now, return the query as-is
	// In a full implementation, this would collect terms and use only the top N
	return query, nil
}
