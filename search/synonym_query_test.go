// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestSynonymQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

func TestSynonymQueryEquals(t *testing.T) {
	queryUtilsCheckEqual(t,
		search.NewSynonymQueryBuilder("foo").Build(), search.NewSynonymQueryBuilder("foo").Build())
	queryUtilsCheckEqual(t,
		search.NewSynonymQueryBuilder("foo").AddTerm(index.NewTerm("foo", "bar")).Build(),
		search.NewSynonymQueryBuilder("foo").AddTerm(index.NewTerm("foo", "bar")).Build())

	queryUtilsCheckEqual(t,
		search.NewSynonymQueryBuilder("a").
			AddTerm(index.NewTerm("a", "a")).
			AddTerm(index.NewTerm("a", "b")).
			Build(),
		search.NewSynonymQueryBuilder("a").
			AddTerm(index.NewTerm("a", "b")).
			AddTerm(index.NewTerm("a", "a")).
			Build())

	queryUtilsCheckEqual(t,
		search.NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "b"), 0.4).
			AddTermWithBoost(index.NewTerm("field", "c"), 0.2).
			AddTerm(index.NewTerm("field", "d")).
			Build(),
		search.NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "b"), 0.4).
			AddTermWithBoost(index.NewTerm("field", "c"), 0.2).
			AddTerm(index.NewTerm("field", "d")).
			Build())

	queryUtilsCheckUnequal(t,
		search.NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.4).Build(),
		search.NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "b"), 0.4).Build())

	queryUtilsCheckUnequal(t,
		search.NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.2).Build(),
		search.NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.4).Build())

	queryUtilsCheckUnequal(t,
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "b"), 0.4).Build(),
		search.NewSynonymQueryBuilder("field2").AddTermWithBoost(index.NewTerm("field2", "b"), 0.4).Build())
}

func TestSynonymQueryHashCode(t *testing.T) {
	q0 := search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0.4).Build()
	q1 := search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0.4).Build()
	q2 := search.NewSynonymQueryBuilder("field2").AddTermWithBoost(index.NewTerm("field2", "a"), 0.4).Build()

	if q0.HashCode() != q1.HashCode() {
		t.Fatalf("expected equal hash codes, got %d and %d", q0.HashCode(), q1.HashCode())
	}
	if q0.HashCode() == q2.HashCode() {
		t.Fatalf("expected different hash codes, got %d", q0.HashCode())
	}
}

func TestSynonymQueryGetField(t *testing.T) {
	query := search.NewSynonymQueryBuilder("field1").AddTerm(index.NewTerm("field1", "a")).Build()
	if got := query.GetField(); got != "field1" {
		t.Fatalf("expected field1, got %q", got)
	}
}

func TestSynonymQueryBogusParams(t *testing.T) {
	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").
			AddTerm(index.NewTerm("field1", "a")).
			AddTerm(index.NewTerm("field2", "b"))
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 1.3)
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.NaN()))
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.Inf(1)))
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.Inf(-1)))
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), -0.3)
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0)
	})

	expectThrowsPanic(t, func() {
		search.NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.Copysign(0, -1)))
	})

	// expectThrows(NullPointerException.class, () -> new SynonymQuery.Builder(null)...):
	// NewSynonymQueryBuilder takes a Go string, which cannot be null, so these
	// two Java cases have no Go input to render.
}

func TestSynonymQueryToString(t *testing.T) {
	if got := search.NewSynonymQueryBuilder("foo").Build().String(); got != "Synonym()" {
		t.Fatalf("expected Synonym(), got %q", got)
	}
	t1 := index.NewTerm("foo", "bar")
	if got := search.NewSynonymQueryBuilder("foo").AddTerm(t1).Build().String(); got != "Synonym(foo:bar)" {
		t.Fatalf("expected Synonym(foo:bar), got %q", got)
	}
	t2 := index.NewTerm("foo", "baz")
	if got := search.NewSynonymQueryBuilder("foo").AddTerm(t1).AddTerm(t2).Build().String(); got != "Synonym(foo:bar foo:baz)" {
		t.Fatalf("expected Synonym(foo:bar foo:baz), got %q", got)
	}
}

func TestSynonymQueryScores(t *testing.T) {
	doTestSynonymScores(t, 1)
	doTestSynonymScores(t, math.MaxInt32)
}

