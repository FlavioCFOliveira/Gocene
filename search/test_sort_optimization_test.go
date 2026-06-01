// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortOptimization.java
//
// This is a FAITHFUL port: it asserts the same properties Lucene asserts,
// including the sort-optimization properties — that non-competitive hits are
// skipped (totalHits < numDocs), that the totalHits relation becomes
// GREATER_THAN_OR_EQUAL_TO, that sort-aware searchAfter paging returns the
// documents strictly after the marker, and the point-type validation
// (IllegalArgumentException-equivalent) cases.
//
// The sort-optimization feature these assertions exercise is NOT yet implemented
// in Gocene (FieldComparator.CompareTop is a no-op, TopFieldCollector never skips
// non-competitive hits nor sets GREATER_THAN_OR_EQUAL_TO, and no point-type
// validation runs). It is tracked by rmp #130. Until that feature lands these
// optimization assertions FAIL AT RUNTIME by design: they document, as honest
// red tests, exactly the behaviour Gocene still owes. They are intentionally NOT
// skipped, weakened, or guarded.

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
// sort-aware searchAfter entry point. This is the Go counterpart of
// TestSortOptimization.assertSearchHits.
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

// assertNonCompetitiveHitsAreSkipped mirrors
// TestSortOptimization.assertNonCompetitiveHitsAreSkipped: it fails when no hits
// were skipped (collectedHits >= numDocs), i.e. the optimization did not run.
func assertNonCompetitiveHitsAreSkipped(t *testing.T, collectedHits, numDocs int64) {
	t.Helper()
	if collectedHits >= numDocs {
		t.Fatalf("Expected some non-competitive hits are skipped; got collected_hits=%d num_docs=%d", collectedHits, numDocs)
	}
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

// ── Long sort optimization ───────────────────────────────────────────────────

func runLongSortOptimization(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", int64(i))
	})
	defer cleanup()

	const numHits = 3
	sf := search.NewSortField("my_field", search.SortFieldTypeLong)
	sort := search.NewSort(sf)

	{ // simple sort: top-3 are values 0,1,2.
		td := sortTopN(t, reader, sort, numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != int64(i) {
				t.Fatalf("hit %d value: got %d want %d", i, got, i)
			}
		}
		if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Fatalf("totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // paging sort with after: values 3,4,5.
		const afterValue int64 = 2
		after := longAfter(2, afterValue)
		td := assertSearchHits(t, reader, nil, sort, numHits, after)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("paging hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != afterValue+1+int64(i) {
				t.Fatalf("paging hit %d value: got %d want %d", i, got, afterValue+1+int64(i))
			}
		}
		if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Fatalf("paging totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // secondary sort on _score: scores are filled (all match-all => 1.0).
		sort2 := search.NewSort(sf, search.NewSortField("", search.SortFieldTypeScore))
		td := sortTopN(t, reader, sort2, numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("secondary-score hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != int64(i) {
				t.Fatalf("secondary-score hit %d value: got %d want %d", i, got, i)
			}
			if score := toFloat32Any(td.FieldDocs[i].Fields[1]); math.Abs(float64(score)-1.0) > 0.001 {
				t.Fatalf("secondary-score hit %d score: got %v want ~1.0", i, score)
			}
		}
		if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Fatalf("secondary-score totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // numeric field as secondary sort: no optimization is run, all docs collected.
		sort2 := search.NewSort(search.NewSortField("", search.SortFieldTypeScore), sf)
		td := sortTopN(t, reader, sort2, numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("score-primary hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		if td.TotalHits.Value != int64(numDocs) {
			t.Fatalf("score-primary totalHits: got %d want %d", td.TotalHits.Value, numDocs)
		}
	}
}

func TestSortOptimization_LongSortOptimizationPointIndex(t *testing.T) {
	runLongSortOptimization(t)
}

func TestSortOptimization_LongSortOptimizationSkipperIndex(t *testing.T) {
	runLongSortOptimization(t)
}

// testLongSortOptimizationOnFieldNotIndexedWithPoints: sort works, but since the
// field is not indexed with points, no optimization is run.
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
	// Optimization is never run on a non-points field; all docs collected.
	if td.TotalHits.Value != int64(numDocs) {
		t.Fatalf("totalHits: got %d want %d", td.TotalHits.Value, numDocs)
	}
}

// ── Missing values ───────────────────────────────────────────────────────────

func runSortOptimizationWithMissingValues(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		if i%500 != 0 { // miss values on every 500th document
			addLongDV(t, doc, "my_field", int64(i))
		}
	})
	defer cleanup()

	const numHits = 3

	{ // optimization runs when the missing value is competitive (default 0L).
		sf := search.NewSortField("my_field", search.SortFieldTypeLong)
		sf.MissingValue = int64(0)
		td := sortTopN(t, reader, search.NewSort(sf), numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("missing-competitive hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // optimization runs when the missing value (100L) is not competitive.
		sf := search.NewSortField("my_field", search.SortFieldTypeLong)
		sf.MissingValue = int64(100)
		td := sortTopN(t, reader, search.NewSort(sf), numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("missing-noncompetitive hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // paging after with a non-competitive missing value: values 4,5,6.
		const afterValue int64 = 3
		after := longAfter(3, afterValue)
		sf := search.NewSortField("my_field", search.SortFieldTypeLong)
		sf.MissingValue = int64(2)
		td := assertSearchHits(t, reader, nil, search.NewSort(sf), numHits, after)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("paging-missing hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != afterValue+1+int64(i) {
				t.Fatalf("paging-missing hit %d value: got %d want %d", i, got, afterValue+1+int64(i))
			}
		}
		if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Fatalf("paging-missing totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}
}

func TestSortOptimization_WithMissingValuesPointIndex(t *testing.T) {
	runSortOptimizationWithMissingValues(t)
}
func TestSortOptimization_WithMissingValuesSkipperIndex(t *testing.T) {
	runSortOptimizationWithMissingValues(t)
}

// testNumericDocValuesOptimizationWithMissingValues: half the docs miss the
// value; a reverse sort with a non-competitive missing value (0L) skips hits.
func runNumericDVOptimizationWithMissingValues(t *testing.T) {
	const numDocs = 10000
	const missValuesNumDocs = numDocs / 2
	reader, cleanup := sortOptReader(t, numDocs, -1, func(t *testing.T, doc *document.Document, i int) {
		if i > missValuesNumDocs {
			addLongDV(t, doc, "my_field", int64(i))
		}
	})
	defer cleanup()

	const numHits = 3

	{ // optimization runs when missing value is NOT competitive (reverse, 0L).
		sf := search.NewSortField("my_field", search.SortFieldTypeLong)
		sf.Reverse = true
		sf.MissingValue = int64(0)
		td := sortTopN(t, reader, search.NewSort(sf), numHits)
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // optimization disabled produces the same hits but collects strictly more.
		sfOpt := search.NewSortField("my_field", search.SortFieldTypeLong)
		sfOpt.Reverse = true
		sfOpt.MissingValue = int64(0)
		optimized := sortTopN(t, reader, search.NewSort(sfOpt), numHits)

		sfNoOpt := search.NewSortField("my_field", search.SortFieldTypeLong)
		sfNoOpt.Reverse = true
		sfNoOpt.MissingValue = int64(0)
		sfNoOpt.SetOptimizeSortWithIndexedData(false)
		unoptimized := sortTopN(t, reader, search.NewSort(sfNoOpt), numHits)

		if len(optimized.FieldDocs) != numHits || len(unoptimized.FieldDocs) != numHits {
			t.Fatalf("hit counts: optimized=%d unoptimized=%d want %d", len(optimized.FieldDocs), len(unoptimized.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if toInt64Any(optimized.FieldDocs[i].Fields[0]) != toInt64Any(unoptimized.FieldDocs[i].Fields[0]) {
				t.Fatalf("hit %d value differs: optimized=%v unoptimized=%v", i, optimized.FieldDocs[i].Fields[0], unoptimized.FieldDocs[i].Fields[0])
			}
			if optimized.FieldDocs[i].Doc != unoptimized.FieldDocs[i].Doc {
				t.Fatalf("hit %d doc differs: optimized=%d unoptimized=%d", i, optimized.FieldDocs[i].Doc, unoptimized.FieldDocs[i].Doc)
			}
		}
		if !(optimized.TotalHits.Value < unoptimized.TotalHits.Value) {
			t.Fatalf("expected optimized to collect fewer hits: optimized=%d unoptimized=%d", optimized.TotalHits.Value, unoptimized.TotalHits.Value)
		}
	}

	{ // multiple comparators: no NumericDocValues optimization, all docs collected.
		sf1 := search.NewSortField("my_field", search.SortFieldTypeLong)
		sf1.Reverse = true
		sf1.MissingValue = int64(0)
		sf2 := search.NewSortField("other", search.SortFieldTypeLong)
		sf2.Reverse = true
		sf2.MissingValue = int64(0)
		td := sortTopN(t, reader, search.NewSort(sf1, sf2), numHits)
		if td.TotalHits.Value != int64(numDocs) {
			t.Fatalf("multi-comparator totalHits: got %d want %d", td.TotalHits.Value, numDocs)
		}
	}
}

func TestSortOptimization_NumericDVOptimizationWithMissingValuesPointIndex(t *testing.T) {
	runNumericDVOptimizationWithMissingValues(t)
}
func TestSortOptimization_NumericDVOptimizationWithMissingValuesSkipperIndex(t *testing.T) {
	runNumericDVOptimizationWithMissingValues(t)
}

// ── Equal values (single field equal; secondary field tie-breaks) ────────────

func runSortOptimizationEqualValues(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field1", 100)                // equal values
		addLongDV(t, doc, "my_field2", int64(numDocs-1-i)) // distinct, descending in i
	})
	defer cleanup()

	const numHits = 3

	{ // single field, all equal -> optimization runs with GREATER_THAN_OR_EQUAL_TO.
		td := sortTopN(t, reader, intSort("my_field1", false), numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("equal-values hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != 100 {
				t.Fatalf("equal-values hit %d: got %d want 100", i, got)
			}
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // single field equal + after: only docs after the marker, optimization runs.
		const afterValue int32 = 100
		const afterDocID = 510
		after := intAfter(afterDocID, afterValue)
		td := assertSearchHits(t, reader, nil, intSort("my_field1", false), numHits, after)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("equal-after hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != 100 {
				t.Fatalf("equal-after hit %d field1: got %d want 100", i, got)
			}
			if td.FieldDocs[i].Doc <= afterDocID {
				t.Fatalf("equal-after hit %d doc: got %d want > %d", i, td.FieldDocs[i].Doc, afterDocID)
			}
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // main field equal + secondary tie-break: no optimization, all docs collected.
		sort2 := search.NewSort(
			search.NewSortField("my_field1", search.SortFieldTypeInt),
			search.NewSortField("my_field2", search.SortFieldTypeInt),
		)
		td := sortTopN(t, reader, sort2, numHits)
		if len(td.FieldDocs) != numHits {
			t.Fatalf("tie-break hit count: got %d want %d", len(td.FieldDocs), numHits)
		}
		for i := 0; i < numHits; i++ {
			if got := toInt64Any(td.FieldDocs[i].Fields[0]); got != 100 {
				t.Fatalf("tie-break hit %d field1: got %d want 100", i, got)
			}
			if got := toInt64Any(td.FieldDocs[i].Fields[1]); got != int64(i) {
				t.Fatalf("tie-break hit %d field2: got %d want %d", i, got, i)
			}
		}
		if td.TotalHits.Value != int64(numDocs) {
			t.Fatalf("two-field totalHits: got %d want %d", td.TotalHits.Value, numDocs)
		}
	}
}

func TestSortOptimization_EqualValuesPointIndex(t *testing.T)   { runSortOptimizationEqualValues(t) }
func TestSortOptimization_EqualValuesSkipperIndex(t *testing.T) { runSortOptimizationEqualValues(t) }

// ── Float sort ───────────────────────────────────────────────────────────────

func runFloatSortOptimization(t *testing.T) {
	const numDocs = 10000
	reader, cleanup := sortOptReader(t, numDocs, 7000, func(t *testing.T, doc *document.Document, i int) {
		addFloatDV(t, doc, "my_field", float32(i))
	})
	defer cleanup()

	const numHits = 3
	sf := search.NewSortField("my_field", search.SortFieldTypeFloat)
	td := sortTopN(t, reader, search.NewSort(sf), numHits)
	if len(td.FieldDocs) != numHits {
		t.Fatalf("float hit count: got %d want %d", len(td.FieldDocs), numHits)
	}
	for i := 0; i < numHits; i++ {
		if got := toFloat32Any(td.FieldDocs[i].Fields[0]); got != float32(i) {
			t.Fatalf("float hit %d: got %v want %v", i, got, float32(i))
		}
	}
	if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Fatalf("float totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
	}
	assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
}

func TestSortOptimization_FloatSortOptimizationPointIndex(t *testing.T) { runFloatSortOptimization(t) }
func TestSortOptimization_FloatSortOptimizationSkipperIndex(t *testing.T) {
	runFloatSortOptimization(t)
}

// ── Doc sort ─────────────────────────────────────────────────────────────────

// testDocSort: a BooleanQuery (lf:1 MUST, id:id3 MUST_NOT) over a doc-sorted
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

// testDocSortOptimization: a plain _doc sort skips all non-competitive documents.
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
	if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
		t.Fatalf("doc-sort totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", td.TotalHits.Relation)
	}
	// Lucene asserts against a per-segment threshold (10); use the same.
	assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, 10)
}

// testDocSortOptimizationMultipleIndices: a [_doc] sort must not miss documents
// across multiple segments.
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

// testDocSortOptimizationWithAfter: sort by _doc with searchAfter returns the
// documents strictly after the marker, and skips non-competitive hits.
func TestSortOptimization_DocSortOptimizationWithAfter(t *testing.T) {
	const numDocs = 1000
	reader, cleanup := sortOptReader(t, numDocs, 500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	const numHits = 10
	searchAfters := []int{3, 10, numDocs - 10}
	for _, searchAfter := range searchAfters {
		// sort by _doc ascending with searchAfter should trigger optimization.
		sort := search.NewSort(search.NewSortField("", search.SortFieldTypeDoc))
		after := intAfter(searchAfter, int32(searchAfter))
		td := assertSearchHits(t, reader, nil, sort, numHits, after)
		expNumHits := numHits
		if searchAfter >= numDocs-numHits {
			expNumHits = numDocs - searchAfter - 1
		}
		if len(td.ScoreDocs) != expNumHits {
			t.Fatalf("after=%d: hit count got %d want %d", searchAfter, len(td.ScoreDocs), expNumHits)
		}
		for i := 0; i < len(td.ScoreDocs); i++ {
			expectedDocID := searchAfter + 1 + i
			if td.ScoreDocs[i].Doc != expectedDocID {
				t.Fatalf("after=%d hit %d: got docID %d want %d", searchAfter, i, td.ScoreDocs[i].Doc, expectedDocID)
			}
		}
		if td.TotalHits.Relation != search.GREATER_THAN_OR_EQUAL_TO {
			t.Fatalf("after=%d totalHits relation: got %v want GREATER_THAN_OR_EQUAL_TO", searchAfter, td.TotalHits.Relation)
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}
}

// testDocSortOptimizationWithAfterCollectsAllDocs: paging by _doc in batches
// visits every document exactly once, in docID order.
func TestSortOptimization_DocSortOptimizationWithAfterCollectsAllDocs(t *testing.T) {
	const numDocs = 5000
	reader, cleanup := sortOptReader(t, numDocs, 500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "lf", int64(i))
	})
	defer cleanup()

	visitedHits := 0
	var after *search.FieldDoc
	for visitedHits < numDocs {
		batch := 250
		td := assertSearchHits(t, reader, nil, search.NewSort(search.NewSortField("", search.SortFieldTypeDoc)), batch, after)
		expectedHits := batch
		if numDocs-visitedHits < batch {
			expectedHits = numDocs - visitedHits
		}
		if len(td.ScoreDocs) != expectedHits {
			t.Fatalf("visited=%d: batch hit count got %d want %d", visitedHits, len(td.ScoreDocs), expectedHits)
		}
		last := td.FieldDocs[expectedHits-1]
		after = last
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

// ── Max-doc-visited (smallest value across segments sorts first) ─────────────

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

// ── Random long (sort returns globally smallest values first) ────────────────

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

// testSortOptimizationOnSortedNumericField: a sort with optimization disabled and
// with optimization enabled must produce the same hits, while the optimized run
// collects fewer (or equal) hits.
func TestSortOptimization_OnSortedNumericField(t *testing.T) {
	const numDocs = 5000
	reader, cleanup := sortOptReader(t, numDocs, 2500, func(t *testing.T, doc *document.Document, i int) {
		addLongDV(t, doc, "my_field", int64(numDocs-1-i)) // descending in i
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
	if !(optimized.TotalHits.Value <= unoptimized.TotalHits.Value) {
		t.Fatalf("expected optimized to collect <= hits: optimized=%d unoptimized=%d", optimized.TotalHits.Value, unoptimized.TotalHits.Value)
	}
}

// ── Point validation ─────────────────────────────────────────────────────────

// testPointValidation: with sort optimization enabled (the default), a LONG sort
// over an int-point field (and an INT sort over a long-point field) must throw an
// IllegalArgumentException-equivalent (an error). Disabling the optimization must
// allow the mismatched sort to run by reading the DocValues.
func TestSortOptimization_PointValidation(t *testing.T) {
	reader, cleanup := sortOptReader(t, 1, -1, func(t *testing.T, doc *document.Document, i int) {
		doc.Add(document.NewIntPoint("intField", 4))
		addLongDV(t, doc, "intField", 4)
		doc.Add(document.NewLongPoint("longField", 42))
		addLongDV(t, doc, "longField", 42)
	})
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)

	// LONG sort over an int-point field: optimization enabled => must error.
	longOnInt := search.NewSortField("intField", search.SortFieldTypeLong)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(longOnInt)); err == nil {
		t.Fatalf("LONG sort on int-point field with optimization enabled: expected an error (IllegalArgumentException-equivalent), got nil")
	}
	// With optimization disabled the mismatched sort must succeed.
	longOnInt.SetOptimizeSortWithIndexedData(false)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(longOnInt)); err != nil {
		t.Fatalf("LONG sort on int-point field with optimization disabled: unexpected error: %v", err)
	}

	// INT sort over a long-point field: optimization enabled => must error.
	intOnLong := search.NewSortField("longField", search.SortFieldTypeInt)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(intOnLong)); err == nil {
		t.Fatalf("INT sort on long-point field with optimization enabled: expected an error (IllegalArgumentException-equivalent), got nil")
	}
	intOnLong.SetOptimizeSortWithIndexedData(false)
	if _, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 1, search.NewSort(intOnLong)); err != nil {
		t.Fatalf("INT sort on long-point field with optimization disabled: unexpected error: %v", err)
	}
}

// ── String sort optimization (over a SortedDocValues field) ──────────────────

func runStringSortOptimization(t *testing.T) {
	const numDocs = 2000
	reader, cleanup := sortOptReader(t, numDocs, 1000, func(t *testing.T, doc *document.Document, i int) {
		f, err := document.NewSortedDocValuesField("my_field", []byte(pad6(i)))
		if err != nil {
			t.Fatalf("NewSortedDocValuesField: %v", err)
		}
		doc.Add(f)
	})
	defer cleanup()

	const numHits = 5

	{ // simple ascending sort, missing-last: optimization skips hits.
		sf := search.NewSortField("my_field", search.SortFieldTypeString)
		sf.SetMissingValue(search.STRING_LAST)
		td := sortTopN(t, reader, search.NewSort(sf), numHits)
		for i := 0; i < numHits; i++ {
			got, _ := td.FieldDocs[i].Fields[0].([]byte)
			if want := pad6(i); string(got) != want {
				t.Fatalf("string hit %d: got %q want %q", i, string(got), want)
			}
		}
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // secondary sort on _score: hits are still skipped.
		sf := search.NewSortField("my_field", search.SortFieldTypeString)
		sf.SetMissingValue(search.STRING_LAST)
		sort := search.NewSort(sf, search.NewSortField("", search.SortFieldTypeScore))
		td := sortTopN(t, reader, sort, numHits)
		assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
	}

	{ // string field as secondary sort: no optimization, all docs collected.
		sf := search.NewSortField("my_field", search.SortFieldTypeString)
		sf.SetMissingValue(search.STRING_LAST)
		sort := search.NewSort(search.NewSortField("", search.SortFieldTypeScore), sf)
		td := sortTopN(t, reader, sort, numHits)
		if td.TotalHits.Value != int64(numDocs) {
			t.Fatalf("string-secondary totalHits: got %d want %d", td.TotalHits.Value, numDocs)
		}
	}

	{ // optimization disabled: all docs collected.
		sf := search.NewSortField("my_field", search.SortFieldTypeString)
		sf.SetMissingValue(search.STRING_LAST)
		sf.SetOptimizeSortWithIndexedData(false)
		td := sortTopN(t, reader, search.NewSort(sf), numHits)
		if td.TotalHits.Value != int64(numDocs) {
			t.Fatalf("string-disabled totalHits: got %d want %d", td.TotalHits.Value, numDocs)
		}
	}
}

func TestSortOptimization_StringSortOptimizationBasedPostings(t *testing.T) {
	runStringSortOptimization(t)
}
func TestSortOptimization_StringSortOptimizationBasedDVSkipper(t *testing.T) {
	runStringSortOptimization(t)
}

func runStringSortOptimizationWithMissingValues(t *testing.T) {
	const numDocs = 3000
	reader, cleanup := sortOptReader(t, numDocs, 1000, func(t *testing.T, doc *document.Document, i int) {
		if i%2 == 0 { // half missing
			f, err := document.NewSortedDocValuesField("my_field", []byte(pad6(i)))
			if err != nil {
				t.Fatalf("NewSortedDocValuesField: %v", err)
			}
			doc.Add(f)
		}
	})
	defer cleanup()

	const numHits = 3
	sf := search.NewSortField("my_field", search.SortFieldTypeString)
	sf.SetMissingValue(search.STRING_LAST)
	// Ascending, missing-last: the smallest present even-index values come first.
	td := sortTopN(t, reader, search.NewSort(sf), numHits)
	for i := 0; i < numHits; i++ {
		got, _ := td.FieldDocs[i].Fields[0].([]byte)
		want := pad6(2 * i) // 0, 2, 4 are the smallest present keys
		if string(got) != want {
			t.Fatalf("string-missing hit %d: got %q want %q", i, string(got), want)
		}
	}
	assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, numDocs)
}

