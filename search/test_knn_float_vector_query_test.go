// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnFloatVectorQuery.java

import (
	"math"
	"slices"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testKnnFloatVectorQuery renders TestKnnFloatVectorQuery extends
// BaseKnnVectorQueryTestCase.
type testKnnFloatVectorQuery struct {
	*knnVectorQueryTestCase
}

func newTestKnnFloatVectorQuery(t *testing.T) *testKnnFloatVectorQuery {
	c := &knnVectorQueryTestCase{t: t}
	c.getKnnVectorQueryWithFilter = func(field string, query []float32, k int, queryFilter search.Query) abstractKnnVectorQuery {
		return search.NewKnnFloatVectorQueryWithFilter(field, query, k, queryFilter)
	}
	c.getThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query) abstractKnnVectorQuery {
		return newThrowingKnnFloatVectorQuery(field, vec, k, query)
	}
	c.getCappedResultsThrowingKnnVectorQuery = func(field string, vec []float32, k int, query search.Query, maxResults int) abstractKnnVectorQuery {
		return newCappedResultsThrowingKnnFloatVectorQuery(field, vec, k, query, maxResults)
	}
	c.randomVector = randomFloatVector
	c.getKnnVectorFieldWithSimilarity = func(name string, vector []float32, similarityFunction index.VectorSimilarityFunction) document.IndexableField {
		return mustKnnFloatVectorField(t, name, vector, similarityFunction)
	}
	c.getKnnVectorField = func(name string, vector []float32) document.IndexableField {
		f, err := document.NewKnnFloatVectorFieldEuclidean(name, vector)
		if err != nil {
			t.Fatalf("KnnFloatVectorField: %v", err)
		}
		return f
	}
	return &testKnnFloatVectorQuery{c}
}

func mustKnnFloatVectorField(t *testing.T, name string, vector []float32, similarityFunction index.VectorSimilarityFunction) *document.KnnFloatVectorField {
	t.Helper()
	f, err := document.NewKnnFloatVectorField(name, vector, similarityFunction)
	if err != nil {
		t.Fatalf("KnnFloatVectorField: %v", err)
	}
	return f
}

func (c *testKnnFloatVectorQuery) testToString() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	query := c.getKnnVectorQuery("field", []float32{0.0, 1.0}, 10)
	if got, want := knnQueryToString(t, query, "ignored"), "KnnFloatVectorQuery:field[0.0,...][10]"; got != want {
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
	if got, want := knnQueryToString(t, query, "ignored"), "KnnFloatVectorQuery:field[0.0,...][10][id:text]"; got != want {
		t.Fatalf("toString = %q, want %q", got, want)
	}
}

func (c *testKnnFloatVectorQuery) testVectorEncodingMismatch() {
	t := c.t
	indexStore := c.getIndexStore("field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	defer mustClose(t, indexStore)
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, reader)
	var filter search.Query
	if randomBoolean() {
		filter = search.NewMatchAllDocsQuery()
	}
	query := search.NewKnnByteVectorQueryWithFilter("field", []byte{0, 1}, 10, filter)
	searcher := newSearcher(t, reader)
	if _, err := searcher.Search(query, 10); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

func (c *testKnnFloatVectorQuery) testGetTarget() {
	t := c.t
	queryVector := []float32{0, 1}
	q1 := search.NewKnnFloatVectorQuery("f1", queryVector, 10)

	if got := q1.GetTargetCopy(); !slices.Equal(queryVector, got) {
		t.Fatalf("getTargetCopy() = %v, want %v", got, queryVector)
	}
	if got := q1.GetTargetCopy(); &got[0] == &queryVector[0] {
		t.Fatal("assertNotEquals(queryVector, q1.getTargetCopy()): same array")
	}
}

func (c *testKnnFloatVectorQuery) testScoreNegativeDotProduct() {
	t := c.t
	d := newDirectory()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	doc := document.NewDocument()
	doc.Add(c.getKnnVectorFieldWithSimilarity("field", []float32{-1, 0}, index.VectorSimilarityFunctionDotProduct))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(c.getKnnVectorFieldWithSimilarity("field", []float32{1, 0}, index.VectorSimilarityFunctionDotProduct))
	mustAddDocument(t, w, doc)
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	if n := len(mustLeaves(t, reader)); n != 1 {
		t.Fatalf("leaves = %d, want 1", n)
	}
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", []float32{1, 0}, 2)
	rewritten, err := query.Rewrite(searcher)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	weight := mustCreateWeight(t, searcher, rewritten, search.COMPLETE, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, reader)[0])

	// scores are normalized to lie in [0, 1]
	it := scorer.Iterator()
	if got := it.Cost(); got != 2 {
		t.Fatalf("cost = %d, want 2", got)
	}
	if got := mustNextDoc(t, it); got != 0 {
		t.Fatalf("nextDoc = %d, want 0", got)
	}
	if s := mustScore(t, scorer); !(0 <= s) {
		t.Fatalf("score = %v, want >= 0", s)
	}
	assertAdvance(t, it, 1, 1)
	assertScorerScore(t, scorer, 1, 0)
}

