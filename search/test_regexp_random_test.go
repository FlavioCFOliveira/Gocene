// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestRegexpRandom.java
// (Apache Lucene 10.5.0).
//
// Create an index with terms from 000-999. Generates random regexps according
// to simple patterns, and validates the correct number of hits are returned.

package search_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// rrSetUp renders setUp(); the returned function renders tearDown().
func rrSetUp(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)

	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetOmitNorms(true)
	field := newField(t, "field", "", customType)
	doc.Add(field)

	// NumberFormat df = new DecimalFormat("000", new DecimalFormatSymbols(Locale.ROOT));
	for i := 0; i < 1000; i++ {
		field.SetStringValue(fmt.Sprintf("%03d", i))
		mustAddDocument(t, writer, doc)
	}

	reader := mustGetReader(t, writer)
	mustClose(t, writer)
	tearDown := func() {
		mustClose(t, reader, dir)
	}
	searcher := newSearcher(t, reader)
	return searcher, tearDown
}

// rrN renders the private N(): a random digit.
func rrN() byte {
	return byte(0x30 + random().Intn(10))
}

// rrFillPattern renders the private fillPattern(String).
func rrFillPattern(wildcardPattern string) string {
	var sb strings.Builder
	for i := 0; i < len(wildcardPattern); i++ {
		switch wildcardPattern[i] {
		case 'N':
			sb.WriteByte(rrN())
		default:
			sb.WriteByte(wildcardPattern[i])
		}
	}
	return sb.String()
}

// rrAssertPatternHits renders the private assertPatternHits(String, int).
func rrAssertPatternHits(t *testing.T, searcher *search.IndexSearcher, pattern string, numHits int) {
	t.Helper()
	wq := search.NewRegexpQuery(index.NewTerm("field", rrFillPattern(pattern)))
	docs := mustSearch(t, searcher, wq, 25)
	if docs.TotalHits.Value != int64(numHits) {
		t.Fatalf("Incorrect hits for pattern: %s: expected %d, got %d", pattern, numHits, docs.TotalHits.Value)
	}
}

func TestRegexpRandomRegexps(t *testing.T) {
	searcher, tearDown := rrSetUp(t)
	defer tearDown()
	num := atLeast(1)
	for i := 0; i < num; i++ {
		rrAssertPatternHits(t, searcher, "NNN", 1)
		rrAssertPatternHits(t, searcher, ".NN", 10)
		rrAssertPatternHits(t, searcher, "N.N", 10)
		rrAssertPatternHits(t, searcher, "NN.", 10)
	}

	for i := 0; i < num; i++ {
		rrAssertPatternHits(t, searcher, ".{1,2}N", 100)
		rrAssertPatternHits(t, searcher, "N.{1,2}", 100)
		rrAssertPatternHits(t, searcher, ".{1,3}", 1000)

		rrAssertPatternHits(t, searcher, "NN[3-7]", 5)
		rrAssertPatternHits(t, searcher, "N[2-6][3-7]", 25)
		rrAssertPatternHits(t, searcher, "[1-5][2-6][3-7]", 125)
		rrAssertPatternHits(t, searcher, "[0-4][3-7][4-8]", 125)
		rrAssertPatternHits(t, searcher, "[2-6][0-4]N", 25)
		rrAssertPatternHits(t, searcher, "[2-6]NN", 5)

		rrAssertPatternHits(t, searcher, "NN.*", 10)
		rrAssertPatternHits(t, searcher, "N.*", 100)
		rrAssertPatternHits(t, searcher, ".*", 1000)

		rrAssertPatternHits(t, searcher, ".*NN", 10)
		rrAssertPatternHits(t, searcher, ".*N", 100)

		rrAssertPatternHits(t, searcher, "N.*N", 10)

		// combo of ? and * operators
		rrAssertPatternHits(t, searcher, ".N.*", 100)
		rrAssertPatternHits(t, searcher, "N..*", 100)

		rrAssertPatternHits(t, searcher, ".*N.", 100)
		rrAssertPatternHits(t, searcher, ".*..", 1000)
		rrAssertPatternHits(t, searcher, ".*.N", 100)
	}
}
