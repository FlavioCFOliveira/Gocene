// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// LuceneDFRBasicModel is the Go port of
// org.apache.lucene.search.similarities.BasicModel — the abstract base for
// DFR basic models.
//
// Java's BasicModel is an abstract class with two abstract methods; in Go
// we expose the same contract as an interface.
type LuceneDFRBasicModel interface {
	// Score returns informationContent * aeTimes1pTfn / (1 + tfn). Must be
	// non-decreasing with tfn.
	Score(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) float64

	// Explain returns the canonical Explanation tree for the model.
	Explain(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) Explanation

	// String returns the model code (e.g. "G", "I(n)").
	String() string
}

// LuceneBasicModelG mirrors org.apache.lucene.search.similarities.
// BasicModelG — Geometric as limiting form of Bose-Einstein. Stateless.
type LuceneBasicModelG struct{}

// NewLuceneBasicModelG constructs a BasicModelG. The Java class is
// parameter-free; constructor mirrors the no-arg signature.
func NewLuceneBasicModelG() *LuceneBasicModelG { return &LuceneBasicModelG{} }

// Score implements LuceneDFRBasicModel.
// Formula (canonical Lucene rewrite): (B - (B - A) / (1 + tfn)) * aeTimes1pTfn,
// where A = log2(lambda + 1), B = log2((1 + lambda) / lambda),
// lambda = F / (N + F), F = totalTermFreq + 1, N = numberOfDocuments.
func (LuceneBasicModelG) Score(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) float64 {
	F := float64(stats.TotalTermFreq() + 1)
	N := float64(stats.NumberOfDocuments())
	lambda := F / (N + F)
	A := LuceneSimLog2(lambda + 1)
	B := LuceneSimLog2((1 + lambda) / lambda)
	return (B - (B-A)/(1+tfn)) * aeTimes1pTfn
}

// Explain returns the canonical BasicModelG tree.
func (m LuceneBasicModelG) Explain(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) Explanation {
	F := float64(stats.TotalTermFreq() + 1)
	N := float64(stats.NumberOfDocuments())
	lambda := F / (N + F)
	lambdaExp := NewExplanation(true, float32(lambda),
		"lambda, computed as F / (N + F) from:")
	lambdaExp.AddDetail(NewExplanation(true, float32(F),
		"F, total number of occurrences of term across all docs + 1"))
	lambdaExp.AddDetail(NewExplanation(true, float32(N),
		"N, total number of documents with field"))
	score := m.Score(stats, tfn, aeTimes1pTfn)
	exp := NewExplanation(true, float32(score*(1+tfn)/aeTimes1pTfn),
		"BasicModelG, computed as log2(lambda + 1) + tfn * log2((1 + lambda) / lambda) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	exp.AddDetail(lambdaExp)
	return exp
}

// String returns "G".
func (LuceneBasicModelG) String() string { return "G" }

// LuceneBasicModelIF mirrors BasicModelIF — approximation of I(ne).
type LuceneBasicModelIF struct{}

// NewLuceneBasicModelIF returns the parameter-free I(F) basic model.
func NewLuceneBasicModelIF() *LuceneBasicModelIF { return &LuceneBasicModelIF{} }

// Score implements LuceneDFRBasicModel.
// A * aeTimes1pTfn * (1 - 1 / (1 + tfn)), A = log2(1 + (N+1)/(F+0.5)).
func (LuceneBasicModelIF) Score(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) float64 {
	N := float64(stats.NumberOfDocuments())
	F := float64(stats.TotalTermFreq())
	A := LuceneSimLog2(1 + (N+1)/(F+0.5))
	return A * aeTimes1pTfn * (1 - 1/(1+tfn))
}

// Explain returns the canonical I(F) tree.
func (m LuceneBasicModelIF) Explain(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) Explanation {
	score := m.Score(stats, tfn, aeTimes1pTfn)
	exp := NewExplanation(true, float32(score*(1+tfn)/aeTimes1pTfn),
		"BasicModelIF, computed as tfn * log2(1 + (N + 1) / (F + 0.5)) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()),
		"N, total number of documents with field"))
	exp.AddDetail(NewExplanation(true, float32(stats.TotalTermFreq()),
		"F, total number of occurrences of term across all documents"))
	return exp
}

