// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseKnnVectorQueryTestCase.java
//
// BaseKnnVectorQueryTestCase is an abstract JUnit class: its abstract methods
// are rendered as the function fields of knnVectorQueryTestCase, each concrete
// subclass supplies them in its own file, and every inherited test method is a
// method of knnVectorQueryTestCase that the subclass's Test<Class><Method>
// functions call.

import (
	"errors"
	"math"
	"math/rand"
	"regexp"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// knnEpsilon renders BaseKnnVectorQueryTestCase.EPSILON.
const knnEpsilon = 0.001

// timeLimitingKnnCollectorManagerBlocker names the class
// testTimeLimitingKnnCollectorManager exercises: Gocene's exported
// TimeLimitingKnnCollectorManager is a deadline-based type with no Lucene
// counterpart, and the faithful TimeLimitingKnnCollectorManager(
// KnnCollectorManager, QueryTimeout) is not exported.
const timeLimitingKnnCollectorManagerBlocker = "requires org.apache.lucene.search.TimeLimitingKnnCollectorManager(" +
	"KnnCollectorManager, QueryTimeout) with newCollector(int, KnnSearchStrategy, LeafReaderContext) (not ported)"

// alwaysKnnVectorsFormatBlocker names the test-framework members
// testSameFieldDifferentFormats needs.
const alwaysKnnVectorsFormatBlocker = "requires TestUtil.alwaysKnnVectorsFormat(KnnVectorsFormat) and " +
	"LuceneTestCase.randomVectorFormat(VectorEncoding) (not ported)"

// bitSetIteratorGetBitSetOverrideBlocker names what ThrowingBitSetQuery
// needs: an anonymous BitSetIterator subclass overriding getBitSet(), which
// AcceptDocs.createBitSet must dispatch to through `instanceof BitSetIterator`.
// Gocene's util.BitSetIterator is a concrete struct whose GetBitSet cannot be
// overridden, and AcceptDocs matches only *util.BitSetIterator.
const bitSetIteratorGetBitSetOverrideBlocker = "requires an overridable org.apache.lucene.util.BitSetIterator.getBitSet() " +
	"(anonymous subclass dispatched by AcceptDocs.createBitSet) (not ported)"

// errExactSearchNotSupported renders the UnsupportedOperationException("exact
// search is not supported") of the ThrowingKnnVectorQuery subclasses.
var errExactSearchNotSupported = errors.New("exact search is not supported")

// abstractKnnVectorQuery renders the members of AbstractKnnVectorQuery the
// tests call.
type abstractKnnVectorQuery interface {
	search.Query
	GetField() string
	GetK() int
	GetFilter() search.Query
	GetSearchStrategy() knn.KnnSearchStrategy
}

// knnVectorQueryTestCase renders BaseKnnVectorQueryTestCase. The function
// fields are its abstract methods (and the overridable newDirectoryForTest).
type knnVectorQueryTestCase struct {
	t *testing.T

	// getKnnVectorQueryWithFilter renders the abstract
	// getKnnVectorQuery(String, float[], int, Query).
	getKnnVectorQueryWithFilter func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery
	// getThrowingKnnVectorQuery renders the abstract
	// getThrowingKnnVectorQuery(String, float[], int, Query).
	getThrowingKnnVectorQuery func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery
	// getCappedResultsThrowingKnnVectorQuery renders the abstract
	// getCappedResultsThrowingKnnVectorQuery(String, float[], int, Query, int).
	getCappedResultsThrowingKnnVectorQuery func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery
	// randomVector renders the abstract randomVector(int).
	randomVector func(dim int) []float32
	// getKnnVectorFieldWithSimilarity renders the abstract
	// getKnnVectorField(String, float[], VectorSimilarityFunction).
	getKnnVectorFieldWithSimilarity func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField
	// getKnnVectorField renders the abstract getKnnVectorField(String, float[]).
	getKnnVectorField func(name string, vector []float32) document.IndexableField
	// newDirectoryForTest renders the overridable newDirectoryForTest(); nil
	// selects the base implementation, LuceneTestCase.newDirectory(random()).
	newDirectoryForTestOverride func() store.Directory
}

// getKnnVectorQuery renders getKnnVectorQuery(String, float[], int).
func (c *knnVectorQueryTestCase) getKnnVectorQuery(field string, query []float32, k int) abstractKnnVectorQuery {
	return c.getKnnVectorQueryWithFilter(field, query, k, nil)
}

// newDirectoryForTest renders BaseKnnVectorQueryTestCase.newDirectoryForTest().
func (c *knnVectorQueryTestCase) newDirectoryForTest() store.Directory {
	if c.newDirectoryForTestOverride != nil {
		return c.newDirectoryForTestOverride()
	}
	return newDirectory()
}

// randomBoolean renders RandomizedTest.randomBoolean().
func randomBoolean() bool {
	return random().Intn(2) == 0
}

// randomIntBetween renders RandomizedTest.randomIntBetween(int, int).
func randomIntBetween(min, max int) int {
	return min + random().Intn(max-min+1)
}

// randomizedFrequently renders RandomizedTest.frequently(): !rarely(), where
// RandomizedTest.rarely() is randomInt(100) >= 90 over [0, 100].
func randomizedFrequently() bool {
	return random().Intn(101) < 90
}

// knnQueryToString renders Query.toString(String) on a KNN query.
func knnQueryToString(t *testing.T, q search.Query, field string) string {
	t.Helper()
	ts, ok := q.(interface{ ToString(string) string })
	if !ok {
		t.Fatalf("%T has no toString(String)", q)
	}
	return ts.ToString(field)
}

func (c *knnVectorQueryTestCase) testEquals() {
	t := c.t
	q1 := c.getKnnVectorQuery("f1", []float32{0, 1}, 10)
	filter1 := search.NewTermQuery(index.NewTerm("id", "id1"))
	q2 := c.getKnnVectorQueryWithFilter("f1", []float32{0, 1}, 10, filter1)

	if q2.Equals(q1) {
		t.Fatal("assertNotEquals(q2, q1)")
	}
	if q1.Equals(q2) {
		t.Fatal("assertNotEquals(q1, q2)")
	}
	if !q2.Equals(c.getKnnVectorQueryWithFilter("f1", []float32{0, 1}, 10, filter1)) {
		t.Fatal("assertEquals(q2, getKnnVectorQuery(f1, {0,1}, 10, filter1))")
	}

	filter2 := search.NewTermQuery(index.NewTerm("id", "id2"))
	if q2.Equals(c.getKnnVectorQueryWithFilter("f1", []float32{0, 1}, 10, filter2)) {
		t.Fatal("assertNotEquals(q2, getKnnVectorQuery(f1, {0,1}, 10, filter2))")
	}

	if !q1.Equals(c.getKnnVectorQuery("f1", []float32{0, 1}, 10)) {
		t.Fatal("assertEquals(q1, getKnnVectorQuery(f1, {0,1}, 10))")
	}

	if q1.Equals(nil) {
		t.Fatal("assertNotEquals(null, q1)")
	}

	if q1.Equals(search.NewTermQuery(index.NewTerm("f1", "x"))) {
		t.Fatal("assertNotEquals(q1, TermQuery)")
	}

	if q1.Equals(c.getKnnVectorQuery("f2", []float32{0, 1}, 10)) {
		t.Fatal("assertNotEquals(q1, field f2)")
	}
	if q1.Equals(c.getKnnVectorQuery("f1", []float32{1, 1}, 10)) {
		t.Fatal("assertNotEquals(q1, target {1,1})")
	}
	if q1.Equals(c.getKnnVectorQuery("f1", []float32{0, 1}, 2)) {
		t.Fatal("assertNotEquals(q1, k 2)")
	}
	if q1.Equals(c.getKnnVectorQuery("f1", []float32{0}, 10)) {
		t.Fatal("assertNotEquals(q1, target {0})")
	}
}

func (c *knnVectorQueryTestCase) testGetField() {
	t := c.t
	q1 := c.getKnnVectorQuery("f1", []float32{0, 1}, 10)
	filter1 := search.NewTermQuery(index.NewTerm("id", "id1"))
	q2 := c.getKnnVectorQueryWithFilter("f2", []float32{0, 1}, 10, filter1)

	if got := q1.GetField(); got != "f1" {
		t.Fatalf("q1.getField() = %q, want f1", got)
	}
	if got := q2.GetField(); got != "f2" {
		t.Fatalf("q2.getField() = %q, want f2", got)
	}
}

func (c *knnVectorQueryTestCase) testGetK() {
	t := c.t
	q1 := c.getKnnVectorQuery("f1", []float32{0, 1}, 6)
	filter1 := search.NewTermQuery(index.NewTerm("id", "id1"))
	q2 := c.getKnnVectorQueryWithFilter("f2", []float32{0, 1}, 7, filter1)

	if got := q1.GetK(); got != 6 {
		t.Fatalf("q1.getK() = %d, want 6", got)
	}
	if got := q2.GetK(); got != 7 {
		t.Fatalf("q2.getK() = %d, want 7", got)
	}
}

func (c *knnVectorQueryTestCase) testGetFilter() {
	t := c.t
	q1 := c.getKnnVectorQuery("f1", []float32{0, 1}, 6)
	filter1 := search.NewTermQuery(index.NewTerm("id", "id1"))
	q2 := c.getKnnVectorQueryWithFilter("f2", []float32{0, 1}, 7, filter1)

	if q1.GetFilter() != nil {
		t.Fatalf("q1.getFilter() = %v, want null", q1.GetFilter())
	}
	if f := q2.GetFilter(); f == nil || !filter1.Equals(f) {
		t.Fatalf("q2.getFilter() = %v, want %v", f, filter1)
	}
}

// testEmptyIndex tests if a AbstractKnnVectorQuery is rewritten to a
// MatchNoDocsQuery when there are no documents to match.
func (c *knnVectorQueryTestCase) testEmptyIndex() {
	t := c.t
	indexStore := c.getIndexStore("field")
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getKnnVectorQuery("field", []float32{1, 2}, 10)
	assertKnnMatches(t, searcher, kvq, 0)
	q := mustRewrite(t, searcher, kvq)
	if _, ok := q.(*search.MatchNoDocsQuery); !ok {
		t.Fatalf("rewrite = %T, want MatchNoDocsQuery", q)
	}
}

// testFindAll tests that a AbstractKnnVectorQuery whose topK >= numDocs
// returns all the documents in score order.
func (c *knnVectorQueryTestCase) testFindAll() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getKnnVectorQuery("field", []float32{0, 0}, 10)
	assertKnnMatches(t, searcher, kvq, 3)
	scoreDocs := mustSearch(t, searcher, kvq, 3).ScoreDocs
	assertIdMatches(t, reader, "id2", scoreDocs[0])
	assertIdMatches(t, reader, "id0", scoreDocs[1])
	assertIdMatches(t, reader, "id1", scoreDocs[2])
}

