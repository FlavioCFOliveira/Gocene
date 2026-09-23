// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans

// Port of lucene/queries/src/test/org/apache/lucene/queries/spans/TestBasics.java
// (Apache Lucene 10.5.0).
//
// Blockers: beforeClass() builds the shared searcher with
// LuceneTestCase.newSearcher, which always wraps an
// org.apache.lucene.tests.search.AssertingIndexSearcher (not ported); the
// span queries of the test methods are built by the test-side
// org.apache.lucene.queries.spans.SpanTestUtil, which wraps every query in an
// AssertingSpanQuery (with AssertingSpanWeight and AssertingSpans), none of
// which Gocene has ported; and the corpus text comes from
// org.apache.lucene.tests.util.English (not ported). testBooleanSpanQuery and
// testDismaxSpanQuery build their own index but search it through
// newSearcher as well. Every test method therefore fails naming them.

import "testing"

const basicsBlocker = "requires org.apache.lucene.tests.search.AssertingIndexSearcher (LuceneTestCase.newSearcher), " +
	"org.apache.lucene.queries.spans.SpanTestUtil with AssertingSpanQuery/AssertingSpanWeight/AssertingSpans, " +
	"and org.apache.lucene.tests.util.English (not ported)"

// basicsBeforeClass renders TestBasics.beforeClass(): 2000 documents whose
// "field" is English.intToEnglish(i), searched through newSearcher.
func basicsBeforeClass(t *testing.T) {
	t.Helper()
	t.Fatal(basicsBlocker)
}

func TestBasics_testTerm(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testTerm2(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testPhrase(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testPhrase2(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testBoolean(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testBoolean2(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNearExact(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanTermQuery(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNearUnordered(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNearOrdered(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNot(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotNoOverflowOnLargeSpans(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanWithMultipleNotSingle(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanWithMultipleNotMany(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testNpeInSpanNearWithSpanNot(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testNpeInSpanNearInSpanFirstInSpanNot(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotWindowOne(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotWindowTwoBefore(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotWindowNegPost(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotWindowNegPre(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNotWindowDoubleExcludesBefore(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanFirst(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanPositionRange(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanOr(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanExactNested(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanNearOr(t *testing.T) {
	basicsBeforeClass(t)
}

func TestBasics_testSpanComplex1(t *testing.T) {
	basicsBeforeClass(t)
}

// LUCENE-4477 / LUCENE-4401:
func TestBasics_testBooleanSpanQuery(t *testing.T) {
	t.Fatal(basicsBlocker)
}

// LUCENE-4477 / LUCENE-4401:
func TestBasics_testDismaxSpanQuery(t *testing.T) {
	t.Fatal(basicsBlocker)
}
