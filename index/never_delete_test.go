// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestNeverDelete.
// Source: lucene/core/src/test/org/apache/lucene/index/TestNeverDelete.java
//
// GOC-4144: With NoDeletionPolicy, no file referenced by a commit point is
// ever deleted while indexing and reopening concurrently.
//
// Lucene deviations: RandomIndexWriter is replaced by the plain
// index.IndexWriter (Gocene exposes no random test wrapper). slowFileExists
// is inlined via store.Directory.FileExists. MockAnalyzer is replaced by
// analysis.NewWhitespaceAnalyzer, matching the other index_test ports.
package index_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNeverDelete_Indexing indexes and commits from multiple goroutines while a
// reader is repeatedly reopened, asserting that every file seen in any commit
// point still exists on disk.
func TestNeverDelete_Indexing(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer func() { _ = dir.Close() }()

	config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
	config.SetIndexDeletionPolicy(index.NoDeletionPolicyInstance)
	config.SetMaxBufferedDocs(10)

	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := w.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}

	const stopIterations = 100

	var wg sync.WaitGroup
	var indexErr error
	var indexErrOnce sync.Once
	fail := func(err error) { indexErrOnce.Do(func() { indexErr = err }) }

	for x := 0; x < 3; x++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for docCount := 0; docCount < stopIterations; docCount++ {
				doc := document.NewDocument()
				sf, err := document.NewStringField("dc", strconv.Itoa(docCount), true)
				if err != nil {
					fail(err)
					return
				}
				doc.Add(sf)
				tf, err := document.NewTextField("field", "here is some text", true)
				if err != nil {
					fail(err)
					return
				}
				doc.Add(tf)
				if err := w.AddDocument(doc); err != nil {
					fail(err)
					return
				}
				if docCount%13 == 0 {
					if err := w.Commit(); err != nil {
						fail(err)
						return
					}
				}
			}
		}()
	}

	allFiles := make(map[string]struct{})

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	for iterations := 1; iterations < stopIterations; iterations++ {
		ic := r.GetIndexCommit()
		if ic == nil {
			t.Fatal("GetIndexCommit returned nil")
		}
		files, err := ic.GetFileNames()
		if err != nil {
			t.Fatalf("GetFileNames: %v", err)
		}
		for _, name := range files {
			allFiles[name] = struct{}{}
		}
		// Make sure no old files were removed.
		for name := range allFiles {
			if !dir.FileExists(name) {
				t.Fatalf("file %s does not exist", name)
			}
		}
		r2, err := index.OpenIfChanged(r)
		if err != nil {
			t.Fatalf("OpenIfChanged: %v", err)
		}
		if r2 != nil {
			if err := r.Close(); err != nil {
				t.Fatalf("close stale reader: %v", err)
			}
			r = r2.(*index.DirectoryReader)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	wg.Wait()
	if indexErr != nil {
		t.Fatalf("indexing goroutine failed: %v", indexErr)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
}
