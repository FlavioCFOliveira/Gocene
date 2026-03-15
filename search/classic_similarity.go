// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// ClassicSimilarity implements the classic Lucene TF/IDF scoring.
// This is the original Lucene scoring formula before BM25 was introduced.
// It uses term frequency (TF) and inverse document frequency (IDF) with
// document length normalization.
type ClassicSimilarity struct {
	*BaseSimilarity
}

// NewClassicSimilarity creates a new ClassicSimilarity with default parameters.
func NewClassicSimilarity() *ClassicSimilarity {
	return &ClassicSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
	}
}

// ComputeNorm computes the normalization value for a field.
// In ClassicSimilarity, norms are encoded as 1/sqrt(length).
func (s *ClassicSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	// Simplified implementation - full implementation would use norms
	return 1.0
}

// Tf computes the term frequency component.
// ClassicSimilarity uses sqrt(freq) for term frequency.
func (s *ClassicSimilarity) Tf(freq float64) float64 {
	return math.Sqrt(freq)
}

// Idf computes the inverse document frequency.
// ClassicSimilarity uses log(N/n) where N is total docs and n is doc frequency.
func (s *ClassicSimilarity) Idf(totalDocs, docFreq int) float64 {
	return math.Log(float64(totalDocs) / float64(docFreq))
}

// IdfExplain computes IDF with explanation.
// Similar to Lucene's ClassicSimilarity.idfExplain method.
func (s *ClassicSimilarity) IdfExplain(totalDocs, docFreq int) float64 {
	return s.Idf(totalDocs, docFreq)
}

// LengthNorm computes the length normalization factor.
// In ClassicSimilarity, this is 1/sqrt(numTerms).
func (s *ClassicSimilarity) LengthNorm(numTerms int) float64 {
	return 1.0 / math.Sqrt(float64(numTerms))
}

// ScoreTfIdf calculates the TF/IDF score.
// The formula is: tf * idf * boost * lengthNorm
func (s *ClassicSimilarity) ScoreTfIdf(freq float64, totalDocs, docFreq, numTerms int, boost float64) float64 {
	tf := s.Tf(freq)
	idf := s.Idf(totalDocs, docFreq)
	lengthNorm := s.LengthNorm(numTerms)
	return tf * idf * boost * lengthNorm
}

// Score calculates the classic TF/IDF score.
// This is a simplified version for basic scoring.
func (s *ClassicSimilarity) Score(freq float64, totalDocs, docFreq int) float64 {
	tf := s.Tf(freq)
	idf := s.Idf(totalDocs, docFreq)
	return tf * idf
}

// QueryNorm computes the query normalization factor.
// This normalizes query weights so that the sum of squared weights equals 1.
func (s *ClassicSimilarity) QueryNorm(sumOfSquaredWeights float64) float64 {
	return 1.0 / math.Sqrt(sumOfSquaredWeights)
}

// Coord is the coordination factor.
// Rewards documents that contain more query terms.
// Returns overlap / maxOverlap.
func (s *ClassicSimilarity) Coord(overlap, maxOverlap int) float64 {
	return float64(overlap) / float64(maxOverlap)
}

// SloppyFreq computes the sloppy term frequency.
// Used for phrase queries with slop (proximity matching).
func (s *ClassicSimilarity) SloppyFreq(distance int) float64 {
	return 1.0 / (float64(distance) + 1.0)
}

// EncodeNorm encodes a normalization value.
// In Lucene, norms are encoded as a single byte.
func (s *ClassicSimilarity) EncodeNorm(norm float64) byte {
	// Lucene encodes norms as bytes (0-255 range)
	// This is a simplified encoding
	if norm <= 0 {
		return 0
	}
	if norm >= 1 {
		return 255
	}
	return byte(norm * 255)
}

// DecodeNorm decodes a normalization value.
func (s *ClassicSimilarity) DecodeNorm(encoded byte) float64 {
	return float64(encoded) / 255.0
}

// String returns a string representation of this similarity.
func (s *ClassicSimilarity) String() string {
	return "ClassicSimilarity"
}

// ComputeWeight computes the weight for a query (implements Similarity interface).
func (s *ClassicSimilarity) ComputeWeight(queryWeight float32, stats interface{}) Weight {
	return nil
}

// Scorer creates a SimScorer for scoring documents (implements Similarity interface).
func (s *ClassicSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewClassicSimScorer(s, collectionStats, termStats)
}
