// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// ClassicSimScorer implements TF/IDF scoring for ClassicSimilarity.
//
// This is the Go port of Lucene's ClassicSimilarity scoring logic.
type ClassicSimScorer struct {
	*BaseSimScorer
	similarity *ClassicSimilarity
	idf        float64
	weight     float64
}

// NewClassicSimScorer creates a new ClassicSimScorer.
func NewClassicSimScorer(similarity *ClassicSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *ClassicSimScorer {
	idf := 1.0
	if termStats != nil && collectionStats != nil && termStats.DocFreq() > 0 {
		idf = similarity.Idf(collectionStats.MaxDoc(), termStats.DocFreq())
	}
	return &ClassicSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		idf:           idf,
		weight:        1.0,
	}
}

// Score calculates the TF/IDF score for a document.
// Formula: tf * idf * weight
func (s *ClassicSimScorer) Score(doc int, freq float32) float32 {
	tf := s.similarity.Tf(float64(freq))
	score := tf * s.idf * s.weight
	return float32(score)
}

// Idf returns the IDF value.
func (s *ClassicSimScorer) Idf() float64 {
	return s.idf
}

// Ensure ClassicSimScorer implements SimScorer
var _ SimScorer = (*ClassicSimScorer)(nil)

// tfCalculator computes TF values
type tfCalculator struct{}

// Tf computes the term frequency component.
func tf(freq float64) float64 {
	return math.Sqrt(freq)
}

// idfCalculator computes IDF values
type idfCalculator struct{}

// Idf computes the inverse document frequency.
func idf(totalDocs, docFreq int) float64 {
	if docFreq == 0 {
		return 0
	}
	return math.Log(float64(totalDocs) / float64(docFreq))
}
