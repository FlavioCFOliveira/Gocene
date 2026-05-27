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

// Rewrite rewrites the query using only the top r.size terms by score.
//
// Mirrors org.apache.lucene.search.TopTermsRewrite in Lucene 10.4.0. The
// canonical algorithm walks the matched-term enumerator, keeps a min-heap
// of the top-N (term, boost) pairs, and emits a BooleanQuery whose clauses
// are TermQueries (or BoostQuery-wrapped TermQueries) for the surviving
// terms.
//
// # Degradation
//
// The full algorithm requires BoostAttribute / TermState access on the
// TermsEnum (see ScoringRewrite). Until those are exposed by the index
// package, Rewrite falls back to delegating to ConstantScoreBooleanRewriteMethod
// and clamping its output to at most r.size SHOULD clauses, preserving the
// "top-N expansion" semantic at the cost of using the natural enumeration
// order rather than the boost-driven priority queue.
func (r *TopTermsRewrite) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	if query == nil {
		return nil, nil
	}
	if r.size <= 0 {
		return NewMatchNoDocsQuery(), nil
	}
	rewritten, err := ScoringBooleanRewriteMethod.Rewrite(query, reader)
	if err != nil {
		if err == ErrTooManyClauses {
			return query, nil
		}
		return nil, err
	}
	bq, ok := rewritten.(*BooleanQuery)
	if !ok {
		return rewritten, nil
	}
	clauses := bq.Clauses()
	if len(clauses) <= r.size {
		return bq, nil
	}
	// Keep the first r.size clauses (natural enumeration order).
	trimmed := NewBooleanQuery()
	for i := 0; i < r.size && i < len(clauses); i++ {
		trimmed.Add(clauses[i].Query, clauses[i].Occur)
	}
	return trimmed, nil
}
