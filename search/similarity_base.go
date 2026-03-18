// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SimilarityBase is the base class for Similarity implementations.
// This is the Go port of Lucene's org.apache.lucene.search.similarities.SimilarityBase.
type SimilarityBase struct {
	*BaseSimilarity
	// DiscountOverlaps indicates whether to discount overlaps
	DiscountOverlaps bool
}

// NewSimilarityBase creates a new SimilarityBase.
func NewSimilarityBase() *SimilarityBase {
	return &SimilarityBase{
		BaseSimilarity:   NewBaseSimilarity(),
		DiscountOverlaps: true,
	}
}

// Ensure SimilarityBase implements Similarity
var _ Similarity = (*SimilarityBase)(nil)
