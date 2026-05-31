// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Coverage for NRTReader.Refresh against a live IndexWriter (rmp #15):
// refreshing after adding documents WITHOUT an explicit commit makes the new
// documents visible and searchable.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestNRTReader_RefreshSeesUncommittedDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	add := func(id, body string) {
		doc := document.NewDocument()
		idF, _ := document.NewStringField("id", id, true)
		doc.Add(idF)
		bF, _ := document.NewTextField("body", body, true)
		doc.Add(bF)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	add("1", "alpha")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	base, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	nrt, err := index.NewNRTReader(base, w)
	if err != nil {
		t.Fatalf("NewNRTReader: %v", err)
	}
	defer nrt.Close()

	if got := nrt.NumDocs(); got != 1 {
		t.Fatalf("initial NumDocs = %d, want 1", got)
	}

	// Add two more documents WITHOUT committing.
	add("2", "beta")
	add("3", "alpha")

	if err := nrt.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := nrt.NumDocs(); got != 3 {
		t.Fatalf("NumDocs after refresh = %d, want 3 (uncommitted docs must be visible)", got)
	}

	// The newly added (uncommitted) documents are searchable through the
	// refreshed reader.
	s := search.NewIndexSearcher(nrt.GetReader())
	top, err := s.Search(search.NewTermQuery(index.NewTerm("body", "alpha")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 2 { // docs 1 and 3
		t.Errorf("alpha hits after refresh = %d, want 2", top.TotalHits.Value)
	}

	// A refresh with no further changes reports the reader unchanged (no error).
	if err := nrt.Refresh(); err != nil {
		t.Fatalf("idempotent Refresh: %v", err)
	}
	if got := nrt.NumDocs(); got != 3 {
		t.Fatalf("NumDocs after no-op refresh = %d, want 3", got)
	}
}