func (c *testKnnFloatVectorQuery) testScoreDotProduct() {
	t := c.t
	d := newDirectory()
	defer mustClose(t, d)
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
	for j := 1; j <= 5; j++ {
		doc := document.NewDocument()
		doc.Add(c.getKnnVectorFieldWithSimilarity("field",
			util.L2NormalizeThrow([]float32{float32(j), float32(j * j)}, true), index.VectorSimilarityFunctionDotProduct))
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	if n := len(mustLeaves(t, reader)); n != 1 {
		t.Fatalf("leaves = %d, want 1", n)
	}
	searcher := search.NewIndexSearcher(reader)
	query := c.getKnnVectorQuery("field", util.L2NormalizeThrow([]float32{2, 3}, true), 3)
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

	// doc 1 happens to have the max score
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

func (c *testKnnFloatVectorQuery) testDocAndScoreQueryBasics() {
	t := c.t
	directory := newDirectory()
	defer mustClose(t, directory)
	iw := newRandomIndexWriter(t, directory)
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "field", "value"+itoa(i), false))
		mustAddDocument(t, iw, doc)
		if i%10 == 0 {
			if err := iw.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
		}
	}
	reader := mustGetReader(t, iw)
	mustClose(t, iw)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	var scoreDocsList []*search.ScoreDoc
	for doc := 0; doc < 30; doc += 1 + random().Intn(5) {
		scoreDocsList = append(scoreDocsList, search.NewScoreDoc(doc, random().Float32(), -1))
	}
	scoreDocs := scoreDocsList
	docs := make([]int, len(scoreDocs))
	scores := make([]float32, len(scoreDocs))
	maxScore := float32(math.SmallestNonzeroFloat32)
	for i := range scoreDocs {
		docs[i] = scoreDocs[i].Doc
		scores[i] = scoreDocs[i].Score
		maxScore = max(maxScore, scores[i])
	}
	indexReader := searcher.GetIndexReader()
	leaves, err := indexReader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	segments := search.FindSegmentStarts(leaves, docs)

	ctx, err := indexReader.GetContext()
	if err != nil {
		t.Fatalf("getContext: %v", err)
	}
	query := search.NewDocAndScoreQuery(docs, scores, maxScore, segments, int64(len(scoreDocs)), ctx.ID())

	w, err := query.CreateWeight(searcher, search.TOP_SCORES, 1.0)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	topDocs := mustSearch(t, searcher, query, 100)
	if topDocs.TotalHits.Value != int64(len(scoreDocs)) {
		t.Fatalf("totalHits = %d, want %d", topDocs.TotalHits.Value, len(scoreDocs))
	}
	if query.Visited() != topDocs.TotalHits.Value {
		t.Fatalf("visited = %d, want %d", query.Visited(), topDocs.TotalHits.Value)
	}
	if topDocs.TotalHits.Relation != search.EQUAL_TO {
		t.Fatalf("relation = %v, want EQUAL_TO", topDocs.TotalHits.Relation)
	}
	sort.SliceStable(topDocs.ScoreDocs, func(i, j int) bool { return topDocs.ScoreDocs[i].Doc < topDocs.ScoreDocs[j].Doc })
	if len(topDocs.ScoreDocs) != len(scoreDocs) {
		t.Fatalf("scoreDocs.length = %d, want %d", len(topDocs.ScoreDocs), len(scoreDocs))
	}
	for i := range scoreDocs {
		if scoreDocs[i].Doc != topDocs.ScoreDocs[i].Doc {
			t.Fatalf("doc[%d] = %d, want %d", i, topDocs.ScoreDocs[i].Doc, scoreDocs[i].Doc)
		}
		assertFloatEquals(t, "score", float64(scoreDocs[i].Score), float64(topDocs.ScoreDocs[i].Score), 0.0001)
		if !mustExplain(t, searcher, query, scoreDocs[i].Doc).IsMatch() {
			t.Fatalf("explain(%d) is not a match", scoreDocs[i].Doc)
		}
	}

	for _, leafReaderContext := range searcher.GetLeafContexts() {
		scorer, err := w.Scorer(leafReaderContext)
		if err != nil {
			t.Fatalf("scorer: %v", err)
		}
		count, err := w.Count(leafReaderContext)
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if scorer == nil {
			if count != 0 {
				t.Fatalf("count = %d, want 0", count)
			}
		} else {
			maxS, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
			if err != nil {
				t.Fatalf("getMaxScore: %v", err)
			}
			if !(maxS > 0.0) {
				t.Fatalf("%v: getMaxScore = %v, want > 0", leafReaderContext, maxS)
			}
			if !(count > 0) {
				t.Fatalf("count = %d, want > 0", count)
			}
			iteratorCount := 0
			for mustNextDoc(t, scorer.Iterator()) != search.NO_MORE_DOCS {
				iteratorCount++
			}
			if iteratorCount != count {
				t.Fatalf("iteratorCount = %d, want %d", iteratorCount, count)
			}
		}
	}
}

