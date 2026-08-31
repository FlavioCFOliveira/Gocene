// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// NewLuceneAxiomaticF1EXP returns AxiomaticF1EXP(s, k) with queryLen=1.
// Mirrors org.apache.lucene.search.similarities.AxiomaticF1EXP(float s, float k).
func NewLuceneAxiomaticF1EXP(s, k float32) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFGrowth,
		LN:          axiomaticLNWithGrowth(s),
		TFLN:        axiomaticTFLNConstant,
		IDF:         axiomaticIDFPow(k),
		Gamma:       axiomaticGammaZero,
		TFExplain:   axiomaticTFGrowthExplain,
		LNExplain:   axiomaticLNWithGrowthExplain(s),
		TFLNExplain: axiomaticTFLNConstantExplain,
		IDFExplain:  axiomaticIDFPowExplain(k),
		Name:        "F1EXP",
	}
	return NewLuceneAxiomaticSimilarity(s, 1, k, hooks)
}

// NewLuceneAxiomaticF1EXPWithS returns AxiomaticF1EXP(s) with queryLen=1 and k=0.35.
// Mirrors org.apache.lucene.search.similarities.AxiomaticF1EXP(float s).
func NewLuceneAxiomaticF1EXPWithS(s float32) *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF1EXP(s, 0.35)
}

// NewLuceneAxiomaticF1EXPDefault returns the parameter-free F1EXP
// (s=0.25, k=0.35, queryLen=1).
// Mirrors org.apache.lucene.search.similarities.AxiomaticF1EXP().
func NewLuceneAxiomaticF1EXPDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF1EXP(0.25, 0.35)
}
