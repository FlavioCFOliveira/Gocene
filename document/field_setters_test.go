// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func newStoredType() *FieldType {
	ft := NewFieldType()
	ft.SetStored(true)
	return ft
}

func TestField_SetStringValue(t *testing.T) {
	f, err := NewField("title", "hello", newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	f.SetStringValue("world")
	if got := f.StringValue(); got != "world" {
		t.Fatalf("StringValue = %q, want world", got)
	}
}

func TestField_SetStringValue_PanicsOnWrongType(t *testing.T) {
	f, err := NewField("title", []byte{1, 2, 3}, newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when changing []byte field to string")
		} else if !strings.Contains(r.(string), "cannot change") {
			t.Fatalf("unexpected panic message: %v", r)
		}
	}()
	f.SetStringValue("oops")
}

func TestField_SetBytesValue_OnIndexedPanics(t *testing.T) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	f, err := NewField("data", []byte{1, 2}, ft)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when mutating binary value on indexed field")
		}
	}()
	f.SetBytesValue([]byte{9})
}

func TestField_NumericSetters(t *testing.T) {
	ft := newStoredType()
	f, err := NewField("n", int64(0), ft)
	if err != nil {
		t.Fatal(err)
	}
	f.SetIntValue(42)
	if got, want := f.NumericValue(), int32(42); got != want {
		t.Fatalf("NumericValue = %v (%T), want %v (%T)", got, got, want, want)
	}
	f.SetLongValue(100)
	if got, want := f.NumericValue(), int64(100); got != want {
		t.Fatalf("NumericValue = %v, want %v", got, want)
	}
	f.SetFloatValue(1.5)
	if got, want := f.NumericValue(), float32(1.5); got != want {
		t.Fatalf("NumericValue = %v, want %v", got, want)
	}
	f.SetDoubleValue(2.5)
	if got, want := f.NumericValue(), float64(2.5); got != want {
		t.Fatalf("NumericValue = %v, want %v", got, want)
	}
}

func TestField_NumericSetters_PanicOnStringField(t *testing.T) {
	f, err := NewField("n", "text", newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for SetIntValue on string field")
		}
	}()
	f.SetIntValue(1)
}

func TestField_InvertableType(t *testing.T) {
	f, err := NewField("x", "y", newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	if got := f.InvertableType(); got != InvertableTypeTokenStream {
		t.Fatalf("InvertableType = %v, want TOKEN_STREAM", got)
	}
}

func TestField_GetCharSequenceValue(t *testing.T) {
	f, err := NewField("x", "abc", newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetCharSequenceValue(); got != "abc" {
		t.Fatalf("GetCharSequenceValue = %q", got)
	}
}