// throwingKnnFloatVectorQuery renders TestKnnFloatVectorQuery.ThrowingKnnVectorQuery.
type throwingKnnFloatVectorQuery struct {
	*search.KnnFloatVectorQuery
}

func newThrowingKnnFloatVectorQuery(field string, target []float32, k int, filter search.Query) *throwingKnnFloatVectorQuery {
	q := &throwingKnnFloatVectorQuery{search.NewKnnFloatVectorQueryWithStrategy(field, target, k, filter, knn.NewHnsw(0))}
	search.SetKnnVectorQueryOverrides(&q.BaseKnnVectorQuery, q)
	return q
}

// ExactSearch overrides the protected exactSearch.
func (q *throwingKnnFloatVectorQuery) ExactSearch(ctx *index.LeafReaderContext, acceptIterator search.DocIdSetIterator, queryTimeout index.QueryTimeout) (*search.TopDocs, error) {
	return nil, errExactSearchNotSupported
}

// ToString overrides toString(String), which returns null.
func (q *throwingKnnFloatVectorQuery) ToString(field string) string {
	return ""
}

// cappedResultsThrowingKnnFloatVectorQuery renders
// TestKnnFloatVectorQuery.CappedResultsThrowingKnnVectorQuery.
type cappedResultsThrowingKnnFloatVectorQuery struct {
	*throwingKnnFloatVectorQuery
	maxResults int
}

func newCappedResultsThrowingKnnFloatVectorQuery(field string, target []float32, k int, filter search.Query, maxResults int) *cappedResultsThrowingKnnFloatVectorQuery {
	q := &cappedResultsThrowingKnnFloatVectorQuery{newThrowingKnnFloatVectorQuery(field, target, k, filter), maxResults}
	search.SetKnnVectorQueryOverrides(&q.BaseKnnVectorQuery, q)
	return q
}

// ApproximateSearch overrides the protected approximateSearch.
func (q *cappedResultsThrowingKnnFloatVectorQuery) ApproximateSearch(ctx *index.LeafReaderContext, acceptDocs search.AcceptDocs, visitedLimit int, knnCollectorManager knn.KnnCollectorManager) (*search.TopDocs, error) {
	topDocs, err := q.KnnFloatVectorQuery.ApproximateSearch(ctx, acceptDocs, math.MaxInt32, knnCollectorManager)
	if err != nil {
		return nil, err
	}
	return capTopDocs(topDocs, q.maxResults), nil
}