func (c *knnVectorQueryTestCase) testFindFewer() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getKnnVectorQuery("field", []float32{0, 0}, 2)
	assertKnnMatches(t, searcher, kvq, 2)
	scoreDocs := mustSearch(t, searcher, kvq, 3).ScoreDocs
	if len(scoreDocs) != 2 {
		t.Fatalf("scoreDocs.length = %d, want 2", len(scoreDocs))
	}
	assertTopIdsMatches(t, reader, map[string]bool{"id2": true, "id0": true}, scoreDocs)
}

func (c *knnVectorQueryTestCase) testSearchBoost() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	vectorQuery := c.getKnnVectorQuery("field", []float32{0, 0}, 10)
	scoreDocs := mustSearch(t, searcher, vectorQuery, 3).ScoreDocs

	boostQuery := search.NewBoostQuery(vectorQuery, 3.0)
	boostScoreDocs := mustSearch(t, searcher, boostQuery, 3).ScoreDocs
	if len(scoreDocs) != len(boostScoreDocs) {
		t.Fatalf("boosted length = %d, want %d", len(boostScoreDocs), len(scoreDocs))
	}

	for i := range scoreDocs {
		scoreDoc := scoreDocs[i]
		boostScoreDoc := boostScoreDocs[i]

		if scoreDoc.Doc != boostScoreDoc.Doc {
			t.Fatalf("doc[%d] = %d, want %d", i, boostScoreDoc.Doc, scoreDoc.Doc)
		}
		if math.Abs(float64(scoreDoc.Score*3.0-boostScoreDoc.Score)) > 0.001 {
			t.Fatalf("score[%d] = %v, want %v", i, boostScoreDoc.Score, scoreDoc.Score*3.0)
		}
	}
}

// testSimpleFilter tests that a AbstractKnnVectorQuery applies the filter
// query.
func (c *knnVectorQueryTestCase) testSimpleFilter() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	filter := search.NewTermQuery(index.NewTerm("id", "id2"))
	kvq := c.getKnnVectorQueryWithFilter("field", []float32{0, 0}, 10, filter)
	topDocs := mustSearch(t, searcher, kvq, 3)
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", topDocs.TotalHits.Value)
	}
	assertIdMatches(t, reader, "id2", topDocs.ScoreDocs[0])
}

