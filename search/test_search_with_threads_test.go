// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestSearchWithThreads.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

func TestSearchWithThreads(t *testing.T) {
	numThreads := 2             // TEST_NIGHTLY ? 5 : 2
	numSearches := atLeast(500) // TEST_NIGHTLY ? atLeast(2000) : atLeast(500)
	numDocs := atLeast(200)     // TEST_NIGHTLY ? atLeast(10000) : atLeast(200)
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	doc := document.NewDocument()
	body := newTextField(t, "body", "", false)
	doc.Add(body)
	var sb strings.Builder
	for docCount := 0; docCount < numDocs; docCount++ {
		numTerms := random().Intn(10)
		for termCount := 0; termCount < numTerms; termCount++ {
			if random().Intn(2) == 0 {
				sb.WriteString("aaa")
			} else {
				sb.WriteString("bbb")
			}
			sb.WriteByte(' ')
		}
		body.SetStringValue(sb.String())
		mustAddDocument(t, w, doc)
		sb.Reset()
	}
	r := mustGetReader(t, w)
	mustClose(t, w)

	s := newSearcher(t, r)

	var failed atomic.Bool
	var netSearch atomic.Int64

	collectorManager := testsearch.DummyTotalHitCountCollectorCreateManager()
	var threads sync.WaitGroup
	for threadID := 0; threadID < numThreads; threadID++ {
		threads.Add(1)
		go func() {
			defer threads.Done()
			totHits := int64(0)
			totSearch := int64(0)
			for ; totSearch < int64(numSearches) && !failed.Load(); totSearch++ {
				hits, err := search.SearchWithCollectorManager(s, search.NewTermQuery(index.NewTerm("body", "aaa")), collectorManager)
				if err != nil {
					failed.Store(true)
					t.Errorf("RuntimeException: %v", err)
					return
				}
				totHits += int64(hits)
				hits, err = search.SearchWithCollectorManager(s, search.NewTermQuery(index.NewTerm("body", "bbb")), collectorManager)
				if err != nil {
					failed.Store(true)
					t.Errorf("RuntimeException: %v", err)
					return
				}
				totHits += int64(hits)
			}
			if !(totSearch > 0 && totHits > 0) {
				t.Errorf("assertTrue(totSearch > 0 && totHits > 0): totSearch=%d totHits=%d", totSearch, totHits)
			}
			netSearch.Add(totSearch)
		}()
		// threads[threadID].setDaemon(true): no Go counterpart.
	}

	threads.Wait()

	if testing.Verbose() {
		t.Logf("%d threads did %d searches", numThreads, netSearch.Load())
	}

	mustClose(t, r, dir)
}
