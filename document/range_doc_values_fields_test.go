// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func TestIntRangeDocValuesField(t *testing.T) {
	f, err := NewIntRangeDocValuesField("r", []int32{1, 2}, []int32{10, 20})
	if err != nil {
		t.Fatal(err)
	}
	if f.GetMin(0) != 1 || f.GetMax(0) != 10 || f.GetMin(1) != 2 || f.GetMax(1) != 20 {
		t.Fatalf("min/max mismatch")
	}
}

func TestIntRangeDocValuesField_TooManyDimsErrors(t *testing.T) {
	_, err := NewIntRangeDocValuesField("r", []int32{1, 1, 1, 1, 1}, []int32{2, 2, 2, 2, 2})
	if err == nil {
		t.Fatalf("expected error for > 4 dims")
	}
}

func TestLongRangeDocValuesField(t *testing.T) {
	f, err := NewLongRangeDocValuesField("r", []int64{-1}, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
	if f.GetMin(0) != -1 || f.GetMax(0) != 1 {
		t.Fatalf("min/max mismatch")
	}
}

func TestFloatRangeDocValuesField(t *testing.T) {
	f, err := NewFloatRangeDocValuesField("r", []float32{-1.5}, []float32{1.5})
	if err != nil {
		t.Fatal(err)
	}
	if f.GetMin(0) != -1.5 || f.GetMax(0) != 1.5 {
		t.Fatalf("min/max mismatch")
	}
}

func TestDoubleRangeDocValuesField(t *testing.T) {
	f, err := NewDoubleRangeDocValuesField("r", []float64{-2}, []float64{2})
	if err != nil {
		t.Fatal(err)
	}
	if f.GetMin(0) != -2 || f.GetMax(0) != 2 {
		t.Fatalf("min/max mismatch")
	}
}

func TestRangeFieldQueryType_String(t *testing.T) {
	cases := map[RangeFieldQueryType]string{
		RangeFieldQueryTypeIntersects: "INTERSECTS",
		RangeFieldQueryTypeWithin:     "WITHIN",
		RangeFieldQueryTypeContains:   "CONTAINS",
		RangeFieldQueryTypeCrosses:    "CROSSES",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(v), got, want)
		}
	}
}

func TestRangeFieldQuery_Basic(t *testing.T) {
	enc, err := EncodeIntRangeLucene([]int32{0}, []int32{10})
	if err != nil {
		t.Fatal(err)
	}
	q, err := NewRangeFieldQuery("r", enc, 1, RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatal(err)
	}
	if q.Field() != "r" || q.QueryType() != RangeFieldQueryTypeIntersects || q.NumDims() != 1 {
		t.Fatalf("query attrs wrong")
	}
}

func TestRangeFieldQuery_NumDimsValidation(t *testing.T) {
	if _, err := NewRangeFieldQuery("r", []byte{0, 0}, 5, RangeFieldQueryTypeIntersects); err == nil {
		t.Fatalf("expected error for numDims=5")
	}
	if _, err := NewRangeFieldQuery("", []byte{0}, 1, RangeFieldQueryTypeIntersects); err == nil {
		t.Fatalf("expected error for empty field")
	}
}

func TestRangeFieldQuery_Equals(t *testing.T) {
	enc1, _ := EncodeIntRangeLucene([]int32{0}, []int32{10})
	enc2, _ := EncodeIntRangeLucene([]int32{0}, []int32{10})
	a, _ := NewRangeFieldQuery("r", enc1, 1, RangeFieldQueryTypeIntersects)
	b, _ := NewRangeFieldQuery("r", enc2, 1, RangeFieldQueryTypeIntersects)
	if !a.Equals(b) {
		t.Fatalf("equal queries should match")
	}
	c, _ := NewRangeFieldQuery("other", enc1, 1, RangeFieldQueryTypeIntersects)
	if a.Equals(c) {
		t.Fatalf("different fields should differ")
	}
}

func TestBinaryRangeDocValues(t *testing.T) {
	enc, _ := EncodeIntRangeLucene([]int32{1}, []int32{10})
	b := NewBinaryRangeDocValues(enc, 1, 4)
	if b.NumDims() != 1 || b.NumBytesPerDimension() != 4 {
		t.Fatalf("attrs mismatch")
	}
	if len(b.PackedValue()) != 8 {
		t.Fatalf("packed length wrong")
	}
}
