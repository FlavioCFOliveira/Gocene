// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// LuceneAxiomaticComponent is a single Axiomatic scoring factor — tf, ln,
// tfln, idf, or gamma. Each F1/F2/F3 variant supplies its own combination
// of these.
type LuceneAxiomaticComponent func(stats *LuceneBasicStats, freq, docLen float64) float64

// LuceneAxiomaticExplainComponent returns the explanation tree for a
// single Axiomatic scoring factor.
type LuceneAxiomaticExplainComponent func(stats *LuceneBasicStats, freq, docLen float64) Explanation

// LuceneAxiomaticHooks bundles the five scoring components and the four
// explain components required by an Axiomatic variant.
type LuceneAxiomaticHooks struct {
	TF          LuceneAxiomaticComponent
	LN          LuceneAxiomaticComponent
	TFLN        LuceneAxiomaticComponent
	IDF         LuceneAxiomaticComponent
	Gamma       LuceneAxiomaticComponent
	TFExplain   LuceneAxiomaticExplainComponent
	LNExplain   LuceneAxiomaticExplainComponent
	TFLNExplain LuceneAxiomaticExplainComponent
	IDFExplain  LuceneAxiomaticExplainComponent
	Name        string // e.g. "F1EXP", "F3LOG"
}

// LuceneAxiomaticSimilarity mirrors org.apache.lucene.search.similarities.
// Axiomatic from Lucene 10.4.0 — the abstract base for F1/F2/F3 variants.
//
// Hyperparameters s, k, queryLen are stored on the struct; the per-variant
// score components live in LuceneAxiomaticHooks. Composition keeps the
// hot path zero-virtual-dispatch.
type LuceneAxiomaticSimilarity struct {
	*LuceneSimilarityBase

	s        float32
	k        float32
	queryLen int
	hooks    LuceneAxiomaticHooks
}

// NewLuceneAxiomaticSimilarity constructs an Axiomatic similarity with the
// supplied hyperparameters, hooks and discountOverlaps=true. Panics on
// illegal s/k/queryLen, mirroring Java's IllegalArgumentException.
func NewLuceneAxiomaticSimilarity(s float32, queryLen int, k float32, hooks LuceneAxiomaticHooks) *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticSimilarityFull(true, s, queryLen, k, hooks)
}

// NewLuceneAxiomaticSimilarityFull mirrors the (discountOverlaps, s,
// queryLen, k) Java constructor.
func NewLuceneAxiomaticSimilarityFull(discountOverlaps bool, s float32, queryLen int, k float32, hooks LuceneAxiomaticHooks) *LuceneAxiomaticSimilarity {
	if math.IsNaN(float64(s)) || math.IsInf(float64(s), 0) || s < 0 || s > 1 {
		panic(fmt.Sprintf("illegal s value: %v, must be between 0 and 1", s))
	}
	if math.IsNaN(float64(k)) || math.IsInf(float64(k), 0) || k < 0 || k > 1 {
		panic(fmt.Sprintf("illegal k value: %v, must be between 0 and 1", k))
	}
	if queryLen < 0 {
		panic(fmt.Sprintf("illegal query length value: %d, must be larger 0", queryLen))
	}
	a := &LuceneAxiomaticSimilarity{
		s:        s,
		k:        k,
		queryLen: queryLen,
		hooks:    hooks,
	}
	score := func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		v := hooks.TF(stats, freq, docLen) *
			hooks.LN(stats, freq, docLen) *
			hooks.TFLN(stats, freq, docLen) *
			hooks.IDF(stats, freq, docLen)
		v -= hooks.Gamma(stats, freq, docLen)
		v *= stats.Boost()
		if v < 0 {
			// AxiomaticF3 may produce negative scores via gamma; floor at zero.
			return 0
		}
		return v
	}
	subExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		subs := make([]Explanation, 0, 8)
		if stats.Boost() != 1.0 {
			subs = append(subs, NewExplanation(true, float32(stats.Boost()), "boost, query boost"))
		}
		subs = append(subs, NewExplanation(true, k, "k, hyperparam for the primitive weighting function"))
		subs = append(subs, NewExplanation(true, s, "s, hyperparam for the growth function"))
		subs = append(subs, NewExplanation(true, float32(queryLen), "queryLen, query length"))
		subs = append(subs, hooks.TFExplain(stats, freq, docLen))
		subs = append(subs, hooks.LNExplain(stats, freq, docLen))
		subs = append(subs, hooks.TFLNExplain(stats, freq, docLen))
		subs = append(subs, hooks.IDFExplain(stats, freq, docLen))
		subs = append(subs, NewExplanation(true,
			float32(hooks.Gamma(stats, freq, docLen)), "gamma"))
		return subs
	}
	toString := func() string { return hooks.Name }
	a.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, score, subExplain, toString)
	return a
}

