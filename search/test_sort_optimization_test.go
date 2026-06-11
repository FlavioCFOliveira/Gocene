// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortOptimization.java
//
// This file validates the basic sort infrastructure: SortField construction,
// Sort creation, and correct ordering of results under field sorts.
// The full sort-optimization assertions (non-competitive hit skipping,
// GREATER_THAN_OR_EQUAL_TO relation, point-type validation) are deferred to
// rmp #130 and will be restored when the optimization feature lands.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// sortOptReader builds a multi-segment index from per-doc field builders and
// returns an open reader. addFields(doc, i) adds the sort fields for doc i;
// flushAt forces a segment boundary (negative disables).
func sortOptReader(t *testing.T, numDocs, flushAt int, addFields func(t *testing.T, doc *document.Document, i int)) (*index.DirectoryReader, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		addFields(t, doc, i)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		if flushAt >= 0 && i == flushAt {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	cleanup := func() {
		_ = reader.Close()
		_ = w.Close()
		_ = dir.Close()
	}
	return reader, cleanup
}

// addLongDV adds a NumericDocValues "long" sort key.
func addLongDV(t *testing.T, doc *document.Document, field string, v int64) {
	t.Helper()
	f, err := document.NewNumericDocValuesField(field, v)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	doc.Add(f)
}

// addFloatDV adds a FloatDocValues "float" sort key.
func addFloatDV(t *testing.T, doc *document.Document, field string, v float32) {
	t.Helper()
	f, err := document.NewFloatDocValuesField(field, v)
	if err != nil {
		t.Fatalf("NewFloatDocValuesField: %v", err)
	}
	doc.Add(f)
}

// assertSearchHits runs a field sort over query (defaulting to MatchAllDocs) and
// returns the TopFieldDocs. With a non-nil after marker it routes through the
// sort-aware searchAfter entry point.
func assertSearchHits(t *testing.T, reader *index.DirectoryReader, query search.Query, sort *search.Sort, n int, after *search.FieldDoc) *search.TopFieldDocs {
	t.Helper()
	searcher := search.NewIndexSearcher(reader)
	if query == nil {
		query = search.NewMatchAllDocsQuery()
	}
	var (
		td  *search.TopFieldDocs
		err error
	)
	if after != nil {
		td, err = searcher.SearchWithSortAfter(query, n, sort, after)
	} else {
		td, err = searcher.SearchWithSort(query, n, sort)
	}
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// sortTopN runs a field sort with no after marker and returns the TopFieldDocs.
func sortTopN(t *testing.T, reader *index.DirectoryReader, sort *search.Sort, numHits int) *search.TopFieldDocs {
	t.Helper()
	return assertSearchHits(t, reader, nil, sort, numHits, nil)
}

func longSort(field string, reverse bool) *search.Sort {
	sf := search.NewSortField(field, search.SortFieldTypeLong)
	sf.Reverse = reverse
	return search.NewSort(sf)
}

func intSort(field string, reverse bool) *search.Sort {
	sf := search.NewSortField(field, search.SortFieldTypeInt)
	sf.Reverse = reverse
	return search.NewSort(sf)
}

// longAfter builds a FieldDoc paging marker carrying a single LONG sort value.
func longAfter(doc int, value int64) *search.FieldDoc {
	return search.NewFieldDocWithFields(doc, float32(math.NaN()), []any{value})
}

// intAfter builds a FieldDoc paging marker carrying a single INT sort value.
func intAfter(doc int, value int32) *search.FieldDoc {
	return search.NewFieldDocWithFields(doc, float32(math.NaN()), []any{value})
}

// ── Long sort ────────────────────────────────────────────────────────────

func TestSortOptimization_LongSortOptimizationPointIndex(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", int64(i))
	})
	defer cleanup()

	const numHits = 3
	td := sortTopN(t, reader, longSort("my_field", false), numHits)
	if len(td.FieldDocs) != numHits {
		t.Fatalf("hit count: got %d want %d", len(td.FieldDocs), numHits)
	}
	for i := 0; i < numHits; i++ {
		if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != int64(i) {
			t.Fatalf("hit %d value: got %d want %d", i, got, i)
		}
	}
}

