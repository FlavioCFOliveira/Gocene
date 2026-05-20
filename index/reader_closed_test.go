// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// readerClosedAtLeast mirrors Lucene's atLeast(): a value no smaller than min.
// The package index_test cannot see the unexported atLeast in package index,
// so this test keeps a local copy.
func readerClosedAtLeast(min int) int {
	return min + rand.Intn(3)
}

// TestReaderClosed ports org.apache.lucene.index.TestReaderClosed#test.
//
// It builds a small single-segment index, runs a TermRangeQuery against a
// searcher, closes the underlying DirectoryReader, and asserts that a second
// search fails with an AlreadyClosedException.
//
// Two divergences from Lucene:
//   - Lucene drives indexing through RandomIndexWriter and reads via
//     IndexWriter.getReader (NRT). Gocene exposes neither, so this port uses
//     the plain IndexWriter and reopens the committed index with
//     OpenDirectoryReader, mirroring TestBinaryTerms.
//   - Lucene's testReaderChaining (LUCENE-3800) is omitted: it depends on
//     OwnCacheKeyMultiReader, which Gocene does not provide.
//
// Currently skipped: see the t.Skip below.
func TestReaderClosed(t *testing.T) {
	// Blocker 1 (infrastructure gap): OpenDirectoryReader materialises each
	// segment via NewSegmentReader (index/directory_reader.go:462/497), which
	// leaves SegmentReader.coreReaders nil; term-level searches then match no
	// documents. Same gap that skips TestBinaryTerms.
	//
	// Blocker 2 (semantic gap): DirectoryReader.Close
	// (index/directory_reader.go:597) only nils r.readers; it sets no
	// closed flag and does not cause GetSegmentReaders to fail. A search
	// after Close therefore returns 0 hits silently instead of raising
	// AlreadyClosedException, so the assertion below cannot hold.
	//
	// Unskip once OpenDirectoryReader wires SegmentCoreReaders and
	// DirectoryReader operations reject use after Close.
	t.Skip("blocked: OpenDirectoryReader builds SegmentReader without core readers, and DirectoryReader.Close does not make later use raise AlreadyClosedException (index/directory_reader.go:462/497/597)")

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Lucene writes random Unicode strings here; deterministic distinct
	// values exercise the TermRangeQuery the same way without randomness.
	num := readerClosedAtLeast(10)
	for i := 0; i < num; i++ {
		doc := document.NewDocument()
		field, err := document.NewStringField("field", fmt.Sprintf("term%d", i), false)
		if err != nil {
			t.Fatalf("Failed to create field for doc %d: %v", i, err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("Failed to force merge: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	if reader.GetRefCount() <= 0 {
		t.Fatalf("Expected positive refcount, got %d", reader.GetRefCount())
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermRangeQuery("field", []byte("a"), []byte("z"), true, true)

	if _, err := searcher.Search(query, 5); err != nil {
		t.Fatalf("Search before close failed: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("Failed to close reader: %v", err)
	}

	// After the reader is closed the search must fail. Lucene also tolerates
	// RejectedExecutionException from a closed thread pool; Gocene's searcher
	// is single-threaded, so only AlreadyClosedException is expected.
	_, err = searcher.Search(query, 5)
	if err == nil {
		t.Fatal("Expected search after reader close to fail, got nil error")
	}
	var ace *index.AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Fatalf("Expected AlreadyClosedException after reader close, got %T: %v", err, err)
	}
}
