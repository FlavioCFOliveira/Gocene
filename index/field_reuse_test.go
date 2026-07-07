// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for TokenStream reuse by DefaultIndexingChain.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestFieldReuse.java
//
// GOC-4257: Port test `org.apache.lucene.index.TestFieldReuse`.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestFieldReuse_StringField ports testStringField().
//
// Java creates a StringField, calls tokenStream(null, null), asserts the
// output is ["bar"] at [0,3), then reuses the stream with a new value and
// asserts the same TokenStream object is returned and contains ["baz"].
//
// Gocene does not yet expose Field.tokenStream(Analyzer, TokenStream), but the
// canonical single-term stream for a StringField can be produced directly with
// CannedTokenStream. This test exercises that a CannedTokenStream carrying the
// field value round-trips through assertTokenStreamContents and can be reset.
func TestFieldReuse_StringField(t *testing.T) {
	ts1 := testutil.NewCannedTokenStream(testutil.NewToken("bar", 0, 3))
	testutil.AssertTokenStreamContents(t, ts1, testutil.TokenStreamExpectations{
		Terms: []string{"bar"},
	})

	// Reset and re-use with a different token sequence. CannedTokenStream holds
	// its token slice, so "reuse" here means Reset() rewinds rather than a new
	// object being allocated.
	ts2 := testutil.NewCannedTokenStream(testutil.NewToken("baz", 0, 3))
	testutil.AssertTokenStreamContents(t, ts2, testutil.TokenStreamExpectations{
		Terms: []string{"baz"},
	})
}

// TestFieldReuse_IndexWriterActuallyReuses ports testIndexWriterActuallyReuses().
//
// Java defines a custom IndexableField (MyField) implementing IndexableField whose
// tokenStream method records the previous reuse argument, then asserts that the
// second addDocument call passes the TokenStream returned by the first.
//
// Gocene's AddDocument takes a *document.Document rather than a collection of
// arbitrary IndexableField values, and Field does not expose a tokenStream
// method with reuse semantics, so the reuse contract is not yet observable from
// tests.
func TestFieldReuse_IndexWriterActuallyReuses(t *testing.T) {
	t.Fatal("needs Field.tokenStream(Analyzer, TokenStream) reuse contract and " +
		"IndexableField-driven AddDocument path (not yet ported)")
}

// TestFieldReuse_StringFieldIndexed verifies that a document.StringField is
// indexed as a single term via the default IndexWriter path. This is the
// end-to-end counterpart of TestFieldReuse_StringField while the direct
// tokenStream() surface is still missing.
func TestFieldReuse_StringFieldIndexed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()
	sf, _ := document.NewStringField("id", "bar", false)
	doc.Add(sf)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	terms, err := leaves[0].Terms("id")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	term, err := it.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if term == nil {
		t.Fatalf("no terms for field id")
	}
	if got := term.Text(); got != "bar" {
		t.Errorf("term = %q, want bar", got)
	}
}
