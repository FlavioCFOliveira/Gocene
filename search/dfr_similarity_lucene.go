// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LuceneDFRSimilarity mirrors org.apache.lucene.search.similarities.
// DFRSimilarity from Lucene 10.4.0 — divergence from randomness framework.
//
// The class composes three plug-in components:
//
//   - basicModel       — measures the informative content (BasicModelG, I(n), I(ne), I(F))
//   - afterEffect      — first normalization of information gain (AfterEffectB or L)
//   - normalization    — second (length) normalization (H1, H2, H3, Z, NoNormalization)
//
// The legacy [DFRSimilarity] struct is preserved untouched; this canonical
// type lives alongside it.
type LuceneDFRSimilarity struct {
	*LuceneSimilarityBase

	basicModel    LuceneDFRBasicModel
	afterEffect   LuceneDFRAfterEffect
	normalization LuceneDFRNormalization
}

// NewLuceneDFRSimilarity builds a DFRSimilarity from the three components
// with discountOverlaps=true. All three components are required (panic on
// nil — mirrors Java's NullPointerException).
func NewLuceneDFRSimilarity(basicModel LuceneDFRBasicModel, afterEffect LuceneDFRAfterEffect, normalization LuceneDFRNormalization) *LuceneDFRSimilarity {
	return NewLuceneDFRSimilarityFull(basicModel, afterEffect, normalization, true)
}

// NewLuceneDFRSimilarityFull mirrors the expert constructor with
// discountOverlaps configurable.
func NewLuceneDFRSimilarityFull(basicModel LuceneDFRBasicModel, afterEffect LuceneDFRAfterEffect, normalization LuceneDFRNormalization, discountOverlaps bool) *LuceneDFRSimilarity {
	if basicModel == nil || afterEffect == nil || normalization == nil {
		panic("LuceneDFRSimilarity: null parameters not allowed.")
	}
	d := &LuceneDFRSimilarity{
		basicModel:    basicModel,
		afterEffect:   afterEffect,
		normalization: normalization,
	}
	score := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		tfn := normalization.Tfn(stats, freq, docLen)
		ae := afterEffect.ScoreTimes1pTfn(stats)
		return stats.Boost() * basicModel.Score(stats, tfn, ae)
	}
	subExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		subs := make([]Explanation, 0, 4)
		if stats.Boost() != 1.0 {
			subs = append(subs, NewExplanation(true, float32(stats.Boost()), "boost, query boost"))
		}
		tfn := normalization.Tfn(stats, freq, docLen)
		ae := afterEffect.ScoreTimes1pTfn(stats)
		subs = append(subs, normalization.Explain(stats, freq, docLen))
		subs = append(subs, basicModel.Explain(stats, tfn, ae))
		subs = append(subs, afterEffect.Explain(stats, tfn))
		return subs
	}
	toString := func() string {
		return "DFR " + basicModel.String() + afterEffect.String() + normalization.String()
	}
	d.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, score, subExplain, toString)
	return d
}

// GetBasicModel returns the configured basic model.
func (d *LuceneDFRSimilarity) GetBasicModel() LuceneDFRBasicModel { return d.basicModel }

// GetAfterEffect returns the configured after effect.
func (d *LuceneDFRSimilarity) GetAfterEffect() LuceneDFRAfterEffect { return d.afterEffect }

// GetNormalization returns the configured length normalization.
func (d *LuceneDFRSimilarity) GetNormalization() LuceneDFRNormalization { return d.normalization }

// String mirrors DFRSimilarity.toString.
func (d *LuceneDFRSimilarity) String() string {
	return "DFR " + d.basicModel.String() + d.afterEffect.String() + d.normalization.String()
}

// Explicit interface assertion to defend against accidental embedding
// regressions. The composition above delegates GetDiscountOverlaps,
// ComputeNormFromInvertState, Scorer104 through LuceneSimilarityBase.
var _ LuceneSimilarity = (*LuceneDFRSimilarity)(nil)

// Ensure the base method set is reachable through index.FieldInvertState —
// guards against accidental import cycles.
var _ = (*index.FieldInvertState)(nil)