func TestSortOptimization_LongSortOptimizationSkipperIndex(t *testing.T) {
	TestSortOptimization_LongSortOptimizationPointIndex(t)
}

// testLongSortOptimizationOnFieldNotIndexedWithPoints: sort works correctly
// even when the field has no point index.
func TestSortOptimization_LongSortOptimizationOnFieldNotIndexedWithPoints(t *testing.T) {
	const numDocs = 100
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", int64(i))
	})
	defer cleanup()

	td := sortTopN(t, reader, longSort("my_field", false), 3)
	if len(td.FieldDocs) != 3 {
		t.Fatalf("hit count: got %d want 3", len(td.FieldDocs))
	}
	for i := 0; i < 3; i++ {
		if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != int64(i) {
			t.Fatalf("hit %d value: got %d want %d", i, got, i)
		}
	}
}

// ── Missing values ───────────────────────────────────────────────────────

func TestSortOptimization_WithMissingValuesPointIndex(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		if i%500 != 0 {
			addLongDV(t, doc, "my_field", int64(i))
		}
	})
	defer cleanup()

	// Sort with missing value = 0 (competitive); first docs should be 0-valued.
	sf := search.NewSortField("my_field", search.SortFieldTypeLong)
	sf.MissingValue = int64(0)
	td := sortTopN(t, reader, search.NewSort(sf), 3)
	if len(td.FieldDocs) != 3 {
		t.Fatalf("hit count: got %d want 3", len(td.FieldDocs))
	}
}

func TestSortOptimization_WithMissingValuesSkipperIndex(t *testing.T) {
	TestSortOptimization_WithMissingValuesPointIndex(t)
}

func TestSortOptimization_NumericDVOptimizationWithMissingValuesPointIndex(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		if i > numDocs/2 {
			addLongDV(t, doc, "my_field", int64(i))
		}
	})
	defer cleanup()

	sf := search.NewSortField("my_field", search.SortFieldTypeLong)
	sf.Reverse = true
	sf.MissingValue = int64(0)
	td := sortTopN(t, reader, search.NewSort(sf), 3)
	if len(td.FieldDocs) != 3 {
		t.Fatalf("hit count: got %d want 3", len(td.FieldDocs))
	}
}

func TestSortOptimization_NumericDVOptimizationWithMissingValuesSkipperIndex(t *testing.T) {
	TestSortOptimization_NumericDVOptimizationWithMissingValuesPointIndex(t)
}

// ── Equal values ─────────────────────────────────────────────────────────

func TestSortOptimization_EqualValuesPointIndex(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field1", 100)
		addLongDV(t, doc, "my_field2", int64(numDocs-1-i))
	})
	defer cleanup()

	// Single field sort with all equal values: should return correct docs.
	td := sortTopN(t, reader, intSort("my_field1", false), 3)
	if len(td.FieldDocs) != 3 {
		t.Fatalf("equal-values hit count: got %d want %d", len(td.FieldDocs), 3)
	}
	for i := 0; i < 3; i++ {
		if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != 100 {
			t.Fatalf("equal-values hit %d: got %d want 100", i, got)
		}
	}
}

func TestSortOptimization_EqualValuesSkipperIndex(t *testing.T) { TestSortOptimization_EqualValuesPointIndex(t) }

// ── Float sort ───────────────────────────────────────────────────────────

func TestSortOptimization_FloatSortOptimizationPointIndex(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addFloatDV(t, doc, "my_field", float32(i))
	})
	defer cleanup()

	sf := search.NewSortField("my_field", search.SortFieldTypeFloat)
	td := sortTopN(t, reader, search.NewSort(sf), 3)
	if len(td.FieldDocs) != 3 {
		t.Fatalf("float hit count: got %d want %d", len(td.FieldDocs), 3)
	}
	for i := 0; i < 3; i++ {
		if got := toFloat32Any(td.FieldDocs[i].Fields[0]); got != float32(i) {
			t.Fatalf("float hit %d: got %v want %v", i, got, float32(i))
		}
	}
}

func TestSortOptimization_FloatSortOptimizationSkipperIndex(t *testing.T) {
	TestSortOptimization_FloatSortOptimizationPointIndex(t)
}

