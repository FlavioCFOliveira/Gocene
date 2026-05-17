// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LuceneBooleanSimilarity mirrors org.apache.lucene.search.similarities.
// BooleanSimilarity from Lucene 10.4.0. Each match scores `boost` — no
// term or collection statistics are consulted.
//
// Norms, when present, follow the SimilarityBase/BM25 encoding so the
// similarity can be swapped after indexing.
type LuceneBooleanSimilarity struct{}

// NewLuceneBooleanSimilarity returns a BooleanSimilarity. The Java class
// has a single no-arg constructor; we expose the same shape.
func NewLuceneBooleanSimilarity() *LuceneBooleanSimilarity {
	return &LuceneBooleanSimilarity{}
}

// GetDiscountOverlaps satisfies LuceneSimilarity. Hard-coded to true to
// match SimilarityBase/BM25 — see the Java javadoc.
func (s *LuceneBooleanSimilarity) GetDiscountOverlaps() bool { return true }

// ComputeNormFromInvertState satisfies LuceneSimilarity.
func (s *LuceneBooleanSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return DefaultComputeNormFromInvertState(state, true)
}

// Scorer104 returns a SimScorer whose Score104 is `boost` for any freq/norm.
func (s *LuceneBooleanSimilarity) Scorer104(boost float32, _ *CollectionStatistics, _ ...*TermStatistics) LuceneSimScorer {
	return &luceneBooleanScorer{boost: boost}
}

// luceneBooleanScorer is the BooleanWeight peer from Java.
type luceneBooleanScorer struct {
	boost float32
}

// Score104 returns the configured boost regardless of freq/norm.
func (s *luceneBooleanScorer) Score104(_ float32, _ int64) float32 { return s.boost }

// AsBulkSimScorer returns the default bulk wrapper.
func (s *luceneBooleanScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 mirrors BooleanWeight.explain.
func (s *luceneBooleanScorer) Explain104(_ Explanation, _ int64) Explanation {
	queryBoost := NewExplanation(true, s.boost, "boost, query boost")
	exp := NewExplanation(true, s.boost, "score(LuceneBooleanSimilarity), computed from:")
	exp.AddDetail(queryBoost)
	return exp
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*LuceneBooleanSimilarity)(nil)
	_ LuceneSimScorer  = (*luceneBooleanScorer)(nil)
)