func (c *knnVectorQueryTestCase) testFilterWithNoVectorMatches() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	filter := search.NewTermQuery(index.NewTerm("other", "value"))
	kvq := c.getKnnVectorQueryWithFilter("field", []float32{0, 0}, 10, filter)
	topDocs := mustSearch(t, searcher, kvq, 3)
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("totalHits = %d, want 0", topDocs.TotalHits.Value)
	}
}

func (c *knnVectorQueryTestCase) testMatchAllFilter() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	// make sure we don't drop to exact search, even though the filter matches
	// fewer than k docs
	kvq := c.getThrowingKnnVectorQuery("field", []float32{0, 0}, 10, search.NewMatchAllDocsQuery())
	topDocs := mustSearch(t, searcher, kvq, 3)
	if topDocs.TotalHits.Value != 3 {
		t.Fatalf("totalHits = %d, want 3", topDocs.TotalHits.Value)
	}
}

func (c *knnVectorQueryTestCase) testDimensionMismatch() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getKnnVectorQuery("field", []float32{0}, 1)
	_, err := searcher.Search(kvq, 10)
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), "vector query dimension: 1 differs from field dimension: 2"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func (c *knnVectorQueryTestCase) testNonVectorField() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	assertKnnMatches(t, searcher, c.getKnnVectorQuery("xyzzy", []float32{0}, 10), 0)
	assertKnnMatches(t, searcher, c.getKnnVectorQuery("id", []float32{0}, 10), 0)
}

// testIllegalArguments tests bad parameters.
func (c *knnVectorQueryTestCase) testIllegalArguments() {
	expectThrowsPanic(c.t, func() { c.getKnnVectorQuery("xx", []float32{1}, 0) })
}

func (c *knnVectorQueryTestCase) testDifferentReader() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	query := c.getKnnVectorQuery("field", []float32{2, 3}, 3)
	dasq, err := query.Rewrite(newSearcher(t, reader))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	leafSearcher := newSearcher(t, mustLeaves(t, reader)[0].LeafReader())
	if _, err := dasq.CreateWeight(leafSearcher, search.COMPLETE, 1); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

func (c *knnVectorQueryTestCase) testScoreEuclidean() {
	t := c.t
	vectors := make([][]float32, 5)
	for j := 0; j < 5; j++ {
		vectors[j] = []float32{float32(j), float32(j)}
	}
	d := c.getStableIndexStore("field", vectors...)
	defer mustClose(t, d)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", []float32{2, 3}, 3)
	rewritten, err := query.Rewrite(searcher)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	weight := mustCreateWeight(t, searcher, rewritten, search.COMPLETE, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, reader)[0])

	// prior to advancing, score is 0
	if got := scorer.DocID(); got != -1 {
		t.Fatalf("docID = %d, want -1", got)
	}
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })

	// This is 1 / ((l2distance((2,3), (2, 2)) = 1) + 1) = 0.5
	assertMaxScore(t, scorer, 2, 1/2.0, 0)
	assertMaxScore(t, scorer, math.MaxInt32, 1/2.0, 0)

	it := scorer.Iterator()
	if got := it.Cost(); got != 3 {
		t.Fatalf("cost = %d, want 3", got)
	}
	firstDoc := mustNextDoc(t, it)
	if firstDoc == 1 {
		assertScorerScore(t, scorer, 1/6.0, 0)
		assertAdvance(t, it, 3, 3)
		assertScorerScore(t, scorer, 1/2.0, 0)
		assertAdvance(t, it, 4, search.NO_MORE_DOCS)
	} else {
		if firstDoc != 2 {
			t.Fatalf("firstDoc = %d, want 2", firstDoc)
		}
		assertScorerScore(t, scorer, 1/2.0, 0)
		assertAdvance(t, it, 4, 4)
		assertScorerScore(t, scorer, 1/6.0, 0)
		assertAdvance(t, it, 5, search.NO_MORE_DOCS)
	}
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })
}

func (c *knnVectorQueryTestCase) testScoreCosine() {
	t := c.t
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	for j := 1; j <= 5; j++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorFieldWithSimilarity("field", []float32{float32(j), float32(j * j)}, index.VectorSimilarityFunctionCosine))
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	if n := len(mustLeaves(t, reader)); n != 1 {
		t.Fatalf("leaves = %d, want 1", n)
	}
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", []float32{2, 3}, 3)
	rewritten, err := query.Rewrite(searcher)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	weight := mustCreateWeight(t, searcher, rewritten, search.COMPLETE, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, reader)[0])

	// prior to advancing, score is undefined
	if got := scorer.DocID(); got != -1 {
		t.Fatalf("docID = %d, want -1", got)
	}
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })

	// score0 = ((2,3) * (1, 1) = 5) / (||2, 3|| * ||1, 1|| = sqrt(26)), then
	// normalized by (1 + x) /2.
	score0 := float32((1 + (2*1+3*1)/math.Sqrt((2*2+3*3)*(1*1+1*1))) / 2)

	// score1 = ((2,3) * (2, 4) = 16) / (||2, 3|| * ||2, 4|| = sqrt(260)), then
	// normalized by (1 + x) /2
	score1 := float32((1 + (2*2+3*4)/math.Sqrt((2*2+3*3)*(2*2+4*4))) / 2)

	// doc 1 happens to have the maximum score
	assertMaxScore(t, scorer, 2, float64(score1), 0.0001)
	assertMaxScore(t, scorer, math.MaxInt32, float64(score1), 0.0001)

	it := scorer.Iterator()
	if got := it.Cost(); got != 3 {
		t.Fatalf("cost = %d, want 3", got)
	}
	if got := mustNextDoc(t, it); got != 0 {
		t.Fatalf("nextDoc = %d, want 0", got)
	}
	// doc 0 has (1, 1)
	assertScorerScore(t, scorer, float64(score0), 0.0001)
	assertAdvance(t, it, 1, 1)
	assertScorerScore(t, scorer, float64(score1), 0.0001)

	// since topK was 3
	assertAdvance(t, it, 4, search.NO_MORE_DOCS)
	expectThrowsPanic(t, func() { _, _ = scorer.Score() })
}

