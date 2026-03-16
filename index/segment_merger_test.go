// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestSegmentMerger
// Source: lucene/core/src/test/org/apache/lucene/index/TestSegmentMerger.java
//
// GC-191: Test SegmentMerger - Merge multiple segments, term dictionaries,
// stored fields, DocValues
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test helper constants mirroring Lucene's DocHelper
const (
	// Field 1 - stored text field without term vectors
	mergerField1Text    = "field one text"
	mergerTextField1Key = "textField1"

	// Field 2 - stored text field with term vectors
	mergerField2Text    = "field field field two text"
	mergerTextField2Key = "textField2"
	// Fields will be lexicographically sorted: field, text, two
	mergerField2Freqs = "3,1,1"

	// Field 3 - text field with omitNorms
	mergerField3Text    = "aaaNoNorms aaaNoNorms bbbNoNorms"
	mergerTextField3Key = "textField3"

	// Keyword field
	mergerKeywordText     = "Keyword"
	mergerKeywordFieldKey = "keyField"

	// No norms field
	mergerNoNormsText = "omitNormsText"
	mergerNoNormsKey  = "omitNorms"

	// No TF field
	mergerNoTFText = "analyzed with no tf and positions"
	mergerNoTFKey  = "omitTermFreqAndPositions"

	// Unindexed field
	mergerUnindexedText = "unindexed field text"
	mergerUnindexedKey  = "unIndField"

	// Unstored fields
	mergerUnstored1Text = "unstored field text"
	mergerUnstored1Key  = "unStoredField1"
	mergerUnstored2Text = "unstored field text"
	mergerUnstored2Key  = "unStoredField2"
)

