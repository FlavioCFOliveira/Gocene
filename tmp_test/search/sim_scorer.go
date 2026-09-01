// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SimScorer scores documents based on similarity.
type SimScorer interface {
	// Score scores a document given the term frequency and the encoded norm
	// value read from the field's NumericDocValues norms. The norm value is
	// the low 8 bits produced by Similarity.computeNorm / SmallFloat.IntToByte4
	// and is 1 (the encoded average-length sentinel) when norms are absent.
	Score(doc int, freq float32, norm int64) float32
}

// BaseSimScorer provides common functionality.
type BaseSimScorer struct{}

// NewBaseSimScorer creates a new BaseSimScorer.
func NewBaseSimScorer() *BaseSimScorer {
	return &BaseSimScorer{}
}

// Score returns a default score. The norm argument is accepted for API parity
// with Lucene's SimScorer.score(float, long) but ignored by this base scorer.
func (s *BaseSimScorer) Score(doc int, freq float32, norm int64) float32 {
	return 1.0
}
