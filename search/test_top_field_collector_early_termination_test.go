// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestTopFieldCollectorEarlyTermination.java
// (Apache Lucene 10.5.0).
//
// Java uses one Sort instance both as the index sort and as the search sort;
// Gocene declares the index-side Sort (spi.Sort, aliased in index) apart from
// search.Sort, so the test builds both from the same SortField.

package search_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// tfcetForceMergeMaxSegmentCount renders FORCE_MERGE_MAX_SEGMENT_COUNT.
const tfcetForceMergeMaxSegmentCount = 5

// tfcetTestCase renders the fields of TestTopFieldCollectorEarlyTermination.
type tfcetTestCase struct {
	numDocs   int
	terms     []string
	dir       store.Directory
	sortField *search.SortField
	sort      *search.Sort
	iw        *testindex.RandomIndexWriter
	reader    *index.DirectoryReader
}

func newTfcetTestCase() *tfcetTestCase {
	sf := search.NewSortField("ndv1", spi.SortFieldTypeLong)
	return &tfcetTestCase{sortField: sf, sort: search.NewSort(sf)}
}

// randomDocument renders the private randomDocument().
func (tc *tfcetTestCase) randomDocument(t *testing.T) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	ndv1, err := document.NewNumericDocValuesField("ndv1", int64(random().Intn(10)))
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(ndv1)
	ndv2, err := document.NewNumericDocValuesField("ndv2", int64(random().Intn(10)))
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(ndv2)
	doc.Add(mustStringField(t, "s", tc.terms[random().Intn(len(tc.terms))], true))
	return doc
}

// createRandomIndex renders the private createRandomIndex(boolean).
func (tc *tfcetTestCase) createRandomIndex(t *testing.T, singleSortedSegment bool) {
	t.Helper()
	tc.dir = newDirectory()
	tc.numDocs = atLeast(150)
	numTerms := nextInt(1, tc.numDocs/5)
	randomTerms := map[string]struct{}{}
	for len(randomTerms) < numTerms {
		randomTerms[randomSimpleStringR(random())] = struct{}{}
	}
	tc.terms = tc.terms[:0]
	for term := range randomTerms {
		tc.terms = append(tc.terms, term)
	}
	seed := random().Int63()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer()) // new MockAnalyzer(new Random(seed))
	// if (iwc.getMergePolicy() instanceof MockRandomMergePolicy) -- MockRandomMergePolicy is
	// not part of Gocene's newIndexWriterConfig, so the branch never applies.
	iwc.SetMergeScheduler(index.NewSerialMergeScheduler()) // for reproducible tests
	iwc.SetIndexSort(index.NewSort(tc.sortField))
	iw, err := testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(seed)), tc.dir, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	tc.iw = iw
	tc.iw.SetDoRandomForceMerge(false) // don't do this, it may happen anyway with MockRandomMP
	for i := 0; i < tc.numDocs; i++ {
		doc := tc.randomDocument(t)
		mustAddDocument(t, tc.iw, doc)
		if i == tc.numDocs/2 || (i != tc.numDocs-1 && random().Intn(8) == 0) {
			mustCommit(t, tc.iw)
		}
		if random().Intn(15) == 0 {
			term := tc.terms[random().Intn(len(tc.terms))]
			if _, err := tc.iw.DeleteDocuments(index.NewTerm("s", term)); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
		}
	}
	if singleSortedSegment {
		if err := tc.iw.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	} else if random().Intn(2) == 0 {
		if err := tc.iw.ForceMerge(tfcetForceMergeMaxSegmentCount); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	tc.reader = mustGetReader(t, tc.iw)
	if tc.reader.NumDocs() == 0 {
		mustAddDocument(t, tc.iw, document.NewDocument())
		mustClose(t, tc.reader)
		tc.reader = mustGetReader(t, tc.iw)
	}
}

// closeIndex renders the private closeIndex().
func (tc *tfcetTestCase) closeIndex(t *testing.T) {
	t.Helper()
	mustClose(t, tc.reader, tc.iw, tc.dir)
}

func TestTopFieldCollectorEarlyTerminationEarlyTermination(t *testing.T) {
	newTfcetTestCase().doTestEarlyTermination(t, false)
}

func TestTopFieldCollectorEarlyTerminationEarlyTerminationWhenPaging(t *testing.T) {
	newTfcetTestCase().doTestEarlyTermination(t, true)
}

