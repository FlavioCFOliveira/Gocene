// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs_test contains tests for the codecs package.
//
// Ported from Apache Lucene's org.apache.lucene.codecs.perfield.TestPerFieldDocValuesFormat
// Source: lucene/core/src/test/org/apache/lucene/codecs/perfield/TestPerFieldDocValuesFormat.java
//
// GC-212: Test PerFieldDocValuesFormat
package codecs_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPerFieldDocValuesFormat_TwoFieldsTwoFormats tests using different
// doc values formats for different fields.
// Source: TestPerFieldDocValuesFormat.testTwoFieldsTwoFormats()
func TestPerFieldDocValuesFormat_TwoFieldsTwoFormats(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat not yet fully implemented - GC-212")

	// This test verifies that different fields can use different doc values formats.
	// In the Java test:
	// - dv1 field uses the "fast" format (default)
	// - dv2 field uses the "slow" format (Asserting)
	//
	// The test creates a document with:
	// - A text field (fieldname) that is indexed and stored
	// - A numeric doc values field (dv1)
	// - A binary doc values field (dv2)
	//
	// It then verifies that:
	// - The document can be searched
	// - The doc values can be retrieved correctly
	// - Each field uses its specified format

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create analyzer
	analyzer := analysis.NewWhitespaceAnalyzer()

	// Create index writer config with custom codec
	config := index.NewIndexWriterConfig(analyzer)

	// TODO: Set up PerFieldDocValuesFormat codec
	// The codec should return different DocValuesFormat based on field name:
	// - "dv1" -> default format
	// - "dv2" -> asserting format (or another test format)
	_ = config

	// TODO: Create IndexWriter with custom config
	// writer, err := index.NewIndexWriter(dir, config)
	// if err != nil {
	//     t.Fatalf("Failed to create IndexWriter: %v", err)
	// }
	// defer writer.Close()

	// Create document with multiple fields
	doc := document.NewDocument()

	// Add text field
	longTerm := "longtermlongtermlongtermlongtermlongtermlongtermlongtermlongterm" +
		"longtermlongtermlongtermlongtermlongtermlongtermlong" +
		"termlongtermlongtermlongterm"
	text := "This is the text to be indexed. " + longTerm

	textField, err := document.NewTextField("fieldname", text, true)
	if err != nil {
		t.Fatalf("Failed to create text field: %v", err)
	}
	doc.Add(textField)

	// Add numeric doc values field
	numericDV, err := document.NewNumericDocValuesField("dv1", 5)
	if err != nil {
		t.Fatalf("Failed to create numeric doc values field: %v", err)
	}
	doc.Add(numericDV)

	// Add binary doc values field
	binaryDV, err := document.NewBinaryDocValuesField("dv2", []byte("hello world"))
	if err != nil {
		t.Fatalf("Failed to create binary doc values field: %v", err)
	}
	doc.Add(binaryDV)

	// TODO: Add document to index
	// err = writer.AddDocument(doc)
	// if err != nil {
	//     t.Fatalf("Failed to add document: %v", err)
	// }
	// writer.Close()

	// TODO: Open reader and verify doc values
	// reader, err := index.NewDirectoryReader(dir)
	// if err != nil {
	//     t.Fatalf("Failed to open reader: %v", err)
	// }
	// defer reader.Close()
	//
	// Verify:
	// - Search for longTerm returns 1 hit
	// - Search for "text" returns 1 hit
	// - dv1 has value 5
	// - dv2 has value "hello world"
}

