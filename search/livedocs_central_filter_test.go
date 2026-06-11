// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSearch_LiveDocsExcludedCentrally is the regression for rmp #4762:
// liveDocs must be applied centrally in IndexSearcher.searchLeaf, not only inside
// TermScorer. MatchAllDocsQuery's scorer iterates every doc id without consulting
// liveDocs, so a deleted document would leak into the results unless searchLeaf
// filters it. After deleting one of five documents the match-all query must
// return exactly the four live docs and never the deleted ordinal.
func TestSearch_LiveDocsExcludedCentrally(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	analyzer := analysis.NewWhitespaceAnalyzer()
	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analyzer))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	ids := []string{"d0", "d1", "d2", "d3", "d4"}
	for _, id := range ids {
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", id, true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(idField)
		body, err := document.NewTextField("body", "hello", true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(body)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%s): %v", id, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Delete d2 and commit so the deletion is applied to the committed segment.
	if err := writer.DeleteDocuments(index.NewTerm("id", "d2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit (delete): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	if got := reader.NumDocs(); got != 4 {
		t.Fatalf("NumDocs after deleting d2 = %d, want 4", got)
	}

	searcher := search.NewIndexSearcher(reader)
	top, err := searcher.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search(MatchAllDocsQuery): %v", err)
	}

	// The match-all scorer visits ordinal 2 (the deleted doc); searchLeaf must
	// drop it. Expect exactly the four live ordinals {0,1,3,4}.
	if len(top.ScoreDocs) != 4 {
		t.Fatalf("MatchAllDocsQuery returned %d hits, want 4 (deleted doc leaked?)", len(top.ScoreDocs))
	}
	for _, sd := range top.ScoreDocs {
		if sd.Doc == 2 {
			t.Errorf("deleted doc ordinal 2 was returned by MatchAllDocsQuery")
		}
}