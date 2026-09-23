// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMaxPosition.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// maxPositionTextField renders new TextField("foo", new CannedTokenStream(tokens)).
func maxPositionTextField(t *testing.T, tokens ...testanalysis.Token) *document.Field {
	t.Helper()
	f, err := document.NewField("foo", testanalysis.NewCannedTokenStream(tokens...), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	return f
}

func TestMaxPositionTooBigPosition(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(nil))
	doc := document.NewDocument()
	// This is at position 1:
	t1 := testanalysis.NewToken("foo", 0, 3)
	t1 = testanalysis.NewTokenWithPosInc(t1.Text, 2, t1.StartOffset, t1.EndOffset)
	if rand.Intn(2) == 0 {
		t1.Payload = []byte{0x1}
	}
	// This should overflow max:
	t2 := testanalysis.NewTokenWithPosInc("foo", index.MaxPosition, 4, 7)
	if rand.Intn(2) == 0 {
		t2.Payload = []byte{0x1}
	}
	doc.Add(maxPositionTextField(t, t1, t2))
	if _, err := iw.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}

	// Document should not be visible:
	r, err := index.OpenDirectoryReaderFromWriter(iw)
	if err != nil {
		t.Fatalf("DirectoryReader.open(iw): %v", err)
	}
	if r.NumDocs() != 0 {
		t.Fatalf("expected 0 docs, got %d", r.NumDocs())
	}
	mustClose(t, r, iw, dir)
}

func TestMaxPositionMaxPosition(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(nil))
	doc := document.NewDocument()
	// This is at position 0:
	t1 := testanalysis.NewToken("foo", 0, 3)
	if rand.Intn(2) == 0 {
		t1.Payload = []byte{0x1}
	}
	t2 := testanalysis.NewTokenWithPosInc("foo", index.MaxPosition, 4, 7)
	if rand.Intn(2) == 0 {
		t2.Payload = []byte{0x1}
	}
	doc.Add(maxPositionTextField(t, t1, t2))
	mustAddDocument(t, iw, doc)

	// Document should be visible:
	r, err := index.OpenDirectoryReaderFromWriter(iw)
	if err != nil {
		t.Fatalf("DirectoryReader.open(iw): %v", err)
	}
	if r.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", r.NumDocs())
	}
	mustClose(t, r, iw, dir)
	// PostingsEnum postings = MultiTerms.getTermPostingsEnum(r, "foo", new BytesRef("foo"))
	// then asserts docID 0, freq 2, positions 0 and MAX_POSITION.
	t.Fatal("org.apache.lucene.index.MultiTerms#getTermPostingsEnum(IndexReader, String, BytesRef) is not ported")
}
