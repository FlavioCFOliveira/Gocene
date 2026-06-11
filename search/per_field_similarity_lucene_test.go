// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLucenePerFieldSimilarityWrapper_DispatchByField verifies that the
// wrapper routes Scorer104 to the per-field Similarity.
func TestLucenePerFieldSimilarityWrapper_DispatchByField(t *testing.T) {
	tf := NewRawTFSimilarity()
	boolean := NewLuceneBooleanSimilarity()
	w := NewLucenePerFieldSimilarityWrapper(func(field string) LuceneSimilarity {
		if field == "title" {
			return boolean
		}
		return tf
	})

	csTitle := NewCollectionStatistics("title", 100, 80, 800, 200)
	csBody := NewCollectionStatistics("body", 100, 80, 800, 200)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 10, 25)

	titleScorer := w.Scorer104(1.0, csTitle, ts)
	bodyScorer := w.Scorer104(2.0, csBody, ts)

	if got := titleScorer.Score104(7, 1); got != 1.0 {
		t.Fatalf("title (boolean) score: got %v, want 1.0", got)
	}
	if got := bodyScorer.Score104(7, 1); got != 14.0 {
		t.Fatalf("body (raw tf) score: got %v, want 14.0 (boost*freq)", got)
	}
}

// TestLucenePerFieldSimilarityWrapper_ComputeNormDispatch verifies that
// the norm calculation also routes through Get(field).
func TestLucenePerFieldSimilarityWrapper_ComputeNormDispatch(t *testing.T) {
	bm25Discount := NewLuceneBM25SimilarityFull(1.2, 0.75, true)
	bm25NoDiscount := NewLuceneBM25SimilarityFull(1.2, 0.75, false)
	w := NewLucenePerFieldSimilarityWrapper(func(field string) LuceneSimilarity {
		if field == "discount" {
			return bm25Discount
		}
		return bm25NoDiscount
	})

	stateDiscount := index.NewFieldInvertStateFull(10, "discount", index.IndexOptionsDocsAndFreqs,
		0, 10, 3, 0, 0, 0)
	stateNoDiscount := index.NewFieldInvertStateFull(10, "no", index.IndexOptionsDocsAndFreqs,
		0, 10, 3, 0, 0, 0)

	if got, want := w.ComputeNormFromInvertState(stateDiscount), bm25Discount.ComputeNormFromInvertState(stateDiscount); got != want {
		t.Fatalf("discount norm: got %d, want %d", got, want)
	}
	if got, want := w.ComputeNormFromInvertState(stateNoDiscount), bm25NoDiscount.ComputeNormFromInvertState(stateNoDiscount); got != want {
		t.Fatalf("no-discount norm: got %d, want %d", got, want)
	}

// TestLucenePerFieldSimilarityWrapper_NilGetterPanics defends the contract.
func TestLucenePerFieldSimilarityWrapper_NilGetterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil getter")
		}
	}()
	NewLucenePerFieldSimilarityWrapper(nil)
}

// TestLucenePerFieldSimilarityWrapper_GetterReturnsNil verifies the
// defensive fallback to a noop scorer.
func TestLucenePerFieldSimilarityWrapper_GetterReturnsNil(t *testing.T) {
	w := NewLucenePerFieldSimilarityWrapper(func(_ string) LuceneSimilarity { return nil })
	cs := NewCollectionStatistics("title", 100, 80, 800, 200)
	sc := w.Scorer104(1.0, cs)
	if got := sc.Score104(7, 1); got != 0 {
		t.Fatalf("noop fallback score: got %v, want 0", got)
	}
}