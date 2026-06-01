// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// RawTFSimilarity is a Similarity that returns the raw sloppy term
// frequency multiplied by the query boost. It mirrors
// org.apache.lucene.search.similarities.RawTFSimilarity from Lucene 10.4.0.
//
// Norms and collection statistics are ignored. The class is primarily used
// in tests and as a baseline; production scoring should prefer
// [BM25Similarity] or [ClassicSimilarity].
type RawTFSimilarity struct {
	*BaseSimilarity
	discountOverlaps bool
}

// NewRawTFSimilarity returns a RawTFSimilarity with discountOverlaps = true
// (matching the no-arg Java constructor).
func NewRawTFSimilarity() *RawTFSimilarity {
	return &RawTFSimilarity{BaseSimilarity: NewBaseSimilarity(), discountOverlaps: true}
}

// NewRawTFSimilarityWithDiscount mirrors the expert Java constructor.
func NewRawTFSimilarityWithDiscount(discountOverlaps bool) *RawTFSimilarity {
	return &RawTFSimilarity{BaseSimilarity: NewBaseSimilarity(), discountOverlaps: discountOverlaps}
}

// Scorer satisfies the legacy [Similarity] interface so RawTFSimilarity can be
// installed via IndexSearcher.SetSimilarity and flow through the legacy
// TermWeight scoring path. The returned SimScorer scores a document as its raw
// term frequency, mirroring the Lucene RawTFSimilarity contract that
// score == freq. This complements the Lucene-faithful [RawTFSimilarity.Scorer104]
// surface used by the block-max scoring path.
func (s *RawTFSimilarity) Scorer(_ *CollectionStatistics, _ *TermStatistics) SimScorer {
	return rawTFLegacySimScorer{}
}

// rawTFLegacySimScorer is the legacy SimScorer whose Score returns the raw
// frequency, ignoring doc and norms.
type rawTFLegacySimScorer struct{}

// Score returns the raw term frequency.
func (rawTFLegacySimScorer) Score(_ int, freq float32) float32 { return freq }

// GetDiscountOverlaps satisfies LuceneSimilarity.
func (s *RawTFSimilarity) GetDiscountOverlaps() bool { return s.discountOverlaps }

// ComputeNormFromInvertState satisfies LuceneSimilarity.
func (s *RawTFSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return DefaultComputeNormFromInvertState(state, s.discountOverlaps)
}

// Scorer104 returns a SimScorer whose Score104 is `boost * freq`,
// mirroring the anonymous Java implementation byte-for-byte.
func (s *RawTFSimilarity) Scorer104(boost float32, _ *CollectionStatistics, _ ...*TermStatistics) LuceneSimScorer {
	return &rawTFSimScorer{boost: boost}
}

// rawTFSimScorer is the anonymous SimScorer subclass returned by
// RawTFSimilarity.scorer.
type rawTFSimScorer struct {
	boost float32
}

// Score104 returns boost * freq, ignoring the norm.
func (s *rawTFSimScorer) Score104(freq float32, _ int64) float32 {
	return s.boost * freq
}

// AsBulkSimScorer returns the default bulk wrapper.
func (s *rawTFSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 produces the standard SimScorer.explain tree.
func (s *rawTFSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	exp := NewExplanation(true, s.Score104(float32(freq.GetValue()), norm),
		"score(freq="+formatFloatGeneric(freq.GetValue())+"), with freq of:")
	exp.AddDetail(freq)
	return exp
}

// formatFloatGeneric mirrors Java's default Float.toString() output for
// Explanation strings. Java's Float.toString uses the shortest decimal
// representation that round-trips; Go's 'g' verb is the closest
// equivalent without writing a full shortest-decimal implementation.
// Tests that compare explanation text byte-for-byte should normalise
// numerically on both sides.
func formatFloatGeneric(f float32) string {
	return strconv.FormatFloat(float64(f), 'g', -1, 32)
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*RawTFSimilarity)(nil)
	_ LuceneSimScorer  = (*rawTFSimScorer)(nil)
)
