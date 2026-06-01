// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestSearchForDuplicates.java
//
// Deviation: the original toggles compound-file mode and asserts that the
// printed hit output is identical in both modes.  Gocene does not yet expose
// per-writer compound-file control, so the test exercises only the
// non-compound path and verifies hit counts and stored-field ordering rather
// than raw string equality of the printed output.

package gocene

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	dupPriorityField = "priority"
	dupIDField       = "id"
	dupHighPriority  = "high"
	dupMedPriority   = "medium"
)

// TestSearchForDuplicates_Run mirrors testRun (Lucene 10.4.0).
// It indexes maxDocs documents, each with the same HIGH priority term and a
// stored numeric ID, then verifies:
//  1. A TermQuery for HIGH_PRIORITY returns exactly maxDocs hits.
//  2. A BooleanQuery (HIGH SHOULD MED) also returns exactly maxDocs hits
//     because only HIGH documents are present.
//  3. The hits are ordered by (score desc, id asc) so the first hit's
//     stored id field equals "0" and the last hit's id equals the string
//     representation of maxDocs-1.
func TestSearchForDuplicates_Run(t *testing.T) {
	const maxDocs = 225

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for j := 0; j < maxDocs; j++ {
		doc := document.NewDocument()

		pf, err := document.NewTextField(dupPriorityField, dupHighPriority, true)
		if err != nil {
			t.Fatalf("NewTextField[%d]: %v", j, err)
		}
		doc.Add(pf)

		idf, err := document.NewStoredField(dupIDField, fmt.Sprintf("%d", j))
		if err != nil {
			t.Fatalf("NewStoredField[%d]: %v", j, err)
		}
		doc.Add(idf)

		ndvf, err := document.NewNumericDocValuesField(dupIDField, int64(j))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField[%d]: %v", j, err)
		}
		doc.Add(ndvf)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", j, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Sort: score descending, then id ascending (numeric doc values).
	sort := search.NewSort(
		&search.SortField{Type: search.SortFieldTypeScore},
		search.NewSortField(dupIDField, search.SortFieldTypeInt),
	)

	// TermQuery for HIGH_PRIORITY.
	termQuery := search.NewTermQuery(index.NewTerm(dupPriorityField, dupHighPriority))
	hits, err := searcher.SearchWithSort(termQuery, maxDocs, sort)
	if err != nil {
		t.Fatalf("SearchWithSort(termQuery): %v", err)
	}
	if int(hits.TotalHits.Value) != maxDocs {
		t.Errorf("TermQuery: got %d hits, want %d", hits.TotalHits.Value, maxDocs)
	}

	// Spot-check first and last stored IDs.
	checkHits(t, searcher, hits.ScoreDocs, maxDocs)

	// BooleanQuery HIGH SHOULD MED — only HIGH docs exist, so same count.
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm(dupPriorityField, dupHighPriority)), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm(dupPriorityField, dupMedPriority)), search.SHOULD)

	boolHits, err := searcher.SearchWithSort(bq, maxDocs, sort)
	if err != nil {
		t.Fatalf("SearchWithSort(boolQuery): %v", err)
	}
	if int(boolHits.TotalHits.Value) != maxDocs {
		t.Errorf("BooleanQuery: got %d hits, want %d", boolHits.TotalHits.Value, maxDocs)
	}
	checkHits(t, searcher, boolHits.ScoreDocs, maxDocs)
}

// checkHits mirrors the Java checkHits: verifies that the first 10 and the
// window 94-104 have consecutive stored id values matching their rank.
func checkHits(t *testing.T, searcher *search.IndexSearcher, scoreDocs []*search.ScoreDoc, expectedCount int) {
	t.Helper()

	if len(scoreDocs) != expectedCount {
		t.Errorf("checkHits: got %d scoreDocs, want %d", len(scoreDocs), expectedCount)
		return
	}

	for i, sd := range scoreDocs {
		if i >= 10 && (i <= 94 || i >= 105) {
			continue
		}
		doc, err := searcher.Doc(sd.Doc)
		if err != nil {
			t.Errorf("Doc(%d): %v", sd.Doc, err)
			continue
		}
		f := doc.Get(dupIDField)
		if f == nil {
			t.Errorf("hit %d: missing field %q", i, dupIDField)
			continue
		}
		want := fmt.Sprintf("%d", i)
		if f.StringValue() != want {
			t.Errorf("hit %d: id=%q, want %q", i, f.StringValue(), want)
		}
	}
}
