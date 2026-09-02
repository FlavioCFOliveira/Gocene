// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Similarity is the index-time subset of Lucene's
// org.apache.lucene.search.similarities.Similarity contract: the part
// IndexWriter needs for per-field norm encoding during analysis.
//
// The full Similarity contract (including the query-time Scorer) lives in
// the search package, which imports index for FieldInvertState — so index
// cannot import search back without a cycle. search.Similarity structurally
// satisfies this narrower interface (same ComputeNormFromInvertState and
// GetDiscountOverlaps signatures), so any concrete similarity (e.g.
// search.BM25Similarity) can be stored here without either package knowing
// about the other's full type.
type Similarity interface {
	// GetDiscountOverlaps reports whether overlap tokens (position
	// increment of zero) are discounted from a document's length when
	// computing norms.
	GetDiscountOverlaps() bool

	// ComputeNormFromInvertState computes the normalization value for a
	// field at index time, mirroring Similarity.computeNorm(FieldInvertState).
	ComputeNormFromInvertState(state *FieldInvertState) int64
}