// TestPerFieldDocValuesFormat_MergeCalledOnTwoFormats tests that merge is called
// correctly when using multiple doc values formats.
// Source: TestPerFieldDocValuesFormat.testMergeCalledOnTwoFormats()
func TestPerFieldDocValuesFormat_MergeCalledOnTwoFormats(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat merge testing not yet fully implemented - GC-212")

	// This test verifies that when segments are merged:
	// - The merge method is called on each DocValuesFormat
	// - Fields using the same format are merged together
	// - Field names are correctly tracked during merge
	//
	// In the Java test:
	// - dv1 and dv2 use format dvf1
	// - dv3 uses format dvf2
	// - After merging segments, each format's merge is called exactly once
	// - The field names passed to merge are correctly tracked

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Create merge-recording doc values format wrappers
	// These wrappers track:
	// - Number of merge calls (nbMergeCalls)
	// - Field names passed to merge (fieldNames)

	// Create index writer config with custom codec
	config := index.NewIndexWriterConfig(nil)

	// TODO: Set up codec that returns different formats based on field:
	// - "dv1", "dv2" -> dvf1 (wrapper that records merge calls)
	// - "dv3" -> dvf2 (wrapper that records merge calls)
	_ = config

	// TODO: Create IndexWriter

	// Add first document with all three fields
	doc1 := document.NewDocument()
	numericDV1, _ := document.NewNumericDocValuesField("dv1", 5)
	doc1.Add(numericDV1)
	numericDV2, _ := document.NewNumericDocValuesField("dv2", 42)
	doc1.Add(numericDV2)
	binaryDV3, _ := document.NewBinaryDocValuesField("dv3", []byte("hello world"))
	doc1.Add(binaryDV3)

	// TODO: Add doc1 and commit

	// Add second document with all three fields
	doc2 := document.NewDocument()
	numericDV1b, _ := document.NewNumericDocValuesField("dv1", 8)
	doc2.Add(numericDV1b)
	numericDV2b, _ := document.NewNumericDocValuesField("dv2", 45)
	doc2.Add(numericDV2b)
	binaryDV3b, _ := document.NewBinaryDocValuesField("dv3", []byte("goodbye world"))
	doc2.Add(binaryDV3b)

	// TODO: Add doc2 and commit

	// TODO: Force merge to 1 segment

	// TODO: Verify:
	// - dvf1.nbMergeCalls == 1
	// - dvf1.fieldNames contains "dv1" and "dv2"
	// - dvf2.nbMergeCalls == 1
	// - dvf2.fieldNames contains "dv3"
	_ = doc1
	_ = doc2
}

// TestPerFieldDocValuesFormat_MergeWithIndexedFields tests merging doc values
// with regular indexed fields (that don't have doc values).
// Source: TestPerFieldDocValuesFormat.testDocValuesMergeWithIndexedFields()
func TestPerFieldDocValuesFormat_MergeWithIndexedFields(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat merge with indexed fields not yet fully implemented - GC-212")

	// This test verifies that when merging segments:
	// - Only fields with doc values are passed to the DocValuesFormat merge
	// - Regular indexed fields (without doc values) are ignored
	//
	// In the Java test:
	// - Document 1 has: dv1 (doc values), normalField (indexed text)
	// - Document 2 has: anotherField (indexed text), normalField (indexed text)
	// - After merge, only "dv1" should be in the merge field names

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Create merge-recording doc values format wrapper

	// Create index writer config
	config := index.NewIndexWriterConfig(nil)

	// TODO: Set up codec that uses recording format for all fields
	_ = config

	// TODO: Create IndexWriter

	// Add first document with doc values and indexed field
	doc1 := document.NewDocument()
	numericDV, _ := document.NewNumericDocValuesField("dv1", 5)
	doc1.Add(numericDV)
	textField1, _ := document.NewTextField("normalField", "not a doc value", false)
	doc1.Add(textField1)

	// TODO: Add doc1 and commit

	// Add second document with only indexed fields (no doc values)
	doc2 := document.NewDocument()
	textField2, _ := document.NewTextField("anotherField", "again no doc values here", false)
	doc2.Add(textField2)
	textField3, _ := document.NewTextField("normalField", "my document without doc values", false)
	doc2.Add(textField3)

	// TODO: Add doc2 and commit

	// TODO: Force merge to 1 segment

	// TODO: Verify:
	// - nbMergeCalls == 1
	// - fieldNames contains only "dv1" (not "normalField" or "anotherField")
	_ = doc1
	_ = doc2
}

