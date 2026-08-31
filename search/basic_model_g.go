// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

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
