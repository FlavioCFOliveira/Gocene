// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBoolean2.java
// (Apache Lucene 10.5.0): test BooleanQuery2 against BooleanQuery by
// overriding the standard query parser. This also tests the scoring order of
// BooleanQuery. The static @BeforeClass state is rebuilt per test
// (boolean2BeforeClass).

package search_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// directoryCopyFromBlocker names Directory.copyFrom(Directory, String, String, IOContext).
const directoryCopyFromBlocker = "requires org.apache.lucene.store.Directory#copyFrom(Directory, String, String, IOContext) (not ported)"

const (
	// boolean2NumExtraDocs renders NUM_EXTRA_DOCS: num "extra" docs containing
	// value in "field2" added to the "big" clone of the index.
	boolean2NumExtraDocs = 6000
	boolean2Field        = "field"
)

var boolean2DocFields = []string{
	"w1 w2 w3 w4 w5", "w1 w3 w2 w3", "w1 xx w2 yy w3", "w1 w3 xx w2 yy mm",
}

// boolean2Class renders the static fields of TestBoolean2.
type boolean2Class struct {
	searcher              *search.IndexSearcher
	singleSegmentSearcher *search.IndexSearcher
	bigSearcher           *search.IndexSearcher
	reader                index.IndexReaderInterface
	littleReader          index.IndexReaderInterface
	singleSegmentReader   index.IndexReaderInterface

	// numFillerDocs renders NUM_FILLER_DOCS: num of empty docs injected
	// between every doc in the (main) index.
	numFillerDocs int
	// preFillerDocs renders PRE_FILLER_DOCS: num of empty docs injected prior
	// to the first doc in the (main) index.
	preFillerDocs int

	directory              store.Directory
	singleSegmentDirectory store.Directory
	dir2                   store.Directory
	mulFactor              int
}

// boolean2CopyOf renders the private static copyOf(Directory).
func boolean2CopyOf(t *testing.T, dir store.Directory) store.Directory {
	t.Helper()
	// Directory copy = newFSDirectory(createTempDir());
	t.Fatal(newFSDirectoryBlocker)
	return nil
}

