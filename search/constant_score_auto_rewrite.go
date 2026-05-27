// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ConstantScoreAutoRewrite automatically chooses between rewrite methods.
// This is the Go port of Lucene's ConstantScoreAutoRewrite.
type ConstantScoreAutoRewrite struct {
	// threshold is the number of terms above which to use a more efficient method
	threshold int
}

// NewConstantScoreAutoRewrite creates a new ConstantScoreAutoRewrite.
func NewConstantScoreAutoRewrite() *ConstantScoreAutoRewrite {
	return &ConstantScoreAutoRewrite{
		threshold: 16, // Default threshold
	}
}

// SetThreshold sets the threshold for choosing rewrite methods.
func (r *ConstantScoreAutoRewrite) SetThreshold(threshold int) {
	r.threshold = threshold
}

// GetThreshold returns the threshold for choosing rewrite methods.
func (r *ConstantScoreAutoRewrite) GetThreshold() int {
	return r.threshold
}

// Rewrite rewrites the query using an automatic method selection.
//
// Mirrors org.apache.lucene.search.MultiTermQuery.CONSTANT_SCORE_AUTO_REWRITE
// in Lucene 10.4.0. The strategy is:
//   - delegate to ConstantScoreBooleanRewriteMethod, which expands the
//     multi-term query into a BooleanQuery of TermQueries wrapped in a
//     ConstantScoreQuery; and
//   - if the expansion would exceed r.threshold clauses (signalled by
//     ErrTooManyClauses), fall back to wrapping the original query in a
//     ConstantScoreQuery, which is the cheap branch that does not
//     materialise per-term postings.
//
// The full Lucene path also walks PostingsEnum to collect a doc-id-set
// efficiently for the "many terms" case; Gocene defers that optimisation
// because TermsEnum.TermState() is not yet exposed (see ScoringRewrite).
func (r *ConstantScoreAutoRewrite) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	if query == nil {
		return nil, nil
	}
	rewritten, err := ConstantScoreBooleanRewriteMethod.Rewrite(query, reader)
	if err != nil {
		if err == ErrTooManyClauses {
			// Many-terms branch: keep the query as a single ConstantScoreQuery.
			return NewConstantScoreQuery(query), nil
		}
		return nil, err
	}
	// Apply the threshold check on the resulting BooleanQuery (if any).
	if cs, ok := rewritten.(*ConstantScoreQuery); ok {
		if bq, ok := cs.Query().(*BooleanQuery); ok {
			if len(bq.Clauses()) > r.threshold {
				return NewConstantScoreQuery(query), nil
			}
		}
	}
	return rewritten, nil
}
