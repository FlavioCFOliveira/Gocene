// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestDoubleRange_ToString mirrors Lucene 10.4.0 TestDoubleRange.testToString:
//
//	new DoubleRange("foo", new double[]{0.1,1.1,2.1,3.1}, new double[]{.2,1.2,2.2,3.2})
//	  .toString() == "DoubleRange <foo: [0.1 : 0.2] [1.1 : 1.2] [2.1 : 2.2] [3.1 : 3.2]>"
func TestDoubleRange_ToString(t *testing.T) {
	r, err := NewDoubleRange("foo",
		[]float64{0.1, 1.1, 2.1, 3.1},
		[]float64{0.2, 1.2, 2.2, 3.2})
	if err != nil {
		t.Fatalf("NewDoubleRange: %v", err)
	}
	got := r.String()
	want := "DoubleRange <foo: [0.1 : 0.2] [1.1 : 1.2] [2.1 : 2.2] [3.1 : 3.2]>"
	if got != want {
		t.Fatalf("String() mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestDoubleRange_BasicRoundTrip(t *testing.T) {
	r, err := NewDoubleRange("r", []float64{-1.5, 0}, []float64{2.5, 10})
	if err != nil {
		t.Fatalf("NewDoubleRange: %v", err)
	}
	if got := r.NumDimensions(); got != 2 {
		t.Fatalf("NumDimensions = %d, want 2", got)
	}
	if got, want := r.GetMin(0), -1.5; got != want {
		t.Fatalf("GetMin(0) = %v, want %v", got, want)
	}
	if got, want := r.GetMax(0), 2.5; got != want {
		t.Fatalf("GetMax(0) = %v, want %v", got, want)
	}
	if got, want := r.GetMin(1), 0.0; got != want {
		t.Fatalf("GetMin(1) = %v, want %v", got, want)
	}
	if got, want := r.GetMax(1), 10.0; got != want {
		t.Fatalf("GetMax(1) = %v, want %v", got, want)
	}
	if got, want := r.FieldType().PointDimensionCount(), 4; got != want {
		t.Fatalf("PointDimensionCount = %d, want %d", got, want)
	}
}

func TestDoubleRange_OpenEndedRange(t *testing.T) {
	r, err := NewDoubleRange("r", []float64{math.Inf(-1)}, []float64{math.Inf(+1)})
	if err != nil {
		t.Fatalf("NewDoubleRange: %v", err)
	}
	if got, want := r.GetMin(0), math.Inf(-1); got != want {
		t.Fatalf("GetMin(0) = %v, want %v", got, want)
	}
	if got, want := r.GetMax(0), math.Inf(+1); got != want {
		t.Fatalf("GetMax(0) = %v, want %v", got, want)
	}
}

func TestDoubleRange_MinGreaterThanMaxErrors(t *testing.T) {
	_, err := NewDoubleRange("r", []float64{10}, []float64{5})
	if err == nil {
		t.Fatalf("expected error for min > max")
	}
	if !strings.Contains(err.Error(), "min value") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestDoubleRange_NaNRejected(t *testing.T) {
	if _, err := NewDoubleRange("r", []float64{math.NaN()}, []float64{1}); err == nil {
		t.Fatalf("expected error for NaN min")
	}
	if _, err := NewDoubleRange("r", []float64{0}, []float64{math.NaN()}); err == nil {
		t.Fatalf("expected error for NaN max")
	}
}

func TestDoubleRange_MismatchedSizesErrors(t *testing.T) {
	_, err := NewDoubleRange("r", []float64{1, 2}, []float64{3})
	if err == nil {
		t.Fatalf("expected error for mismatched dimension counts")
	}
}

func TestDoubleRange_EmptyErrors(t *testing.T) {
	if _, err := NewDoubleRange("r", nil, nil); err == nil {
		t.Fatalf("expected error for nil min/max")
	}
	if _, err := NewDoubleRange("r", []float64{}, []float64{}); err == nil {
		t.Fatalf("expected error for empty min/max")
	}
}

func TestDoubleRange_TooManyDimensionsErrors(t *testing.T) {
	min := []float64{0, 0, 0, 0, 0}
	max := []float64{1, 1, 1, 1, 1}
	_, err := NewDoubleRange("r", min, max)
	if err == nil {
		t.Fatalf("expected error for 5 dimensions")
	}
	if !strings.Contains(err.Error(), "4 dimensions") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// TestDoubleRange_EncodeMatchesSortableBytes verifies the wire layout matches
// Lucene's verifyAndEncode: minD0 ... minD{N-1} | maxD0 ... maxD{N-1}, each
// dimension encoded via DoubleToSortableLong + LongToSortableBytes.
func TestDoubleRange_EncodeMatchesSortableBytes(t *testing.T) {
	min := []float64{-1.5, 0, 100.25}
	max := []float64{10.5, 50, 200.75}
	got, err := EncodeDoubleRange(min, max)
	if err != nil {
		t.Fatalf("EncodeDoubleRange: %v", err)
	}
	want := make([]byte, 2*len(min)*DoubleRangeBytes)
	for i, v := range min {
		util.LongToSortableBytes(util.DoubleToSortableLong(v), want, i*DoubleRangeBytes)
	}
	for i, v := range max {
		util.LongToSortableBytes(util.DoubleToSortableLong(v), want, len(min)*DoubleRangeBytes+i*DoubleRangeBytes)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte %d = 0x%02x, want 0x%02x", i, got[i], want[i])
		}
	}
}

func TestDoubleRange_GetMinOutOfRangePanics(t *testing.T) {
	r, err := NewDoubleRange("r", []float64{0}, []float64{1})
	if err != nil {
		t.Fatalf("NewDoubleRange: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for dim out of range")
		}
	}()
	_ = r.GetMin(1)
}

// TestDoubleRange_DeprecatedAliases ensures the deprecated DoubleRangeLucene
// type alias and constructor variables still resolve to the new canonical
// implementations.
func TestDoubleRange_DeprecatedAliases(t *testing.T) {
	r, err := NewDoubleRangeLucene("r", []float64{-7.5}, []float64{42.25})
	if err != nil {
		t.Fatalf("NewDoubleRangeLucene: %v", err)
	}
	var _ *DoubleRangeLucene = r // alias is identical type
	if got, want := r.GetMin(0), -7.5; got != want {
		t.Fatalf("GetMin = %v, want %v", got, want)
	}
	got, err := EncodeDoubleRangeLucene([]float64{1.25}, []float64{2.75})
	if err != nil {
		t.Fatalf("EncodeDoubleRangeLucene: %v", err)
	}
	want, err := EncodeDoubleRange([]float64{1.25}, []float64{2.75})
	if err != nil {
		t.Fatalf("EncodeDoubleRange: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("alias encoding mismatch")
	}
}