// boolean2BeforeClass renders beforeClass(); afterClass is registered with
// t.Cleanup.
func boolean2BeforeClass(t *testing.T) *boolean2Class {
	t.Helper()
	c := &boolean2Class{}
	// in some runs, test immediate adjacency of matches - in others, force a full bucket gap
	// between docs
	if random().Intn(2) == 0 {
		c.numFillerDocs = 0
	} else {
		c.numFillerDocs = search.BooleanScorerSIZE
	}
	c.preFillerDocs = nextInt(0, c.numFillerDocs/2)
	if testing.Verbose() {
		t.Logf("TEST: NUM_FILLER_DOCS=%d PRE_FILLER_DOCS=%d", c.numFillerDocs, c.preFillerDocs)
	}

	if c.numFillerDocs*c.preFillerDocs > 100000 {
		// directory = newFSDirectory(createTempDir());
		t.Fatal(newFSDirectoryBlocker)
	}
	c.directory = newDirectory()

	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	// randomized codecs are sometimes too costly for this test:
	iwc.SetCodec(index.GetDefaultCodec())
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, c.directory, iwc)
	// we'll make a ton of docs, disable store/norms/vectors
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetOmitNorms(true)

	doc := document.NewDocument()
	for filler := 0; filler < c.preFillerDocs; filler++ {
		mustAddDocument(t, writer, doc)
	}
	for i := 0; i < len(boolean2DocFields); i++ {
		doc.Add(newFieldNoRandom(t, boolean2Field, boolean2DocFields[i], ft))
		mustAddDocument(t, writer, doc)

		doc = document.NewDocument()
		for filler := 0; filler < c.numFillerDocs; filler++ {
			mustAddDocument(t, writer, doc)
		}
	}
	mustClose(t, writer)
	t.Cleanup(func() { c.afterClass(t) })
	c.littleReader = mustOpenDirectoryReader(t, c.directory)
	c.searcher = newSearcher(t, c.littleReader)
	// this is intentionally using the baseline sim, because it compares against bigSearcher (which
	// uses a random one)
	c.searcher.SetSimilarity(search.NewClassicSimilarity())

	// make a copy of our index using a single segment
	if c.numFillerDocs*c.preFillerDocs > 100000 {
		// singleSegmentDirectory = newFSDirectory(createTempDir());
		t.Fatal(newFSDirectoryBlocker)
	}
	c.singleSegmentDirectory = newDirectory()

	// TODO: this test does not need to be doing this crazy stuff. please improve it!
	// for (String fileName : directory.listAll()) { ... singleSegmentDirectory.copyFrom(directory, fileName, fileName, IOContext.DEFAULT); ... }
	t.Fatal(directoryCopyFromBlocker)

	iwc = newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	// we need docID order to be preserved:
	// randomized codecs are sometimes too costly for this test:
	iwc.SetCodec(index.GetDefaultCodec())
	iwc.SetMergePolicy(newLogMergePolicy())
	func() {
		w := mustNewIndexWriter(t, c.singleSegmentDirectory, iwc)
		defer mustClose(t, w)
		if _, err := w.ForceMergeWithObserver(1, true); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}()
	c.singleSegmentReader = mustOpenDirectoryReader(t, c.singleSegmentDirectory)
	c.singleSegmentSearcher = newSearcher(t, c.singleSegmentReader)
	c.singleSegmentSearcher.SetSimilarity(c.searcher.GetSimilarity())

	// Make big index
	c.dir2 = boolean2CopyOf(t, c.directory)

	// First multiply small test index:
	c.mulFactor = 1
	docCount := 0
	if testing.Verbose() {
		t.Log("TEST: now copy index...")
	}
	for {
		if testing.Verbose() {
			t.Log("TEST: cycle...")
		}
		cp := boolean2CopyOf(t, c.dir2)

		iwc = newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
		// randomized codecs are sometimes too costly for this test:
		iwc.SetCodec(index.GetDefaultCodec())
		w := newRandomIndexWriterWithConfig(t, c.dir2, iwc)
		if _, err := w.AddIndexes(cp); err != nil {
			t.Fatalf("addIndexes: %v", err)
		}
		mustClose(t, cp)
		stats, err := w.GetDocStats()
		if err != nil {
			t.Fatal(err)
		}
		docCount = stats.MaxDoc
		mustClose(t, w)
		c.mulFactor *= 2
		if !(docCount < 3000*c.numFillerDocs) {
			break
		}
	}

	iwc = newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	// randomized codecs are sometimes too costly for this test:
	iwc.SetCodec(index.GetDefaultCodec())
	w := newRandomIndexWriterWithConfig(t, c.dir2, iwc)

	doc = document.NewDocument()
	doc.Add(newFieldNoRandom(t, "field2", "xxx", ft))
	for i := 0; i < boolean2NumExtraDocs/2; i++ {
		mustAddDocument(t, w, doc)
	}
	doc = document.NewDocument()
	doc.Add(newFieldNoRandom(t, "field2", "big bad bug", ft))
	for i := 0; i < boolean2NumExtraDocs/2; i++ {
		mustAddDocument(t, w, doc)
	}
	c.reader = mustGetReader(t, w)
	c.bigSearcher = newSearcher(t, c.reader)
	mustClose(t, w)
	return c
}

// afterClass renders afterClass().
func (c *boolean2Class) afterClass(t *testing.T) {
	for _, cl := range []interface{ Close() error }{c.reader, c.littleReader, c.singleSegmentReader, c.dir2, c.directory, c.singleSegmentDirectory} {
		if cl == nil {
			continue
		}
		if err := cl.Close(); err != nil {
			t.Errorf("afterClass: %v", err)
		}
	}
}

// newFieldNoRandom renders new Field(String, String, FieldType).
func newFieldNoRandom(t *testing.T, name, value string, ft *document.FieldType) *document.Field {
	t.Helper()
	f, err := document.NewField(name, value, ft)
	if err != nil {
		t.Fatalf("new Field: %v", err)
	}
	return f
}

