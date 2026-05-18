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

// TestLongRange_ToString mirrors Lucene 10.4.0 TestLongRange.testToString:
//
//	new LongRange("foo", new long[]{1,11,21,31}, new long[]{2,12,22,32})
//	  .toString() == "LongRange <foo: [1 : 2] [11 : 12] [21 : 22] [31 : 32]>"
func TestLongRange_ToString(t *testing.T) {
	r, err := NewLongRange("foo", []int64{1, 11, 21, 31}, []int64{2, 12, 22, 32})
	if err != nil {
		t.Fatalf("NewLongRange: %v", err)
	}
	got := r.String()
	want := "LongRange <foo: [1 : 2] [11 : 12] [21 : 22] [31 : 32]>"
	if got != want {
		t.Fatalf("String() mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestLongRange_BasicRoundTrip(t *testing.T) {
	r, err := NewLongRange("r", []int64{-100, 0}, []int64{200, 50})
	if err != nil {
		t.Fatalf("NewLongRange: %v", err)
	}
	if got := r.NumDimensions(); got != 2 {
		t.Fatalf("NumDimensions = %d, want 2", got)
	}
	if got, want := r.GetMin(0), int64(-100); got != want {
		t.Fatalf("GetMin(0) = %d, want %d", got, want)
	}
	if got, want := r.GetMax(0), int64(200); got != want {
		t.Fatalf("GetMax(0) = %d, want %d", got, want)
	}
	if got, want := r.GetMin(1), int64(0); got != want {
		t.Fatalf("GetMin(1) = %d, want %d", got, want)
	}
	if got, want := r.GetMax(1), int64(50); got != want {
		t.Fatalf("GetMax(1) = %d, want %d", got, want)
	}
	if got, want := r.FieldType().PointDimensionCount(), 4; got != want {
		t.Fatalf("PointDimensionCount = %d, want %d", got, want)
	}
}

func TestLongRange_OpenEndedRange(t *testing.T) {
	r, err := NewLongRange("r", []int64{math.MinInt64}, []int64{math.MaxInt64})
	if err != nil {
		t.Fatalf("NewLongRange: %v", err)
	}
	if got, want := r.GetMin(0), int64(math.MinInt64); got != want {
		t.Fatalf("GetMin(0) = %d, want %d", got, want)
	}
	if got, want := r.GetMax(0), int64(math.MaxInt64); got != want {
		t.Fatalf("GetMax(0) = %d, want %d", got, want)
	}
}

func TestLongRange_MinGreaterThanMaxErrors(t *testing.T) {
	_, err := NewLongRange("r", []int64{10}, []int64{5})
	if err == nil {
		t.Fatalf("expected error for min > max")
	}
	if !strings.Contains(err.Error(), "min value") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestLongRange_MismatchedSizesErrors(t *testing.T) {
	_, err := NewLongRange("r", []int64{1, 2}, []int64{3})
	if err == nil {
		t.Fatalf("expected error for mismatched dimension counts")
	}
}

func TestLongRange_EmptyErrors(t *testing.T) {
	if _, err := NewLongRange("r", nil, nil); err == nil {
		t.Fatalf("expected error for nil min/max")
	}
	if _, err := NewLongRange("r", []int64{}, []int64{}); err == nil {
		t.Fatalf("expected error for empty min/max")
	}
}

func TestLongRange_TooManyDimensionsErrors(t *testing.T) {
	min := []int64{0, 0, 0, 0, 0}
	max := []int64{1, 1, 1, 1, 1}
	_, err := NewLongRange("r", min, max)
	if err == nil {
		t.Fatalf("expected error for 5 dimensions")
	}
	if !strings.Contains(err.Error(), "4 dimensions") {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// TestLongRange_EncodeMatchesSortableBytes verifies the wire layout matches
// Lucene's verifyAndEncode: minD0 ... minD{N-1} | maxD0 ... maxD{N-1}, each
// dimension encoded via LongToSortableBytes (sign-flipped big-endian).
func TestLongRange_EncodeMatchesSortableBytes(t *testing.T) {
	min := []int64{-1, 0, 100}
	max := []int64{10, 50, 200}
	got, err := EncodeLongRange(min, max)
	if err != nil {
		t.Fatalf("EncodeLongRange: %v", err)
	}
	want := make([]byte, 2*len(min)*LongRangeBytes)
	for i, v := range min {
		util.LongToSortableBytes(v, want, i*LongRangeBytes)
	}
	for i, v := range max {
		util.LongToSortableBytes(v, want, len(min)*LongRangeBytes+i*LongRangeBytes)
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

func TestLongRange_GetMinOutOfRangePanics(t *testing.T) {
	r, err := NewLongRange("r", []int64{0}, []int64{1})
	if err != nil {
		t.Fatalf("NewLongRange: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for dim out of range")
		}
	}()
	_ = r.GetMin(1)
}

// TestLongRange_DeprecatedAliases ensures the deprecated LongRangeLucene
// type alias and constructor variables still resolve to the new canonical
// implementations.
func TestLongRange_DeprecatedAliases(t *testing.T) {
	r, err := NewLongRangeLucene("r", []int64{-7}, []int64{42})
	if err != nil {
		t.Fatalf("NewLongRangeLucene: %v", err)
	}
	var _ *LongRangeLucene = r // alias is identical type
	if got, want := r.GetMin(0), int64(-7); got != want {
		t.Fatalf("GetMin = %d, want %d", got, want)
	}
	got, err := EncodeLongRangeLucene([]int64{1}, []int64{2})
	if err != nil {
		t.Fatalf("EncodeLongRangeLucene: %v", err)
	}
	want, err := EncodeLongRange([]int64{1}, []int64{2})
	if err != nil {
		t.Fatalf("EncodeLongRange: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("alias encoding mismatch")
	}
}
