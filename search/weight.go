// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// Weight is the internal representation of a query.
// This is the Go port of Lucene's org.apache.lucene.search.Weight.
type Weight interface {
	// GetQuery returns the parent query.
	GetQuery() Query

	// Explain returns an explanation of the score for the given document.
	// This is used by the query explanation mechanism.
	Explain(context *index.LeafReaderContext, doc int) (Explanation, error)

	// ScorerSupplier creates a ScorerSupplier for this weight.
	// The ScorerSupplier allows getting cost information before creating the actual Scorer.
	ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error)

	// Scorer creates a scorer for this weight.
	// This is a convenience method that delegates to ScorerSupplier.
	Scorer(context *index.LeafReaderContext) (Scorer, error)

	// BulkScorer creates a bulk scorer for efficient bulk scoring.
	// This is a convenience method that delegates to ScorerSupplier.
	BulkScorer(context *index.LeafReaderContext) (BulkScorer, error)

	// IsCacheable returns true if this weight can be cached for the given leaf.
	IsCacheable(ctx *index.LeafReaderContext) bool

	// Count returns the count of matching documents in sub-linear time.
	// Returns -1 if the count cannot be computed efficiently.
	Count(context *index.LeafReaderContext) (int, error)

	// Matches returns the matches for a specific document, or nil if there are no matches.
	Matches(context *index.LeafReaderContext, doc int) (Matches, error)
}

// BaseWeight provides common functionality for weights.
type BaseWeight struct {
	query Query
}

// NewBaseWeight creates a new BaseWeight.
func NewBaseWeight(query Query) *BaseWeight {
	return &BaseWeight{query: query}
}

// GetQuery returns the parent query.
func (w *BaseWeight) GetQuery() Query {
	return w.query
}

// Explain returns an explanation of the score for the given document.
func (w *BaseWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return nil, nil
}

// ScorerSupplier creates a ScorerSupplier for this weight.
func (w *BaseWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}

// Scorer creates a scorer for this weight.
func (w *BaseWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *BaseWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached.
func (w *BaseWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return false
}

// Count returns the count of matching documents.
func (w *BaseWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *BaseWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure BaseWeight implements Weight
var _ Weight = (*BaseWeight)(nil)

// scorerMatch positions a freshly created Scorer on the requested leaf-local
// document and reports whether the scorer actually matches that document.
//
// It mirrors the universal shape of Lucene's Weight.explain implementations,
// which pull a Scorer for the leaf and advance its iterator to doc: a hit
// occurs precisely when iterator().advance(doc) == doc. Driving the
// explanation off the same Scorer the search path uses guarantees that the
// explained value equals the scored value — the property Lucene preserves by
// computing the explained score from a live Scorer rather than re-deriving it.
//
// The returned score is meaningful only when matched is true; callers must
// treat it as undefined otherwise. A nil scorer (no candidates on this leaf)
// is reported as a non-match with a zero score and no error.
func scorerMatch(w Weight, context *index.LeafReaderContext, doc int) (matched bool, score float32, err error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return false, 0, err
	}
	if scorer == nil {
		return false, 0, nil
	}
	advanced, err := scorer.Advance(doc)
	if err != nil {
		return false, 0, err
	}
	if advanced != doc {
		return false, 0, nil
	}
	return true, scorer.Score(), nil
}
