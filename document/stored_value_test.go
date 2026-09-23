// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestStoredValue_Constructors(t *testing.T) {
	cases := []struct {
		name string
		v    *document.StoredValue
		kind document.StoredValueType
	}{
		{"int", document.NewStoredValueInt(42), document.StoredValueTypeInteger},
		{"long", document.NewStoredValueLong(42), document.StoredValueTypeLong},
		{"float", document.NewStoredValueFloat(1.5), document.StoredValueTypeFloat},
		{"double", document.NewStoredValueDouble(1.5), document.StoredValueTypeDouble},
		{"binary", document.NewStoredValueBinary([]byte{1, 2, 3}), document.StoredValueTypeBinary},
		{"string", document.NewStoredValueString("hi"), document.StoredValueTypeString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.v.Type(); got != c.kind {
				t.Fatalf("GetType = %v, want %v", got, c.kind)
			}
		})
	}
}

func TestStoredValue_Getters(t *testing.T) {
	if v := document.NewStoredValueInt(7).IntValue(); v != 7 {
		t.Fatalf("int = %d", v)
	}
	if v := document.NewStoredValueLong(7).LongValue(); v != 7 {
		t.Fatalf("long = %d", v)
	}
	if v := document.NewStoredValueFloat(1.5).FloatValue(); v != 1.5 {
		t.Fatalf("float = %v", v)
	}
	if v := document.NewStoredValueDouble(1.5).DoubleValue(); v != 1.5 {
		t.Fatalf("double = %v", v)
	}
	if v := document.NewStoredValueBinary([]byte{9, 8, 7}).BinaryValue(); !bytes.Equal(v, []byte{9, 8, 7}) {
		t.Fatalf("binary = %v", v)
	}
	if v := document.NewStoredValueString("x").StringValue(); v != "x" {
		t.Fatalf("string = %v", v)
	}
}

func TestStoredValue_GetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on mismatched getter")
		}
	}()
	document.NewStoredValueInt(1).StringValue()
}

func TestStoredValue_Setters(t *testing.T) {
	v := document.NewStoredValueInt(1)
	v.SetIntValue(99)
	if v.IntValue() != 99 {
		t.Fatalf("setter int failed")
	}

	s := document.NewStoredValueString("a")
	s.SetStringValue("b")
	if s.StringValue() != "b" {
		t.Fatalf("setter string failed")
	}
}

func TestStoredValue_SetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on mismatched setter")
		}
	}()
	document.NewStoredValueInt(1).SetStringValue("x")
}

func TestStoredValue_NilBinaryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil binary constructor")
		}
	}()
	document.NewStoredValueBinary(nil)
}

func TestStoredValue_DataInputRoundTrip(t *testing.T) {
	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	in := store.NewByteArrayDataInput(payload)
	dsi := spi.NewStoredFieldDataInput(in, in.Length())

	v := document.NewStoredValueDataInput(dsi)
	if v.Type() != document.StoredValueTypeDataInput {
		t.Fatalf("GetType = %v, want DATA_INPUT", v.Type())
	}
	got := v.DataInputValue()
	if got.GetLength() != len(payload) {
		t.Fatalf("Length = %d, want %d", got.GetLength(), len(payload))
	}
	if got.DataInput() != in {
		t.Fatalf("DataInput identity lost")
	}

	// Setter on the same kind must accept a fresh value.
	v.SetDataInputValue(spi.NewStoredFieldDataInput(in, 2))
	if v.DataInputValue().GetLength() != 2 {
		t.Fatalf("after SetDataInputValue length = %d, want 2",
			v.DataInputValue().GetLength())
	}
}

func TestStoredValue_NilDataInputPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil DataInput constructor")
		}
	}()
	document.NewStoredValueDataInput(nil)
}

func TestStoredValue_DataInputGetterTypeMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when reading DATA_INPUT off an INTEGER")
		}
	}()
	document.NewStoredValueInt(1).DataInputValue()
}
