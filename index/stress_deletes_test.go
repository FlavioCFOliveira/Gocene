// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GOC-4180: port of org.apache.lucene.index.TestStressDeletes.
//
// Make sure that the order of adds/deletes across threads is respected as long
// as each ID is only changed by one thread at a time.
func TestStressDeletes(t *testing.T) {
	const numIDs = 100
	locks := make([]sync.Mutex, numIDs)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	cfg := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const iters = 2000
	var existsMu sync.Mutex
	exists := make(map[int]bool)

	rng := rand.New(rand.NewSource(42))
	numThreads := 2 + rng.Intn(5) // [2, 6]
	deleteMode := rng.Intn(3)   // 0: term, 1: query, 2: mixed

	var wg sync.WaitGroup
	startingGun := make(chan struct{})

	for tID := 0; tID < numThreads; tID++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			<-startingGun
			for iter := 0; iter < iters; iter++ {
				id := r.Intn(numIDs)
				locks[id].Lock()
				existsMu.Lock()
				present := exists[id]
				existsMu.Unlock()

				if !present {
					doc := document.NewDocument()
					f, _ := document.NewStringField("id", strconv.Itoa(id), false)
					doc.Add(f)
					if err := w.AddDocument(doc); err != nil {
						locks[id].Unlock()
						t.Errorf("AddDocument: %v", err)
						return
					}
					existsMu.Lock()
					exists[id] = true
					existsMu.Unlock()
				} else {
					term := index.NewTerm("id", strconv.Itoa(id))
					byTerm := deleteMode == 0 || (deleteMode == 2 && r.Intn(2) == 0)
					if byTerm {
						if err := w.DeleteDocuments(term); err != nil {
							locks[id].Unlock()
							t.Errorf("DeleteDocuments: %v", err)
							return
						}
					} else {
						if err := w.DeleteDocumentsQuery(search.NewTermQuery(term)); err != nil {
							locks[id].Unlock()
							t.Errorf("DeleteDocumentsQuery: %v", err)
							return
						}
					}
					existsMu.Lock()
					exists[id] = false
					existsMu.Unlock()
				}
				locks[id].Unlock()

				if r.Intn(500) == 2 {
					if err := w.Commit(); err != nil {
						t.Errorf("Commit: %v", err)
						return
					}
				}
			}
		}(int64(1000 + tID))
	}

	close(startingGun)
	wg.Wait()

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	searcher := search.NewIndexSearcher(reader)

	for id, present := range exists {
		hits, err := searcher.Search(search.NewTermQuery(index.NewTerm("id", strconv.Itoa(id))), 1)
		if err != nil {
			t.Fatalf("Search id=%d: %v", id, err)
		}
		want := int64(0)
		if present {
			want = 1
		}
		if got := hits.TotalHits.Value; got != want {
			t.Errorf("id=%d: got %d hits, want %d", id, got, want)
		}
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
}
