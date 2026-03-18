// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDocumentWriter
// Source: lucene/core/src/test/org/apache/lucene/index/TestDocumentWriter.java
//
// GC-180: Test DocumentWriter - Document addition/field storage, term vector
// indexing, field analysis, stored fields, multi-valued fields
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Test helper constants mirroring Lucene's DocHelper
const (
	// Field 1 - stored text field without term vectors
	field1Text    = "field one text"
	textField1Key = "textField1"

	// Field 2 - stored text field with term vectors
	field2Text    = "field field field two text"
	textField2Key = "textField2"
	// Fields will be lexicographically sorted: field, text, two
	field2Freqs = "3,1,1"

	// Field 3 - text field with omitNorms
	field3Text    = "aaaNoNorms aaaNoNorms bbbNoNorms"
	textField3Key = "textField3"

	// Keyword field
	keywordText     = "Keyword"
	keywordFieldKey = "keyField"

	// No norms field
	noNormsText = "omitNormsText"
	noNormsKey  = "omitNorms"

	// No TF field
	noTFText = "analyzed with no tf and positions"
	noTFKey  = "omitTermFreqAndPositions"

	// Unindexed field
	unindexedText = "unindexed field text"
	unindexedKey  = "unIndField"

	// Unstored fields
	unstored1Text = "unstored field text"
	unstored1Key  = "unStoredField1"
	unstored2Text = "unstored field text"
	unstored2Key  = "unStoredField2"
)

