// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMaxScoreBulkScorer.java
//
// The Java tests are integration-level (require IndexWriter / IndexSearcher).
// These tests cover the Go public contract with synthetic scorers.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMaxScoreBulkScorer_InnerWindowSize verifies the exported constant.
func TestMaxScoreBulkScorer_InnerWindowSize(t *testing.T) {
	if search.InnerWindowSize != 1<<12 {
		t.Errorf("InnerWindowSize=%d, want %d", search.InnerWindowSize, 1<<12)
	}
}

// TestMaxScoreBulkScorer_CollectsAllDocs verifies all matching docs are
// collected when acceptDocs is nil.
func TestMaxScoreBulkScorer_CollectsAllDocs(t *testing.T) {
	sA := newConstantScorer([]int{0, 1, 3}, 2, 2)
	sB := newConstantScorer([]int{0, 3, 4, 5}, 1, 1)
	bs := search.NewMaxScoreBulkScorer(100, []search.Scorer{sA, sB}, nil)

	lc := &batchLeafCollector{}
	if err := bs.Score(lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	wantDocs := []int{0, 1, 3, 4, 5}
	if len(lc.docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", lc.docs, wantDocs)
	}
	for i, d := range wantDocs {
		if lc.docs[i] != d {
			t.Errorf("docs[%d]=%d, want %d", i, lc.docs[i], d)
		}
	}
}

// TestMaxScoreBulkScorer_SumScores verifies scores are summed for docs that
// match multiple scorers.
func TestMaxScoreBulkScorer_SumScores(t *testing.T) {
	// doc 0 matches sA (score=2) and sB (score=1) → expected sum=3
	sA := newConstantScorer([]int{0, 1}, 2, 2)
	sB := newConstantScorer([]int{0, 3}, 1, 1)
	bs := search.NewMaxScoreBulkScorer(100, []search.Scorer{sA, sB}, nil)

	lc := &batchLeafCollector{}
	if err := bs.Score(lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	// docs: 0 (sA+sB=3), 1 (sA=2), 3 (sB=1)
	wantDocs := []int{0, 1, 3}
	wantScores := []float32{3, 2, 1}
	if len(lc.docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", lc.docs, wantDocs)
	}
	const eps = float32(1e-4)
	for i, want := range wantScores {
		got := lc.scores[i]
		if got < want-eps || got > want+eps {
			t.Errorf("scores[%d]=%v, want %v", i, got, want)
		}
	}
}

// TestMaxScoreBulkScorer_Cost verifies Cost() equals sum of all scorer costs.
func TestMaxScoreBulkScorer_Cost(t *testing.T) {
	sA := newConstantScorer(make([]int, 10), 1, 1)
	sB := newConstantScorer(make([]int, 20), 1, 1)
	bs := search.NewMaxScoreBulkScorer(100, []search.Scorer{sA, sB}, nil)
	if bs.Cost() != 30 {
		t.Errorf("Cost()=%d, want 30", bs.Cost())
	}
}

// TestMaxScoreBulkScorer_EmptyScorer verifies no docs are collected.
func TestMaxScoreBulkScorer_EmptyScorer(t *testing.T) {
	sA := newConstantScorer([]int{}, 1, 1)
	bs := search.NewMaxScoreBulkScorer(100, []search.Scorer{sA}, nil)
	lc := &batchLeafCollector{}
	if err := bs.Score(lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if len(lc.docs) != 0 {
		t.Errorf("docs=%v, want empty", lc.docs)
	}
}

// TestMaxScoreBulkScorer_ImplementsBulkScorer checks interface satisfaction.
func TestMaxScoreBulkScorer_ImplementsBulkScorer(t *testing.T) {
	sA := newConstantScorer([]int{0}, 1, 1)
	var _ search.BulkScorer = search.NewMaxScoreBulkScorer(100, []search.Scorer{sA}, nil)
}

// TestMaxScoreBulkScorer_IntegrationBasics mirrors the structure of
// TestMaxScoreBulkScorer.testBasicsWithTwoDisjunctionClauses but without
// a real index — uses synthetic scorers instead.
// Degraded: block-max optimisations (advanceShallow) are not active.
func TestMaxScoreBulkScorer_IntegrationBasics(t *testing.T) {
	t.Skip("full integration test requires IndexWriter/IndexSearcher — deferred")
}
