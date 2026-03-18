// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Mock implementations for testing

type mockQuery struct{}

func (q *mockQuery) Rewrite(reader search.IndexReader) (search.Query, error) { return q, nil }
func (q *mockQuery) Clone() search.Query                                     { return &mockQuery{} }
func (q *mockQuery) Equals(other search.Query) bool                          { _, ok := other.(*mockQuery); return ok }
func (q *mockQuery) HashCode() int                                           { return 0 }
func (q *mockQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}

type mockWeight struct {
	query search.Query
}

func (w *mockWeight) GetQuery() search.Query { return w.query }
func (w *mockWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return search.NewExplanation(true, 1.0, "mock explanation"), nil
}
func (w *mockWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}
func (w *mockWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	return nil, nil
}
func (w *mockWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}
func (w *mockWeight) IsCacheable(ctx *index.LeafReaderContext) bool       { return false }
func (w *mockWeight) Count(context *index.LeafReaderContext) (int, error) { return -1, nil }
func (w *mockWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	return nil, nil
}

// Test BlockJoinWeight creation
func TestNewBlockJoinWeight(t *testing.T) {
	query := &mockQuery{}
	childWeight := &mockWeight{query: query}
	parentWeight := &mockWeight{query: query}
	scoreMode := Total

	weight := NewBlockJoinWeight(query, childWeight, parentWeight, scoreMode)

	if weight == nil {
		t.Fatal("expected non-nil weight")
	}

	if weight.GetQuery() != query {
		t.Error("expected query to match")
	}

	if weight.GetChildWeight() != childWeight {
		t.Error("expected child weight to match")
	}

	if weight.GetParentWeight() != parentWeight {
		t.Error("expected parent weight to match")
	}

	if weight.GetScoreMode() != scoreMode {
		t.Error("expected score mode to match")
	}
}

// Test BlockJoinWeight IsCacheable
func TestBlockJoinWeightIsCacheable(t *testing.T) {
	query := &mockQuery{}
	childWeight := &mockWeight{query: query}
	parentWeight := &mockWeight{query: query}

	weight := NewBlockJoinWeight(query, childWeight, parentWeight, Total)

	// Block join weights should not be cacheable
	if weight.IsCacheable(nil) {
		t.Error("expected BlockJoinWeight to not be cacheable")
	}
}

// Test BlockJoinWeight Count
func TestBlockJoinWeightCount(t *testing.T) {
	query := &mockQuery{}
	childWeight := &mockWeight{query: query}
	parentWeight := &mockWeight{query: query}

	weight := NewBlockJoinWeight(query, childWeight, parentWeight, Total)

	count, err := weight.Count(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should return -1 (cannot compute efficiently)
	if count != -1 {
		t.Errorf("expected count to be -1, got %d", count)
	}
}

// Test BlockJoinWeight GetQuery
func TestBlockJoinWeightGetQuery(t *testing.T) {
	query := &mockQuery{}
	childWeight := &mockWeight{query: query}
	parentWeight := &mockWeight{query: query}

	weight := NewBlockJoinWeight(query, childWeight, parentWeight, Total)

	if weight.GetQuery() != query {
		t.Error("expected GetQuery to return the correct query")
	}
}
