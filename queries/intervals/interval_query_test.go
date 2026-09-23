// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervals_test

// Port of
// lucene/queries/src/test/org/apache/lucene/queries/intervals/TestIntervalQuery.java
// (Apache Lucene 10.5.0).
//
// Blocker: setUp() runs before every test method and ends with
// LuceneTestCase.newSearcher(reader), which always wraps an
// org.apache.lucene.tests.search.AssertingIndexSearcher (not ported). setUp is
// rendered in full up to that call (the documents are indexed with a
// RandomIndexWriter and a reader is opened), then every test fails naming the
// missing class.

import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs" // registers the default codec (Codec.getDefault())
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

const intervalQueryField = "field"

var intervalQueryDocFields = []string{
	"w1 w2 w3 w4 w5",
	"w1 w3 w2 w3",
	"w1 xx w2 w4 yy w3",
	"w1 w3 xx w2 yy w3",
	"w2 w1",
	"w2 w1 w3 w2 w4",
	"coordinate genome mapping research",
	"coordinate genome research",
	"greater new york",
	"x x x x x intend x x x message x x x message x x x addressed x x",
	"issue with intervals queries from search engine. So it's a big issue for us as we need to do ordered searches. Thank you to help us concerning that issue",
	"場外好朋友",
	"alice bob alice alice carl alice bob alice carl",
}

// intervalQuerySetUp renders TestIntervalQuery.setUp() and tearDown().
func intervalQuerySetUp(t *testing.T) {
	t.Helper()
	directory := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	iwc := index.NewIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
	iwc.SetMergePolicy(index.NewLogDocMergePolicy())
	writer, err := testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(rand.Int63())), directory, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	for _, docField := range intervalQueryDocFields {
		doc := document.NewDocument()
		f, err := document.NewTextField(intervalQueryField, docField, true)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		if err := directory.Close(); err != nil {
			t.Errorf("close directory: %v", err)
		}
	})
	// searcher = newSearcher(reader);
	t.Fatal("requires org.apache.lucene.tests.search.AssertingIndexSearcher (LuceneTestCase.newSearcher) (not ported)")
}

func TestIntervalQuery_testPhraseQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedNearQueryWidth3(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedNearQueryWidth4(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedNearQueryGaps1(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedNearQueryGaps2(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrderedNearQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testUnorderedQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNonOverlappingQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testFieldInToString(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNullConstructorArgs(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNotWithinQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNotContainingQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testContainingQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testContainedByQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNotContainedByQuery(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNonExistentTerms(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOr(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrWithGaps(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrWithinDifference(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrWithinConjunctionFilter(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testUnordered(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testUnorderedNoOverlaps(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrInUnorderedMaxGaps(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testUnorderedWithNoGap(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedWithGaps(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedWithGaps2(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testOrderedWithNoGap(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testNestedOrInContainedBy(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testDefinedGaps(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testScoring(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testAdvanceBehavior(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testUnicodePrefix(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testExtendDisjunctions(t *testing.T) {
	intervalQuerySetUp(t)
}

func TestIntervalQuery_testEquality(t *testing.T) {
	intervalQuerySetUp(t)
}
