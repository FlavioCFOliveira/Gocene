// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains comprehensive tests for DocValues functionality.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDocValues
// and related test files:
//   - TestDocValues.java
//   - TestNumericDocValuesUpdates.java
//   - TestMultiDocValues.java
//
// GC-119: Index Tests - DocValues Comprehensive
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDocValuesBasics tests basic DocValues operations.
func TestDocValuesBasics(t *testing.T) {
	t.Run("create numeric doc values field", func(t *testing.T) {
		field, err := document.NewNumericDocValuesField("field", 42)
		if err != nil {
			t.Fatalf("NewNumericDocValuesField() error = %v", err)
		}
		if field == nil {
			t.Fatal("NewNumericDocValuesField() returned nil")
		}
		if field.Name() != "field" {
			t.Errorf("Name() = %s, want field", field.Name())
		}
	})

	t.Run("create binary doc values field", func(t *testing.T) {
		data := []byte("test data")
		field, err := document.NewBinaryDocValuesField("field", data)
		if err != nil {
			t.Fatalf("NewBinaryDocValuesField() error = %v", err)
		}
		if field == nil {
			t.Fatal("NewBinaryDocValuesField() returned nil")
		}
		if field.Name() != "field" {
			t.Errorf("Name() = %s, want field", field.Name())
		}
	})

	t.Run("create sorted doc values field", func(t *testing.T) {
		field, err := document.NewSortedDocValuesField("field", []byte("value"))
		if err != nil {
			t.Fatalf("NewSortedDocValuesField() error = %v", err)
		}
		if field == nil {
			t.Fatal("NewSortedDocValuesField() returned nil")
		}
		if field.Name() != "field" {
			t.Errorf("Name() = %s, want field", field.Name())
		}
	})

	t.Run("create sorted set doc values field", func(t *testing.T) {
		values := [][]byte{[]byte("value1"), []byte("value2")}
		field, err := document.NewSortedSetDocValuesField("field", values)
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField() error = %v", err)
		}
		if field == nil {
			t.Fatal("NewSortedSetDocValuesField() returned nil")
		}
		if field.Name() != "field" {
			t.Errorf("Name() = %s, want field", field.Name())
		}
	})
}

// TestDocValuesIntegration tests DocValues with IndexWriter.
func TestDocValuesIntegration(t *testing.T) {
	t.Run("index document with doc values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Create document with DocValues
		numericField, _ := document.NewNumericDocValuesField("id", 1)
		sortedField, _ := document.NewSortedDocValuesField("category", []byte("test"))
		doc := &testDocumentWithFields{
			fields: []interface{}{
				numericField,
				sortedField,
			},
		}

		// Add document
		err := writer.AddDocument(doc)
		if err != nil {
			t.Errorf("AddDocument() error = %v", err)
		}

		// Commit
		err = writer.Commit()
		if err != nil {
			t.Errorf("Commit() error = %v", err)
		}
	})

	t.Run("multiple documents with doc values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add multiple documents
		for i := 0; i < 10; i++ {
			numericField, _ := document.NewNumericDocValuesField("id", int64(i))
			sortedField, _ := document.NewSortedDocValuesField("category", []byte("category"))
			doc := &testDocumentWithFields{
				fields: []interface{}{
					numericField,
					sortedField,
				},
			}
			writer.AddDocument(doc)
		}

		writer.Commit()
	})

	t.Run("doc values with updates", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add document
		numericField, _ := document.NewNumericDocValuesField("id", 1)
		sortedField, _ := document.NewSortedDocValuesField("category", []byte("old"))
		doc := &testDocumentWithFields{
			fields: []interface{}{
				numericField,
				sortedField,
			},
		}
		writer.AddDocument(doc)
		writer.Commit()

		// Update document
		updatedNumericField, _ := document.NewNumericDocValuesField("id", 1)
		updatedSortedField, _ := document.NewSortedDocValuesField("category", []byte("new"))
		updatedDoc := &testDocumentWithFields{
			fields: []interface{}{
				updatedNumericField,
				updatedSortedField,
			},
		}
		term := index.NewTerm("id", "1")
		writer.UpdateDocument(term, updatedDoc)
		writer.Commit()
	})
}

// TestDocValuesTypes tests different DocValues types.
func TestDocValuesTypes(t *testing.T) {
	t.Run("binary doc values with different sizes", func(t *testing.T) {
		// Empty
		emptyField, err := document.NewBinaryDocValuesField("empty", []byte{})
		if err != nil {
			t.Errorf("NewBinaryDocValuesField() with empty data error = %v", err)
		}
		if emptyField == nil {
			t.Error("NewBinaryDocValuesField() with empty data returned nil")
		}

		// Small
		smallField, err := document.NewBinaryDocValuesField("small", []byte("x"))
		if err != nil {
			t.Errorf("NewBinaryDocValuesField() with small data error = %v", err)
		}
		if smallField == nil {
			t.Error("NewBinaryDocValuesField() with small data returned nil")
		}

		// Large
		largeData := make([]byte, 1000)
		largeField, err := document.NewBinaryDocValuesField("large", largeData)
		if err != nil {
			t.Errorf("NewBinaryDocValuesField() with large data error = %v", err)
		}
		if largeField == nil {
			t.Error("NewBinaryDocValuesField() with large data returned nil")
		}
	})

	t.Run("sorted set doc values with multiple values", func(t *testing.T) {
		// Single value
		singleValue := [][]byte{[]byte("one")}
		singleField, err := document.NewSortedSetDocValuesField("single", singleValue)
		if err != nil {
			t.Errorf("NewSortedSetDocValuesField() with single value error = %v", err)
		}
		if singleField == nil {
			t.Error("NewSortedSetDocValuesField() with single value returned nil")
		}

		// Multiple values
		multiValues := [][]byte{[]byte("one"), []byte("two"), []byte("three")}
		multiField, err := document.NewSortedSetDocValuesField("multi", multiValues)
		if err != nil {
			t.Errorf("NewSortedSetDocValuesField() with multiple values error = %v", err)
		}
		if multiField == nil {
			t.Error("NewSortedSetDocValuesField() with multiple values returned nil")
		}

		// Empty values
		emptyValues := [][]byte{}
		emptyField, err := document.NewSortedSetDocValuesField("empty", emptyValues)
		if err != nil {
			t.Errorf("NewSortedSetDocValuesField() with empty values error = %v", err)
		}
		if emptyField == nil {
			t.Error("NewSortedSetDocValuesField() with empty values returned nil")
		}
	})
}