// capTopDocs renders the body shared by the CappedResultsThrowingKnnVectorQuery
// subclasses: the first min(totalHits, maxResults) hits with an EQUAL_TO total.
func capTopDocs(topDocs *search.TopDocs, maxResults int) *search.TopDocs {
	results := min(topDocs.TotalHits.Value, int64(maxResults))
	scoreDocs := make([]*search.ScoreDoc, results)
	copy(scoreDocs, topDocs.ScoreDocs[:len(scoreDocs)])
	return search.NewTopDocs(search.NewTotalHits(results, search.EQUAL_TO), scoreDocs)
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestKnnFloatVectorQueryEquals(t *testing.T)     { newTestKnnFloatVectorQuery(t).testEquals() }
func TestKnnFloatVectorQueryGetField(t *testing.T)   { newTestKnnFloatVectorQuery(t).testGetField() }
func TestKnnFloatVectorQueryGetK(t *testing.T)       { newTestKnnFloatVectorQuery(t).testGetK() }
func TestKnnFloatVectorQueryGetFilter(t *testing.T)  { newTestKnnFloatVectorQuery(t).testGetFilter() }
func TestKnnFloatVectorQueryEmptyIndex(t *testing.T) { newTestKnnFloatVectorQuery(t).testEmptyIndex() }
func TestKnnFloatVectorQueryFindAll(t *testing.T)    { newTestKnnFloatVectorQuery(t).testFindAll() }
func TestKnnFloatVectorQueryFindFewer(t *testing.T)  { newTestKnnFloatVectorQuery(t).testFindFewer() }
func TestKnnFloatVectorQuerySearchBoost(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testSearchBoost()
}
func TestKnnFloatVectorQuerySimpleFilter(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testSimpleFilter()
}
func TestKnnFloatVectorQueryFilterWithNoVectorMatches(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testFilterWithNoVectorMatches()
}
func TestKnnFloatVectorQueryMatchAllFilter(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testMatchAllFilter()
}
func TestKnnFloatVectorQueryDimensionMismatch(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testDimensionMismatch()
}
func TestKnnFloatVectorQueryNonVectorField(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testNonVectorField()
}
func TestKnnFloatVectorQueryIllegalArguments(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testIllegalArguments()
}
func TestKnnFloatVectorQueryDifferentReader(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testDifferentReader()
}
func TestKnnFloatVectorQueryScoreEuclidean(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testScoreEuclidean()
}
func TestKnnFloatVectorQueryScoreCosine(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testScoreCosine()
}
func TestKnnFloatVectorQueryScoreMIP(t *testing.T) { newTestKnnFloatVectorQuery(t).testScoreMIP() }
func TestKnnFloatVectorQueryExplain(t *testing.T)  { newTestKnnFloatVectorQuery(t).testExplain() }
func TestKnnFloatVectorQueryExplainMultipleSegments(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testExplainMultipleSegments()
}
func TestKnnFloatVectorQuerySkewedIndex(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testSkewedIndex()
}
func TestKnnFloatVectorQueryRandomConsistencySingleThreaded(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testRandomConsistencySingleThreaded()
}
func TestKnnFloatVectorQueryRandomConsistencyMultiThreaded(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testRandomConsistencyMultiThreaded()
}
func TestKnnFloatVectorQueryRandom(t *testing.T) { newTestKnnFloatVectorQuery(t).testRandom() }
func TestKnnFloatVectorQueryRandomWithFilter(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testRandomWithFilter()
}
func TestKnnFloatVectorQueryFilterWithSameScore(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testFilterWithSameScore()
}
func TestKnnFloatVectorQueryDeletes(t *testing.T)    { newTestKnnFloatVectorQuery(t).testDeletes() }
func TestKnnFloatVectorQueryAllDeletes(t *testing.T) { newTestKnnFloatVectorQuery(t).testAllDeletes() }
func TestKnnFloatVectorQueryMergeAwayAllValues(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testMergeAwayAllValues()
}
func TestKnnFloatVectorQueryNoLiveDocsReader(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testNoLiveDocsReader()
}
func TestKnnFloatVectorQueryBitSetQuery(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testBitSetQuery()
}
func TestKnnFloatVectorQueryTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testTimeLimitingKnnCollectorManager()
}
func TestKnnFloatVectorQueryTimeout(t *testing.T) { newTestKnnFloatVectorQuery(t).testTimeout() }
func TestKnnFloatVectorQuerySameFieldDifferentFormats(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testSameFieldDifferentFormats()
}
func TestKnnFloatVectorQueryStrategy(t *testing.T) { newTestKnnFloatVectorQuery(t).testStrategy() }

// Declared by TestKnnFloatVectorQuery.
func TestKnnFloatVectorQueryToString(t *testing.T) { newTestKnnFloatVectorQuery(t).testToString() }
func TestKnnFloatVectorQueryVectorEncodingMismatch(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testVectorEncodingMismatch()
}
func TestKnnFloatVectorQueryGetTarget(t *testing.T) { newTestKnnFloatVectorQuery(t).testGetTarget() }
func TestKnnFloatVectorQueryScoreNegativeDotProduct(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testScoreNegativeDotProduct()
}
func TestKnnFloatVectorQueryScoreDotProduct(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testScoreDotProduct()
}
func TestKnnFloatVectorQueryDocAndScoreQueryBasics(t *testing.T) {
	newTestKnnFloatVectorQuery(t).testDocAndScoreQueryBasics()
}
