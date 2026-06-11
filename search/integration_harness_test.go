// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared end-to-end integration harness for the search-package ports of the
// Apache Lucene 10.4.0 query test suites (rmp #18 / #123).
//
// These suites were previously gutted to t.Fatal("requires complete
// IndexWriter+IndexSearcher integration"); the integration now works
// (SegmentCoreReaders are wired during OpenDirectoryReader, rmp #4, and
// multi-segment IndexSearcher search is verified), so the suites are restored
// against this deterministic builder. The harness deliberately avoids the
// upstream RandomIndexWriter: it builds a committed on-disk index with the
// production codec so postings/doc-values are really flushed and read back
// through the same code path Lucene exercises.
package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so postings / doc-values are flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// integrationIndex accumulates documents and, on Searcher(), commits them to a
// real directory and returns an IndexSearcher over the committed index.
type integrationIndex struct {
	t   testing.TB
	dir store.Directory
	w   *index.IndexWriter
}

// newIntegrationIndex opens an in-memory directory and IndexWriter with a
// whitespace analyzer (the closest deterministic stand-in for the upstream
// MockAnalyzer) and the production codec.
func newIntegrationIndex(t testing.TB) *integrationIndex {
	t.Helper()
	return newIntegrationIndexWithDir(t, store.NewByteBuffersDirectory())
}

// newIntegrationIndexWithDir is newIntegrationIndex over a caller-supplied
// directory, letting suites that need a specific store backend (e.g. an
// MMapDirectory) exercise the same flush/read path.
func newIntegrationIndexWithDir(t testing.TB, dir store.Directory) *integrationIndex {
	t.Helper()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Disable auto-flush so that explicit Commit() calls are the sole
	// mechanism that flushes segments. The DocumentsWriter auto-flush
	// writes files using its own segment-name counter, which collides
	// with the SegmentInfos counter used by the Commit path when the
	// test exercises multi-segment indices via repeated ix.commit().
	config.SetMaxBufferedDocs(-1)
	config.SetRAMBufferSizeMB(-1)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return &integrationIndex{t: t, dir: dir, w: w}
}

// addString adds one document carrying a single indexed (not stored)
// StringField — the single-token field type used by exact-match query tests.
func (ix *integrationIndex) addString(field, value string) {
	ix.t.Helper()
	doc := document.NewDocument()
	f, err := document.NewStringField(field, value, false)
	if err != nil {
		ix.t.Fatalf("NewStringField(%q): %v", field, err)
	}
	doc.Add(f)
	if err := ix.w.AddDocument(doc); err != nil {
		ix.t.Fatalf("AddDocument: %v", err)
	}
}

// addText adds one document carrying a single tokenized (not stored) TextField.
func (ix *integrationIndex) addText(field, value string) {
	ix.t.Helper()
	doc := document.NewDocument()
	f, err := document.NewTextField(field, value, false)
	if err != nil {
		ix.t.Fatalf("NewTextField(%q): %v", field, err)
	}
	doc.Add(f)
	if err := ix.w.AddDocument(doc); err != nil {
		ix.t.Fatalf("AddDocument: %v", err)
	}
}

// addDoc adds a fully-formed document.
func (ix *integrationIndex) addDoc(doc *document.Document) {
	ix.t.Helper()
	if err := ix.w.AddDocument(doc); err != nil {
		ix.t.Fatalf("AddDocument: %v", err)
	}
}

// commit flushes the buffered documents into a new segment, letting callers
// build a multi-segment index that forces real cross-segment reads.
func (ix *integrationIndex) commit() {
	ix.t.Helper()
	if err := ix.w.Commit(); err != nil {
		ix.t.Fatalf("Commit: %v", err)
	}
}

// forceMerge merges the buffered/flushed segments down to maxNumSegments,
// letting callers force a single-segment index (the analogue of
// RandomIndexWriter.forceMerge in the upstream suites).
func (ix *integrationIndex) forceMerge(maxNumSegments int) {
	ix.t.Helper()
	if err := ix.w.ForceMerge(maxNumSegments); err != nil {
		ix.t.Fatalf("ForceMerge: %v", err)
	}
}

// searcher closes the writer, opens the committed index and returns an
// IndexSearcher plus a cleanup that closes the reader and directory.
func (ix *integrationIndex) searcher() (*search.IndexSearcher, func()) {
	ix.t.Helper()
	if err := ix.w.Close(); err != nil {
		ix.t.Fatalf("writer.Close: %v", err)
	}
	r, err := index.OpenDirectoryReader(ix.dir)
	if err != nil {
		ix.t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(r), func() {
		_ = r.Close()
		_ = ix.dir.Close()
	}

// assertHitCount runs the query and fails unless it matches want documents.
func assertHitCount(t testing.TB, s *search.IndexSearcher, q search.Query, want int64) {
	t.Helper()
	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != want {
		t.Errorf("matched documents = %d, want %d", top.TotalHits.Value, want)
	}
}