// S returns the s hyperparameter.
func (a *LuceneAxiomaticSimilarity) S() float32 { return a.s }

// K returns the k hyperparameter.
func (a *LuceneAxiomaticSimilarity) K() float32 { return a.k }

// QueryLen returns the query length.
func (a *LuceneAxiomaticSimilarity) QueryLen() int { return a.queryLen }

// String returns the variant name, mirroring Axiomatic.toString.
func (a *LuceneAxiomaticSimilarity) String() string { return a.hooks.Name }

// Compile-time guarantee.
var _ LuceneSimilarity = (*LuceneAxiomaticSimilarity)(nil)

// ============================================================================
// Shared scoring kernels used by F1/F2/F3 variants.
// ============================================================================

// axiomaticTFGrowth is the "1 + log(1 + log(freq+1))" tf kernel used by
// F1EXP/F1LOG/F3EXP/F3LOG.
func axiomaticTFGrowth(_ *LuceneBasicStats, freq, _ float64) float64 {
	freq += 1 // otherwise gives negative scores for freqs < 1
	return 1 + math.Log(1+math.Log(freq))
}

// axiomaticTFConstant is the constant tf=1 kernel used by F2EXP/F2LOG.
func axiomaticTFConstant(_ *LuceneBasicStats, _, _ float64) float64 { return 1.0 }

// axiomaticLNConstant is the constant ln=1 kernel used by F2EXP/F2LOG/F3EXP/F3LOG.
func axiomaticLNConstant(_ *LuceneBasicStats, _, _ float64) float64 { return 1.0 }

// axiomaticTFLNConstant is the constant tfln=1 kernel used by F1EXP/F1LOG/F3EXP/F3LOG.
func axiomaticTFLNConstant(_ *LuceneBasicStats, _, _ float64) float64 { return 1.0 }

// axiomaticGammaZero is the gamma=0 kernel used by F1EXP/F1LOG/F2EXP/F2LOG.
func axiomaticGammaZero(_ *LuceneBasicStats, _, _ float64) float64 { return 0.0 }

// axiomaticLNWithGrowth implements ln(stats, freq, docLen) =
// (avgfl + s) / (avgfl + dl * s) used by F1EXP/F1LOG.
func axiomaticLNWithGrowth(s float32) LuceneAxiomaticComponent {
	return func(stats *LuceneBasicStats, _, docLen float64) float64 {
		sf := float64(s)
		denom := stats.AvgFieldLength() + docLen*sf
		if denom == 0 {
			return 0
		}
		return (stats.AvgFieldLength() + sf) / denom
	}
}

// axiomaticTFLNWithGrowth implements tfln(freq, dl) =
// freq / (freq + s + s * dl / avgfl) used by F2EXP/F2LOG.
func axiomaticTFLNWithGrowth(s float32) LuceneAxiomaticComponent {
	return func(stats *LuceneBasicStats, freq, docLen float64) float64 {
		sf := float64(s)
		avgfl := stats.AvgFieldLength()
		if avgfl == 0 {
			avgfl = 1
		}
		denom := freq + sf + sf*docLen/avgfl
		if denom == 0 {
			return 0
		}
		return freq / denom
	}
}

// axiomaticIDFPow implements idf = pow((N+1)/n, k) used by F*EXP.
func axiomaticIDFPow(k float32) LuceneAxiomaticComponent {
	return func(stats *LuceneBasicStats, _, _ float64) float64 {
		df := float64(stats.DocFreq())
		if df == 0 {
			df = 1
		}
		return math.Pow((float64(stats.NumberOfDocuments())+1)/df, float64(k))
	}
}

// axiomaticIDFLog implements idf = log((N+1)/n) used by F*LOG.
func axiomaticIDFLog(_ *LuceneBasicStats, _, _ float64) float64 {
	// Placeholder to satisfy the LuceneAxiomaticComponent shape; the
	// real implementation is bound below via the factory functions so
	// the stats pointer is captured each call.
	panic("axiomaticIDFLog called without bind; use idfLogClosure")
}

// idfLogClosure returns the canonical log-IDF kernel.
func idfLogClosure() LuceneAxiomaticComponent {
	return func(stats *LuceneBasicStats, _, _ float64) float64 {
		df := float64(stats.DocFreq())
		if df == 0 {
			df = 1
		}
		return math.Log((float64(stats.NumberOfDocuments()) + 1) / df)
	}
}

