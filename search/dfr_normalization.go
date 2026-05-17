// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// LuceneDFRNormalization mirrors org.apache.lucene.search.similarities.
// Normalization — the second (length) normalization in DFR.
type LuceneDFRNormalization interface {
	// Tfn returns the normalized term frequency.
	Tfn(stats *LuceneBasicStats, tf, length float64) float64

	// Explain returns the canonical Explanation tree.
	Explain(stats *LuceneBasicStats, tf, length float64) Explanation

	// String returns the normalization code ("1", "2", "3(mu)", "Z(z)", "").
	String() string
}

// LuceneNoNormalization mirrors Normalization.NoNormalization — passes
// the term frequency through unchanged.
type LuceneNoNormalization struct{}

// NewLuceneNoNormalization returns a parameter-free no-op normalization.
func NewLuceneNoNormalization() *LuceneNoNormalization { return &LuceneNoNormalization{} }

// Tfn returns tf unchanged.
func (LuceneNoNormalization) Tfn(_ *LuceneBasicStats, tf, _ float64) float64 { return tf }

// Explain returns a constant "no normalization" Explanation.
func (LuceneNoNormalization) Explain(_ *LuceneBasicStats, _, _ float64) Explanation {
	return NewExplanation(true, 1, "no normalization")
}

// String returns the empty code "".
func (LuceneNoNormalization) String() string { return "" }

// LuceneNormalizationH1 mirrors NormalizationH1 — uniform distribution.
// tfn = tf * c * (avgfl / fl).
type LuceneNormalizationH1 struct {
	c float32
}

// NewLuceneNormalizationH1 returns NormalizationH1 with c=1.
func NewLuceneNormalizationH1() *LuceneNormalizationH1 {
	return NewLuceneNormalizationH1WithC(1.0)
}

// NewLuceneNormalizationH1WithC returns NormalizationH1 with the supplied c.
// Panics on illegal c, mirroring Java's IllegalArgumentException.
func NewLuceneNormalizationH1WithC(c float32) *LuceneNormalizationH1 {
	if math.IsNaN(float64(c)) || math.IsInf(float64(c), 0) || c < 0 {
		panic(fmt.Sprintf("illegal c value: %v, must be a non-negative finite value", c))
	}
	return &LuceneNormalizationH1{c: c}
}

// GetC returns the c parameter.
func (n *LuceneNormalizationH1) GetC() float32 { return n.c }

// Tfn implements LuceneDFRNormalization.
func (n *LuceneNormalizationH1) Tfn(stats *LuceneBasicStats, tf, length float64) float64 {
	if length == 0 {
		return tf
	}
	return tf * float64(n.c) * (stats.AvgFieldLength() / length)
}

// Explain returns the canonical NormalizationH1 tree.
func (n *LuceneNormalizationH1) Explain(stats *LuceneBasicStats, tf, length float64) Explanation {
	exp := NewExplanation(true, float32(n.Tfn(stats, tf, length)),
		"NormalizationH1, computed as tf * c * (avgfl / fl) from:")
	exp.AddDetail(NewExplanation(true, float32(tf), "tf, number of occurrences of term in the document"))
	exp.AddDetail(NewExplanation(true, n.c, "c, hyper-parameter"))
	exp.AddDetail(NewExplanation(true, float32(stats.AvgFieldLength()),
		"avgfl, average length of field across all documents"))
	exp.AddDetail(NewExplanation(true, float32(length), "fl, field length of the document"))
	return exp
}

// String returns "1".
func (n *LuceneNormalizationH1) String() string { return "1" }

// LuceneNormalizationH2 mirrors NormalizationH2 — tf inversely related to length.
// tfn = tf * log2(1 + c * avgfl / fl).
type LuceneNormalizationH2 struct {
	c float32
}

// NewLuceneNormalizationH2 returns NormalizationH2 with c=1.
func NewLuceneNormalizationH2() *LuceneNormalizationH2 {
	return NewLuceneNormalizationH2WithC(1.0)
}

// NewLuceneNormalizationH2WithC returns NormalizationH2 with the supplied c.
func NewLuceneNormalizationH2WithC(c float32) *LuceneNormalizationH2 {
	if math.IsNaN(float64(c)) || math.IsInf(float64(c), 0) || c < 0 {
		panic(fmt.Sprintf("illegal c value: %v, must be a non-negative finite value", c))
	}
	return &LuceneNormalizationH2{c: c}
}

// GetC returns the c parameter.
func (n *LuceneNormalizationH2) GetC() float32 { return n.c }

// Tfn implements LuceneDFRNormalization.
func (n *LuceneNormalizationH2) Tfn(stats *LuceneBasicStats, tf, length float64) float64 {
	if length == 0 {
		return tf
	}
	return tf * LuceneSimLog2(1+float64(n.c)*stats.AvgFieldLength()/length)
}

// Explain returns the canonical NormalizationH2 tree.
func (n *LuceneNormalizationH2) Explain(stats *LuceneBasicStats, tf, length float64) Explanation {
	exp := NewExplanation(true, float32(n.Tfn(stats, tf, length)),
		"NormalizationH2, computed as tf * log2(1 + c * avgfl / fl) from:")
	exp.AddDetail(NewExplanation(true, float32(tf), "tf, number of occurrences of term in the document"))
	exp.AddDetail(NewExplanation(true, n.c, "c, hyper-parameter"))
	exp.AddDetail(NewExplanation(true, float32(stats.AvgFieldLength()),
		"avgfl, average length of field across all documents"))
	exp.AddDetail(NewExplanation(true, float32(length), "fl, field length of the document"))
	return exp
}