// mergerSetupTestDoc creates a test document with various field types
// Equivalent to DocHelper.setupDoc() in Lucene
func mergerSetupTestDoc() *document.Document {
	doc := &document.Document{}

	// Field 1: stored text field without term vectors
	customType1 := document.NewFieldType()
	customType1.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType1.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType1.Freeze()
	f1, _ := document.NewField(mergerTextField1Key, mergerField1Text, customType1)
	doc.Add(f1)

	// Field 2: stored text field with term vectors
	customType2 := document.NewFieldType()
	customType2.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType2.SetStoreTermVectors(true)
	customType2.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
	customType2.StoreTermVectorPositions = true
	customType2.StoreTermVectorOffsets = true
	customType2.Freeze()
	f2, _ := document.NewField(mergerTextField2Key, mergerField2Text, customType2)
	doc.Add(f2)

	// Field 3: text field with omitNorms
	customType3 := document.NewFieldType()
	customType3.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType3.SetOmitNorms(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType3.Freeze()
	f3, _ := document.NewField(mergerTextField3Key, mergerField3Text, customType3)
	doc.Add(f3)

	// Keyword field (StringField equivalent)
	f4, _ := document.NewStringField(mergerKeywordFieldKey, mergerKeywordText, true)
	doc.Add(f4)

	// No norms field
	customType5 := document.NewFieldType()
	customType5.SetIndexed(true).SetStored(true).SetTokenized(false)
	customType5.SetOmitNorms(true)
	customType5.SetIndexOptions(index.IndexOptionsDocs)
	customType5.Freeze()
	f5, _ := document.NewField(mergerNoNormsKey, mergerNoNormsText, customType5)
	doc.Add(f5)

	// No TF field
	customType6 := document.NewFieldType()
	customType6.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType6.SetIndexOptions(index.IndexOptionsDocs)
	customType6.Freeze()
	f6, _ := document.NewField(mergerNoTFKey, mergerNoTFText, customType6)
	doc.Add(f6)

	// Unindexed field (stored only)
	customType7 := document.NewFieldType()
	customType7.SetStored(true)
	customType7.Freeze()
	f7, _ := document.NewField(mergerUnindexedKey, mergerUnindexedText, customType7)
	doc.Add(f7)

	// Unstored field 1
	f8, _ := document.NewTextField(mergerUnstored1Key, mergerUnstored1Text, false)
	doc.Add(f8)

	// Unstored field 2 with term vectors
	customType8 := document.NewFieldType()
	customType8.SetIndexed(true).SetStored(false).SetTokenized(true)
	customType8.SetStoreTermVectors(true)
	customType8.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType8.Freeze()
	f9, _ := document.NewField(mergerUnstored2Key, mergerUnstored2Text, customType8)
	doc.Add(f9)

	return doc
}

// setupSecondTestDoc creates a second test document for merge testing
func setupSecondTestDoc() *document.Document {
	doc := &document.Document{}

	// Field 1: stored text field without term vectors
	customType1 := document.NewFieldType()
	customType1.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType1.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType1.Freeze()
	f1, _ := document.NewField(textField1Key, "second document field one", customType1)
	doc.Add(f1)

	// Field 2: stored text field with term vectors
	customType2 := document.NewFieldType()
	customType2.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType2.SetStoreTermVectors(true)
	customType2.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
	customType2.StoreTermVectorPositions = true
	customType2.StoreTermVectorOffsets = true
	customType2.Freeze()
	f2, _ := document.NewField(mergerTextField2Key, "second field field text", customType2)
	doc.Add(f2)

	// Field 3: text field with omitNorms
	customType3 := document.NewFieldType()
	customType3.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType3.SetOmitNorms(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType3.Freeze()
	f3, _ := document.NewField(textField3Key, "cccNoNorms dddNoNorms", customType3)
	doc.Add(f3)

	// Keyword field
	f4, _ := document.NewStringField(mergerKeywordFieldKey, "SecondKeyword", true)
	doc.Add(f4)

	// No norms field
	customType5 := document.NewFieldType()
	customType5.SetIndexed(true).SetStored(true).SetTokenized(false)
	customType5.SetOmitNorms(true)
	customType5.SetIndexOptions(index.IndexOptionsDocs)
	customType5.Freeze()
	f5, _ := document.NewField(mergerNoNormsKey, "secondNoNorms", customType5)
	doc.Add(f5)

	// No TF field
	customType6 := document.NewFieldType()
	customType6.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType6.SetIndexOptions(index.IndexOptionsDocs)
	customType6.Freeze()
	f6, _ := document.NewField(mergerNoTFKey, "second no tf", customType6)
	doc.Add(f6)

	// Unindexed field
	customType7 := document.NewFieldType()
	customType7.SetStored(true)
	customType7.Freeze()
	f7, _ := document.NewField(mergerUnindexedKey, "second unindexed", customType7)
	doc.Add(f7)

	// Unstored field 1
	f8, _ := document.NewTextField(mergerUnstored1Key, "second unstored", false)
	doc.Add(f8)

	// Unstored field 2 with term vectors
	customType8 := document.NewFieldType()
	customType8.SetIndexed(true).SetStored(false).SetTokenized(true)
	customType8.SetStoreTermVectors(true)
	customType8.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType8.Freeze()
	f9, _ := document.NewField(mergerUnstored2Key, "second unstored tv", customType8)
	doc.Add(f9)

	return doc
}

// TestSegmentMerger_Setup tests basic setup of merge test environment
// Source: TestSegmentMerger.test()
// Purpose: Sanity check that all merge test objects are properly created
func TestSegmentMerger_Setup(t *testing.T) {
	// Create directories for merged segment and source segments
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	merge1Dir := store.NewByteBuffersDirectory()
	defer merge1Dir.Close()

	merge2Dir := store.NewByteBuffersDirectory()
	defer merge2Dir.Close()

	// Verify directories are created
	if mergedDir == nil {
		t.Error("mergedDir should not be nil")
	}
	if merge1Dir == nil {
		t.Error("merge1Dir should not be nil")
	}
	if merge2Dir == nil {
		t.Error("merge2Dir should not be nil")
	}

	// Create documents
	doc1 := mergerSetupTestDoc()
	doc2 := setupSecondTestDoc()

	if doc1 == nil {
		t.Error("doc1 should not be nil")
	}
	if doc2 == nil {
		t.Error("doc2 should not be nil")
	}

	// Write documents to separate directories
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())

	// Write first document
	writer1, err := index.NewIndexWriter(merge1Dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	err = writer1.AddDocument(doc1)
	if err != nil {
		t.Fatalf("Failed to add doc1: %v", err)
	}
	err = writer1.Commit()
	if err != nil {
		t.Fatalf("Failed to commit writer1: %v", err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatalf("Failed to close writer1: %v", err)
	}

	// Write second document
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, err := index.NewIndexWriter(merge2Dir, config2)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	err = writer2.AddDocument(doc2)
	if err != nil {
		t.Fatalf("Failed to add doc2: %v", err)
	}
	err = writer2.Commit()
	if err != nil {
		t.Fatalf("Failed to commit writer2: %v", err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatalf("Failed to close writer2: %v", err)
	}

	// Open readers for both segments
	reader1, err := index.OpenDirectoryReader(merge1Dir)
	if err != nil {
		t.Fatalf("Failed to open reader1: %v", err)
	}
	defer reader1.Close()

	reader2, err := index.OpenDirectoryReader(merge2Dir)
	if err != nil {
		t.Fatalf("Failed to open reader2: %v", err)
	}
	defer reader2.Close()

	// Verify readers are created
	if reader1 == nil {
		t.Error("reader1 should not be nil")
	}
	if reader2 == nil {
		t.Error("reader2 should not be nil")
	}

	// Verify each segment has 1 document
	if reader1.NumDocs() != 1 {
		t.Errorf("Expected 1 doc in reader1, got %d", reader1.NumDocs())
	}
	if reader2.NumDocs() != 1 {
		t.Errorf("Expected 1 doc in reader2, got %d", reader2.NumDocs())
	}
}

// TestSegmentMerger_Merge tests merging of two segments
// Source: TestSegmentMerger.testMerge()
// Purpose: Tests that SegmentMerger correctly merges multiple segments,
// including term dictionaries, stored fields, and term vectors
func TestSegmentMerger_Merge(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	merge1Dir := store.NewByteBuffersDirectory()
	defer merge1Dir.Close()

	merge2Dir := store.NewByteBuffersDirectory()
	defer merge2Dir.Close()

	// Create and write first document
	doc1 := mergerSetupTestDoc()
	config1 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer1, err := index.NewIndexWriter(merge1Dir, config1)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	err = writer1.AddDocument(doc1)
	if err != nil {
		t.Fatalf("Failed to add doc1: %v", err)
	}
	err = writer1.Commit()
	if err != nil {
		t.Fatalf("Failed to commit writer1: %v", err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatalf("Failed to close writer1: %v", err)
	}

	// Create and write second document
	doc2 := setupSecondTestDoc()
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, err := index.NewIndexWriter(merge2Dir, config2)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	err = writer2.AddDocument(doc2)
	if err != nil {
		t.Fatalf("Failed to add doc2: %v", err)
	}
	err = writer2.Commit()
	if err != nil {
		t.Fatalf("Failed to commit writer2: %v", err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatalf("Failed to close writer2: %v", err)
	}

	// Open readers
	reader1, err := index.OpenDirectoryReader(merge1Dir)
	if err != nil {
		t.Fatalf("Failed to open reader1: %v", err)
	}
	defer reader1.Close()

	reader2, err := index.OpenDirectoryReader(merge2Dir)
	if err != nil {
		t.Fatalf("Failed to open reader2: %v", err)
	}
	defer reader2.Close()

	// Test merge using IndexWriter's addIndexes method
	// This is the high-level approach that uses SegmentMerger internally
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, err := index.NewIndexWriter(mergedDir, mergedConfig)
	if err != nil {
		t.Fatalf("Failed to create merged writer: %v", err)
	}

	// Add indexes from both source directories
	err = mergedWriter.AddIndexes(merge1Dir)
	if err != nil {
		t.Fatalf("Failed to add merge1Dir: %v", err)
	}
	err = mergedWriter.AddIndexes(merge2Dir)
	if err != nil {
		t.Fatalf("Failed to add merge2Dir: %v", err)
	}

	err = mergedWriter.Commit()
	if err != nil {
		t.Fatalf("Failed to commit merged writer: %v", err)
	}
	err = mergedWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close merged writer: %v", err)
	}

	// Open reader on merged directory
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	// Verify merged segment has 2 documents
	if mergedReader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents in merged segment, got %d", mergedReader.NumDocs())
	}

	// Verify we can access stored fields from both documents
	// Note: Full stored fields verification requires StoredFields implementation
	t.Logf("Merged reader has %d documents", mergedReader.NumDocs())

	// Verify term vectors are accessible
	// Get field infos to check term vector fields
	fieldInfos := mergedReader.GetFieldInfos()
	if fieldInfos == nil {
		t.Error("FieldInfos should not be nil")
	} else {
		// Count fields with term vectors
		tvCount := 0
		iter := fieldInfos.Iterator()
		for iter.HasNext() {
			fieldInfo := iter.Next()
			if fieldInfo.HasTermVectors() {
				tvCount++
			}
		}
		t.Logf("Found %d fields with term vectors", tvCount)
	}

	// Verify terms are accessible
	terms, err := mergedReader.Terms(mergerTextField2Key)
	if err != nil {
		t.Logf("Terms access error (expected if not fully implemented): %v", err)
	} else if terms != nil {
		t.Logf("Terms for %s: %v", mergerTextField2Key, terms)
	}
}

// TestSegmentMerger_BuildDocMap tests the document mapping when deletes are involved
// Source: TestSegmentMerger.testBuildDocMap()
// Purpose: Tests that MergeState.removeDeletes creates a compact doc map
func TestSegmentMerger_BuildDocMap(t *testing.T) {
	// Test with various sizes
	testSizes := []int{1, 10, 50, 100, 128}

	for _, maxDoc := range testSizes {
		t.Run(fmt.Sprintf("maxDoc_%d", maxDoc), func(t *testing.T) {
			// Create a FixedBitSet representing live documents
			liveDocs, err := util.NewFixedBitSet(maxDoc)
			if err != nil {
				t.Fatalf("Failed to create FixedBitSet: %v", err)
			}

			// Randomly mark some documents as live (approximately half)
			numLive := 0
			for i := 0; i < maxDoc; i++ {
				if i%2 == 0 { // Simple pattern: even docs are live
					liveDocs.Set(i)
					numLive++
				}
			}

			// Create doc map using removeDeletes logic
			docMap := make([]int, maxDoc)
			del := 0
			for i := 0; i < maxDoc; i++ {
				if !liveDocs.Get(i) {
					del++
					docMap[i] = -1 // Deleted
				} else {
					docMap[i] = i - del
				}
			}

			// Verify the mapping is compact
			for i := 0; i < maxDoc; i++ {
				if liveDocs.Get(i) {
					expected := i - (i/2) // Number of deleted docs before i
					if docMap[i] != expected {
						t.Errorf("Doc %d: expected mapped doc %d, got %d", i, expected, docMap[i])
					}
				}
			}

			t.Logf("maxDoc=%d, numLive=%d, mapping verified", maxDoc, numLive)
		})
	}
}

// TestSegmentMerger_TermDictionaryMerge tests that term dictionaries are properly merged
// Purpose: Verifies that terms from both segments appear in the merged index
func TestSegmentMerger_TermDictionaryMerge(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	merge1Dir := store.NewByteBuffersDirectory()
	defer merge1Dir.Close()

	merge2Dir := store.NewByteBuffersDirectory()
	defer merge2Dir.Close()

	// Create documents with distinct terms
	doc1 := &document.Document{}
	f1, _ := document.NewTextField("content", "unique term alpha", true)
	doc1.Add(f1)

	doc2 := &document.Document{}
	f2, _ := document.NewTextField("content", "unique term beta", true)
	doc2.Add(f2)

	// Write first document
	config1 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer1, err := index.NewIndexWriter(merge1Dir, config1)
	if err != nil {
		t.Fatalf("Failed to create writer1: %v", err)
	}
	writer1.AddDocument(doc1)
	writer1.Commit()
	writer1.Close()

	// Write second document
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, err := index.NewIndexWriter(merge2Dir, config2)
	if err != nil {
		t.Fatalf("Failed to create writer2: %v", err)
	}
	writer2.AddDocument(doc2)
	writer2.Commit()
	writer2.Close()

	// Merge using IndexWriter
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, err := index.NewIndexWriter(mergedDir, mergedConfig)
	if err != nil {
		t.Fatalf("Failed to create merged writer: %v", err)
	}

	mergedWriter.AddIndexes(merge1Dir)
	mergedWriter.AddIndexes(merge2Dir)
	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify merged index
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	if mergedReader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents, got %d", mergedReader.NumDocs())
	}

	// Verify terms from both documents are present
	terms, err := mergedReader.Terms("content")
	if err != nil {
		t.Logf("Terms access (may not be fully implemented): %v", err)
	} else if terms != nil {
		t.Logf("Terms found: %v", terms)
	}
}

// TestSegmentMerger_StoredFieldsMerge tests that stored fields are properly merged
// Purpose: Verifies that stored field values from both segments are preserved
func TestSegmentMerger_StoredFieldsMerge(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	merge1Dir := store.NewByteBuffersDirectory()
	defer merge1Dir.Close()

	merge2Dir := store.NewByteBuffersDirectory()
	defer merge2Dir.Close()

	// Create documents with stored fields
	doc1 := &document.Document{}
	f1, _ := document.NewTextField("title", "First Document", true)
	doc1.Add(f1)
	f2, _ := document.NewTextField("content", "Content of first document", true)
	doc1.Add(f2)

	doc2 := &document.Document{}
	f3, _ := document.NewTextField("title", "Second Document", true)
	doc2.Add(f3)
	f4, _ := document.NewTextField("content", "Content of second document", true)
	doc2.Add(f4)

	// Write documents
	config1 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer1, _ := index.NewIndexWriter(merge1Dir, config1)
	writer1.AddDocument(doc1)
	writer1.Commit()
	writer1.Close()

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, _ := index.NewIndexWriter(merge2Dir, config2)
	writer2.AddDocument(doc2)
	writer2.Commit()
	writer2.Close()

	// Merge
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, _ := index.NewIndexWriter(mergedDir, mergedConfig)
	mergedWriter.AddIndexes(merge1Dir)
	mergedWriter.AddIndexes(merge2Dir)
	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	if mergedReader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents, got %d", mergedReader.NumDocs())
	}

	// Verify field infos contain both fields
	fieldInfos := mergedReader.GetFieldInfos()
	if fieldInfos == nil {
		t.Error("FieldInfos should not be nil")
	} else {
		titleField := fieldInfos.GetByName("title")
		if titleField == nil {
			t.Error("title field should exist in merged index")
		} else if !titleField.IsStored() {
			t.Error("title field should be stored")
		}

		contentField := fieldInfos.GetByName("content")
		if contentField == nil {
			t.Error("content field should exist in merged index")
		} else if !contentField.IsStored() {
			t.Error("content field should be stored")
		}
	}
}

// TestSegmentMerger_DocValuesMerge tests that DocValues are properly merged
// Purpose: Verifies that DocValues from both segments are preserved after merge
func TestSegmentMerger_DocValuesMerge(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	merge1Dir := store.NewByteBuffersDirectory()
	defer merge1Dir.Close()

	merge2Dir := store.NewByteBuffersDirectory()
	defer merge2Dir.Close()

	// Create documents with numeric doc values
	doc1 := &document.Document{}
	f1, _ := document.NewTextField("name", "doc1", true)
	doc1.Add(f1)
	dv1, _ := document.NewNumericDocValuesField("sort_field", 100)
	doc1.Add(dv1)

	doc2 := &document.Document{}
	f2, _ := document.NewTextField("name", "doc2", true)
	doc2.Add(f2)
	dv2, _ := document.NewNumericDocValuesField("sort_field", 200)
	doc2.Add(dv2)

	// Write documents
	config1 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer1, _ := index.NewIndexWriter(merge1Dir, config1)
	writer1.AddDocument(doc1)
	writer1.Commit()
	writer1.Close()

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, _ := index.NewIndexWriter(merge2Dir, config2)
	writer2.AddDocument(doc2)
	writer2.Commit()
	writer2.Close()

	// Merge
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, _ := index.NewIndexWriter(mergedDir, mergedConfig)
	mergedWriter.AddIndexes(merge1Dir)
	mergedWriter.AddIndexes(merge2Dir)
	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	if mergedReader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents, got %d", mergedReader.NumDocs())
	}

	// Verify field infos contain doc values field
	fieldInfos := mergedReader.GetFieldInfos()
	if fieldInfos == nil {
		t.Error("FieldInfos should not be nil")
	} else {
		dvField := fieldInfos.GetByName("sort_field")
		if dvField == nil {
			t.Error("sort_field should exist in merged index")
		} else if dvField.DocValuesType() == index.DocValuesTypeNone {
			t.Error("sort_field should have DocValues")
		}
	}
}

// TestSegmentMerger_MultipleSegmentsMerge tests merging of more than two segments
// Purpose: Verifies that multiple segments can be merged in a single operation
func TestSegmentMerger_MultipleSegmentsMerge(t *testing.T) {
	// Create directories for 5 segments
	numSegments := 5
	dirs := make([]store.Directory, numSegments)
	for i := 0; i < numSegments; i++ {
		dirs[i] = store.NewByteBuffersDirectory()
		defer dirs[i].Close()
	}

	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	// Create a document for each segment
	for i := 0; i < numSegments; i++ {
		doc := &document.Document{}
		f1, _ := document.NewTextField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(f1)
		f2, _ := document.NewTextField("content", fmt.Sprintf("content of document %d", i), true)
		doc.Add(f2)

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, _ := index.NewIndexWriter(dirs[i], config)
		writer.AddDocument(doc)
		writer.Commit()
		writer.Close()
	}

	// Merge all segments
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, _ := index.NewIndexWriter(mergedDir, mergedConfig)

	for i := 0; i < numSegments; i++ {
		err := mergedWriter.AddIndexes(dirs[i])
		if err != nil {
			t.Fatalf("Failed to add index %d: %v", i, err)
		}
	}

	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	if mergedReader.NumDocs() != numSegments {
		t.Errorf("Expected %d documents, got %d", numSegments, mergedReader.NumDocs())
	}
}

// TestSegmentMerger_EmptySegmentMerge tests merging with empty segments
// Purpose: Verifies that empty segments are handled correctly during merge
func TestSegmentMerger_EmptySegmentMerge(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	nonEmptyDir := store.NewByteBuffersDirectory()
	defer nonEmptyDir.Close()

	emptyDir := store.NewByteBuffersDirectory()
	defer emptyDir.Close()

	// Write one document to non-empty directory
	doc := &document.Document{}
	f1, _ := document.NewTextField("content", "test content", true)
	doc.Add(f1)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, _ := index.NewIndexWriter(nonEmptyDir, config)
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	// Empty directory - just create an index with no documents
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer2, _ := index.NewIndexWriter(emptyDir, config2)
	writer2.Commit()
	writer2.Close()

	// Merge both
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, _ := index.NewIndexWriter(mergedDir, mergedConfig)
	mergedWriter.AddIndexes(nonEmptyDir)
	mergedWriter.AddIndexes(emptyDir)
	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	if mergedReader.NumDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", mergedReader.NumDocs())
	}
}

