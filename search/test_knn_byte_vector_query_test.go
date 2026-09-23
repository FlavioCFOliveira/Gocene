// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnByteVectorQuery.java

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testKnnByteVectorQuery renders TestKnnByteVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testKnnByteVectorQuery struct {
	*knnVectorQueryTestCase
}

func newTestKnnByteVectorQuery(t *testing.T) *testKnnByteVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		return search.NewKnnByteVectorQueryWithFilter(field, floatToBytes(query), k, queryFilter)
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		return newThrowingKnnByteVectorQuery(field, floatToBytes(vec), k, query)
	}
	c.getCappedResultsThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery {
		return newCappedResultsThrowingKnnByteVectorQuery(field, floatToBytes(vec), k, query, maxResults)
	}
	c.randomVector = byteVectorAsFloats
	c.getKnnVectorFieldWithSimilarity = func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField {
		return mustKnnByteVectorField(t, name, floatToBytes(vector), similarityFunction)
	}
	c.getKnnVectorField = func(name string, vector []float32) document.IndexableField {
		return mustKnnByteVectorField(t, name, floatToBytes(vector), index.VectorSimilarityFunctionEuclidean)
	}
	return &testKnnByteVectorQuery{c}
}

func mustKnnByteVectorField(t *testing.T, name string, vector []byte, similarityFunction index.VectorSimilarityFunction) *document.KnnByteVectorField {
	t.Helper()
	f, err := document.NewKnnByteVectorField(name, vector, similarityFunction)
	if err != nil {
		t.Fatalf("KnnByteVectorField: %v", err)
	}
	return f
}

// floatToBytes renders TestKnnByteVectorQuery.floatToBytes(float[]).
func floatToBytes(query []float32) []byte {
	b := make([]byte, len(query))
	for i := range query {
		if util.AssertsEnabled() && !(query[i] <= math.MaxInt8 && query[i] >= math.MinInt8 && float32(math.Mod(float64(query[i]), 1)) == 0) {
			panic(util.NewAssertionError(fmt.Sprintf("float value cannot be converted to byte; provided: %v", query[i])))
		}
		b[i] = byte(int8(query[i]))
	}
	return b
}

func (c *testKnnByteVectorQuery) testToString() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	query := c.getKnnVectorQuery("field", []float32{0, 1}, 10)
	if got, want := knnQueryToString(t, query, "ignored"), "KnnByteVectorQuery:field[0,...][10]"; got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}

	rewritten, err := query.Rewrite(newSearcher(t, reader))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	assertDocScoreQueryToString(t, rewritten)

	// test with filter
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	query = c.getKnnVectorQueryWithFilter("field", []float32{0, 1}, 10, filter)
	if got, want := knnQueryToString(t, query, "ignored"), "KnnByteVectorQuery:field[0,...][10][id:text]"; got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}
}

func (c *testKnnByteVectorQuery) testGetTarget() {
	t := c.t
	queryVectorBytes := floatToBytes([]float32{0, 1})
	q1 := search.NewKnnByteVectorQuery("f1", queryVectorBytes, 10)
	if got := q1.GetTargetCopy(); !bytes.Equal(queryVectorBytes, got) {
		t.Fatalf("getTargetCopy() = %v, want %v", got, queryVectorBytes)
	}
	if got := q1.GetTargetCopy(); &got[0] == &queryVectorBytes[0] {
		t.Fatal("assertNotSame(queryVectorBytes, q1.getTargetCopy())")
	}
}

func (c *testKnnByteVectorQuery) testVectorEncodingMismatch() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	var filter search.Query
	if randomBoolean() {
		filter = search.NewMatchAllDocsQuery()
	}
	query := search.NewKnnFloatVectorQueryWithFilter("field", []float32{0, 1}, 10, filter)
	searcher := newSearcher(t, reader)
	if _, err := searcher.Search(query, 10); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

// throwingKnnByteVectorQuery renders TestKnnByteVectorQuery.ThrowingKnnVectorQuery.
type throwingKnnByteVectorQuery struct {
	*search.KnnByteVectorQuery
}

func newThrowingKnnByteVectorQuery(field string, target []byte, k int, filter search.Query) *throwingKnnByteVectorQuery {
	q := &throwingKnnByteVectorQuery{search.NewKnnByteVectorQueryWithStrategy(field, target, k, filter, knn.NewHnsw(0))}
	search.SetKnnVectorQueryOverrides(&q.BaseKnnVectorQuery, q)
	return q
}

// ExactSearch overrides the protected exactSearch.
func (q *throwingKnnByteVectorQuery) ExactSearch(ctx *index.LeafReaderContext, acceptIterator search.DocIdSetIterator, queryTimeout index.QueryTimeout) (*search.TopDocs, error) {
	return nil, errExactSearchNotSupported
}

// ToString overrides toString(String), which returns null.
func (q *throwingKnnByteVectorQuery) ToString(field string) string {
	return ""
}

// cappedResultsThrowingKnnByteVectorQuery renders
// TestKnnByteVectorQuery.CappedResultsThrowingKnnVectorQuery.
type cappedResultsThrowingKnnByteVectorQuery struct {
	*throwingKnnByteVectorQuery
	maxResults int
}

func newCappedResultsThrowingKnnByteVectorQuery(field string, target []byte, k int, filter search.Query, maxResults int) *cappedResultsThrowingKnnByteVectorQuery {
	q := &cappedResultsThrowingKnnByteVectorQuery{newThrowingKnnByteVectorQuery(field, target, k, filter), maxResults}
	search.SetKnnVectorQueryOverrides(&q.BaseKnnVectorQuery, q)
	return q
}

