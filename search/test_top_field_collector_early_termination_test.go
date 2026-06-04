// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestTopFieldCollectorEarlyTermination.java
//
// canEarlyTerminate (the two unit cases below) is fully ported and passes: it
// exercises TopFieldCollector.CanEarlyTerminate against the same search-sort /
// index-sort prefix combinations the upstream test asserts.
//
// The two doTestEarlyTermination cases assert the *behaviour* of early
// termination: a TopFieldCollectorManager built with totalHitsThreshold == 1
// must, when the slice holds more live documents than the requested hit count,
// stop collecting early and report TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO
// while still returning the same top hits as an exhaustive (threshold MAX) run.
//
// Feature gap: Gocene's TopFieldCollector does not yet implement competitive-hit
// skipping / early termination — totalHitsThreshold is recorded but inert (see
// top_field_collector.go and rmp #130), so the collector always scans every
// matching document and always reports EQUAL_TO. The early-termination
// assertions below therefore FAIL honestly at runtime (no t.Skip, real expected
// Lucene behaviour asserted). Gocene's single-collector search drives one logical
// slice spanning all segments, so maxSliceSize equals the total live-doc count.
package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
)

// earlyTermSort is the LONG sort on "ndv1" shared by the doTest cases.
func earlyTermSort() *search.Sort {
	return search.NewSort(search.NewSortField("ndv1", search.SortFieldTypeLong))
}

func TestTopFieldCollectorEarlyTermination_EarlyTermination(t *testing.T) {
	doTestEarlyTermination(t, false)
}

func TestTopFieldCollectorEarlyTermination_EarlyTerminationWhenPaging(t *testing.T) {
	doTestEarlyTermination(t, true)
}

// doTestEarlyTermination mirrors TestTopFieldCollectorEarlyTermination.doTestEarlyTermination.
func doTestEarlyTermination(t *testing.T, paging bool) {
	rng := rand.New(rand.NewSource(20260601))
	ix := newIntegrationIndex(t)
	numDocs := 200
	terms := []string{"aa", "bb", "cc", "dd", "ee"}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		ndv1, err := document.NewNumericDocValuesField("ndv1", int64(rng.Intn(10)))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField(ndv1): %v", err)
		}
		doc.Add(ndv1)
		ndv2, err := document.NewNumericDocValuesField("ndv2", int64(rng.Intn(10)))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField(ndv2): %v", err)
		}
		doc.Add(ndv2)
		sf, err := document.NewStringField("s", terms[rng.Intn(len(terms))], true)
		if err != nil {
			t.Fatalf("NewStringField(s): %v", err)
		}
		doc.Add(sf)
		ix.addDoc(doc)
		if i == numDocs/2 || (i != numDocs-1 && rng.Intn(8) == 0) {
			ix.commit()
		}
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	sort := earlyTermSort()
	// Gocene drives a single logical slice across all segments, so the maximum
	// slice size is the total live-doc count.
	maxSliceSize := searcher.GetIndexReader().NumDocs()
	numHits := 1 + rng.Intn(maxSliceSize)

	var after *search.FieldDoc
	if paging {
		td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
		if err != nil {
			t.Fatalf("SearchWithSort(paging seed): %v", err)
		}
		// SearchWithSort returns *TopFieldDocs whose FieldDocs carry the per-hit
		// sort values; the Lucene original casts the last scoreDoc to FieldDoc
		// ((FieldDoc) td.scoreDocs[td.scoreDocs.length - 1]). In Gocene the typed
		// FieldDoc lives in the parallel FieldDocs slice.
		after = td.FieldDocs[len(td.FieldDocs)-1]
	}

	query := search.Query(search.NewMatchAllDocsQuery())
	if rng.Intn(2) == 0 {
		query = search.NewTermQuery(index.NewTerm("s", terms[rng.Intn(len(terms))]))
	}

	td1 := runFieldManager(t, searcher, query, sort, numHits, after, 1<<30)
	td2 := runFieldManager(t, searcher, query, sort, numHits, after, 1)

	if td1.TotalHits.Relation == search.GREATER_THAN_OR_EQUAL_TO {
		t.Errorf("threshold-MAX run reported GREATER_THAN_OR_EQUAL_TO, want EQUAL_TO")
	}

	_, isMatchAll := query.(*search.MatchAllDocsQuery)
	if !paging && maxSliceSize > numHits && isMatchAll {
		// The threshold-1 run must sometimes early terminate.
		if td2.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Errorf("threshold-1 MatchAll run relation = %v, want GREATER_THAN_OR_EQUAL_TO (early termination)",
				td2.TotalHits.Relation)
		}
	}

	if td2.TotalHits.Relation == search.GREATER_THAN_OR_EQUAL_TO {
		if td2.TotalHits.Value < int64(len(td1.ScoreDocs)) {
			t.Errorf("td2.totalHits.value %d < td1.scoreDocs.length %d", td2.TotalHits.Value, len(td1.ScoreDocs))
		}
		if td2.TotalHits.Value > int64(searcher.GetIndexReader().MaxDoc()) {
			t.Errorf("td2.totalHits.value %d > maxDoc %d", td2.TotalHits.Value, searcher.GetIndexReader().MaxDoc())
		}
	} else if td2.TotalHits.Value != td1.TotalHits.Value {
		t.Errorf("td2.totalHits.value %d != td1.totalHits.value %d", td2.TotalHits.Value, td1.TotalHits.Value)
	}

	testutil.CheckEqual(t, query, td1.ScoreDocs, td2.ScoreDocs)
}

