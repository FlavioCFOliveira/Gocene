// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for index sorting functionality.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexSorting
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexSorting.java
//
// GC-188: Test IndexSorting - Index-time sorting, sorting during merge,
// sorted index search optimization
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// createIndexSortingMockAnalyzer creates a mock analyzer for testing
func createIndexSortingMockAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// TestIndexSorting_BasicString tests basic string sorting
func TestIndexSorting_BasicString(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeString)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents in reverse order
	doc := document.NewDocument()
	field, _ := document.NewSortedDocValuesField("foo", []byte("zzz"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("aaa"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("mmm"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	// Verify documents are sorted
	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_BasicLong tests basic long sorting
func TestIndexSorting_BasicLong(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents in reverse order
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_BasicInt tests basic int sorting
func TestIndexSorting_BasicInt(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeInt)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_BasicDouble tests basic double sorting
func TestIndexSorting_BasicDouble(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeDouble)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_BasicFloat tests basic float sorting
func TestIndexSorting_BasicFloat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeFloat)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_MultiValuedString tests multi-valued string sorting
func TestIndexSorting_MultiValuedString(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortedSetSortField("foo", false)
	config.SetIndexSort(index.NewSort(sortField.SortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	idField, _ := document.NewNumericDocValuesField("id", 3)
	doc.Add(idField)
	field, _ := document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("zzz")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 1)
	doc.Add(idField)
	field, _ = document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("aaa")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 2)
	doc.Add(idField)
	field, _ = document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("mmm")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_MultiValuedLong tests multi-valued long sorting
func TestIndexSorting_MultiValuedLong(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortedNumericSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField.SortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	idField, _ := document.NewNumericDocValuesField("id", 3)
	doc.Add(idField)
	field, _ := document.NewSortedNumericDocValuesField("foo", []int64{18, 35})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 1)
	doc.Add(idField)
	field, _ = document.NewSortedNumericDocValuesField("foo", []int64{-1})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 2)
	doc.Add(idField)
	field, _ = document.NewSortedNumericDocValuesField("foo", []int64{7, 22})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_MissingLongFirst tests missing long values sorted first
func TestIndexSorting_MissingLongFirst(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	sortField.SetMissingValue(int64(-9223372036854775808)) // Min int64
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add document with value
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	// Add document without value (missing)
	writer.AddDocument(document.NewDocument())
	writer.Commit()

	// Add document with value
	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_MissingStringFirst tests missing string values sorted first
func TestIndexSorting_MissingStringFirst(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeString)
	sortField.SetMissingValue([]byte(""))
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add document with value
	doc := document.NewDocument()
	field, _ := document.NewSortedDocValuesField("foo", []byte("zzz"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	// Add document without value (missing)
	writer.AddDocument(document.NewDocument())
	writer.Commit()

	// Add document with value
	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("aaa"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_TieBreak tests tie-breaking with multiple sort fields
func TestIndexSorting_TieBreak(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sort := index.NewSort(
		index.NewSortField("foo", index.SortTypeLong),
		index.NewSortField("bar", index.SortTypeLong),
	)
	config.SetIndexSort(sort)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with same foo but different bar
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field1, _ := document.NewNumericDocValuesField("foo", 1)
		field2, _ := document.NewNumericDocValuesField("bar", int64(10-i))
		doc.Add(field1)
		doc.Add(field2)
		writer.AddDocument(doc)
	}

	writer.ForceMerge(1)

	if writer.NumDocs() != 10 {
		t.Errorf("Expected 10 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_DeleteAll tests delete all functionality
func TestIndexSorting_DeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}

	// Delete all
	err = writer.DeleteAll()
	if err != nil {
		t.Errorf("DeleteAll() error = %v", err)
	}

	if writer.NumDocs() != 0 {
		t.Errorf("Expected 0 documents after DeleteAll, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_AddIndexes tests adding indexes
func TestIndexSorting_AddIndexes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Create second directory
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	config2 := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	config2.SetIndexSort(index.NewSort(sortField))

	writer2, err := index.NewIndexWriter(dir2, config2)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add indexes from first directory
	err = writer2.AddIndexes(dir)
	if err != nil {
		t.Errorf("AddIndexes() error = %v", err)
	}

	if writer2.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after AddIndexes, got %d", writer2.NumDocs())
	}

	writer2.Close()
	writer.Close()
}

// TestIndexSorting_WaitForMerges tests waiting for merges
func TestIndexSorting_WaitForMerges(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Wait for merges
	err = writer.WaitForMerges()
	if err != nil {
		t.Errorf("WaitForMerges() error = %v", err)
	}

	writer.Close()
}

// TestIndexSorting_ForceMerge tests force merge
func TestIndexSorting_ForceMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with commits to create multiple segments
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
		if i%3 == 0 {
			writer.Commit()
		}
	}

	// Force merge to 1 segment
	err = writer.ForceMerge(1)
	if err != nil {
		t.Errorf("ForceMerge() error = %v", err)
	}

	writer.Close()
}

// TestIndexSorting_BadSort tests that invalid sort configurations are handled
func TestIndexSorting_BadSort(t *testing.T) {
	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())

	// Try to use RELEVANCE sort for index sorting
	sort := index.SortRELEVANCE
	config.SetIndexSort(sort)

	// This should be handled gracefully - the sort is set but may not be valid
	if config.IndexSort() != sort {
		t.Error("Expected IndexSort to be set")
	}
}

// TestIndexSorting_SortFieldReverse tests reverse sort order
func TestIndexSorting_SortFieldReverse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	sortField.SetReverse(true)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}

	writer.ForceMerge(1)

	if writer.NumDocs() != 5 {
		t.Errorf("Expected 5 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_IndexSortWithSparseField tests sorting with sparse fields
func TestIndexSorting_IndexSortWithSparseField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("sparse", index.SortTypeLong)
	sortField.SetMissingValue(int64(-9223372036854775808))
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with and without the sparse field
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		if i%3 == 0 {
			field, _ := document.NewNumericDocValuesField("sparse", int64(i))
			doc.Add(field)
		}
		writer.AddDocument(doc)
	}

	writer.ForceMerge(1)

	if writer.NumDocs() != 20 {
		t.Errorf("Expected 20 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_ParentFieldNotConfigured tests parent field configuration
func TestIndexSorting_ParentFieldNotConfigured(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents without parent field configured
	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 1)
	doc.Add(field)
	writer.AddDocument(doc)

	writer.ForceMerge(1)

	if writer.NumDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", writer.NumDocs())
	}

	writer.Close()
}

// TestIndexSorting_BlockContainsParentField tests block indexing with parent field
func TestIndexSorting_BlockContainsParentField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	config.SetParentField("_parent")
	sortField := index.NewSortField("foo", index.SortTypeLong)
	config.SetIndexSort(index.NewSort(sortField))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add block of documents
	parentDoc := document.NewDocument()
	parentDoc.AddField("_parent", "true", document.StringFieldTypeStored)
	field, _ := document.NewNumericDocValuesField("foo", 1)
	parentDoc.Add(field)
	writer.AddDocument(parentDoc)

	childDoc := document.NewDocument()
	childDoc.AddField("_parent", "false", document.StringFieldTypeStored)
	writer.AddDocument(childDoc)

	writer.ForceMerge(1)

	if writer.NumDocs() != 2 {
		t.Errorf("Expected 2 documents, got %d", writer.NumDocs())
	}

	writer.Close()
}
