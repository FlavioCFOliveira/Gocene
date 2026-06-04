// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestSearchWithThreads.java
//
// This test hammers a single shared IndexSearcher from several goroutines, each
// running many TermQuery counting searches concurrently, and asserts every
// worker observed both searches and a positive hit total. It verifies that the
// committed-reader search path is safe for concurrent reads (run with -race).
package search_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSearchWithThreads_Search mirrors TestSearchWithThreads.test.
func TestSearchWithThreads_Search(t *testing.T) {
	const (
		numThreads  = 2
		numSearches = 500
		numDocs     = 200
	)

	ix := newIntegrationIndex(t)
	// Build numDocs documents, each with a deterministic-but-varied mix of the
	// two tokens "aaa" and "bbb" so that both terms match a non-trivial subset.
	for docCount := 0; docCount < numDocs; docCount++ {
		var sb strings.Builder
		numTerms := docCount % 10
		for termCount := 0; termCount < numTerms; termCount++ {
			if (docCount+termCount)%2 == 0 {
				sb.WriteString("aaa")
			} else {
				sb.WriteString("bbb")
			}
			sb.WriteByte(' ')
		}
		doc := document.NewDocument()
		f, err := document.NewTextField("body", sb.String(), false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	aaa := search.NewTermQuery(index.NewTerm("body", "aaa"))
	bbb := search.NewTermQuery(index.NewTerm("body", "bbb"))

	var failed atomic.Bool
	var netSearch atomic.Int64
	var startingGun sync.WaitGroup
	startingGun.Add(1)

	count := func(q search.Query) (int64, error) {
		c := search.NewTotalHitCountCollector()
		if err := searcher.SearchWithCollector(q, c); err != nil {
			return 0, err
		}
		return int64(c.GetTotalHits()), nil
	}

	var wg sync.WaitGroup
	for threadID := 0; threadID < numThreads; threadID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startingGun.Wait()
			var totHits, totSearch int64
			for ; totSearch < numSearches && !failed.Load(); totSearch++ {
				h1, err := count(aaa)
				if err != nil {
					failed.Store(true)
					t.Errorf("count(aaa): %v", err)
					return
				}
				h2, err := count(bbb)
				if err != nil {
					failed.Store(true)
					t.Errorf("count(bbb): %v", err)
					return
				}
				totHits += h1 + h2
			}
			if !(totSearch > 0 && totHits > 0) {
				failed.Store(true)
				t.Errorf("worker observed totSearch=%d totHits=%d, want both > 0", totSearch, totHits)
				return
			}
			netSearch.Add(totSearch)
		}()
	}

	startingGun.Done()
	wg.Wait()

	if failed.Load() {
		t.Fatal("at least one concurrent search worker failed")
	}
	if got := netSearch.Load(); got != int64(numThreads*numSearches) {
		t.Errorf("netSearch = %d, want %d", got, numThreads*numSearches)
	}
}
