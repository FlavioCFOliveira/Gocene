// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewDrillSidewaysWeight(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	// Create weight
	var searcher *search.IndexSearcher
	weight, err := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if weight == nil {
		t.Fatal("expected weight to be created")
	}
	if weight.query != dsq {
		t.Error("expected query to be set")
	}
}

func TestDrillSidewaysWeightIsCacheable(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	// Should be cacheable when no drill-downs
	if !weight.IsCacheable(nil) {
		t.Error("expected weight to be cacheable with no drill-downs")
	}
}

func TestDrillSidewaysWeightExplain(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	// Explain should not panic - it may return nil with nil context
	// This is expected behavior when context is nil
	_, _ = weight.Explain(nil, 0)
	// Just verify it doesn't panic
}

func TestNewDrillSidewaysScorer(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	if scorer == nil {
		t.Fatal("expected scorer to be created")
	}
	if scorer.weight != weight {
		t.Error("expected weight to be set")
	}
}

func TestDrillSidewaysScorerDocID(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// Initial doc ID should be -1
	if scorer.DocID() != -1 {
		t.Errorf("expected initial doc ID to be -1, got %d", scorer.DocID())
	}
}

func TestDrillSidewaysScorerScore(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// Score should be 1.0 with no base scorer
	score := scorer.Score()
	if score != 1.0 {
		t.Errorf("expected score 1.0, got %f", score)
	}
}

func TestDrillSidewaysScorerCost(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// Cost should be 0 with no base scorer
	cost := scorer.Cost()
	if cost != 0 {
		t.Errorf("expected cost 0, got %d", cost)
	}
}

func TestDrillSidewaysScorerGetMaxScore(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// Max score should be 1.0 with no base scorer
	maxScore := scorer.GetMaxScore(100)
	if maxScore != 1.0 {
		t.Errorf("expected max score 1.0, got %f", maxScore)
	}
}

func TestDrillSidewaysScorerDocIDRunEnd(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// DocIDRunEnd should return currentDoc + 1
	scorer.currentDoc = 10
	runEnd := scorer.DocIDRunEnd()
	if runEnd != 11 {
		t.Errorf("expected DocIDRunEnd 11, got %d", runEnd)
	}
}

func TestDrillSidewaysScorerGetBaseScorer(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// GetBaseScorer should return nil
	if scorer.GetBaseScorer() != nil {
		t.Error("expected GetBaseScorer to return nil")
	}
}

func TestDrillSidewaysScorerGetDrillDownScorer(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	drillDownScorers := make(map[string]search.Scorer)
	scorer := NewDrillSidewaysScorer(weight, nil, drillDownScorers, nil)

	// GetDrillDownScorer should return nil for non-existent dimension
	if scorer.GetDrillDownScorer("nonexistent") != nil {
		t.Error("expected GetDrillDownScorer to return nil for non-existent dimension")
	}
}

func TestDrillSidewaysScorerMatches(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	var searcher *search.IndexSearcher
	weight, _ := NewDrillSidewaysWeight(dsq, searcher, false, 1.0)

	scorer := NewDrillSidewaysScorer(weight, nil, nil, nil)

	// Matches should return false when currentDoc is NO_MORE_DOCS
	scorer.currentDoc = search.NO_MORE_DOCS
	if scorer.Matches() {
		t.Error("expected Matches to return false when currentDoc is NO_MORE_DOCS")
	}
}
