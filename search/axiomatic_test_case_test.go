// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/AxiomaticTestCase.java
//
// AxiomaticTestCase is the abstract BaseSimilarityTestCase subclass that builds
// an Axiomatic similarity from randomized parameters s (in [0,1]), query length
// and k (in [0,1]) and runs the inherited scoring-invariant tests. Its concrete
// subclasses pick a specific Axiomatic F-variant via getAxiomaticModel.
//
// Go has no abstract test classes, so this port materialises the Axiomatic
// F-variants (F1EXP/F1LOG/F2EXP/F2LOG/F3EXP/F3LOG) directly and runs the shared
// BaseSimilarityTestCase scoring invariants (checkSimilarityScoring) across the
// boundary parameter values AxiomaticTestCase.getSimilarity selects (s and k in
// {0, tiny, 1} and a representative query length), exactly as the concrete
// subclasses would.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// axiomaticParams mirrors the boundary values AxiomaticTestCase.getSimilarity
// draws for the s and k parameters: minimum, tiny, and maximum.
var axiomaticParams = []float32{0, math.SmallestNonzeroFloat32, 1}

// axiomaticQueryLens mirrors the boundary query-length values (minimum, tiny,
// and a large representative value).
var axiomaticQueryLens = []int{0, 1, 1 << 20}

func TestAxiomaticTestCase_F1EXP(t *testing.T) {
	for _, s := range axiomaticParams {
		for _, k := range axiomaticParams {
			checkSimilarityScoring(t, "F1EXP", search.NewLuceneAxiomaticF1EXP(s, k), false)
		}
	}
}

func TestAxiomaticTestCase_F1LOG(t *testing.T) {
	for _, s := range axiomaticParams {
		checkSimilarityScoring(t, "F1LOG", search.NewLuceneAxiomaticF1LOG(s), false)
	}
}

func TestAxiomaticTestCase_F2EXP(t *testing.T) {
	for _, s := range axiomaticParams {
		for _, k := range axiomaticParams {
			checkSimilarityScoring(t, "F2EXP", search.NewLuceneAxiomaticF2EXP(s, k), false)
		}
	}
}

func TestAxiomaticTestCase_F2LOG(t *testing.T) {
	for _, s := range axiomaticParams {
		checkSimilarityScoring(t, "F2LOG", search.NewLuceneAxiomaticF2LOG(s), false)
	}
}

func TestAxiomaticTestCase_F3EXP(t *testing.T) {
	for _, s := range axiomaticParams {
		for _, queryLen := range axiomaticQueryLens {
			for _, k := range axiomaticParams {
				checkSimilarityScoring(t, "F3EXP", search.NewLuceneAxiomaticF3EXP(s, queryLen, k), false)
			}
		}
	}
}

func TestAxiomaticTestCase_F3LOG(t *testing.T) {
	for _, s := range axiomaticParams {
		for _, queryLen := range axiomaticQueryLens {
			checkSimilarityScoring(t, "F3LOG", search.NewLuceneAxiomaticF3LOG(s, queryLen), false)
		}
}	}
