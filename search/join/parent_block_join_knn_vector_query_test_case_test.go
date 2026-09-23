// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/ParentBlockJoinKnnVectorQueryTestCase.java
// (Apache Lucene 10.5.0).
//
// The abstract class is rendered as parentBlockJoinKnnVectorQueryTestCase,
// whose abstract methods are function fields supplied by the two concrete
// subclasses (TestParentBlockJoinByteKnnVectorQuery,
// TestParentBlockJoinFloatKnnVectorQuery); each subclass file declares one Go
// test per inherited test method.
type parentBlockJoinKnnVectorQueryTestCase struct {
	// randomVector renders the abstract randomVector(int).
	randomVector func(dim int) []float32
	// getParentJoinKnnQuery renders the abstract getParentJoinKnnQuery(String,
	// float[], Query, int, BitSetProducer).
	getParentJoinKnnQuery func(t testing.TB, fieldName string, queryVector []float32, childFilter search.Query, k int,
		parentBitSet BitSetProducer) search.Query
	// getKnnVectorField renders the abstract getKnnVectorField(String, float[]).
	getKnnVectorField func(t testing.TB, name string, vector []float32) document.IndexableField
	// getKnnVectorFieldWithSimilarity renders the abstract
	// getKnnVectorField(String, float[], VectorSimilarityFunction).
	getKnnVectorFieldWithSimilarity func(t testing.TB, name string, vector []float32,
		vectorSimilarityFunction spi.VectorSimilarityFunction) document.IndexableField
}

