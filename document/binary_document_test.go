// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GOC-3983: Port of org.apache.lucene.document.TestBinaryDocument
//
// Mirrors the Java fixture: a binary StoredField alongside a stored
// String field, written via IndexWriter, then read back via
// DirectoryReader.StoredFields().Document(0, visitor), with both values
// compared against the originals.
//
// This is an exhaustive roundtrip — if any layer of the
// document/index/codec/store stack regresses on stored-field round-tripping
// for binary values, this test will surface the gap rather than mask it.

const (
	binaryDocValStored = "this text will be stored as a byte array in the index"
)

// TestBinaryFieldInIndex mirrors Java testBinaryFieldInIndex.
//
// Verifies:
//  1. A binary StoredField and a stored String field can coexist in a Document.
//  2. The document round-trips through IndexWriter and DirectoryReader.
//  3. The reconstructed Document yields the original binary bytes via
//     GetBinaryValue and the original string via GetString.
func TestBinaryFieldInIndex(t *testing.T) {
	binaryFldStored, err := document.NewStoredFieldFromBytes(
		"binaryStored", []byte(binaryDocValStored))
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes: %v", err)
	}

	ft := document.NewFieldType().SetStored(true)
	ft.Freeze()
	stringFldStored, err := document.NewField(
		"stringStored", binaryDocValStored, ft)
	if err != nil {
		t.Fatalf("NewField(stringStored): %v", err)
	}

	doc := document.NewDocument()
	doc.Add(binaryFldStored)
	doc.Add(stringFldStored)

	// Field count assertion mirrors Java assertEquals(2, doc.getFields().size()).
	if got := doc.Size(); got != 2 {
		t.Fatalf("doc.Size() = %d, want 2", got)
	}

	// Build a single-document index using the established roundtrip path
	// already exercised by document_indexing_roundtrip_test.go.
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("writer.Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != 1 {
		t.Fatalf("reader.NumDocs() = %d, want 1", got)
	}

	storedFields, err := reader.StoredFields()
	if err != nil {
		// Real gap — surface it but do not crash subsequent suites.
		t.Fatalf("reader.StoredFields() unavailable: %v "+
			"(blocks GOC-3983 stored-value assertions; document the gap)", err)
		return
	}

	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("StoredFields.Document(0, visitor) failed: %v "+
			"(blocks GOC-3983 stored-value assertions; document the gap)", err)
		return
	}

	docFromReader := visitor.GetDocument()
	if docFromReader == nil {
		t.Fatal("visitor.GetDocument() returned nil")
	}

	// Binary roundtrip assertion (Java: BytesRef + new String(bytes,offset,length,UTF_8)).
	bytes := docFromReader.GetBinaryValue("binaryStored")
	if bytes == nil {
		t.Fatal("docFromReader.GetBinaryValue(\"binaryStored\") returned nil")
	}
	if got := string(bytes); got != binaryDocValStored {
		t.Errorf("binary roundtrip mismatch:\n got  %q\n want %q",
			got, binaryDocValStored)
	}

	// Stored-string roundtrip assertion (Java: doc.get("stringStored")).
	if got := docFromReader.GetString("stringStored"); got != binaryDocValStored {
		t.Errorf("string roundtrip mismatch:\n got  %q\n want %q",
			got, binaryDocValStored)
	}
}

// TestBinaryFieldFromDataInputInIndex mirrors Java
// testBinaryFieldFromDataInputInIndex.
//
// Deferred: StoredValueTypeDataInput is a placeholder in stored_value.go:30,
// awaiting the port of StoredFieldDataInput (Java
// org.apache.lucene.index.StoredFieldDataInput). The skeleton below documents
// the intended fixture so the test can be enabled by deleting the t.Skip
// once the port lands.
func TestBinaryFieldFromDataInputInIndex(t *testing.T) {
	byteArray := []byte(binaryDocValStored)
	badi := store.NewByteArrayDataInput(byteArray)
	sfdi := index.NewStoredFieldDataInputFromByteArray(badi)
	binaryFldStored, err := document.NewStoredFieldFromDataInput("binaryStored", sfdi)
	if err != nil {
		t.Fatalf("NewStoredFieldFromDataInput: %v", err)
	}

	doc := document.NewDocument()
	doc.Add(binaryFldStored)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(0, visitor); err != nil {
		t.Fatalf("Document: %v", err)
	}
	docFromReader := visitor.GetDocument()
	bytes := docFromReader.GetBinaryValue("binaryStored")
	if bytes == nil {
		t.Fatal("GetBinaryValue returned nil")
	}
	if got := string(bytes); got != binaryDocValStored {
		t.Errorf("got %q want %q", got, binaryDocValStored)
	}
}
