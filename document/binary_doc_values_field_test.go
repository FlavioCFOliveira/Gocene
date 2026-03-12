// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewBinaryDocValuesField(t *testing.T) {
	value := []byte("test value")
	field, err := NewBinaryDocValuesField("testField", value)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.Name() != "testField" {
		t.Errorf("Expected field name 'testField', got: %s", field.Name())
	}

	if !bytes.Equal(field.GetValue(), value) {
		t.Errorf("Expected value %v, got: %v", value, field.GetValue())
	}
}

func TestBinaryDocValuesFieldType(t *testing.T) {
	field, err := NewBinaryDocValuesField("testField", []byte("value"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	ft := field.FieldType()

	if ft.Indexed {
		t.Error("Expected field to not be indexed")
	}

	if ft.Stored {
		t.Error("Expected field to not be stored")
	}

	if ft.DocValuesType != index.DocValuesTypeBinary {
		t.Errorf("Expected DocValuesTypeBinary, got: %v", ft.DocValuesType)
	}
}

func TestBinaryDocValuesFieldEmptyValue(t *testing.T) {
	field, err := NewBinaryDocValuesField("testField", []byte{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(field.GetValue()) != 0 {
		t.Errorf("Expected empty value, got: %v", field.GetValue())
	}
}

func TestBinaryDocValuesFieldNilValue(t *testing.T) {
	field, err := NewBinaryDocValuesField("testField", nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.GetValue() != nil {
		t.Errorf("Expected nil value, got: %v", field.GetValue())
	}
}

func TestBinaryDocValuesFieldLargeValue(t *testing.T) {
	value := make([]byte, 1024)
	for i := range value {
		value[i] = byte(i % 256)
	}

	field, err := NewBinaryDocValuesField("testField", value)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !bytes.Equal(field.GetValue(), value) {
		t.Errorf("Expected value to match input")
	}
}