// encodeInts renders the static encodeInts(int[]): Arrays.toString(int[]).
func encodeInts(ints []int) string {
	parts := make([]string, len(ints))
	for i, v := range ints {
		parts[i] = strconv.Itoa(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// knnParentFilter renders the static parentFilter(IndexReader).
func knnParentFilter(t testing.TB, r index.IndexReaderInterface) BitSetProducer {
	t.Helper()
	// Create a filter that defines "parent" documents in the index
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	mustCheckJoinIndex(t, r, parentsFilter)
	return parentsFilter
}

// makeKnnParent renders makeParent(int[]).
func makeKnnParent(t testing.TB, children []int) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "docType", "_parent", false),
		newStringField(t, "id", encodeInts(children), true))
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testEmptyIndex(t *testing.T) {
	indexStore := c.getIndexStore(t, "field")
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	kvq := c.getParentJoinKnnQuery(t, "field", []float32{1, 2}, nil, 2,
		NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent"))))
	assertKnnMatches(t, searcher, kvq, 0)
	q := mustRewrite(t, searcher, kvq)
	if _, ok := q.(*search.MatchNoDocsQuery); !ok {
		t.Fatalf("expected MatchNoDocsQuery, got %T", q)
	}
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testIndexWithNoVectorsNorParents(t *testing.T) {
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := mustNewIndexWriter(t, d, iwc)
		defer mustClose(t, w)
		// Add some documents without a vector
		for i := 0; i < 5; i++ {
			mustAddDocument(t, w, newTestDocument(mustStringFieldPlain(t, "other", "value")))
		}
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	// Create parent filter directly, tests use "check" to verify parentIds exist. Production
	// may not verify we handle it gracefully
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	query := c.getParentJoinKnnQuery(t, "field", []float32{2, 2}, nil, 3, parentFilter)
	topDocs := mustSearch(t, searcher, query, 3)
	assertInt64Equals(t, 0, topDocs.TotalHits.Value)
	assertIntEquals(t, 0, len(topDocs.ScoreDocs))
	// Test with match_all filter and large k to test exact search
	query = c.getParentJoinKnnQuery(t, "field", []float32{2, 2}, search.Instance, 10, parentFilter)
	topDocs = mustSearch(t, searcher, query, 3)
	assertInt64Equals(t, 0, topDocs.TotalHits.Value)
	assertIntEquals(t, 0, len(topDocs.ScoreDocs))
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testIndexWithNoParents(t *testing.T) {
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := mustNewIndexWriter(t, d, iwc)
		defer mustClose(t, w)
		for i := 0; i < 3; i++ {
			mustAddDocument(t, w, newTestDocument(
				c.getKnnVectorField(t, "field", []float32{2, 2}),
				newStringField(t, "id", strconv.Itoa(i), true)))
		}
		// Add some documents without a vector
		for i := 0; i < 5; i++ {
			mustAddDocument(t, w, newTestDocument(mustStringFieldPlain(t, "other", "value")))
		}
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	// Create parent filter directly, tests use "check" to verify parentIds exist. Production
	// may not
	// verify we handle it gracefully
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	query := c.getParentJoinKnnQuery(t, "field", []float32{2, 2}, nil, 3, parentFilter)
	topDocs := mustSearch(t, searcher, query, 3)
	assertInt64Equals(t, 0, topDocs.TotalHits.Value)
	assertIntEquals(t, 0, len(topDocs.ScoreDocs))
	// Test with match_all filter and large k to test exact search
	query = c.getParentJoinKnnQuery(t, "field", []float32{2, 2}, search.Instance, 10, parentFilter)
	topDocs = mustSearch(t, searcher, query, 3)
	assertInt64Equals(t, 0, topDocs.TotalHits.Value)
	assertIntEquals(t, 0, len(topDocs.ScoreDocs))
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testFilterWithNoVectorMatches(t *testing.T) {
	indexStore := c.getIndexStore(t, "field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, indexStore)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	filter := search.NewTermQuery(index.NewTerm("other", "value"))
	parentFilter := knnParentFilter(t, reader)
	kvq := c.getParentJoinKnnQuery(t, "field", []float32{1, 2}, filter, 2, parentFilter)
	topDocs := mustSearch(t, searcher, kvq, 3)
	assertInt64Equals(t, 0, topDocs.TotalHits.Value)
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testScoringWithMultipleChildren(t *testing.T) {
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := mustNewIndexWriter(t, d, iwc)
		defer mustClose(t, w)
		toAdd := make([]*document.Document, 0)
		for j := 1; j <= 5; j++ {
			toAdd = append(toAdd, newTestDocument(
				c.getKnnVectorField(t, "field", []float32{float32(j), float32(j)}),
				newStringField(t, "id", strconv.Itoa(j), true)))
		}
		toAdd = append(toAdd, makeKnnParent(t, []int{1, 2, 3, 4, 5}))
		mustAddDocuments(t, w, toAdd...)
		toAdd = make([]*document.Document, 0)
		for j := 7; j <= 11; j++ {
			toAdd = append(toAdd, newTestDocument(
				c.getKnnVectorField(t, "field", []float32{float32(j), float32(j)}),
				newStringField(t, "id", strconv.Itoa(j), true)))
		}
		toAdd = append(toAdd, makeKnnParent(t, []int{6, 7, 8, 9, 10}))
		mustAddDocuments(t, w, toAdd...)
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	assertIntEquals(t, 1, len(mustLeaves(t, reader)))
	searcher := search.NewIndexSearcher(reader)
	parentFilter := knnParentFilter(t, searcher.GetIndexReader())
	query := c.getParentJoinKnnQuery(t, "field", []float32{2, 2}, nil, 3, parentFilter)
	assertScorerResults(t, searcher, query, []float32{1, 1.0 / 51}, []string{"2", "7"}, 2)
	query = c.getParentJoinKnnQuery(t, "field", []float32{6, 6}, nil, 3, parentFilter)
	assertScorerResults(t, searcher, query, []float32{1.0 / 3, 1.0 / 3}, []string{"5", "7"}, 2)
	query = c.getParentJoinKnnQuery(t, "field", []float32{6, 6}, search.Instance, 20, parentFilter)
	assertScorerResults(t, searcher, query, []float32{1.0 / 3, 1.0 / 3}, []string{"5", "7"}, 2)
	query = c.getParentJoinKnnQuery(t, "field", []float32{6, 6}, search.Instance, 1, parentFilter)
	assertScorerResults(t, searcher, query, []float32{1.0 / 3, 1.0 / 3}, []string{"5", "7"}, 1)
}

// testSkewedIndex tests that when vectors are abnormally distributed among
// segments, we still find the top K.
func (c *parentBlockJoinKnnVectorQueryTestCase) testSkewedIndex(t *testing.T) {
	/* We have to choose the numbers carefully here so that some segment has more than the expected
	 * number of top K documents, but no more than K documents in total (otherwise we might occasionally
	 * randomly fail to find one).
	 */
	d := newDirectory()
	defer mustClose(t, d)
	func() {
		w := mustNewIndexWriter(t, d, index.NewIndexWriterConfig())
		defer mustClose(t, w)
		r := 0
		for i := 0; i < 5; i++ {
			for j := 0; j < 5; j++ {
				mustAddDocuments(t, w,
					newTestDocument(
						c.getKnnVectorField(t, "field", []float32{float32(r), float32(r)}),
						newStringField(t, "id", strconv.Itoa(r), true)),
					makeKnnParent(t, []int{r}))
				r++
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
		}
	}()
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)
	results := mustSearch(t, searcher,
		c.getParentJoinKnnQuery(t, "field", []float32{0, 0}, nil, 8, knnParentFilter(t, searcher.GetIndexReader())), 10)
	assertIntEquals(t, 8, len(results.ScoreDocs))
	assertIDMatches(t, reader, "0", results.ScoreDocs[0].Doc)
	assertIDMatches(t, reader, "7", results.ScoreDocs[7].Doc)
	// test some results in the middle of the sequence - also tests docid tiebreaking
	results = mustSearch(t, searcher,
		c.getParentJoinKnnQuery(t, "field", []float32{10, 10}, nil, 8, knnParentFilter(t, searcher.GetIndexReader())), 10)
	assertIntEquals(t, 8, len(results.ScoreDocs))
	assertIDMatches(t, reader, "10", results.ScoreDocs[0].Doc)
	assertIDMatches(t, reader, "6", results.ScoreDocs[7].Doc)
}

// testTimeout tests that the query times out correctly.
func (c *parentBlockJoinKnnVectorQueryTestCase) testTimeout(t *testing.T) {
	indexStore := c.getIndexStore(t, "field", []float32{0, 1}, []float32{1, 2}, []float32{0, 0})
	reader := mustOpenDirectoryReader(t, indexStore)
	defer mustClose(t, indexStore)
	defer mustClose(t, reader)
	parentFilter := knnParentFilter(t, reader)
	searcher := newSearcher(t, reader)
	query := c.getParentJoinKnnQuery(t, "field", []float32{1, 2}, nil, 2, parentFilter)
	exactQuery := c.getParentJoinKnnQuery(t, "field", []float32{1, 2}, search.Instance, 10, parentFilter)
	assertIntEquals(t, 2, mustCount(t, searcher, query))      // Expect some results without timeout
	assertIntEquals(t, 3, mustCount(t, searcher, exactQuery)) // Same for exact search
	searcher.SetTimeout(alwaysTimeout{})                      // Immediately timeout
	assertIntEquals(t, 0, mustCount(t, searcher, query))      // Expect no results with the timeout
	assertIntEquals(t, 0, mustCount(t, searcher, exactQuery)) // Same for exact search
	searcher.SetTimeout(&countingQueryTimeout{remaining: 1})  // Only score 1 parent
	// Note: We get partial results when the HNSW graph has 1 layer, but no results for > 1 layer
	// because the timeout is exhausted while finding the best entry node for the last level
	if n := mustCount(t, searcher, query); !(n <= 1) {
		t.Fatalf("count: expected <= 1, got %d", n)
	}
	searcher.SetTimeout(&countingQueryTimeout{remaining: 1}) // Only score 1 parent
	if n := mustCount(t, searcher, exactQuery); !(n <= 1) {
		t.Fatalf("count: expected <= 1, got %d", n)
	}
}

// alwaysTimeout renders the lambda () -> true.
type alwaysTimeout struct{}

func (alwaysTimeout) ShouldExit() bool { return true }

func (c *parentBlockJoinKnnVectorQueryTestCase) getIndexStore(t testing.TB, field string, contents ...[]float32) store.Directory {
	t.Helper()
	indexStore := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	writer := newRandomIndexWriterWithConfig(t, indexStore, iwc)
	for i := range contents {
		mustAddDocuments(t, writer,
			newTestDocument(
				c.getKnnVectorField(t, field, contents[i]),
				newStringField(t, "id", strconv.Itoa(i), true)),
			makeKnnParent(t, []int{i}))
	}
	// Add some documents without a vector
	for i := 0; i < 5; i++ {
		mustAddDocuments(t, writer,
			newTestDocument(mustStringFieldPlain(t, "other", "value")),
			makeKnnParent(t, []int{}))
	}
	mustClose(t, writer)
	return indexStore
}

func assertKnnMatches(t testing.TB, searcher *search.IndexSearcher, q search.Query, expectedMatches int) {
	t.Helper()
	result := mustSearch(t, searcher, q, 1000).ScoreDocs
	assertIntEquals(t, expectedMatches, len(result))
}

func assertIDMatches(t testing.TB, reader index.IndexReaderInterface, expectedID string, docID int) {
	t.Helper()
	assertStringEquals(t, expectedID, storedGet(t, reader, docID, "id"))
}

func assertScorerResults(t testing.TB, searcher *search.IndexSearcher, query search.Query, possibleScores []float32,
	possibleIDs []string, count int) {
	t.Helper()
	reader := searcher.GetIndexReader()
	rewritten, err := query.Rewrite(searcher)
	if err != nil {
		t.Fatal(err)
	}
	weight := mustCreateWeight(t, searcher, rewritten, search.COMPLETE, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, searcher.GetIndexReader())[0])
	// prior to advancing, score is undefined
	assertIntEquals(t, -1, scorer.DocID())
	// expectThrows(ArrayIndexOutOfBoundsException.class, scorer::score)
	expectThrowsPanicOrError(t, func() error {
		_, err := scorer.Score()
		return err
	})
	it := scorer.Iterator()
	idToScore := map[string]float32{}
	for i := range possibleIDs {
		idToScore[possibleIDs[i]] = possibleScores[i]
	}
	for i := 0; i < count; i++ {
		docID := mustNextDoc(t, it)
		if docID == search.NO_MORE_DOCS {
			t.Fatal("assertNotEquals(NO_MORE_DOCS, docId)")
		}
		actualID := storedGet(t, reader, docID, "id")
		want, ok := idToScore[actualID]
		if !ok {
			t.Fatalf("unexpected id %q", actualID)
		}
		if got := mustScore(t, scorer); math.Abs(float64(want-got)) > 0.0001 {
			t.Fatalf("score of %q: expected %v, got %v", actualID, want, got)
		}
	}
}

// expectThrowsPanicOrError renders expectThrows for a Java unchecked
// exception, which a Go port surfaces as either an error or a panic.
func expectThrowsPanicOrError(t testing.TB, fn func() error) {
	t.Helper()
	var err error
	panicked := func() (p bool) {
		defer func() {
			if recover() != nil {
				p = true
			}
		}()
		err = fn()
		return false
	}()
	if !panicked && err == nil {
		t.Fatal("expected an exception")
	}
}

// countingQueryTimeout renders the private static class CountingQueryTimeout.
type countingQueryTimeout struct {
	remaining int
}

func (c *countingQueryTimeout) ShouldExit() bool {
	if c.remaining > 0 {
		c.remaining--
		return false
	}
	return true
}

func (c *parentBlockJoinKnnVectorQueryTestCase) testTwoSegments(t *testing.T) {
	// see https://github.com/apache/lucene/issues/15005
	dim := 1 + random().Intn(9) // random().nextInt(1, 10)
	d := newDirectory()
	defer mustClose(t, d)
	writer := newRandomIndexWriterWithConfig(t, d, newIndexWriterConfig())
	mustAddDocuments(t, writer, c.createFamily(t, "a", 2, dim)...)
	mustAddDocuments(t, writer, c.createFamily(t, "b", 3, dim)...)
	mustCommit(t, writer)
	mustAddDocuments(t, writer, c.createFamily(t, "c", 1, dim)...)
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	searcher := newSearcher(t, reader)
	query := c.getParentJoinKnnQuery(t, "field", c.randomVector(dim), nil, 3, parentFilter)
	results := mustSearch(t, searcher, query, 3)
	assertIntEquals(t, 3, len(results.ScoreDocs))
	if !(results.TotalHits.Value >= int64(len(results.ScoreDocs))) {
		t.Fatalf("totalHits %d < %d", results.TotalHits.Value, len(results.ScoreDocs))
	}
	resultParentIDs := map[string]struct{}{}
	for _, scoreDoc := range results.ScoreDocs {
		parentID := storedGet(t, reader, scoreDoc.Doc, "parentId")
		if _, ok := resultParentIDs[parentID]; ok {
			t.Fatalf("duplicate parent %q", parentID)
		}
		resultParentIDs[parentID] = struct{}{}
	}
	assertSetEquals(t, asSet("a", "b", "c"), resultParentIDs)
}

func (c *parentBlockJoinKnnVectorQueryTestCase) createFamily(t testing.TB, parentID string, size, dim int) []*document.Document {
	family := make([]*document.Document, 0)
	for i := 0; i < size; i++ {
		family = append(family, newTestDocument(
			c.getKnnVectorField(t, "field", c.randomVector(dim)),
			mustStoredStringField(t, "parentId", parentID)))
	}
	family = append(family, newTestDocument(mustStringFieldPlain(t, "docType", "_parent")))
	return family
}

// testRandom tests with random vectors, number of documents, etc. Uses
// RandomIndexWriter.
func (c *parentBlockJoinKnnVectorQueryTestCase) testRandom(t *testing.T) {
	numDocs := atLeast(100)
	dimension := atLeast(5)
	numIters := atLeast(10)
	everyDocHasAVector := random().Intn(2) == 0
	numParentsWithChildren := 0
	d := newDirectory()
	defer mustClose(t, d)
	w := newRandomIndexWriter(t, d)
	family := make([]*document.Document, 0)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if random().Intn(5) == 1 {
			if len(family) != 0 {
				numParentsWithChildren++
				doc.Add(mustStoredStringField(t, "id", strconv.Itoa(i)))
			} else {
				doc.Add(mustStoredStringField(t, "id", "pnoc"+strconv.Itoa(i)))
			}
			doc.Add(mustStringFieldPlain(t, "docType", "_parent"))
			family = append(family, doc)
			mustAddDocuments(t, w, family...)
			family = family[:0]
		} else if everyDocHasAVector || random().Intn(10) != 2 {
			// NOTE: only child documents are allowed to have a vector!
			// Otherwise the query's assumptions are invalidated??
			doc.Add(c.getKnnVectorField(t, "field", c.randomVector(dimension)))
			doc.Add(mustStoredStringField(t, "id", "c"+strconv.Itoa(i)))
			family = append(family, doc)
		}
	}
	mustClose(t, w)
	// trailing children with no parent document are dropped
	reader := mustOpenDirectoryReader(t, d)
	defer mustClose(t, reader)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	searcher := newSearcher(t, reader)
	for i := 0; i < numIters; i++ {
		k := random().Intn(80) + 1
		// TODO: test with child filter
		query := c.getParentJoinKnnQuery(t, "field", c.randomVector(dimension), nil, k, parentFilter)
		n := random().Intn(100) + 1
		results := mustSearch(t, searcher, query, n)
		expected := min(min(n, k), numParentsWithChildren)
		// we may get fewer results than requested if there are deletions, but this test doesn't
		// test that
		if util.AssertsEnabled() && reader.HasDeletions() {
			t.Fatal(util.NewAssertionError("reader.hasDeletions() == false"))
		}
		assertIntEquals(t, expected, len(results.ScoreDocs))
		if !(results.TotalHits.Value >= int64(len(results.ScoreDocs))) {
			t.Fatalf("totalHits %d < %d", results.TotalHits.Value, len(results.ScoreDocs))
		}
		// verify the results are in descending score order
		last := float32(math.MaxFloat32)
		for _, scoreDoc := range results.ScoreDocs {
			if !(scoreDoc.Score <= last) {
				t.Fatalf("scores not descending: %v after %v", scoreDoc.Score, last)
			}
			last = scoreDoc.Score
		}
	}
}

// randomBinaryTerm renders TestUtil.randomBinaryTerm(Random, int): length
// random bytes.
func randomBinaryTerm(r *rand.Rand, length int) []byte {
	b := make([]byte, length)
	r.Read(b)
	return b
}
