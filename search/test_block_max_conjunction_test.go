// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBlockMaxConjunction.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// bmcMaybeWrap renders the private maybeWrap(Query).
func bmcMaybeWrap(t *testing.T, query search.Query) search.Query {
	t.Helper()
	if random().Intn(2) == 0 {
		// query = new BlockScoreQueryWrapper(query, TestUtil.nextInt(random(), 2, 8));
		// query = new AssertingQuery(random(), query);
		t.Fatal(blockScoreQueryWrapperBlocker)
	}
	return query
}

// bmcMaybeWrapTwoPhase renders the private maybeWrapTwoPhase(Query).
func bmcMaybeWrapTwoPhase(t *testing.T, query search.Query) search.Query {
	t.Helper()
	if random().Intn(2) == 0 {
		query = testsearch.NewRandomApproximationQuery(query, random())
		// query = new AssertingQuery(random(), query);
		t.Fatal(assertingQueryBlocker)
	}
	return query
}

func TestBlockMaxConjunctionRandom(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	numDocs := atLeast(1000)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numValues := random().Intn(1 << random().Intn(5))
		start := random().Intn(10)
		for j := 0; j < numValues; j++ {
			doc.Add(mustStringField(t, "foo", strconv.Itoa(start+j), false))
		}
		mustAddDocument(t, w, doc)
	}
	reader := mustOpenDirectoryReaderFromWriter(t, w)
	mustClose(t, w)
	// Disable search concurrency for this test: it requires a single segment, and no intra-segment
	// concurrency for its assertions to always be valid
	searcher := newSearcherWithOptions(t, reader, random().Intn(2) == 0, random().Intn(2) == 0, false)

	for iter := 0; iter < 100; iter++ {
		start := random().Intn(10)
		numClauses := random().Intn(1 << random().Intn(5))
		builder := search.NewBooleanQueryBuilder()
		for i := 0; i < numClauses; i++ {
			builder.Add(bmcMaybeWrap(t, search.NewTermQuery(index.NewTerm("foo", strconv.Itoa(start+i)))), search.MUST)
		}
		query := builder.Build()

		testsearch.CheckTopScores(t, random(), query, searcher)

		filterTerm := random().Intn(30)
		filteredQuery := search.NewBooleanQueryBuilder().
			Add(query, search.MUST).
			Add(search.NewTermQuery(index.NewTerm("foo", strconv.Itoa(filterTerm))), search.FILTER).
			Build()

		testsearch.CheckTopScores(t, random(), filteredQuery, searcher)

		builder = search.NewBooleanQueryBuilder()
		for i := 0; i < numClauses; i++ {
			builder.Add(bmcMaybeWrapTwoPhase(t, search.NewTermQuery(index.NewTerm("foo", strconv.Itoa(start+i)))), search.MUST)
		}

		twoPhaseQuery := search.NewBooleanQueryBuilder().
			Add(query, search.MUST).
			Add(search.NewTermQuery(index.NewTerm("foo", strconv.Itoa(filterTerm))), search.FILTER).
			Build()

		testsearch.CheckTopScores(t, random(), twoPhaseQuery, searcher)
	}
	mustClose(t, reader, dir)
}
