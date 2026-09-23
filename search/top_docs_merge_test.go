// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestTopDocsMerge.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// shardSearcher renders the private static ShardSearcher, an IndexSearcher
// over the parent context that searches a single leaf.
type shardSearcher struct {
	*search.IndexSearcher
	ctx *index.LeafReaderContext
}

// newShardSearcher renders new ShardSearcher(LeafReaderContext, IndexReaderContext).
func newShardSearcher(ctx *index.LeafReaderContext, parent index.IndexReaderContext) *shardSearcher {
	return &shardSearcher{IndexSearcher: search.NewIndexSearcherFromContext(parent), ctx: ctx}
}

// searchCollector renders search(Weight, Collector).
func (s *shardSearcher) searchCollector(weight search.Weight, collector search.Collector) error {
	return search.IndexSearcherSearchLeaf(s.IndexSearcher, s.ctx, 0, search.NO_MORE_DOCS, weight, collector)
}

// searchTopN renders search(Weight, int).
func (s *shardSearcher) searchTopN(weight search.Weight, topN int) (*search.TopDocs, error) {
	manager, err := search.NewTopScoreDocCollectorManager(topN, nil, math.MaxInt32)
	if err != nil {
		return nil, err
	}
	collector, err := manager.NewCollector()
	if err != nil {
		return nil, err
	}
	if err := search.IndexSearcherSearchLeaf(s.IndexSearcher, s.ctx, 0, search.NO_MORE_DOCS, weight, collector); err != nil {
		return nil, err
	}
	return collector.TopDocs(), nil
}

// String renders toString().
func (s *shardSearcher) String() string {
	return "ShardSearcher(" + fmt.Sprint(s.ctx) + ")"
}

func TestTopDocsMergeSort_1(t *testing.T) {
	topDocsMergeTestSort(t, false)
}

func TestTopDocsMergeSort_2(t *testing.T) {
	topDocsMergeTestSort(t, true)
}

func TestTopDocsMergeInconsistentTopDocsFail(t *testing.T) {
	topDocs := []*search.TopDocs{
		search.NewTopDocs(search.NewTotalHits(1, search.EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(1, 1.0, 5)}),
		search.NewTopDocs(search.NewTotalHits(1, search.EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(1, 1.0, -1)}),
	}
	if random().Intn(2) == 0 {
		topDocs[0], topDocs[1] = topDocs[1], topDocs[0]
	}
	// expectThrows(IllegalArgumentException.class, () -> TopDocs.merge(0, 2, topDocs));
	expectThrowsPanic(t, func() {
		search.Merge(0, 2, topDocs, nil)
	})
}

