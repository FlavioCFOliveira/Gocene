// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// LuceneIndriDirichletSimilarity mirrors IndriDirichletSimilarity — the
// Bayesian smoothing with Dirichlet priors as implemented in the Indri
// search engine.
//
//	score = log((freq + mu * P(t|C)) / (docLen + mu))
//
// Note: unlike LMDirichletSimilarity, this implementation can return
// negative scores. Lucene preserves them; we do the same.
//
// The legacy [IndriSimilarity] struct is preserved untouched.
type LuceneIndriDirichletSimilarity struct {
	*LuceneLMSimilarity
	mu float32
}

// NewLuceneIndriDirichletSimilarity returns IndriDirichletSimilarity with
// the Indri collection model and mu=2000.
func NewLuceneIndriDirichletSimilarity() *LuceneIndriDirichletSimilarity {
	return NewLuceneIndriDirichletSimilarityWithMu(2000)
}

// NewLuceneIndriDirichletSimilarityWithMu returns IndriDirichletSimilarity
// with the Indri collection model and the supplied mu.
func NewLuceneIndriDirichletSimilarityWithMu(mu float32) *LuceneIndriDirichletSimilarity {
	return NewLuceneIndriDirichletSimilarityFull(NewLuceneIndriCollectionModel(), true, mu)
}

// NewLuceneIndriDirichletSimilarityFull mirrors the (collectionModel,
// discountOverlaps, mu) constructor. mu validation is identical to
// LMDirichletSimilarity.
func NewLuceneIndriDirichletSimilarityFull(collectionModel LuceneLMCollectionModel, discountOverlaps bool, mu float32) *LuceneIndriDirichletSimilarity {
	if math.IsNaN(float64(mu)) || math.IsInf(float64(mu), 0) || mu < 0 {
		panic(fmt.Sprintf("illegal mu value: %v, must be a non-negative finite value", mu))
	}
	d := &LuceneIndriDirichletSimilarity{mu: mu}
	score := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		p := stats.CollectionProbability()
		muF := float64(mu)
		den := docLen + muF
		if den == 0 {
			return 0
		}
		return math.Log((freq + muF*p) / den)
	}
	subExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		subs := make([]Explanation, 0, 4)
		if stats.Boost() != 1.0 {
			subs = append(subs, NewExplanation(true, float32(stats.Boost()), "boost"))
		}
		subs = append(subs, NewExplanation(true, mu, "mu"))
		muF := float64(mu)
		p := stats.CollectionProbability()
		weight := NewExplanation(true, float32(math.Log((freq+muF*p)/(docLen+muF))), "term weight")
		subs = append(subs, weight)
		subs = append(subs, NewExplanation(true, float32(math.Log(muF/(docLen+muF))), "document norm"))
		return subs
	}
	d.LuceneLMSimilarity = NewLuceneLMSimilarity(collectionModel,
		fmt.Sprintf("IndriDirichlet(%f)", mu),
		score, subExplain, discountOverlaps)
	return d
}

// GetMu returns the mu parameter.
func (s *LuceneIndriDirichletSimilarity) GetMu() float32 { return s.mu }

// Compile-time guarantee.
var _ LuceneSimilarity = (*LuceneIndriDirichletSimilarity)(nil)
