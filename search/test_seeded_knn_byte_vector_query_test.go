// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnByteVectorQuery.java

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testSeededKnnByteVectorQuery renders TestSeededKnnByteVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testSeededKnnByteVectorQuery struct {
	*knnVectorQueryTestCase
}

func newTestSeededKnnByteVectorQuery(t *testing.T) *testSeededKnnByteVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromByteQuery(new KnnByteVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromByteQuery(new TestKnnByteVectorQuery.ThrowingKnnVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.getCappedResultsThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery {
		// SeededKnnVectorQuery.fromByteQuery(new TestKnnByteVectorQuery.CappedResultsThrowingKnnVectorQuery(...), MATCH_NONE)
		t.Fatal(seededKnnVectorQueryBlocker)
		return nil
	}
	c.randomVector = byteVectorAsFloats
	c.getKnnVectorFieldWithSimilarity = func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField {
		return mustKnnByteVectorField(t, name, floatToBytes(vector), similarityFunction)
	}
	c.getKnnVectorField = func(name string, vector []float32) document.IndexableField {
		return mustKnnByteVectorField(t, name, floatToBytes(vector), index.VectorSimilarityFunctionEuclidean)
	}
	return &testSeededKnnByteVectorQuery{c}
}

func (c *testSeededKnnByteVectorQuery) testSeedWithTimeout() {
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
	// Query knnQuery = SeededKnnVectorQuery.fromByteQuery(knnVectorQuery, seed);
	t.Fatal(seededKnnVectorQueryBlocker)
}

// testRandomWithSeed tests with random vectors and a random seed. Uses
// RandomIndexWriter.
func (c *testSeededKnnByteVectorQuery) testRandomWithSeed() {
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
func TestSeededKnnByteVectorQueryEquals(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testEquals()
}
func TestSeededKnnByteVectorQueryGetField(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testGetField()
}
func TestSeededKnnByteVectorQueryGetK(t *testing.T) { newTestSeededKnnByteVectorQuery(t).testGetK() }
func TestSeededKnnByteVectorQueryGetFilter(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testGetFilter()
}
func TestSeededKnnByteVectorQueryEmptyIndex(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testEmptyIndex()
}
func TestSeededKnnByteVectorQueryFindAll(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testFindAll()
}
func TestSeededKnnByteVectorQueryFindFewer(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testFindFewer()
}
func TestSeededKnnByteVectorQuerySearchBoost(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testSearchBoost()
}
func TestSeededKnnByteVectorQuerySimpleFilter(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testSimpleFilter()
}
func TestSeededKnnByteVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestSeededKnnByteVectorQueryMatchAllFilter(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testMatchAllFilter()
}
func TestSeededKnnByteVectorQueryDimensionMismatch(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testDimensionMismatch()
}
func TestSeededKnnByteVectorQueryNonVectorField(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testNonVectorField()
}
func TestSeededKnnByteVectorQueryIllegalArguments(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testIllegalArguments()
}
func TestSeededKnnByteVectorQueryDifferentReader(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testDifferentReader()
}
func TestSeededKnnByteVectorQueryScoreEuclidean(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testScoreEuclidean()
}
func TestSeededKnnByteVectorQueryScoreCosine(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testScoreCosine()
}
func TestSeededKnnByteVectorQueryScoreMIP(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testScoreMIP()
}
func TestSeededKnnByteVectorQueryExplain(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testExplain()
}
func TestSeededKnnByteVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testExplainMultipleSegments()
}
func TestSeededKnnByteVectorQuerySkewedIndex(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testSkewedIndex()
}
func TestSeededKnnByteVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestSeededKnnByteVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestSeededKnnByteVectorQueryRandom(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testRandom()
}
func TestSeededKnnByteVectorQueryRandomWithFilter(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testRandomWithFilter()
}
func TestSeededKnnByteVectorQueryFilterWithSameScore(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testFilterWithSameScore()
}
func TestSeededKnnByteVectorQueryDeletes(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testDeletes()
}
func TestSeededKnnByteVectorQueryAllDeletes(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testAllDeletes()
}
func TestSeededKnnByteVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testMergeAwayAllValues()
}
func TestSeededKnnByteVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testNoLiveDocsReader()
}
func TestSeededKnnByteVectorQueryBitSetQuery(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testBitSetQuery()
}
func TestSeededKnnByteVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestSeededKnnByteVectorQueryTimeout(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testTimeout()
}
func TestSeededKnnByteVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testSameFieldDifferentFormats()
}
func TestSeededKnnByteVectorQueryStrategy(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testStrategy()
}

// Declared by TestSeededKnnByteVectorQuery.
func TestSeededKnnByteVectorQuerySeedWithTimeout(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testSeedWithTimeout()
}
func TestSeededKnnByteVectorQueryRandomWithSeed(t *testing.T) {
	newTestSeededKnnByteVectorQuery(t).testRandomWithSeed()
}
