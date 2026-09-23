// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnFloatVectorQuery.java

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testSeededKnnFloatVectorQuery renders TestSeededKnnFloatVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testSeededKnnFloatVectorQuery struct {
	*knnVectorQueryTestCase
}

func newTestSeededKnnFloatVectorQuery(t *testing.T) *testSeededKnnFloatVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromFloatQuery(new KnnFloatVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromFloatQuery(new TestKnnFloatVectorQuery.ThrowingKnnVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.getCappedResultsThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromFloatQuery(new TestKnnFloatVectorQuery.CappedResultsThrowingKnnVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.randomVector = randomFloatVector
	c.getKnnVectorFieldWithSimilarity = func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField {
		return mustKnnFloatVectorField(t, name, vector, similarityFunction)
	}
	c.getKnnVectorField = func(name string, vector []float32) document.IndexableField {
		return mustKnnFloatVectorField(t, name, vector, index.VectorSimilarityFunctionEuclidean)
	}
	return &testSeededKnnFloatVectorQuery{c}
}

func (c *testSeededKnnFloatVectorQuery) testSeedWithTimeout() {
	t := c.t
	numDocs := atLeast(50)
	dimension := atLeast(5)
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
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
	mustClose(t, w)

	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	searcher.SetTimeout(queryTimeoutFunc(func() bool { return true }))
	// Query knnQuery = SeededKnnVectorQuery.fromFloatQuery(knnVectorQuery, seed);
	t.Fatal(seededKnnVectorQueryBlocker)
}

// testRandomWithSeed tests with random vectors and a random seed. Uses
// RandomIndexWriter.
func (c *testSeededKnnFloatVectorQuery) testRandomWithSeed() {
	t := c.t
	d := c.newDirectoryForTest()
	defer mustClose(t, d)
	// Always use the default kNN format to have predictable behavior around
	// when it hits visitedLimit. This is fine since the test targets
	// AbstractKnnVectorQuery logic, not the kNN format implementation.
	// iwc.setCodec(TestUtil.alwaysKnnVectorsFormat(new Lucene99HnswVectorsFormat(
	//     DEFAULT_MAX_CONN, DEFAULT_BEAM_WIDTH, 0)));
	t.Fatal(alwaysKnnVectorsFormatOnlyBlocker)
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestSeededKnnFloatVectorQueryEquals(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testEquals()
}
func TestSeededKnnFloatVectorQueryGetField(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testGetField()
}
func TestSeededKnnFloatVectorQueryGetK(t *testing.T) { newTestSeededKnnFloatVectorQuery(t).testGetK() }
func TestSeededKnnFloatVectorQueryGetFilter(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testGetFilter()
}
func TestSeededKnnFloatVectorQueryEmptyIndex(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testEmptyIndex()
}
func TestSeededKnnFloatVectorQueryFindAll(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testFindAll()
}
func TestSeededKnnFloatVectorQueryFindFewer(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testFindFewer()
}
func TestSeededKnnFloatVectorQuerySearchBoost(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testSearchBoost()
}
func TestSeededKnnFloatVectorQuerySimpleFilter(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testSimpleFilter()
}
func TestSeededKnnFloatVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestSeededKnnFloatVectorQueryMatchAllFilter(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testMatchAllFilter()
}
func TestSeededKnnFloatVectorQueryDimensionMismatch(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testDimensionMismatch()
}
func TestSeededKnnFloatVectorQueryNonVectorField(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testNonVectorField()
}
func TestSeededKnnFloatVectorQueryIllegalArguments(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testIllegalArguments()
}
func TestSeededKnnFloatVectorQueryDifferentReader(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testDifferentReader()
}
func TestSeededKnnFloatVectorQueryScoreEuclidean(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testScoreEuclidean()
}
func TestSeededKnnFloatVectorQueryScoreCosine(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testScoreCosine()
}
func TestSeededKnnFloatVectorQueryScoreMIP(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testScoreMIP()
}
func TestSeededKnnFloatVectorQueryExplain(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testExplain()
}
func TestSeededKnnFloatVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testExplainMultipleSegments()
}
func TestSeededKnnFloatVectorQuerySkewedIndex(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testSkewedIndex()
}
func TestSeededKnnFloatVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestSeededKnnFloatVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestSeededKnnFloatVectorQueryRandom(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testRandom()
}
func TestSeededKnnFloatVectorQueryRandomWithFilter(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testRandomWithFilter()
}
func TestSeededKnnFloatVectorQueryFilterWithSameScore(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testFilterWithSameScore()
}
func TestSeededKnnFloatVectorQueryDeletes(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testDeletes()
}
func TestSeededKnnFloatVectorQueryAllDeletes(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testAllDeletes()
}
func TestSeededKnnFloatVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testMergeAwayAllValues()
}
func TestSeededKnnFloatVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testNoLiveDocsReader()
}
func TestSeededKnnFloatVectorQueryBitSetQuery(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testBitSetQuery()
}
func TestSeededKnnFloatVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestSeededKnnFloatVectorQueryTimeout(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testTimeout()
}
func TestSeededKnnFloatVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testSameFieldDifferentFormats()
}
func TestSeededKnnFloatVectorQueryStrategy(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testStrategy()
}

// Declared by TestSeededKnnFloatVectorQuery.
func TestSeededKnnFloatVectorQuerySeedWithTimeout(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testSeedWithTimeout()
}
func TestSeededKnnFloatVectorQueryRandomWithSeed(t *testing.T) {
	newTestSeededKnnFloatVectorQuery(t).testRandomWithSeed()
}
