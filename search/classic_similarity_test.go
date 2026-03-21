// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

func TestClassicSimilarity_Tf(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		freq     float64
		expected float64
	}{
		{1, 1.0},
		{4, 2.0},
		{9, 3.0},
		{0, 0.0},
		{100, 10.0},
	}

	for _, test := range tests {
		result := sim.Tf(test.freq)
		if math.Abs(result-test.expected) > 0.0001 {
			t.Errorf("Tf(%f) = %f, expected %f", test.freq, result, test.expected)
		}
	}
}

func TestClassicSimilarity_Idf(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		totalDocs int
		docFreq   int
		expected  float64
	}{
		{100, 1, math.Log(100)},
		{100, 10, math.Log(10)},
		{100, 50, math.Log(2)},
		{100, 100, math.Log(1)},
		{1000, 10, math.Log(100)},
	}

	for _, test := range tests {
		result := sim.Idf(test.totalDocs, test.docFreq)
		expected := test.expected
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("Idf(%d, %d) = %f, expected %f", test.totalDocs, test.docFreq, result, expected)
		}
	}
}

func TestClassicSimilarity_LengthNorm(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		numTerms int
		expected float64
	}{
		{1, 1.0},
		{4, 0.5},
		{9, 1.0 / 3.0},
		{100, 0.1},
	}

	for _, test := range tests {
		result := sim.LengthNorm(test.numTerms)
		if math.Abs(result-test.expected) > 0.0001 {
			t.Errorf("LengthNorm(%d) = %f, expected %f", test.numTerms, result, test.expected)
		}
	}
}

func TestClassicSimilarity_ScoreTfIdf(t *testing.T) {
	sim := NewClassicSimilarity()

	// Test with freq=4, totalDocs=100, docFreq=10, numTerms=4, boost=1.0
	// tf = sqrt(4) = 2
	// idf = log(100/10) = log(10) ≈ 2.302
	// lengthNorm = 1/sqrt(4) = 0.5
	// score = 2 * 2.302 * 1.0 * 0.5 ≈ 2.302
	score := sim.ScoreTfIdf(4, 100, 10, 4, 1.0)
	expected := 2.0 * math.Log(10) * 0.5
	if math.Abs(score-expected) > 0.0001 {
		t.Errorf("ScoreTfIdf(4, 100, 10, 4, 1.0) = %f, expected %f", score, expected)
	}
}

func TestClassicSimilarity_QueryNorm(t *testing.T) {
	sim := NewClassicSimilarity()

	// QueryNorm(4) = 1/sqrt(4) = 0.5
	result := sim.QueryNorm(4.0)
	expected := float32(0.5)
	if math.Abs(float64(result-expected)) > 0.0001 {
		t.Errorf("QueryNorm(4.0) = %f, expected %f", result, expected)
	}

	// QueryNorm(1) = 1/sqrt(1) = 1.0
	result = sim.QueryNorm(1.0)
	expected = float32(1.0)
	if math.Abs(float64(result-expected)) > 0.0001 {
		t.Errorf("QueryNorm(1.0) = %f, expected %f", result, expected)
	}
}

func TestClassicSimilarity_Coord(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		overlap    int
		maxOverlap int
		expected   float32
	}{
		{1, 1, 1.0},
		{2, 3, float32(2.0 / 3.0)},
		{3, 3, 1.0},
		{0, 5, 0.0},
	}

	for _, test := range tests {
		result := sim.Coord(test.overlap, test.maxOverlap)
		if math.Abs(float64(result-test.expected)) > 0.0001 {
			t.Errorf("Coord(%d, %d) = %f, expected %f", test.overlap, test.maxOverlap, result, test.expected)
		}
	}
}

func TestClassicSimilarity_SloppyFreq(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		distance int
		expected float64
	}{
		{0, 1.0},
		{1, 0.5},
		{2, 1.0 / 3.0},
		{9, 0.1},
	}

	for _, test := range tests {
		result := sim.SloppyFreq(test.distance)
		if math.Abs(result-test.expected) > 0.0001 {
			t.Errorf("SloppyFreq(%d) = %f, expected %f", test.distance, result, test.expected)
		}
	}
}

func TestClassicSimilarity_EncodeDecodeNorm(t *testing.T) {
	sim := NewClassicSimilarity()

	tests := []struct {
		norm float64
	}{
		{0.0},
		{0.5},
		{1.0},
		{0.25},
		{0.75},
	}

	for _, test := range tests {
		encoded := sim.EncodeNorm(test.norm)
		decoded := sim.DecodeNorm(encoded)
		// Allow for some precision loss due to byte encoding
		if math.Abs(decoded-test.norm) > 0.01 {
			t.Errorf("Encode/Decode norm %f: decoded %f, expected close to %f", test.norm, decoded, test.norm)
		}
	}
}

func TestClassicSimilarity_String(t *testing.T) {
	sim := NewClassicSimilarity()
	if sim.String() != "ClassicSimilarity" {
		t.Errorf("String() = %s, expected ClassicSimilarity", sim.String())
	}
}

func TestClassicSimilarity_CompareWithBM25(t *testing.T) {
	// Compare scoring between ClassicSimilarity and BM25Similarity
	classic := NewClassicSimilarity()
	bm25 := NewBM25Similarity()

	// Both should produce reasonable scores, but with different formulas
	// TF/IDF: tf = sqrt(freq), idf = log(N/n)
	// BM25: uses saturation and length normalization

	// Test with same parameters
	freq := float64(3)
	totalDocs := 100
	docFreq := 10

	classicScore := classic.Score(freq, totalDocs, docFreq)
	bm25Score := bm25.ScoreBM25(freq, 100, 100, bm25.InverseDocumentFrequency(totalDocs, docFreq))

	// Both should produce positive scores
	if classicScore <= 0 {
		t.Error("ClassicSimilarity should produce positive scores")
	}
	if bm25Score <= 0 {
		t.Error("BM25Similarity should produce positive scores")
	}

	// Log the difference for information
	t.Logf("Classic score: %f, BM25 score: %f", classicScore, bm25Score)
}
