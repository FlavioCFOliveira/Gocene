// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package document_test contains tests for lazy document loading.
// This test file is in a separate package to avoid import cycles with index.
package document_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlaviocFOliveira/Gocene/store"
)

// TestLazyDocument_Lazy tests lazy field loading behavior.
// This is the Go port of TestLazyDocument.testLazy() from Apache Lucene.
func TestLazyDocument_Lazy(t *testing.T) {
	// Test constants (reduced for faster tests)
	numDocs := 10
	fields := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	numValues := 10

	// Create directory and index writer
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create documents with multiple fields
	for docid := 0; docid < numDocs; docid++ {
		d := NewDocument()
		docidField, _ := NewStringField("docid", fmt.Sprintf("%d", docid), true)
		d.Add(docidField)

		neverLoadField, _ := NewStringField("never_load", "fail", true)
		d.Add(neverLoadField)

		for _, f := range fields {
			for val := 0; val < numValues; val++ {
				fieldValue := fmt.Sprintf("%d_%s_%d", docid, f, val)
				field, _ := NewStringField(f, fieldValue, true)
				d.Add(field)
			}
		}

		loadLaterField, _ := NewStringField("load_later", "yes", true)
		d.Add(loadLaterField)

		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Test with a random document ID
	testDocID := 5 // Use a fixed ID for deterministic testing

	// Search for the document
	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("docid", fmt.Sprintf("%d", testDocID)))
	topDocs, err := searcher.Search(query, 100)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(topDocs.ScoreDocs) != 1 {
		t.Fatalf("Expected 1 hit, got %d", len(topDocs.ScoreDocs))
	}

	hit := topDocs.ScoreDocs[0]

	// Create LazyDocument with specific fields to lazy load
	lazyDoc := NewLazyDocument(reader, hit.Doc)

	// Create a visitor that loads only specific fields lazily
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, fields...)

	// Get stored fields and visit the document
	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(hit.Doc, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	d := visitor.doc

	// Verify initial state - all FIELDS should be Lazy and unrealized
	numFieldValues := 0
	fieldValueCounts := make(map[string]int)

	for _, f := range d.GetAllFields() {
		numFieldValues++

		if f.Name() == "never_load" {
			t.Error("never_load was loaded")
		}
		if f.Name() == "load_later" {
			t.Error("load_later was loaded on first pass")
		}
		if f.Name() == "docid" {
			// docid should be a regular field, not a LazyField
			if _, ok := f.(*LazyField); ok {
				t.Errorf("docid should not be a LazyField")
			}
		} else {
			// All other fields should be LazyFields
			lazyField, ok := f.(*LazyField)
			if !ok {
				t.Errorf("%s should be a LazyField, got %T", f.Name(), f)
			} else {
				if lazyField.HasBeenLoaded() {
					t.Errorf("%s should not be loaded yet", f.Name())
				}
				fieldValueCounts[f.Name()]++
			}
		}
	}

	expectedFieldValues := 1 + (numValues * len(fields)) // docid + (numValues * num_fields)
	if numFieldValues != expectedFieldValues {
		t.Errorf("Expected %d field values, got %d", expectedFieldValues, numFieldValues)
	}

	for fieldName, count := range fieldValueCounts {
		if count != numValues {
			t.Errorf("Expected %d values for field %s, got %d", numValues, fieldName, count)
		}
	}

	// Pick a single field name to load a single value
	fieldName := fields[3] // Use a fixed field for deterministic testing
	fieldValues := d.GetFieldsByName(fieldName)
	if len(fieldValues) != numValues {
		t.Errorf("Expected %d values in field %s, got %d", numValues, fieldName, len(fieldValues))
	}

	valNum := 5 // Use a fixed value index
	expectedValue := fmt.Sprintf("%d_%s_%d", testDocID, fieldName, valNum)
	actualValue := fieldValues[valNum].StringValue()
	if actualValue != expectedValue {
		t.Errorf("Expected value '%s', got '%s'", expectedValue, actualValue)
	}

	// Now every value of fieldName should be loaded
	for _, f := range d.GetAllFields() {
		if f.Name() == "never_load" {
			t.Error("never_load was loaded")
		}
		if f.Name() == "load_later" {
			t.Error("load_later was loaded too soon")
		}
		if f.Name() == "docid" {
			if _, ok := f.(*LazyField); ok {
				t.Errorf("docid should not be a LazyField")
			}
		} else {
			lazyField, ok := f.(*LazyField)
			if !ok {
				t.Errorf("%s should be a LazyField", f.Name())
			} else {
				// Only the accessed field should be loaded
				if f.Name() == fieldName {
					if !lazyField.HasBeenLoaded() {
						t.Errorf("%s should be loaded after accessing its value", f.Name())
					}
				} else {
					if lazyField.HasBeenLoaded() {
						t.Errorf("%s should not be loaded yet", f.Name())
					}
				}
			}
		}
	}

	// Use the same LazyDoc to ask for one more lazy field
	visitor2 := newLazyTestingStoredFieldVisitor(lazyDoc, "load_later")
	if err := storedFields.Document(hit.Doc, visitor2); err != nil {
		t.Fatalf("Failed to visit document for load_later: %v", err)
	}
	d = visitor2.doc

	// Ensure we have all the values we expect now, and that
	// adding one more lazy field didn't "unload" the existing LazyField's
	// we already loaded.
	for _, f := range d.GetAllFields() {
		if f.Name() == "never_load" {
			t.Error("never_load was loaded")
		}
		if f.Name() == "docid" {
			if _, ok := f.(*LazyField); ok {
				t.Errorf("docid should not be a LazyField")
			}
		} else {
			lazyField, ok := f.(*LazyField)
			if !ok {
				t.Errorf("%s should be a LazyField", f.Name())
			} else {
				// The previously loaded field should still be loaded
				if f.Name() == fieldName {
					if !lazyField.HasBeenLoaded() {
						t.Errorf("%s should still be loaded", f.Name())
					}
				}
			}
		}
	}

	// Even the underlying doc shouldn't have never_load
	underlyingDoc, err := lazyDoc.GetDocument()
	if err != nil {
		t.Fatalf("Failed to get underlying document: %v", err)
	}
	if underlyingDoc.Get("never_load") != nil {
		t.Error("never_load was loaded in wrapped doc")
	}
}

