// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SimScorer scores documents based on similarity.
type SimScorer interface {
	// Score scores a document given the term frequency and norm.
	Score(doc int, freq float32) float32
}

// BaseSimScorer provides common functionality.
type BaseSimScorer struct{}

// NewBaseSimScorer creates a new BaseSimScorer.
func NewBaseSimScorer() *BaseSimScorer {
	return &BaseSimScorer{}
}

// Score returns a default score.
func (s *BaseSimScorer) Score(doc int, freq float32) float32 {
	return 1.0
}
