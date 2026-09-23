// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestWildcardRandom.java
// (Apache Lucene 10.5.0).
//
// Create an index with terms from 000-999. Generates random wildcards
// according to patterns, and validates the correct number of hits are
// returned.

package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// wrSetUp renders setUp(); the returned function renders tearDown().
func wrSetUp(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)

	doc := document.NewDocument()
	field := newStringField(t, "field", "", false)
	doc.Add(field)

	// NumberFormat df = new DecimalFormat("000", new DecimalFormatSymbols(Locale.ROOT));
	for i := 0; i < 1000; i++ {
		field.SetStringValue(fmt.Sprintf("%03d", i))
		mustAddDocument(t, writer, doc)
	}

	reader := mustGetReader(t, writer)
	tearDown := func() {
		mustClose(t, reader, dir)
	}
	searcher := newSearcher(t, reader)
	mustClose(t, writer)
	if testing.Verbose() {
		t.Logf("TEST: setUp searcher=%v", searcher)
	}
	return searcher, tearDown
}

// wrAssertPatternHits renders the private assertPatternHits(String, int).
func wrAssertPatternHits(t *testing.T, searcher *search.IndexSearcher, pattern string, numHits int) {
	t.Helper()
	// TODO: run with different rewrites
	filledPattern := rrFillPattern(pattern)
	if testing.Verbose() {
		t.Logf("TEST: run wildcard pattern=%s filled=%s", pattern, filledPattern)
	}
	wq := search.NewWildcardQuery(index.NewTerm("field", filledPattern))
	docs := mustSearch(t, searcher, wq, 25)
	if docs.TotalHits.Value != int64(numHits) {
		t.Fatalf("Incorrect hits for pattern: %s: expected %d, got %d", pattern, numHits, docs.TotalHits.Value)
	}
}

func TestWildcardRandomWildcards(t *testing.T) {
	searcher, tearDown := wrSetUp(t)
	defer tearDown()
	num := atLeast(1)
	for i := 0; i < num; i++ {
		wrAssertPatternHits(t, searcher, "NNN", 1)
		wrAssertPatternHits(t, searcher, "?NN", 10)
		wrAssertPatternHits(t, searcher, "N?N", 10)
		wrAssertPatternHits(t, searcher, "NN?", 10)
	}

	for i := 0; i < num; i++ {
		wrAssertPatternHits(t, searcher, "??N", 100)
		wrAssertPatternHits(t, searcher, "N??", 100)
		wrAssertPatternHits(t, searcher, "???", 1000)

		wrAssertPatternHits(t, searcher, "NN*", 10)
		wrAssertPatternHits(t, searcher, "N*", 100)
		wrAssertPatternHits(t, searcher, "*", 1000)

		wrAssertPatternHits(t, searcher, "*NN", 10)
		wrAssertPatternHits(t, searcher, "*N", 100)

		wrAssertPatternHits(t, searcher, "N*N", 10)

		// combo of ? and * operators
		wrAssertPatternHits(t, searcher, "?N*", 100)
		wrAssertPatternHits(t, searcher, "N?*", 100)

		wrAssertPatternHits(t, searcher, "*N?", 100)
		wrAssertPatternHits(t, searcher, "*??", 1000)
		wrAssertPatternHits(t, searcher, "*?N", 100)
	}
}
