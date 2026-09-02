// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func TestEncodeDecodeIntLucene(t *testing.T) {
	cases := []int32{-2147483648, -1, 0, 1, 2147483647, 123456789}
	for _, v := range cases {
		b := make([]byte, 4)
		EncodeDimensionIntLucene(v, b, 0)
		got := DecodeDimensionIntLucene(b, 0)
		if got != v {
			t.Fatalf("round-trip int %d -> %d", v, got)
		}
	}
}

func TestSortableOrdering_Int(t *testing.T) {
	a := PackIntsLucene(-100)
	b := PackIntsLucene(0)
	c := PackIntsLucene(100)
	// unsigned byte-order must be a < b < c
	if string(a) >= string(b) || string(b) >= string(c) {
		t.Fatalf("ordering broken: %v %v %v", a, b, c)
	}
}

func TestEncodeDecodeLongLucene(t *testing.T) {
	cases := []int64{-9223372036854775808, -1, 0, 1, 9223372036854775807}
	for _, v := range cases {
		b := make([]byte, 8)
		EncodeDimensionLongLucene(v, b, 0)
		if DecodeDimensionLongLucene(b, 0) != v {
			t.Fatalf("round-trip long %d", v)
		}
	}
}

func TestEncodeDecodeFloatLucene(t *testing.T) {
	cases := []float32{-1e30, -1, 0, 1, 1e30}
	for _, v := range cases {
		b := make([]byte, 4)
		EncodeDimensionFloatLucene(v, b, 0)
		if DecodeDimensionFloatLucene(b, 0) != v {
			t.Fatalf("round-trip float %v", v)
		}
	}
}

func TestEncodeDecodeDoubleLucene(t *testing.T) {
	cases := []float64{-1e300, -1, 0, 1, 1e300}
	for _, v := range cases {
		b := make([]byte, 8)
		EncodeDimensionDoubleLucene(v, b, 0)
		if DecodeDimensionDoubleLucene(b, 0) != v {
			t.Fatalf("round-trip double %v", v)
		}
	}
}

func TestNewIntPointLucene_MultiDim(t *testing.T) {
	p, err := NewIntPointLucene("xy", 10, -20, 30)
	if err != nil {
		t.Fatal(err)
	}
	if p.NumDimensions() != 3 || p.BytesPerDimension() != 4 {
		t.Fatalf("dims = (%d, %d)", p.NumDimensions(), p.BytesPerDimension())
	}
	if len(p.PointValues()) != 12 {
		t.Fatalf("packed length = %d", len(p.PointValues()))
	}
}

func TestNewXxxPointLucene_NoValuesErrors(t *testing.T) {
	if _, err := NewIntPointLucene("k"); err == nil {
		t.Fatalf("expected error for empty IntPoint")
	}
	if _, err := NewLongPointLucene("k"); err == nil {
		t.Fatalf("expected error for empty LongPoint")
	}
	if _, err := NewFloatPointLucene("k"); err == nil {
		t.Fatalf("expected error for empty FloatPoint")
	}
	if _, err := NewDoublePointLucene("k"); err == nil {
		t.Fatalf("expected error for empty DoublePoint")
	}
}
