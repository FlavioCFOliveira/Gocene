// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewNumericDocValuesField(t *testing.T) {
	field, err := NewNumericDocValuesField("testField", 42)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.Name() != "testField" {
		t.Errorf("Expected field name 'testField', got: %s", field.Name())
	}

	if field.GetValue() != 42 {
		t.Errorf("Expected value 42, got: %d", field.GetValue())
	}
}

func TestNumericDocValuesFieldType(t *testing.T) {
	field, err := NewNumericDocValuesField("testField", 123)
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

	if ft.DocValuesType != index.DocValuesTypeNumeric {
		t.Errorf("Expected DocValuesTypeNumeric, got: %v", ft.DocValuesType)
	}
}

func TestNumericDocValuesFieldZeroValue(t *testing.T) {
	field, err := NewNumericDocValuesField("testField", 0)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.GetValue() != 0 {
		t.Errorf("Expected value 0, got: %d", field.GetValue())
	}
}

func TestNumericDocValuesFieldNegativeValue(t *testing.T) {
	field, err := NewNumericDocValuesField("testField", -999)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.GetValue() != -999 {
		t.Errorf("Expected value -999, got: %d", field.GetValue())
	}
}

func TestNumericDocValuesFieldLargeValue(t *testing.T) {
	largeValue := int64(9223372036854775807) // Max int64
	field, err := NewNumericDocValuesField("testField", largeValue)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.GetValue() != largeValue {
		t.Errorf("Expected value %d, got: %d", largeValue, field.GetValue())
	}
}