func doTestSynonymScores(t *testing.T, totalHitsThreshold int) {
	t.Helper()
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	doc := newTestDocument(mustStringField(t, "f", "a", false))
	mustAddDocument(t, w, doc)

	doc = newTestDocument(mustStringField(t, "f", "b", false))
	for i := 0; i < 10; i++ {
		mustAddDocument(t, w, doc)
	}
	boost := float32(1)
	if random().Intn(2) == 0 {
		boost = random().Float32()
	}
	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	b := boost
	if boost == 0 {
		b = 1
	}
	query := search.NewSynonymQueryBuilder("f").
		AddTermWithBoost(index.NewTerm("f", "a"), b).
		AddTermWithBoost(index.NewTerm("f", "b"), b).
		Build()

	collectorManager := mustTopScoreDocCollectorManager(t, min(reader.NumDocs(), totalHitsThreshold), totalHitsThreshold)
	topDocs := mustSearchWithManager(t, searcher, query, collectorManager)
	if topDocs.TotalHits.Value < int64(totalHitsThreshold) {
		if topDocs.TotalHits.Value != 11 || topDocs.TotalHits.Relation != search.EQUAL_TO {
			t.Fatalf("expected 11 EQUAL_TO, got %v", topDocs.TotalHits)
		}
	}
	// All docs must have the same score
	for i := 0; i < len(topDocs.ScoreDocs); i++ {
		if topDocs.ScoreDocs[0].Score != topDocs.ScoreDocs[i].Score {
			t.Fatalf("score %d: expected %v, got %v", i, topDocs.ScoreDocs[0].Score, topDocs.ScoreDocs[i].Score)
		}
	}

	mustClose(t, reader, w, dir)
}

func TestSynonymQueryBoosts(t *testing.T) {
	doTestSynonymBoosts(t, 1)
	doTestSynonymBoosts(t, math.MaxInt32)
}

func doTestSynonymBoosts(t *testing.T, totalHitsThreshold int) {
	t.Helper()
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	doc := document.NewDocument()
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetOmitNorms(true)
	newSynField := func(value string) *document.Field {
		f, err := document.NewField("f", value, ft)
		if err != nil {
			t.Fatalf("new Field: %v", err)
		}
		return f
	}
	doc.Add(newSynField("c"))
	mustAddDocument(t, w, doc)
	for i := 0; i < 10; i++ {
		doc.Clear()
		doc.Add(newSynField("a a a a"))
		mustAddDocument(t, w, doc)
		if i%2 == 0 {
			doc.Clear()
			doc.Add(newSynField("b b"))
			mustAddDocument(t, w, doc)
		} else {
			doc.Clear()
			doc.Add(newSynField("a a b"))
			mustAddDocument(t, w, doc)
		}
	}
	doc.Clear()
	doc.Add(newSynField("c"))
	mustAddDocument(t, w, doc)
	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	query := search.NewSynonymQueryBuilder("f").
		AddTermWithBoost(index.NewTerm("f", "a"), 0.25).
		AddTermWithBoost(index.NewTerm("f", "b"), 0.5).
		AddTerm(index.NewTerm("f", "c")).
		Build()

	collectorManager := mustTopScoreDocCollectorManager(t, min(reader.NumDocs(), totalHitsThreshold), totalHitsThreshold)
	topDocs := mustSearchWithManager(t, searcher, query, collectorManager)
	if topDocs.TotalHits.Value < int64(totalHitsThreshold) {
		if topDocs.TotalHits.Relation != search.EQUAL_TO {
			t.Fatalf("expected EQUAL_TO, got %v", topDocs.TotalHits.Relation)
		}
		if topDocs.TotalHits.Value != 22 {
			t.Fatalf("expected 22, got %d", topDocs.TotalHits.Value)
		}
	} else if topDocs.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Fatalf("expected GREATER_THAN_OR_EQUAL_TO, got %v", topDocs.TotalHits.Relation)
	}
	// All docs must have the same score
	for i := 0; i < len(topDocs.ScoreDocs); i++ {
		if topDocs.ScoreDocs[0].Score != topDocs.ScoreDocs[i].Score {
			t.Fatalf("score %d: expected %v, got %v", i, topDocs.ScoreDocs[0].Score, topDocs.ScoreDocs[i].Score)
		}
	}

	mustClose(t, reader, w, dir)
}

func TestSynonymQueryMergeImpacts(t *testing.T) {
	t.Fatal("requires org.apache.lucene.search.SynonymQuery.mergeImpacts(ImpactsEnum[], float[]) (not ported)")
}