func (c *knnVectorQueryTestCase) testScoreMIP() {
	t := c.t
	indexStore := c.getIndexStoreWithSimilarity("field", index.VectorSimilarityFunctionMaximumInnerProduct,
		[]float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getKnnVectorQuery("field", []float32{0, -1}, 10)
	assertKnnMatches(t, searcher, kvq, 3)
	scoreDocs := mustSearch(t, searcher, kvq, 3).ScoreDocs
	assertIdMatches(t, reader, "id2", scoreDocs[0])
	assertIdMatches(t, reader, "id0", scoreDocs[1])
	assertIdMatches(t, reader, "id1", scoreDocs[2])

	assertFloatEquals(t, "score[0]", 1.0, float64(scoreDocs[0].Score), 1e-7)
	assertFloatEquals(t, "score[1]", float64(float32(1)/2), float64(scoreDocs[1].Score), 1e-7)
	assertFloatEquals(t, "score[2]", float64(float32(1)/3), float64(scoreDocs[2].Score), 1e-7)
}

func (c *knnVectorQueryTestCase) testExplain() {
	t := c.t
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	for j := 0; j < 5; j++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorField("field", []float32{float32(j), float32(j)}))
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", []float32{2, 3}, 3)
	matched := mustExplain(t, searcher, query, 2)
	assertKnnExplanation(t, matched, true, 1/2.0, "within top 3 docs")

	nomatch := mustExplain(t, searcher, query, 5)
	assertKnnExplanation(t, nomatch, false, 0, "not in top 3 docs")
	if n := len(matched.GetDetails()); n != 0 {
		t.Fatalf("matched details = %d, want 0", n)
	}
}

func (c *knnVectorQueryTestCase) testExplainMultipleSegments() {
	t := c.t
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	for j := 0; j < 5; j++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorField("field", []float32{float32(j), float32(j)}))
		mustAddDocument(t, w, doc)
		mustCommit(t, w)
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", []float32{2, 3}, 3)
	matched := mustExplain(t, searcher, query, 2)
	assertKnnExplanation(t, matched, true, 1/2.0, "within top 3 docs")

	nomatch := mustExplain(t, searcher, query, 4)
	assertKnnExplanation(t, nomatch, false, 0, "not in top 3 docs")
	if n := len(matched.GetDetails()); n != 0 {
		t.Fatalf("matched details = %d, want 0", n)
	}
}

// testSkewedIndex tests that when vectors are abnormally distributed among
// segments, we still find the top K.
func (c *knnVectorQueryTestCase) testSkewedIndex() {
	t := c.t
	// We have to choose the numbers carefully here so that some segment has
	// more than the expected number of top K documents, but no more than K
	// documents in total (otherwise we might occasionally randomly fail to
	// find one).
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	r := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			doc := document.NewDocument()
			doc.Add(c.getKnnVectorField("field", []float32{float32(r), float32(r)}))
			doc.Add(newStringField(t, "id", "id"+itoa(r), true))
			mustAddDocument(t, w, doc)
			r++
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	results := mustSearch(t, searcher, c.getKnnVectorQuery("field", []float32{0, 0}, 8), 10)
	if len(results.ScoreDocs) != 8 {
		t.Fatalf("scoreDocs.length = %d, want 8", len(results.ScoreDocs))
	}
	assertIdMatches(t, reader, "id0", results.ScoreDocs[0])
	assertIdMatches(t, reader, "id7", results.ScoreDocs[7])

	// test some results in the middle of the sequence - also tests docid
	// tiebreaking
	results = mustSearch(t, searcher, c.getKnnVectorQuery("field", []float32{10, 10}, 8), 10)
	if len(results.ScoreDocs) != 8 {
		t.Fatalf("scoreDocs.length = %d, want 8", len(results.ScoreDocs))
	}
	assertIdMatches(t, reader, "id10", results.ScoreDocs[0])
	assertIdMatches(t, reader, "id6", results.ScoreDocs[7])
}

// testRandomConsistencySingleThreaded tests with random vectors, number of
// documents, etc.
func (c *knnVectorQueryTestCase) testRandomConsistencySingleThreaded() {
	c.assertRandomConsistency(false)
}

// @AwaitsFix(bugUrl = "https://github.com/apache/lucene/issues/14180") is
// commented out upstream, so the test runs.
func (c *knnVectorQueryTestCase) testRandomConsistencyMultiThreaded() {
	c.assertRandomConsistency(true)
}

func (c *knnVectorQueryTestCase) assertRandomConsistency(multiThreaded bool) {
	t := c.t
	numDocs := 100
	dimension := 4
	numIters := 10
	everyDocHasAVector := randomBoolean()
	r := random()
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	// To ensure consistency between seeded runs, remove some randomness
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergeScheduler(index.NewSerialMergeScheduler())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	iwc.SetMaxBufferedDocs(numDocs)
	iwc.SetRAMBufferSizeMB(index.DISABLE_AUTO_FLUSH)
	w := mustNewIndexWriter(t, d, iwc)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if everyDocHasAVector || random().Intn(10) != 2 {
			doc.Add(c.getKnnVectorField("field", c.randomVector(dimension)))
		}
		mustAddDocument(t, w, doc)
		if r.Intn(2) == 0 && i%50 == 0 {
			if err := w.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
		}
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcherWithOptions(t, reader, true, true, multiThreaded)
	// first get the initial set of docs, and we expect all future queries to
	// be exactly the same
	k := random().Intn(80) + 1
	query := c.getKnnVectorQuery("field", c.randomVector(dimension), k)
	n := random().Intn(100) + 1
	expectedResults := mustSearch(t, searcher, query, n)
	for i := 0; i < numIters; i++ {
		results := mustSearch(t, searcher, query, n)
		if expectedResults.TotalHits.Value != results.TotalHits.Value {
			t.Fatalf("totalHits = %d, want %d", results.TotalHits.Value, expectedResults.TotalHits.Value)
		}
		if len(expectedResults.ScoreDocs) != len(results.ScoreDocs) {
			t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), len(expectedResults.ScoreDocs))
		}
		for j := range results.ScoreDocs {
			if expectedResults.ScoreDocs[j].Doc != results.ScoreDocs[j].Doc {
				t.Fatalf("doc[%d] = %d, want %d", j, results.ScoreDocs[j].Doc, expectedResults.ScoreDocs[j].Doc)
			}
			assertFloatEquals(t, "score", float64(expectedResults.ScoreDocs[j].Score), float64(results.ScoreDocs[j].Score), knnEpsilon)
		}
	}
}

// testRandom tests with random vectors, number of documents, etc. Uses
// RandomIndexWriter.
func (c *knnVectorQueryTestCase) testRandom() {
	t := c.t
	numDocs := atLeast(100)
	dimension := atLeast(5)
	numIters := atLeast(10)
	everyDocHasAVector := randomBoolean()
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	w := newRandomIndexWriter(t, d)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if everyDocHasAVector || random().Intn(10) != 2 {
			doc.Add(c.getKnnVectorField("field", c.randomVector(dimension)))
		}
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	for i := 0; i < numIters; i++ {
		k := random().Intn(80) + 1
		query := c.getKnnVectorQuery("field", c.randomVector(dimension), k)
		n := random().Intn(100) + 1
		results := mustSearch(t, searcher, query, n)
		expected := min(min(n, k), reader.NumDocs())
		// we may get fewer results than requested if there are deletions, but
		// this test doesn't test that
		if util.AssertsEnabled() && reader.HasDeletions() {
			panic(util.NewAssertionError("reader.hasDeletions() == false"))
		}
		if len(results.ScoreDocs) != expected {
			t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), expected)
		}
		if results.TotalHits.Value < int64(len(results.ScoreDocs)) {
			t.Fatalf("totalHits %d < scoreDocs.length %d", results.TotalHits.Value, len(results.ScoreDocs))
		}
		// verify the results are in descending score order
		assertDescendingScores(t, results.ScoreDocs)
	}
}