// boolean2SearchTopScore renders searcher.search(query, new
// TopScoreDocCollectorManager(numHits, Integer.MAX_VALUE)).
func boolean2SearchTopScore(t *testing.T, s *search.IndexSearcher, query search.Query, numHits int) *search.TopDocs {
	t.Helper()
	collectorManager, err := search.NewTopScoreDocCollectorManager(numHits, nil, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	td, err := search.SearchWithCollectorManager[*search.TopScoreDocCollector, *search.TopDocs](s, query, collectorManager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// queriesTest renders queriesTest(Query, int[]).
func (c *boolean2Class) queriesTest(t *testing.T, query search.Query, expDocNrs []int) {
	t.Helper()
	// adjust the expected doc numbers according to our filler docs
	if 0 < c.numFillerDocs {
		expDocNrs = append([]int(nil), expDocNrs...)
		for i := 0; i < len(expDocNrs); i++ {
			expDocNrs[i] = c.preFillerDocs + ((c.numFillerDocs + 1) * expDocNrs[i])
		}
	}

	topDocsToCheck := atLeast(1000)
	// The asserting searcher will sometimes return the bulk scorer and
	// sometimes return a default impl around the scorer so that we can
	// compare BS1 and BS2
	hits1 := boolean2SearchTopScore(t, c.searcher, query, topDocsToCheck).ScoreDocs
	hits2 := boolean2SearchTopScore(t, c.searcher, query, topDocsToCheck).ScoreDocs

	testsearch.CheckHitsQuery(t, query, hits1, hits2, expDocNrs)

	// Since we have no deleted docs, we should also be able to verify identical matches &
	// scores against an single segment copy of our index
	topDocs := boolean2SearchTopScore(t, c.singleSegmentSearcher, query, topDocsToCheck)
	hits2 = topDocs.ScoreDocs
	testsearch.CheckHitsQuery(t, query, hits1, hits2, expDocNrs)

	// sanity check expected num matches in bigSearcher
	if want, got := int64(c.mulFactor)*topDocs.TotalHits.Value, int64(mustCount(t, c.bigSearcher, query)); want != got {
		t.Fatalf("bigSearcher count: expected %d, got %d", want, got)
	}

	// now check 2 diff scorers from the bigSearcher as well
	hits1 = boolean2SearchTopScore(t, c.bigSearcher, query, topDocsToCheck).ScoreDocs
	hits2 = boolean2SearchTopScore(t, c.bigSearcher, query, topDocsToCheck).ScoreDocs

	// NOTE: just comparing results, not vetting against expDocNrs
	// since we have dups in bigSearcher
	testsearch.CheckEqual(t, query, hits1, hits2)
}

func boolean2Term(text string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(boolean2Field, text))
}

func TestBoolean2Queries01(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.MUST)
	c.queriesTest(t, query.Build(), []int{2, 3})
}

func TestBoolean2Queries02(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.SHOULD)
	c.queriesTest(t, query.Build(), []int{2, 3, 1, 0})
}

func TestBoolean2Queries03(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.SHOULD)
	query.Add(boolean2Term("xx"), search.SHOULD)
	c.queriesTest(t, query.Build(), []int{2, 3, 1, 0})
}

func TestBoolean2Queries04(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.SHOULD)
	query.Add(boolean2Term("xx"), search.MUST_NOT)
	c.queriesTest(t, query.Build(), []int{1, 0})
}

func TestBoolean2Queries05(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.MUST_NOT)
	c.queriesTest(t, query.Build(), []int{1, 0})
}

func TestBoolean2Queries06(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.MUST_NOT)
	query.Add(boolean2Term("w5"), search.MUST_NOT)
	c.queriesTest(t, query.Build(), []int{1})
}

func TestBoolean2Queries07(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST_NOT)
	query.Add(boolean2Term("xx"), search.MUST_NOT)
	query.Add(boolean2Term("w5"), search.MUST_NOT)
	c.queriesTest(t, query.Build(), []int{})
}

func TestBoolean2Queries08(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.SHOULD)
	query.Add(boolean2Term("w5"), search.MUST_NOT)
	c.queriesTest(t, query.Build(), []int{2, 3, 1})
}

func TestBoolean2Queries09(t *testing.T) {
	c := boolean2BeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(boolean2Term("w3"), search.MUST)
	query.Add(boolean2Term("xx"), search.MUST)
	query.Add(boolean2Term("w2"), search.MUST)
	query.Add(boolean2Term("zz"), search.SHOULD)
	c.queriesTest(t, query.Build(), []int{2, 3})
}

