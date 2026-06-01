// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/DistributionTestCase.java
//
// DistributionTestCase is the abstract BaseSimilarityTestCase subclass that
// builds an IBSimilarity from the concrete subclass's Distribution, a randomly
// chosen Lambda (DF or TTF) and a randomly chosen Normalization, then runs the
// inherited scoring-invariant tests.
//
// Go has no abstract test classes, so this port materialises the IB
// distributions Gocene exposes (LL/SPL) and runs the shared
// BaseSimilarityTestCase scoring invariants (checkSimilarityScoring) across both
// Lambdas and every Normalization, exactly as the concrete subclasses (one per
// Distribution) would.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ibDistributions returns the IB distributions Gocene exposes, mirroring the
// set of concrete DistributionTestCase subclasses.
func ibDistributions() map[string]func() search.LuceneIBDistribution {
	return map[string]func() search.LuceneIBDistribution{
		"LL":  func() search.LuceneIBDistribution { return search.NewLuceneDistributionLL() },
		"SPL": func() search.LuceneIBDistribution { return search.NewLuceneDistributionSPL() },
	}
}

// ibLambdas returns the two Lambda choices DistributionTestCase draws.
func ibLambdas() map[string]func() search.LuceneIBLambda {
	return map[string]func() search.LuceneIBLambda{
		"DF":  func() search.LuceneIBLambda { return search.NewLuceneLambdaDF() },
		"TTF": func() search.LuceneIBLambda { return search.NewLuceneLambdaTTF() },
	}
}

func TestDistributionTestCase(t *testing.T) {
	for distName, distribution := range ibDistributions() {
		for lambdaName, lambda := range ibLambdas() {
			for normName, normalization := range dfrNormalizations() {
				name := "IB(" + distName + "," + lambdaName + "," + normName + ")"
				sim := search.NewLuceneIBSimilarity(distribution(), lambda(), normalization())
				checkSimilarityScoring(t, name, sim, false)
			}
		}
	}
}
