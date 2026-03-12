// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewSortedSetDocValuesField(t *testing.T) {
	values := [][]byte{
		[]byte("tag1"),
		[]byte("tag2"),
		[]byte("tag3"),
	}

	field, err := NewSortedSetDocValuesField("tags", values)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.Name() != "tags" {
		t.Errorf("Expected field name 'tags', got: %s", field.Name())
	}

	if field.ValueCount() != 3 {
		t.Errorf("Expected 3 values, got: %d", field.ValueCount())
	}

	retrievedValues := field.GetValues()
	if len(retrievedValues) != 3 {
		t.Errorf("Expected 3 retrieved values, got: %d", len(retrievedValues))
	}
}

func TestSortedSetDocValuesFieldType(t *testing.T) {
	values := [][]byte{[]byte("value1"), []byte("value2")}
	field, err := NewSortedSetDocValuesField("testField", values)
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

	if ft.DocValuesType != index.DocValuesTypeSortedSet {
		t.Errorf("Expected DocValuesTypeSortedSet, got: %v", ft.DocValuesType)
	}
}

func TestSortedSetDocValuesFieldEmptyValues(t *testing.T) {
	field, err := NewSortedSetDocValuesField("tags", [][]byte{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.ValueCount() != 0 {
		t.Errorf("Expected 0 values, got: %d", field.ValueCount())
	}

	if len(field.GetValues()) != 0 {
		t.Errorf("Expected empty values slice, got: %v", field.GetValues())
	}
}

func TestSortedSetDocValuesFieldAddValue(t *testing.T) {
	field, err := NewSortedSetDocValuesField("tags", [][]byte{[]byte("initial")})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.ValueCount() != 1 {
		t.Errorf("Expected 1 value, got: %d", field.ValueCount())
	}

	field.AddValue([]byte("added"))

	if field.ValueCount() != 2 {
		t.Errorf("Expected 2 values after adding, got: %d", field.ValueCount())
	}

	values := field.GetValues()
	if len(values) != 2 {
		t.Errorf("Expected 2 values in slice, got: %d", len(values))
	}

	if string(values[1]) != "added" {
		t.Errorf("Expected second value to be 'added', got: %s", string(values[1]))
	}
}

func TestSortedSetDocValuesFieldSingleValue(t *testing.T) {
	values := [][]byte{[]byte("only_value")}

	field, err := NewSortedSetDocValuesField("tags", values)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.ValueCount() != 1 {
		t.Errorf("Expected 1 value, got: %d", field.ValueCount())
	}

	retrievedValues := field.GetValues()
	if len(retrievedValues) != 1 {
		t.Errorf("Expected 1 retrieved value, got: %d", len(retrievedValues))
	}

	if string(retrievedValues[0]) != "only_value" {
		t.Errorf("Expected value 'only_value', got: %s", string(retrievedValues[0]))
	}
}