// doTestEarlyTermination renders the private doTestEarlyTermination(boolean).
func (tc *tfcetTestCase) doTestEarlyTermination(t *testing.T, paging bool) {
	t.Helper()
	iters := atLeast(1)
	for i := 0; i < iters; i++ {
		tc.createRandomIndex(t, false)
		for j := 0; j < iters; j++ {
			searcher := newSearcher(t, tc.reader)
			maxSliceSize := 0
			for _, slice := range searcher.GetSlices() {
				numDocs := 0 // number of live docs in the slice
				for _, partition := range slice.Partitions {
					leaf := partition.Ctx.LeafReader()
					liveDocs := leaf.GetLiveDocs()
					maxDoc := min(partition.MaxDocId, leaf.MaxDoc())
					for doc := partition.MinDocId; doc < maxDoc; doc++ {
						if liveDocs == nil || liveDocs.Get(doc) {
							numDocs++
						}
					}
				}
				maxSliceSize = max(maxSliceSize, numDocs)
			}
			numHits := nextInt(1, tc.numDocs)
			var after *search.FieldDoc
			if paging {
				if searcher.GetIndexReader().NumDocs() <= 0 {
					panic("assert searcher.getIndexReader().numDocs() > 0")
				}
				td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, tc.sort, false)
				if err != nil {
					t.Fatalf("search: %v", err)
				}
				after = td.FieldDocs[len(td.FieldDocs)-1]
			}
			var afterDoc *search.ScoreDoc
			if after != nil {
				afterDoc = after.ScoreDoc
			}
			manager1, err := search.NewTopFieldCollectorManager(tc.sort, numHits, afterDoc, math.MaxInt32)
			if err != nil {
				t.Fatal(err)
			}
			manager2, err := search.NewTopFieldCollectorManager(tc.sort, numHits, afterDoc, 1)
			if err != nil {
				t.Fatal(err)
			}

			var query search.Query
			if random().Intn(2) == 0 {
				query = search.NewTermQuery(index.NewTerm("s", tc.terms[random().Intn(len(tc.terms))]))
			} else {
				query = search.NewMatchAllDocsQuery()
			}
			td1, err := search.SearchWithCollectorManager(searcher, query, manager1)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			td2, err := search.SearchWithCollectorManager(searcher, query, manager2)
			if err != nil {
				t.Fatalf("search: %v", err)
			}

			if td1.TotalHits.Relation == search.GREATER_THAN_OR_EQUAL_TO {
				t.Fatal("assertNotEquals(GREATER_THAN_OR_EQUAL_TO, td1.totalHits.relation())")
			}
			_, isMatchAll := query.(*search.MatchAllDocsQuery)
			if !paging && maxSliceSize > numHits && isMatchAll {
				// Make sure that we sometimes early terminate
				if td2.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
					t.Fatalf("expected td2 relation GREATER_THAN_OR_EQUAL_TO, got %v", td2.TotalHits.Relation)
				}
			}
			if td2.TotalHits.Relation == search.GREATER_THAN_OR_EQUAL_TO {
				if !(td2.TotalHits.Value >= int64(len(td1.ScoreDocs))) {
					t.Fatal("assertTrue(td2.totalHits.value() >= td1.scoreDocs.length)")
				}
				if !(td2.TotalHits.Value <= int64(tc.reader.MaxDoc())) {
					t.Fatal("assertTrue(td2.totalHits.value() <= reader.maxDoc())")
				}
			} else if td2.TotalHits.Value != td1.TotalHits.Value {
				t.Fatalf("totalHits: td2=%d td1=%d", td2.TotalHits.Value, td1.TotalHits.Value)
			}
			testsearch.CheckEqual(t, query, td1.ScoreDocs, td2.ScoreDocs)
		}
		tc.closeIndex(t)
	}
}

func TestTopFieldCollectorEarlyTerminationCanEarlyTerminateOnDocId(t *testing.T) {
	fieldDoc := func() *search.SortField { return &search.SortField{Type: spi.SortFieldTypeDoc} }
	longA := func() *search.SortField { return search.NewSortField("a", spi.SortFieldTypeLong) }
	longB := func() *search.SortField { return search.NewSortField("b", spi.SortFieldTypeLong) }

	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(fieldDoc())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), nil))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), nil))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(longB())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(longB())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(longB(), fieldDoc())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(fieldDoc())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA(), fieldDoc()), search.NewSort(fieldDoc())))
}

func TestTopFieldCollectorEarlyTerminationCanEarlyTerminateOnPrefix(t *testing.T) {
	longA := func() *search.SortField { return search.NewSortField("a", spi.SortFieldTypeLong) }
	strB := func() *search.SortField { return search.NewSortField("b", spi.SortFieldTypeString) }
	strC := func() *search.SortField { return search.NewSortField("c", spi.SortFieldTypeString) }
	longARev := func() *search.SortField { return search.NewSortFieldWithReverse("a", spi.SortFieldTypeLong, true) }
	longC := func() *search.SortField { return search.NewSortField("c", spi.SortFieldTypeLong) }

	assertTrue(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(longA())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(longA(), strB()), search.NewSort(longA(), strB())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(longA(), strB())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longARev()), nil))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longARev()), search.NewSort(longA())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA(), strB()), search.NewSort(longA())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA(), strB()), search.NewSort(longA(), strC())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA(), strB()), search.NewSort(longC(), strB())))
}

func assertTrue(t *testing.T, got bool) {
	t.Helper()
	if !got {
		t.Error("expected canEarlyTerminate = true, got false")
	}
}

func assertFalse(t *testing.T, got bool) {
	t.Helper()
	if got {
		t.Error("expected canEarlyTerminate = false, got true")
	}
}
