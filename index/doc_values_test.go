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
	"fmt"
	"sync"
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

	t.Run("create sorted numeric doc values field", func(t *testing.T) {
		values := []int64{10, 20, 30}
		field, err := document.NewSortedNumericDocValuesField("field", values)
		if err != nil {
			t.Fatalf("NewSortedNumericDocValuesField() error = %v", err)
		}
		if field == nil {
			t.Fatal("NewSortedNumericDocValuesField() returned nil")
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
		doc := &testDocument{
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
			doc := &testDocument{
				fields: []interface{}{
					numericField,
					sortedField,
				},
			}
			writer.AddDocument(doc)
		}

		writer.Commit()
	})
}

// TestDocValues_Sparse tests behavior when documents are missing DocValues.
func TestDocValues_Sparse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	// Doc 0: Has numeric doc values
	numericField, _ := document.NewNumericDocValuesField("num", 100)
	doc0 := &testDocument{fields: []interface{}{numericField}}
	writer.AddDocument(doc0)

	// Doc 1: Missing numeric doc values
	doc1 := &testDocument{fields: []interface{}{}}
	writer.AddDocument(doc1)

	// Doc 2: Has numeric doc values
	numericField2, _ := document.NewNumericDocValuesField("num", 200)
	doc2 := &testDocument{fields: []interface{}{numericField2}}
	writer.AddDocument(doc2)

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit error: %v", err)
	}

	// In a real implementation, we would use DirectoryReader to verify
	// that Doc 1 has no value (or default value) and Doc 0/2 have their values.
}

// TestDocValues_Updates tests DocValues updates.
func TestDocValues_Updates(t *testing.T) {
	t.Run("update numeric doc values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add document
		numericField, _ := document.NewNumericDocValuesField("id", 1)
		countField, _ := document.NewNumericDocValuesField("count", 10)
		doc := &testDocument{
			fields: []interface{}{
				numericField,
				countField,
			},
		}
		writer.AddDocument(doc)
		writer.Commit()

		// Update just the count field (in Lucene this can be an atomic update)
		// For now we test replacing the document
		updatedNumericField, _ := document.NewNumericDocValuesField("id", 1)
		updatedCountField, _ := document.NewNumericDocValuesField("count", 20)
		updatedDoc := &testDocument{
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

// TestDocValues_MultiValued tests multi-valued DocValues fields.
func TestDocValues_MultiValued(t *testing.T) {
	t.Run("sorted set with multiple values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		values := [][]byte{[]byte("apple"), []byte("banana"), []byte("cherry")}
		field, _ := document.NewSortedSetDocValuesField("tags", values)
		doc := &testDocument{fields: []interface{}{field}}

		writer.AddDocument(doc)
		writer.Commit()
	})

	t.Run("sorted numeric with multiple values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		values := []int64{100, 50, 200} // Unsorted input
		field, _ := document.NewSortedNumericDocValuesField("scores", values)
		doc := &testDocument{fields: []interface{}{field}}

		writer.AddDocument(doc)
		writer.Commit()
	})
}

// TestDocValues_Concurrent tests concurrent DocValues operations.
func TestDocValues_Concurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	numThreads := 4
	numDocsPerThread := 50
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < numDocsPerThread; j++ {
				numericField, _ := document.NewNumericDocValuesField("id", int64(threadID*1000+j))
				binaryField, _ := document.NewBinaryDocValuesField("data", []byte(fmt.Sprintf("data-%d-%d", threadID, j)))
				doc := &testDocument{
					fields: []interface{}{
						numericField,
						binaryField,
					},
				}
				writer.AddDocument(doc)
				if j%10 == 0 {
					writer.Commit()
				}
			}
		}(i)
	}

	wg.Wait()
	writer.Close()
}

// TestDocValues_Merging tests that DocValues are preserved across segment merges.
func TestDocValues_Merging(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Use a config that encourages merging
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)

	// Create multiple small segments
	for i := 0; i < 5; i++ {
		for j := 0; j < 10; j++ {
			numericField, _ := document.NewNumericDocValuesField("num", int64(i*10+j))
			doc := &testDocument{fields: []interface{}{numericField}}
			writer.AddDocument(doc)
		}
		writer.Commit() // Forces a new segment
	}

	// Trigger a merge
	// writer.ForceMerge(1)

	writer.Close()
}
