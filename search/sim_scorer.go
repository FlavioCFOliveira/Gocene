// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SimScorer is not declared here: it is the nested class
// org.apache.lucene.search.similarities.Similarity.SimScorer and therefore
// lives in similarity.go, alongside its enclosing Similarity.

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