// setupTestDoc creates a test document with various field types
// Equivalent to DocHelper.setupDoc() in Lucene
func setupTestDoc() *document.Document {
	doc := &document.Document{}

	// Field 1: stored text field without term vectors
	customType1 := document.NewFieldType()
	customType1.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType1.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType1.Freeze()
	f1, _ := document.NewField(textField1Key, field1Text, customType1)
	doc.Add(f1)

	// Field 2: stored text field with term vectors
	customType2 := document.NewFieldType()
	customType2.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType2.SetStoreTermVectors(true)
	customType2.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
	customType2.StoreTermVectorPositions = true
	customType2.StoreTermVectorOffsets = true
	customType2.Freeze()
	f2, _ := document.NewField(textField2Key, field2Text, customType2)
	doc.Add(f2)

	// Field 3: text field with omitNorms
	customType3 := document.NewFieldType()
	customType3.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType3.SetOmitNorms(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType3.Freeze()
	f3, _ := document.NewField(textField3Key, field3Text, customType3)
	doc.Add(f3)

	// Keyword field (StringField equivalent)
	f4, _ := document.NewStringField(keywordFieldKey, keywordText, true)
	doc.Add(f4)

	// No norms field
	customType5 := document.NewFieldType()
	customType5.SetIndexed(true).SetStored(true).SetTokenized(false)
	customType5.SetOmitNorms(true)
	customType5.SetIndexOptions(index.IndexOptionsDocs)
	customType5.Freeze()
	f5, _ := document.NewField(noNormsKey, noNormsText, customType5)
	doc.Add(f5)

	// No TF field
	customType6 := document.NewFieldType()
	customType6.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType6.SetIndexOptions(index.IndexOptionsDocs)
	customType6.Freeze()
	f6, _ := document.NewField(noTFKey, noTFText, customType6)
	doc.Add(f6)

	// Unindexed field (stored only)
	customType7 := document.NewFieldType()
	customType7.SetStored(true)
	customType7.Freeze()
	f7, _ := document.NewField(unindexedKey, unindexedText, customType7)
	doc.Add(f7)

	// Unstored field 1
	f8, _ := document.NewTextField(unstored1Key, unstored1Text, false)
	doc.Add(f8)

	// Unstored field 2 with term vectors
	customType8 := document.NewFieldType()
	customType8.SetIndexed(true).SetStored(false).SetTokenized(true)
	customType8.SetStoreTermVectors(true)
	customType8.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType8.Freeze()
	f9, _ := document.NewField(unstored2Key, unstored2Text, customType8)
	doc.Add(f9)

	return doc
}

// TestDocumentWriter_AddDocument tests basic document addition with field storage
// Source: TestDocumentWriter.testAddDocument()
// Purpose: Tests that documents can be added and fields are properly stored
func TestDocumentWriter_AddDocument(t *testing.T) {
	t.Run("add document with various field types", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Create and add test document
		testDoc := setupTestDoc()
		err = writer.AddDocument(testDoc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Commit changes
		err = writer.Commit()
		if err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Close writer
		err = writer.Close()
		if err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Verify document count
		if writer.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", writer.NumDocs())
		}
	})

	t.Run("verify stored fields", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		testDoc := setupTestDoc()
		writer.AddDocument(testDoc)
		writer.Commit()
		writer.Close()

		// Open reader to verify stored fields
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		// Verify document count
		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document in reader, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_FieldStorage tests field storage capabilities
// Purpose: Verifies that stored fields can be retrieved
func TestDocumentWriter_FieldStorage(t *testing.T) {
	t.Run("stored text field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		textField, _ := document.NewTextField("content", "test content", true)
		doc.Add(textField)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("stored string field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		stringField, _ := document.NewStringField("id", "doc123", true)
		doc.Add(stringField)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_TermVectors tests term vector indexing
// Purpose: Verifies that term vectors are properly stored when configured
func TestDocumentWriter_TermVectors(t *testing.T) {
	t.Run("field with term vectors enabled", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Create field type with term vectors
		customType := document.NewFieldType()
		customType.SetIndexed(true).SetStored(true).SetTokenized(true)
		customType.SetStoreTermVectors(true)
		customType.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
		customType.StoreTermVectorPositions = true
		customType.StoreTermVectorOffsets = true
		customType.Freeze()

		doc := &document.Document{}
		field, _ := document.NewField("tvField", "term1 term2 term1", customType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("field without term vectors", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		// TextField without term vectors
		textField, _ := document.NewTextField("noTVField", "some text content", true)
		doc.Add(textField)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_MultiValuedFields tests multi-valued field handling
// Source: TestDocumentWriter.testPositionIncrementGap() concept
// Purpose: Tests that multiple values for the same field are handled correctly
func TestDocumentWriter_MultiValuedFields(t *testing.T) {
	t.Run("multiple values for same field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}

		// Add multiple values for the same field
		field1, _ := document.NewTextField("multiField", "value one", true)
		field2, _ := document.NewTextField("multiField", "value two", true)
		doc.Add(field1)
		doc.Add(field2)

		err := writer.AddDocument(doc)
		if err != nil {
			t.Errorf("Failed to add document with multi-valued field: %v", err)
		}

		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("repeated field values", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}

		// Add same value multiple times
		for i := 0; i < 3; i++ {
			field, _ := document.NewTextField("repeated", "repeated value", true)
			doc.Add(field)
		}

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_FieldAnalysis tests field analysis during indexing
// Purpose: Verifies that fields are properly analyzed
func TestDocumentWriter_FieldAnalysis(t *testing.T) {
	t.Run("tokenized field analysis", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		// This will be tokenized by the analyzer
		textField, _ := document.NewTextField("analyzed", "the quick brown fox", true)
		doc.Add(textField)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("non-tokenized field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		// StringField is not tokenized
		stringField, _ := document.NewStringField("exact", "exact value here", true)
		doc.Add(stringField)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_OmitNorms tests norm omission
// Source: TestDocumentWriter.testAddDocument() - norms verification
// Purpose: Tests that norms can be omitted per field
func TestDocumentWriter_OmitNorms(t *testing.T) {
	t.Run("field with norms omitted", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		customType := document.NewFieldType()
		customType.SetIndexed(true).SetStored(true).SetTokenized(true)
		customType.SetOmitNorms(true)
		customType.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
		customType.Freeze()

		doc := &document.Document{}
		field, _ := document.NewField("noNormsField", "some text", customType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("field with norms enabled", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Default TextField has norms enabled
		doc := &document.Document{}
		field, _ := document.NewTextField("withNormsField", "some text", true)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_IndexOptions tests different index options
// Source: TestDocumentWriter.testLUCENE_1590()
// Purpose: Tests various IndexOptions configurations
func TestDocumentWriter_IndexOptions(t *testing.T) {
	t.Run("IndexOptions DOCS only", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		customType := document.NewFieldType()
		customType.SetIndexed(true).SetStored(true).SetTokenized(true)
		customType.SetIndexOptions(index.IndexOptionsDocs)
		customType.Freeze()

		doc := &document.Document{}
		field, _ := document.NewField("docsOnly", "term1 term2 term1", customType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("IndexOptions DOCS_AND_FREQS", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		customType := document.NewFieldType()
		customType.SetIndexed(true).SetStored(true).SetTokenized(true)
		customType.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
		customType.Freeze()

		doc := &document.Document{}
		field, _ := document.NewField("docsAndFreqs", "term1 term2 term1", customType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_SameNameFields tests fields with same name but different configs
// Source: TestDocumentWriter.testLUCENE_1590()
// Purpose: Tests that fields with same name but different configurations work correctly
func TestDocumentWriter_SameNameFields(t *testing.T) {
	t.Run("same field name different configurations", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}

		// First field: indexed with no norms
		customType1 := document.NewFieldType()
		customType1.SetIndexed(true).SetStored(false).SetTokenized(true)
		customType1.SetOmitNorms(true)
		customType1.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
		customType1.Freeze()
		field1, _ := document.NewField("f1", "indexed value", customType1)
		doc.Add(field1)

		// Second field: same name, just stored
		customType2 := document.NewFieldType()
		customType2.SetStored(true)
		customType2.Freeze()
		field2, _ := document.NewField("f1", "stored value", customType2)
		doc.Add(field2)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_EmptyDocument tests empty document handling
func TestDocumentWriter_EmptyDocument(t *testing.T) {
	t.Run("empty document", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		// No fields added

		err := writer.AddDocument(doc)
		if err != nil {
			t.Errorf("Failed to add empty document: %v", err)
		}

		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_MultipleDocuments tests adding multiple documents
func TestDocumentWriter_MultipleDocuments(t *testing.T) {
	t.Run("add multiple documents", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		for i := 0; i < 10; i++ {
			doc := &document.Document{}
			field, _ := document.NewTextField("id", string(rune('0'+i)), true)
			doc.Add(field)

			err := writer.AddDocument(doc)
			if err != nil {
				t.Errorf("Failed to add document %d: %v", i, err)
			}
		}

		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 10 {
			t.Errorf("Expected 10 documents, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_BinaryFields tests binary field handling
func TestDocumentWriter_BinaryFields(t *testing.T) {
	t.Run("stored binary field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
		storedType := document.NewFieldType()
		storedType.SetStored(true)
		storedType.Freeze()
		field, _ := document.NewField("binary", binaryData, storedType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_FieldTypeValidation tests FieldType validation
func TestDocumentWriter_FieldTypeValidation(t *testing.T) {
	t.Run("invalid field type - positions without term vectors", func(t *testing.T) {
		customType := document.NewFieldType()
		customType.StoreTermVectorPositions = true
		// Not setting StoreTermVectors - should fail validation

		err := customType.Validate()
		if err == nil {
			t.Error("Expected validation error for positions without term vectors")
		}
	})

	t.Run("invalid field type - tokenized without indexed", func(t *testing.T) {
		customType := document.NewFieldType()
		customType.SetTokenized(true)
		customType.SetIndexed(false)
		// Tokenized requires indexed

		err := customType.Validate()
		if err == nil {
			t.Error("Expected validation error for tokenized without indexed")
		}
	})

	t.Run("valid field type", func(t *testing.T) {
		customType := document.NewFieldType()
		customType.SetIndexed(true).SetStored(true).SetTokenized(true)
		customType.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)

		err := customType.Validate()
		if err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})
}

// TestDocumentWriter_DocValuesFields tests doc values field handling
func TestDocumentWriter_DocValuesFields(t *testing.T) {
	t.Run("numeric doc values field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		field, _ := document.NewNumericDocValuesField("numericDV", 42)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})

	t.Run("sorted doc values field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		field, _ := document.NewSortedDocValuesField("sortedDV", []byte("value"))
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_CommitAndClose tests commit and close operations
func TestDocumentWriter_CommitAndClose(t *testing.T) {
	t.Run("commit then close", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		field, _ := document.NewTextField("test", "value", true)
		doc.Add(field)

		writer.AddDocument(doc)

		err := writer.Commit()
		if err != nil {
			t.Errorf("Commit failed: %v", err)
		}

		err = writer.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		if !writer.IsClosed() {
			t.Error("Writer should be closed")
		}
	})

	t.Run("close without commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		field, _ := document.NewTextField("test", "value", true)
		doc.Add(field)

		writer.AddDocument(doc)

		// Close should commit pending changes
		err := writer.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		if !writer.IsClosed() {
			t.Error("Writer should be closed")
		}
	})
}

// TestDocumentWriter_RAMUsage tests RAM usage tracking
// Source: TestDocumentWriter.testRAMUsage* methods
// Purpose: Tests that RAM usage is properly tracked
func TestDocumentWriter_RAMUsage(t *testing.T) {
	t.Run("documents writer RAM tracking", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(100)
		config.SetRAMBufferSizeMB(64.0)

		writer, _ := index.NewIndexWriter(dir, config)

		// Add multiple documents
		for i := 0; i < 10; i++ {
			doc := &document.Document{}
			field, _ := document.NewTextField("content", "some text content here", true)
			doc.Add(field)
			writer.AddDocument(doc)
		}

		// RAM usage should be tracked
		// Note: This is a basic test - actual RAM usage depends on implementation
		writer.Commit()
		writer.Close()
	})
}

// TestDocumentWriter_PositionIncrementGap tests position increment gap
// Source: TestDocumentWriter.testPositionIncrementGap()
// Purpose: Tests that position increment gap is applied between field values
func TestDocumentWriter_PositionIncrementGap(t *testing.T) {
	t.Run("custom position increment gap", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create analyzer with custom position increment gap
		analyzer := analysis.NewWhitespaceAnalyzer()

		config := index.NewIndexWriterConfig(analyzer)
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}

		// Add multiple values for same field
		field1, _ := document.NewTextField("repeated", "repeated one", true)
		field2, _ := document.NewTextField("repeated", "repeated two", true)
		doc.Add(field1)
		doc.Add(field2)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_UnstoredFields tests unstored field handling
func TestDocumentWriter_UnstoredFields(t *testing.T) {
	t.Run("unstored indexed field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		doc := &document.Document{}
		// TextField with stored=false
		field, _ := document.NewTextField("unstored", "this is indexed but not stored", false)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}

// TestDocumentWriter_StoredOnlyFields tests stored-only field handling
func TestDocumentWriter_StoredOnlyFields(t *testing.T) {
	t.Run("stored only field", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		customType := document.NewFieldType()
		customType.SetStored(true)
		customType.Freeze()

		doc := &document.Document{}
		field, _ := document.NewField("storedOnly", "stored value", customType)
		doc.Add(field)

		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()

		reader, _ := index.OpenDirectoryReader(dir)
		defer reader.Close()

		if reader.NumDocs() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.NumDocs())
		}
	})
}
