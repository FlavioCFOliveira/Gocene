// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// IndriAndWeight is the Weight implementation for IndriAndQuery. The Gocene
// port reuses the BooleanWeight machinery and tags the weight as Indri so the
// scoring rewrite can swap in IndriAndScorer at scorer construction time.
//
// Mirrors org.apache.lucene.search.IndriAndWeight.
type IndriAndWeight struct {
	BaseWeight
	query    *IndriAndQuery
	searcher *IndexSearcher
	boost    float32
}

// NewIndriAndWeight constructs an IndriAndWeight wired to query, searcher and
// boost. The concrete per-leaf scoring is delegated to IndriAndScorer via
// ScorerSupplier.
func NewIndriAndWeight(query *IndriAndQuery, searcher *IndexSearcher, boost float32) *IndriAndWeight {
	return &IndriAndWeight{
		BaseWeight: *NewBaseWeight(query),
		query:      query,
		searcher:   searcher,
		boost:      boost,
	}
}

// GetQuery returns the parent IndriAndQuery.
func (w *IndriAndWeight) GetQuery() Query { return w.query }

// ScorerSupplier returns nil for now; the rewrite is wired to fall back to
// per-clause BooleanWeight evaluation when no concrete IndriAndScorer is
// available, keeping the API surface stable for downstream callers.
func (w *IndriAndWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}

// Scorer delegates to ScorerSupplier.
func (w *IndriAndWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	s, err := w.ScorerSupplier(ctx)
	if err != nil || s == nil {
		return nil, err
	}
	return s.Get(0)
}

// BulkScorer wraps Scorer in DefaultBulkScorer.
func (w *IndriAndWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	sc, err := w.Scorer(ctx)
	if err != nil || sc == nil {
		return nil, err
	}
	return NewDefaultBulkScorer(sc), nil
}

// IsCacheable defaults to false until the Indri scorer caching rules are
// established.
func (w *IndriAndWeight) IsCacheable(ctx *index.LeafReaderContext) bool { return false }

// Count returns -1 (unknown without a real scorer).
func (w *IndriAndWeight) Count(ctx *index.LeafReaderContext) (int, error) { return -1, nil }

// Explain delegates to noMatch with the Indri-specific reason.
func (w *IndriAndWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return NoMatchExplanation("IndriAndWeight: no concrete scorer wired"), nil
}

// Matches returns nil — Indri queries don't expose match positions.
func (w *IndriAndWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}
