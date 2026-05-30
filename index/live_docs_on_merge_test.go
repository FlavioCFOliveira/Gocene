// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// readSegmentsRaw returns the raw on-disk bytes of the current segments_N file.
// It fails the test if no segments_N file is present. Reading the raw bytes
// (rather than ReadSegmentInfos, which strips _gocene_* keys on parse) is the
// only way to prove that a given userData key was never written to disk.
func readSegmentsRaw(t *testing.T, dir store.Directory) []byte {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	var name string
	for _, f := range files {
		if strings.HasPrefix(f, "segments_") {
			name = f // last one wins; there is normally exactly one.
		}
	}
	if name == "" {
		t.Fatal("no segments_N file found")
	}
	in, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", name, err)
	}
	defer in.Close()
	n := in.Length()
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes(%s): %v", name, err)
	}
	return buf
}

// countLivFiles returns the number of .liv files present in the directory.
func countLivFiles(t *testing.T, dir store.Directory) int {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	count := 0
	for _, f := range files {
		if strings.HasSuffix(f, ".liv") {
			count++
		}
	}
	return count
}

// TestLiveDocsOnMerge_DeletesCarriedThroughCommit is acceptance criterion (1) of
// rmp #4789: a same-session delete that targets buffered documents is carried
// into the flushed segment as a real Lucene90 .liv file (not a _gocene_del_
// userData key), so a reopened reader's NumDocs reflects the deletion and the
// segments_N file carries no _gocene_del_ marker.
func TestLiveDocsOnMerge_DeletesCarriedThroughCommit(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(createTestAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add five documents with a distinct id, then delete one of them within the
	// same session (before any commit). The delete resolves against buffered
	// documents and is recorded as a deleted ordinal on the pending segment.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		idField, ferr := document.NewStringField("id", string(rune('a'+i)), true)
		if ferr != nil {
			t.Fatalf("NewStringField: %v", ferr)
		}
		doc.Add(idField)
		bodyField, ferr := document.NewTextField("body", "the quick brown fox", true)
		if ferr != nil {
			t.Fatalf("NewTextField: %v", ferr)
		}
		doc.Add(bodyField)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := w.DeleteDocuments(index.NewTerm("id", "c")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A real .liv file must have been written for the segment that inherited the
	// deletion (previously the deletion lived only in a _gocene_del_ userData key).
	if got := countLivFiles(t, dir); got != 1 {
		t.Errorf("expected exactly 1 .liv file on disk, got %d", got)
	}

	// The raw segments_N bytes must contain no _gocene_del_ marker (nor any
	// _gocene_ key at all once the segment carries no parentField / index sort).
	raw := readSegmentsRaw(t, dir)
	if bytes.Contains(raw, []byte("_gocene_del_")) {
		t.Error("segments_N still contains a _gocene_del_ key")
	}
	if bytes.Contains(raw, []byte("_gocene_")) {
		t.Error("segments_N still contains a _gocene_ key")
	}

	// Reopen: NumDocs must reflect the deletion (5 added - 1 deleted = 4), proving
	// the .liv was read back via loadLiveDocsFromDisk.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 4 {
		t.Errorf("NumDocs after reopen: want 4, got %d", got)
	}
	if got := r.MaxDoc(); got != 5 {
		t.Errorf("MaxDoc after reopen: want 5, got %d", got)
	}
}

// TestLiveDocsOnMerge_ForceMergeCarriesDeletes verifies that deletions survive a
// ForceMerge(1): the writer compacts/merges segments and the resulting index
// still reports the post-delete NumDocs after reopen, with no _gocene_del_ key.
func TestLiveDocsOnMerge_ForceMergeCarriesDeletes(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(createTestAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 6; i++ {
		doc := document.NewDocument()
		idField, ferr := document.NewStringField("id", string(rune('a'+i)), true)
		if ferr != nil {
			t.Fatalf("NewStringField: %v", ferr)
		}
		doc.Add(idField)
		bodyField, ferr := document.NewTextField("body", "lorem ipsum dolor", true)
		if ferr != nil {
			t.Fatalf("NewTextField: %v", ferr)
		}
		doc.Add(bodyField)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	// Delete two buffered docs, then ForceMerge to a single segment.
	if err := w.DeleteDocuments(index.NewTerm("id", "b")); err != nil {
		t.Fatalf("DeleteDocuments b: %v", err)
	}
	if err := w.DeleteDocuments(index.NewTerm("id", "e")); err != nil {
		t.Fatalf("DeleteDocuments e: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw := readSegmentsRaw(t, dir)
	if bytes.Contains(raw, []byte("_gocene_del_")) {
		t.Error("segments_N still contains a _gocene_del_ key after ForceMerge")
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 4 {
		t.Errorf("NumDocs after ForceMerge+reopen: want 4, got %d", got)
	}
}