// testRandomWithFilter tests with random vectors and a random filter. Uses
// RandomIndexWriter.
func (c *knnVectorQueryTestCase) testRandomWithFilter() {
	t := c.t
	numDocs := 1000
	dimension := atLeast(5)
	numIters := atLeast(10)
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	// Always use the default kNN format to have predictable behavior around
	// when it hits visitedLimit. This is fine since the test targets
	// AbstractKnnVectorQuery logic, not the kNN format implementation.
	iwc := index.NewIndexWriterConfig()
	iwc.SetCodec(index.GetDefaultCodec())
	w := newRandomIndexWriterWithConfig(t, d, iwc)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorField("field", c.randomVector(dimension)))
		doc.Add(mustNumericDocValuesField(t, "tag", int64(i)))
		doc.Add(document.NewIntPoint("tag", int32(i)))
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w)

	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	for i := 0; i < numIters; i++ {
		lower := int32(random().Intn(500))

		// Test a filter with cost less than k and check we use exact search
		filter1 := intPointNewRangeQuery(t, "tag", lower, lower+8)
		results := mustSearch(t, searcher, c.getKnnVectorQueryWithFilter("field", c.randomVector(dimension), 10, filter1), numDocs)
		if results.TotalHits.Value != 9 {
			t.Fatalf("totalHits = %d, want 9", results.TotalHits.Value)
		}
		if results.TotalHits.Value != int64(len(results.ScoreDocs)) {
			t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), results.TotalHits.Value)
		}
		expectExactSearchUnsupported(t, searcher, c.getThrowingKnnVectorQuery("field", c.randomVector(dimension), 10, filter1), numDocs)

		// Test an unrestrictive filter and check we use approximate search
		filter3 := intPointNewRangeQuery(t, "tag", lower, int32(numDocs))
		sorted, err := searcher.SearchWithSortNoScores(
			c.getThrowingKnnVectorQuery("field", c.randomVector(dimension), 5, filter3),
			numDocs,
			search.NewSort(search.NewSortField("tag", spi.SortFieldTypeInt)))
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if sorted.TotalHits.Value != 5 {
			t.Fatalf("totalHits = %d, want 5", sorted.TotalHits.Value)
		}
		if sorted.TotalHits.Value != int64(len(sorted.ScoreDocs)) {
			t.Fatalf("scoreDocs.length = %d, want %d", len(sorted.ScoreDocs), sorted.TotalHits.Value)
		}

		for _, fieldDoc := range sorted.FieldDocs {
			if len(fieldDoc.Fields) != 1 {
				t.Fatalf("fieldDoc.fields.length = %d, want 1", len(fieldDoc.Fields))
			}

			tag := fieldDoc.Fields[0].(int32)
			if !(lower <= tag && tag <= int32(numDocs)) {
				t.Fatalf("tag %d outside [%d, %d]", tag, lower, numDocs)
			}
		}
		// Test a filter with cost slightly more than k, and check we use exact
		// search as k results are not retrieved from approximate search
		filter5 := intPointNewRangeQuery(t, "tag", lower, lower+11)
		results = mustSearch(t, searcher, c.getKnnVectorQueryWithFilter("field", c.randomVector(dimension), 10, filter5), numDocs)
		if results.TotalHits.Value != 10 {
			t.Fatalf("totalHits = %d, want 10", results.TotalHits.Value)
		}
		if results.TotalHits.Value != int64(len(results.ScoreDocs)) {
			t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), results.TotalHits.Value)
		}
		expectExactSearchUnsupported(t, searcher,
			c.getCappedResultsThrowingKnnVectorQuery("field", c.randomVector(dimension), 10, filter5, 5), numDocs)
		if results.TotalHits.Value != 10 {
			t.Fatalf("totalHits = %d, want 10", results.TotalHits.Value)
		}
		if results.TotalHits.Value != int64(len(results.ScoreDocs)) {
			t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), results.TotalHits.Value)
		}
	}
	// Test a filter that exhausts visitedLimit in upper levels, and switches
	// to exact search due to extreme edge cases, removing the randomness
	vector := make([]float32, dimension)
	for i := 0; i < dimension; i++ {
		if i%2 == 0 {
			vector[i] = 42
		} else {
			vector[i] = 7
		}
	}
	filter4 := intPointNewRangeQuery(t, "tag", 250, 256)
	expectExactSearchUnsupported(t, searcher, c.getThrowingKnnVectorQuery("field", vector, 1, filter4), numDocs)
}

// testFilterWithSameScore tests filtering when all vectors have the same
// score.
func (c *knnVectorQueryTestCase) testFilterWithSameScore() {
	t := c.t
	numDocs := 100
	dimension := atLeast(5)
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	// Always use the default kNN format to have predictable behavior around
	// when it hits visitedLimit. This is fine since the test targets
	// AbstractKnnVectorQuery logic, not the kNN format implementation.
	iwc := index.NewIndexWriterConfig()
	iwc.SetCodec(index.GetDefaultCodec())
	w := mustNewIndexWriter(t, d, iwc)
	vector := c.randomVector(dimension)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorField("field", vector))
		doc.Add(document.NewIntPoint("tag", int32(i)))
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w)

	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	lower := int32(random().Intn(50))
	size := 5

	// Test a restrictive filter, which usually performs exact search
	filter1 := intPointNewRangeQuery(t, "tag", lower, lower+6)
	results := mustSearch(t, searcher, c.getKnnVectorQueryWithFilter("field", c.randomVector(dimension), size, filter1), size)
	if len(results.ScoreDocs) != size {
		t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), size)
	}

	// Test an unrestrictive filter, which usually performs approximate search
	filter2 := intPointNewRangeQuery(t, "tag", lower, int32(numDocs))
	results = mustSearch(t, searcher, c.getKnnVectorQueryWithFilter("field", c.randomVector(dimension), size, filter2), size)
	if len(results.ScoreDocs) != size {
		t.Fatalf("scoreDocs.length = %d, want %d", len(results.ScoreDocs), size)
	}
}

