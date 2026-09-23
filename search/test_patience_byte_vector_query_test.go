// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPatienceByteVectorQuery.java

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testPatienceByteVectorQuery renders TestPatienceByteVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testPatienceByteVectorQuery struct {
	*knnVectorQueryTestCase
	wrapSeeded bool
}

func newTestPatienceByteVectorQuery(t *testing.T) *testPatienceByteVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	p := &testPatienceByteVectorQuery{knnVectorQueryTestCase: c}
	// setUp()
	p.wrapSeeded = randomBoolean()
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		// wrapSeeded ? PatienceKnnVectorQuery.fromSeededQuery(SeededKnnVectorQuery.fromByteQuery(...))
		//            : PatienceKnnVectorQuery.fromByteQuery(knnQuery)
		t.Fatal(patienceKnnVectorQueryBlocker)
		return nil
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		// PatienceKnnVectorQuery.fromByteQuery(new ThrowingKnnVectorQuery(...))
		t.Fatal(patienceKnnVectorQueryBlocker)
		return nil
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
	return p
}

func (c *testPatienceByteVectorQuery) testToString() {
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
	want += "KnnByteVectorQuery:field[0,...][10]"
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
	want += "KnnByteVectorQuery:field[0,...][10][id:text]"
	if c.wrapSeeded {
		want += "}"
	}
	want += "}"
	if got := knnQueryToString(t, query, "ignored"); got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestPatienceByteVectorQueryEquals(t *testing.T) { newTestPatienceByteVectorQuery(t).testEquals() }
func TestPatienceByteVectorQueryGetField(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testGetField()
}
func TestPatienceByteVectorQueryGetK(t *testing.T) { newTestPatienceByteVectorQuery(t).testGetK() }
func TestPatienceByteVectorQueryGetFilter(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testGetFilter()
}
func TestPatienceByteVectorQueryEmptyIndex(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testEmptyIndex()
}
func TestPatienceByteVectorQueryFindAll(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testFindAll()
}
func TestPatienceByteVectorQueryFindFewer(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testFindFewer()
}
func TestPatienceByteVectorQuerySearchBoost(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testSearchBoost()
}
func TestPatienceByteVectorQuerySimpleFilter(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testSimpleFilter()
}
func TestPatienceByteVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestPatienceByteVectorQueryMatchAllFilter(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testMatchAllFilter()
}
func TestPatienceByteVectorQueryDimensionMismatch(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testDimensionMismatch()
}
func TestPatienceByteVectorQueryNonVectorField(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testNonVectorField()
}
func TestPatienceByteVectorQueryIllegalArguments(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testIllegalArguments()
}
func TestPatienceByteVectorQueryDifferentReader(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testDifferentReader()
}
func TestPatienceByteVectorQueryScoreEuclidean(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testScoreEuclidean()
}
func TestPatienceByteVectorQueryScoreCosine(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testScoreCosine()
}
func TestPatienceByteVectorQueryScoreMIP(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testScoreMIP()
}
func TestPatienceByteVectorQueryExplain(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testExplain()
}
func TestPatienceByteVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testExplainMultipleSegments()
}
func TestPatienceByteVectorQuerySkewedIndex(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testSkewedIndex()
}
func TestPatienceByteVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestPatienceByteVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestPatienceByteVectorQueryRandom(t *testing.T) { newTestPatienceByteVectorQuery(t).testRandom() }
func TestPatienceByteVectorQueryRandomWithFilter(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testRandomWithFilter()
}
func TestPatienceByteVectorQueryFilterWithSameScore(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testFilterWithSameScore()
}
func TestPatienceByteVectorQueryDeletes(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testDeletes()
}
func TestPatienceByteVectorQueryAllDeletes(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testAllDeletes()
}
func TestPatienceByteVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testMergeAwayAllValues()
}
func TestPatienceByteVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testNoLiveDocsReader()
}
func TestPatienceByteVectorQueryBitSetQuery(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testBitSetQuery()
}
func TestPatienceByteVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestPatienceByteVectorQueryTimeout(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testTimeout()
}
func TestPatienceByteVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testSameFieldDifferentFormats()
}
func TestPatienceByteVectorQueryStrategy(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testStrategy()
}

// Declared by TestPatienceByteVectorQuery.
func TestPatienceByteVectorQueryToString(t *testing.T) {
	newTestPatienceByteVectorQuery(t).testToString()
}