// MergeRecordingDocValuesFormat is a wrapper around a DocValuesFormat that
// records merge calls and field names.
// This is the Go equivalent of the Java test's MergeRecordingDocValueFormatWrapper.
type MergeRecordingDocValuesFormat struct {
	name         string
	delegate     DocValuesFormat // TODO: Define DocValuesFormat interface
	fieldNames   []string
	fieldNamesMu sync.Mutex
	nbMergeCalls int
	mergeCallsMu sync.Mutex
}

// NewMergeRecordingDocValuesFormat creates a new recording wrapper.
func NewMergeRecordingDocValuesFormat(name string, delegate DocValuesFormat) *MergeRecordingDocValuesFormat {
	return &MergeRecordingDocValuesFormat{
		name:       name,
		delegate:   delegate,
		fieldNames: make([]string, 0),
	}
}

// GetName returns the format name.
func (f *MergeRecordingDocValuesFormat) GetName() string {
	return f.name
}

// RecordMerge records a merge call with the given field names.
func (f *MergeRecordingDocValuesFormat) RecordMerge(fields []string) {
	f.mergeCallsMu.Lock()
	f.nbMergeCalls++
	f.mergeCallsMu.Unlock()

	f.fieldNamesMu.Lock()
	f.fieldNames = append(f.fieldNames, fields...)
	f.fieldNamesMu.Unlock()
}

// GetFieldNames returns the recorded field names.
func (f *MergeRecordingDocValuesFormat) GetFieldNames() []string {
	f.fieldNamesMu.Lock()
	defer f.fieldNamesMu.Unlock()
	result := make([]string, len(f.fieldNames))
	copy(result, f.fieldNames)
	return result
}

// GetMergeCallCount returns the number of merge calls.
func (f *MergeRecordingDocValuesFormat) GetMergeCallCount() int {
	f.mergeCallsMu.Lock()
	defer f.mergeCallsMu.Unlock()
	return f.nbMergeCalls
}

// DocValuesFormat is a placeholder for the DocValuesFormat interface.
// TODO: This should be replaced with the actual implementation when available.
type DocValuesFormat interface {
	GetName() string
	// FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error)
	// FieldsProducer(state *SegmentReadState) (DocValuesProducer, error)
}

// TestPerFieldDocValuesFormat_Basic is a basic test for PerFieldDocValuesFormat.
// It tests that the format can be instantiated and basic operations work.
func TestPerFieldDocValuesFormat_Basic(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat not yet fully implemented - GC-212")

	// TODO: Implement basic test once DocValuesFormat is available
	// This should test:
	// - Creating a PerFieldDocValuesFormat
	// - Getting format for a field
	// - Basic field format mapping
}

// TestPerFieldDocValuesFormat_FieldMapping tests field to format mapping.
func TestPerFieldDocValuesFormat_FieldMapping(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat field mapping not yet fully implemented - GC-212")

	// TODO: Test that:
	// - Fields are correctly mapped to their specified formats
	// - Default format is used when no specific format is specified
	// - Field format mapping is consistent across operations
}

// TestPerFieldDocValuesFormat_SegmentSuffix tests that segment suffix is respected.
// This is mentioned as a TODO in the Java source.
func TestPerFieldDocValuesFormat_SegmentSuffix(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat segment suffix testing not yet fully implemented - GC-212")

	// TODO: Test that segment suffix is respected by all codec APIs
	// This is important for ensuring that different formats don't conflict
	// when writing to the same directory.
}

