// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bitvectors

import "testing"

// TestFlatBitVectorsScorer_Score verifies the population-count scoring
// of 1-bit quantised vectors.
func TestFlatBitVectorsScorer_Score(t *testing.T) {
	scorer := FlatBitVectorsScorer{}

	// Two identical vectors → popcount of each byte.
	a := []byte{0b1111_0000, 0b1010_1010}
	b := []byte{0b1111_0000, 0b1010_1010}
	got := scorer.Score(a, b)
	want := 4 + 4 // 4 bits set in first byte, 4 in second
	if got != want {
		t.Errorf("identical vectors: got %d, want %d", got, want)
	}

	// Orthogonal vectors → zero score.
	c := []byte{0b1111_1111, 0b0000_0000}
	d := []byte{0b0000_0000, 0b1111_1111}
	got = scorer.Score(c, d)
	want = 0
	if got != want {
		t.Errorf("orthogonal vectors: got %d, want %d", got, want)
	}

	// Mismatched lengths → zero score (defensive).
	e := []byte{0b1111_1111}
	f := []byte{0b1111_1111, 0b0000_0000}
	got = scorer.Score(e, f)
	want = 0
	if got != want {
		t.Errorf("mismatched lengths: got %d, want %d", got, want)
	}
}

// TestHnswBitVectorsFormat_Defaults verifies default parameter injection.
func TestHnswBitVectorsFormat_Defaults(t *testing.T) {
	f := NewHnswBitVectorsFormat(0, 0)
	if f.MaxConn != 16 {
		t.Errorf("MaxConn default: got %d, want 16", f.MaxConn)
	}
	if f.BeamWidth != 100 {
		t.Errorf("BeamWidth default: got %d, want 100", f.BeamWidth)
	}
}

// TestHnswBitVectorsFormat_Custom verifies custom parameter retention.
func TestHnswBitVectorsFormat_Custom(t *testing.T) {
	f := NewHnswBitVectorsFormat(32, 200)
	if f.MaxConn != 32 {
		t.Errorf("MaxConn: got %d, want 32", f.MaxConn)
	}
	if f.BeamWidth != 200 {
		t.Errorf("BeamWidth: got %d, want 200", f.BeamWidth)
	}
}
