// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LuceneDFRAfterEffect mirrors org.apache.lucene.search.similarities.
// AfterEffect — the first normalization of information gain in DFR.
type LuceneDFRAfterEffect interface {
	// ScoreTimes1pTfn returns the after-effect score multiplied by (1+tfn).
	// The result must not depend on tfn (that is the whole reason the
	// interface returns the product up-front: the caller divides by
	// (1+tfn) when needed).
	ScoreTimes1pTfn(stats *LuceneBasicStats) float64

	// Explain returns the canonical Explanation tree for the effect.
	Explain(stats *LuceneBasicStats, tfn float64) Explanation

	// String returns the after-effect code ("B" or "L").
	String() string
}

// LuceneAfterEffectB mirrors AfterEffectB — ratio of two Bernoulli processes.
type LuceneAfterEffectB struct{}

// NewLuceneAfterEffectB constructs the parameter-free AfterEffectB.
func NewLuceneAfterEffectB() *LuceneAfterEffectB { return &LuceneAfterEffectB{} }

// ScoreTimes1pTfn implements LuceneDFRAfterEffect.
// (F+1)/n where F = totalTermFreq + 1, n = docFreq + 1.
func (LuceneAfterEffectB) ScoreTimes1pTfn(stats *LuceneBasicStats) float64 {
	F := float64(stats.TotalTermFreq() + 1)
	n := float64(stats.DocFreq() + 1)
	if n == 0 {
		return 0
	}
	return (F + 1.0) / n
}

// Explain returns the canonical AfterEffectB tree.
func (e LuceneAfterEffectB) Explain(stats *LuceneBasicStats, tfn float64) Explanation {
	score := float32(e.ScoreTimes1pTfn(stats) / (1 + tfn))
	exp := NewExplanation(true, score,
		"AfterEffectB, computed as (F + 1) / (n * (tfn + 1)) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	exp.AddDetail(NewExplanation(true, float32(stats.TotalTermFreq()),
		"F, total number of occurrences of term across all documents + 1"))
	exp.AddDetail(NewExplanation(true, float32(stats.DocFreq()),
		"n, number of documents containing term + 1"))
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	return exp
}

// String returns "B".
func (LuceneAfterEffectB) String() string { return "B" }

// LuceneAfterEffectL mirrors AfterEffectL — Laplace's law of succession.
type LuceneAfterEffectL struct{}

// NewLuceneAfterEffectL constructs the parameter-free AfterEffectL.
func NewLuceneAfterEffectL() *LuceneAfterEffectL { return &LuceneAfterEffectL{} }

// ScoreTimes1pTfn implements LuceneDFRAfterEffect. Always returns 1.
func (LuceneAfterEffectL) ScoreTimes1pTfn(_ *LuceneBasicStats) float64 { return 1.0 }

// Explain returns the canonical AfterEffectL tree.
func (e LuceneAfterEffectL) Explain(stats *LuceneBasicStats, tfn float64) Explanation {
	score := float32(e.ScoreTimes1pTfn(stats) / (1 + tfn))
	exp := NewExplanation(true, score,
		"AfterEffectL, computed as 1 / (tfn + 1) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	return exp
}

// String returns "L".
func (LuceneAfterEffectL) String() string { return "L" }

// Compile-time guarantees.
var (
	_ LuceneDFRAfterEffect = LuceneAfterEffectB{}
	_ LuceneDFRAfterEffect = LuceneAfterEffectL{}
)
