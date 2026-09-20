// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Coverage for IndexWriter.DeleteDocumentsQuery against the real mechanism:
// FrozenBufferedUpdates.applyQueryDeletes evaluates each query per segment
// through the spi.QueryScorerSource that package search registers (the port of
// `new IndexSearcher(readerContext.reader())` in
// org.apache.lucene.index.FrozenBufferedUpdates#applyQueryDeletes). Query-based
// deletes are applied to committed segments and become visible after commit,
// and a Query that is not an org.apache.lucene.search.Query surfaces a clear
// error rather than being silently dropped.
//
// This file replaces the coverage of the removed QueryDeleteExecutor registry,
// whose producer no longer existed and whose design — one DirectoryReader over
// every committed segment, a global search, and a global-to-segment docID
// remap — is not Lucene's.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Registers the production codec and, through package search, the
	// spi.QueryScorerSource factory applyQueryDeletes constructs per segment.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func qdAddDoc(t *testing.T, w *index.IndexWriter, id string) {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewStringField("id", id, true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(f)
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// TestQueryDelete_AppliedAfterCommit verifies that a buffered query delete is
// applied to committed segments and visible after the next commit.
func TestQueryDelete_AppliedAfterCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, id := range []string{"aaa", "bbb", "aaa", "ccc", "aaa"} {
		qdAddDoc(t, w, id)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	query := search.NewTermQuery(index.NewTerm("id", "aaa"))
	if _, err := w.DeleteDocumentsQuery([]index.Query{query}); err != nil {
		t.Fatalf("DeleteDocumentsQuery: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit (post query-delete): %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 2 {
		t.Fatalf("NumDocs after query-delete = %d, want 2", got)
	}
	if got := reader.NumDeletedDocs(); got != 3 {
		t.Fatalf("NumDeletedDocs = %d, want 3", got)
	}
	s := search.NewIndexSearcher(reader)
	top, err := s.Search(search.NewTermQuery(index.NewTerm("id", "aaa")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Fatalf("id:aaa still matches %d docs after query-delete, want 0", top.TotalHits.Value)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// foreignQuery satisfies the shared spi.Query contract but is not an
// org.apache.lucene.search.Query, so it can be buffered as a delete but cannot
// be rewritten or scored.
type foreignQuery struct{}

func (foreignQuery) Equals(other spi.Query) bool { _, ok := other.(foreignQuery); return ok }
func (foreignQuery) HashCode() int               { return 0 }

// TestQueryDelete_UnsupportedTypeErrors verifies that a query which is not an
// org.apache.lucene.search.Query yields a clear error at commit instead of a
// silent no-op. Silently dropping it would under-delete.
func TestQueryDelete_UnsupportedTypeErrors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	qdAddDoc(t, w, "aaa")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if _, err := w.DeleteDocumentsQuery([]index.Query{foreignQuery{}}); err != nil {
		t.Fatalf("DeleteDocumentsQuery (buffering) should not error: %v", err)
	}
	err = w.Commit()
	if err == nil {
		t.Fatalf("Commit should fail for an unsupported query type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported query type") {
		t.Fatalf("error = %q, want it to mention 'unsupported query type'", err.Error())
	}
}
