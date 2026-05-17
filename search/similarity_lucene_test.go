// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestDefaultComputeNormFromInvertState_DocsOnly verifies that the
// DOCS-only branch uses UniqueTermCount, matching Lucene 10.4.0.
func TestDefaultComputeNormFromInvertState_DocsOnly(t *testing.T) {
	s := index.NewFieldInvertStateFull(10, "f", index.IndexOptionsDocs,
		/*position*/ 0,
		/*length*/ 17,
		/*numOverlap*/ 3,
		/*offset*/ 0,
		/*maxTermFrequency*/ 0,
		/*uniqueTermCount*/ 5)
	got := DefaultComputeNormFromInvertState(s, true)
	want, err := util.IntToByte4(5)
	if err != nil {
		t.Fatalf("IntToByte4(5): %v", err)
	}
	if got != int64(want) {
		t.Fatalf("DOCS branch: got %d, want %d", got, want)
	}
}

// TestDefaultComputeNormFromInvertState_DiscountOverlaps verifies the
// length-minus-overlap branch used when discountOverlaps is true.
func TestDefaultComputeNormFromInvertState_DiscountOverlaps(t *testing.T) {
	s := index.NewFieldInvertStateFull(10, "f", index.IndexOptionsDocsAndFreqs,
		0, 12, 2, 0, 0, 0)
	got := DefaultComputeNormFromInvertState(s, true)
	want, _ := util.IntToByte4(10) // 12 - 2
	if got != int64(want) {
		t.Fatalf("discountOverlaps: got %d, want %d", got, want)
	}
}

// TestDefaultComputeNormFromInvertState_NoDiscount verifies that when
// discountOverlaps is false the raw length is encoded.
func TestDefaultComputeNormFromInvertState_NoDiscount(t *testing.T) {
	s := index.NewFieldInvertStateFull(10, "f", index.IndexOptionsDocsAndFreqs,
		0, 12, 2, 0, 0, 0)
	got := DefaultComputeNormFromInvertState(s, false)
	want, _ := util.IntToByte4(12)
	if got != int64(want) {
		t.Fatalf("no discount: got %d, want %d", got, want)
	}
}

// TestDefaultComputeNormFromInvertState_NilState defends against accidental
// nil state pointers; Lucene cannot pass null here but we keep Gocene defensive.
func TestDefaultComputeNormFromInvertState_NilState(t *testing.T) {
	if got := DefaultComputeNormFromInvertState(nil, true); got != 1 {
		t.Fatalf("nil state: got %d, want 1", got)
	}
}

// fakeLuceneSimScorer is a deterministic LuceneSimScorer used to exercise
// DefaultBulkSimScorer without dragging in a full Similarity implementation.
type fakeLuceneSimScorer struct{}

func (fakeLuceneSimScorer) Score104(freq float32, norm int64) float32 {
	return freq + float32(norm)
}

func (s fakeLuceneSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

func (fakeLuceneSimScorer) Explain104(freq Explanation, _ int64) Explanation {
	return NewExplanation(true, freq.GetValue(), "fake")
}

// TestDefaultBulkSimScorer_RoundTrip verifies that the bulk scorer matches
// the single-document scorer for the same inputs.
func TestDefaultBulkSimScorer_RoundTrip(t *testing.T) {
	scorer := fakeLuceneSimScorer{}
	bulk := scorer.AsBulkSimScorer()
	freqs := []float32{1, 2, 3, 4}
	norms := []int64{10, 20, 30, 40}
	got := make([]float32, len(freqs))
	bulk.ScoreBulk(len(freqs), freqs, norms, got)
	for i := range freqs {
		want := scorer.Score104(freqs[i], norms[i])
		if got[i] != want {
			t.Fatalf("bulk[%d]: got %f, want %f", i, got[i], want)
		}
	}
}

// TestDefaultBulkSimScorer_NilScorerPanics defends against accidental nil
// scorer wrapping.
func TestDefaultBulkSimScorer_NilScorerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil scorer")
		}
	}()
	NewDefaultBulkSimScorer(nil)
}
