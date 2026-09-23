// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestStressDeletes.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestStressDeletes makes sure that order of adds/deletes across threads is
// respected as long as each ID is only changed by one thread at a time.
func TestStressDeletes(t *testing.T) {
	numIDs := atLeast(100)
	locks := make([]sync.Mutex, numIDs)

	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	iters := atLeast(2000)
	var existsMu sync.Mutex
	exists := make(map[int]bool)
	threads := make([]int, nextInt(2, 6))
	startingGun := make(chan struct{})
	deleteMode := rand.Intn(3)
	var wg sync.WaitGroup
	for range threads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startingGun
			for iter := 0; iter < iters; iter++ {
				id := rand.Intn(numIDs)
				if err := stressDeletesStep(w, &locks[id], &existsMu, exists, id, deleteMode); err != nil {
					t.Errorf("RuntimeException: %v", err)
					return
				}
				if rand.Intn(500) == 2 {
					r, err := index.OpenDirectoryReaderFromWriterWithOptions(w, rand.Intn(2) == 0, false)
					if err != nil {
						t.Errorf("DirectoryReader.open(w): %v", err)
						return
					}
					if err := r.Close(); err != nil {
						t.Errorf("close: %v", err)
						return
					}
				}
				if rand.Intn(500) == 2 {
					if _, err := w.Commit(); err != nil {
						t.Errorf("commit: %v", err)
						return
					}
				}
			}
		}()
	}

	close(startingGun)
	wg.Wait()

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	s := search.NewIndexSearcher(r)
	for id, live := range exists {
		hits, err := s.Search(search.NewTermQuery(index.NewTerm("id", strconv.Itoa(id))), 1)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		want := int64(0)
		if live {
			want = 1
		}
		if hits.TotalHits.Value != want {
			t.Fatalf("id %d: expected %d hits, got %d", id, want, hits.TotalHits.Value)
		}
	}
	mustClose(t, r, w, dir)
}

// stressDeletesStep renders the synchronized (locks[id]) block of the Java
// thread body.
func stressDeletesStep(w *index.IndexWriter, lock *sync.Mutex, existsMu *sync.Mutex, exists map[int]bool, id, deleteMode int) error {
	lock.Lock()
	defer lock.Unlock()
	existsMu.Lock()
	v, ok := exists[id]
	existsMu.Unlock()
	term := index.NewTerm("id", strconv.Itoa(id))
	if !ok || !v {
		doc := document.NewDocument()
		f, err := document.NewStringField("id", strconv.Itoa(id), false)
		if err != nil {
			return err
		}
		doc.Add(f)
		if _, err := w.AddDocument(doc); err != nil {
			return err
		}
		existsMu.Lock()
		exists[id] = true
		existsMu.Unlock()
		return nil
	}
	var err error
	switch {
	case deleteMode == 0:
		// Always delete by term
		_, err = w.DeleteDocuments([]index.Term{*term})
	case deleteMode == 1:
		// Always delete by query
		_, err = w.DeleteDocumentsQuery([]index.Query{search.NewTermQuery(term)})
	default:
		// Mixed
		if rand.Intn(2) == 0 {
			_, err = w.DeleteDocuments([]index.Term{*term})
		} else {
			_, err = w.DeleteDocumentsQuery([]index.Query{search.NewTermQuery(term)})
		}
	}
	if err != nil {
		return err
	}
	existsMu.Lock()
	exists[id] = false
	existsMu.Unlock()
	return nil
}
