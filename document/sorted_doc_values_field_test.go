// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewSortedDocValuesField(t *testing.T) {
	value := []byte("category1")
	field, err := NewSortedDocValuesField("category", value)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if field.Name() != "category" {
		t.Errorf("Expected field name 'category', got: %s", field.Name())
	}

	if !bytes.Equal(field.GetValue(), value) {
		t.Errorf("Expected value %v, got: %v", value, field.GetValue())
	}
}

func TestSortedDocValuesFieldType(t *testing.T) {
	field, err := NewSortedDocValuesField("testField", []byte("value"))
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

	if ft.DocValuesType != index.DocValuesTypeSorted {
		t.Errorf("Expected DocValuesTypeSorted, got: %v", ft.DocValuesType)
	}
}

func TestSortedDocValuesFieldEmptyValue(t *testing.T) {
	field, err := NewSortedDocValuesField("testField", []byte{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(field.GetValue()) != 0 {
		t.Errorf("Expected empty value, got: %v", field.GetValue())
	}
}

func TestSortedDocValuesFieldMultipleCategories(t *testing.T) {
	categories := [][]byte{
		[]byte("electronics"),
		[]byte("books"),
		[]byte("clothing"),
	}

	for _, cat := range categories {
		field, err := NewSortedDocValuesField("category", cat)
		if err != nil {
			t.Fatalf("Expected no error for category %s, got: %v", cat, err)
		}

		if !bytes.Equal(field.GetValue(), cat) {
			t.Errorf("Expected value %s, got: %s", cat, field.GetValue())
		}
	}
}
