// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

// TestLuceneBooleanSimilarity_Score verifies that the score equals the
// configured boost regardless of freq/norm.
func TestLuceneBooleanSimilarity_Score(t *testing.T) {
	s := NewLuceneBooleanSimilarity()
	for _, boost := range []float32{0.25, 1.0, 7.5} {
		scorer := s.Scorer104(boost, nil)
		for _, freq := range []float32{1, 5, 100} {
			for _, norm := range []int64{1, 64, 255} {
				if got := scorer.Score104(freq, norm); got != boost {
					t.Fatalf("boost=%v freq=%v norm=%d: got %v, want %v", boost, freq, norm, got, boost)
				}
			}
		}
	}
}

// TestLuceneBooleanSimilarity_DiscountOverlapsTrue verifies the hard-
// coded flag.
func TestLuceneBooleanSimilarity_DiscountOverlapsTrue(t *testing.T) {
	if !NewLuceneBooleanSimilarity().GetDiscountOverlaps() {
		t.Fatal("discountOverlaps must always be true")
	}
}

// TestLuceneBooleanSimilarity_Explain verifies that the explanation
// surfaces the boost as the score.
func TestLuceneBooleanSimilarity_Explain(t *testing.T) {
	s := NewLuceneBooleanSimilarity()
	scorer := s.Scorer104(3.0, nil)
	exp := scorer.Explain104(NewExplanation(true, 5, "freq"), 1)
	if !exp.IsMatch() {
		t.Fatal("explain should be a match")
	}
	if got := exp.GetValue(); got != 3.0 {
		t.Fatalf("explain value: got %v, want 3.0", got)
	}
}

// TestLuceneTFIDFSimilarity_PhraseIdfSum verifies that the phrase idfExplain
// sums per-term idfs.
func TestLuceneTFIDFSimilarity_PhraseIdfSum(t *testing.T) {
	// We use ClassicSimilarity to exercise the TFIDF base. The phrase
	// idf is the sum of per-term idfs.
	s := NewLuceneClassicSimilarity()
	cs := NewCollectionStatistics("body", 100, 100, 1000, 100)
	ts1 := NewTermStatistics(nil, 10, 100)
	ts2 := NewTermStatistics(nil, 20, 200)
	phrase := s.IdfExplainPhrase(cs, []*TermStatistics{ts1, ts2})
	single1 := s.IdfExplainSingle(cs, ts1)
	single2 := s.IdfExplainSingle(cs, ts2)
	got := phrase.GetValue()
	want := single1.GetValue() + single2.GetValue()
	if got != want {
		t.Fatalf("phrase idf: got %v, want %v", got, want)
	}
}
