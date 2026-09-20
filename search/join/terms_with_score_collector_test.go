// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"
)

func TestNewTermsWithScoreCollector(t *testing.T) {
	collector := NewTermsWithScoreCollector("test_field", Max)

	if collector == nil {
		t.Fatal("Expected TermsWithScoreCollector to be created")
	}

	if collector.GetField() != "test_field" {
		t.Errorf("Expected field 'test_field', got '%s'", collector.GetField())
	}

	if collector.GetScoreMode() != Max {
		t.Errorf("Expected score mode Max, got %v", collector.GetScoreMode())
	}
}

func TestTermsWithScoreCollectorCollect(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Total)

	// Collect some terms
	err := collector.Collect([]byte("term1"), 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = collector.Collect([]byte("term2"), 2.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = collector.Collect([]byte("term3"), 3.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check total hits
	if collector.GetTotalHits() != 3 {
		t.Errorf("Expected 3 total hits, got %d", collector.GetTotalHits())
	}

	// Check unique terms
	if collector.GetUniqueTermCount() != 3 {
		t.Errorf("Expected 3 unique terms, got %d", collector.GetUniqueTermCount())
	}

	// Get terms
	terms := collector.GetTerms()
	if len(terms) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(terms))
	}
}

func TestTermsWithScoreCollectorDuplicateTerms(t *testing.T) {
	// Test with Total score mode
	collector := NewTermsWithScoreCollector("field", Total)

	// Collect same term multiple times
	collector.Collect([]byte("term1"), 1.0)
	collector.Collect([]byte("term1"), 2.0)
	collector.Collect([]byte("term1"), 3.0)

	// Should have only 1 unique term
	if collector.GetUniqueTermCount() != 1 {
		t.Errorf("Expected 1 unique term, got %d", collector.GetUniqueTermCount())
	}

	// Total hits should be 3
	if collector.GetTotalHits() != 3 {
		t.Errorf("Expected 3 total hits, got %d", collector.GetTotalHits())
	}

	// Score should be sum (6.0) for Total mode
	termsWithScores := collector.GetTermsWithScores()
	if len(termsWithScores) != 1 {
		t.Fatalf("Expected 1 term with score, got %d", len(termsWithScores))
	}

	if termsWithScores[0].Score != 6.0 {
		t.Errorf("Expected score 6.0 for Total mode, got %f", termsWithScores[0].Score)
	}
}

func TestTermsWithScoreCollectorMaxScoreMode(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Max)

	// Collect same term multiple times with different scores
	collector.Collect([]byte("term1"), 1.0)
	collector.Collect([]byte("term1"), 5.0)
	collector.Collect([]byte("term1"), 3.0)

	termsWithScores := collector.GetTermsWithScores()
	if len(termsWithScores) != 1 {
		t.Fatalf("Expected 1 term with score, got %d", len(termsWithScores))
	}

	// Score should be max (5.0) for Max mode
	if termsWithScores[0].Score != 5.0 {
		t.Errorf("Expected score 5.0 for Max mode, got %f", termsWithScores[0].Score)
	}
}

func TestTermsWithScoreCollectorMinScoreMode(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Min)

	// Collect same term multiple times with different scores
	collector.Collect([]byte("term1"), 5.0)
	collector.Collect([]byte("term1"), 1.0)
	collector.Collect([]byte("term1"), 3.0)

	termsWithScores := collector.GetTermsWithScores()
	if len(termsWithScores) != 1 {
		t.Fatalf("Expected 1 term with score, got %d", len(termsWithScores))
	}

	// Score should be min (1.0) for Min mode
	if termsWithScores[0].Score != 1.0 {
		t.Errorf("Expected score 1.0 for Min mode, got %f", termsWithScores[0].Score)
	}
}

func TestTermsWithScoreCollectorAvgScoreMode(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Avg)

	// Collect same term multiple times with different scores
	collector.Collect([]byte("term1"), 2.0)
	collector.Collect([]byte("term1"), 4.0)
	collector.Collect([]byte("term1"), 6.0)

	termsWithScores := collector.GetTermsWithScores()
	if len(termsWithScores) != 1 {
		t.Fatalf("Expected 1 term with score, got %d", len(termsWithScores))
	}

	// Score should be average (4.0) for Avg mode
	if termsWithScores[0].Score != 4.0 {
		t.Errorf("Expected score 4.0 for Avg mode, got %f", termsWithScores[0].Score)
	}
}

