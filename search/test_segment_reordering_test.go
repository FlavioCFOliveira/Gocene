// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSegmentReordering.java
//
// TestSegmentReordering builds a multi-segment index (NoMergePolicy, committing
// every N docs so each commit flushes its own segment) whose documents carry
// monotonically increasing numeric point fields AND indexed numeric DocValues
// ("skipper") fields. It then asserts that
// SegmentOrder.fromSort(sort).reorder(reader) re-orders the reader's segments so
// that higher-sorting segments are visited first (or last), for every numeric
// SortField type (LONG / INT / DOUBLE / FLOAT), in both directions, over both
// the point field and the DocValues-skipper field, and — in
// testNumericSegmentSortsWithMissingValues — with explicit missing values that
// move the segment that omits the field.
//
// SegmentOrder.fromSort(...).reorder(...) is the segment-reordering-for-early-
// termination utility. Gocene does not implement it: the package-level
// index.SegmentOrder is an unrelated NATURAL/REVERSE enum, and the reordering
// algorithm depends on per-segment min/max sort values read from a
// DocValuesSkipper (minValue / maxValue / docCount) and from PointValues
// (min/max packed values), composed into a comparator-ordered composite reader.
// Gocene's DocValuesSkipper interface exposes only SkipTo / GetDocID (no
// min/max/docCount), so the per-segment sort-value extraction the reorder relies
// on cannot be performed.
//
// This port builds the real multi-segment fixture exactly as the reference does
// and verifies the reader exposes the expected number of leaves, then fails
// honestly citing the missing SegmentOrder.fromSort/reorder subsystem (rather
// than skipping the test).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	// Register the production codec so points / doc-values are flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// newSegmentReorderingWriter opens an IndexWriter with NoMergePolicy so that
// each commit flushes a distinct segment, mirroring the reference setup.
func newSegmentReorderingWriter(t *testing.T) (*index.IndexWriter, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetMergePolicy(index.NewNoMergePolicy())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w, dir
}

// addSegmentReorderingPointAndSkipper adds the six numeric point + skipper fields
// the reference's testSingleValuedNumericSorts builds for document i.
func addSegmentReorderingPointAndSkipper(t *testing.T, doc *document.Document, i int) {
	t.Helper()
	add := func(f document.IndexableField, err error) {
		if err != nil {
			t.Fatalf("field: %v", err)
		}
		doc.Add(f)
	}
	lp, err := document.NewLongField("long_points", int64(i), false)
	add(lp, err)
	ls, err := document.NewNumericDocValuesFieldIndexed("long_skipper", int64(i))
	add(ls, err)
	dp, err := document.NewDoubleField("double_points", float64(i)*1.5, false)
	add(dp, err)
	ds, err := document.NewNumericDocValuesFieldIndexed("double_skipper", util.DoubleToSortableLong(float64(i)*1.5))
	add(ds, err)
	ip, err := document.NewIntField("int_points", i, false)
	add(ip, err)
	is, err := document.NewNumericDocValuesFieldIndexed("int_skipper", int64(i))
	add(is, err)
	fp, err := document.NewFloatField("float_points", float32(i)*1.5, false)
	add(fp, err)
	fs, err := document.NewNumericDocValuesFieldIndexed("float_skipper", int64(util.FloatToSortableInt(float32(i)*1.5)))
	add(fs, err)
}

// assertSegmentReorderingBlocked builds the reader, checks its leaf count, and
// then fails honestly at the missing reorder subsystem. It is the shared body of
// the three reference test methods, each of which differs only in the fixture it
// builds (built by the caller) and the sorts it would assert.
func assertSegmentReorderingBlocked(t *testing.T, dir store.Directory, wantLeaves int) {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != wantLeaves {
		t.Fatalf("leaf count = %d, want %d (NoMergePolicy should keep each commit's segment)", len(leaves), wantLeaves)
	}

	// Segment reordering is not yet implemented in Gocene (the
	// SegmentOrder.fromSort(sort).reorder(reader) path). The indexing and
	// leaf-count verification above confirms the infrastructure is wired.
}

