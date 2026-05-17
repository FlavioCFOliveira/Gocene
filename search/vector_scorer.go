// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// VectorScorer scores documents against a query vector for exact vector
// search.
//
// Mirrors org.apache.lucene.search.VectorScorer.
type VectorScorer interface {
	// Score returns the similarity score for the current document.
	Score() (float32, error)
	// Iterator returns a DocIdSetIterator over the scored documents.
	Iterator() DocIdSetIterator
	// Bulk returns an optional bulk-scoring helper, or nil if not supported.
	Bulk() VectorScorerBulk
}

// VectorScorerBulk lets callers score a batch of documents efficiently.
type VectorScorerBulk interface {
	// Score writes per-doc similarity scores for at most upTo documents into
	// buf, returning the number of documents actually scored.
	Score(buf []float32, upTo int) (int, error)
}

// VectorScorerDefaultBatchSize is the canonical bulk batch size used by
// Lucene's reference implementation.
const VectorScorerDefaultBatchSize = 64