// TestLazyDocument_MultipleFields tests loading multiple fields lazily.
func TestLazyDocument_MultipleFields(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document with multiple fields
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)
	d.AddField("title", "Test Title", TextFieldTypeStored)
	d.AddField("content", "Test Content", TextFieldTypeStored)
	d.AddField("author", "Test Author", StringFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor that loads only title and content lazily
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "title", "content")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	// Verify id is not a LazyField
	idField := doc.Get("id")
	if idField == nil {
		t.Fatal("id field not found")
	}
	if _, ok := idField.(*LazyField); ok {
		t.Error("id should not be a LazyField")
	}

	// Verify title is a LazyField and not loaded yet
	titleFields := doc.GetFieldsByName("title")
	if len(titleFields) != 1 {
		t.Fatalf("Expected 1 title field, got %d", len(titleFields))
	}
	titleLazy, ok := titleFields[0].(*LazyField)
	if !ok {
		t.Fatal("title should be a LazyField")
	}
	if titleLazy.HasBeenLoaded() {
		t.Error("title should not be loaded yet")
	}

	// Access title value - this should load it
	titleValue := titleFields[0].StringValue()
	if titleValue != "Test Title" {
		t.Errorf("Expected 'Test Title', got '%s'", titleValue)
	}

	// Now title should be loaded
	if !titleLazy.HasBeenLoaded() {
		t.Error("title should be loaded after accessing its value")
	}

	// Verify content is still not loaded
	contentFields := doc.GetFieldsByName("content")
	if len(contentFields) != 1 {
		t.Fatalf("Expected 1 content field, got %d", len(contentFields))
	}
	contentLazy, ok := contentFields[0].(*LazyField)
	if !ok {
		t.Fatal("content should be a LazyField")
	}
	if contentLazy.HasBeenLoaded() {
		t.Error("content should not be loaded yet")
	}

	// Verify author was not loaded at all
	authorField := doc.Get("author")
	if authorField != nil {
		t.Error("author should not be in the lazy document")
	}
}