// String returns "I(F)".
func (LuceneBasicModelIF) String() string { return "I(F)" }

// LuceneBasicModelIne mirrors BasicModelIne — Tf-idf mixture of Poisson
// and inverse document frequency.
type LuceneBasicModelIne struct{}

// NewLuceneBasicModelIne returns the parameter-free I(ne) basic model.
func NewLuceneBasicModelIne() *LuceneBasicModelIne { return &LuceneBasicModelIne{} }

// Score implements LuceneDFRBasicModel.
// A * aeTimes1pTfn * (1 - 1 / (1 + tfn)),
// ne = N * (1 - ((N-1)/N)^F), A = log2((N+1)/(ne+0.5)).
func (LuceneBasicModelIne) Score(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) float64 {
	N := float64(stats.NumberOfDocuments())
	F := float64(stats.TotalTermFreq())
	ne := N * (1 - math.Pow((N-1)/N, F))
	A := LuceneSimLog2((N + 1) / (ne + 0.5))
	return A * aeTimes1pTfn * (1 - 1/(1+tfn))
}

// Explain returns the canonical I(ne) tree.
func (m LuceneBasicModelIne) Explain(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) Explanation {
	F := float64(stats.TotalTermFreq())
	N := float64(stats.NumberOfDocuments())
	ne := N * (1 - math.Pow((N-1)/N, F))
	neExp := NewExplanation(true, float32(ne),
		"ne, computed as N * (1 - Math.pow((N - 1) / N, F)) from:")
	neExp.AddDetail(NewExplanation(true, float32(F),
		"F, total number of occurrences of term across all docs"))
	neExp.AddDetail(NewExplanation(true, float32(N),
		"N, total number of documents with field"))
	score := m.Score(stats, tfn, aeTimes1pTfn)
	exp := NewExplanation(true, float32(score*(1+tfn)/aeTimes1pTfn),
		"BasicModelIne, computed as tfn * log2((N + 1) / (ne + 0.5)) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	exp.AddDetail(neExp)
	return exp
}

// String returns "I(ne)".
func (LuceneBasicModelIne) String() string { return "I(ne)" }

// LuceneBasicModelIn mirrors BasicModelIn — basic tf-idf model of randomness.
type LuceneBasicModelIn struct{}

// NewLuceneBasicModelIn returns the parameter-free I(n) basic model.
func NewLuceneBasicModelIn() *LuceneBasicModelIn { return &LuceneBasicModelIn{} }

// Score implements LuceneDFRBasicModel.
// A * aeTimes1pTfn * (1 - 1/(1+tfn)), A = log2((N+1)/(n+0.5)).
func (LuceneBasicModelIn) Score(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) float64 {
	N := float64(stats.NumberOfDocuments())
	n := float64(stats.DocFreq())
	A := LuceneSimLog2((N + 1) / (n + 0.5))
	return A * aeTimes1pTfn * (1 - 1/(1+tfn))
}

// Explain returns the canonical I(n) tree.
func (m LuceneBasicModelIn) Explain(stats *LuceneBasicStats, tfn, aeTimes1pTfn float64) Explanation {
	score := m.Score(stats, tfn, aeTimes1pTfn)
	exp := NewExplanation(true, float32(score*(1+tfn)/aeTimes1pTfn),
		"BasicModelIn, computed as tfn * log2((N + 1) / (n + 0.5)) from:")
	exp.AddDetail(NewExplanation(true, float32(tfn), "tfn, normalized term frequency"))
	exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()),
		"N, total number of documents with field"))
	exp.AddDetail(NewExplanation(true, float32(stats.DocFreq()),
		"n, number of documents containing term"))
	return exp
}

// String returns "I(n)".
func (LuceneBasicModelIn) String() string { return "I(n)" }

// Compile-time guarantees.
var (
	_ LuceneDFRBasicModel = LuceneBasicModelG{}
	_ LuceneDFRBasicModel = LuceneBasicModelIF{}
	_ LuceneDFRBasicModel = LuceneBasicModelIne{}
	_ LuceneDFRBasicModel = LuceneBasicModelIn{}
)

// modelNotFinite is a defensive helper invoked from tests; production
// scoring relies on Lucene's documented monotonicity guarantees.
func modelNotFinite(name string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s produced non-finite value %v", name, v)
	}
	return nil
}
