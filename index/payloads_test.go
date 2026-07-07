// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for payload encoding and retrieval.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestPayloads.java
//
// GOC-4250: Port test `org.apache.lucene.index.TestPayloads`.
package index_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// payloadType is a FieldType that indexes positions and payloads.
func payloadType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	return ft
}

// payloadAnalyzer returns a test-only analyzer that ignores the supplied reader
// and returns a fresh TokenStream built by factory on every call.
type payloadAnalyzer struct {
	factory func() analysis.TokenStream
}

func (a *payloadAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.factory(), nil
}
func (a *payloadAnalyzer) Close() error { return nil }

// fieldAwarePayloadAnalyzer is an analyzer whose TokenStream factory receives
// the field name so it can emit payloads only for selected fields.
type fieldAwarePayloadAnalyzer struct {
	factory func(fieldName string) analysis.TokenStream
}

func (a *fieldAwarePayloadAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.factory(fieldName), nil
}
func (a *fieldAwarePayloadAnalyzer) Close() error { return nil }

// newPayloadWriter creates an IndexWriter configured with the supplied
// canned-token analyzer factory.
func newPayloadWriter(t *testing.T, factory func() analysis.TokenStream) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(&payloadAnalyzer{factory: factory})
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return dir, writer
}

// openCommittedReader flushes the writer and opens a fresh DirectoryReader.
func openCommittedReaderPayloads(t *testing.T, dir store.Directory, writer *index.IndexWriter) *index.DirectoryReader {
	t.Helper()
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return reader
}

// assertPayloads verifies the postings for term carry the expected payloads in
// order.
func assertPayloads(t *testing.T, reader *index.DirectoryReader, field, term string, wantPayloads [][]byte) {
	t.Helper()
	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0]
	terms, err := leaf.Terms(field)
	if err != nil {
		t.Fatalf("Terms(%q): %v", field, err)
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	var postings schema.PostingsEnum
	for {
		tt, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if tt == nil {
			break
		}
		if tt.Text() == term {
			postings, err = it.Postings(schema.PostingsFlagPayloads)
			if err != nil {
				t.Fatalf("Postings: %v", err)
			}
			break
		}
	}
	if postings == nil {
		t.Fatalf("no postings for term %q in field %q", term, field)
	}
	docID, err := postings.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID == schema.NO_MORE_DOCS {
		t.Fatalf("no docs for term %q", term)
	}
	freq, err := postings.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	if freq != len(wantPayloads) {
		t.Fatalf("freq = %d, want %d", freq, len(wantPayloads))
	}
	for i := 0; i < freq; i++ {
		if _, err := postings.NextPosition(); err != nil {
			t.Fatalf("NextPosition: %v", err)
		}
		got, err := postings.GetPayload()
		if err != nil {
			t.Fatalf("GetPayload: %v", err)
		}
		if !bytes.Equal(got, wantPayloads[i]) {
			t.Errorf("payload #%d: got %v, want %v", i, got, wantPayloads[i])
		}
	}
}

// TestPayloads_Payload ports testPayload().
//
// Java constructs a BytesRef from a string, checks its length, clones it,
// and asserts byte-for-byte equality between the original and the clone.
func TestPayloads_Payload(t *testing.T) {
	payload := util.NewBytesRef([]byte("This is a test!"))

	if payload.Length != len("This is a test!") {
		t.Errorf("wrong payload length: want %d, got %d", len("This is a test!"), payload.Length)
	}

	clone := payload.Clone()
	if clone.Length != payload.Length {
		t.Errorf("clone length mismatch: want %d, got %d", payload.Length, clone.Length)
	}
	for i := 0; i < payload.Length; i++ {
		if clone.Bytes[clone.Offset+i] != payload.Bytes[payload.Offset+i] {
			t.Errorf("byte mismatch at index %d: want %d, got %d",
				i, payload.Bytes[payload.Offset+i], clone.Bytes[clone.Offset+i])
		}
	}
}

// TestPayloads_FieldBit ports testPayloadFieldBit().
//
// Java writes documents with a payload-bearing field and a payload-free field,
// then uses getOnlyLeafReader(DirectoryReader.open(dir)) to verify that
// FieldInfo.hasPayloads() reflects whether any payload was stored.
func TestPayloads_FieldBit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Analyzer emits a payload only for field f1; f2 sees a plain token.
	config := index.NewIndexWriterConfig(&fieldAwarePayloadAnalyzer{factory: func(fieldName string) analysis.TokenStream {
		if fieldName == "f1" {
			return testutil.NewCannedTokenStream(
				testutil.NewToken("a", 0, 1).WithPayload([]byte{0x01}),
			)
		}
		return testutil.NewCannedTokenStream(
			testutil.NewToken("a", 0, 1),
		)
	}})
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()
	f1, _ := document.NewField("f1", "x", payloadType())
	doc.Add(f1)
	f2, _ := document.NewField("f2", "y", document.TextFieldTypeNotStored)
	doc.Add(f2)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	reader := openCommittedReaderPayloads(t, dir, writer)
	defer reader.Close()

	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	fi := leaves[0].GetFieldInfos().GetByName("f1")
	if fi == nil {
		t.Fatalf("field f1 not found")
	}
	if !fi.HasPayloads() {
		t.Errorf("f1.HasPayloads() = false, want true")
	}
	fi2 := leaves[0].GetFieldInfos().GetByName("f2")
	if fi2 == nil {
		t.Fatalf("field f2 not found")
	}
	if fi2.HasPayloads() {
		t.Errorf("f2.HasPayloads() = true, want false")
	}
}