func (c *knnVectorQueryTestCase) testDeletes() {
	t := c.t
	dir := c.newDirectoryForTest()
	defer mustClose(t, dir)
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	defer mustClose(t, w)
	numDocs := atLeast(100)
	dim := 30
	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		d.Add(newStringField(t, "index", itoa(i), true))
		if randomizedFrequently() {
			d.Add(c.getKnnVectorField("vector", c.randomVector(dim)))
		}
		mustAddDocument(t, w, d)
	}
	mustCommit(t, w)

	// Delete some documents at random, both those with and without vectors
	toDelete := make(map[string]bool)
	for i := 0; i < 25; i++ {
		idx := random().Intn(numDocs)
		toDelete[itoa(idx)] = true
	}
	terms := make([]index.Term, 0, len(toDelete))
	for v := range toDelete {
		terms = append(terms, *index.NewTerm("index", v))
	}
	if _, err := w.DeleteDocuments(terms); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	mustCommit(t, w)

	hits := 50
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	allIds := make(map[string]bool)
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("vector", c.randomVector(dim), hits)
	topDocs := mustSearch(t, searcher, query, numDocs)
	storedFields := mustStoredFields(t, reader)
	for _, scoreDoc := range topDocs.ScoreDocs {
		visitor := document.NewDocumentStoredFieldVisitorFor("index")
		if err := storedFields.Document(scoreDoc.Doc, visitor); err != nil {
			t.Fatalf("document(%d): %v", scoreDoc.Doc, err)
		}
		idx := visitor.GetDocument().Get("index").StringValue()
		if toDelete[idx] {
			t.Fatalf("search returned a deleted document: %s", idx)
		}
		allIds[idx] = true
	}
	if len(allIds) != hits {
		t.Fatalf("search missed some documents: got %d, want %d", len(allIds), hits)
	}
}

func (c *knnVectorQueryTestCase) testAllDeletes() {
	t := c.t
	dir := c.newDirectoryForTest()
	defer mustClose(t, dir)
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	defer mustClose(t, w)
	numDocs := atLeast(100)
	dim := 30
	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		d.Add(c.getKnnVectorField("vector", c.randomVector(dim)))
		mustAddDocument(t, w, d)
	}
	mustCommit(t, w)

	if _, err := w.DeleteDocumentsQuery([]index.Query{search.NewMatchAllDocsQuery()}); err != nil {
		t.Fatalf("deleteDocuments(MatchAllDocsQuery): %v", err)
	}
	mustCommit(t, w)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("vector", c.randomVector(dim), numDocs)
	topDocs := mustSearch(t, searcher, query, numDocs)
	if len(topDocs.ScoreDocs) != 0 {
		t.Fatalf("scoreDocs.length = %d, want 0", len(topDocs.ScoreDocs))
	}
}

// testMergeAwayAllValues tests ghost fields, that have a field info but no
// values.
func (c *knnVectorQueryTestCase) testMergeAwayAllValues() {
	t := c.t
	dim := 30
	dir := c.newDirectoryForTest()
	defer mustClose(t, dir)
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	defer mustClose(t, w)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "0", false))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "1", false))
	doc.Add(c.getKnnVectorField("field", c.randomVector(dim)))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", "1")}); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader := mustOpenDirectoryReaderFromWriter(t, w)
	defer mustClose(t, reader)
	leafReader := getOnlyLeafReader(t, reader)
	fi := leafReader.GetFieldInfos().FieldInfo("field")
	if fi == nil {
		t.Fatal("fieldInfo(field) is null")
	}
	var docID int
	var err error
	switch fi.VectorEncoding() {
	case index.VectorEncodingByte:
		vectorValues, verr := leafReader.GetByteVectorValues("field")
		if verr != nil {
			t.Fatalf("getByteVectorValues: %v", verr)
		}
		if vectorValues == nil {
			t.Fatal("vectorValues is null")
		}
		docID, err = vectorValues.Iterator().NextDoc()
	case index.VectorEncodingFloat32:
		vectorValues, verr := leafReader.GetFloatVectorValues("field")
		if verr != nil {
			t.Fatalf("getFloatVectorValues: %v", verr)
		}
		if vectorValues == nil {
			t.Fatal("vectorValues is null")
		}
		docID, err = vectorValues.Iterator().NextDoc()
	default:
		panic(util.NewAssertionError(""))
	}
	if err != nil {
		t.Fatalf("nextDoc: %v", err)
	}
	if docID != search.NO_MORE_DOCS {
		t.Fatalf("nextDoc = %d, want NO_MORE_DOCS", docID)
	}
}

// testNoLiveDocsReader checks that the query behaves reasonably when using a
// custom filter reader where there are no live docs.
func (c *knnVectorQueryTestCase) testNoLiveDocsReader() {
	t := c.t
	iwc := newIndexWriterConfig()
	dir := c.newDirectoryForTest()
	defer mustClose(t, dir)
	w := mustNewIndexWriter(t, dir, iwc)
	defer mustClose(t, w)
	numDocs := 10
	dim := 30
	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		d.Add(newStringField(t, "index", itoa(i), false))
		d.Add(c.getKnnVectorField("vector", c.randomVector(dim)))
		mustAddDocument(t, w, d)
	}
	mustCommit(t, w)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	// DirectoryReader wrappedReader = new NoLiveDocsDirectoryReader(reader);
	t.Fatal(filterDirectoryReaderSubReaderWrapperBlocker)
}

// testBitSetQuery tests that AbstractKnnVectorQuery optimizes the case where
// the filter query is backed by BitSetIterator.
func (c *knnVectorQueryTestCase) testBitSetQuery() {
	t := c.t
	iwc := newIndexWriterConfig()
	dir := c.newDirectoryForTest()
	defer mustClose(t, dir)
	w := mustNewIndexWriter(t, dir, iwc)
	defer mustClose(t, w)
	numDocs := 100
	dim := 30
	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		d.Add(c.getKnnVectorField("vector", c.randomVector(dim)))
		mustAddDocument(t, w, d)
	}
	mustCommit(t, w)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	// Query filter = new ThrowingBitSetQuery(new FixedBitSet(numDocs));
	t.Fatal(bitSetIteratorGetBitSetOverrideBlocker)
}

// testTimeLimitingKnnCollectorManager tests the functionality of
// TimeLimitingKnnCollectorManager.
func (c *knnVectorQueryTestCase) testTimeLimitingKnnCollectorManager() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	_ = searcher // consumed by the blocked body below
	t.Fatal(timeLimitingKnnCollectorManagerBlocker)
}

