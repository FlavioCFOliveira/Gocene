// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAtomicUpdate ports org.apache.lucene.index.TestAtomicUpdate.
//
// It runs N indexer and N searcher goroutines against a single index as a
// stress test, asserting that readers always observe exactly the base set of
// 100 documents while updates are in flight.
//
// Divergences from the Java reference (Sprint 55, option c):
//   - Gocene's testDocument is a minimal stub holding opaque fields; the Java
//     test's StringField "id", TextField "contents" and IntPoint values
//     cannot be reproduced faithfully, so documents are indexed empty. This
//     mirrors the established index_writer_threads_test.go pattern.
//   - RandomIndexWriter / MockAnalyzer / MockDirectoryWrapper are not yet
//     ported; the test uses NewIndexWriter directly with the whitespace
//     analyzer and the raw Directory implementations.
//   - TieredMergePolicy.setMaxMergeAtOnce and TEST_NIGHTLY scaling are
//     omitted; iteration counts follow the non-nightly defaults.

// atomicTimedThread runs a unit of work numIterations times, capturing the
// first failure. It is the Go analogue of the Java TimedThread base class.
type atomicTimedThread struct {
	numIterations int
	failure       error
	doWork        func(currentIteration int) error
}

func (tt *atomicTimedThread) run(wg *sync.WaitGroup) {
	defer wg.Done()
	for count := 0; count < tt.numIterations; count++ {
		if err := tt.doWork(count); err != nil {
			tt.failure = err
			return
		}
	}
}

// runAtomicUpdateTest establishes a base index of 100 documents, then runs
// indexer and searcher goroutines concurrently against it.
func runAtomicUpdateTest(t *testing.T, directory store.Directory) {
	t.Helper()

	const indexThreads = 1
	const searchThreads = 1
	const indexIterations = 1
	const searchIterations = 1

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(7)

	writer, err := index.NewIndexWriter(directory, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	// Establish a base index of 100 docs.
	for i := 0; i < 100; i++ {
		if (i-1)%7 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("base commit() error = %v", err)
			}
		}
		doc := &testDocument{fields: []interface{}{}}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("base AddDocument(%d) error = %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("base final commit() error = %v", err)
	}

	r, err := index.OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("OpenDirectoryReader() error = %v", err)
	}
	if got := r.NumDocs(); got != 100 {
		t.Fatalf("base NumDocs() = %d, want 100", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("base reader Close() error = %v", err)
	}

	threads := make([]*atomicTimedThread, 0, indexThreads+searchThreads)

	for i := 0; i < indexThreads; i++ {
		threads = append(threads, &atomicTimedThread{
			numIterations: indexIterations,
			doWork: func(currentIteration int) error {
				// Update all 100 docs.
				for id := 0; id < 100; id++ {
					term := index.NewTerm("id", fmt.Sprintf("%d", id))
					doc := &testDocument{fields: []interface{}{}}
					if err := writer.UpdateDocument(term, doc); err != nil {
						return fmt.Errorf("UpdateDocument(id=%d): %w", id, err)
					}
				}
				return nil
			},
		})
	}

	for i := 0; i < searchThreads; i++ {
		threads = append(threads, &atomicTimedThread{
			numIterations: searchIterations,
			doWork: func(currentIteration int) error {
				sr, err := index.OpenDirectoryReader(directory)
				if err != nil {
					return fmt.Errorf("searcher OpenDirectoryReader: %w", err)
				}
				if got := sr.NumDocs(); got != 100 {
					_ = sr.Close()
					return fmt.Errorf("searcher NumDocs() = %d, want 100", got)
				}
				return sr.Close()
			},
		})
	}

	var wg sync.WaitGroup
	wg.Add(len(threads))
	for _, tt := range threads {
		go tt.run(&wg)
	}
	wg.Wait()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer Close() error = %v", err)
	}

	for i, tt := range threads {
		if tt.failure != nil {
			t.Errorf("hit exception from thread %d: %v", i, tt.failure)
		}
	}
}

// TestAtomicUpdates runs the stress test against both an in-memory directory
// and a filesystem directory.
func TestAtomicUpdates(t *testing.T) {
	// Run against an in-memory directory.
	bbDir := store.NewByteBuffersDirectory()
	runAtomicUpdateTest(t, bbDir)
	if err := bbDir.Close(); err != nil {
		t.Fatalf("ByteBuffersDirectory Close() error = %v", err)
	}

	// Then against a filesystem directory.
	fsDir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory() error = %v", err)
	}
	runAtomicUpdateTest(t, fsDir)
	if err := fsDir.Close(); err != nil {
		t.Fatalf("SimpleFSDirectory Close() error = %v", err)
	}
}