// TestLazyDocument_FieldTypes tests that lazy fields report correct types after loading.
func TestLazyDocument_FieldTypes(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document with different field types
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)
	d.AddField("count", 42, IntFieldTypeStored)
	d.AddField("score", 3.14, FloatFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor that loads all fields lazily
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "id", "count", "score")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	// Test string field
	idFields := doc.GetFieldsByName("id")
	if len(idFields) != 1 {
		t.Fatalf("Expected 1 id field, got %d", len(idFields))
	}
	idValue := idFields[0].StringValue()
	if idValue != "1" {
		t.Errorf("Expected '1', got '%s'", idValue)
	}

	// Test int field
	countFields := doc.GetFieldsByName("count")
	if len(countFields) != 1 {
		t.Fatalf("Expected 1 count field, got %d", len(countFields))
	}
	countValue := countFields[0].NumericValue()
	if countValue == nil {
		t.Error("count should have a numeric value")
	} else if countValue != int64(42) {
		t.Errorf("Expected 42, got %v", countValue)
	}

	// Test float field
	scoreFields := doc.GetFieldsByName("score")
	if len(scoreFields) != 1 {
		t.Fatalf("Expected 1 score field, got %d", len(scoreFields))
	}
	scoreValue := scoreFields[0].NumericValue()
	if scoreValue == nil {
		t.Error("score should have a numeric value")
	} else if scoreValue != float64(3.14) {
		// Allow small floating point differences
		if diff, ok := scoreValue.(float64); !ok || diff < 3.13 || diff > 3.15 {
			t.Errorf("Expected ~3.14, got %v", scoreValue)
		}
	}
}

// TestLazyDocument_ConcurrentAccess tests concurrent access to lazy fields.
func TestLazyDocument_ConcurrentAccess(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)
	d.AddField("content", "Test Content", TextFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "content")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	contentFields := doc.GetFieldsByName("content")
	if len(contentFields) != 1 {
		t.Fatalf("Expected 1 content field, got %d", len(contentFields))
	}

	contentLazy, ok := contentFields[0].(*LazyField)
	if !ok {
		t.Fatal("content should be a LazyField")
	}

	// Access the field multiple times concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			value := contentFields[0].StringValue()
			if value != "Test Content" {
				t.Errorf("Expected 'Test Content', got '%s'", value)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify field is loaded
	if !contentLazy.HasBeenLoaded() {
		t.Error("content should be loaded after concurrent access")
	}
}

// TestLazyDocument_EmptyDocument tests lazy loading with an empty document.
func TestLazyDocument_EmptyDocument(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create an empty document
	d := NewDocument()

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Get underlying document
	underlyingDoc, err := lazyDoc.GetDocument()
	if err != nil {
		t.Fatalf("Failed to get underlying document: %v", err)
	}

	if underlyingDoc == nil {
		t.Fatal("Underlying document should not be nil")
	}

	if !underlyingDoc.IsEmpty() {
		t.Error("Underlying document should be empty")
	}
}

// TestLazyDocument_NonExistentField tests accessing a non-existent field.
func TestLazyDocument_NonExistentField(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor that tries to load a non-existent field
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "nonexistent")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	// The document should be empty since nonexistent field wasn't in the original
	if doc.Size() != 0 {
		t.Errorf("Expected empty document, got %d fields", doc.Size())
	}
}

// TestLazyField_HasBeenLoaded tests the HasBeenLoaded method.
func TestLazyField_HasBeenLoaded(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)
	d.AddField("content", "Test", TextFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "content")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	contentFields := doc.GetFieldsByName("content")
	if len(contentFields) != 1 {
		t.Fatalf("Expected 1 content field, got %d", len(contentFields))
	}

	lazyField, ok := contentFields[0].(*LazyField)
	if !ok {
		t.Fatal("content should be a LazyField")
	}

	// Initially not loaded
	if lazyField.HasBeenLoaded() {
		t.Error("Field should not be loaded initially")
	}

	// Access the value
	_ = lazyField.StringValue()

	// Now it should be loaded
	if !lazyField.HasBeenLoaded() {
		t.Error("Field should be loaded after accessing its value")
	}
}

