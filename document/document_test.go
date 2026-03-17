// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewDocument(t *testing.T) {
	doc := NewDocument()
	if doc == nil {
		t.Fatal("Expected non-nil Document")
	}
	if !doc.IsEmpty() {
		t.Error("New document should be empty")
	}
}

func TestDocument_Add(t *testing.T) {
	doc := NewDocument()

	field, _ := NewTextField("title", "Hello World", true)
	doc.Add(field)

	if doc.Size() != 1 {
		t.Errorf("Expected size 1, got %d", doc.Size())
	}

	// Test panic on nil
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on nil field")
		}
	}()
	doc.Add(nil)
}

func TestDocument_AddField(t *testing.T) {
	doc := NewDocument()

	err := doc.AddField("title", "Hello World", TextFieldTypeStored)
	if err != nil {
		t.Fatalf("AddField failed: %v", err)
	}

	if doc.Size() != 1 {
		t.Errorf("Expected size 1, got %d", doc.Size())
	}
}

func TestDocument_Get(t *testing.T) {
	doc := NewDocument()

	// Get from empty document
	if doc.Get("title") != nil {
		t.Error("Expected nil for non-existent field")
	}

	// Add and get
	field, _ := NewTextField("title", "Hello World", true)
	doc.Add(field)

	got := doc.Get("title")
	if got == nil {
		t.Fatal("Expected to find field")
	}
	if got.Name() != "title" {
		t.Errorf("Expected name 'title', got '%s'", got.Name())
	}
}

func TestDocument_GetFields(t *testing.T) {
	doc := NewDocument()

	// Add multiple fields with same name
	doc.AddField("tag", "go", StringFieldTypeStored)
	doc.AddField("tag", "lucene", StringFieldTypeStored)
	doc.AddField("tag", "search", StringFieldTypeStored)

	fields := doc.GetFieldsByName("tag")
	if len(fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(fields))
	}

	// Get non-existent
	fields = doc.GetFieldsByName("nonexistent")
	if len(fields) != 0 {
		t.Error("Expected empty slice for non-existent field")
	}
}

func TestDocument_GetAllFields(t *testing.T) {
	doc := NewDocument()
	doc.AddField("title", "Hello", TextFieldTypeStored)
	doc.AddField("body", "World", TextFieldTypeStored)

	fields := doc.GetAllFields()
	if len(fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(fields))
	}

	// Verify it's a copy
	fields[0] = nil
	if doc.Size() != 2 {
		t.Error("GetAllFields should return a copy")
	}
}

func TestDocument_GetFieldNames(t *testing.T) {
	doc := NewDocument()
	doc.AddField("title", "Hello", TextFieldTypeStored)
	doc.AddField("body", "World", TextFieldTypeStored)
	doc.AddField("title", "Duplicate", TextFieldTypeStored) // Same name

	names := doc.GetFieldNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 unique names, got %d", len(names))
	}
}

func TestDocument_RemoveField(t *testing.T) {
	doc := NewDocument()
	doc.AddField("title", "Hello", TextFieldTypeStored)

	removed := doc.RemoveField("title")
	if !removed {
		t.Error("Expected field to be removed")
	}
	if doc.Size() != 0 {
		t.Error("Expected document to be empty after removal")
	}

	// Remove non-existent
	removed = doc.RemoveField("nonexistent")
	if removed {
		t.Error("Expected false for non-existent field")
	}
}

func TestDocument_RemoveFields(t *testing.T) {
	doc := NewDocument()
	doc.AddField("tag", "go", StringFieldTypeStored)
	doc.AddField("tag", "lucene", StringFieldTypeStored)
	doc.AddField("title", "Hello", TextFieldTypeStored)

	count := doc.RemoveFields("tag")
	if count != 2 {
		t.Errorf("Expected 2 fields removed, got %d", count)
	}
	if doc.Size() != 1 {
		t.Errorf("Expected 1 field remaining, got %d", doc.Size())
	}
}

// Note: TestDocument_GetValues and TestDocument_Clear are also defined in document_extended_test.go

func TestDocument_HasField(t *testing.T) {
	doc := NewDocument()
	doc.AddField("title", "Hello", TextFieldTypeStored)

	if !doc.HasField("title") {
		t.Error("Expected HasField to return true")
	}
	if doc.HasField("nonexistent") {
		t.Error("Expected HasField to return false")
	}
}

func TestDocument_GetFieldCount(t *testing.T) {
	doc := NewDocument()
	doc.AddField("tag", "go", StringFieldTypeStored)
	doc.AddField("tag", "lucene", StringFieldTypeStored)
	doc.AddField("title", "Hello", TextFieldTypeStored)

	count := doc.GetFieldCount("tag")
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

// FieldType Tests

func TestNewFieldType(t *testing.T) {
	ft := NewFieldType()
	if ft == nil {
		t.Fatal("Expected non-nil FieldType")
	}
	if ft.Indexed {
		t.Error("Expected Indexed to be false by default")
	}
	if ft.Stored {
		t.Error("Expected Stored to be false by default")
	}
}

func TestFieldType_Setters(t *testing.T) {
	ft := NewFieldType()

	ft.SetIndexed(true).
		SetStored(true).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqs)

	if !ft.Indexed {
		t.Error("Expected Indexed to be true")
	}
	if !ft.Stored {
		t.Error("Expected Stored to be true")
	}
	if !ft.Tokenized {
		t.Error("Expected Tokenized to be true")
	}
	if ft.IndexOptions != index.IndexOptionsDocsAndFreqs {
		t.Error("Expected IndexOptions to be set")
	}
}