// testTimeout tests that the query times out correctly.
func (c *knnVectorQueryTestCase) testTimeout() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	query := c.getKnnVectorQuery("field", []float32{0.0, 1.0}, 2)
	exactQuery := c.getKnnVectorQueryWithFilter("field", []float32{0.0, 1.0}, 10, search.NewMatchAllDocsQuery())

	assertCount(t, searcher, query, 2)      // Expect some results without timeout
	assertCount(t, searcher, exactQuery, 3) // Same for exact search

	searcher.SetTimeout(queryTimeoutFunc(func() bool { return true })) // Immediately timeout
	assertCount(t, searcher, query, 0)                                 // Expect no results with the timeout
	assertCount(t, searcher, exactQuery, 0)                            // Same for exact search

	searcher.SetTimeout(newKnnCountingQueryTimeout(1)) // Only score 1 doc
	// Note: We get partial results when the HNSW graph has 1 layer, but no
	// results for > 1 layer because the timeout is exhausted while finding the
	// best entry node for the last level
	if got := mustCount(t, searcher, query); got > 1 {
		t.Fatalf("count = %d, want <= 1", got)
	}

	searcher.SetTimeout(newKnnCountingQueryTimeout(1)) // Only score 1 doc
	if got := mustCount(t, searcher, exactQuery); got > 1 {
		t.Fatalf("count = %d, want <= 1", got)
	}
}

// getIndexStore creates a new directory and adds documents with the given
// vectors as kNN vector fields.
func (c *knnVectorQueryTestCase) getIndexStore(field string, contents ...[]float32) store.Directory {
	return c.getIndexStoreWithSimilarity(field, index.VectorSimilarityFunctionEuclidean, contents...)
}

// getIndexStoreWithSimilarity creates a new directory and adds documents with
// the given vectors with similarity as kNN vector fields.
func (c *knnVectorQueryTestCase) getIndexStoreWithSimilarity(field string, vectorSimilarityFunction index.VectorSimilarityFunction, contents ...[]float32) store.Directory {
	t := c.t
	indexStore := c.newDirectoryForTest()
	writer := newRandomIndexWriter(t, indexStore)
	for i := range contents {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorFieldWithSimilarity(field, contents[i], vectorSimilarityFunction))
		doc.Add(newStringField(t, "id", "id"+itoa(i), true))
		mustAddDocument(t, writer, doc)
		if randomBoolean() {
			// Add some documents without a vector
			for j := 0; j < randomIntBetween(1, 5); j++ {
				doc = document.NewDocument()
				doc.Add(newStringField(t, "other", "value", false))
				// Add fields that will be matched by our test filters but won't
				// have vectors
				doc.Add(newStringField(t, "id", "id"+itoa(j), true))
				mustAddDocument(t, writer, doc)
			}
		}
	}
	// Add some documents without a vector
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "other", "value", false))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)
	return indexStore
}

// getStableIndexStore creates a new directory and adds documents with the
// given vectors as kNN vector fields, preserving the order of the added
// documents.
func (c *knnVectorQueryTestCase) getStableIndexStore(field string, contents ...[]float32) store.Directory {
	t := c.t
	indexStore := c.newDirectoryForTest()
	writer := mustNewIndexWriter(t, indexStore, index.NewIndexWriterConfig())
	for i := range contents {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorField(field, contents[i]))
		doc.Add(newStringField(t, "id", "id"+itoa(i), true))
		mustAddDocument(t, writer, doc)
	}
	// Add some documents without a vector
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "other", "value", false))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)
	return indexStore
}

// assertKnnMatches renders BaseKnnVectorQueryTestCase.assertMatches.
func assertKnnMatches(t *testing.T, searcher *search.IndexSearcher, q search.Query, expectedMatches int) {
	t.Helper()
	result := mustSearch(t, searcher, q, 1000).ScoreDocs
	if len(result) != expectedMatches {
		t.Fatalf("matches = %d, want %d", len(result), expectedMatches)
	}
}

// assertIdMatches renders BaseKnnVectorQueryTestCase.assertIdMatches.
func assertIdMatches(t *testing.T, reader storedFieldsProvider, expectedId string, scoreDoc *search.ScoreDoc) {
	t.Helper()
	actualId := storedDocument(t, mustStoredFields(t, reader), scoreDoc.Doc).Get("id").StringValue()
	if actualId != expectedId {
		t.Fatalf("id = %q, want %q", actualId, expectedId)
	}
}

// assertTopIdsMatches renders BaseKnnVectorQueryTestCase.assertTopIdsMatches.
func assertTopIdsMatches(t *testing.T, reader storedFieldsProvider, expectedIds map[string]bool, scoreDocs []*search.ScoreDoc) {
	t.Helper()
	actualIds := make(map[string]bool)
	storedFields := mustStoredFields(t, reader)
	for _, scoreDoc := range scoreDocs {
		actualIds[storedDocument(t, storedFields, scoreDoc.Doc).Get("id").StringValue()] = true
	}
	if len(expectedIds) != len(actualIds) {
		t.Fatalf("ids = %v, want %v", actualIds, expectedIds)
	}
	for id := range expectedIds {
		if !actualIds[id] {
			t.Fatalf("ids = %v, want %v", actualIds, expectedIds)
		}
	}
}

// docAndScoreQueryToString matches the pattern
// assertDocScoreQueryToString checks.
var docAndScoreQueryToString = regexp.MustCompile(`^DocAndScoreQuery\[\d+,...]\[\d+.\d+,...],1.0$`)

// assertDocScoreQueryToString renders
// BaseKnnVectorQueryTestCase.assertDocScoreQueryToString.
func assertDocScoreQueryToString(t *testing.T, query search.Query) {
	t.Helper()
	queryString := knnQueryToString(t, query, "ignored")
	// The string should contain matching docIds and their score. Since a
	// forceMerge could occur in this test, we must not assert that a specific
	// doc_id is matched But that instead the string format is expected and
	// that the max score is 1.0
	if !docAndScoreQueryToString.MatchString(queryString) {
		t.Fatalf("toString = %q does not match %s", queryString, docAndScoreQueryToString)
	}
}

// knnCountingQueryTimeout renders BaseKnnVectorQueryTestCase.CountingQueryTimeout.
type knnCountingQueryTimeout struct {
	remaining int
}

func newKnnCountingQueryTimeout(count int) *knnCountingQueryTimeout {
	return &knnCountingQueryTimeout{remaining: count}
}

func (q *knnCountingQueryTimeout) ShouldExit() bool {
	if q.remaining > 0 {
		q.remaining--
		return false
	}
	return true
}

// queryTimeoutFunc renders a QueryTimeout lambda (`() -> true`).
type queryTimeoutFunc func() bool

func (f queryTimeoutFunc) ShouldExit() bool { return f() }