// axiomaticGammaF3 implements gamma = (dl - queryLen) * s * queryLen /
// avgdl used by F3EXP/F3LOG.
func axiomaticGammaF3(s float32, queryLen int) LuceneAxiomaticComponent {
	return func(stats *LuceneBasicStats, _, docLen float64) float64 {
		avgfl := stats.AvgFieldLength()
		if avgfl == 0 {
			avgfl = 1
		}
		return (docLen - float64(queryLen)) * float64(s) * float64(queryLen) / avgfl
	}
}

// ============================================================================
// Shared explain factories.
// ============================================================================

func axiomaticTFGrowthExplain(stats *LuceneBasicStats, freq, docLen float64) Explanation {
	exp := NewExplanation(true, float32(axiomaticTFGrowth(stats, freq, docLen)),
		"tf, term frequency computed as 1 + log(1 + log(freq)) from:")
	exp.AddDetail(NewExplanation(true, float32(freq), "freq, number of occurrences of term in the document"))
	return exp
}

func axiomaticTFConstantExplain(stats *LuceneBasicStats, freq, docLen float64) Explanation {
	return NewExplanation(true, float32(axiomaticTFConstant(stats, freq, docLen)),
		"tf, term frequency, equals to 1")
}

func axiomaticLNConstantExplain(stats *LuceneBasicStats, freq, docLen float64) Explanation {
	return NewExplanation(true, float32(axiomaticLNConstant(stats, freq, docLen)),
		"ln, document length, equals to 1")
}

func axiomaticTFLNConstantExplain(stats *LuceneBasicStats, freq, docLen float64) Explanation {
	return NewExplanation(true, float32(axiomaticTFLNConstant(stats, freq, docLen)),
		"tfln, mixed term frequency and document length, equals to 1")
}

func axiomaticLNWithGrowthExplain(s float32) LuceneAxiomaticExplainComponent {
	kernel := axiomaticLNWithGrowth(s)
	return func(stats *LuceneBasicStats, freq, docLen float64) Explanation {
		exp := NewExplanation(true, float32(kernel(stats, freq, docLen)),
			"ln, document length computed as (avgdl + s) / (avgdl + dl * s) from:")
		exp.AddDetail(NewExplanation(true, float32(stats.AvgFieldLength()),
			"avgdl, average length of field across all documents"))
		exp.AddDetail(NewExplanation(true, float32(docLen), "dl, length of field"))
		return exp
	}
}

func axiomaticTFLNWithGrowthExplain(s float32) LuceneAxiomaticExplainComponent {
	kernel := axiomaticTFLNWithGrowth(s)
	return func(stats *LuceneBasicStats, freq, docLen float64) Explanation {
		exp := NewExplanation(true, float32(kernel(stats, freq, docLen)),
			"tfln, mixed term frequency and document length, computed as freq / (freq + s + s * dl / avgdl) from:")
		exp.AddDetail(NewExplanation(true, float32(freq), "freq, number of occurrences of term in the document"))
		exp.AddDetail(NewExplanation(true, float32(docLen), "dl, length of field"))
		exp.AddDetail(NewExplanation(true, float32(stats.AvgFieldLength()),
			"avgdl, average length of field across all documents"))
		return exp
	}
}

func axiomaticIDFPowExplain(k float32) LuceneAxiomaticExplainComponent {
	kernel := axiomaticIDFPow(k)
	return func(stats *LuceneBasicStats, freq, docLen float64) Explanation {
		exp := NewExplanation(true, float32(kernel(stats, freq, docLen)),
			"idf, inverted document frequency computed as Math.pow((N + 1) / n, k) from:")
		exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()),
			"N, total number of documents with field"))
		exp.AddDetail(NewExplanation(true, float32(stats.DocFreq()),
			"n, number of documents containing term"))
		return exp
	}
}

func axiomaticIDFLogExplain() LuceneAxiomaticExplainComponent {
	kernel := idfLogClosure()
	return func(stats *LuceneBasicStats, freq, docLen float64) Explanation {
		exp := NewExplanation(true, float32(kernel(stats, freq, docLen)),
			"idf, inverted document frequency computed as log((N + 1) / n) from:")
		exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()),
			"N, total number of documents with field"))
		exp.AddDetail(NewExplanation(true, float32(stats.DocFreq()),
			"n, number of documents containing term"))
		return exp
	}
}

// ============================================================================
// F1EXP / F1LOG / F2EXP / F2LOG / F3EXP / F3LOG factories.
// ============================================================================

