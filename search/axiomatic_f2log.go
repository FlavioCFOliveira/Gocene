// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// NewLuceneAxiomaticF2LOG returns AxiomaticF2LOG(s) with queryLen=1, k=0.35.
// Mirrors org.apache.lucene.search.similarities.AxiomaticF2LOG(float s).
func NewLuceneAxiomaticF2LOG(s float32) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFConstant,
		LN:          axiomaticLNConstant,
		TFLN:        axiomaticTFLNWithGrowth(s),
		IDF:         idfLogClosure(),
		Gamma:       axiomaticGammaZero,
		TFExplain:   axiomaticTFConstantExplain,
		LNExplain:   axiomaticLNConstantExplain,
		TFLNExplain: axiomaticTFLNWithGrowthExplain(s),
		IDFExplain:  axiomaticIDFLogExplain(),
		Name:        "F2LOG",
	}
	return NewLuceneAxiomaticSimilarity(s, 1, 0.35, hooks)
}

// NewLuceneAxiomaticF2LOGDefault returns parameter-free F2LOG (s=0.25).
// Mirrors org.apache.lucene.search.similarities.AxiomaticF2LOG().
func NewLuceneAxiomaticF2LOGDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF2LOG(0.25)
}
