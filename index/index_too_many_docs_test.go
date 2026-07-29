// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexTooManyDocs ports the core assertion of
// org.apache.lucene.index.TestIndexTooManyDocs (Apache Lucene 10.4.0):
// once the lowered document cap is reached, AddDocument/UpdateDocument fail
// with an IllegalArgumentException whose message starts with the canonical
// "number of documents in the index cannot exceed " + getActualMaxDocs().
func TestIndexTooManyDocs(t *testing.T) {
	// Restore the default cap after the test, matching Lucene's finally block.
	orig := index.GetActualMaxDocs()
	defer index.SetMaxDocs(orig)

	const cap = 5
	index.SetMaxDocs(cap)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < cap; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("unexpected error adding document %d: %v", i, err)
		}
	}

	// The next add must hit the cap.
	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "x", true)
	doc.Add(idField)
	_, err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected AddDocument to fail once the document cap is reached")
	}
	wantPrefix := "number of documents in the index cannot exceed " + strconv.Itoa(cap)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("error message mismatch: got %q, want prefix %q", err.Error(), wantPrefix)
	}

	// UpdateDocument must also respect the cap.
	replacement := document.NewDocument()
	idField2, _ := document.NewStringField("id", "0", true)
	replacement.Add(idField2)
	_, err = writer.UpdateDocument(index.NewTerm("id", "0"), replacement)
	if err == nil {
		t.Fatal("expected UpdateDocument to fail once the document cap is reached")
	}
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("update error message mismatch: got %q, want prefix %q", err.Error(), wantPrefix)
	}
}

