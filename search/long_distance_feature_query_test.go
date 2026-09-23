// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/document/TestLongDistanceFeatureQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// newLogMergePolicyUseCFS renders LuceneTestCase.newLogMergePolicy(boolean useCFS).
func newLogMergePolicyUseCFS(useCFS bool) logMergePolicy {
	logmp := newLogMergePolicy()
	if useCFS {
		logmp.SetNoCFSRatio(1.0)
	} else {
		logmp.SetNoCFSRatio(0.0)
	}
	return logmp
}

// mustLongDistanceFeatureQuery renders LongField.newDistanceFeatureQuery(String, float, long, long).
func mustLongDistanceFeatureQuery(t *testing.T, field string, weight float32, origin, pivotDistance int64) search.Query {
	t.Helper()
	q, err := search.LongFieldNewDistanceFeatureQuery(field, weight, origin, pivotDistance)
	if err != nil {
		t.Fatalf("LongField.newDistanceFeatureQuery: %v", err)
	}
	return q
}

// mustLongField renders new LongField(String, long, Store).
func mustLongField(t *testing.T, name string, value int64, stored bool) *document.LongFieldLucene {
	t.Helper()
	f, err := document.NewLongFieldLucene(name, value, stored)
	if err != nil {
		t.Fatalf("new LongField: %v", err)
	}
	return f
}

// newLongDistanceWriter renders new RandomIndexWriter(random(), dir,
// newIndexWriterConfig().setMergePolicy(newLogMergePolicy(random().nextBoolean()))).
func newLongDistanceWriterConfig() *index.IndexWriterConfig {
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newLogMergePolicyUseCFS(random().Intn(2) == 0))
	return iwc
}

func TestLongDistanceFeatureQueryEqualsAndHashcode(t *testing.T) {
	q1 := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	q2 := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	queryUtilsCheckEqual(t, q1, q2)

	q3 := mustLongDistanceFeatureQuery(t, "bar", 3, 10, 5)
	queryUtilsCheckUnequal(t, q1, q3)

	q4 := mustLongDistanceFeatureQuery(t, "foo", 4, 10, 5)
	queryUtilsCheckUnequal(t, q1, q4)

	q5 := mustLongDistanceFeatureQuery(t, "foo", 3, 9, 5)
	queryUtilsCheckUnequal(t, q1, q5)

	q6 := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 6)
	queryUtilsCheckUnequal(t, q1, q6)
}

func TestLongDistanceFeatureQueryBasics(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	field := mustLongField(t, "foo", 0, false)
	doc := newTestDocument(field)

	for _, v := range []int64{3, 12, 8, -1, 7} {
		field.SetLongValue(v)
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(1, float32(3*(5./(5.+2.))), -1),
			search.NewScoreDoc(2, float32(3*(5./(5.+2.))), -1),
		},
		topHits.ScoreDocs)

	q = mustLongDistanceFeatureQuery(t, "foo", 3, 7, 5)
	collectorManager = mustTopScoreDocCollectorManager(t, 2, 1)
	topHits = mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}
	testsearch.CheckExplanations(t, q, "", searcher, false)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(4, float32(3*(5./(5.+0.))), -1),
			search.NewScoreDoc(2, float32(3*(5./(5.+1.))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLongDistanceFeatureQueryOverUnderFlow(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	field := mustLongField(t, "foo", 0, false)
	doc := newTestDocument(field)

	for _, v := range []int64{3, 12, -10, math.MaxInt64, math.MinInt64} {
		field.SetLongValue(v)
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLongDistanceFeatureQuery(t, "foo", 3, math.MaxInt64-1, 100)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(3, float32(3*(100./(100.+1.))), -1),
			// rounding makes the distance treated as if it was MAX_VALUE
			search.NewScoreDoc(0, float32(3*(100./(100.+float64(math.MaxInt64)))), -1),
		},
		topHits.ScoreDocs)

	q = mustLongDistanceFeatureQuery(t, "foo", 3, math.MinInt64+1, 100)
	topHits = mustSearchWithManager(t, searcher, q, collectorManager)

	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}
	testsearch.CheckExplanations(t, q, "", searcher, false)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(4, float32(3*(100./(100.+1.))), -1),
			// rounding makes the distance treated as if it was MAX_VALUE
			search.NewScoreDoc(0, float32(3*(100./(100.+float64(math.MaxInt64)))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLongDistanceFeatureQueryMissingField(t *testing.T) {
	reader := newMultiReader(t)
	searcher := newSearcher(t, reader)

	q := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	topHits := mustSearch(t, searcher, q, 2)
	if topHits.TotalHits.Value != 0 {
		t.Fatalf("expected 0, got %d", topHits.TotalHits.Value)
	}
}

func TestLongDistanceFeatureQueryMissingValue(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	field := mustLongField(t, "foo", 0, false)
	doc := newTestDocument(field)

	field.SetLongValue(3)
	mustAddDocument(t, w, doc)

	mustAddDocument(t, w, document.NewDocument())

	field.SetLongValue(7)
	mustAddDocument(t, w, doc)

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	collectorManager := mustTopScoreDocCollectorManager(t, 3, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(2, float32(3*(5./(5.+3.))), -1),
			search.NewScoreDoc(0, float32(3*(5./(5.+7.))), -1),
		},
		topHits.ScoreDocs)

	testsearch.CheckExplanations(t, q, "", searcher, false)

	mustClose(t, reader, w, dir)
}

func TestLongDistanceFeatureQueryMultiValued(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())

	for _, values := range [][]int64{
		{3, 1000, math.MaxInt64},
		{-100, 12, 999},
		{math.MinInt64, -1000, 8},
		{-1},
		{math.MinInt64, 7},
	} {
		doc := document.NewDocument()
		for _, v := range values {
			doc.Add(mustLongField(t, "foo", v, false))
		}
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLongDistanceFeatureQuery(t, "foo", 3, 10, 5)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(1, float32(3*(5./(5.+2.))), -1),
			search.NewScoreDoc(2, float32(3*(5./(5.+2.))), -1),
		},
		topHits.ScoreDocs)

	q = mustLongDistanceFeatureQuery(t, "foo", 3, 7, 5)
	collectorManager = mustTopScoreDocCollectorManager(t, 2, 1)
	topHits = mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}
	testsearch.CheckExplanations(t, q, "", searcher, false)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(4, float32(3*(5./(5.+0.))), -1),
			search.NewScoreDoc(2, float32(3*(5./(5.+1.))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLongDistanceFeatureQueryRandom(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newLongDistanceWriterConfig())
	field := mustLongField(t, "foo", 0, false)
	doc := newTestDocument(field)

	numDocs := atLeast(10000)
	for i := 0; i < numDocs; i++ {
		v := int64(random().Uint64())
		field.SetLongValue(v)
		mustAddDocument(t, w, doc)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	searcher := newSearcher(t, reader)

	for iter := 0; iter < 10; iter++ {
		origin := int64(random().Uint64())
		var pivotDistance int64
		for {
			pivotDistance = int64(random().Uint64())
			if pivotDistance > 0 {
				break
			}
		}
		boost := float32(1+random().Intn(10)) / 3
		q := mustLongDistanceFeatureQuery(t, "foo", boost, origin, pivotDistance)

		testsearch.CheckTopScores(t, random(), q, searcher)
	}

	mustClose(t, reader, w, dir)
}