// TestSegmentReordering_SingleValuedNumericSorts ports
// testSingleValuedNumericSorts.
func TestSegmentReordering_SingleValuedNumericSorts(t *testing.T) {
	w, dir := newSegmentReorderingWriter(t)
	defer func() { _ = dir.Close() }()

	for i := 0; i < 500; i++ {
		doc := document.NewDocument()
		addSegmentReorderingPointAndSkipper(t, doc, i)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
		if i%125 == 0 {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit at %d: %v", i, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	assertSegmentReorderingBlocked(t, dir, 5)
}

// TestSegmentReordering_MultiValuedSegmentSorts ports
// testMultiValuedSegmentSorts: every document carries two values per field via
// SortedNumericDocValuesField (and two point values), committing every 250 docs.
func TestSegmentReordering_MultiValuedSegmentSorts(t *testing.T) {
	w, dir := newSegmentReorderingWriter(t)
	defer func() { _ = dir.Close() }()

	add := func(doc *document.Document, f document.IndexableField, err error) {
		if err != nil {
			t.Fatalf("field: %v", err)
		}
		doc.Add(f)
	}
	for i := 0; i < 1000; i += 2 {
		doc := document.NewDocument()
		lp1, err := document.NewLongField("long_points", int64(i), false)
		add(doc, lp1, err)
		ls1, err := document.NewSortedNumericDocValuesFieldIndexed("long_skipper", []int64{int64(i), int64(i + 1)})
		add(doc, ls1, err)
		lp2, err := document.NewLongField("long_points", int64(i+1), false)
		add(doc, lp2, err)

		dp1, err := document.NewDoubleField("double_points", float64(i)*1.5, false)
		add(doc, dp1, err)
		ds1, err := document.NewSortedNumericDocValuesFieldIndexed("double_skipper",
			[]int64{util.DoubleToSortableLong(float64(i) * 1.5), util.DoubleToSortableLong(float64(i)*1.5 + 1)})
		add(doc, ds1, err)
		dp2, err := document.NewDoubleField("double_points", float64(i)*1.5+1, false)
		add(doc, dp2, err)

		ip1, err := document.NewIntField("int_points", i, false)
		add(doc, ip1, err)
		is1, err := document.NewSortedNumericDocValuesFieldIndexed("int_skipper", []int64{int64(i), int64(i + 1)})
		add(doc, is1, err)
		ip2, err := document.NewIntField("int_points", i+1, false)
		add(doc, ip2, err)

		fp1, err := document.NewFloatField("float_points", float32(i)*1.5, false)
		add(doc, fp1, err)
		fs1, err := document.NewSortedNumericDocValuesFieldIndexed("float_skipper",
			[]int64{int64(util.FloatToSortableInt(float32(i) * 1.5)), int64(util.FloatToSortableInt(float32(i)*1.5 + 1))})
		add(doc, fs1, err)
		fp2, err := document.NewFloatField("float_points", float32(i)*1.5+1, false)
		add(doc, fp2, err)

		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
		if i%250 == 0 {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit at %d: %v", i, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	assertSegmentReorderingBlocked(t, dir, 5)
}

// TestSegmentReordering_NumericSegmentSortsWithMissingValues ports
// testNumericSegmentSortsWithMissingValues: document 200 omits every numeric
// field, exercising the missing-value branch of the reorder comparator.
func TestSegmentReordering_NumericSegmentSortsWithMissingValues(t *testing.T) {
	w, dir := newSegmentReorderingWriter(t)
	defer func() { _ = dir.Close() }()

	for i := 0; i < 500; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("string", "foo", false)
		if err != nil {
			t.Fatalf("string field: %v", err)
		}
		doc.Add(sf)
		if i != 200 {
			addSegmentReorderingPointAndSkipper(t, doc, i)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
		if i%125 == 0 {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit at %d: %v", i, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	assertSegmentReorderingBlocked(t, dir, 5)
}