func TestSynonymQueryRandomTopDocs(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	numDocs := atLeast(100) // TEST_NIGHTLY is false; at night, make sure some terms have skip data
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numValues := random().Intn(1 << random().Intn(5))
		start := random().Intn(10)
		for j := 0; j < numValues; j++ {
			freq := nextInt(1, 1<<random().Intn(3))
			for k := 0; k < freq; k++ {
				tf, err := document.NewTextField("foo", strconv.Itoa(start+j), false)
				if err != nil {
					t.Fatalf("new TextField: %v", err)
				}
				doc.Add(tf)
			}
		}
		mustAddDocument(t, w, doc)
	}
	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	mustClose(t, w)
	searcher := newSearcher(t, reader)

	for term1 := 0; term1 < 15; term1++ {
		var term2 int
		for {
			term2 = random().Intn(15)
			if term1 != term2 {
				break
			}
		}
		boost1 := float32(1)
		if random().Intn(2) == 0 {
			boost1 = max(random().Float32(), minNormalFloat32)
		}
		boost2 := float32(1)
		if random().Intn(2) == 0 {
			boost2 = max(random().Float32(), minNormalFloat32)
		}
		query := search.NewSynonymQueryBuilder("foo").
			AddTermWithBoost(index.NewTerm("foo", strconv.Itoa(term1)), boost1).
			AddTermWithBoost(index.NewTerm("foo", strconv.Itoa(term2)), boost2).
			Build()

		completeManager := mustTopScoreDocCollectorManager(t, 10, math.MaxInt32) // COMPLETE
		topScoresManager := mustTopScoreDocCollectorManager(t, 10, 1)            // TOP_SCORES

		complete := mustSearchWithManager(t, searcher, query, completeManager)
		topScores := mustSearchWithManager(t, searcher, query, topScoresManager)
		testsearch.CheckEqual(t, query, complete.ScoreDocs, topScores.ScoreDocs)

		filterTerm := random().Intn(15)
		filteredQuery := search.NewBooleanQueryBuilder().
			Add(query, search.MUST).
			Add(search.NewTermQuery(index.NewTerm("foo", strconv.Itoa(filterTerm))), search.FILTER).
			Build()

		completeManager = mustTopScoreDocCollectorManager(t, 10, math.MaxInt32) // COMPLETE
		topScoresManager = mustTopScoreDocCollectorManager(t, 10, 1)            // TOP_SCORES

		complete = mustSearchWithManager(t, searcher, filteredQuery, completeManager)
		topScores = mustSearchWithManager(t, searcher, filteredQuery, topScoresManager)
		testsearch.CheckEqual(t, query, complete.ScoreDocs, topScores.ScoreDocs)
	}
	mustClose(t, reader, dir)
}

// minNormalFloat32 renders Float.MIN_NORMAL (0x1.0p-126f).
const minNormalFloat32 = float32(1.1754943508222875e-38)

func TestSynonymQueryRewrite(t *testing.T) {
	searcher := search.NewIndexSearcher(newMultiReader(t))

	// zero length SynonymQuery is rewritten
	q := search.NewSynonymQueryBuilder("f").Build()
	if len(q.GetTerms()) != 0 {
		t.Fatalf("expected no terms, got %d", len(q.GetTerms()))
	}
	if got := mustRewrite(t, searcher, q); !got.Equals(search.MatchNoDocsQueryInstance) {
		t.Fatalf("expected MatchNoDocsQuery.INSTANCE, got %v", got)
	}

	// non-boosted single term SynonymQuery is rewritten
	q = search.NewSynonymQueryBuilder("f").AddTermWithBoost(index.NewTerm("f", ""), 1).Build()
	if len(q.GetTerms()) != 1 {
		t.Fatalf("expected 1 term, got %d", len(q.GetTerms()))
	}
	if got := mustRewrite(t, searcher, q); !got.Equals(search.NewTermQuery(index.NewTerm("f", ""))) {
		t.Fatalf("expected TermQuery f:, got %v", got)
	}

	// boosted single term SynonymQuery is not rewritten
	q = search.NewSynonymQueryBuilder("f").AddTermWithBoost(index.NewTerm("f", ""), 0.8).Build()
	if len(q.GetTerms()) != 1 {
		t.Fatalf("expected 1 term, got %d", len(q.GetTerms()))
	}
	if got := mustRewrite(t, searcher, q); !got.Equals(q) {
		t.Fatalf("expected %v, got %v", q, got)
	}

	// multiple term SynonymQuery is not rewritten
	q = search.NewSynonymQueryBuilder("f").AddTermWithBoost(index.NewTerm("f", ""), 1).AddTermWithBoost(index.NewTerm("f", ""), 1).Build()
	if len(q.GetTerms()) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(q.GetTerms()))
	}
	if got := mustRewrite(t, searcher, q); !got.Equals(q) {
		t.Fatalf("expected %v, got %v", q, got)
	}
}
