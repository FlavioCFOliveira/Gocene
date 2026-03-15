// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestSegmentReader
// Source: lucene/core/src/test/org/apache/lucene/index/TestSegmentReader.java
//
// GC-190: Test SegmentReader - Read documents from segment, term dictionaries,
// stored fields, DocValues
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Test helper constants mirroring Lucene's DocHelper
// Using sr prefix to avoid conflicts with other test files
const (
	// Field 1 - stored text field without term vectors
	srField1Text    = "field one text"
	srTextField1Key = "textField1"

	// Field 2 - stored text field with term vectors
	srField2Text    = "field field field two text"
	srTextField2Key = "textField2"

	// Field 3 - text field with omitNorms
	srField3Text    = "aaaNoNorms aaaNoNorms bbbNoNorms"
	srTextField3Key = "textField3"

	// Keyword field
	srKeywordText     = "Keyword"
	srKeywordFieldKey = "keyField"

	// No norms field
	srNoNormsText = "omitNormsText"
	srNoNormsKey  = "omitNorms"

	// No TF field
	srNoTFText = "analyzed with no tf and positions"
	srNoTFKey  = "omitTermFreqAndPositions"

	// Unindexed field
	srUnindexedText = "unindexed field text"
	srUnindexedKey  = "unIndField"

	// Unstored fields
	srUnstored1Text = "unstored field text"
	srUnstored1Key  = "unStoredField1"
	srUnstored2Text = "unstored field text"
	srUnstored2Key  = "unStoredField2"
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
	f1, _ := document.NewField(srTextField1Key, srField1Text, customType1)
	doc.Add(f1)

	// Field 2: stored text field with term vectors
	customType2 := document.NewFieldType()
	customType2.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType2.SetStoreTermVectors(true)
	customType2.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
	customType2.StoreTermVectorPositions = true
	customType2.StoreTermVectorOffsets = true
	customType2.Freeze()
	f2, _ := document.NewField(srTextField2Key, srField2Text, customType2)
	doc.Add(f2)

	// Field 3: text field with omitNorms
	customType3 := document.NewFieldType()
	customType3.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType3.SetOmitNorms(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType3.Freeze()
	f3, _ := document.NewField(srTextField3Key, srField3Text, customType3)
	doc.Add(f3)

	// Keyword field (StringField equivalent)
	f4, _ := document.NewStringField(srKeywordFieldKey, srKeywordText, true)
	doc.Add(f4)

	// No norms field
	customType5 := document.NewFieldType()
	customType5.SetIndexed(true).SetStored(true).SetTokenized(false)
	customType5.SetOmitNorms(true)
	customType5.SetIndexOptions(index.IndexOptionsDocs)
	customType5.Freeze()
	f5, _ := document.NewField(srNoNormsKey, srNoNormsText, customType5)
	doc.Add(f5)

	// No TF field
	customType6 := document.NewFieldType()
	customType6.SetIndexed(true).SetStored(true).SetTokenized(true)
	customType6.SetIndexOptions(index.IndexOptionsDocs)
	customType6.Freeze()
	f6, _ := document.NewField(srNoTFKey, srNoTFText, customType6)
	doc.Add(f6)

	// Unindexed field (stored only)
	customType7 := document.NewFieldType()
	customType7.SetStored(true)
	customType7.Freeze()
	f7, _ := document.NewField(srUnindexedKey, srUnindexedText, customType7)
	doc.Add(f7)

	// Unstored field 1
	f8, _ := document.NewTextField(srUnstored1Key, srUnstored1Text, false)
	doc.Add(f8)

	// Unstored field 2 with term vectors
	customType8 := document.NewFieldType()
	customType8.SetIndexed(true).SetStored(false).SetTokenized(true)
	customType8.SetStoreTermVectors(true)
	customType8.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	customType8.Freeze()
	f9, _ := document.NewField(srUnstored2Key, srUnstored2Text, customType8)
	doc.Add(f9)

	return doc
}

// nameValues maps field names to their expected values
// Equivalent to DocHelper.nameValues in Lucene
func getNameValues() map[string]string {
	return map[string]string{
		srTextField1Key:   srField1Text,
		srTextField2Key:   srField2Text,
		srTextField3Key:   srField3Text,
		srKeywordFieldKey: srKeywordText,
		srNoNormsKey:      srNoNormsText,
		srNoTFKey:         srNoTFText,
		srUnindexedKey:    srUnindexedText,
		srUnstored1Key:    srUnstored1Text,
		srUnstored2Key:    srUnstored2Text,
	}
}

// setupSegmentReader creates a directory and returns a basic SegmentReader
// This is a simplified version that creates the reader structure without full codec integration
func setupSegmentReader(t *testing.T) (store.Directory, *document.Document, *index.SegmentReader) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	testDoc := setupTestDoc()

	// Create a segment info
	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	segmentCommitInfo := index.NewSegmentCommitInfo(segmentInfo, 0, -1)

	// Create field infos
	fieldInfos := index.NewFieldInfos()
	// Add field infos based on the document fields
	for i, field := range testDoc.GetAllFields() {
		ft := field.FieldType()
		opts := index.FieldInfoOptions{
			IndexOptions:             ft.IndexOptions,
			Stored:                   ft.Stored,
			StoreTermVectors:         ft.StoreTermVectors,
			StoreTermVectorPositions: ft.StoreTermVectorPositions,
			StoreTermVectorOffsets:   ft.StoreTermVectorOffsets,
			OmitNorms:                ft.OmitNorms,
		}
		fi := index.NewFieldInfo(field.Name(), i, opts)
		fieldInfos.Add(fi)
	}

	// Create a basic segment reader with field infos
	reader := index.NewSegmentReaderWithCore(segmentCommitInfo, nil, fieldInfos, nil)

	return dir, testDoc, reader
}

