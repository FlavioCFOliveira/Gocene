// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestStoredValue_Constructors(t *testing.T) {
	cases := []struct {
		name string
		v    *StoredValue
		kind StoredValueType
	}{
		{"int", NewStoredValueInt(42), StoredValueTypeInteger},
		{"long", NewStoredValueLong(42), StoredValueTypeLong},
		{"float", NewStoredValueFloat(1.5), StoredValueTypeFloat},
		{"double", NewStoredValueDouble(1.5), StoredValueTypeDouble},
		{"binary", NewStoredValueBinary([]byte{1, 2, 3}), StoredValueTypeBinary},
		{"string", NewStoredValueString("hi"), StoredValueTypeString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.v.GetType(); got != c.kind {
				t.Fatalf("GetType = %v, want %v", got, c.kind)
			}
		})
	}
}

func TestStoredValue_Getters(t *testing.T) {
	if v := NewStoredValueInt(7).GetIntValue(); v != 7 {
		t.Fatalf("int = %d", v)
	}
	if v := NewStoredValueLong(7).GetLongValue(); v != 7 {
		t.Fatalf("long = %d", v)
	}
	if v := NewStoredValueFloat(1.5).GetFloatValue(); v != 1.5 {
		t.Fatalf("float = %v", v)
	}
	if v := NewStoredValueDouble(1.5).GetDoubleValue(); v != 1.5 {
		t.Fatalf("double = %v", v)
	}
	if v := NewStoredValueBinary([]byte{9, 8, 7}).GetBinaryValue(); !bytes.Equal(v, []byte{9, 8, 7}) {
		t.Fatalf("binary = %v", v)
	}
	if v := NewStoredValueString("x").GetStringValue(); v != "x" {
		t.Fatalf("string = %v", v)
	}
}

func TestStoredValue_GetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on mismatched getter")
		}
	}()
	NewStoredValueInt(1).GetStringValue()
}

func TestStoredValue_Setters(t *testing.T) {
	v := NewStoredValueInt(1)
	v.SetIntValue(99)
	if v.GetIntValue() != 99 {
		t.Fatalf("setter int failed")
	}

	s := NewStoredValueString("a")
	s.SetStringValue("b")
	if s.GetStringValue() != "b" {
		t.Fatalf("setter string failed")
	}
}

func TestStoredValue_SetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on mismatched setter")
		}
	}()
	NewStoredValueInt(1).SetStringValue("x")
}

func TestStoredValue_NilBinaryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil binary constructor")
		}
	}()
	NewStoredValueBinary(nil)
}

func TestStoredValue_DataInputRoundTrip(t *testing.T) {
	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	in := store.NewByteArrayDataInput(payload)
	dsi := index.NewStoredFieldDataInputFromByteArray(in)

	v := NewStoredValueDataInput(dsi)
	if v.GetType() != StoredValueTypeDataInput {
		t.Fatalf("GetType = %v, want DATA_INPUT", v.GetType())
	}
	got := v.GetDataInputValue()
	if got.GetLength() != len(payload) {
		t.Fatalf("Length = %d, want %d", got.GetLength(), len(payload))
	}
	if got.DataInput() != in {
		t.Fatalf("DataInput identity lost")
	}

	// Setter on the same kind must accept a fresh value.
	v.SetDataInputValue(index.NewStoredFieldDataInput(in, 2))
	if v.GetDataInputValue().GetLength() != 2 {
		t.Fatalf("after SetDataInputValue length = %d, want 2",
			v.GetDataInputValue().GetLength())
	}
}

func TestStoredValue_NilDataInputPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil DataInput constructor")
		}
	}()
	NewStoredValueDataInput(nil)
}

func TestStoredValue_DataInputGetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when reading DATA_INPUT off an INTEGER")
		}
	}()
	NewStoredValueInt(1).GetDataInputValue()
}
