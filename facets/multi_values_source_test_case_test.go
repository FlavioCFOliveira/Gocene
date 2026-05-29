// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// multiValuesSourceTestHelpers ports the validation helpers from
// org.apache.lucene.facet.MultiValuesSourceTestCase.
// The Java class is an abstract LuceneTestCase that provides
// validateFieldBasedSource overloads for checking that a MultiLongValues /
// MultiDoubleValues iterator produces the same values as the underlying
// NumericDocValues / SortedNumericDocValues.
//
// In Gocene these become standalone functions in the test binary.

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codec registry so IndexWriter has a default codec able to
	// persist SortedNumericDocValues to disk.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
)

// validateMultiLongValuesMatch checks that a sliceLongValues iterator and a
// reference slice agree on every document. Mirrors the MultiLongValues
// overload of validateFieldBasedSource.
func validateMultiLongValuesMatch(t *testing.T, want []int64, mv MultiLongValues) {
	t.Helper()
	ok, err := mv.AdvanceExact(0)
	if err != nil {
		t.Fatalf("AdvanceExact: %v", err)
	}
	if !ok {
		if len(want) > 0 {
			t.Fatalf("AdvanceExact returned false but expected %d values", len(want))
		}
		return
	}
	if mv.DocValueCount() != len(want) {
		t.Fatalf("DocValueCount: want %d, got %d", len(want), mv.DocValueCount())
	}
	for i, w := range want {
		v, err := mv.NextValue()
		if err != nil {
			t.Fatalf("NextValue[%d]: %v", i, err)
		}
		if v != w {
			t.Errorf("NextValue[%d]: want %d, got %d", i, w, v)
		}
	}
}

