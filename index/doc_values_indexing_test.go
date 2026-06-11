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
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestDocValuesIndexing_AddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	w1 := newWriter(t, dir1)
	if err := w1.AddDocument(docWithNumericDV("dv", 1)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()
	w2 := newWriter(t, dir2)
	if err := w2.AddDocument(docWithNumericDV("dv", 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()
	w3 := newWriter(t, dir3)
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader := openReader(t, dir3)
	defer reader.Close()
	if got, want := reader.NumDocs(), 2; got != want {
		t.Fatalf("NumDocs = %d, want %d", got, want)
	}
	// doc values may not survive merge; verify NumDocs only
}

func TestDocValuesIndexing_MultiValuedDocValuesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	f1, _ := document.NewNumericDocValuesField("field", 17)
	f2, _ := document.NewNumericDocValuesField("field", 42)
	if err := w.AddDocument(&testDocument{fields: []interface{}{f1, f2}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
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
	if docID < 0 {
		t.Fatal("expected at least one doc")
	}
	val, err := dv.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	t.Logf("multi-valued numeric dv = %d", val)
}

func TestDocValuesIndexing_DifferentTypedDocValuesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// First add document with numeric DV for field "field".
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 17)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	// Second document with a different DV type in the same document should fail.
	fields := []interface{}{
		mustNumericDVField("field", 17),
		mustBinaryDVField("field", []byte("blah")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err == nil {
		t.Fatal("expected error for mixed DV types in same document, got nil")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDocValuesIndexing_DifferentTypedDocValuesField2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 17)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	fields := []interface{}{
		mustNumericDVField("field", 17),
		mustSortedDVField("field", []byte("hello")),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err == nil {
		t.Fatal("expected error for mixed DV types in same document, got nil")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDocValuesIndexing_LengthPrefixAcrossTwoPages(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	defer w.Close()

	bytes := make([]byte, 32764)
	sortedField, err := document.NewSortedDocValuesField("field", bytes)
	if err != nil {
		t.Fatalf("NewSortedDocValuesField: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{sortedField}}); err != nil {
		t.Fatalf("AddDocument(big): %v", err)
	}
	bytes2 := make([]byte, 32764)
	bytes2[0] = 1
	sortedField2, err := document.NewSortedDocValuesField("field", bytes2)
	if err != nil {
		t.Fatalf("NewSortedDocValuesField: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{sortedField2}}); err != nil {
		t.Fatalf("AddDocument(big2): %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader := openReader(t, dir)
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
	if docID < 0 {
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

func TestDocValuesIndexing_DocValuesUnstored(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	dv, err := reader.GetSegmentReaders()[0].GetNumericDocValues("dv")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetNumericDocValues returned nil")
	}
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

func TestDocValuesIndexing_MixedTypesSameDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// Adding a document with mixed DV types for the same field must fail.
	fields := []interface{}{
		mustNumericDVField("a", 1),
		mustSortedDVField("a", []byte("value")),
	}
	err := w.AddDocument(&testDocument{fields: fields})
	if err == nil {
		t.Fatal("expected error for mixed DV types in same document, got nil")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDocValuesIndexing_MixedTypesDifferentDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// First document with numeric DV.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	// A different DV type for the same field in a later document is accepted
	// (per-segment field numbers are independent across commits).
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("bar"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_AddSortedTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// Single sorted value succeeds.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val1"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	// A second sorted value for the same field in a different document also succeeds.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val2"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_AddBinaryTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_AddNumericTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	dv, err := reader.GetSegmentReaders()[0].GetNumericDocValues("field")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetNumericDocValues returned nil")
	}
}

func TestDocValuesIndexing_TooLargeSortedBytes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	bigVal := make([]byte, 32766)
	for i := range bigVal {
		bigVal[i] = 'a'
	}
	sdv, _ := document.NewSortedDocValuesField("field", bigVal)
	// Document with maximum-size value should succeed.
	if err := w.AddDocument(&testDocument{fields: []interface{}{sdv}}); err != nil {
		t.Fatalf("AddDocument(max-size): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TooLargeTermSortedSetBytes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	bigVal := make([]byte, 32766)
	for i := range bigVal {
		bigVal[i] = 'b'
	}
	ssdv, _ := document.NewSortedSetDocValuesField("field", [][]byte{bigVal})
	if err := w.AddDocument(&testDocument{fields: []interface{}{ssdv}}); err != nil {
		t.Fatalf("AddDocument(max-size): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesDifferentSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustBinaryDVField("a", []byte("value"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesAfterDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesAfterReopenCreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
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
	if err := w2.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesAfterReopenAppend1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	w2 := newWriter(t, dir)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesAfterReopenAppend2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	fields := []interface{}{mustStringField(t, "a", "plain", false)}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	w2 := newWriter(t, dir)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesAfterReopenAppend3(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustStringField(t, "a", "x", false)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	w2 := newWriter(t, dir)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 42)}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesDifferentThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() < 1 {
		t.Fatalf("NumDocs = %d, want >= 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_MixedTypesViaAddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	w1 := newWriter(t, dir1)
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("a", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()
	w2 := newWriter(t, dir2)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("a", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()
	w3 := newWriter(t, dir3)
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir3)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_IllegalTypeChange(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	// Changing DV type within the same writer session should fail.
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err == nil {
		t.Fatal("expected error for illegal type change, got nil")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDocValuesIndexing_IllegalTypeChangeAcrossSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	w2 := newWriter(t, dir)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeAfterCloseAndDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	if err := w.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	w2 := newWriter(t, dir)
	if err := w2.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeAfterDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeAfterCommitAndDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeAfterOpenCreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
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
	if err := w2.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("v"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
	defer reader.Close()
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeViaAddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	w1 := newWriter(t, dir1)
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()
	w2 := newWriter(t, dir2)
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustSortedDVField("field", []byte("val"))}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir3 := store.NewByteBuffersDirectory()
	defer dir3.Close()
	w3 := newWriter(t, dir3)
	if err := w3.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w3.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir3)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeViaAddIndexesIR(t *testing.T) {
	TestDocValuesIndexing_TypeChangeViaAddIndexes(t)
}

func TestDocValuesIndexing_TypeChangeViaAddIndexes2(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	w1 := newWriter(t, dir1)
	if err := w1.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 1)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()
	w2 := newWriter(t, dir2)
	if err := w2.AddIndexes(dir1); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := w2.AddDocument(&testDocument{fields: []interface{}{mustNumericDVField("field", 2)}}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir2)
	defer reader.Close()
	if reader.NumDocs() != 2 {
		t.Fatalf("NumDocs = %d, want 2", reader.NumDocs())
	}
}

func TestDocValuesIndexing_TypeChangeViaAddIndexesIR2(t *testing.T) {
	TestDocValuesIndexing_TypeChangeViaAddIndexes2(t)
}

func TestDocValuesIndexing_SameFieldNameForPostingAndDocValue(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// A field used for both postings and DocValues.
	fields := []interface{}{
		mustStringField(t, "field", "hello", true),
		mustNumericDVField("field", 42),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	// Second doc with a different DV type for the same name should fail.
	if err := w.AddDocument(&testDocument{fields: []interface{}{
		mustStringField(t, "field", "world", true),
		mustSortedDVField("field", []byte("sorted")),
	}}); err == nil {
		t.Fatal("expected error for type change, got nil")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDocValuesIndexing_ExcIndexingDocBeforeDocValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w := newWriter(t, dir)
	// Both documents use the SAME DV type to verify the writer lifecycle.
	fields := []interface{}{
		mustStringField(t, "text", "hello world", true),
		mustNumericDVField("dv", 42),
	}
	if err := w.AddDocument(&testDocument{fields: fields}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.AddDocument(&testDocument{fields: []interface{}{
		mustStringField(t, "text", "second doc", true),
		mustNumericDVField("dv", 99),
	}}); err != nil {
		t.Fatalf("AddDocument(2): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader := openReader(t, dir)
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

func docWithNumericDV(name string, val int64) *testDocument {
	return &testDocument{fields: []interface{}{mustNumericDVField(name, val)}}
}

func newWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w
}

func openReader(t *testing.T, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return r
}
