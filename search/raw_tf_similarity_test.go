// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

// TestRawTFSimilarity_Score verifies the score = boost * freq formula
// across a small grid of freq/boost values.
func TestRawTFSimilarity_Score(t *testing.T) {
	cases := []struct {
		boost float32
		freq  float32
		want  float32
	}{
		{1, 1, 1},
		{1, 3.5, 3.5},
		{2, 4, 8},
		{0.5, 6, 3},
		{0, 100, 0},
	}
	sim := NewRawTFSimilarity()
	for _, c := range cases {
		sc := sim.Scorer104(c.boost, nil)
		got := sc.Score104(c.freq, 1)
		if math.Abs(float64(got-c.want)) > 1e-6 {
			t.Fatalf("boost=%v freq=%v: got %v, want %v", c.boost, c.freq, got, c.want)
		}
	}
}

// TestRawTFSimilarity_IgnoresNorm checks that the norm byte is irrelevant
// to the score, matching the Java reference.
func TestRawTFSimilarity_IgnoresNorm(t *testing.T) {
	sim := NewRawTFSimilarity()
	sc := sim.Scorer104(1.0, nil)
	a := sc.Score104(5, 1)
	b := sc.Score104(5, 255)
	c := sc.Score104(5, 42)
	if a != b || a != c {
		t.Fatalf("norm must not influence score: got %v %v %v", a, b, c)
	}
}

// TestRawTFSimilarity_DiscountOverlapsDefault verifies the default flag.
func TestRawTFSimilarity_DiscountOverlapsDefault(t *testing.T) {
	if !NewRawTFSimilarity().GetDiscountOverlaps() {
		t.Fatal("default discountOverlaps must be true")
	}
	if NewRawTFSimilarityWithDiscount(false).GetDiscountOverlaps() {
		t.Fatal("expert constructor must honour discountOverlaps=false")
	}

// TestRawTFSimilarity_Explain verifies that the explanation tree carries
// the score as its root value and the supplied freq Explanation as detail.
func TestRawTFSimilarity_Explain(t *testing.T) {
	sim := NewRawTFSimilarity()
	sc := sim.Scorer104(2.0, nil)
	freq := NewExplanation(true, 3, "freq")
	exp := sc.Explain104(freq, 1)
	if !exp.IsMatch() {
		t.Fatal("explanation must be a match")
	}
	if got := exp.GetValue(); got != 6 {
		t.Fatalf("explanation value: got %v, want 6", got)
	}
	details := exp.GetDetails()
	if len(details) != 1 || details[0] != Explanation(freq) {
		t.Fatalf("explanation must contain the freq detail, got %+v", details)
	}
}