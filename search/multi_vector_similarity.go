// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiVectorSimilarity computes similarity between two multi-vectors using a
// per-vector index.VectorSimilarityFunction. Each multi-vector is a list of token
// vectors of the same dimension; the two multi-vectors may differ in token
// count.
//
// Mirrors org.apache.lucene.search.MultiVectorSimilarity.
type MultiVectorSimilarity interface {
	Compare(a, b [][]float32, sim index.VectorSimilarityFunction) float32
}

// SumMaxSimilarity is the canonical MultiVectorSimilarity that returns, for
// each token vector in a, the maximum similarity against any token vector in
// b, then sums those maxima.
type SumMaxSimilarity struct{}

// Compare implements MultiVectorSimilarity for SumMaxSimilarity.
func (SumMaxSimilarity) Compare(a, b [][]float32, sim index.VectorSimilarityFunction) float32 {
	total := float32(0)
	for _, q := range a {
		var best float32 = -1
		first := true
		for _, d := range b {
			s := sim.CompareFloat(q, d)
			if first || s > best {
				best = s
				first = false
			}
		}
		if !first {
			total += best
		}
	}
	return total
}
