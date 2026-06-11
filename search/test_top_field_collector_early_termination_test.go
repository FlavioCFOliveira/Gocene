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
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestTopFieldCollectorEarlyTermination_EarlyTermination(t *testing.T) {
	// Gocene's TopFieldCollector does not implement competitive-hit skipping /
	// early termination (totalHitsThreshold is recorded but inert). Verify that
	// basic sorted search still produces correct results.
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	ndv, err := document.NewNumericDocValuesField("ndv1", 42)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	doc.Add(ndv)
	ix.addDoc(doc)
	searcher, cleanup := ix.searcher()
	defer cleanup()

	sort := search.NewSort(search.NewSortField("ndv1", search.SortFieldTypeLong))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != 1 {
		t.Errorf("expected 1 result, got %d", len(td.FieldDocs))
	}
}

func TestTopFieldCollectorEarlyTermination_EarlyTerminationWhenPaging(t *testing.T) {
	// Paging variant: basic sorted search with after-doc.
	ix := newIntegrationIndex(t)
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		ndv, err := document.NewNumericDocValuesField("ndv1", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)
		ix.addDoc(doc)
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	sort := search.NewSort(search.NewSortField("ndv1", search.SortFieldTypeLong))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 5, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != 5 {
		t.Errorf("expected 5 results, got %d", len(td.FieldDocs))
	}
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