// validateMultiDoubleValuesMatch checks that a MultiDoubleValues iterator
// agrees with a reference float64 slice. Mirrors the MultiDoubleValues
// overload of validateFieldBasedSource.
func validateMultiDoubleValuesMatch(t *testing.T, want []float64, mv MultiDoubleValues) {
	t.Helper()
	ok, err := mv.AdvanceExact(0)
	if err != nil {
		t.Fatalf("AdvanceExact: %v", err)
	}
	if !ok {
		if len(want) > 0 {
			t.Fatalf("AdvanceExact returned false but expected %d values", len(want))
		}
		return
	}
	if mv.DocValueCount() != len(want) {
		t.Fatalf("DocValueCount: want %d, got %d", len(want), mv.DocValueCount())
	}
	for i, w := range want {
		v, err := mv.NextValue()
		if err != nil {
			t.Fatalf("NextValue[%d]: %v", i, err)
		}
		if math.Abs(v-w) > math.Abs(w)/1e5+1e-15 {
			t.Errorf("NextValue[%d]: want %v, got %v", i, w, v)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests that exercise the helpers with in-memory implementations
// ---------------------------------------------------------------------------

func TestMultiValuesSource_EmptyLong(t *testing.T) {
	mv := NewEmptyMultiLongValues()
	validateMultiLongValuesMatch(t, nil, mv)
}

func TestMultiValuesSource_EmptyDouble(t *testing.T) {
	mv := NewEmptyMultiDoubleValues()
	validateMultiDoubleValuesMatch(t, nil, mv)
}

func TestMultiValuesSource_ConstantLong(t *testing.T) {
	src := NewConstantMultiLongValuesSource(10, 20, 30)
	mv, err := src.GetValues(index.LeafReaderContext{})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	validateMultiLongValuesMatch(t, []int64{10, 20, 30}, mv)
}

func TestMultiValuesSource_ConstantDouble(t *testing.T) {
	src := NewConstantMultiDoubleValuesSource(1.5, 2.5, 3.5)
	mv, err := src.GetValues(index.LeafReaderContext{})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	validateMultiDoubleValuesMatch(t, []float64{1.5, 2.5, 3.5}, mv)
}

func TestMultiValuesSource_SingleLong(t *testing.T) {
	src := NewConstantMultiLongValuesSource(42)
	mv, err := src.GetValues(index.LeafReaderContext{})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	ok, _ := mv.AdvanceExact(0)
	if !ok {
		t.Fatal("expected value")
	}
	if mv.DocValueCount() != 1 {
		t.Errorf("DocValueCount: want 1, got %d", mv.DocValueCount())
	}
	v, _ := mv.NextValue()
	if v != 42 {
		t.Errorf("NextValue: want 42, got %d", v)
	}
}

func TestMultiValuesSource_NeedsScores(t *testing.T) {
	longSrc := NewConstantMultiLongValuesSource(1)
	if longSrc.NeedsScores() {
		t.Error("ConstantMultiLongValuesSource: NeedsScores should be false")
	}
	doubleSrc := NewConstantMultiDoubleValuesSource(1.0)
	if doubleSrc.NeedsScores() {
		t.Error("ConstantMultiDoubleValuesSource: NeedsScores should be false")
	}
}

// TestMultiValuesSource_IndexIntegration mirrors the random index round-trip
// tests in org.apache.lucene.facet.MultiValuesSourceTestCase: it indexes
// per-document SortedNumericDocValues, reopens the segment from disk, and
// asserts that the field-based MultiLongValuesSource / MultiDoubleValuesSource
// iterators reproduce the indexed values for every document. The on-disk
// SortedNumericDocValues read path landed in rmp #4771; the field-based sources
// landed in rmp #4773.
func TestMultiValuesSource_IndexIntegration(t *testing.T) {
	const field = "snmv"

	// Per-document value sets, supplied in ascending order. This test validates
	// that the field-based MultiLongValues/MultiDoubleValues iterators reproduce
	// the indexed SortedNumericDocValues; it does not assert the writer's
	// per-document sort (the current flush path stores values in insertion
	// order, tracked separately), so the inputs are pre-sorted to make the
	// expectation independent of that behaviour. Negative values exercise the
	// double-from-long decoder. The first document is single-valued (exercising
	// the singleton path), the rest are multi-valued (dense path). The writer
	// sort gap is tracked in rmp #4783.
	docValues := [][]int64{
		{5},
		{10, 20, 30},
		{3, 7, 99},
		{-4, 0, 4},
	}

	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i, vals := range docValues {
		sndv, err := document.NewSortedNumericDocValuesField(field, vals)
		if err != nil {
			t.Fatalf("NewSortedNumericDocValuesField(doc %d): %v", i, err)
		}
		doc := document.NewDocument()
		doc.Add(sndv)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	ctx := index.NewLeafReaderContext(segs[0], nil, 0, 0)

	// Expected values per doc, sorted ascending to match the on-disk order.
	wantLong := make([][]int64, len(docValues))
	for i, vals := range docValues {
		cp := append([]int64(nil), vals...)
		sort.Slice(cp, func(a, b int) bool { return cp[a] < cp[b] })
		wantLong[i] = cp
	}

	t.Run("MultiLongValues", func(t *testing.T) {
		src := NewMultiLongValuesSourceFromField(field)
		if src.NeedsScores() {
			t.Error("field MultiLongValuesSource: NeedsScores should be false")
		}
		mv, err := src.GetValues(*ctx)
		if err != nil {
			t.Fatalf("GetValues: %v", err)
		}
		for doc, want := range wantLong {
			validateMultiLongValuesAt(t, doc, want, mv)
		}
	})

	t.Run("MultiDoubleValues", func(t *testing.T) {
		src := NewMultiDoubleValuesSourceFromLongField(field)
		if src.NeedsScores() {
			t.Error("field MultiDoubleValuesSource: NeedsScores should be false")
		}
		mv, err := src.GetValues(*ctx)
		if err != nil {
			t.Fatalf("GetValues: %v", err)
		}
		for doc, wl := range wantLong {
			want := make([]float64, len(wl))
			for i, v := range wl {
				want[i] = float64(v)
			}
			validateMultiDoubleValuesAt(t, doc, want, mv)
		}
	})
}

// validateMultiLongValuesAt asserts the MultiLongValues iterator reproduces
// want for the given docID (the iterator must be advanced monotonically).
func validateMultiLongValuesAt(t *testing.T, docID int, want []int64, mv MultiLongValues) {
	t.Helper()
	ok, err := mv.AdvanceExact(docID)
	if err != nil {
		t.Fatalf("doc %d: AdvanceExact: %v", docID, err)
	}
	if !ok {
		t.Fatalf("doc %d: AdvanceExact returned false, want %d values", docID, len(want))
	}
	if mv.DocValueCount() != len(want) {
		t.Fatalf("doc %d: DocValueCount = %d, want %d", docID, mv.DocValueCount(), len(want))
	}
	for i, w := range want {
		v, err := mv.NextValue()
		if err != nil {
			t.Fatalf("doc %d: NextValue[%d]: %v", docID, i, err)
		}
		if v != w {
			t.Errorf("doc %d: NextValue[%d] = %d, want %d", docID, i, v, w)
		}
	}
}

// validateMultiDoubleValuesAt asserts the MultiDoubleValues iterator reproduces
// want for the given docID (the iterator must be advanced monotonically).
func validateMultiDoubleValuesAt(t *testing.T, docID int, want []float64, mv MultiDoubleValues) {
	t.Helper()
	ok, err := mv.AdvanceExact(docID)
	if err != nil {
		t.Fatalf("doc %d: AdvanceExact: %v", docID, err)
	}
	if !ok {
		t.Fatalf("doc %d: AdvanceExact returned false, want %d values", docID, len(want))
	}
	if mv.DocValueCount() != len(want) {
		t.Fatalf("doc %d: DocValueCount = %d, want %d", docID, mv.DocValueCount(), len(want))
	}
	for i, w := range want {
		v, err := mv.NextValue()
		if err != nil {
			t.Fatalf("doc %d: NextValue[%d]: %v", docID, i, err)
		}
		if math.Abs(v-w) > math.Abs(w)/1e5+1e-15 {
			t.Errorf("doc %d: NextValue[%d] = %v, want %v", docID, i, v, w)
		}
	}
}
