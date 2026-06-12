// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestLongRangeFieldQueries.java
//
// testBasics indexes eight LongRange documents (integer-valued boxes), then
// counts INTERSECTS / WITHIN / CONTAINS / CROSSES hits for the query box
// [-11,-15]..[15,20]. Expected: 7 / 3 / 2 / 4. The LongRange field is indexed
// through the live BKD points path; queries are the BKD-backed
// search.RangeFieldQuery. bytes-per-dim = 8.

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

const longRangeFieldName = "longRangeField"

func buildLongRangeBasicsIndex(t *testing.T, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(min, max []int64) {
		doc := document.NewDocument()
		f, err := document.NewLongRange(longRangeFieldName, min, max)
		if err != nil {
			t.Fatalf("NewLongRange(%v,%v): %v", min, max, err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	add([]int64{-10, -10}, []int64{9, 10})           // intersects (within)
	add([]int64{10, -10}, []int64{20, 10})           // intersects (crosses)
	add([]int64{-20, -20}, []int64{30, 30})          // contains, crosses
	add([]int64{-11, -11}, []int64{1, 11})           // intersects (within)
	add([]int64{12, 1}, []int64{15, 29})             // intersects (crosses)
	add([]int64{-122, 1}, []int64{-115, 29})         // disjoint
	add([]int64{math.MinInt64, 1}, []int64{-11, 29}) // intersects (crosses)
	add([]int64{-11, -15}, []int64{15, 20})          // equal

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return reader
}

func longRangeQuery(t *testing.T, qt search.RangeFieldQueryType) search.Query {
	t.Helper()
	qMin := encodeLongRangeQueryBounds([]int64{-11, -15})
	qMax := encodeLongRangeQueryBounds([]int64{15, 20})
	q, err := search.NewRangeFieldQueryFull(longRangeFieldName, qMin, qMax, 2, 8, qt)
	if err != nil {
		t.Fatalf("NewRangeFieldQueryFull: %v", err)
	}
	return q
}

// TestLongRangeFieldQueries_Basics ports TestLongRangeFieldQueries.testBasics.
func TestLongRangeFieldQueries_Basics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	reader := buildLongRangeBasicsIndex(t, dir)
	defer func() { _ = reader.Close() }()

	if got := rangeQueryCount(t, reader, longRangeQuery(t, search.RangeFieldQueryTypeIntersects)); got != 7 {
		t.Fatalf("INTERSECTS count: got %d want 7", got)
	}
	if got := rangeQueryCount(t, reader, longRangeQuery(t, search.RangeFieldQueryTypeWithin)); got != 3 {
		t.Fatalf("WITHIN count: got %d want 3", got)
	}
	if got := rangeQueryCount(t, reader, longRangeQuery(t, search.RangeFieldQueryTypeContains)); got != 2 {
		t.Fatalf("CONTAINS count: got %d want 2", got)
	}
	if got := rangeQueryCount(t, reader, longRangeQuery(t, search.RangeFieldQueryTypeCrosses)); got != 4 {
		t.Fatalf("CROSSES count: got %d want 4", got)
	}
}