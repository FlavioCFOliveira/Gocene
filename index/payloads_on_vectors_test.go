// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestPayloadsOnVectors.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// payloadsOnVectorsType renders new FieldType(TextField.TYPE_NOT_STORED) with
// term vectors, positions and payloads, and random offsets.
func payloadsOnVectorsType(positions bool) *document.FieldType {
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(positions)
	customType.SetStoreTermVectorPayloads(true)
	customType.SetStoreTermVectorOffsets(rand.Intn(2) == 0)
	return customType
}

// payloadsOnVectorsTokenizer renders
// new MockTokenizer(MockTokenizer.WHITESPACE, true) + setReader(new StringReader(text)).
func payloadsOnVectorsTokenizer(text string) analysis.TokenStream {
	ts := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
	ts.SetReader(strings.NewReader(text))
	return ts
}

func payloadsOnVectorsWithPayload(t *testing.T) analysis.TokenStream {
	t.Helper()
	withPayload := testanalysis.NewToken("withPayload", 0, 11).WithPayload([]byte("test"))
	ts := testanalysis.NewCannedTokenStream(withPayload)
	if !ts.HasAttribute(analysis.PayloadAttributeType) {
		t.Fatal("assertTrue(ts.hasAttribute(PayloadAttribute.class))")
	}
	return ts
}

// TestPayloadsOnVectorsMixupDocs: some docs have payload att, some not. The
// Java test swaps the token stream of the same Field with
// Field#setTokenStream(TokenStream) between documents.
func TestPayloadsOnVectorsMixupDocs(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	field, err := document.NewField("field", payloadsOnVectorsTokenizer("here we go"), payloadsOnVectorsType(true))
	if err != nil {
		t.Fatalf("new Field: %v", err)
	}
	doc.Add(field)
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	payloadsOnVectorsWithPayload(t)
	mustClose(t, writer, dir)
	t.Fatal("org.apache.lucene.document.Field#setTokenStream(TokenStream) is not ported")
}

// TestPayloadsOnVectorsMixupMultiValued: some field instances have payload
// att, some not.
func TestPayloadsOnVectorsMixupMultiValued(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	customType := payloadsOnVectorsType(true)
	for _, ts := range []analysis.TokenStream{
		payloadsOnVectorsTokenizer("here we go"),
		payloadsOnVectorsWithPayload(t),
		payloadsOnVectorsTokenizer("nopayload"),
	} {
		f, err := document.NewField("field", ts, customType)
		if err != nil {
			t.Fatalf("new Field: %v", err)
		}
		doc.Add(f)
	}
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	termVectors, err := reader.TermVectors()
	if err != nil {
		t.Fatalf("termVectors: %v", err)
	}
	terms, err := termVectors.GetField(0, "field")
	if err != nil {
		t.Fatalf("termVectors.get: %v", err)
	}
	if terms == nil {
		t.Fatal("assert terms != null")
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	found, err := termsEnum.SeekExact(index.NewTerm("field", "withPayload"))
	if err != nil || !found {
		t.Fatalf("seekExact(withPayload): %t (%v)", found, err)
	}
	de, err := termsEnum.Postings(index.PostingsFlagAll)
	if err != nil {
		t.Fatalf("postings: %v", err)
	}
	if doc, err := de.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("nextDoc: expected 0, got %d (%v)", doc, err)
	}
	if pos, err := de.NextPosition(); err != nil || pos != 3 {
		t.Fatalf("nextPosition: expected 3, got %d (%v)", pos, err)
	}
	if payload, err := de.GetPayload(); err != nil || !bytes.Equal(payload, []byte("test")) {
		t.Fatalf("getPayload: expected test, got %q (%v)", payload, err)
	}
	mustClose(t, writer, reader, dir)
}

func TestPayloadsOnVectorsPayloadsWithoutPositions(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	doc.Add(newField(t, "field", "foo", payloadsOnVectorsType(false)))

	if _, err := writer.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}

	mustClose(t, writer, dir)
}
