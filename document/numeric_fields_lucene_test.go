// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestIntFieldLucene_Type(t *testing.T) {
	f, err := NewIntFieldLucene("n", 42, false)
	if err != nil {
		t.Fatal(err)
	}
	ft := f.FieldType()
	if ft.PointDimensionCount() != 1 || ft.PointNumBytes() != 4 {
		t.Fatalf("dimensions = (%d, %d)", ft.PointDimensionCount(), ft.PointNumBytes())
	}
	if ft.GetDocValuesType() != index.DocValuesTypeSortedNumeric {
		t.Fatalf("docValuesType = %v", ft.GetDocValuesType())
	}
	// Sortable-bytes round-trip
	got := util.SortableBytesToInt(f.BinaryValue(), 0)
	if got != 42 {
		t.Fatalf("decoded = %d", got)
	}
}

func TestLongFieldLucene_Type(t *testing.T) {
	f, err := NewLongFieldLucene("n", -1234567890123, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.FieldType().PointNumBytes() != 8 {
		t.Fatalf("numBytes = %d", f.FieldType().PointNumBytes())
	}
	if !f.FieldType().IsStored() {
		t.Fatalf("stored variant should be stored")
	}
	got := util.SortableBytesToLong(f.BinaryValue(), 0)
	if got != -1234567890123 {
		t.Fatalf("decoded = %d", got)
	}
}

func TestFloatFieldLucene_RoundTrip(t *testing.T) {
	f, err := NewFloatFieldLucene("n", -3.14, false)
	if err != nil {
		t.Fatal(err)
	}
	enc := util.SortableBytesToInt(f.BinaryValue(), 0)
	got := util.SortableIntToFloat(enc)
	if got != -3.14 {
		t.Fatalf("decoded = %v", got)
	}
}

func TestDoubleFieldLucene_RoundTrip(t *testing.T) {
	f, err := NewDoubleFieldLucene("n", -2.71828, true)
	if err != nil {
		t.Fatal(err)
	}
	enc := util.SortableBytesToLong(f.BinaryValue(), 0)
	got := util.SortableLongToDouble(enc)
	if got != -2.71828 {
		t.Fatalf("decoded = %v", got)
	}
}

func TestNumericFieldTYPEs(t *testing.T) {
	pairs := []struct {
		ft   *FieldType
		want int
	}{
		{IntFieldTYPE, 4},
		{IntFieldTYPESTORED, 4},
		{LongFieldTYPE, 8},
		{FloatFieldTYPE, 4},
		{DoubleFieldTYPE, 8},
	}
	for _, p := range pairs {
		if p.ft.PointNumBytes() != p.want {
			t.Fatalf("unexpected numBytes %d", p.ft.PointNumBytes())
		}
		if !p.ft.IsFrozen() {
			t.Fatalf("FieldType must be frozen")
		}
	}
}
