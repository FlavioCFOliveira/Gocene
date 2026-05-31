// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// End-to-end coverage for IndexWriter.ForceMerge wired to the real
// SegmentMerger (rmp #14/#114): force-merging multiple committed segments
// reduces the segment count to one AND preserves every document's searchable
// content and stored fields, with deleted documents compacted out.

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

func fmAddDoc(t *testing.T, w *index.IndexWriter, id, body string) {
	t.Helper()
	doc := document.NewDocument()
	idF, err := document.NewStringField("id", id, true)
	if err != nil {
		t.Fatalf("id field: %v", err)
	}
	doc.Add(idF)
	bF, err := document.NewTextField("body", body, true)
	if err != nil {
		t.Fatalf("body field: %v", err)
	}
	doc.Add(bF)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

func fmHits(t *testing.T, s *search.IndexSearcher, term string) int {
	t.Helper()
	top, err := s.Search(search.NewTermQuery(index.NewTerm("body", term)), 100)
	if err != nil {
		t.Fatalf("Search %q: %v", term, err)
	}
	return int(top.TotalHits.Value)
}

func TestForceMerge_RealMergePreservesContent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Three committed segments.
	fmAddDoc(t, w, "1", "alpha beta")
	fmAddDoc(t, w, "2", "beta gamma")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit 1: %v", err)
	}
	fmAddDoc(t, w, "3", "alpha gamma")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	fmAddDoc(t, w, "4", "delta")
	fmAddDoc(t, w, "5", "alpha delta")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit 3: %v", err)
	}

	if c := w.GetSegmentCount(); c < 2 {
		t.Fatalf("expected multiple segments before merge, got %d", c)
	}

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if c := w.GetSegmentCount(); c != 1 {
		t.Fatalf("segment count after ForceMerge = %d, want 1", c)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := len(reader.GetSegmentReaders()); got != 1 {
		t.Fatalf("reader segment count = %d, want 1", got)
	}
	if got := reader.NumDocs(); got != 5 {
		t.Fatalf("NumDocs after merge = %d, want 5", got)
	}

	s := search.NewIndexSearcher(reader)
	if got := fmHits(t, s, "alpha"); got != 3 { // docs 1,3,5
		t.Errorf("alpha hits = %d, want 3", got)
	}
	if got := fmHits(t, s, "beta"); got != 2 { // docs 1,2
		t.Errorf("beta hits = %d, want 2", got)
	}
	if got := fmHits(t, s, "delta"); got != 2 { // docs 4,5
		t.Errorf("delta hits = %d, want 2", got)
	}
	// id:1 still resolvable as a postings term in the merged segment.
	top, err := s.Search(search.NewTermQuery(index.NewTerm("id", "1")), 10)
	if err != nil {
		t.Fatalf("Search id:1: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("id:1 hits = %d, want 1", top.TotalHits.Value)
	}
}

func TestForceMerge_CompactsDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	fmAddDoc(t, w, "1", "alpha")
	fmAddDoc(t, w, "2", "beta")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit 1: %v", err)
	}
	fmAddDoc(t, w, "3", "alpha")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	// Delete id:2 then force-merge: the merged segment must compact it out.
	if err := w.DeleteDocuments(index.NewTerm("id", "2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if c := w.GetSegmentCount(); c != 1 {
		t.Fatalf("segment count = %d, want 1", c)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 2 {
		t.Fatalf("NumDocs after merge = %d, want 2", got)
	}
	if got := reader.MaxDoc(); got != 2 {
		t.Fatalf("MaxDoc after merge = %d, want 2 (deletes compacted)", got)
	}
	if got := reader.NumDeletedDocs(); got != 0 {
		t.Fatalf("NumDeletedDocs after merge = %d, want 0", got)
	}
	s := search.NewIndexSearcher(reader)
	if got := fmHits(t, s, "alpha"); got != 2 { // ids 1 and 3
		t.Errorf("alpha hits = %d, want 2", got)
	}
	if got := fmHits(t, s, "beta"); got != 0 { // id 2 was deleted
		t.Errorf("beta hits = %d, want 0", got)
	}
}
