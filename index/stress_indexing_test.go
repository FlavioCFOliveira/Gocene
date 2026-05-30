// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GOC-4217: port of org.apache.lucene.index.TestStressIndexing.
//
// Runs two indexer threads (add 10 docs, delete 5, repeatedly) and two searcher
// threads (constantly reopen a DirectoryReader) against a single index.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub in Gocene, so the
// delete half of each indexer iteration cannot exercise real behaviour yet.
// The full roundtrip is retained for when delete application lands. See
// stress_deletes_test.go for the same documented infra gap.

// runIterations mirrors TimedThread.RUN_ITERATIONS (non-nightly path).
const runIterations = 20

// intToEnglish is a minimal stand-in for Lucene's tests English.intToEnglish:
// it just produces deterministic multi-token text from an integer so the
// "contents" field is non-trivial.
func intToEnglish(n int) string {
	ones := []string{
		"zero", "one", "two", "three", "four",
		"five", "six", "seven", "eight", "nine",
	}
	if n < 0 {
		n = -n
	}
	if n == 0 {
		return ones[0]
	}
	var words []string
	for n > 0 {
		words = append(words, ones[n%10])
		n /= 10
	}
	out := words[0]
	for i := 1; i < len(words); i++ {
		out = words[i] + " " + out
	}
	return out
}

// indexerWork performs one TestStressIndexing.IndexerThread.doWork iteration.
func indexerWork(w *index.IndexWriter, r *rand.Rand, nextID *int) error {
	// Add 10 docs.
	for j := 0; j < 10; j++ {
		d := document.NewDocument()
		n := r.Int()
		idField, err := document.NewStringField("id", strconv.Itoa(*nextID), true)
		if err != nil {
			return err
		}
		*nextID++
		d.Add(idField)
		contents, err := document.NewTextField("contents", intToEnglish(n), false)
		if err != nil {
			return err
		}
		d.Add(contents)
		if err := w.AddDocument(d); err != nil {
			return err
		}
	}

	// Delete 5 docs.
	deleteID := *nextID - 1
	for j := 0; j < 5; j++ {
		if err := w.DeleteDocuments(index.NewTerm("id", strconv.Itoa(deleteID))); err != nil {
			return err
		}
		deleteID -= 2
	}
	return nil
}

func TestStressIndexAndSearching(t *testing.T) {
	t.Fatal("infra gap: DeleteDocuments is a no-op stub; full roundtrip retained for when delete application lands")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	cfg := index.NewIndexWriterConfig(analyzer)
	cfg.SetOpenMode(index.CREATE)
	cfg.SetMaxBufferedDocs(10)
	cfg.SetMergeScheduler(index.NewConcurrentMergeScheduler())

	modifier, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// failed is the shared cross-thread error flag (TimedThread.anyErrors).
	var failed atomic.Bool

	var wg sync.WaitGroup

	// Two indexer threads.
	for tID := 0; tID < 2; tID++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			nextID := 0
			for iter := 0; iter < runIterations; iter++ {
				if failed.Load() {
					return
				}
				if err := indexerWork(modifier, r, &nextID); err != nil {
					t.Errorf("indexer: %v", err)
					failed.Store(true)
					return
				}
			}
		}(int64(1000 + tID))
	}

	// Two searcher threads.
	for tID := 0; tID < 2; tID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iter := 0; iter < runIterations; iter++ {
				if failed.Load() {
					return
				}
				for i := 0; i < 100; i++ {
					ir, err := index.OpenDirectoryReader(dir)
					if err != nil {
						t.Errorf("searcher: OpenDirectoryReader: %v", err)
						failed.Store(true)
						return
					}
					if err := ir.Close(); err != nil {
						t.Errorf("searcher: reader.Close: %v", err)
						failed.Store(true)
						return
					}
				}
			}
		}()
	}

	wg.Wait()

	if err := modifier.Close(); err != nil {
		t.Fatalf("modifier.Close: %v", err)
	}
	if failed.Load() {
		t.Fatal("one or more stress threads failed")
	}
}