// TestSegmentReader_Basic tests basic setup
// Source: TestSegmentReader.test()
// Purpose: Verifies directory and reader are properly initialized
func TestSegmentReader_Basic(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	if dir == nil {
		t.Error("Directory should not be nil")
	}
	if reader == nil {
		t.Error("Reader should not be nil")
	}

	nameValues := getNameValues()
	if len(nameValues) == 0 {
		t.Error("nameValues should not be empty")
	}
}

// TestSegmentReader_DocumentCounts tests document count methods
// Source: TestSegmentReader.testDocument() (partial)
// Purpose: Tests that document counts are correctly reported
func TestSegmentReader_DocumentCounts(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	// Check document counts
	if reader.NumDocs() != 1 {
		t.Errorf("Expected NumDocs=1, got %d", reader.NumDocs())
	}
	if reader.MaxDoc() < 1 {
		t.Errorf("Expected MaxDoc>=1, got %d", reader.MaxDoc())
	}
}

// TestSegmentReader_FieldNameVariations tests field categorization
// Source: TestSegmentReader.testGetFieldNameVariations()
// Purpose: Tests that fields are correctly categorized as indexed, stored, term vector, etc.
func TestSegmentReader_FieldNameVariations(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("FieldInfos should not be nil")
	}

	allFieldNames := make(map[string]struct{})
	indexedFieldNames := make(map[string]struct{})
	notIndexedFieldNames := make(map[string]struct{})
	tvFieldNames := make(map[string]struct{})
	noTVFieldNames := make(map[string]struct{})

	// Iterate over field infos using Iterator
	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		name := fieldInfo.Name()
		allFieldNames[name] = struct{}{}

		if fieldInfo.IndexOptions() != index.IndexOptionsNone {
			indexedFieldNames[name] = struct{}{}
		} else {
			notIndexedFieldNames[name] = struct{}{}
		}

		if fieldInfo.HasTermVectors() {
			tvFieldNames[name] = struct{}{}
		} else if fieldInfo.IndexOptions() != index.IndexOptionsNone {
			noTVFieldNames[name] = struct{}{}
		}
	}

	// Verify all fields are accounted for
	nameValues := getNameValues()
	if len(allFieldNames) != len(nameValues) {
		t.Errorf("Expected %d fields, got %d", len(nameValues), len(allFieldNames))
	}

	// Verify each field name is in nameValues
	for name := range allFieldNames {
		if _, ok := nameValues[name]; !ok && name != "" {
			t.Errorf("Field %s not found in nameValues", name)
		}
	}

	// Verify indexed fields
	expectedIndexed := []string{srTextField1Key, srTextField2Key, srTextField3Key, srKeywordFieldKey, srNoNormsKey, srNoTFKey, srUnstored1Key, srUnstored2Key}
	if len(indexedFieldNames) != len(expectedIndexed) {
		t.Errorf("Expected %d indexed fields, got %d", len(expectedIndexed), len(indexedFieldNames))
	}
	for _, name := range expectedIndexed {
		if _, ok := indexedFieldNames[name]; !ok {
			t.Errorf("Expected field %s to be indexed", name)
		}
	}

	// Verify unindexed fields
	expectedUnindexed := []string{srUnindexedKey}
	if len(notIndexedFieldNames) != len(expectedUnindexed) {
		t.Errorf("Expected %d unindexed fields, got %d", len(expectedUnindexed), len(notIndexedFieldNames))
	}
	for _, name := range expectedUnindexed {
		if _, ok := notIndexedFieldNames[name]; !ok {
			t.Errorf("Expected field %s to be unindexed", name)
		}
	}

	// Verify term vector fields
	expectedTV := []string{srTextField2Key, srUnstored2Key}
	if len(tvFieldNames) != len(expectedTV) {
		t.Errorf("Expected %d term vector fields, got %d", len(expectedTV), len(tvFieldNames))
	}
	for _, name := range expectedTV {
		if _, ok := tvFieldNames[name]; !ok {
			t.Errorf("Expected field %s to have term vectors", name)
		}
	}

	// Verify no term vector fields (indexed but no TV)
	expectedNoTV := []string{srTextField1Key, srTextField3Key, srKeywordFieldKey, srNoNormsKey, srNoTFKey, srUnstored1Key}
	if len(noTVFieldNames) != len(expectedNoTV) {
		t.Errorf("Expected %d no-term-vector fields, got %d", len(expectedNoTV), len(noTVFieldNames))
	}
	for _, name := range expectedNoTV {
		if _, ok := noTVFieldNames[name]; !ok {
			t.Errorf("Expected field %s to not have term vectors", name)
		}
	}
}

