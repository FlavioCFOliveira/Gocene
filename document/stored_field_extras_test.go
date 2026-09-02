// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"
)

func TestStoredField_BytesOffset(t *testing.T) {
	src := []byte{1, 2, 3, 4, 5}
	f, err := NewStoredFieldFromBytesOffset("k", src, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.BinaryValue(), []byte{2, 3, 4}) {
		t.Fatalf("BinaryValue = %v, want [2 3 4]", f.BinaryValue())
	}
	if _, err := NewStoredFieldFromBytesOffset("k", src, -1, 1); err == nil {
		t.Fatalf("expected error for negative offset")
	}
	if _, err := NewStoredFieldFromBytesOffset("k", src, 4, 5); err == nil {
		t.Fatalf("expected error for over-cap range")
	}
}

func TestStoredField_Float32(t *testing.T) {
	f, err := NewStoredFieldFromFloat32("k", 1.5)
	if err != nil {
		t.Fatal(err)
	}
	// NewField widens float32 -> float64 (pre-existing Gocene behaviour).
	if got, want := f.NumericValue(), float64(1.5); got != want {
		t.Fatalf("NumericValue = %v, want %v", got, want)
	}
}

func TestStoredField_StoredValue(t *testing.T) {
	cases := []struct {
		name string
		f    func() *StoredField
		kind StoredValueType
	}{
		{
			"string",
			func() *StoredField { f, _ := NewStoredField("k", "v"); return f },
			StoredValueTypeString,
		},
		{
			"binary",
			func() *StoredField { f, _ := NewStoredFieldFromBytes("k", []byte{1}); return f },
			StoredValueTypeBinary,
		},
		{
			"long",
			func() *StoredField { f, _ := NewStoredFieldFromInt64("k", 42); return f },
			StoredValueTypeLong,
		},
		{
			// NewField widens float32 -> float64, so StoredValue sees a DOUBLE.
			"float32",
			func() *StoredField { f, _ := NewStoredFieldFromFloat32("k", 1.5); return f },
			StoredValueTypeDouble,
		},
		{
			"double",
			func() *StoredField { f, _ := NewStoredFieldFromFloat64("k", 1.5); return f },
			StoredValueTypeDouble,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sv := c.f().StoredValue()
			if sv == nil {
				t.Fatalf("StoredValue is nil")
			}
			if sv.GetType() != c.kind {
				t.Fatalf("type = %v, want %v", sv.GetType(), c.kind)
			}
		})
	}
}

func TestStoredField_TYPEAlias(t *testing.T) {
	if StoredFieldTYPE != StoredFieldType {
		t.Fatalf("TYPE alias must match StoredFieldType")
	}
}