func TestFieldType_Freeze(t *testing.T) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.Freeze()

	if !ft.IsFrozen() {
		t.Error("Expected FieldType to be frozen")
	}

	// Should panic when trying to modify frozen FieldType
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on modifying frozen FieldType")
		}
	}()
	ft.SetIndexed(false)
}

func TestFieldType_Validate(t *testing.T) {
	// Valid configuration
	ft := NewFieldType()
	ft.SetIndexed(true).
		SetIndexOptions(index.IndexOptionsDocs)
	if err := ft.Validate(); err != nil {
		t.Errorf("Expected valid FieldType: %v", err)
	}

	// Invalid: indexed but IndexOptionsNone
	ft2 := NewFieldType()
	ft2.SetIndexed(true)
	if err := ft2.Validate(); err == nil {
		t.Error("Expected validation error for indexed field with IndexOptionsNone")
	}

	// Invalid: tokenized but not indexed
	ft3 := NewFieldType()
	ft3.SetTokenized(true)
	if err := ft3.Validate(); err == nil {
		t.Error("Expected validation error for tokenized non-indexed field")
	}
}

// TextField Tests

func TestNewTextField(t *testing.T) {
	// Stored
	field, err := NewTextField("title", "Hello World", true)
	if err != nil {
		t.Fatalf("Failed to create TextField: %v", err)
	}
	if field.Name() != "title" {
		t.Errorf("Expected name 'title', got '%s'", field.Name())
	}
	if field.StringValue() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", field.StringValue())
	}
	if !field.FieldType().Stored {
		t.Error("Expected field to be stored")
	}
	if !field.FieldType().Tokenized {
		t.Error("Expected field to be tokenized")
	}

	// Not stored
	field2, _ := NewTextField("body", "Content", false)
	if field2.FieldType().Stored {
		t.Error("Expected field to not be stored")
	}
}

// StringField Tests

func TestNewStringField(t *testing.T) {
	field, err := NewStringField("id", "doc-123", true)
	if err != nil {
		t.Fatalf("Failed to create StringField: %v", err)
	}
	if field.Name() != "id" {
		t.Errorf("Expected name 'id', got '%s'", field.Name())
	}
	if field.StringValue() != "doc-123" {
		t.Errorf("Expected 'doc-123', got '%s'", field.StringValue())
	}
	if !field.FieldType().Indexed {
		t.Error("Expected field to be indexed")
	}
	if field.FieldType().Tokenized {
		t.Error("Expected field to not be tokenized")
	}
	if !field.FieldType().OmitNorms {
		t.Error("Expected field to omit norms")
	}
}

func TestNewStringFieldFromBytes(t *testing.T) {
	field, err := NewStringFieldFromBytes("id", []byte("doc-456"), true)
	if err != nil {
		t.Fatalf("Failed to create StringField from bytes: %v", err)
	}
	if field.StringValue() != "doc-456" {
		t.Errorf("Expected 'doc-456', got '%s'", field.StringValue())
	}
}

// StoredField Tests

func TestNewStoredField(t *testing.T) {
	field, err := NewStoredField("metadata", "some data")
	if err != nil {
		t.Fatalf("Failed to create StoredField: %v", err)
	}
	if field.Name() != "metadata" {
		t.Errorf("Expected name 'metadata', got '%s'", field.Name())
	}
	if field.FieldType().Indexed {
		t.Error("Expected field to not be indexed")
	}
	if !field.FieldType().Stored {
		t.Error("Expected field to be stored")
	}
}

func TestNewStoredFieldFromInt(t *testing.T) {
	field, err := NewStoredFieldFromInt("count", 42)
	if err != nil {
		t.Fatalf("Failed to create StoredField from int: %v", err)
	}
	if field.NumericValue() != int64(42) {
		t.Errorf("Expected 42, got %v", field.NumericValue())
	}
}

func TestNewStoredFieldFromInt64(t *testing.T) {
	field, err := NewStoredFieldFromInt64("timestamp", 1234567890)
	if err != nil {
		t.Fatalf("Failed to create StoredField from int64: %v", err)
	}
	if field.NumericValue() != int64(1234567890) {
		t.Errorf("Expected 1234567890, got %v", field.NumericValue())
	}
}

func TestNewStoredFieldFromFloat64(t *testing.T) {
	field, err := NewStoredFieldFromFloat64("score", 3.14)
	if err != nil {
		t.Fatalf("Failed to create StoredField from float64: %v", err)
	}
	if field.NumericValue() != 3.14 {
		t.Errorf("Expected 3.14, got %v", field.NumericValue())
	}
}

// Field Base Class Tests

func TestNewField(t *testing.T) {
	// String value
	ft := NewFieldType()
	ft.SetStored(true)
	field, err := NewField("name", "value", ft)
	if err != nil {
		t.Fatalf("Failed to create Field: %v", err)
	}
	if field.StringValue() != "value" {
		t.Errorf("Expected 'value', got '%s'", field.StringValue())
	}

	// Byte slice
	field2, err := NewField("data", []byte("binary"), ft)
	if err != nil {
		t.Fatalf("Failed to create Field from bytes: %v", err)
	}
	if string(field2.BinaryValue()) != "binary" {
		t.Errorf("Expected 'binary', got '%s'", string(field2.BinaryValue()))
	}

	// Int
	field3, err := NewField("num", 42, ft)
	if err != nil {
		t.Fatalf("Failed to create Field from int: %v", err)
	}
	if field3.NumericValue() != int64(42) {
		t.Errorf("Expected 42, got %v", field3.NumericValue())
	}

	// Error cases
	_, err = NewField("", "value", ft)
	if err == nil {
		t.Error("Expected error for empty name")
	}

	_, err = NewField("name", "value", nil)
	if err == nil {
		t.Error("Expected error for nil FieldType")
	}
}