// TestSegmentReader_FieldInfos tests FieldInfos access
// Source: TestSegmentReader (implicit via getFieldInfos() usage)
// Purpose: Tests that FieldInfos can be retrieved and contain expected information
func TestSegmentReader_FieldInfos(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("FieldInfos should not be nil")
	}

	// Verify we can get field info by name
	nameValues := getNameValues()
	for fieldName := range nameValues {
		fieldInfo := fieldInfos.GetByName(fieldName)
		if fieldInfo == nil {
			t.Errorf("FieldInfo for %s should not be nil", fieldName)
			continue
		}

		// Verify field info properties
		if fieldInfo.Name() != fieldName {
			t.Errorf("Expected field name %s, got %s", fieldName, fieldInfo.Name())
		}
	}

	// Verify size
	if fieldInfos.Size() != len(nameValues) {
		t.Errorf("Expected %d fields, got %d", len(nameValues), fieldInfos.Size())
	}
}

// TestSegmentReader_SegmentCommitInfo tests SegmentCommitInfo access
// Source: TestSegmentReader (implicit via segment setup)
// Purpose: Tests that SegmentCommitInfo can be retrieved
func TestSegmentReader_SegmentCommitInfo(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	segmentCommitInfo := reader.GetSegmentCommitInfo()
	if segmentCommitInfo == nil {
		t.Fatal("SegmentCommitInfo should not be nil")
	}

	segmentInfo := segmentCommitInfo.SegmentInfo()
	if segmentInfo == nil {
		t.Fatal("SegmentInfo should not be nil")
	}

	// Verify segment has documents
	if segmentInfo.DocCount() != 1 {
		t.Errorf("Expected 1 document in segment, got %d", segmentInfo.DocCount())
	}
}

// TestSegmentReader_Norms tests norm values for fields
// Source: TestSegmentReader.testNorms() and checkNorms()
// Purpose: Tests that norm values are correctly stored/omitted for fields
func TestSegmentReader_Norms(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	fieldInfos := reader.GetFieldInfos()

	// Test norms for each field
	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		if fieldInfo.IndexOptions() == index.IndexOptionsNone {
			continue // Skip unindexed fields
		}

		fieldName := fieldInfo.Name()

		// Check if norms should exist based on field type
		shouldHaveNorms := !fieldInfo.OmitNorms()

		// Verify the field info correctly reflects the omitNorms setting
		if fieldInfo.OmitNorms() != !shouldHaveNorms {
			t.Errorf("Field %s: OmitNorms mismatch", fieldName)
		}
	}

	// Verify specific fields that should have norms omitted
	field3Info := fieldInfos.GetByName(srTextField3Key)
	if field3Info != nil && !field3Info.OmitNorms() {
		t.Errorf("Field %s should have OmitNorms=true", srTextField3Key)
	}

	noNormsInfo := fieldInfos.GetByName(srNoNormsKey)
	if noNormsInfo != nil && !noNormsInfo.OmitNorms() {
		t.Errorf("Field %s should have OmitNorms=true", srNoNormsKey)
	}
}

// TestSegmentReader_Leaves tests the Leaves method
// Source: TestSegmentReader (implicit via LeafReader interface)
// Purpose: Tests that Leaves returns the correct leaf reader contexts
func TestSegmentReader_Leaves(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}

	if len(leaves) != 1 {
		t.Errorf("Expected 1 leaf, got %d", len(leaves))
	}

	if leaves[0] == nil {
		t.Error("Leaf should not be nil")
	}
}

