// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestSameScoresWithThreads.java
//
// This test builds a single-threaded baseline of TopDocs for a set of terms,
// then re-runs the same TermQuery searches from several goroutines (in a
// shuffled order, repeatedly) and asserts that each concurrent result is
// byte-for-byte identical to the baseline — same totalHits, same number of
// hits, same doc ids, and bit-identical scores. It guards against any shared
// mutable state corrupting scoring under concurrency (run with -race).
package search_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSameScoresWithThreads_TestSameScoresWithThreads mirrors
// TestSameScoresWithThreads.test.
func TestSameScoresWithThreads_TestSameScoresWithThreads(t *testing.T) {
	// A small fixed vocabulary; documents draw a varying multiset from it so the
	// per-term scores (which depend on term frequency and document length) differ
	// across documents, making the score-equality assertion meaningful.
	vocab := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}

	ix := newIntegrationIndex(t)
	const numDocs = 400
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < numDocs; i++ {
		n := 1 + rng.Intn(12)
		body := ""
		for j := 0; j < n; j++ {
			if j > 0 {
				body += " "
			}
			body += vocab[rng.Intn(len(vocab))]
		}
		doc := document.NewDocument()
		f, err := document.NewTextField("body", body, false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
		if i%97 == 0 {
			ix.commit() // force a multi-segment index
		}
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	// Baseline: search every vocabulary term once, single-threaded.
	type answer struct {
		term string
		top  *search.TopDocs
	}
	answers := make([]answer, 0, len(vocab))
	for _, term := range vocab {
		top, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", term)), 100)
		if err != nil {
			t.Fatalf("baseline Search(%q): %v", term, err)
		}
		answers = append(answers, answer{term: term, top: top})
	}

	const numThreads = 3
	var startingGun sync.WaitGroup
	startingGun.Add(1)
	var wg sync.WaitGroup
	errCh := make(chan error, numThreads)

	for threadID := 0; threadID < numThreads; threadID++ {
		wg.Add(1)
		seed := int64(threadID + 1)
		go func() {
			defer wg.Done()
			startingGun.Wait()
			r := rand.New(rand.NewSource(seed))
			for iter := 0; iter < 20; iter++ {
				order := r.Perm(len(answers))
				for _, idx := range order {
					expected := answers[idx]
					actual, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", expected.term)), 100)
					if err != nil {
						errCh <- fmt.Errorf("Search(%q): %w", expected.term, err)
						return
					}
					if err := compareTopDocs(expected.term, expected.top, actual); err != nil {
						errCh <- err
						return
					}
				}
			}
		}()
	}

	startingGun.Done()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// compareTopDocs asserts that two TopDocs for the same query are identical.
func compareTopDocs(term string, expected, actual *search.TopDocs) error {
	if expected.TotalHits.Value != actual.TotalHits.Value {
		return fmt.Errorf("query=%q: totalHits %d != %d", term, expected.TotalHits.Value, actual.TotalHits.Value)
	}
	if len(expected.ScoreDocs) != len(actual.ScoreDocs) {
		return fmt.Errorf("query=%q: scoreDocs length %d != %d", term, len(expected.ScoreDocs), len(actual.ScoreDocs))
	}
	for hit := range expected.ScoreDocs {
		if expected.ScoreDocs[hit].Doc != actual.ScoreDocs[hit].Doc {
			return fmt.Errorf("query=%q: hit %d doc %d != %d", term, hit, expected.ScoreDocs[hit].Doc, actual.ScoreDocs[hit].Doc)
		}
		// Floats really should be identical across runs.
		if expected.ScoreDocs[hit].Score != actual.ScoreDocs[hit].Score {
			return fmt.Errorf("query=%q: hit %d score %v != %v", term, hit, expected.ScoreDocs[hit].Score, actual.ScoreDocs[hit].Score)
		}
	return nil
}