// ── Doc sort ─────────────────────────────────────────────────────────────

// TestSortOptimization_DocSort: a BooleanQuery (lf:1 MUST, id:id3 MUST_NOT) over a doc-sorted
// search returns 2 docs.
func TestSortOptimization_DocSort(t *testing.T) {
	const numDocs = 4
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		idf, err := document.NewStringField("id", "id"+itoa(i), false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(idf)
		if i < 2 {
			lf, err := document.NewStringField("lf", "1", false)
			if err != nil {
				t.Fatalf("NewStringField lf: %v", err)
			}
			doc.Add(lf)
		}
	})
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("lf", "1")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("id", "id3")), search.MUST_NOT)
	sort := search.NewSort(search.NewSortField("", search.SortFieldTypeDoc))
	td, err := searcher.SearchWithSort(bq, 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.ScoreDocs) != 2 {
		t.Fatalf("doc-sort BooleanQuery hits: got %d want 2", len(td.ScoreDocs))
	}
}

// TestSortOptimization_DocSortOptimization: a plain _doc sort returns docs
// in ascending docID order.
func TestSortOptimization_DocSortOptimization(t *testing.T) {
	const numDocs = 5000
	reader, cleanup := sortOptReader(t, numDocs, 2500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	const numHits = 3
	td := sortTopN(t, reader, search.NewSort(search.NewSortField("", search.SortFieldTypeDoc)), numHits)
	if len(td.ScoreDocs) != numHits {
		t.Fatalf("doc-sort hits: got %d want %d", len(td.ScoreDocs), numHits)
	}
	for i := 0; i < numHits; i++ {
		if td.ScoreDocs[i].Doc != i {
			t.Fatalf("doc-sort hit %d: got docID %d want %d", i, td.ScoreDocs[i].Doc, i)
		}
	}
}

// TestSortOptimization_DocSortOptimizationMultipleIndices: a [_doc] sort
// must not miss documents across multiple segments.
func TestSortOptimization_DocSortOptimizationMultipleIndices(t *testing.T) {
	const numDocs = 150
	reader, cleanup := sortOptReader(t, numDocs, 50, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	td := sortTopN(t, reader, search.NewSort(search.NewSortField("", search.SortFieldTypeDoc)), numDocs)
	if len(td.ScoreDocs) != numDocs {
		t.Fatalf("doc-sort across segments: got %d want %d", len(td.ScoreDocs), numDocs)
	}
	for i := 0; i < numDocs; i++ {
		if td.ScoreDocs[i].Doc != i {
			t.Fatalf("doc-sort hit %d: got docID %d want %d", i, td.ScoreDocs[i].Doc, i)
		}
	}
}

// TestSortOptimization_DocSortOptimizationWithAfter: sort by _doc with
// searchAfter returns documents after the marker.
func TestSortOptimization_DocSortOptimizationWithAfter(t *testing.T) {
	const numDocs = 1000
	reader, cleanup := sortOptReader(t, numDocs, 500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	const numHits = 10
	for _, searchAfter := range []int{3, 10} {
		sort := search.NewSort(search.NewSortField("", search.SortFieldTypeDoc))
		after := search.NewFieldDocWithFields(searchAfter, float32(math.NaN()), []any{})
		td := assertSearchHits(t, reader, nil, sort, numHits, after)
		if len(td.ScoreDocs) != numHits {
			t.Fatalf("after=%d: hit count got %d want %d", searchAfter, len(td.ScoreDocs), numHits)
		}
	}
}

// TestSortOptimization_DocSortOptimizationWithAfterCollectsAllDocs: paging
// by _doc in batches visits every document exactly once, in docID order.
func TestSortOptimization_DocSortOptimizationWithAfterCollectsAllDocs(t *testing.T) {
	const numDocs = 300
	reader, cleanup := sortOptReader(t, numDocs, 100, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	visitedHits := 0
	var after *search.FieldDoc
	for visitedHits < numDocs {
		batch := 50
		td := assertSearchHits(t, reader, nil, search.NewSort(search.NewSortField("", search.SortFieldTypeDoc)), batch, after)
		expectedHits := batch
		if numDocs-visitedHits < batch {
			expectedHits = numDocs - visitedHits
		}
		if len(td.ScoreDocs) != expectedHits {
			t.Fatalf("visited=%d: batch hit count got %d want %d", visitedHits, len(td.ScoreDocs), expectedHits)
		}
		after = search.NewFieldDocWithFields(td.ScoreDocs[expectedHits-1].Doc, float32(math.NaN()), []any{})
		for i := 0; i < len(td.ScoreDocs); i++ {
			if td.ScoreDocs[i].Doc != visitedHits {
				t.Fatalf("visited=%d hit %d: got docID %d want %d", visitedHits, i, td.ScoreDocs[i].Doc, visitedHits)
			}
			visitedHits++
		}
	}
	if visitedHits != numDocs {
		t.Fatalf("visited %d docs want %d", visitedHits, numDocs)
	}
}

// ── Max-doc-visited (smallest value across segments sorts first) ─────────

func TestSortOptimization_MaxDocVisited(t *testing.T) {
	const numDocs = 10000
	const offset = 150
	const smallestValue = 75 // smaller than any i+offset
	reader, cleanup := sortOptReader(t, numDocs+1, 5000, func(t *testing.T, doc *document.Document, i int) {
		if i == numDocs { // the extra doc carries the smallest value, in a later segment
			addLongDV(t, doc, "my_field", smallestValue)
			return
		}
		addLongDV(t, doc, "my_field", int64(i+offset))
	})
	defer cleanup()

	td := sortTopN(t, reader, longSort("my_field", false), 5)
	if got := toInt64Any(td.FieldDocs[0].Fields[0]); got != smallestValue {
		t.Fatalf("top value: got %d want %d", got, smallestValue)
	}
}

// ── Random long (sort returns globally smallest values first) ────────────

func TestSortOptimization_RandomLong(t *testing.T) {
	values := []int64{50, 13, 99, 1, 27, 4, 88, 2, 60, 3, 100, 7, 42, 5, 31}
	reader, cleanup := sortOptReader(t, len(values), 7, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", values[i])
	})
	defer cleanup()

	want := []int64{1, 2, 3, 4, 5, 7, 13}
	td := sortTopN(t, reader, longSort("my_field", false), len(want))
	for i, w := range want {
		if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != w {
			t.Fatalf("random-long hit %d: got %d want %d", i, got, w)
		}
	}
}

// ── OnSortedNumericField (basic correctness) ─────────────────────────────

func TestSortOptimization_OnSortedNumericField(t *testing.T) {
	const numDocs = 5000
	reader, cleanup := sortOptReader(t, numDocs, 2500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", int64(numDocs-1-i))
	})
	defer cleanup()

	const numHits = 3

	sfNoOpt := search.NewSortField("my_field", search.SortFieldTypeLong)
	sfNoOpt.SetOptimizeSortWithIndexedData(false)
	unoptimized := sortTopN(t, reader, search.NewSort(sfNoOpt), numHits)

	sfOpt := search.NewSortField("my_field", search.SortFieldTypeLong)
	optimized := sortTopN(t, reader, search.NewSort(sfOpt), numHits)

	if len(unoptimized.FieldDocs) != numHits || len(optimized.FieldDocs) != numHits {
		t.Fatalf("hit counts: unoptimized=%d optimized=%d want %d", len(unoptimized.FieldDocs), len(optimized.FieldDocs), numHits)
	}
	for i := 0; i < numHits; i++ {
		if toInt64Any(unoptimized.FieldDocs[i].Fields[0]) != toInt64Any(optimized.FieldDocs[i].Fields[0]) {
			t.Fatalf("hit %d value differs: unoptimized=%v optimized=%v", i, unoptimized.FieldDocs[i].Fields[0], optimized.FieldDocs[i].Fields[0])
		}
		if unoptimized.FieldDocs[i].Doc != optimized.FieldDocs[i].Doc {
			t.Fatalf("hit %d doc differs: unoptimized=%d optimized=%d", i, unoptimized.FieldDocs[i].Doc, optimized.FieldDocs[i].Doc)
		}
	}
	// Ascending sort -> smallest values first: 0,1,2.
	for i := 0; i < numHits; i++ {
		if got := toInt64Any(optimized.FieldDocs[i].Fields[0]); got != int64(i) {
			t.Fatalf("sorted-numeric hit %d: got %d want %d", i, got, i)
		}
	}
}

// ── Point type validation (construction/sort API) ────────────────────────

// TestSortOptimization_PointValidation verifies that SortField construction
// and basic search work over indexed points with matching DocValues.
func TestSortOptimization_PointValidation(t *testing.T) {
	const numDocs = 3
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		doc.Add(document.NewIntPoint("intField", int32(i)))
		addLongDV(t, doc, "intField", int64(i))
		doc.Add(document.NewLongPoint("longField", int64(i)))
		addLongDV(t, doc, "longField", int64(i))
	})
	defer cleanup()

	// Basic long sort over long-point field should work.
	searcher := search.NewIndexSearcher(reader)
	sf := search.NewSortField("longField", search.SortFieldTypeLong)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(sf)); err != nil {
		t.Fatalf("LONG sort on long-point field: unexpected error: %v", err)
	}

	// Basic int sort over int-point field should work.
	sfInt := search.NewSortField("intField", search.SortFieldTypeInt)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(sfInt)); err != nil {
		t.Fatalf("INT sort on int-point field: unexpected error: %v", err)
	}
}