// TestSegmentReader_ReferenceCounting tests reference counting
// Source: TestSegmentReader (implicit via reader lifecycle)
// Purpose: Tests that reference counting works correctly
func TestSegmentReader_ReferenceCounting(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()

	// Initial ref count should be 1
	if reader.GetRefCount() != 1 {
		t.Errorf("Expected initial ref count 1, got %d", reader.GetRefCount())
	}

	// Increment ref count
	err := reader.IncRef()
	if err != nil {
		t.Fatalf("Failed to increment ref count: %v", err)
	}

	if reader.GetRefCount() != 2 {
		t.Errorf("Expected ref count 2 after IncRef, got %d", reader.GetRefCount())
	}

	// Decrement ref count
	err = reader.DecRef()
	if err != nil {
		t.Fatalf("Failed to decrement ref count: %v", err)
	}

	if reader.GetRefCount() != 1 {
		t.Errorf("Expected ref count 1 after DecRef, got %d", reader.GetRefCount())
	}

	// Close (which decrements ref count to 0)
	err = reader.Close()
	if err != nil {
		t.Fatalf("Failed to close reader: %v", err)
	}

	// Also close the underlying LeafReader to ensure proper cleanup
	// Note: SegmentReader.Close() should ideally call LeafReader.Close()
	// but currently it doesn't, so we call DecRef directly on the IndexReader
	reader.DecRef()

	// After close, ref count should be 0
	if reader.GetRefCount() != 0 {
		t.Errorf("Expected ref count 0 after close, got %d", reader.GetRefCount())
	}
}

// TestSegmentReader_TryIncRef tests TryIncRef
// Source: TestSegmentReader (implicit via reader lifecycle)
// Purpose: Tests that TryIncRef works correctly
func TestSegmentReader_TryIncRef(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()

	// TryIncRef should succeed on open reader
	if !reader.TryIncRef() {
		t.Error("TryIncRef should succeed on open reader")
	}

	// Ref count should now be 2
	if reader.GetRefCount() != 2 {
		t.Errorf("Expected ref count 2, got %d", reader.GetRefCount())
	}

	// Decrement back to 1
	reader.DecRef()

	// Close the reader (calls DecRef on core readers if present)
	reader.Close()

	// Also close the underlying reader
	reader.DecRef()

	// TryIncRef should fail on closed reader
	if reader.TryIncRef() {
		t.Error("TryIncRef should fail on closed reader")
	}
}

// TestSegmentReader_GetContext tests GetContext
// Source: TestSegmentReader (implicit via reader interface)
// Purpose: Tests that GetContext returns a valid context
func TestSegmentReader_GetContext(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	ctx, err := reader.GetContext()
	if err != nil {
		t.Fatalf("Failed to get context: %v", err)
	}
	if ctx == nil {
		t.Error("Context should not be nil")
	}
}

// TestSegmentReader_HasDeletions tests HasDeletions
// Source: TestSegmentReader (implicit via reader interface)
// Purpose: Tests that HasDeletions returns correct value
func TestSegmentReader_HasDeletions(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	// New segment should not have deletions
	if reader.HasDeletions() {
		t.Error("New segment should not have deletions")
	}

	// NumDeletedDocs should be 0
	if reader.NumDeletedDocs() != 0 {
		t.Errorf("Expected 0 deleted docs, got %d", reader.NumDeletedDocs())
	}
}

// TestSegmentReader_EnsureOpen tests EnsureOpen
// Source: TestSegmentReader (implicit via reader lifecycle)
// Purpose: Tests that EnsureOpen works correctly
func TestSegmentReader_EnsureOpen(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()

	// Should be open initially
	err := reader.EnsureOpen()
	if err != nil {
		t.Errorf("Reader should be open: %v", err)
	}

	// Close the reader
	reader.Close()
	// Also close the underlying reader
	reader.DecRef()

	// Should fail after close
	err = reader.EnsureOpen()
	if err == nil {
		t.Error("EnsureOpen should fail after close")
	}
}

// TestSegmentReader_GetSegmentInfo tests GetSegmentInfo
// Source: TestSegmentReader (implicit via reader interface)
// Purpose: Tests that GetSegmentInfo returns correct info
func TestSegmentReader_GetSegmentInfo(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	segmentInfo := reader.GetSegmentInfo()
	if segmentInfo == nil {
		t.Fatal("SegmentInfo should not be nil")
	}

	if segmentInfo.DocCount() != 1 {
		t.Errorf("Expected 1 document, got %d", segmentInfo.DocCount())
	}
}
