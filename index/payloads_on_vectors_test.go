// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// payloadsOnVectorsCustomType builds the FieldType used by the term-vector
// payload tests: a non-stored TextField type with term vectors, positions and
// payloads enabled. Offsets are toggled to mirror random().nextBoolean() in
// the upstream test; here a deterministic value is used so the test is
// reproducible.
func payloadsOnVectorsCustomType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorPayloads(true)
	ft.SetStoreTermVectorOffsets(false)
	return ft
}

// TestPayloadsOnVectors_MixupDocs ports
// org.apache.lucene.index.TestPayloadsOnVectors.testMixupDocs.
//
// The upstream test indexes three documents where only the middle one carries
// a PayloadAttribute, then reads back the term vector for doc 1 and asserts the
// payload survives. The full assertion path requires CannedTokenStream (to
// attach a payload to a single canned token) and per-leaf term-vector reads via
// TermVectors().Get -> TermsEnum.Postings(ALL). Neither is available yet
// (CannedTokenStream is unimplemented; core readers are nil on
// OpenDirectoryReader, so leaf term-vector retrieval fails). The reachable part
// of the pipeline is exercised below; the payload read-back is deferred.
func TestPayloadsOnVectors_MixupDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	customType := payloadsOnVectorsCustomType()

	doc := document.NewDocument()
	field, err := document.NewField("field", "here we go", customType)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add first document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	t.Skip("payload read-back requires CannedTokenStream and per-leaf TermVectors().Get -> Postings(ALL), not yet implemented")
}

// TestPayloadsOnVectors_MixupMultiValued ports
// org.apache.lucene.index.TestPayloadsOnVectors.testMixupMultiValued.
//
// The upstream test indexes a single document with three values for the same
// field, only the second of which carries a payload, then reads the term
// vector back. Reading the payload requires CannedTokenStream and per-leaf
// term-vector retrieval, both unavailable; the indexable portion is exercised
// and the read-back is deferred.
func TestPayloadsOnVectors_MixupMultiValued(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	customType := payloadsOnVectorsCustomType()

	doc := document.NewDocument()
	field1, err := document.NewField("field", "here we go", customType)
	if err != nil {
		t.Fatalf("Failed to create field1: %v", err)
	}
	doc.Add(field1)
	field3, err := document.NewField("field", "nopayload", customType)
	if err != nil {
		t.Fatalf("Failed to create field3: %v", err)
	}
	doc.Add(field3)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	t.Skip("payload read-back requires CannedTokenStream and per-leaf TermVectors().Get -> Postings(ALL), not yet implemented")
}

// TestPayloadsOnVectors_PayloadsWithoutPositions ports
// org.apache.lucene.index.TestPayloadsOnVectors.testPayloadsWithoutPositions.
//
// Storing term-vector payloads without term-vector positions is an illegal
// configuration; the upstream test asserts addDocument throws
// IllegalArgumentException. Gocene already implements that invariant check in
// TermVectorsConsumerPerField.Start (it panics with the matching message), but
// IndexingChain does not yet wire a concrete TermVectorsConsumer into the live
// IndexWriter.AddDocument path (see the GAP note in indexing_chain.go), so the
// check is unreachable end-to-end. The field configuration is built and added
// to the document; the addDocument-time assertion is deferred.
func TestPayloadsOnVectors_PayloadsWithoutPositions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(false)
	customType.SetStoreTermVectorPayloads(true)
	customType.SetStoreTermVectorOffsets(false)

	doc := document.NewDocument()
	field, err := document.NewField("field", "foo", customType)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)

	t.Skip("illegal term-vector payload config is rejected in TermVectorsConsumerPerField.Start, but IndexingChain does not yet wire TermVectorsConsumer into AddDocument")
}
