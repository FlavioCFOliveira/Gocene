// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPatienceFloatVectorQuery.java

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testPatienceFloatVectorQuery renders TestPatienceFloatVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testPatienceFloatVectorQuery struct {
	*knnVectorQueryTestCase
	wrapSeeded bool
}

func newTestPatienceFloatVectorQuery(t *testing.T) *testPatienceFloatVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	p := &testPatienceFloatVectorQuery{knnVectorQueryTestCase: c}
	// setUp()
	p.wrapSeeded = randomBoolean()
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		// wrapSeeded ? PatienceKnnVectorQuery.fromSeededQuery(SeededKnnVectorQuery.fromFloatQuery(...))
		//            : PatienceKnnVectorQuery.fromFloatQuery(knnQuery)
		t.Fatal(patienceKnnVectorQueryBlocker)
		return nil
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		// PatienceKnnVectorQuery.fromFloatQuery(new ThrowingKnnVectorQuery(...))
		t.Fatal(patienceKnnVectorQueryBlocker)
		return nil
	}
	c.getCappedResultsThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery {
		return newCappedResultsThrowingKnnFloatVectorQuery(field, vec, k, query, maxResults)
	}
	c.randomVector = randomFloatVector
	c.getKnnVectorFieldWithSimilarity = func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField {
		return mustKnnFloatVectorField(t, name, vector, similarityFunction)
	}
	c.getKnnVectorField = func(name string, vector []float32) document.IndexableField {
		return mustKnnFloatVectorField(t, name, vector, index.VectorSimilarityFunctionEuclidean)
	}
	return p
}

func (c *testPatienceFloatVectorQuery) testToString() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	query := c.getKnnVectorQuery("field", []float32{0.0, 1.0}, 10)
	want := "PatienceKnnVectorQuery{saturationThreshold=0.995, patience=7, delegate="
	if c.wrapSeeded {
		want += "SeededKnnVectorQuery{seed=MatchNoDocsQuery(\"\"), seedWeight=null, delegate="
	}
	want += "KnnFloatVectorQuery:field[0.0,...][10]"
	if c.wrapSeeded {
		want += "}"
	}
	want += "}"
	if got := knnQueryToString(t, query, "ignored"); got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}

	rewritten, err := query.Rewrite(newSearcher(t, reader))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	assertDocScoreQueryToString(t, rewritten)

	// test with filter
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	query = c.getKnnVectorQueryWithFilter("field", []float32{0.0, 1.0}, 10, filter)
	want = "PatienceKnnVectorQuery{saturationThreshold=0.995, patience=7, delegate="
	if c.wrapSeeded {
		want += "SeededKnnVectorQuery{seed=MatchNoDocsQuery(\"\"), seedWeight=null, delegate="
	}
	want += "KnnFloatVectorQuery:field[0.0,...][10][id:text]"
	if c.wrapSeeded {
		want += "}"
	}
	want += "}"
	if got := knnQueryToString(t, query, "ignored"); got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestPatienceFloatVectorQueryEquals(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testEquals()
}
func TestPatienceFloatVectorQueryGetField(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testGetField()
}
func TestPatienceFloatVectorQueryGetK(t *testing.T) { newTestPatienceFloatVectorQuery(t).testGetK() }
func TestPatienceFloatVectorQueryGetFilter(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testGetFilter()
}
func TestPatienceFloatVectorQueryEmptyIndex(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testEmptyIndex()
}
func TestPatienceFloatVectorQueryFindAll(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testFindAll()
}
func TestPatienceFloatVectorQueryFindFewer(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testFindFewer()
}
func TestPatienceFloatVectorQuerySearchBoost(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testSearchBoost()
}
func TestPatienceFloatVectorQuerySimpleFilter(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testSimpleFilter()
}
func TestPatienceFloatVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestPatienceFloatVectorQueryMatchAllFilter(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testMatchAllFilter()
}
func TestPatienceFloatVectorQueryDimensionMismatch(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testDimensionMismatch()
}
func TestPatienceFloatVectorQueryNonVectorField(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testNonVectorField()
}
func TestPatienceFloatVectorQueryIllegalArguments(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testIllegalArguments()
}
func TestPatienceFloatVectorQueryDifferentReader(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testDifferentReader()
}
func TestPatienceFloatVectorQueryScoreEuclidean(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testScoreEuclidean()
}
func TestPatienceFloatVectorQueryScoreCosine(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testScoreCosine()
}
func TestPatienceFloatVectorQueryScoreMIP(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testScoreMIP()
}
func TestPatienceFloatVectorQueryExplain(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testExplain()
}
func TestPatienceFloatVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testExplainMultipleSegments()
}
func TestPatienceFloatVectorQuerySkewedIndex(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testSkewedIndex()
}
func TestPatienceFloatVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestPatienceFloatVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestPatienceFloatVectorQueryRandom(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testRandom()
}
func TestPatienceFloatVectorQueryRandomWithFilter(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testRandomWithFilter()
}
func TestPatienceFloatVectorQueryFilterWithSameScore(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testFilterWithSameScore()
}
func TestPatienceFloatVectorQueryDeletes(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testDeletes()
}
func TestPatienceFloatVectorQueryAllDeletes(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testAllDeletes()
}
func TestPatienceFloatVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testMergeAwayAllValues()
}
func TestPatienceFloatVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testNoLiveDocsReader()
}
func TestPatienceFloatVectorQueryBitSetQuery(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testBitSetQuery()
}
func TestPatienceFloatVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestPatienceFloatVectorQueryTimeout(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testTimeout()
}
func TestPatienceFloatVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testSameFieldDifferentFormats()
}
func TestPatienceFloatVectorQueryStrategy(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testStrategy()
}

// Declared by TestPatienceFloatVectorQuery.
func TestPatienceFloatVectorQueryToString(t *testing.T) {
	newTestPatienceFloatVectorQuery(t).testToString()
}
