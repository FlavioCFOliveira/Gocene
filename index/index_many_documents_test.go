// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexManyDocuments ports org.apache.lucene.index.TestIndexManyDocuments.
//
// It indexes a large number of documents from two concurrent goroutines and
// asserts that no document is lost: both the writer's DocStats.MaxDoc and the
// reopened DirectoryReader.MaxDoc must equal the requested document count.
//
// Divergences from the Java reference (Sprint 55, option c):
//   - Gocene's testDocument is a minimal stub holding opaque fields; the Java
//     test's TextField "field" cannot be reproduced faithfully, so documents
//     are indexed empty. This mirrors index_writer_threads_test.go.
//   - newFSDirectory(createTempDir()) is replaced by NewByteBuffersDirectory,
//     and the random MockAnalyzer by createTestAnalyzer, matching the
//     established index_writer_threads_test.go pattern.
//   - TestUtil.nextInt random maxBufferedDocs and atLeast(10000) scaling are
//     replaced by fixed values: numDocs is kept modest to keep the unit test
//     fast, with maxBufferedDocs well below it so flushing still occurs.
func TestIndexManyDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(createTestAnalyzer())
	iwc.SetMaxBufferedDocs(128)

	const numDocs = 2000

	w, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	var count atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for int(count.Add(1)-1) < numDocs {
				doc := &testDocument{fields: []interface{}{}}
				if err := w.AddDocument(doc); err != nil {
					t.Errorf("AddDocument() error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := w.GetDocStats().MaxDoc; got != numDocs {
		t.Errorf("lost %d documents; maxBufferedDocs=%d: MaxDoc = %d, want %d",
			numDocs-got, iwc.MaxBufferedDocs(), got, numDocs)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader() error = %v", err)
	}
	if got := r.MaxDoc(); got != numDocs {
		t.Errorf("reader MaxDoc = %d, want %d", got, numDocs)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close() error = %v", err)
	}
}
