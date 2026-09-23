// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans

// Port of
// lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanSimilarity.java
// (Apache Lucene 10.5.0).
//
// Blockers: setUp() fills the similarity list with classes of
// org.apache.lucene.search.similarities that Gocene has not ported
// (AxiomaticF1EXP, AxiomaticF1LOG, AxiomaticF2EXP, AxiomaticF2LOG,
// AxiomaticF3EXP, AxiomaticF3LOG, BooleanSimilarity, BasicModelG,
// BasicModelIF, BasicModelIn, BasicModelIne, NormalizationH3,
// Normalization.NoNormalization, LambdaDF, LambdaTTF, DFISimilarity,
// IndependenceStandardized, IndependenceSaturated, IndependenceChiSquared),
// and testCrazySpans searches through LuceneTestCase.newSearcher, which always
// wraps an org.apache.lucene.tests.search.AssertingIndexSearcher (not ported).

import "testing"

// spanSimilaritySetUpBlocker names what setUp() needs.
const spanSimilaritySetUpBlocker = "requires org.apache.lucene.search.similarities AxiomaticF1EXP, AxiomaticF1LOG, " +
	"AxiomaticF2EXP, AxiomaticF2LOG, AxiomaticF3EXP, AxiomaticF3LOG, BooleanSimilarity, BasicModelG, " +
	"BasicModelIF, BasicModelIn, BasicModelIne, NormalizationH3, Normalization.NoNormalization, LambdaDF, " +
	"LambdaTTF, DFISimilarity, IndependenceStandardized, IndependenceSaturated, IndependenceChiSquared and " +
	"org.apache.lucene.tests.search.AssertingIndexSearcher (not ported)"

// make sure all sims work with spanOR(termX, termY) where termY does not exist
func TestSpanSimilarity_testCrazySpans(t *testing.T) {
	// setUp() cannot build the similarity list.
	t.Fatal(spanSimilaritySetUpBlocker)
}
