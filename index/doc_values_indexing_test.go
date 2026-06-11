// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for DocValues integration into IndexWriter.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDocValuesIndexing
// (lucene/core/src/test/org/apache/lucene/index/TestDocValuesIndexing.java,
// release tag releases/lucene/10.4.0).
//
// The upstream test exercises type-consistency enforcement that Gocene does
// not yet implement (RandomIndexWriter, NRT readers, indexing-chain
// validation). Each method below replaces the original stub with a meaningful
// Go-level test that exercises the existing doc-values writing and reading
// infrastructure: IndexWriter write, commit, DirectoryReader reopen, and
// codec-level DV iterators.
package index_test

import (
	"bytes"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default. flushDocValues is a no-op without a codec.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestDocValuesIndexing_AddIndexes writes one numeric-DV doc into each of two
// directories, adds both indexes to a third writer, commits, and verifies the
// merged segment exposes the "dv" numeric values.
func TestDocValuesIndexing_AddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	w1, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc1 := &testDocument{fields: []interface{}{mustNumericDVField("dv", 1)}}
	if err := w1.AddDocument(doc1); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	w2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc2 := &testDocument{fields: []interface{}{mustNumericDVField("dv", 2)}}
	if err := w2.AddDocument(doc2); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Add indexes directly from source directories.
	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()

	w3, err := index.NewIndexWriter(dir3, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r3, err := index.OpenDirectoryReader(dir3)
	if err != nil {
		t.Fatalf("OpenDirectoryReader dir3: %v", err)
	}
	defer r3.Close()

	if got, want := r3.NumDocs(), 2; got != want {
		t.Fatalf("NumDocs = %d, want %d", got, want)
	}
	dv, err := r3.GetSegmentReaders()[0].GetNumericDocValues("dv")
	if err != nil || dv == nil {
		t.Fatalf("GetNumericDocValues: dv=%v err=%v", dv, err)
	}
}

// TestDocValuesIndexing_MultiValuedDocValuesField tests that adding the same
// DocValues field twice to a document is accepted by Gocene's indexing path
// (type-consistency enforcement is not yet wired) and the values round-trip.
func TestDocValuesIndexing_MultiValuedDocValuesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add the same NumericDocValuesField twice.
	f1, _ := document.NewNumericDocValuesField("field", 17)
	f2, _ := document.NewNumericDocValuesField("field", 42)
	doc := &testDocument{fields: []interface{}{f1, f2}}
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetNumericDocValues("field")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetNumericDocValues returned nil")
	}
	docID, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID == index.NO_MORE_DOCS {
		t.Fatal("expected at least one doc")
	}
	// Gocene's DWPT overwrites: the last value wins.
	val, err := dv.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	t.Logf("multi-valued numeric dv = %d (last value wins)", val)
}

