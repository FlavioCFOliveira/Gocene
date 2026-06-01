// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/BasicModelTestCase.java
//
// BasicModelTestCase is the abstract BaseSimilarityTestCase subclass that builds
// a DFRSimilarity from the concrete subclass's BasicModel, a randomly chosen
// AfterEffect (L or B) and a randomly chosen Normalization (H1/H2/H3/Z), then
// runs the inherited scoring-invariant tests.
//
// Go has no abstract test classes, so this port materialises the DFR
// BasicModels Gocene exposes (G/IF/Ine/In) and runs the shared
// BaseSimilarityTestCase scoring invariants (checkSimilarityScoring) across both
// AfterEffects and every Normalization, exactly as the concrete subclasses
// (one per BasicModel) would.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// dfrBasicModels returns the DFR basic models Gocene exposes, mirroring the set
// of concrete BasicModelTestCase subclasses.
func dfrBasicModels() map[string]func() search.LuceneDFRBasicModel {
	return map[string]func() search.LuceneDFRBasicModel{
		"G":   func() search.LuceneDFRBasicModel { return search.NewLuceneBasicModelG() },
		"IF":  func() search.LuceneDFRBasicModel { return search.NewLuceneBasicModelIF() },
		"Ine": func() search.LuceneDFRBasicModel { return search.NewLuceneBasicModelIne() },
		"In":  func() search.LuceneDFRBasicModel { return search.NewLuceneBasicModelIn() },
	}
}

// dfrAfterEffects returns the two AfterEffect choices BasicModelTestCase draws.
func dfrAfterEffects() map[string]func() search.LuceneDFRAfterEffect {
	return map[string]func() search.LuceneDFRAfterEffect{
		"L": func() search.LuceneDFRAfterEffect { return search.NewLuceneAfterEffectL() },
		"B": func() search.LuceneDFRAfterEffect { return search.NewLuceneAfterEffectB() },
	}
}

// dfrNormalizations returns the normalizations BasicModelTestCase draws.
func dfrNormalizations() map[string]func() search.LuceneDFRNormalization {
	return map[string]func() search.LuceneDFRNormalization{
		"H1": func() search.LuceneDFRNormalization { return search.NewLuceneNormalizationH1() },
		"H2": func() search.LuceneDFRNormalization { return search.NewLuceneNormalizationH2() },
		"H3": func() search.LuceneDFRNormalization { return search.NewLuceneNormalizationH3() },
		"Z":  func() search.LuceneDFRNormalization { return search.NewLuceneNormalizationZ() },
	}
}

func TestBasicModelTestCase(t *testing.T) {
	for modelName, model := range dfrBasicModels() {
		for aeName, afterEffect := range dfrAfterEffects() {
			for normName, normalization := range dfrNormalizations() {
				name := "DFR(" + modelName + "," + aeName + "," + normName + ")"
				sim := search.NewLuceneDFRSimilarity(model(), afterEffect(), normalization())
				checkSimilarityScoring(t, name, sim, false)
			}
		}
	}
}
