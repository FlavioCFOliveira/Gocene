// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LuceneIBSimilarity mirrors org.apache.lucene.search.similarities.
// IBSimilarity from Lucene 10.4.0 — the information-based retrieval
// framework of Clinchant & Gaussier (SIGIR 2010).
//
// Composes three plug-in components:
//
//   - distribution  — LL (log-logistic) or SPL (smoothed power-law)
//   - lambda        — LambdaDF or LambdaTTF
//   - normalization — any DFR normalization (NoNormalization, H1, H2, H3, Z)
//
// The legacy [IBSimilarity] struct is preserved untouched.
type LuceneIBSimilarity struct {
	*LuceneSimilarityBase

	distribution  LuceneIBDistribution
	lambda        LuceneIBLambda
	normalization LuceneDFRNormalization
}

// NewLuceneIBSimilarity builds an IBSimilarity from the three components
// with discountOverlaps=true.
func NewLuceneIBSimilarity(distribution LuceneIBDistribution, lambda LuceneIBLambda, normalization LuceneDFRNormalization) *LuceneIBSimilarity {
	return NewLuceneIBSimilarityFull(distribution, lambda, normalization, true)
}

// NewLuceneIBSimilarityFull mirrors the expert constructor with
// discountOverlaps configurable. Unlike DFRSimilarity the Java reference
// does NOT null-check the parameters; we keep the same behaviour but
// caller-side nil propagation will panic on first deref — Gocene users
// should pass [LuceneNoNormalization] when they want no normalization.
func NewLuceneIBSimilarityFull(distribution LuceneIBDistribution, lambda LuceneIBLambda, normalization LuceneDFRNormalization, discountOverlaps bool) *LuceneIBSimilarity {
	if distribution == nil || lambda == nil || normalization == nil {
		panic("LuceneIBSimilarity: null parameters not allowed.")
	}
	ib := &LuceneIBSimilarity{
		distribution:  distribution,
		lambda:        lambda,
		normalization: normalization,
	}
	score := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		return stats.Boost() * distribution.Score(stats,
			normalization.Tfn(stats, freq, docLen),
			float64(lambda.Lambda(stats)))
	}
	subExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		subs := make([]Explanation, 0, 4)
		if stats.Boost() != 1.0 {
			subs = append(subs, NewExplanation(true, float32(stats.Boost()), "boost, query boost"))
		}
		normExp := normalization.Explain(stats, freq, docLen)
		lambdaExp := lambda.Explain(stats)
		subs = append(subs, normExp)
		subs = append(subs, lambdaExp)
		subs = append(subs, distribution.Explain(stats,
			float64(normExp.GetValue()), float64(lambdaExp.GetValue())))
		return subs
	}
	toString := func() string {
		return "IB " + distribution.String() + "-" + lambda.String() + normalization.String()
	}
	ib.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, score, subExplain, toString)
	return ib
}

// GetDistribution returns the configured distribution.
func (s *LuceneIBSimilarity) GetDistribution() LuceneIBDistribution { return s.distribution }

// GetLambda returns the configured lambda.
func (s *LuceneIBSimilarity) GetLambda() LuceneIBLambda { return s.lambda }

// GetNormalization returns the configured normalization.
func (s *LuceneIBSimilarity) GetNormalization() LuceneDFRNormalization { return s.normalization }

// String mirrors IBSimilarity.toString.
func (s *LuceneIBSimilarity) String() string {
	return "IB " + s.distribution.String() + "-" + s.lambda.String() + s.normalization.String()
}

// Compile-time guarantee.
var _ LuceneSimilarity = (*LuceneIBSimilarity)(nil)