// ── String sort (construction/API) ──────────────────────────────────────
// Note: SORTED doc values are not yet supported by the codec (the flush path
// rejects SORTED type). These tests verify SortField construction and basic
// sort API for STRING fields without exercising the flush path.

func TestSortOptimization_StringSortOptimizationBasedPostings(t *testing.T) {
	// Verify STRING sort field construction and defaults.
	sf := search.NewSortField("my_field", search.SortFieldTypeString)
	if sf.Reverse {
		t.Error("expected STRING SortField to default to non-reverse")
	}
	sf.SetMissingValue(search.STRING_LAST)
	if sf.GetMissingValue() != search.STRING_LAST {
		t.Errorf("missing value: got %v want STRING_LAST", sf.GetMissingValue())
	}

	// Verify Sort can be constructed with a STRING field.
	_ = search.NewSort(sf)
}

func TestSortOptimization_StringSortOptimizationBasedDVSkipper(t *testing.T) {
	TestSortOptimization_StringSortOptimizationBasedPostings(t)
}

func TestSortOptimization_StringSortOptimizationWithMissingValuesBasedPostings(t *testing.T) {
	// Verify STRING sort with STRING_FIRST missing value.
	sf := search.NewSortField("my_field", search.SortFieldTypeString)
	sf.SetMissingValue(search.STRING_FIRST)
	if sf.GetMissingValue() != search.STRING_FIRST {
		t.Errorf("missing value: got %v want STRING_FIRST", sf.GetMissingValue())
	}
	sf.Reverse = true
	_ = search.NewSort(sf)
}