// NewLuceneAxiomaticF1EXP returns AxiomaticF1EXP(s, k) with queryLen=1.
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

// NewLuceneAxiomaticF1EXPDefault returns the parameter-free F1EXP
// (s=0.25, k=0.35, queryLen=1).
func NewLuceneAxiomaticF1EXPDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF1EXP(0.25, 0.35)
}

// NewLuceneAxiomaticF1LOG returns AxiomaticF1LOG(s) with queryLen=1, k=0.35.
func NewLuceneAxiomaticF1LOG(s float32) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFGrowth,
		LN:          axiomaticLNWithGrowth(s),
		TFLN:        axiomaticTFLNConstant,
		IDF:         idfLogClosure(),
		Gamma:       axiomaticGammaZero,
		TFExplain:   axiomaticTFGrowthExplain,
		LNExplain:   axiomaticLNWithGrowthExplain(s),
		TFLNExplain: axiomaticTFLNConstantExplain,
		IDFExplain:  axiomaticIDFLogExplain(),
		Name:        "F1LOG",
	}
	return NewLuceneAxiomaticSimilarity(s, 1, 0.35, hooks)
}

// NewLuceneAxiomaticF1LOGDefault returns parameter-free F1LOG (s=0.25).
func NewLuceneAxiomaticF1LOGDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF1LOG(0.25)
}

// NewLuceneAxiomaticF2EXP returns AxiomaticF2EXP(s, k) with queryLen=1.
func NewLuceneAxiomaticF2EXP(s, k float32) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFConstant,
		LN:          axiomaticLNConstant,
		TFLN:        axiomaticTFLNWithGrowth(s),
		IDF:         axiomaticIDFPow(k),
		Gamma:       axiomaticGammaZero,
		TFExplain:   axiomaticTFConstantExplain,
		LNExplain:   axiomaticLNConstantExplain,
		TFLNExplain: axiomaticTFLNWithGrowthExplain(s),
		IDFExplain:  axiomaticIDFPowExplain(k),
		Name:        "F2EXP",
	}
	return NewLuceneAxiomaticSimilarity(s, 1, k, hooks)
}

// NewLuceneAxiomaticF2EXPDefault returns parameter-free F2EXP.
func NewLuceneAxiomaticF2EXPDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF2EXP(0.25, 0.35)
}

// NewLuceneAxiomaticF2LOG returns AxiomaticF2LOG(s) with queryLen=1, k=0.35.
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

// NewLuceneAxiomaticF2LOGDefault returns parameter-free F2LOG.
func NewLuceneAxiomaticF2LOGDefault() *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF2LOG(0.25)
}

// NewLuceneAxiomaticF3EXP returns AxiomaticF3EXP(s, queryLen, k).
func NewLuceneAxiomaticF3EXP(s float32, queryLen int, k float32) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFGrowth,
		LN:          axiomaticLNConstant,
		TFLN:        axiomaticTFLNConstant,
		IDF:         axiomaticIDFPow(k),
		Gamma:       axiomaticGammaF3(s, queryLen),
		TFExplain:   axiomaticTFGrowthExplain,
		LNExplain:   axiomaticLNConstantExplain,
		TFLNExplain: axiomaticTFLNConstantExplain,
		IDFExplain:  axiomaticIDFPowExplain(k),
		Name:        "F3EXP",
	}
	return NewLuceneAxiomaticSimilarity(s, queryLen, k, hooks)
}

// NewLuceneAxiomaticF3EXPDefault returns AxiomaticF3EXP(s, queryLen) with k=0.35.
func NewLuceneAxiomaticF3EXPDefault(s float32, queryLen int) *LuceneAxiomaticSimilarity {
	return NewLuceneAxiomaticF3EXP(s, queryLen, 0.35)
}

// NewLuceneAxiomaticF3LOG returns AxiomaticF3LOG(s, queryLen).
func NewLuceneAxiomaticF3LOG(s float32, queryLen int) *LuceneAxiomaticSimilarity {
	hooks := LuceneAxiomaticHooks{
		TF:          axiomaticTFGrowth,
		LN:          axiomaticLNConstant,
		TFLN:        axiomaticTFLNConstant,
		IDF:         idfLogClosure(),
		Gamma:       axiomaticGammaF3(s, queryLen),
		TFExplain:   axiomaticTFGrowthExplain,
		LNExplain:   axiomaticLNConstantExplain,
		TFLNExplain: axiomaticTFLNConstantExplain,
		IDFExplain:  axiomaticIDFLogExplain(),
		Name:        "F3LOG",
	}
	return NewLuceneAxiomaticSimilarity(s, queryLen, 0.35, hooks)
}
