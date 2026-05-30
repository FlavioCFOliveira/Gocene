// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestBagOfPositions ports org.apache.lucene.index.TestBagOfPositions#test.
//
// It indexes numeric terms where each term i appears exactly i times across
// the index, then verifies that every term's totalTermFreq equals its integer
// value. The numTerms-1 term count holds because term "0" has zero postings
// and therefore is never indexed.
//
// Divergences from Lucene:
//   - Lucene drives the write through RandomIndexWriter and spreads the
//     postings across 1-5 concurrent threads with a randomized merge policy;
//     Gocene exposes no randomized test-writer wrapper, so this port uses the
//     plain IndexWriter and a single goroutine. The totalTermFreq invariant is
//     independent of document layout, so the assertion is unaffected.
//   - Lucene reads via IndexWriter.getReader (near-real-time); Gocene's
//     IndexWriter has no NRT reader, so the index is reopened from the
//     directory after commit, matching TestBinaryTerms.
//   - The Java field-type randomization (omitNorms, DOCS_AND_FREQS with term
//     vectors, or DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS) is dropped; a plain
//     TextField is used, since none of those options change totalTermFreq.
func TestBagOfPositions(t *testing.T) {
	// Pre-existing infrastructure gap: OpenDirectoryReader materialises each
	// segment via NewSegmentReader (index/directory_reader.go:462/497), which
	// leaves SegmentReader.coreReaders nil. Without the codec-side wiring that
	// loads SegmentCoreReaders from disk, LeafReader.Terms returns the "core
	// readers are nil" error and the assertions below cannot run. Unskip once
	// OpenDirectoryReader uses NewSegmentReaderWithCore.
	t.Fatal("blocked: OpenDirectoryReader builds SegmentReader without core readers (index/directory_reader.go:462/497); fix is NewSegmentReaderWithCore")

	const numTerms = 100
	maxTermsPerDoc := 10 + rand.Intn(11) // TestUtil.nextInt(random(), 10, 20)

	// Build the postings bag: term i contributes i copies of itself.
	postingsList := make([]string, 0)
	for i := 0; i < numTerms; i++ {
		term := strconv.Itoa(i)
		for j := 0; j < i; j++ {
			postingsList = append(postingsList, term)
		}
	}
	rand.Shuffle(len(postingsList), func(a, b int) {
		postingsList[a], postingsList[b] = postingsList[b], postingsList[a]
	})

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

	// Drain the postings bag into documents of up to maxTermsPerDoc terms each.
	for pos := 0; pos < len(postingsList); {
		end := pos + rand.Intn(maxTermsPerDoc)
		if end > len(postingsList) {
			end = len(postingsList)
		}
		text := ""
		for ; pos < end; pos++ {
			text += " " + postingsList[pos]
		}
		// rand.Intn(maxTermsPerDoc) can be 0; skip empty documents so progress
		// is always made and the loop terminates.
		if end == pos && text == "" {
			pos++
			continue
		}

		doc := document.NewDocument()
		field, err := document.NewTextField("field", text, false)
		if err != nil {
			t.Fatalf("Failed to create field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
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
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf after forceMerge(1), got %d", len(leaves))
	}

	air := leaves[0].LeafReader()
	terms, err := air.Terms("field")
	if err != nil {
		t.Fatalf("Failed to get terms: %v", err)
	}
	// numTerms-1 because there cannot be a term "0" with 0 postings.
	if got := terms.Size(); got != int64(numTerms-1) {
		t.Fatalf("terms.Size() = %d, want %d", got, numTerms-1)
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("Failed to get terms iterator: %v", err)
	}
	for {
		term, err := termsEnum.Next()
		if err != nil {
			t.Fatalf("TermsEnum.Next() failed: %v", err)
		}
		if term == nil {
			break
		}
		value, err := strconv.Atoi(term.Text())
		if err != nil {
			t.Fatalf("term %q is not numeric: %v", term.Text(), err)
		}
		ttf, err := termsEnum.TotalTermFreq()
		if err != nil {
			t.Fatalf("TotalTermFreq() failed for term %q: %v", term.Text(), err)
		}
		if ttf != int64(value) {
			t.Errorf("term %q: totalTermFreq = %d, want %d", term.Text(), ttf, value)
		}
	}
}
