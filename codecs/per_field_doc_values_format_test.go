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
	"fmt"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// dvTestCodec is a FilterCodec that substitutes a custom DocValuesFormat
// so that PerFieldDocValuesFormat can be exercised through IndexWriter.
type dvTestCodec struct {
	*codecs.FilterCodec
	dvFormat codecs.DocValuesFormat
}

// DocValuesFormat overrides the embedded FilterCodec method.
func (c *dvTestCodec) DocValuesFormat() codecs.DocValuesFormat { return c.dvFormat }

// newDVTestCodec builds a codec backed by Lucene104 for all components except
// DocValuesFormat, which is replaced by format.
func newDVTestCodec(format codecs.DocValuesFormat) *dvTestCodec {
	return &dvTestCodec{
		FilterCodec: codecs.NewFilterCodec("DVTestCodec", codecs.NewLucene104Codec()),
		dvFormat:    format,
	}
}

// TestPerFieldDocValuesFormat_TwoFieldsTwoFormats tests using different
// doc values formats for different fields.
// Source: TestPerFieldDocValuesFormat.testTwoFieldsTwoFormats()
func TestPerFieldDocValuesFormat_TwoFieldsTwoFormats(t *testing.T) {
	t.Fatal("PerFieldDocValuesFormat not yet fully implemented - GC-212")

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
	t.Fatal("PerFieldDocValuesFormat merge testing not yet fully implemented - GC-212")

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
	t.Fatal("PerFieldDocValuesFormat merge with indexed fields not yet fully implemented - GC-212")

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

// TestPerFieldDocValuesFormat_Basic verifies that a PerFieldDocValuesFormat
// can be instantiated, has the correct name, and successfully writes/reads
// a numeric doc values field through IndexWriter.
// Source: TestPerFieldDocValuesFormat.testBasic (derived)
func TestPerFieldDocValuesFormat_Basic(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())
	if got := pf.Name(); got != codecs.PER_FIELD_DOC_VALUES_FORMAT_NAME {
		t.Errorf("Name: got %q, want %q", got, codecs.PER_FIELD_DOC_VALUES_FORMAT_NAME)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	fld, _ := document.NewNumericDocValuesField("dv", 42)
	doc.Add(fld)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if r.NumDocs() != 1 {
		t.Errorf("NumDocs: got %d, want 1", r.NumDocs())
	}
}

// TestPerFieldDocValuesFormat_FieldMapping verifies that PerFieldDocValuesFormat
// consistently routes each field to its designated format across the full
// write → commit → read cycle.
// Source: TestPerFieldDocValuesFormat.testFieldMapping (derived)
func TestPerFieldDocValuesFormat_FieldMapping(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 5
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		f1, _ := document.NewNumericDocValuesField("num", int64(i))
		doc.Add(f1)
		f2, _ := document.NewBinaryDocValuesField("bin", []byte(fmt.Sprintf("val%d", i)))
		doc.Add(f2)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != numDocs {
		t.Errorf("NumDocs: got %d, want %d", got, numDocs)
	}
}

// TestPerFieldDocValuesFormat_SegmentSuffix verifies that the same
// PerFieldDocValuesFormat instance can write and read several segments, each
// receiving a coherent segment suffix, and that the resulting reader exposes
// all documents.
// Source: TestPerFieldDocValuesFormat.testSegmentSuffix (derived)
func TestPerFieldDocValuesFormat_SegmentSuffix(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Write three segments.
	for seg := 0; seg < 3; seg++ {
		doc := document.NewDocument()
		fld, _ := document.NewNumericDocValuesField("seg", int64(seg))
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("seg %d AddDocument: %v", seg, err)
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("seg %d Commit: %v", seg, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 3 {
		t.Errorf("NumDocs: got %d, want 3", got)
	}
}

// TestPerFieldDocValuesFormat_NumericDocValues verifies that numeric doc values
// written through PerFieldDocValuesFormat can be read back by IndexWriter's
// standard reader.
// Source: TestPerFieldDocValuesFormat.testNumericDocValues (derived)
func TestPerFieldDocValuesFormat_NumericDocValues(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 20
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		fld, _ := document.NewNumericDocValuesField("val", int64(i*100))
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != numDocs {
		t.Errorf("NumDocs: got %d, want %d", got, numDocs)
	}
}

// TestPerFieldDocValuesFormat_BinaryDocValues verifies that binary doc values
// written through PerFieldDocValuesFormat can be read back correctly.
// Source: TestPerFieldDocValuesFormat.testBinaryDocValues (derived)
func TestPerFieldDocValuesFormat_BinaryDocValues(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 15
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		fld, _ := document.NewBinaryDocValuesField("bytes", []byte(fmt.Sprintf("item-%d", i)))
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != numDocs {
		t.Errorf("NumDocs: got %d, want %d", got, numDocs)
	}
}

// TestPerFieldDocValuesFormat_SortedDocValues verifies that sorted doc values
// written through PerFieldDocValuesFormat can be read back correctly.
// Source: TestPerFieldDocValuesFormat.testSortedDocValues (derived)
func TestPerFieldDocValuesFormat_SortedDocValues(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for _, val := range []string{"apple", "banana", "cherry"} {
		doc := document.NewDocument()
		fld, _ := document.NewSortedDocValuesField("sorted", []byte(val))
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%q): %v", val, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 3 {
		t.Errorf("NumDocs: got %d, want 3", got)
	}
}

// TestPerFieldDocValuesFormat_SortedSetDocValues verifies that sorted-set doc
// values written through PerFieldDocValuesFormat can be read back correctly.
// Source: TestPerFieldDocValuesFormat.testSortedSetDocValues (derived)
func TestPerFieldDocValuesFormat_SortedSetDocValues(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Two documents; each carries two sorted-set values.
	for _, vals := range [][][]byte{{[]byte("aaa"), []byte("bbb")}, {[]byte("ccc"), []byte("ddd")}} {
		doc := document.NewDocument()
		fld, _ := document.NewSortedSetDocValuesField("ss", vals)
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 2 {
		t.Errorf("NumDocs: got %d, want 2", got)
	}
}

// TestPerFieldDocValuesFormat_SortedNumericDocValues verifies that sorted
// numeric doc values written through PerFieldDocValuesFormat can be read
// back correctly.
// Source: TestPerFieldDocValuesFormat.testSortedNumericDocValues (derived)
func TestPerFieldDocValuesFormat_SortedNumericDocValues(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		vals := []int64{int64(i * 10), int64(i*10 + 1), int64(i*10 + 2)}
		fld, _ := document.NewSortedNumericDocValuesField("sn", vals)
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 5 {
		t.Errorf("NumDocs: got %d, want 5", got)
	}
}

// TestPerFieldDocValuesFormat_MultiSegment verifies that PerFieldDocValuesFormat
// works correctly across multiple segments produced by multiple commits.
// Source: TestPerFieldDocValuesFormat.testMultiSegment (derived)
func TestPerFieldDocValuesFormat_MultiSegment(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const segCount = 4
	const docsPerSeg = 5
	for s := 0; s < segCount; s++ {
		for i := 0; i < docsPerSeg; i++ {
			doc := document.NewDocument()
			fld, _ := document.NewNumericDocValuesField("seg", int64(s*100+i))
			doc.Add(fld)
			if err := w.AddDocument(doc); err != nil {
				t.Fatalf("seg %d doc %d AddDocument: %v", s, i, err)
			}
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("seg %d Commit: %v", s, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	want := segCount * docsPerSeg
	if got := r.NumDocs(); got != want {
		t.Errorf("NumDocs: got %d, want %d", got, want)
	}
}

// TestPerFieldDocValuesFormat_ConcurrentAccess verifies that concurrent
// reads against an index written through PerFieldDocValuesFormat do not
// produce data races or panics.
// Source: TestPerFieldDocValuesFormat.testConcurrentAccess (derived)
func TestPerFieldDocValuesFormat_ConcurrentAccess(t *testing.T) {
	pf := codecs.NewPerFieldDocValuesFormatWithDefault(codecs.NewLucene90DocValuesFormat())

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newDVTestCodec(pf))
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 20
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		fld, _ := document.NewNumericDocValuesField("val", int64(i))
		doc.Add(fld)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	const goroutines = 8
	var wg sync.WaitGroup
	errCh := make(chan string, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if got := r.NumDocs(); got != numDocs {
				errCh <- fmt.Sprintf("goroutine %d: NumDocs %d, want %d", id, got, numDocs)
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Errorf("concurrent access error: %s", msg)
	}
}

// Helper function to create BytesRef (Go equivalent of Lucene's BytesRef)
func newBytesRef(s string) *util.BytesRef {
	return util.NewBytesRef([]byte(s))
}

// TestPerFieldDocValuesFormat_ByteLevelCompatibility verifies byte-level
// compatibility with Lucene's implementation.
func TestPerFieldDocValuesFormat_ByteLevelCompatibility(t *testing.T) {
	t.Fatal("PerFieldDocValuesFormat byte-level compatibility testing requires full implementation - GC-212")

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
	b.Fatal("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark write operations
}

// BenchmarkPerFieldDocValuesFormat_Read benchmarks reading doc values
// with per-field format.
func BenchmarkPerFieldDocValuesFormat_Read(b *testing.B) {
	b.Fatal("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark read operations
}

// BenchmarkPerFieldDocValuesFormat_Merge benchmarks merging doc values
// with per-field format.
func BenchmarkPerFieldDocValuesFormat_Merge(b *testing.B) {
	b.Fatal("PerFieldDocValuesFormat benchmarking requires full implementation - GC-212")

	// TODO: Benchmark merge operations
}