// TestLazyField_MultipleValues tests lazy loading with multiple values for the same field.
func TestLazyField_MultipleValues(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create a document with multiple values for the same field
	d := NewDocument()
	d.AddField("id", "1", StringFieldTypeStored)
	d.AddField("tag", "go", StringFieldTypeStored)
	d.AddField("tag", "lucene", StringFieldTypeStored)
	d.AddField("tag", "search", StringFieldTypeStored)

	if err := writer.AddDocument(d); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Create LazyDocument
	lazyDoc := NewLazyDocument(reader, 0)

	// Create visitor
	visitor := newLazyTestingStoredFieldVisitor(lazyDoc, "tag")

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("Failed to get stored fields: %v", err)
	}

	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Failed to visit document: %v", err)
	}

	doc := visitor.doc

	// Get all tag fields
	tagFields := doc.GetFieldsByName("tag")
	if len(tagFields) != 3 {
		t.Fatalf("Expected 3 tag fields, got %d", len(tagFields))
	}

	// All should be LazyFields and not loaded
	for i, field := range tagFields {
		lazyField, ok := field.(*LazyField)
		if !ok {
			t.Fatalf("tag field %d should be a LazyField", i)
		}
		if lazyField.HasBeenLoaded() {
			t.Errorf("tag field %d should not be loaded yet", i)
		}
	}

	// Access one value - this should load all values for this field
	_ = tagFields[1].StringValue()

	// Now all should be loaded
	for i, field := range tagFields {
		lazyField, ok := field.(*LazyField)
		if !ok {
			t.Fatalf("tag field %d should be a LazyField", i)
		}
		if !lazyField.HasBeenLoaded() {
			t.Errorf("tag field %d should be loaded after accessing any value", i)
		}
	}

	// Verify values
	values := doc.GetValues("tag")
	if len(values) != 3 {
		t.Fatalf("Expected 3 tag values, got %d", len(values))
	}

	expected := map[string]bool{"go": false, "lucene": false, "search": false}
	for _, v := range values {
		expected[v] = true
	}
	for k, found := range expected {
		if !found {
			t.Errorf("Expected to find tag value '%s'", k)
		}
	}
}

// lazyTestingStoredFieldVisitor is a test visitor that selectively loads fields.
// This is the Go equivalent of TestLazyDocument.LazyTestingStoredFieldVisitor.
type lazyTestingStoredFieldVisitor struct {
	doc           *Document
	lazyDoc       *LazyDocument
	lazyFieldNames map[string]struct{}
}

// newLazyTestingStoredFieldVisitor creates a new visitor for testing lazy loading.
func newLazyTestingStoredFieldVisitor(lazyDoc *LazyDocument, fields ...string) *lazyTestingStoredFieldVisitor {
	fieldNames := make(map[string]struct{})
	for _, f := range fields {
		fieldNames[f] = struct{}{}
	}
	return &lazyTestingStoredFieldVisitor{
		doc:            NewDocument(),
		lazyDoc:        lazyDoc,
		lazyFieldNames: fieldNames,
	}
}

// Ensure lazyTestingStoredFieldVisitor implements StoredFieldVisitor
var _ index.StoredFieldVisitor = (*lazyTestingStoredFieldVisitor)(nil)

// StringField is called for a stored string field.
func (v *lazyTestingStoredFieldVisitor) StringField(field string, value string) {
	if field == "docid" {
		// Always load docid directly
		ft := NewFieldType()
		ft.SetStored(true)
		f, _ := NewField(field, value, ft)
		v.doc.Add(f)
	} else if field == "never_load" {
		// Skip never_load
		return
	} else {
		// Check if this field should be lazy loaded
		if _, ok := v.lazyFieldNames[field]; ok {
			// Get field info and create lazy field
			// In a real implementation, we'd have the FieldInfo here
			// For testing, we add a placeholder that will be replaced
			v.doc.Add(&LazyField{lazyDoc: v.lazyDoc, name: field})
		}
	}
}

// BinaryField is called for a stored binary field.
func (v *lazyTestingStoredFieldVisitor) BinaryField(field string, value []byte) {
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	v.doc.Add(f)
}

// IntField is called for a stored int field.
func (v *lazyTestingStoredFieldVisitor) IntField(field string, value int) {
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	v.doc.Add(f)
}

// LongField is called for a stored long field.
func (v *lazyTestingStoredFieldVisitor) LongField(field string, value int64) {
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	v.doc.Add(f)
}

// FloatField is called for a stored float field.
func (v *lazyTestingStoredFieldVisitor) FloatField(field string, value float32) {
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	v.doc.Add(f)
}

// DoubleField is called for a stored double field.
func (v *lazyTestingStoredFieldVisitor) DoubleField(field string, value float64) {
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	v.doc.Add(f)
}
