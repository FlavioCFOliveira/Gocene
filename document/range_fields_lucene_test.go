// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func TestIntRangeLucene_BasicRoundTrip(t *testing.T) {
	r, err := NewIntRangeLucene("r", []int32{-5, 0}, []int32{10, 100})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMin(0) != -5 || r.GetMax(0) != 10 || r.GetMin(1) != 0 || r.GetMax(1) != 100 {
		t.Fatalf("min/max mismatch")
	}
	if r.FieldType().PointDimensionCount() != 4 {
		t.Fatalf("dim count = %d", r.FieldType().PointDimensionCount())
	}
}

func TestIntRangeLucene_MinGreaterMaxErrors(t *testing.T) {
	_, err := NewIntRangeLucene("r", []int32{10}, []int32{5})
	if err == nil {
		t.Fatalf("expected error for min > max")
	}
}

func TestIntRangeLucene_MismatchedSizesErrors(t *testing.T) {
	_, err := NewIntRangeLucene("r", []int32{1, 2}, []int32{3})
	if err == nil {
		t.Fatalf("expected error for mismatched sizes")
	}
}

func TestLongRangeLucene_BasicRoundTrip(t *testing.T) {
	r, err := NewLongRangeLucene("r", []int64{-100}, []int64{200})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMin(0) != -100 || r.GetMax(0) != 200 {
		t.Fatalf("min/max mismatch")
	}
}

func TestFloatRangeLucene_BasicRoundTrip(t *testing.T) {
	r, err := NewFloatRangeLucene("r", []float32{-1.5}, []float32{2.5})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMin(0) != -1.5 || r.GetMax(0) != 2.5 {
		t.Fatalf("min/max mismatch")
	}
}

func TestDoubleRangeLucene_BasicRoundTrip(t *testing.T) {
	r, err := NewDoubleRangeLucene("r", []float64{-1.5, 0}, []float64{2.5, 10})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMax(1) != 10 || r.GetMin(0) != -1.5 {
		t.Fatalf("min/max mismatch")
	}
}

func TestRangeLuceneEncoding_PackedSize(t *testing.T) {
	b, err := EncodeIntRangeLucene([]int32{0, 0}, []int32{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(b), 16; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
}