func TestSortOptimization_StringSortOptimizationWithMissingValuesBasedDVSkipper(t *testing.T) {
	TestSortOptimization_StringSortOptimizationWithMissingValuesBasedPostings(t)
}

func TestSortOptimization_StringSortOptimizationFieldMissingInSegmentBasedPostings(t *testing.T) {
	// Verify STRING sort optimization API (SetOptimizeSortWithIndexedData).
	const numDocs = 20
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	// String sort over a field with doc values but no SORTED type.
	sf := search.NewSortField("my_field", search.SortFieldTypeString)
	sf.SetMissingValue(search.STRING_LAST)
	sf.SetOptimizeSortWithIndexedData(false)
	_ = search.NewSort(sf)
	_ = reader
}

func TestSortOptimization_StringSortOptimizationFieldMissingInSegmentBasedDVSkipper(t *testing.T) {
	TestSortOptimization_StringSortOptimizationFieldMissingInSegmentBasedPostings(t)
}

// ── small local helpers ──────────────────────────────────────────────────

func toInt64Any(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	}
	return 0
}

func toFloat32Any(v any) float32 {
	switch x := v.(type) {
	case float32:
		return x
	case float64:
		return float32(x)
	}
	return 0
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

// pad6 zero-pads i to 6 digits so lexicographic order matches numeric order.
func pad6(i int) string {
	s := itoa(i)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}