func TestTopDocsMergePreAssignedShardIndex(t *testing.T) {
	useConstantScore := random().Intn(2) == 0
	numTopDocs := 2 + random().Intn(10)
	topDocs := make([]*search.TopDocs, 0, numTopDocs)
	shardResultMapping := map[int]*search.TopDocs{}
	numHitsTotal := 0
	for i := 0; i < numTopDocs; i++ {
		numHits := 1 + random().Intn(10)
		numHitsTotal += numHits
		scoreDocs := make([]*search.ScoreDoc, numHits)
		for j := range scoreDocs {
			score := float32(1.0)
			if !useConstantScore {
				score = random().Float32()
			}
			// we set the shard index to index in the list here but shuffle the entire list below
			scoreDocs[j] = search.NewScoreDoc((100*i)+j, score, i)
		}
		topDocs = append(topDocs, search.NewTopDocs(search.NewTotalHits(int64(numHits), search.EQUAL_TO), scoreDocs))
		shardResultMapping[i] = topDocs[i]
	}
	// shuffle the entire thing such that we don't get 1 to 1 mapping of shard index to index in the
	// array
	// -- well likely ;)
	random().Shuffle(len(topDocs), func(i, j int) { topDocs[i], topDocs[j] = topDocs[j], topDocs[i] })
	from := random().Intn(numHitsTotal - 1)
	size := 1 + random().Intn(numHitsTotal-from)

	// passing false here means TopDocs.merge uses the incoming ScoreDoc.shardIndex
	// that we already set, instead of the position of that TopDocs in the array:
	merge := search.Merge(from, size, append([]*search.TopDocs(nil), topDocs...), nil)

	if len(merge.ScoreDocs) == 0 {
		t.Fatal("expected merged hits")
	}
	for _, scoreDoc := range merge.ScoreDocs {
		if scoreDoc.ShardIndex == -1 {
			t.Fatal("unexpected unset shard index")
		}
		shardTopDocs := shardResultMapping[scoreDoc.ShardIndex]
		if shardTopDocs == nil {
			t.Fatalf("no shard %d", scoreDoc.ShardIndex)
		}
		found := false
		for _, shardScoreDoc := range shardTopDocs.ScoreDocs {
			if shardScoreDoc == scoreDoc {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("doc %d not found in its shard", scoreDoc.Doc)
		}
	}

	// now ensure merge is stable even if we use our own shard IDs
	random().Shuffle(len(topDocs), func(i, j int) { topDocs[i], topDocs[j] = topDocs[j], topDocs[i] })
	merge2 := search.Merge(from, size, append([]*search.TopDocs(nil), topDocs...), nil)
	// assertArrayEquals(merge.scoreDocs, merge2.scoreDocs): ScoreDoc has no
	// equals, so the arrays are compared element by element by identity.
	if len(merge.ScoreDocs) != len(merge2.ScoreDocs) {
		t.Fatalf("merge sizes differ: %d vs %d", len(merge.ScoreDocs), len(merge2.ScoreDocs))
	}
	for i := range merge.ScoreDocs {
		if merge.ScoreDocs[i] != merge2.ScoreDocs[i] {
			t.Fatalf("hit %d differs", i)
		}
	}
}

// topDocsMergeTestSort renders testSort(boolean useFrom).
func topDocsMergeTestSort(t *testing.T, useFrom bool) {
	numDocs := atLeast(100) // TEST_NIGHTLY ? atLeast(1000) : atLeast(100)

	tokens := []string{"a", "b", "c", "d", "e"}

	if testing.Verbose() {
		t.Log("TEST: make index")
	}

	dir := newDirectory()
	var reader *index.DirectoryReader
	{
		w := newRandomIndexWriter(t, dir)
		// w.setDoRandomForceMerge(false);

		// w.w.getConfig().setMaxBufferedDocs(atLeast(100));

		content := make([]string, atLeast(20))

		for contentIDX := range content {
			var sb strings.Builder
			numTokens := nextInt(1, 10)
			for tokenIDX := 0; tokenIDX < numTokens; tokenIDX++ {
				sb.WriteString(tokens[random().Intn(len(tokens))])
				sb.WriteByte(' ')
			}
			content[contentIDX] = sb.String()
		}

		for docIDX := 0; docIDX < numDocs; docIDX++ {
			doc := document.NewDocument()
			sdv, err := document.NewSortedDocValuesField("string", []byte(randomRealisticUnicodeString(t)))
			if err != nil {
				t.Fatal(err)
			}
			doc.Add(sdv)
			doc.Add(newTextField(t, "text", content[random().Intn(len(content))], false))
			fdv, err := document.NewFloatDocValuesField("float", random().Float32())
			if err != nil {
				t.Fatal(err)
			}
			doc.Add(fdv)
			var intValue int32
			if random().Intn(100) == 17 {
				intValue = math.MinInt32
			} else if random().Intn(100) == 17 {
				intValue = math.MaxInt32
			} else {
				intValue = int32(random().Uint32())
			}
			ndv, err := document.NewNumericDocValuesField("int", int64(intValue))
			if err != nil {
				t.Fatal(err)
			}
			doc.Add(ndv)
			if testing.Verbose() {
				t.Logf("  doc=%v", doc)
			}
			mustAddDocument(t, w, doc)
		}

		reader = mustGetReader(t, w)
		mustClose(t, w)
	}

	// NOTE: sometimes reader has just one segment, which is
	// important to test
	searcher := newSearcher(t, reader)
	ctx := searcher.GetTopReaderContext()

	var subSearchers []*shardSearcher
	var docStarts []int

	if leafCtx, ok := ctx.(*index.LeafReaderContext); ok {
		subSearchers = []*shardSearcher{newShardSearcher(leafCtx, ctx)}
		docStarts = []int{0}
	} else {
		compCTX := ctx.(*index.CompositeReaderContext)
		leaves, err := compCTX.Leaves()
		if err != nil {
			t.Fatal(err)
		}
		size := len(leaves)
		subSearchers = make([]*shardSearcher, size)
		docStarts = make([]int, size)
		docBase := 0
		for searcherIDX := range subSearchers {
			leave := leaves[searcherIDX]
			subSearchers[searcherIDX] = newShardSearcher(leave, compCTX)
			docStarts[searcherIDX] = docBase
			docBase += leave.LeafReader().MaxDoc()
		}
	}

	sortFields := []*search.SortField{
		search.NewSortFieldWithReverse("string", spi.SortFieldTypeString, true),
		search.NewSortFieldWithReverse("string", spi.SortFieldTypeString, false),
		search.NewSortFieldWithReverse("int", spi.SortFieldTypeInt, true),
		search.NewSortFieldWithReverse("int", spi.SortFieldTypeInt, false),
		search.NewSortFieldWithReverse("float", spi.SortFieldTypeFloat, true),
		search.NewSortFieldWithReverse("float", spi.SortFieldTypeFloat, false),
		search.NewSortFieldWithReverse("", spi.SortFieldTypeScore, true),
		search.NewSortFieldWithReverse("", spi.SortFieldTypeScore, false),
		search.NewSortFieldWithReverse("", spi.SortFieldTypeDoc, true),
		search.NewSortFieldWithReverse("", spi.SortFieldTypeDoc, false),
	}

	numIters := atLeast(300)
	for iter := 0; iter < numIters; iter++ {

		// TODO: custom FieldComp...
		query := search.NewTermQuery(index.NewTerm("text", tokens[random().Intn(len(tokens))]))

		var sort *search.Sort
		if random().Intn(10) == 4 {
			// Sort by score
			sort = nil
		} else {
			randomSortFields := make([]*search.SortField, nextInt(1, 3))
			for sortIDX := range randomSortFields {
				randomSortFields[sortIDX] = sortFields[random().Intn(len(sortFields))]
			}
			sort = search.NewSort(randomSortFields...)
		}

		numHits := nextInt(1, numDocs+5)
		// final int numHits = 5;

		if testing.Verbose() {
			t.Logf("TEST: search query=%v sort=%v numHits=%d", query, sort, numHits)
		}

		from := -1
		size := -1
		// First search on whole index:
		var topHits *search.TopDocs
		var topHitsFieldDocs []*search.FieldDoc
		if sort == nil {
			if useFrom {

				from = nextInt(0, numHits-1)
				size = numHits - from
				manager, err := search.NewTopScoreDocCollectorManager(numHits, nil, math.MaxInt32)
				if err != nil {
					t.Fatal(err)
				}
				tempTopHits, err := search.SearchWithCollectorManager[*search.TopScoreDocCollector, *search.TopDocs](searcher, query, manager)
				if err != nil {
					t.Fatal(err)
				}
				if from < len(tempTopHits.ScoreDocs) {
					// Can't use TopDocs#topDocs(start, howMany), since it has different behaviour when
					// start >= hitCount
					// than TopDocs#merge currently has
					newScoreDocs := make([]*search.ScoreDoc, min(size, len(tempTopHits.ScoreDocs)-from))
					copy(newScoreDocs, tempTopHits.ScoreDocs[from:])
					tempTopHits.ScoreDocs = newScoreDocs
					topHits = tempTopHits
				} else {
					topHits = search.NewTopDocs(tempTopHits.TotalHits, []*search.ScoreDoc{})
				}
			} else {
				topHits = mustSearch(t, searcher, query, numHits)
			}
		} else {
			manager, err := search.NewTopFieldCollectorManagerSimple(sort, numHits, math.MaxInt32)
			if err != nil {
				t.Fatal(err)
			}
			topFieldDocs, err := search.SearchWithCollectorManager[*search.TopFieldCollector, *search.TopFieldDocs](searcher, query, manager)
			if err != nil {
				t.Fatal(err)
			}
			if useFrom {
				from = nextInt(0, numHits-1)
				size = numHits - from
				if from < len(topFieldDocs.ScoreDocs) {
					// Can't use TopDocs#topDocs(start, howMany), since it has different behaviour when
					// start >= hitCount
					// than TopDocs#merge currently has
					newFieldDocs := make([]*search.FieldDoc, min(size, len(topFieldDocs.FieldDocs)-from))
					copy(newFieldDocs, topFieldDocs.FieldDocs[from:])
					topFieldDocs = search.NewTopFieldDocsWithFieldDocs(topFieldDocs.TotalHits, newFieldDocs, topFieldDocs.Fields)
					topHits = topFieldDocs.TopDocs
					topHitsFieldDocs = topFieldDocs.FieldDocs
				} else {
					topHits = search.NewTopDocs(topFieldDocs.TotalHits, []*search.ScoreDoc{})
				}
			} else {
				topHits = topFieldDocs.TopDocs
				topHitsFieldDocs = topFieldDocs.FieldDocs
			}
		}

		if testing.Verbose() {
			if useFrom {
				t.Logf("from=%d size=%d", from, size)
			}
			t.Logf("  top search: %d totalHits; hits=%d", topHits.TotalHits.Value, len(topHits.ScoreDocs))
			for _, sd := range topHits.ScoreDocs {
				t.Logf("    doc=%d score=%v", sd.Doc, sd.Score)
			}
		}

		// ... then all shards:
		w := mustCreateWeight(t, searcher, mustRewrite(t, searcher, query), search.COMPLETE, 1)

		shardHits := make([]*search.TopDocs, len(subSearchers))
		var shardFieldHits []*search.TopFieldDocs
		if sort != nil {
			shardFieldHits = make([]*search.TopFieldDocs, len(subSearchers))
		}
		for shardIDX, subSearcher := range subSearchers {
			var subHits *search.TopDocs
			if sort == nil {
				var err error
				subHits, err = subSearcher.searchTopN(w, numHits)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				manager, err := search.NewTopFieldCollectorManager(sort, numHits, nil, math.MaxInt32)
				if err != nil {
					t.Fatal(err)
				}
				c, err := manager.NewCollector()
				if err != nil {
					t.Fatal(err)
				}
				if err := subSearcher.searchCollector(w, c); err != nil {
					t.Fatal(err)
				}
				// subHits = c.topDocs(0, numHits): TopFieldCollector.topDocs
				// returns the covariant TopFieldDocs, rendered by TopFieldDocs();
				// the collector holds at most numHits hits, so the window
				// [0, numHits) is the whole result.
				fieldHits := c.TopFieldDocs()
				shardFieldHits[shardIDX] = fieldHits
				subHits = fieldHits.TopDocs
			}

			for i := range subHits.ScoreDocs {
				subHits.ScoreDocs[i].ShardIndex = shardIDX
			}

			shardHits[shardIDX] = subHits
			if testing.Verbose() {
				t.Logf("  shard=%d %d totalHits hits=%d", shardIDX, subHits.TotalHits.Value, len(subHits.ScoreDocs))
				for _, sd := range subHits.ScoreDocs {
					t.Logf("    doc=%d score=%v", sd.Doc, sd.Score)
				}
			}
		}

		// Merge:
		var mergedHits *search.TopDocs
		var mergedFieldDocs []*search.FieldDoc
		if useFrom {
			if sort == nil {
				mergedHits = search.Merge(from, size, shardHits, nil)
			} else {
				merged, err := search.MergeSort(sort, from, size, shardFieldHits)
				if err != nil {
					t.Fatal(err)
				}
				mergedHits, mergedFieldDocs = merged.TopDocs, merged.FieldDocs
			}
		} else {
			if sort == nil {
				mergedHits = search.MergeSimple(numHits, shardHits)
			} else {
				merged, err := search.MergeSort(sort, 0, numHits, shardFieldHits)
				if err != nil {
					t.Fatal(err)
				}
				mergedHits, mergedFieldDocs = merged.TopDocs, merged.FieldDocs
			}
		}

		if mergedHits.ScoreDocs != nil {
			// Make sure the returned shards are correct:
			for _, sd := range mergedHits.ScoreDocs {
				if want := index.ReaderUtilSubIndex(sd.Doc, docStarts); want != sd.ShardIndex {
					t.Fatalf("doc=%d wrong shard: expected %d, got %d", sd.Doc, want, sd.ShardIndex)
				}
			}
		}

		assertConsistent(t, topHits, topHitsFieldDocs, mergedHits, mergedFieldDocs)
	}
	mustClose(t, reader, dir)
}

// assertConsistent renders TestUtil.assertConsistent(TopDocs, TopDocs). A Go
// TopDocs carries its FieldDocs beside ScoreDocs; a nil FieldDoc slice renders
// scoreDocs that are not FieldDoc instances.
func assertConsistent(t *testing.T, expected *search.TopDocs, expectedFD []*search.FieldDoc, actual *search.TopDocs, actualFD []*search.FieldDoc) {
	t.Helper()
	if (expected.TotalHits.Value == 0) != (actual.TotalHits.Value == 0) {
		t.Fatalf("wrong total hits: expected %d, actual %d", expected.TotalHits.Value, actual.TotalHits.Value)
	}
	if expected.TotalHits.Relation == search.EQUAL_TO {
		if actual.TotalHits.Relation == search.EQUAL_TO {
			if actual.TotalHits.Value != expected.TotalHits.Value {
				t.Fatalf("wrong total hits: expected %d, actual %d", expected.TotalHits.Value, actual.TotalHits.Value)
			}
		} else if actual.TotalHits.Value > expected.TotalHits.Value {
			t.Fatalf("wrong total hits: expected <= %d, actual %d", expected.TotalHits.Value, actual.TotalHits.Value)
		}
	} else if actual.TotalHits.Relation == search.EQUAL_TO {
		if actual.TotalHits.Value < expected.TotalHits.Value {
			t.Fatalf("wrong total hits: expected >= %d, actual %d", expected.TotalHits.Value, actual.TotalHits.Value)
		}
	}
	if len(actual.ScoreDocs) != len(expected.ScoreDocs) {
		t.Fatalf("wrong hit count: expected %d, actual %d", len(expected.ScoreDocs), len(actual.ScoreDocs))
	}
	for hitIDX := range expected.ScoreDocs {
		expectedSD := expected.ScoreDocs[hitIDX]
		actualSD := actual.ScoreDocs[hitIDX]
		if actualSD.Doc != expectedSD.Doc {
			t.Fatalf("wrong hit docID: expected %d, actual %d", expectedSD.Doc, actualSD.Doc)
		}
		if expectedSD.Score != actualSD.Score {
			t.Fatalf("wrong hit score: expected %v, actual %v", expectedSD.Score, actualSD.Score)
		}
		if expectedFD != nil {
			if actualFD == nil {
				t.Fatal("actual hit is not a FieldDoc")
			}
			if !reflect.DeepEqual(expectedFD[hitIDX].Fields, actualFD[hitIDX].Fields) {
				t.Fatalf("wrong sort field values: expected %v, actual %v", expectedFD[hitIDX].Fields, actualFD[hitIDX].Fields)
			}
		} else if actualFD != nil {
			t.Fatal("actual hit is a FieldDoc")
		}
	}
}

func TestTopDocsMergeMergeTotalHitsRelation(t *testing.T) {
	topDocs1 := search.NewTopDocs(search.NewTotalHits(2, search.EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(42, 2, 0)})
	topDocs2 := search.NewTopDocs(search.NewTotalHits(1, search.EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(42, 2, 1)})
	topDocs3 := search.NewTopDocs(search.NewTotalHits(1, search.GREATER_THAN_OR_EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(42, 2, 2)})
	topDocs4 := search.NewTopDocs(search.NewTotalHits(3, search.GREATER_THAN_OR_EQUAL_TO), []*search.ScoreDoc{search.NewScoreDoc(42, 2, 3)})

	assertTotalHits := func(expected, actual *search.TotalHits) {
		t.Helper()
		if expected.Value != actual.Value || expected.Relation != actual.Relation {
			t.Fatalf("totalHits = %v, want %v", actual, expected)
		}
	}

	merged1 := search.MergeSimple(1, []*search.TopDocs{topDocs1, topDocs2})
	assertTotalHits(search.NewTotalHits(3, search.EQUAL_TO), merged1.TotalHits)

	merged2 := search.MergeSimple(1, []*search.TopDocs{topDocs1, topDocs3})
	assertTotalHits(search.NewTotalHits(3, search.GREATER_THAN_OR_EQUAL_TO), merged2.TotalHits)

	merged3 := search.MergeSimple(1, []*search.TopDocs{topDocs3, topDocs4})
	assertTotalHits(search.NewTotalHits(4, search.GREATER_THAN_OR_EQUAL_TO), merged3.TotalHits)

	merged4 := search.MergeSimple(1, []*search.TopDocs{topDocs4, topDocs2})
	assertTotalHits(search.NewTotalHits(4, search.GREATER_THAN_OR_EQUAL_TO), merged4.TotalHits)
}