// ApproximateSearch overrides the protected approximateSearch.
func (q *cappedResultsThrowingKnnByteVectorQuery) ApproximateSearch(ctx *index.LeafReaderContext, acceptDocs search.AcceptDocs, visitedLimit int, knnCollectorManager knn.KnnCollectorManager) (*search.TopDocs, error) {
	topDocs, err := q.KnnByteVectorQuery.ApproximateSearch(ctx, acceptDocs, math.MaxInt32, knnCollectorManager)
	if err != nil {
		return nil, err
	}
	return capTopDocs(topDocs, q.maxResults), nil
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestKnnByteVectorQueryEquals(t *testing.T)      { newTestKnnByteVectorQuery(t).testEquals() }
func TestKnnByteVectorQueryGetField(t *testing.T)    { newTestKnnByteVectorQuery(t).testGetField() }
func TestKnnByteVectorQueryGetK(t *testing.T)        { newTestKnnByteVectorQuery(t).testGetK() }
func TestKnnByteVectorQueryGetFilter(t *testing.T)   { newTestKnnByteVectorQuery(t).testGetFilter() }
func TestKnnByteVectorQueryEmptyIndex(t *testing.T)  { newTestKnnByteVectorQuery(t).testEmptyIndex() }
func TestKnnByteVectorQueryFindAll(t *testing.T)     { newTestKnnByteVectorQuery(t).testFindAll() }
func TestKnnByteVectorQueryFindFewer(t *testing.T)   { newTestKnnByteVectorQuery(t).testFindFewer() }
func TestKnnByteVectorQuerySearchBoost(t *testing.T) { newTestKnnByteVectorQuery(t).testSearchBoost() }
func TestKnnByteVectorQuerySimpleFilter(t *testing.T) {
	newTestKnnByteVectorQuery(t).testSimpleFilter()
}
func TestKnnByteVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestKnnByteVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestKnnByteVectorQueryMatchAllFilter(t *testing.T) {
	newTestKnnByteVectorQuery(t).testMatchAllFilter()
}
func TestKnnByteVectorQueryDimensionMismatch(t *testing.T) {
	newTestKnnByteVectorQuery(t).testDimensionMismatch()
}
func TestKnnByteVectorQueryNonVectorField(t *testing.T) {
	newTestKnnByteVectorQuery(t).testNonVectorField()
}
func TestKnnByteVectorQueryIllegalArguments(t *testing.T) {
	newTestKnnByteVectorQuery(t).testIllegalArguments()
}
func TestKnnByteVectorQueryDifferentReader(t *testing.T) {
	newTestKnnByteVectorQuery(t).testDifferentReader()
}
func TestKnnByteVectorQueryScoreEuclidean(t *testing.T) {
	newTestKnnByteVectorQuery(t).testScoreEuclidean()
}
func TestKnnByteVectorQueryScoreCosine(t *testing.T) { newTestKnnByteVectorQuery(t).testScoreCosine() }
func TestKnnByteVectorQueryScoreMIP(t *testing.T)    { newTestKnnByteVectorQuery(t).testScoreMIP() }
func TestKnnByteVectorQueryExplain(t *testing.T)     { newTestKnnByteVectorQuery(t).testExplain() }
func TestKnnByteVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestKnnByteVectorQuery(t).testExplainMultipleSegments()
}
func TestKnnByteVectorQuerySkewedIndex(t *testing.T) { newTestKnnByteVectorQuery(t).testSkewedIndex() }
func TestKnnByteVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestKnnByteVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestKnnByteVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestKnnByteVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestKnnByteVectorQueryRandom(t *testing.T) { newTestKnnByteVectorQuery(t).testRandom() }
func TestKnnByteVectorQueryRandomWithFilter(t *testing.T) {
	newTestKnnByteVectorQuery(t).testRandomWithFilter()
}
func TestKnnByteVectorQueryFilterWithSameScore(t *testing.T) {
	newTestKnnByteVectorQuery(t).testFilterWithSameScore()
}
func TestKnnByteVectorQueryDeletes(t *testing.T)    { newTestKnnByteVectorQuery(t).testDeletes() }
func TestKnnByteVectorQueryAllDeletes(t *testing.T) { newTestKnnByteVectorQuery(t).testAllDeletes() }
func TestKnnByteVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestKnnByteVectorQuery(t).testMergeAwayAllValues()
}
func TestKnnByteVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestKnnByteVectorQuery(t).testNoLiveDocsReader()
}
func TestKnnByteVectorQueryBitSetQuery(t *testing.T) { newTestKnnByteVectorQuery(t).testBitSetQuery() }
func TestKnnByteVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestKnnByteVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestKnnByteVectorQueryTimeout(t *testing.T) { newTestKnnByteVectorQuery(t).testTimeout() }
func TestKnnByteVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestKnnByteVectorQuery(t).testSameFieldDifferentFormats()
}
func TestKnnByteVectorQueryStrategy(t *testing.T) { newTestKnnByteVectorQuery(t).testStrategy() }

// Declared by TestKnnByteVectorQuery.
func TestKnnByteVectorQueryToString(t *testing.T)  { newTestKnnByteVectorQuery(t).testToString() }
func TestKnnByteVectorQueryGetTarget(t *testing.T) { newTestKnnByteVectorQuery(t).testGetTarget() }
func TestKnnByteVectorQueryVectorEncodingMismatch(t *testing.T) {
	newTestKnnByteVectorQuery(t).testVectorEncodingMismatch()
}