func (c *knnVectorQueryTestCase) testSameFieldDifferentFormats() {
	t := c.t
	directory := c.newDirectoryForTest()
	defer mustClose(t, directory)
	// KnnVectorsFormat format1 = randomVectorFormat(VectorEncoding.FLOAT32);
	// iwc.setCodec(TestUtil.alwaysKnnVectorsFormat(format1));
	t.Fatal(alwaysKnnVectorsFormatBlocker)
}

func (c *knnVectorQueryTestCase) testStrategy() {
	t := c.t
	vector := c.getKnnVectorQuery("vector", c.randomVector(10), 3)
	if vector.GetSearchStrategy() == nil {
		t.Fatal("getSearchStrategy() is null")
	}
	if _, ok := vector.GetSearchStrategy().(*knn.Hnsw); !ok {
		t.Fatalf("getSearchStrategy() = %T, want KnnSearchStrategy.Hnsw", vector.GetSearchStrategy())
	}
}

// ---- assertion helpers ----

func assertFloatEquals(t *testing.T, what string, expected, actual, delta float64) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Fatalf("%s = %v, want %v (delta %v)", what, actual, expected, delta)
	}
}

func assertScorerScore(t *testing.T, scorer search.Scorer, expected, delta float64) {
	t.Helper()
	got, err := scorer.Score()
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	assertFloatEquals(t, "score", expected, float64(got), delta)
}

func assertMaxScore(t *testing.T, scorer search.Scorer, upTo int, expected, delta float64) {
	t.Helper()
	got, err := scorer.GetMaxScore(upTo)
	if err != nil {
		t.Fatalf("getMaxScore: %v", err)
	}
	assertFloatEquals(t, "getMaxScore", expected, float64(got), delta)
}

func assertAdvance(t *testing.T, it search.DocIdSetIterator, target, expected int) {
	t.Helper()
	got, err := it.Advance(target)
	if err != nil {
		t.Fatalf("advance(%d): %v", target, err)
	}
	if got != expected {
		t.Fatalf("advance(%d) = %d, want %d", target, got, expected)
	}
}

func mustExplain(t *testing.T, searcher *search.IndexSearcher, q search.Query, doc int) search.Explanation {
	t.Helper()
	e, err := searcher.Explain(q, doc)
	if err != nil {
		t.Fatalf("explain(%d): %v", doc, err)
	}
	return e
}

func assertKnnExplanation(t *testing.T, e search.Explanation, isMatch bool, value float32, description string) {
	t.Helper()
	if e.IsMatch() != isMatch {
		t.Fatalf("isMatch = %v, want %v", e.IsMatch(), isMatch)
	}
	if e.GetValue() != value {
		t.Fatalf("value = %v, want %v", e.GetValue(), value)
	}
	if isMatch && len(e.GetDetails()) != 0 {
		t.Fatalf("details = %d, want 0", len(e.GetDetails()))
	}
	if e.GetDescription() != description {
		t.Fatalf("description = %q, want %q", e.GetDescription(), description)
	}
}

func mustNumericDocValuesField(t *testing.T, name string, value int64) *document.NumericDocValuesField {
	t.Helper()
	f, err := document.NewNumericDocValuesField(name, value)
	if err != nil {
		t.Fatalf("NumericDocValuesField: %v", err)
	}
	return f
}

// assertDescendingScores verifies the results are in descending score order.
func assertDescendingScores(t *testing.T, scoreDocs []*search.ScoreDoc) {
	t.Helper()
	last := float32(math.MaxFloat32)
	for _, scoreDoc := range scoreDocs {
		if !(scoreDoc.Score <= last) {
			t.Fatalf("score %v > previous %v", scoreDoc.Score, last)
		}
		last = scoreDoc.Score
	}
}

// expectExactSearchUnsupported renders expectThrows(
// UnsupportedOperationException.class, () -> searcher.search(query, n)) for
// the ThrowingKnnVectorQuery subclasses.
func expectExactSearchUnsupported(t *testing.T, searcher *search.IndexSearcher, q search.Query, n int) {
	t.Helper()
	_, err := searcher.Search(q, n)
	if !errors.Is(err, errExactSearchNotSupported) {
		t.Fatalf("expected UnsupportedOperationException, got %v", err)
	}
}

// randomFloatVector renders TestVectorUtil.randomVector(int).
func randomFloatVector(dim int) []float32 {
	v := make([]float32, dim)
	r := random()
	for i := 0; i < dim; i++ {
		v[i] = r.Float32()
	}
	return v
}

// randomVectorBytes renders TestVectorUtil.randomVectorBytes(int):
// TestUtil.randomBinaryTerm(random(), dim) clipped at -127 to avoid overflow.
func randomVectorBytes(dim int) []byte {
	v := randomBinaryTerm(random(), dim)
	for i := range v {
		if int8(v[i]) == -128 {
			v[i] = byte(0x81) // -127
		}
	}
	return v
}

// randomBinaryTerm renders TestUtil.randomBinaryTerm(Random, int): length
// random bytes.
func randomBinaryTerm(r *rand.Rand, length int) []byte {
	b := make([]byte, length)
	r.Read(b)
	return b
}

// byteVectorAsFloats renders the randomVector(int) of the byte subclasses:
// the random bytes widened to float.
func byteVectorAsFloats(dim int) []float32 {
	b := randomVectorBytes(dim)
	v := make([]float32, len(b))
	vi := 0
	for i := range v {
		v[vi] = float32(int8(b[i]))
		vi++
	}
	return v
}

// patienceKnnVectorQueryBlocker names the class the TestPatience*VectorQuery
// subclasses build: Gocene's PatienceKnnVectorQuery is a wrapper with no
// Lucene counterpart (no AbstractKnnVectorQuery base, no fromFloatQuery /
// fromByteQuery / fromSeededQuery factories, no HnswQueueSaturationCollector).
const patienceKnnVectorQueryBlocker = "requires org.apache.lucene.search.PatienceKnnVectorQuery " +
	"(fromFloatQuery/fromByteQuery/fromSeededQuery, an AbstractKnnVectorQuery subclass) (not ported)"

// seededKnnVectorQueryBlocker names the class the TestSeeded*VectorQuery
// subclasses build: Gocene's SeededKnnVectorQuery is a wrapper with no Lucene
// counterpart (no AbstractKnnVectorQuery base, no fromFloatQuery /
// fromByteQuery factories, no SeededCollectorManager).
const seededKnnVectorQueryBlocker = "requires org.apache.lucene.search.SeededKnnVectorQuery " +
	"(fromFloatQuery/fromByteQuery, an AbstractKnnVectorQuery subclass) (not ported)"

// alwaysKnnVectorsFormatOnlyBlocker names TestUtil.alwaysKnnVectorsFormat.
const alwaysKnnVectorsFormatOnlyBlocker = "requires TestUtil.alwaysKnnVectorsFormat(KnnVectorsFormat) (not ported)"