// runFieldManager runs query under a TopFieldCollectorManager with the given
// totalHitsThreshold and returns the reduced TopFieldDocs as a TopDocs.
func runFieldManager(t *testing.T, s *search.IndexSearcher, query search.Query, sort *search.Sort, numHits int, after *search.FieldDoc, threshold int) *search.TopDocs {
	t.Helper()
	var afterSD *search.ScoreDoc
	if after != nil {
		afterSD = after.ScoreDoc
	}
	mgr, err := search.NewTopFieldCollectorManager(sort, numHits, afterSD, threshold)
	if err != nil {
		t.Fatalf("NewTopFieldCollectorManager: %v", err)
	}
	collector, err := mgr.NewCollector()
	if err != nil {
		t.Fatalf("NewCollector: %v", err)
	}
	if err := s.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
	tfd, err := mgr.Reduce([]*search.TopFieldCollector{collector})
	if err != nil {
		t.Fatalf("Reduce: %v", err)
	}
	return tfd.TopDocs
}

// TestTopFieldCollectorEarlyTermination_CanEarlyTerminateOnDocId mirrors
// testCanEarlyTerminateOnDocId.
func TestTopFieldCollectorEarlyTermination_CanEarlyTerminateOnDocId(t *testing.T) {
	fieldDoc := func() *search.SortField { return &search.SortField{Type: search.SortFieldTypeDoc} }
	longA := func() *search.SortField { return search.NewSortField("a", search.SortFieldTypeLong) }
	longB := func() *search.SortField { return search.NewSortField("b", search.SortFieldTypeLong) }

	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(fieldDoc())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), nil))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), nil))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(longB())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(longB())))
	assertTrue(t, search.CanEarlyTerminate(search.NewSort(fieldDoc()), search.NewSort(longB(), fieldDoc())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA()), search.NewSort(fieldDoc())))
	assertFalse(t, search.CanEarlyTerminate(search.NewSort(longA(), fieldDoc()), search.NewSort(fieldDoc())))
}

// TestTopFieldCollectorEarlyTermination_CanEarlyTerminateOnPrefix mirrors
// testCanEarlyTerminateOnPrefix.
func TestTopFieldCollectorEarlyTermination_CanEarlyTerminateOnPrefix(t *testing.T) {
	longA := func() *search.SortField { return search.NewSortField("a", search.SortFieldTypeLong) }
	strB := func() *search.SortField { return search.NewSortField("b", search.SortFieldTypeString) }
	strC := func() *search.SortField { return search.NewSortField("c", search.SortFieldTypeString) }
	longARev := func() *search.SortField { return search.NewSortFieldReverse("a", search.SortFieldTypeLong) }
	longC := func() *search.SortField { return search.NewSortField("c", search.SortFieldTypeLong) }

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