// TestPayloads_Encoding ports testPayloadsEncoding().
//
// Java builds indexes with custom PayloadAnalyzer, writes varying payload bytes
// per position, then reads back PostingsEnum with PAYLOADS flag and validates
// each payload byte sequence.
func TestPayloads_Encoding(t *testing.T) {
	t.Fatal("blocked: PayloadAnalyzer/MockTokenizer (test module) and deterministic " +
		"per-position payload encoding fixture not yet ported")
}

// TestPayloads_ThreadSafety ports testThreadSafety().
//
// Java creates a multi-threaded Analyzer that emits PayloadAttribute tokens
// from N concurrent threads, each indexing into a shared DirectoryReader,
// then asserts that all payloads survive round-trip via PostingsEnum.
func TestPayloads_ThreadSafety(t *testing.T) {
	t.Fatal("blocked: custom per-thread PayloadAnalyzer and multi-threaded indexing " +
		"fixture not yet ported")
}

// TestPayloads_AcrossFields ports testAcrossFields().
//
// Java writes payloads under different field names using a custom Analyzer,
// then validates via PostingsEnum that each field's payloads are distinct.
func TestPayloads_AcrossFields(t *testing.T) {
	dir, writer := newPayloadWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewToken("a", 0, 1).WithPayload([]byte{0x10}),
		)
	})
	defer writer.Close()
	defer dir.Close()

	doc := document.NewDocument()
	f1, _ := document.NewField("f1", "x", payloadType())
	doc.Add(f1)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument f1: %v", err)
	}

	doc2 := document.NewDocument()
	f2, _ := document.NewField("f2", "x", payloadType())
	doc2.Add(f2)
	if err := writer.AddDocument(doc2); err != nil {
		t.Fatalf("AddDocument f2: %v", err)
	}

	reader := openCommittedReaderPayloads(t, dir, writer)
	defer reader.Close()

	assertPayloads(t, reader, "f1", "a", [][]byte{{0x10}})
	assertPayloads(t, reader, "f2", "a", [][]byte{{0x10}})
}

// TestPayloads_MixupDocs ports testMixupDocs().
//
// Java uses CannedTokenStream to emit tokens with payloads on specific
// positions, then verifies that PostingsEnum delivers them in the correct
// docID / position / payload order.
func TestPayloads_MixupDocs(t *testing.T) {
	dir, writer := newPayloadWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewTokenWithPosInc("a", 1, 0, 1).WithPayload([]byte{0x01}),
			testutil.NewTokenWithPosInc("a", 1, 0, 1).WithPayload([]byte{0x02}),
		)
	})
	defer writer.Close()
	defer dir.Close()

	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		field, _ := document.NewField("f", "x", payloadType())
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument #%d: %v", i, err)
		}
	}

	reader := openCommittedReaderPayloads(t, dir, writer)
	defer reader.Close()

	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0]
	terms, err := leaf.Terms("f")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	it.Next()
	postings, err := it.Postings(schema.PostingsFlagPayloads)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	docID := -1
	for {
		d, err := postings.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == schema.NO_MORE_DOCS {
			break
		}
		docID = d
		freq, err := postings.Freq()
		if err != nil {
			t.Fatalf("Freq: %v", err)
		}
		if freq != 2 {
			t.Errorf("doc %d freq = %d, want 2", docID, freq)
		}
		for j := 0; j < freq; j++ {
			pos, err := postings.NextPosition()
			if err != nil {
				t.Fatalf("NextPosition: %v", err)
			}
			if pos != j {
				t.Errorf("doc %d pos #%d = %d, want %d", docID, j, pos, j)
			}
			payload, err := postings.GetPayload()
			if err != nil {
				t.Fatalf("GetPayload: %v", err)
			}
			want := []byte{0x01 + byte(j)}
			if !bytes.Equal(payload, want) {
				t.Errorf("doc %d payload #%d = %v, want %v", docID, j, payload, want)
			}
		}
	}
	if docID != 2 {
		t.Errorf("last docID = %d, want 2", docID)
	}
}

// TestPayloads_MixupMultiValued ports testMixupMultiValued().
//
// Java uses CannedTokenStream on multi-valued fields, verifies payloads
// survive across field instances via PostingsEnum.
func TestPayloads_MixupMultiValued(t *testing.T) {
	dir, writer := newPayloadWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewTokenWithPosInc("a", 1, 0, 1).WithPayload([]byte{0x01}),
			testutil.NewTokenWithPosInc("a", 1, 0, 1).WithPayload([]byte{0x02}),
		)
	})
	defer writer.Close()
	defer dir.Close()

	doc := document.NewDocument()
	f1, _ := document.NewField("f", "x", payloadType())
	doc.Add(f1)
	f2, _ := document.NewField("f", "x", payloadType())
	doc.Add(f2)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	reader := openCommittedReaderPayloads(t, dir, writer)
	defer reader.Close()

	assertPayloads(t, reader, "f", "a", [][]byte{{0x01}, {0x02}, {0x01}, {0x02}})
}