// boolean2SearchSorted renders searcher.search(q, new TopFieldCollectorManager(sort, numHits, 1)).
func boolean2SearchSorted(t *testing.T, s *search.IndexSearcher, q search.Query, sort *search.Sort, numHits int) *search.TopFieldDocs {
	t.Helper()
	manager, err := search.NewTopFieldCollectorManagerSimple(sort, numHits, 1)
	if err != nil {
		t.Fatal(err)
	}
	td, err := search.SearchWithCollectorManager(s, q, manager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

func TestBoolean2RandomQueries(t *testing.T) {
	c := boolean2BeforeClass(t)
	vals := []string{"w1", "w2", "w3", "w4", "w5", "xx", "yy", "zzz"}

	var q1 *search.BooleanQuery
	defer func() {
		if t.Failed() {
			// For easier debugging
			t.Logf("failed query: %v", q1)
		}
	}()

	// increase number of iterations for more complete testing
	num := atLeast(3)
	for i := 0; i < num; i++ {
		level := random().Intn(3)
		q1 = boolean2RandBoolQuery(rand.New(rand.NewSource(random().Int63())), random().Intn(2) == 0, level, boolean2Field, vals, nil).Build()

		// Can't sort by relevance since floating point numbers may not quite
		// match up.
		sort := search.INDEXORDER

		queryUtilsCheckSearcher(t, q1, c.searcher) // baseline sim
		func() {
			// a little hackish, QueryUtils.check is too costly to do on bigSearcher in this loop.
			c.searcher.SetSimilarity(c.bigSearcher.GetSimilarity())       // random sim
			defer c.searcher.SetSimilarity(search.NewClassicSimilarity()) // restore
			queryUtilsCheckSearcher(t, q1, c.searcher)
		}()

		// check diff (randomized) scorers (from AssertingSearcher) produce the same results
		hits1 := boolean2SearchSorted(t, c.searcher, q1, sort, 1000).ScoreDocs
		topDocs := boolean2SearchSorted(t, c.searcher, q1, sort, 1000)
		hits2 := topDocs.ScoreDocs
		testsearch.CheckEqual(t, q1, hits1, hits2)

		q3 := search.NewBooleanQueryBuilder()
		q3.Add(q1, search.SHOULD)
		q3.Add(search.NewPrefixQuery(index.NewTerm("field2", "b")), search.SHOULD)
		if want, got := int64(c.mulFactor)*topDocs.TotalHits.Value+boolean2NumExtraDocs/2, int64(mustCount(t, c.bigSearcher, q3.Build())); want != got {
			t.Fatalf("bigSearcher count: expected %d, got %d", want, got)
		}

		// test diff (randomized) scorers produce the same results on bigSearcher as well
		hits1 = boolean2SearchSorted(t, c.bigSearcher, q1, sort, c.mulFactor).ScoreDocs
		hits2 = boolean2SearchSorted(t, c.bigSearcher, q1, sort, c.mulFactor).ScoreDocs
		testsearch.CheckEqual(t, q1, hits1, hits2)
	}

	// System.out.println("Total hits:"+tot);
}

// boolean2Callback renders the interface Callback: used to set properties or
// change every BooleanQuery generated from randBoolQuery.
type boolean2Callback interface {
	postCreate(q *search.BooleanQueryBuilder)
}

// boolean2RandBoolQuery renders the public static randBoolQuery(Random,
// boolean, int, String, String[], Callback). Random rnd is passed in so that
// the exact same random query may be created more than once.
func boolean2RandBoolQuery(rnd *rand.Rand, allowMust bool, level int, field string, vals []string, cb boolean2Callback) *search.BooleanQueryBuilder {
	current := search.NewBooleanQueryBuilder()
	for i := 0; i < rnd.Intn(len(vals))+1; i++ {
		qType := 0 // term query
		if level > 0 {
			qType = rnd.Intn(10)
		}
		var q search.Query
		if qType < 3 {
			q = search.NewTermQuery(index.NewTerm(field, vals[rnd.Intn(len(vals))]))
		} else if qType < 4 {
			t1 := vals[rnd.Intn(len(vals))]
			t2 := vals[rnd.Intn(len(vals))]
			q = search.NewPhraseQuery(10, field, t1, t2) // slop increases possibility of matching
		} else if qType < 7 {
			q = search.NewWildcardQuery(index.NewTerm(field, "w*"))
		} else {
			q = boolean2RandBoolQuery(rnd, allowMust, level-1, field, vals, cb).Build()
		}

		r := rnd.Intn(10)
		var occur search.Occur
		if r < 2 {
			occur = search.MUST_NOT
		} else if r < 5 {
			if allowMust {
				occur = search.MUST
			} else {
				occur = search.SHOULD
			}
		} else {
			occur = search.SHOULD
		}

		current.Add(q, occur)
	}
	if cb != nil {
		cb.postCreate(current)
	}
	return current
}