func TestTermsWithScoreCollectorGetTopTerms(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Total)

	// Collect terms with different scores
	collector.Collect([]byte("term1"), 1.0)
	collector.Collect([]byte("term2"), 5.0)
	collector.Collect([]byte("term3"), 3.0)
	collector.Collect([]byte("term4"), 2.0)

	// Get top 2 terms
	topTerms := collector.GetTopTerms(2)
	if len(topTerms) != 2 {
		t.Fatalf("Expected 2 top terms, got %d", len(topTerms))
	}

	// Should be sorted by score (highest first)
	if string(topTerms[0].Term) != "term2" {
		t.Errorf("Expected first term to be 'term2', got '%s'", string(topTerms[0].Term))
	}
	if topTerms[0].Score != 5.0 {
		t.Errorf("Expected first score to be 5.0, got %f", topTerms[0].Score)
	}

	if string(topTerms[1].Term) != "term3" {
		t.Errorf("Expected second term to be 'term3', got '%s'", string(topTerms[1].Term))
	}
}

func TestTermsWithScoreCollectorReset(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Total)

	// Collect some terms
	collector.Collect([]byte("term1"), 1.0)
	collector.Collect([]byte("term2"), 2.0)

	// Verify collection
	if collector.GetTotalHits() != 2 {
		t.Errorf("Expected 2 total hits before reset, got %d", collector.GetTotalHits())
	}

	// Reset
	collector.Reset()

	// Verify reset
	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits after reset, got %d", collector.GetTotalHits())
	}

	if collector.GetUniqueTermCount() != 0 {
		t.Errorf("Expected 0 unique terms after reset, got %d", collector.GetUniqueTermCount())
	}

	if len(collector.GetTerms()) != 0 {
		t.Errorf("Expected 0 terms after reset, got %d", len(collector.GetTerms()))
	}
}

func TestTermsWithScoreCollectorEmptyTerm(t *testing.T) {
	collector := NewTermsWithScoreCollector("field", Total)

	// Collect empty term
	err := collector.Collect([]byte{}, 1.0)
	if err != nil {
		t.Errorf("Unexpected error for empty term: %v", err)
	}

	// Should not add empty term
	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits for empty term, got %d", collector.GetTotalHits())
	}
}

func TestNewTermsWithScoreCollectorManager(t *testing.T) {
	manager := NewTermsWithScoreCollectorManager("test_field", Max)

	if manager == nil {
		t.Fatal("Expected TermsWithScoreCollectorManager to be created")
	}

	if manager.field != "test_field" {
		t.Errorf("Expected field 'test_field', got '%s'", manager.field)
	}

	if manager.scoreMode != Max {
		t.Errorf("Expected score mode Max, got %v", manager.scoreMode)
	}
}

func TestTermsWithScoreCollectorManagerReduce(t *testing.T) {
	manager := NewTermsWithScoreCollectorManager("field", Total)

	// Create multiple collectors
	collector1 := NewTermsWithScoreCollector("field", Total)
	collector1.Collect([]byte("term1"), 1.0)
	collector1.Collect([]byte("term2"), 2.0)

	collector2 := NewTermsWithScoreCollector("field", Total)
	collector2.Collect([]byte("term2"), 3.0)
	collector2.Collect([]byte("term3"), 4.0)

	// Reduce
	merged, err := manager.Reduce([]*TermsWithScoreCollector{collector1, collector2})
	if err != nil {
		t.Fatalf("Unexpected error during reduce: %v", err)
	}

	// Check merged results
	if merged.GetUniqueTermCount() != 3 {
		t.Errorf("Expected 3 unique terms after reduce, got %d", merged.GetUniqueTermCount())
	}

	// Check term2 score (should be 2.0 + 3.0 = 5.0 for Total mode)
	termsWithScores := merged.GetTermsWithScores()
	var term2Score float32
	for _, tws := range termsWithScores {
		if string(tws.Term) == "term2" {
			term2Score = tws.Score
			break
		}
	}
	if term2Score != 5.0 {
		t.Errorf("Expected term2 score 5.0 after reduce, got %f", term2Score)
	}
}

func TestTermWithScoreString(t *testing.T) {
	tws := TermWithScore{Term: []byte("test"), Score: 1.5}
	str := tws.String()

	expected := "TermWithScore{term=test, score=1.500000}"
	if str != expected {
		t.Errorf("Expected string '%s', got '%s'", expected, str)
	}
}
