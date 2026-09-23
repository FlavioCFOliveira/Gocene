// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/search/TestMinShouldMatch2.java
// (Apache Lucene 10.5.0); the gocene_monsters build tag renders the
// tests.nightly switch.

package search_test

import "testing"

// test advance with varying numbers of terms with varying minShouldMatch
func TestMinShouldMatch2AdvanceVaryingNumberOfTerms(t *testing.T) {
	c := msm2BeforeClass(t)
	termsList := msm2AllTermsList()
	random().Shuffle(len(termsList), func(i, j int) { termsList[i], termsList[j] = termsList[j], termsList[i] })

	for amount := 25; amount < 200; amount += 25 {
		for numTerms := 2; numTerms <= len(termsList); numTerms++ {
			terms := append([]string(nil), termsList[:numTerms]...)
			for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
				expected := c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
				actual := c.scorer(t, terms, minNrShouldMatch, msm2Scorer)
				msm2AssertAdvance(t, expected, actual, amount)

				expected = c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
				actual = c.scorer(t, terms, minNrShouldMatch, msm2Scorer)
				msm2AssertAdvance(t, expected, actual, amount)
			}
		}
	}
}
