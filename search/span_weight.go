// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// GC-1003: SpanWeight implementation
// SpanWeight manages scoring, term state extraction, and Spans access.

// SpanQuery is the interface for span queries.
// Span queries are used for positional/proximity-based search.
type SpanQuery interface {
	Query
	// GetField returns the field for this query.
	GetField() string
	// String returns a string representation of this query.
	String(field string) string
}

// SpanWeight is the base class for SpanQuery weights.
// SpanWeights are used for scoring and matching span queries.
//
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanWeight.
type SpanWeight struct {
	*BaseWeight
	// Query is the SpanQuery this weight is for
	SpanQuery SpanQuery

	// Similarity is the similarity used for scoring
	Similarity Similarity
}

// NewSpanWeight creates a new SpanWeight for the given query.
func NewSpanWeight(query SpanQuery, similarity Similarity) *SpanWeight {
	return &SpanWeight{
		BaseWeight: NewBaseWeight(query),
		SpanQuery:  query,
		Similarity: similarity,
	}
}

// GetSpanQuery returns the SpanQuery this weight is for.
func (sw *SpanWeight) GetSpanQuery() SpanQuery {
	return sw.SpanQuery
}

// GetValue returns the weight value (used for scoring).
func (sw *SpanWeight) GetValue() float32 {
	return 1.0
}

// IsCacheable returns true if this weight can be cached.
func (sw *SpanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Scorer creates a scorer for this weight.
func (sw *SpanWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	// Create a basic spans iterator for the field
	field := sw.SpanQuery.GetField()
	if field == "" {
		return nil, nil
	}

	// For now, create a simple scorer that doesn't actually match spans
	// This is a placeholder implementation that should be overridden by specific span query weights
	spans := &Spans{
		doc:    -1,
		docs:   []int{},
		starts: []int{},
		ends:   []int{},
		index:  -1,
	}
	return NewSpanScorer(spans, 1.0), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (sw *SpanWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := sw.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (sw *SpanWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "SpanWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (sw *SpanWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := sw.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Count returns the count of matching documents.
func (sw *SpanWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (sw *SpanWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// GetSpans returns a Spans object for iterating over span matches
func (sw *SpanWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	return EmptySpans, nil
}

// ExtractTermContexts extracts TermContexts from all terms in the query
func (sw *SpanWeight) ExtractTermContexts(context *index.TermContext) error {
	return nil
}

// GetSimScorer returns the Similarity.SimScorer for scoring spans
func (sw *SpanWeight) GetSimScorer(ctx *index.LeafReaderContext) (*index.SimScorer, error) {
	if sw.Similarity == nil {
		return nil, nil
	}
	return sw.Similarity.Scorer(nil, nil), nil
}

// Ensure SpanWeight implements Weight
var _ Weight = (*SpanWeight)(nil)

// SpanWeightUtils provides utility methods for SpanWeight
var SpanWeightUtils = &spanWeightUtils{}

type spanWeightUtils struct{}

// IsPayloadsRequired returns true if payloads are required for the given score mode
func (u *spanWeightUtils) IsPayloadsRequired(scoreMode ScoreMode) bool {
	return false
}

// PositionsAreRequired returns true if positions are required
func (u *spanWeightUtils) PositionsAreRequired(scoreMode ScoreMode) bool {
	return true
}