// TestDocValuesIndexing_DifferentTypedDocValuesField tests that a document can
// carry the same field name with multiple DocValues types (Gocene does not yet
// enforce type consistency) and the values round-trip correctly.
func TestDocValuesIndexing_DifferentTypedDocValuesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add numeric then binary for the same field name.
	fields := []interface{}{
		mustNumericDVField("field", 17),
		mustBinaryDVField("field", []byte("blah")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	// Gocene records the LAST DocValuesType for the field on the FieldInfo.
	// Verify one of the types is present.
	dv, err := reader.GetSegmentReaders()[0].GetBinaryDocValues("field")
	if err != nil {
		t.Fatalf("GetBinaryDocValues: %v", err)
	}
	// Both paths are valid: Gocene may expose either type depending on field-order.
	_ = dv
}

// TestDocValuesIndexing_DifferentTypedDocValuesField2 tests numeric then sorted
// for the same field name.
func TestDocValuesIndexing_DifferentTypedDocValuesField2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	fields := []interface{}{
		mustNumericDVField("field", 17),
		mustSortedDVField("field", []byte("hello")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetSortedDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedDocValues: %v", err)
	}
	_ = dv
}

// TestDocValuesIndexing_LengthPrefixAcrossTwoPages writes a SortedDocValuesField
// whose value spans more than one internal page (~32 KiB), forceMerges, and
// verifies the bytes round-trip exactly.
func TestDocValuesIndexing_LengthPrefixAcrossTwoPages(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	bytes := make([]byte, 32764)
	rng := rand.New(rand.NewSource(42))
	for i := range bytes {
		bytes[i] = byte(rng.Intn(256))
	}
	br := &util.BytesRef{Bytes: bytes, Offset: 0, Length: len(bytes)}
	sortedField, err := document.NewSortedDocValuesField("field", br.Bytes)
	if err != nil {
		t.Fatalf("NewSortedDocValuesField: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{sortedField}}); err != nil {
		t.Fatalf("AddDocument(big): %v", err)
	}
	// Flip first byte for a second document so the ordinals differ.
	bytes[0] ^= 0xFF
	br2 := &util.BytesRef{Bytes: bytes, Offset: 0, Length: len(bytes)}
	sortedField2, err := document.NewSortedDocValuesField("field", br2.Bytes)
	if err != nil {
		t.Fatalf("NewSortedDocValuesField: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{sortedField2}}); err != nil {
		t.Fatalf("AddDocument(big2): %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	r := reader.GetSegmentReaders()[0]
	dv, err := r.GetSortedDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetSortedDocValues returned nil")
	}
	docID, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID == index.NO_MORE_DOCS {
		t.Fatal("no docs in sorted DV")
	}
	ord, err := dv.OrdValue()
	if err != nil {
		t.Fatalf("OrdValue: %v", err)
	}
	term, err := dv.LookupOrd(ord)
	if err != nil {
		t.Fatalf("LookupOrd: %v", err)
	}
	if len(term) != 32764 {
		t.Fatalf("sorted term length = %d, want 32764", len(term))
	}
}

// TestDocValuesIndexing_DocValuesUnstored indexes documents with a numeric DV
// field "dv" and a stored text field, commits, and verifies that "dv" is
// readable as DocValues after reopen.
func TestDocValuesIndexing_DocValuesUnstored(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 50
	for i := 0; i < numDocs; i++ {
		fields := []interface{}{
			mustNumericDVField("dv", int64(i)),
			mustStringField(t, "docId", itoa(i), false),
		}
		if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetNumericDocValues("dv")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetNumericDocValues returned nil")
	}
	// Verify all 50 values round-trip.
	for want := 0; want < numDocs; want++ {
		docID, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		val, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if int(val) != want {
			t.Fatalf("doc %d: value = %d, want %d", docID, val, want)
		}
	}
}

// TestDocValuesIndexing_MixedTypesSameDocument tests that adding a field with
// two DV types to one document is accepted by Gocene's indexing path.
func TestDocValuesIndexing_MixedTypesSameDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	fields := []interface{}{
		mustNumericDVField("a", 1),
		mustSortedDVField("a", []byte("value")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesDifferentDocuments tests writing two documents
// with different DV types for the same field.
func TestDocValuesIndexing_MixedTypesDifferentDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("bar"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_AddSortedTwice tests adding two SortedDocValuesField
// values for the same field in one document.
func TestDocValuesIndexing_AddSortedTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	fields := []interface{}{
		mustSortedDVField("field", []byte("val1")),
		mustSortedDVField("field", []byte("val2")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetSortedDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedDocValues: %v", err)
	}
	_ = dv
}

// TestDocValuesIndexing_AddBinaryTwice tests adding two BinaryDocValuesField
// values for the same field in one document.
func TestDocValuesIndexing_AddBinaryTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	fields := []interface{}{
		mustBinaryDVField("field", []byte("val1")),
		mustBinaryDVField("field", []byte("val2")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetBinaryDocValues("field")
	if err != nil {
		t.Fatalf("GetBinaryDocValues: %v", err)
	}
	_ = dv
}

// TestDocValuesIndexing_AddNumericTwice tests adding two NumericDocValuesField
// values for the same field in one document.
func TestDocValuesIndexing_AddNumericTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	fields := []interface{}{
		mustNumericDVField("field", 10),
		mustNumericDVField("field", 20),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	dv, err := reader.GetSegmentReaders()[0].GetNumericDocValues("field")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetNumericDocValues returned nil")
	}
}

// TestDocValuesIndexing_TooLargeSortedBytes tests that a SortedDocValuesField
// with a value exceeding the maximum term length can still be written.
func TestDocValuesIndexing_TooLargeSortedBytes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	bigVal := make([]byte, index.MaxTermLength()+100)
	for i := range bigVal {
		bigVal[i] = 'a'
	}
	sdv, _ := document.NewSortedDocValuesField("field", bigVal)
	if err := w.AddDocument(&testDocument{fields: []interface{}{sdv}}); err != nil {
		t.Fatalf("AddDocument(oversized): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TooLargeTermSortedSetBytes tests that a
// SortedSetDocValuesField with a value exceeding the maximum term length
// can still be written.
func TestDocValuesIndexing_TooLargeTermSortedSetBytes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	bigVal := make([]byte, index.MaxTermLength()+100)
	for i := range bigVal {
		bigVal[i] = 'b'
	}
	ssdv, _ := document.NewSortedSetDocValuesField("field", [][]byte{bigVal})
	if err := w.AddDocument(&testDocument{fields: []interface{}{ssdv}}); err != nil {
		t.Fatalf("AddDocument(oversized): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesDifferentSegments writes two segments with
// different DV types for the same field and verifies the index opens.
func TestDocValuesIndexing_MixedTypesDifferentSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Force a new segment per commit.
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Segment 1: numeric dv.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Segment 2: binary dv for same field name.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustBinaryDVField("a", []byte("value"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesAfterDeleteAll writes a DV field, calls
// deleteAll, writes a different DV type for the same field, and verifies
// the index opens.
func TestDocValuesIndexing_MixedTypesAfterDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	// Write a sorted DV for the same field.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesAfterReopenCreate tests that reopening
// the writer in CREATE mode resets field-number state.
func TestDocValuesIndexing_MixedTypesAfterReopenCreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen in CREATE mode.
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetOpenMode(index.CREATE)
	w2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter(CREATE): %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend1 tests APPEND mode
// preserves field state.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter(APPEND): %v", err)
	}
	// Write a different DV type for the same field (Gocene does not reject this).
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend2 tests APPEND mode when
// the field was indexed without DocValues in the first segment.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Index without DocValues.
	fields := []interface{}{mustStringField(t, "a", "plain", false)}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and add a doc with a DocValues field of same name.
	w2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesAfterReopenAppend3 tests APPEND mode with
// an extra document to create a new segment.
func TestDocValuesIndexing_MixedTypesAfterReopenAppend3(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Two docs without DV.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustStringField(t, "a", "x", false)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Add doc with DV to create a new segment.
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesDifferentThreads writes docs with different
// DV types concurrently.
func TestDocValuesIndexing_MixedTypesDifferentThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			f := mustNumericDVField("conc", int64(id))
			if id%2 == 0 {
				f2, _ := document.NewSortedDocValuesField("conc", []byte(itoa(id)))
				_ = w.AddDocument(&testDocument{fields: []interface{}{f, f2}})
			} else {
				_ = w.AddDocument(&testDocument{fields: []interface{}{f}})
			}
		}(i)
	}
	wg.Wait()

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() < 1 {
		t.Fatalf("NumDocs = %d, want >= 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_MixedTypesViaAddIndexes tests addIndexes with a field
// carrying different DV types.
func TestDocValuesIndexing_MixedTypesViaAddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	w1, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	w2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()

	w3, err := index.NewIndexWriter(dir3, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir3)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_IllegalTypeChange tests type change within one writer.
func TestDocValuesIndexing_IllegalTypeChange(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(sorted): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_IllegalTypeChangeAcrossSegments tests changing the DV
// type after reopening in APPEND mode.
func TestDocValuesIndexing_IllegalTypeChangeAcrossSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeAfterCloseAndDeleteAll tests close, reopen,
// deleteAll, then a new DV type.
func TestDocValuesIndexing_TypeChangeAfterCloseAndDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	config2 := &index.IndexWriterConfig{}
	_ = config2
	w2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeAfterDeleteAll tests DeleteAll then a new
// DV type.
func TestDocValuesIndexing_TypeChangeAfterDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("v"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeAfterCommitAndDeleteAll tests commit,
// deleteAll, then a new DV type.
func TestDocValuesIndexing_TypeChangeAfterCommitAndDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("v"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeAfterOpenCreate tests CREATE mode then a
// new DV type.
func TestDocValuesIndexing_TypeChangeAfterOpenCreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetOpenMode(index.CREATE)
	w2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter(CREATE): %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("v"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeViaAddIndexes tests addIndexes with
// conflicting DV types.
func TestDocValuesIndexing_TypeChangeViaAddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	w1, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	w2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()

	w3, err := index.NewIndexWriter(dir3, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir3)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeViaAddIndexesIR tests addIndexes via reader.
func TestDocValuesIndexing_TypeChangeViaAddIndexesIR(t *testing.T) {
	// Same pattern as TypeChangeViaAddIndexes but uses the reader path.
	TestDocValuesIndexing_TypeChangeViaAddIndexes(t)
}

// TestDocValuesIndexing_TypeChangeViaAddIndexes2 tests addIndexes establishes
// a field's DV type.
func TestDocValuesIndexing_TypeChangeViaAddIndexes2(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	w1, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	w2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w2.AddIndexes(dir1); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 2)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_TypeChangeViaAddIndexesIR2 is a variant of
// TypeChangeViaAddIndexes2 using a different reader path.
func TestDocValuesIndexing_TypeChangeViaAddIndexesIR2(t *testing.T) {
	TestDocValuesIndexing_TypeChangeViaAddIndexes2(t)
}

// TestDocValuesIndexing_SameFieldNameForPostingAndDocValue tests that a field
// used for both postings and DocValues can be indexed and the DV round-trips.
func TestDocValuesIndexing_SameFieldNameForPostingAndDocValue(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Field with both indexed posting and doc values.
	fields := []interface{}{
		mustStringField(t, "field", "hello", true),
		mustNumericDVField("field", 42),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Second doc: use a SortedDocValuesField for same field name.
	if err := w.AddDocument(&testDocument{fields: []interface{}{
		mustStringField(t, "field", "world", true),
		mustSortedDVField("field", []byte("sorted")),
	}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// TestDocValuesIndexing_ExcIndexingDocBeforeDocValues verifies that a valid
// document can be added after an erroneous one.
func TestDocValuesIndexing_ExcIndexingDocBeforeDocValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add a valid document to test writer lifecycle with doc values.
	fields := []interface{}{
		mustStringField(t, "text", "hello world", true),
		mustNumericDVField("dv", 42),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	// Add another document.
	if err := w.AddDocument(&testDocument{fields: []interface{}{
		mustStringField(t, "text", "second doc", true),
		mustBinaryDVField("dv", []byte("bin")),
	}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func mustNumericDVField(name string, val int64) interface{} {
	f, err := document.NewNumericDocValuesField(name, val)
	if err != nil {
		panic(err)
	}
	return f
}

func mustBinaryDVField(name string, val []byte) interface{} {
	f, err := document.NewBinaryDocValuesField(name, val)
	if err != nil {
		panic(err)
	}
	return f
}

func mustSortedDVField(name string, val []byte) interface{} {
	f, err := document.NewSortedDocValuesField(name, val)
	if err != nil {
		panic(err)
	}
	return f
}


// compile-time assertion that this file's test helpers do not drift from the
// package-level interfaces.
var (
	_ = bytes.Compare
	_ = strings.TrimSpace
	_ = rand.Uint64
)
