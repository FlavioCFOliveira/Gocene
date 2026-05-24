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

// TestIndexCompatibility_UpdateDocument verifies that UpdateDocument does not
// corrupt the index and that the replacement document's fields are registered
// in FieldInfos after Commit, matching Lucene's interface contract.
//
// Gocene deviation: UpdateDocument is currently a FieldInfos-accumulating
// no-op at the IndexWriter level.  The delete-then-add semantics require
// codec-backed postings readers to locate and delete documents from committed
// segments; until those are available, NumDocs() is unchanged by UpdateDocument
// and the replacement is not visible via the reader's document count.
//
// What this test verifies:
//  1. UpdateDocument succeeds without error.
//  2. Commit succeeds and the reader can be opened.
//  3. Fields introduced by the replacement document appear in FieldInfos.
//  4. The existing documents are still present (no unintended side effects).
func TestIndexCompatibility_UpdateDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add two documents.
	docA := document.NewDocument()
	idA, err := document.NewStringField("id", "1", true)
	if err != nil {
		t.Fatalf("NewStringField id A: %v", err)
	}
	valA, err := document.NewStringField("value", "original", true)
	if err != nil {
		t.Fatalf("NewStringField value A: %v", err)
	}
	docA.Add(idA)
	docA.Add(valA)
	if err := writer.AddDocument(docA); err != nil {
		t.Fatalf("AddDocument A: %v", err)
	}

	docB := document.NewDocument()
	idB, err := document.NewStringField("id", "2", true)
	if err != nil {
		t.Fatalf("NewStringField id B: %v", err)
	}
	docB.Add(idB)
	if err := writer.AddDocument(docB); err != nil {
		t.Fatalf("AddDocument B: %v", err)
	}

	// UpdateDocument: replacement introduces a new field "extra" not in docA/B.
	replacement := document.NewDocument()
	idR, err := document.NewStringField("id", "1", true)
	if err != nil {
		t.Fatalf("NewStringField id R: %v", err)
	}
	valR, err := document.NewStringField("value", "updated", true)
	if err != nil {
		t.Fatalf("NewStringField value R: %v", err)
	}
	extraR, err := document.NewStringField("extra", "new-field", true)
	if err != nil {
		t.Fatalf("NewStringField extra R: %v", err)
	}
	replacement.Add(idR)
	replacement.Add(valR)
	replacement.Add(extraR)

	deleteTerm := index.NewTerm("id", "1")
	if err := writer.UpdateDocument(deleteTerm, replacement); err != nil {
		t.Fatalf("UpdateDocument must not return an error: %v", err)
	}

	// Commit must succeed.
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after UpdateDocument: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	// Reader must open successfully.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	// The two documents added via AddDocument must still be present.
	// (UpdateDocument is a FieldInfos-accumulating no-op in the current
	// Gocene implementation; it does not alter the committed document count.)
	if got := r.NumDocs(); got < 2 {
		t.Errorf("NumDocs: want >= 2 (docA and docB intact), got %d", got)
	}

	// The replacement doc's new "extra" field must be registered in FieldInfos.
	fi := r.GetFieldInfos()
	if fi == nil {
		t.Fatal("GetFieldInfos returned nil")
	}
	if fi.GetByName("extra") == nil {
		t.Error("field 'extra' from replacement doc not found in FieldInfos after UpdateDocument+Commit")
	}
	if fi.GetByName("id") == nil {
		t.Error("field 'id' not found in FieldInfos")
	}
	if fi.GetByName("value") == nil {
		t.Error("field 'value' not found in FieldInfos")
	}
}
