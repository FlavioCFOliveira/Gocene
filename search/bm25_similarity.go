// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// BM25Similarity implements BM25 scoring.
type BM25Similarity struct {
	*BaseSimilarity
	k1 float64 // Controls term frequency saturation
	b  float64 // Controls document length normalization
}

// NewBM25Similarity creates a new BM25Similarity with default parameters.
func NewBM25Similarity() *BM25Similarity {
	return &BM25Similarity{
		BaseSimilarity: NewBaseSimilarity(),
		k1:             1.2,
		b:              0.75,
	}
}

// NewBM25SimilarityWithParams creates a BM25Similarity with custom parameters.
func NewBM25SimilarityWithParams(k1, b float64) *BM25Similarity {
	if math.IsNaN(k1) || k1 < 0 || math.IsInf(k1, 0) {
		panic("illegal k1 value")
	}
	if math.IsNaN(b) || b < 0 || b > 1 || math.IsInf(b, 0) {
		panic("illegal b value")
	}
	return &BM25Similarity{
		BaseSimilarity: NewBaseSimilarity(),
		k1:             k1,
		b:              b,
	}
}

// K1 returns the k1 parameter.
func (s *BM25Similarity) K1() float64 { return s.k1 }

// B returns the b parameter.
func (s *BM25Similarity) B() float64 { return s.b }

// ComputeNorm computes the norm value considering document length.
func (s *BM25Similarity) ComputeNorm(field string, stats interface{}) float32 {
	// Simplified implementation
	return 1.0
}

// ScoreBM25 calculates the BM25 score.
func (s *BM25Similarity) ScoreBM25(freq, docLength, avgDocLength, idf float64) float64 {
	norm := (1 - s.b) + s.b*(docLength/avgDocLength)
	tfComponent := freq / (freq + s.k1*norm)
	return idf * tfComponent
}

// InverseDocumentFrequency computes IDF.
func (s *BM25Similarity) InverseDocumentFrequency(totalDocs, docFreq int) float64 {
	return math.Log(1 + (float64(totalDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5))
}