// TestDocValuesFieldType tests DocValues field type properties.
func TestDocValuesFieldType(t *testing.T) {
	t.Run("numeric doc values field type", func(t *testing.T) {
		field, _ := document.NewNumericDocValuesField("field", 42)
		ft := field.FieldType()

		if ft == nil {
			t.Fatal("FieldType() returned nil")
		}

		if ft.DocValuesType == index.DocValuesTypeNone {
			t.Error("DocValuesType should not be None")
		}
	})

	t.Run("binary doc values field type", func(t *testing.T) {
		field, _ := document.NewBinaryDocValuesField("field", []byte("data"))
		ft := field.FieldType()

		if ft == nil {
			t.Fatal("FieldType() returned nil")
		}

		if ft.DocValuesType == index.DocValuesTypeNone {
			t.Error("DocValuesType should not be None")
		}
	})

	t.Run("sorted doc values field type", func(t *testing.T) {
		field, _ := document.NewSortedDocValuesField("field", []byte("value"))
		ft := field.FieldType()

		if ft == nil {
			t.Fatal("FieldType() returned nil")
		}

		if ft.DocValuesType == index.DocValuesTypeNone {
			t.Error("DocValuesType should not be None")
		}
	})
}

// TestDocValuesWithDeletes tests DocValues behavior with deleted documents.
func TestDocValuesWithDeletes(t *testing.T) {
	t.Run("doc values survive document deletion", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add documents
		for i := 0; i < 5; i++ {
			numericField, _ := document.NewNumericDocValuesField("id", int64(i))
			sortedField, _ := document.NewSortedDocValuesField("name", []byte("doc"))
			doc := &testDocumentWithFields{
				fields: []interface{}{
					numericField,
					sortedField,
				},
			}
			writer.AddDocument(doc)
		}

		// Delete some documents
		term := index.NewTerm("id", "2")
		writer.DeleteDocuments(term)

		// Commit
		writer.Commit()

		// DocValues should still be accessible for non-deleted documents
		// This would be verified via DirectoryReader in full implementation
		t.Skip("Full verification requires DirectoryReader implementation")
	})
}

// TestMultiDocValues tests multi-valued DocValues.
func TestMultiDocValues(t *testing.T) {
	t.Run("sorted set with many values", func(t *testing.T) {
		// Create many values
		values := make([][]byte, 100)
		for i := 0; i < 100; i++ {
			values[i] = []byte("value")
		}

		field, err := document.NewSortedSetDocValuesField("field", values)
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField() with many values error = %v", err)
		}
		if field == nil {
			t.Fatal("NewSortedSetDocValuesField() with many values returned nil")
		}
	})
}

// TestNumericDocValuesUpdates tests numeric DocValues updates.
func TestNumericDocValuesUpdates(t *testing.T) {
	t.Run("update numeric doc values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add document
		numericField, _ := document.NewNumericDocValuesField("id", 1)
		countField, _ := document.NewNumericDocValuesField("count", 10)
		doc := &testDocumentWithFields{
			fields: []interface{}{
				numericField,
				countField,
			},
		}
		writer.AddDocument(doc)
		writer.Commit()

		// Update just the count field
		updatedNumericField, _ := document.NewNumericDocValuesField("id", 1)
		updatedCountField, _ := document.NewNumericDocValuesField("count", 20)
		updatedDoc := &testDocumentWithFields{
			fields: []interface{}{
				updatedNumericField,
				updatedCountField,
			},
		}
		term := index.NewTerm("id", "1")
		writer.UpdateDocument(term, updatedDoc)
		writer.Commit()
	})
}

// TestDocValuesFieldInfo tests DocValues in FieldInfo.
func TestDocValuesFieldInfo(t *testing.T) {
	t.Run("field info with doc values type", func(t *testing.T) {
		// Create field info for DocValues field
		opts := index.FieldInfoOptions{
			DocValuesType: index.DocValuesTypeNumeric,
		}
		fi := index.NewFieldInfo("test", 0, opts)

		if fi.DocValuesType() != index.DocValuesTypeNumeric {
			t.Errorf("DocValuesType() = %v, want DocValuesTypeNumeric", fi.DocValuesType())
		}
	})

	t.Run("field info without doc values", func(t *testing.T) {
		// Create field info without DocValues
		opts := index.FieldInfoOptions{
			DocValuesType: index.DocValuesTypeNone,
		}
		fi := index.NewFieldInfo("test", 0, opts)

		if fi.DocValuesType() != index.DocValuesTypeNone {
			t.Errorf("DocValuesType() = %v, want DocValuesTypeNone", fi.DocValuesType())
		}
	})
}

// Helper types
type testDocumentWithFields struct {
	fields []interface{}
}

func (d *testDocumentWithFields) GetFields() []interface{} {
	return d.fields
}