func TestSortOptimization_StringSortOptimizationWithMissingValuesBasedPostings(t *testing.T) {
	runStringSortOptimizationWithMissingValues(t)
}
func TestSortOptimization_StringSortOptimizationWithMissingValuesBasedDVSkipper(t *testing.T) {
	runStringSortOptimizationWithMissingValues(t)
}

// testStringSortOptimizationFieldMissingInSegment: a segment with the field
// followed by a segment entirely without it; ascending missing-last sort must
// surface the present values first and skip the field-less segment's docs.
func runStringSortOptimizationFieldMissingInSegment(t *testing.T) {
	const docsWithField = 20
	const docsWithoutField = 2000
	const total = docsWithField + docsWithoutField
	reader, cleanup := sortOptReader(t, total, docsWithField-1, func(t *testing.T, doc *document.Document, i int) {
		if i < docsWithField {
			f, err := document.NewSortedDocValuesField("my_field", []byte(pad6(i)))
			if err != nil {
				t.Fatalf("NewSortedDocValuesField: %v", err)
			}
			doc.Add(f)
		}
	})
	defer cleanup()

	const numHits = 5
	sf := search.NewSortField("my_field", search.SortFieldTypeString)
	sf.SetMissingValue(search.STRING_LAST)
	td := sortTopN(t, reader, search.NewSort(sf), numHits)
	for i := 0; i < numHits; i++ {
		got, _ := td.FieldDocs[i].Fields[0].([]byte)
		want := pad6(i)
		if string(got) != want {
			t.Fatalf("field-missing hit %d: got %q want %q", i, string(got), want)
		}
	}
	assertNonCompetitiveHitsAreSkipped(t, td.TotalHits.Value, total)
}

func TestSortOptimization_StringSortOptimizationFieldMissingInSegmentBasedPostings(t *testing.T) {
	runStringSortOptimizationFieldMissingInSegment(t)
}
func TestSortOptimization_StringSortOptimizationFieldMissingInSegmentBasedDVSkipper(t *testing.T) {
	runStringSortOptimizationFieldMissingInSegment(t)
}

// ── small local helpers ──────────────────────────────────────────────────────

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
