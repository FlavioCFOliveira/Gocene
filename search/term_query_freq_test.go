// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/term_query_freq_test.go
// Source: lucene/core/src/java/org/apache/lucene/search/TermQuery.java
//
//	(TermWeight.scorerSupplier: the postings flag is
//	 scoreMode.needsScores() ? PostingsEnum.FREQS : PostingsEnum.NONE)
//
// Purpose: regression coverage for rmp #4751. A scoring TermQuery must observe
// the true per-document term frequency, so BM25/Classic scores rise with tf.
// The bug was that TermWeight.Scorer requested PostingsEnum.NONE (flag 0)
// unconditionally; the faithful Lucene104 postings reader then returns freq=1
// for every document (the documented NONE contract), collapsing all scores.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestTermQuery_FreqVariesWithTf reproduces the rmp #4751 diagnosis directly:
// three documents in which the query term "alpha" appears 1, 2 and 3 times.
// A scoring TermQuery must (a) score higher-tf documents strictly higher than
// lower-tf documents, and (b) report the true frequency per document through
// the TermWeight.Explain freq sub-detail.
func TestTermQuery_FreqVariesWithTf(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	// Isolate the postings encoding from compound-file packing, matching the
	// task's reproduction recipe.
	config.SetUseCompoundFile(false)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Doc i carries (i+1) copies of "alpha" plus a filler token so the field
	// length differs as Lucene's BM25 expects.
	for i := 0; i < 3; i++ {
		var sb strings.Builder
		for r := 0; r <= i; r++ {
			if r > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString("alpha")
		}
		sb.WriteString(" filler")

		field, ferr := document.NewTextField("content", sb.String(), true)
		if ferr != nil {
			t.Fatalf("NewTextField: %v", ferr)
		}
		doc := document.NewDocument()
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "alpha"))

	top, err := searcher.Search(query, reader.MaxDoc())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(top.ScoreDocs) != 3 {
		t.Fatalf("expected 3 matching docs, got %d", len(top.ScoreDocs))
	}

	// (a) Scores must rise strictly with tf. Doc ordering on disk follows
	// insertion order, so docID i has tf i+1.
	scoreByDoc := make(map[int]float32, 3)
	for _, sd := range top.ScoreDocs {
		scoreByDoc[sd.Doc] = sd.Score
	}
	if !(scoreByDoc[2] > scoreByDoc[1] && scoreByDoc[1] > scoreByDoc[0]) {
		t.Errorf("scores must increase with tf: doc0=%g doc1=%g doc2=%g",
			scoreByDoc[0], scoreByDoc[1], scoreByDoc[2])
	}

	// (b) Per-doc freq via TermWeight.Explain must equal the true tf for that
	// doc. The weight is built exactly as IndexSearcher.searchLeaf does (scoring
	// weight over the segment-level leaf context).
	weight, err := query.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := leafContext(t, reader)
	for docID := 0; docID < 3; docID++ {
		expl, err := weight.Explain(ctx, docID)
		if err != nil {
			t.Fatalf("Explain(doc=%d): %v", docID, err)
		}
		if !expl.IsMatch() {
			t.Fatalf("Explain(doc=%d): expected a match", docID)
		}
		gotFreq, ok := explainFreq(expl)
		if !ok {
			t.Fatalf("Explain(doc=%d): no freq sub-detail found", docID)
		}
		if want := float32(docID + 1); gotFreq != want {
			t.Errorf("doc=%d: freq = %g, want %g", docID, gotFreq, want)
		}
	}
}

// leafContext returns the single-segment leaf context for a reader produced by
// OpenDirectoryReader over a small in-memory index, mirroring the context that
// IndexSearcher.searchLeaf constructs (the SegmentReader, which overrides
// Terms(), at docBase 0).
func leafContext(t *testing.T, reader index.IndexReaderInterface) *index.LeafReaderContext {
	t.Helper()
	dr, ok := reader.(*index.DirectoryReader)
	if !ok {
		t.Fatalf("reader is %T, want *index.DirectoryReader", reader)
	}
	srs := dr.GetSegmentReaders()
	if len(srs) != 1 {
		t.Fatalf("expected exactly 1 segment, got %d", len(srs))
	}
	return index.NewLeafReaderContext(srs[0], nil, 0, 0)
}

// explainFreq walks an Explanation tree looking for the
// "freq, occurrences of term within document" leaf and returns its value.
func explainFreq(e search.Explanation) (float32, bool) {
	for _, d := range e.GetDetails() {
		if strings.Contains(d.GetDescription(), "freq, occurrences of term") {
			return d.GetValue(), true
		}
		if v, ok := explainFreq(d); ok {
			return v, true
		}
	}
	return 0, false
}
