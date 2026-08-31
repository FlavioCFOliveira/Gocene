// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// IBDistribution represents the probabilistic distribution used to model term occurrence.
type IBDistribution interface {
	// Score computes the score for the given term frequency normalization and lambda.
	Score(stats *LuceneBasicStats, tfn, lambda float64) float64
	// Explain provides the explanation for the computed score.
	Explain(stats *LuceneBasicStats, tfn, lambda float64) Explanation
	// String returns the name of the distribution.
	String() string
}

// IBLambda represents the lambda parameter of the probability distribution.
type IBLambda interface {
	// Lambda computes the lambda value for the given statistics.
	Lambda(stats *LuceneBasicStats) float64
	// Explain provides the explanation for the computed lambda.
	Explain(stats *LuceneBasicStats) Explanation
	// String returns the name of the lambda parameter implementation.
	String() string
}

// IBNormalization represents term frequency normalization for the IB model.
type IBNormalization interface {
	// Tfn computes the normalized term frequency.
	Tfn(stats *LuceneBasicStats, freq float64, docLen float64) float64
	// Explain provides the explanation for the normalized term frequency.
	Explain(stats *LuceneBasicStats, freq float64, docLen float64) Explanation
	// String returns the name of the normalization implementation.
	String() string
}

// IBSimilarity provides a framework for the family of information-based models.
// It mirrors org.apache.lucene.search.similarities.IBSimilarity from Lucene 10.4.0.
type IBSimilarity struct {
	*LuceneSimilarityBase
	distribution  IBDistribution
	lambda        IBLambda
	normalization IBNormalization
}

// NewIBSimilarity creates an IBSimilarity from the three components and using default discountOverlaps value.
func NewIBSimilarity(distribution IBDistribution, lambda IBLambda, normalization IBNormalization) *IBSimilarity {
	return NewIBSimilarityWithDiscount(true, distribution, lambda, normalization)
}

// NewIBSimilarityWithDiscount creates an IBSimilarity from the three components and with the specified discountOverlaps value.
func NewIBSimilarityWithDiscount(discountOverlaps bool, distribution IBDistribution, lambda IBLambda, normalization IBNormalization) *IBSimilarity {
	ib := &IBSimilarity{
		distribution:  distribution,
		lambda:        lambda,
		normalization: normalization,
	}

	// score mirrors the protected score(BasicStats, double, double) method in Java.
	scoreFunc := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		return stats.Boost() * ib.distribution.Score(stats, ib.normalization.Tfn(stats, freq, docLen), ib.lambda.Lambda(stats))
	}

	// subExplain mirrors the protected explain(List<Explanation>, BasicStats, double, double) method in Java.
	subExplainFunc := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		subs := []Explanation{}
		if stats.Boost() != 1.0 {
			subs = append(subs, NewExplanation(true, float32(stats.Boost()), "boost, query boost"))
		}
		normExpl := ib.normalization.Explain(stats, freq, docLen)
		lambdaExpl := ib.lambda.Explain(stats)
		subs = append(subs, normExpl, lambdaExpl)
		subs = append(subs, ib.distribution.Explain(stats, normExpl.GetValue(), lambdaExpl.GetValue()))
		return subs
	}

	// toString mirrors the toString() method in Java.
	toStringFunc := func() string {
		return fmt.Sprintf("IB %s-%s%s", ib.distribution.String(), ib.lambda.String(), ib.normalization.String())
	}

	ib.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, scoreFunc, subExplainFunc, toStringFunc)
	return ib
}

// GetDistribution returns the distribution used by this similarity.
func (s *IBSimilarity) GetDistribution() IBDistribution { return s.distribution }

// GetLambda returns the lambda parameter used by this similarity.
func (s *IBSimilarity) GetLambda() IBLambda { return s.lambda }

// GetNormalization returns the term frequency normalization used by this similarity.
func (s *IBSimilarity) GetNormalization() IBNormalization { return s.normalization }