// TestPerFieldDocValuesFormat_NumericDocValues tests numeric doc values
// with per-field format.
func TestPerFieldDocValuesFormat_NumericDocValues(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat numeric doc values not yet fully implemented - GC-212")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Test numeric doc values with per-field format
	// - Create documents with numeric doc values
	// - Use different formats for different numeric fields
	// - Verify values are correctly stored and retrieved
}

// TestPerFieldDocValuesFormat_BinaryDocValues tests binary doc values
// with per-field format.
func TestPerFieldDocValuesFormat_BinaryDocValues(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat binary doc values not yet fully implemented - GC-212")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Test binary doc values with per-field format
	// - Create documents with binary doc values
	// - Use different formats for different binary fields
	// - Verify values are correctly stored and retrieved
}

// TestPerFieldDocValuesFormat_SortedDocValues tests sorted doc values
// with per-field format.
func TestPerFieldDocValuesFormat_SortedDocValues(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat sorted doc values not yet fully implemented - GC-212")

	// TODO: Test sorted doc values with per-field format
	// Sorted doc values are used for faceting and sorting
}

// TestPerFieldDocValuesFormat_SortedSetDocValues tests sorted set doc values
// with per-field format.
func TestPerFieldDocValuesFormat_SortedSetDocValues(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat sorted set doc values not yet fully implemented - GC-212")

	// TODO: Test sorted set doc values with per-field format
	// Sorted set doc values are used for multi-valued fields
}

// TestPerFieldDocValuesFormat_SortedNumericDocValues tests sorted numeric doc values
// with per-field format.
func TestPerFieldDocValuesFormat_SortedNumericDocValues(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat sorted numeric doc values not yet fully implemented - GC-212")

	// TODO: Test sorted numeric doc values with per-field format
	// Sorted numeric doc values are used for multi-valued numeric fields
}

// TestPerFieldDocValuesFormat_MultiSegment tests per-field doc values
// across multiple segments.
func TestPerFieldDocValuesFormat_MultiSegment(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat multi-segment testing not yet fully implemented - GC-212")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Test with multiple segments
	// - Add documents with commits in between
	// - Verify doc values work correctly across segments
	// - Test merging segments
}

// TestPerFieldDocValuesFormat_ConcurrentAccess tests concurrent access
// to per-field doc values.
func TestPerFieldDocValuesFormat_ConcurrentAccess(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat concurrent access testing not yet fully implemented - GC-212")

	// TODO: Test thread safety
	// - Concurrent reads from multiple threads
	// - Concurrent writes (if applicable)
}

// Helper function to create BytesRef (Go equivalent of Lucene's BytesRef)
func newBytesRef(s string) *util.BytesRef {
	return util.NewBytesRef([]byte(s))
}

// TestPerFieldDocValuesFormat_ByteLevelCompatibility verifies byte-level
// compatibility with Lucene's implementation.
func TestPerFieldDocValuesFormat_ByteLevelCompatibility(t *testing.T) {
	t.Skip("PerFieldDocValuesFormat byte-level compatibility testing requires full implementation - GC-212")

	// This test will verify that the Go implementation produces
	// byte-identical output to the Java implementation for the same input.
	// This is a key requirement for Gocene.

	// TODO: Implement byte-level compatibility test
	// - Create identical documents in Java and Go
	// - Compare the serialized bytes
	// - Ensure they are identical
}

// BenchmarkPerFieldDocValuesFormat_Write benchmarks writing doc values
// with per-field format.
func BenchmarkPerFieldDocValuesFormat_Write(b *testing.B) {
	b.Skip("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark write operations
}

// BenchmarkPerFieldDocValuesFormat_Read benchmarks reading doc values
// with per-field format.
func BenchmarkPerFieldDocValuesFormat_Read(b *testing.B) {
	b.Skip("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark read operations
}

// BenchmarkPerFieldDocValuesFormat_Merge benchmarks merging doc values
// with per-field format.
func BenchmarkPerFieldDocValuesFormat_Merge(b *testing.B) {
	b.Skip("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark merge operations
}
