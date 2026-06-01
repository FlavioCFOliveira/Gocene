// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestBagOfPostings ports org.apache.lucene.index.TestBagOfPostings#test.
//
// It indexes numeric terms where each term i appears exactly i times across
// the index, then verifies that every term's docFreq equals its integer value.
// Each document keeps a per-document visited set, so a given term is added at
// most once per document; therefore the i copies of term i land in i distinct
// documents and docFreq(i) == i. The numTerms-1 term count holds because term
// "0" has zero postings and is never indexed.
//
// Divergences from Lucene:
//   - Lucene drives the write through RandomIndexWriter and spreads the
//     postings across 1-5 concurrent threads with a randomized merge policy;
//     Gocene exposes no randomized test-writer wrapper, so this port uses the
//     plain IndexWriter and a single goroutine. The docFreq invariant is
//     independent of document layout, so the assertion is unaffected.
//   - Lucene reads via IndexWriter.getReader (near-real-time); Gocene's
//     IndexWriter has no NRT reader, so the index is reopened from the
//     directory after commit, matching TestBagOfPositions.
//   - Lucene's @SuppressCodecs("Direct") and the SimpleText/MockRandomMergePolicy
//     numTerms halving are codec-randomization concerns with no Gocene analogue.
func TestBagOfPostings(t *testing.T) {
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

	// Drain the postings bag into documents. Each document accepts up to
	// maxTermsPerDoc terms and rejects a token already seen in that document
	// (the rejected token is pushed back), so every term lands in a distinct
	// document each time it appears, matching Lucene's per-thread HashSet.
	postings := append([]string(nil), postingsList...)
	for len(postings) > 0 {
		var text strings.Builder
		visited := make(map[string]struct{})
		for i := 0; i < maxTermsPerDoc && len(postings) > 0; i++ {
			token := postings[0]
			if _, seen := visited[token]; seen {
				// Already in this document; leave it for a later document.
				break
			}
			postings = postings[1:]
			text.WriteByte(' ')
			text.WriteString(token)
			visited[token] = struct{}{}
		}
		// A document is only empty if its very first token was a duplicate,
		// which cannot happen for an empty visited set; guard regardless so
		// the loop always makes progress.
		if text.Len() == 0 {
			postings = postings[1:]
			continue
		}

		doc := document.NewDocument()
		field, err := document.NewTextField("field", text.String(), false)
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
		docFreq, err := termsEnum.DocFreq()
		if err != nil {
			t.Fatalf("DocFreq() failed for term %q: %v", term.Text(), err)
		}
		if docFreq != value {
			t.Errorf("term %q: docFreq = %d, want %d", term.Text(), docFreq, value)
		}
	}
}
