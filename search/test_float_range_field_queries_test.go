// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestFloatRangeFieldQueries.java
//
// testBasics indexes eight FloatRange documents, then counts INTERSECTS /
// WITHIN / CONTAINS / CROSSES hits for the query box [-11,-15]..[15,20]. The
// FloatRange field is indexed through the live BKD points path; queries are the
// BKD-backed search.RangeFieldQuery. bytes-per-dim = 4.

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

const floatRangeFieldName = "floatRangeField"

func buildFloatRangeBasicsIndex(t *testing.T, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(min, max []float32) {
		doc := document.NewDocument()
		f, err := document.NewFloatRangeLucene(floatRangeFieldName, min, max)
		if err != nil {
			t.Fatalf("NewFloatRangeLucene(%v,%v): %v", min, max, err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	negInf := float32(math.Inf(-1))
	add([]float32{-10.0, -10.0}, []float32{9.1, 10.1})    // intersects (within)
	add([]float32{10.0, -10.0}, []float32{20.0, 10.0})    // intersects (crosses)
	add([]float32{-20.0, -20.0}, []float32{30.0, 30.1})   // contains, crosses
	add([]float32{-11.1, -11.2}, []float32{1.23, 11.5})   // intersects (crosses)
	add([]float32{12.33, 1.2}, []float32{15.1, 29.9})     // intersects (crosses)
	add([]float32{-122.33, 1.2}, []float32{-115.1, 29.9}) // disjoint
	add([]float32{negInf, 1.2}, []float32{-11.0, 29.9})   // intersects (crosses)
	add([]float32{-11, -15}, []float32{15, 20})           // equal

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

func floatRangeQuery(t *testing.T, qt search.RangeFieldQueryType) search.Query {
	t.Helper()
	qMin := encodeFloatRangeQueryBounds([]float32{-11.0, -15.0})
	qMax := encodeFloatRangeQueryBounds([]float32{15.0, 20.0})
	q, err := search.NewRangeFieldQueryFull(floatRangeFieldName, qMin, qMax, 2, 4, qt)
	if err != nil {
		t.Fatalf("NewRangeFieldQueryFull: %v", err)
	}
	return q
}

// TestFloatRangeFieldQueries_Basics ports TestFloatRangeFieldQueries.testBasics.
func TestFloatRangeFieldQueries_Basics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	reader := buildFloatRangeBasicsIndex(t, dir)
	defer func() { _ = reader.Close() }()

	if got := rangeQueryCount(t, reader, floatRangeQuery(t, search.RangeFieldQueryTypeIntersects)); got != 7 {
		t.Fatalf("INTERSECTS count: got %d want 7", got)
	}
	if got := rangeQueryCount(t, reader, floatRangeQuery(t, search.RangeFieldQueryTypeWithin)); got != 2 {
		t.Fatalf("WITHIN count: got %d want 2", got)
	}
	if got := rangeQueryCount(t, reader, floatRangeQuery(t, search.RangeFieldQueryTypeContains)); got != 2 {
		t.Fatalf("CONTAINS count: got %d want 2", got)
	}
	if got := rangeQueryCount(t, reader, floatRangeQuery(t, search.RangeFieldQueryTypeCrosses)); got != 5 {
		t.Fatalf("CROSSES count: got %d want 5", got)
	}
}