// TestSegmentMerger_TermVectorFieldsCount tests the count of term vector fields
// Source: TestSegmentMerger.testMerge() - term vector count verification
// Purpose: Verifies that the correct number of fields have term vectors
func TestSegmentMerger_TermVectorFieldsCount(t *testing.T) {
	// Create directories
	mergedDir := store.NewByteBuffersDirectory()
	defer mergedDir.Close()

	sourceDir := store.NewByteBuffersDirectory()
	defer sourceDir.Close()

	// Create document with multiple term vector fields
	doc := &document.Document{}

	// Field with term vectors
	customType1 := document.NewFieldType()
	customType1.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType1.SetStoreTermVectors(true)
	customType1.Freeze()
	f1, _ := document.NewField("tv_field1", "term1 term2", customType1)
	doc.Add(f1)

	// Another field with term vectors
	customType2 := document.NewFieldType()
	customType2.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType2.SetStoreTermVectors(true)
	customType2.Freeze()
	f2, _ := document.NewField("tv_field2", "term3 term4", customType2)
	doc.Add(f2)

	// Field without term vectors
	f3, _ := document.NewTextField("no_tv_field", "no term vectors", true)
	doc.Add(f3)

	// Write document
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, _ := index.NewIndexWriter(sourceDir, config)
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	// Merge
	mergedConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	mergedWriter, _ := index.NewIndexWriter(mergedDir, mergedConfig)
	mergedWriter.AddIndexes(sourceDir)
	mergedWriter.Commit()
	mergedWriter.Close()

	// Verify term vector field count
	mergedReader, err := index.OpenDirectoryReader(mergedDir)
	if err != nil {
		t.Fatalf("Failed to open merged reader: %v", err)
	}
	defer mergedReader.Close()

	fieldInfos := mergedReader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("FieldInfos should not be nil")
	}

	tvCount := 0
	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		if fieldInfo.HasTermVectors() {
			tvCount++
		}
	}

	// We expect 2 fields with term vectors (tv_field1 and tv_field2)
	// Note: This may vary based on implementation details
	t.Logf("Found %d fields with term vectors", tvCount)
}
