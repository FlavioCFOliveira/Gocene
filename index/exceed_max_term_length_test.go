// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene's org.apache.lucene.index.TestExceedMaxTermLength
// (releases/lucene/10.4.0, commit 9983b7c).
//
// The upstream test verifies that IndexWriter throws an error mentioning
// "immense term", the field name, MAX_TERM_LENGTH, and "bytes can be at most"
// when a term longer than MAX_TERM_LENGTH is indexed, via both the token-stream
// and binary-value paths.
//
// Gocene gaps vs the Java reference:
//   - The token-stream inversion bridge is stubbed. The token-stream test
//     verifies that AddDocument returns the expected stub error.
//   - The binary-value path enforces MAX_TERM_LENGTH via a length check in
//     indexing_chain.invertTerm, matching Lucene's behaviour.
//   - Test utilities like MockAnalyzer and TestUtil.randomSimpleString are not
//     ported; hand-crafted values are used instead.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestExceedMaxTermLength_TokenStream verifies that the token-stream inversion
// path (which is stubbed) produces a proper error rather than silently dropping
// the document. The token-stream bridge is not yet ported, so all tokenized
// fields fail with a specific "not yet supported" error.
func TestExceedMaxTermLength_TokenStream(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Create a tokenized field (TextField). Tokenized fields flow through
	// invertTokenStream which is stubbed.
	doc := document.NewDocument()
	ft := document.NewFieldType()
	ft.SetIndexed(true)
	ft.SetTokenized(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()
	f, err := document.NewField("content", "a", ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	doc.Add(f)

	err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error: token-stream inversion not yet supported")
	}
	if !strings.Contains(err.Error(), "token-stream inversion not yet supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestExceedMaxTermLength_BinaryValue indexes a non-tokenized field whose value
// exceeds MAX_TERM_LENGTH and asserts the error message mentions "immense term",
// the max length, and the field name.
func TestExceedMaxTermLength_BinaryValue(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Create a value longer than MAX_TERM_LENGTH bytes.
	longVal := make([]byte, index.MAX_TERM_LENGTH+1)
	for i := range longVal {
		longVal[i] = 'x'
	}

	// StringField is non-tokenized and uses the binary indexing path.
	doc := document.NewDocument()
	sf, err := document.NewStringField("field", string(longVal), false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error indexing term exceeding MAX_TERM_LENGTH")
	}

	msg := err.Error()
	if !strings.Contains(msg, "immense term") {
		t.Errorf("error should mention 'immense term', got: %v", msg)
	}
	maxLenStr := "32766"
	if !strings.Contains(msg, maxLenStr) {
		t.Errorf("error should mention max length %s, got: %v", maxLenStr, msg)
	}
	if !strings.Contains(msg, "field") {
		t.Errorf("error should mention field name, got: %v", msg)
	}
}

// TestMaxTermLengthConstant verifies that the MAX_TERM_LENGTH constant exists
// and matches Lucene's BYTE_BLOCK_SIZE - 2.
func TestMaxTermLengthConstant(t *testing.T) {
	if index.MAX_TERM_LENGTH != 32766 {
		t.Errorf("MAX_TERM_LENGTH = %d, want 32766", index.MAX_TERM_LENGTH)
	}
}

// TestExceedMaxTermLength_NearBoundary verifies that a value exactly at
// MAX_TERM_LENGTH bytes is accepted while one byte beyond is rejected.
func TestExceedMaxTermLength_NearBoundary(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Value at exactly MAX_TERM_LENGTH should be accepted.
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	okVal := make([]byte, index.MAX_TERM_LENGTH)
	for i := range okVal {
		okVal[i] = 'a'
	}

	doc := document.NewDocument()
	sf, err := document.NewStringField("field", string(okVal), false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	if err := writer.AddDocument(doc); err != nil {
		t.Errorf("value at MAX_TERM_LENGTH should be accepted, got: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Value one byte beyond should be rejected.
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter 2: %v", err)
	}

	badVal := make([]byte, index.MAX_TERM_LENGTH+1)
	for i := range badVal {
		badVal[i] = 'b'
	}

	doc2 := document.NewDocument()
	sf2, err := document.NewStringField("field", string(badVal), false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc2.Add(sf2)

	err = writer2.AddDocument(doc2)
	if err == nil {
		t.Fatal("expected error for value exceeding MAX_TERM_LENGTH")
	}
	if !strings.Contains(err.Error(), "immense term") {
		t.Errorf("expected 'immense term' in error, got: %v", err)
	}

	if err := writer2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
