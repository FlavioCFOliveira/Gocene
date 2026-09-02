// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

func TestXYPointField_RoundTrip(t *testing.T) {
	p, err := NewXYPointField("xy", 1.5, -2.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BinaryValue()) != 8 {
		t.Fatalf("encoded len = %d", len(p.BinaryValue()))
	}
	x, y, err := DecodeXY(p.BinaryValue())
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(x-1.5)) > 1e-5 || math.Abs(float64(y+2.5)) > 1e-5 {
		t.Fatalf("decoded (%v, %v) wrong", x, y)
	}
}

func TestXYPointField_NaNErrors(t *testing.T) {
	if _, err := NewXYPointField("xy", float32(math.NaN()), 0); err == nil {
		t.Fatalf("expected error for NaN")
	}
}

func TestXYDocValuesField_RoundTrip(t *testing.T) {
	f, err := NewXYDocValuesField("xy", 3.5, 4.5)
	if err != nil {
		t.Fatal(err)
	}
	encoded := f.NumericValue().(int64)
	x, y := DecodeXYFromLong(encoded)
	if math.Abs(float64(x-3.5)) > 1e-5 || math.Abs(float64(y-4.5)) > 1e-5 {
		t.Fatalf("decoded (%v, %v) wrong", x, y)
	}
}

func TestXYDocValuesPointInGeometryQuery_Basic(t *testing.T) {
	c, err := geo.NewXYCircle(0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	q, err := NewXYDocValuesPointInGeometryQuery("xy", c)
	if err != nil {
		t.Fatal(err)
	}
	if q.Field() != "xy" || len(q.Geometries()) != 1 {
		t.Fatalf("bad query state")
	}
}

func TestXYDocValuesPointInGeometryQuery_Validation(t *testing.T) {
	if _, err := NewXYDocValuesPointInGeometryQuery("", nil); err == nil {
		t.Fatalf("expected error for empty field")
	}
	if _, err := NewXYDocValuesPointInGeometryQuery("f"); err == nil {
		t.Fatalf("expected error for missing geometries")
	}
	if _, err := NewXYDocValuesPointInGeometryQuery("f", nil); err == nil {
		t.Fatalf("expected error for nil geometry")
	}
}
