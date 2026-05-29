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
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
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

// TestMultiValuesSource_IndexIntegration mirrors the full random index
// round-trip tests in the Java source. The on-disk SortedNumericDocValues read
// path is wired (rmp #4771, consumed by the facets accumulators in #4704), but
// the field-based MultiLongValuesSource / MultiDoubleValuesSource.fromField
// implementations this test needs are not yet ported.
func TestMultiValuesSource_IndexIntegration(t *testing.T) {
	t.Skip("requires field-based MultiLongValuesSource/MultiDoubleValuesSource.fromField (rmp #4773)")
}
