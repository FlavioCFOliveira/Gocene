// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LuceneSimilarity is the Lucene 10.4.0 canonical Similarity contract.
//
// It mirrors the surface of org.apache.lucene.search.similarities.Similarity:
// implementations expose index-time norm encoding (ComputeNormFromInvertState)
// and produce a query-time LuceneSimScorer through Scorer104. The legacy
// [Similarity] interface remains in place to keep the existing search code
// compiling; new code should prefer this canonical surface.
//
// Note: the Java reference returns long (signed). In Go the value is carried
// as int64 — the bit pattern matches Lucene byte-for-byte because only the
// low 8 bits are populated by SmallFloat.IntToByte4.
type LuceneSimilarity interface {
	// GetDiscountOverlaps reports whether overlap tokens (position
	// increment of zero) are discounted from a document's length when
	// computing norms.
	GetDiscountOverlaps() bool

	// ComputeNormFromInvertState computes the normalization value for a
	// field at index time, mirroring Similarity.computeNorm(FieldInvertState).
	// Implementations should encode the result with SmallFloat.IntToByte4
	// so the on-disk byte stream matches the Java reference.
	ComputeNormFromInvertState(state *index.FieldInvertState) int64

	// Scorer104 builds a LuceneSimScorer for the supplied collection and
	// term statistics, mirroring Similarity.scorer(float, CollectionStatistics,
	// TermStatistics...).
	Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer
}

// LuceneSimScorer is the Lucene 10.4.0 SimScorer contract.
//
// It mirrors Similarity.SimScorer: a single Score104(freq, norm) method
// scores one document, AsBulkSimScorer returns a bulk variant, and
// Explain104 produces an Explanation tree for IndexSearcher.explain.
type LuceneSimScorer interface {
	// Score104 scores a single document given the sloppy term frequency
	// (finite, positive) and the encoded normalization factor (never 0).
	Score104(freq float32, norm int64) float32

	// AsBulkSimScorer returns a BulkSimScorer producing the exact same
	// scores as this scorer, optimized for batch evaluation. The default
	// implementation wraps the scorer with DefaultBulkSimScorer.
	//
	// NOTE: the returned instance is not safe for concurrent use.
	AsBulkSimScorer() BulkSimScorer

	// Explain104 returns an Explanation tree for a single document.
	Explain104(freq Explanation, norm int64) Explanation
}

// BulkSimScorer mirrors Similarity.BulkSimScorer. Implementations populate
// scores[i] = score(freqs[i], norms[i]) for i in [0, size).
//
// NOTE: it is legal to pass the same backing array for freqs and scores.
type BulkSimScorer interface {
	// ScoreBulk computes scores in bulk. The caller guarantees that all
	// slices have length >= size.
	ScoreBulk(size int, freqs []float32, norms []int64, scores []float32)
}

// DefaultBulkSimScorer wraps a LuceneSimScorer in a straight-line loop. It
// is the fallback returned by LuceneSimScorer implementations that do not
// override AsBulkSimScorer.
type DefaultBulkSimScorer struct {
	scorer LuceneSimScorer
}

// NewDefaultBulkSimScorer returns a DefaultBulkSimScorer wrapping the given
// scorer. The scorer must be non-nil.
func NewDefaultBulkSimScorer(scorer LuceneSimScorer) *DefaultBulkSimScorer {
	if scorer == nil {
		panic("DefaultBulkSimScorer: scorer must not be nil")
	}
	return &DefaultBulkSimScorer{scorer: scorer}
}

// ScoreBulk evaluates the wrapped scorer for each [freqs[i], norms[i]] pair.
func (d *DefaultBulkSimScorer) ScoreBulk(size int, freqs []float32, norms []int64, scores []float32) {
	for i := 0; i < size; i++ {
		scores[i] = d.scorer.Score104(freqs[i], norms[i])
	}
}

// Ensure DefaultBulkSimScorer satisfies BulkSimScorer at compile time.
var _ BulkSimScorer = (*DefaultBulkSimScorer)(nil)

// DefaultComputeNormFromInvertState is the canonical implementation of
// Similarity.computeNorm(FieldInvertState) from Lucene 10.4.0. It returns
// the normalization encoded by SmallFloat.IntToByte4 in the low 8 bits of
// the result.
//
// When IndexOptions is DOCS-only the unique-term count is used; otherwise
// the running length (minus overlaps, when discountOverlaps is true) is
// passed through. Errors from IntToByte4 are impossible here — Lucene
// guarantees the input is non-negative — but we coerce a negative count to
// zero to mirror Lucene's promotion of the encoded byte.
func DefaultComputeNormFromInvertState(state *index.FieldInvertState, discountOverlaps bool) int64 {
	if state == nil {
		return 1
	}
	var numTerms int
	switch {
	case state.IndexOptions() == index.IndexOptionsDocs:
		numTerms = state.UniqueTermCount()
	case discountOverlaps:
		numTerms = state.Length() - state.NumOverlap()
	default:
		numTerms = state.Length()
	}
	if numTerms < 0 {
		numTerms = 0
	}
	b, err := util.IntToByte4(numTerms)
	if err != nil {
		// IntToByte4 only returns an error for negative inputs; numTerms
		// has been clamped above.
		return 0
	}
	return int64(b)
}