// String returns "2".
func (n *LuceneNormalizationH2) String() string { return "2" }

// LuceneNormalizationH3 mirrors NormalizationH3 — Dirichlet priors.
// tfn = (tf + mu * ((F+1)/(T+1))) / (fl + mu) * mu.
type LuceneNormalizationH3 struct {
	mu float32
}

// NewLuceneNormalizationH3 returns NormalizationH3 with mu=800.
func NewLuceneNormalizationH3() *LuceneNormalizationH3 {
	return NewLuceneNormalizationH3WithMu(800.0)
}

// NewLuceneNormalizationH3WithMu returns NormalizationH3 with the supplied mu.
func NewLuceneNormalizationH3WithMu(mu float32) *LuceneNormalizationH3 {
	if math.IsNaN(float64(mu)) || math.IsInf(float64(mu), 0) || mu < 0 {
		panic(fmt.Sprintf("illegal mu value: %v, must be a non-negative finite value", mu))
	}
	return &LuceneNormalizationH3{mu: mu}
}

// GetMu returns the mu parameter.
func (n *LuceneNormalizationH3) GetMu() float32 { return n.mu }

// Tfn implements LuceneDFRNormalization.
func (n *LuceneNormalizationH3) Tfn(stats *LuceneBasicStats, tf, length float64) float64 {
	mu := float64(n.mu)
	num := tf + mu*((float64(stats.TotalTermFreq())+1)/(float64(stats.NumberOfFieldTokens())+1))
	den := length + mu
	if den == 0 {
		return tf
	}
	return num / den * mu
}

// Explain returns the canonical NormalizationH3 tree.
func (n *LuceneNormalizationH3) Explain(stats *LuceneBasicStats, tf, length float64) Explanation {
	exp := NewExplanation(true, float32(n.Tfn(stats, tf, length)),
		"NormalizationH3, computed as (tf + mu * ((F+1) / (T+1))) / (fl + mu) * mu from:")
	exp.AddDetail(NewExplanation(true, float32(tf), "tf, number of occurrences of term in the document"))
	exp.AddDetail(NewExplanation(true, n.mu, "mu, smoothing parameter"))
	exp.AddDetail(NewExplanation(true, float32(stats.TotalTermFreq()),
		"F,  total number of occurrences of term across all documents"))
	exp.AddDetail(NewExplanation(true, float32(stats.NumberOfFieldTokens()),
		"T, total number of tokens of the field across all documents"))
	exp.AddDetail(NewExplanation(true, float32(length), "fl, field length of the document"))
	return exp
}

// String returns "3(mu)".
func (n *LuceneNormalizationH3) String() string { return fmt.Sprintf("3(%v)", n.mu) }

// LuceneNormalizationZ mirrors NormalizationZ — Pareto-Zipf.
// tfn = tf * (avgfl / fl)^z.
type LuceneNormalizationZ struct {
	z float32
}

// NewLuceneNormalizationZ returns NormalizationZ with z=0.30.
func NewLuceneNormalizationZ() *LuceneNormalizationZ {
	return NewLuceneNormalizationZWithZ(0.30)
}

// NewLuceneNormalizationZWithZ returns NormalizationZ with the supplied z.
// Panics if z is outside (0, 0.5).
func NewLuceneNormalizationZWithZ(z float32) *LuceneNormalizationZ {
	if math.IsNaN(float64(z)) || z <= 0 || z >= 0.5 {
		panic(fmt.Sprintf("illegal z value: %v, must be in the range (0 .. 0.5)", z))
	}
	return &LuceneNormalizationZ{z: z}
}

// GetZ returns the z parameter.
func (n *LuceneNormalizationZ) GetZ() float32 { return n.z }

// Tfn implements LuceneDFRNormalization.
func (n *LuceneNormalizationZ) Tfn(stats *LuceneBasicStats, tf, length float64) float64 {
	if length == 0 {
		return tf
	}
	return tf * math.Pow(stats.AvgFieldLength()/length, float64(n.z))
}

// Explain returns the canonical NormalizationZ tree.
func (n *LuceneNormalizationZ) Explain(stats *LuceneBasicStats, tf, length float64) Explanation {
	exp := NewExplanation(true, float32(n.Tfn(stats, tf, length)),
		"NormalizationZ, computed as tf * Math.pow(avgfl / fl, z) from:")
	exp.AddDetail(NewExplanation(true, float32(tf), "tf, number of occurrences of term in the document"))
	exp.AddDetail(NewExplanation(true, float32(stats.AvgFieldLength()),
		"avgfl, average length of field across all documents"))
	exp.AddDetail(NewExplanation(true, float32(length), "fl, field length of the document"))
	exp.AddDetail(NewExplanation(true, n.z, "z, relates to specificity of the language"))
	return exp
}

// String returns "Z(z)".
func (n *LuceneNormalizationZ) String() string { return fmt.Sprintf("Z(%v)", n.z) }

// Compile-time guarantees.
var (
	_ LuceneDFRNormalization = (*LuceneNoNormalization)(nil)
	_ LuceneDFRNormalization = (*LuceneNormalizationH1)(nil)
	_ LuceneDFRNormalization = (*LuceneNormalizationH2)(nil)
	_ LuceneDFRNormalization = (*LuceneNormalizationH3)(nil)
	_ LuceneDFRNormalization = (*LuceneNormalizationZ)(nil)
)
