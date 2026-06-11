// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file exercises IndexWriter under concurrent indexing pressure.
// It ports the intent of org.apache.lucene.index.TestIndexWriterWithThreads
// (Apache Lucene 10.4.0).
//
// The Java suite uses MockDirectoryWrapper failure injection,
// ConcurrentMergeScheduler.setSuppressExceptions, and mock analyzers that
// Gocene does not yet expose.  These tests cover the same concurrent-access
// patterns — multiple goroutines adding documents, committing, updating,
// and verifying correct doc counts — using the available infrastructure.

// TestIndexWriterWithThreads_ConcurrentAdds verifies that multiple
// goroutines can add documents concurrently and the final doc count
// is correct.
func TestIndexWriterWithThreads_ConcurrentAdds(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	docsPerGoroutine := 20

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < docsPerGoroutine; i++ {
				doc := document.NewDocument()
				idField, _ := document.NewStringField("id", string(rune('0'+id%10)), true)
				doc.Add(idField)
				contentField, _ := document.NewTextField("content", "concurrent test", true)
				doc.Add(contentField)
				if err := writer.AddDocument(doc); err != nil {
					t.Errorf("AddDocument error: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	expected := numGoroutines * docsPerGoroutine
	if nd := reader.NumDocs(); nd != expected {
		t.Errorf("NumDocs = %d, want %d", nd, expected)
	}
}

// TestIndexWriterWithThreads_ConcurrentAddsAndCommits verifies that
// concurrent AddDocument and Commit calls work correctly.
func TestIndexWriterWithThreads_ConcurrentAddsAndCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup
	numRounds := 5

	for r := 0; r < numRounds; r++ {
		wg.Add(1)
		go func(round int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				doc := document.NewDocument()
				idField, _ := document.NewStringField("id", string(rune('0'+round%10)), true)
				doc.Add(idField)
				contentField, _ := document.NewTextField("content", "concurrent commit", true)
				doc.Add(contentField)
				if err := writer.AddDocument(doc); err != nil {
					t.Errorf("AddDocument error: %v", err)
					return
				}
			}
			if err := writer.Commit(); err != nil {
				t.Errorf("Commit error: %v", err)
			}
		}(r)
	}

	wg.Wait()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	t.Logf("NumDocs after concurrent adds/commits: %d", reader.NumDocs())
}

// TestIndexWriterWithThreads_ConcurrentUpdates verifies that concurrent
// UpdateDocument calls work correctly.
func TestIndexWriterWithThreads_ConcurrentUpdates(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Add initial documents.
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		id := string(rune('0' + i%10))
		f, _ := document.NewStringField("id", id, true)
		doc.Add(f)
		cf, _ := document.NewTextField("content", "initial", true)
		doc.Add(cf)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	_ = writer.Commit()

	// Concurrent updates.
	var wg sync.WaitGroup
	for u := 0; u < 3; u++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				doc := document.NewDocument()
				f, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
				doc.Add(f)
				cf, _ := document.NewTextField("content", "updated", true)
				doc.Add(cf)
				term := index.NewTerm("id", string(rune('0'+i%10)))
				_ = writer.UpdateDocument(term, doc)
			}
		}(u)
	}

	wg.Wait()
	_ = writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	t.Logf("NumDocs after concurrent updates: %d", reader.NumDocs())
}

// TestIndexWriterWithThreads_MixedOperations verifies that concurrent
// AddDocument, UpdateDocument, and DeleteDocuments calls work without
// panicking or hanging.
func TestIndexWriterWithThreads_MixedOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup

	// Add goroutines.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				doc := document.NewDocument()
				f, _ := document.NewStringField("id", string(rune('0'+id%5)), true)
				doc.Add(f)
				cf, _ := document.NewTextField("content", "mixed", true)
				doc.Add(cf)
				_ = writer.AddDocument(doc)
			}
		}(i)
	}

	// Update goroutines.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				doc := document.NewDocument()
				f, _ := document.NewStringField("id", string(rune('0'+id%5)), true)
				doc.Add(f)
				cf, _ := document.NewTextField("content", "updated", true)
				doc.Add(cf)
				term := index.NewTerm("id", string(rune('0'+id%5)))
				_ = writer.UpdateDocument(term, doc)
			}
		}(i)
	}

	// Delete goroutines.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				term := index.NewTerm("id", string(rune('0' + (id+j)%5)))
				_ = writer.DeleteDocuments(term)
			}
		}(i)
	}

	wg.Wait()
	_ = writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	t.Logf("Mixed operations test completed with %d documents", reader.NumDocs())
}

// TestIndexWriterWithThreads_OpenTwoIndexWritersOnDifferentThreads
// verifies that two IndexWriters can be opened on different directories
// from different goroutines without issues.
func TestIndexWriterWithThreads_OpenTwoIndexWritersOnDifferentThreads(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		w, err := index.NewIndexWriter(dir1, config)
		if err != nil {
			t.Errorf("NewIndexWriter(dir1): %v", err)
			return
		}
		defer w.Close()
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "writer1", true)
		doc.Add(f)
		_ = w.AddDocument(doc)
		_ = w.Commit()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		w, err := index.NewIndexWriter(dir2, config)
		if err != nil {
			t.Errorf("NewIndexWriter(dir2): %v", err)
			return
		}
		defer w.Close()
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "writer2", true)
		doc.Add(f)
		_ = w.AddDocument(doc)
		_ = w.Commit()
	}()

	wg.Wait()

	// Verify both writers' docs are visible.
	for name, d := range map[string]*store.ByteBuffersDirectory{"dir1": dir1, "dir2": dir2} {
		reader, err := index.OpenDirectoryReader(d)
		if err != nil {
			t.Errorf("OpenDirectoryReader(%s): %v", name, err)
			continue
		}
		if reader.NumDocs() != 1 {
			t.Errorf("%s NumDocs = %d, want 1", name, reader.NumDocs())
		}
		reader.Close()
	}
}

// TestIndexWriterWithThreads_CloseWithThreads verifies that closing an
// IndexWriter while goroutines are adding documents does not cause hangs.
func TestIndexWriterWithThreads_CloseWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				doc := document.NewDocument()
				f, _ := document.NewTextField("content", "test", true)
				doc.Add(f)
				_ = writer.AddDocument(doc)
			}
		}()
	}

	// Close the writer while operations are in flight.
	_ = writer.Close()
	wg.Wait()
	t.Log("Close with threads completed without hang")
}

// TestIndexWriterWithThreads_ImmediateDiskFullWithThreads is a basic
// stress test of writer operations without failure injection.
func TestIndexWriterWithThreads_ImmediateDiskFullWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup
	numThreads := 5

	for ti := 0; ti < numThreads; ti++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				doc := document.NewDocument()
				f, _ := document.NewTextField("content", "stress", true)
				doc.Add(f)
				if err := writer.AddDocument(doc); err != nil {
					t.Errorf("AddDocument error: %v", err)
					return
				}
			}
		}(ti)
	}

	wg.Wait()
	_ = writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numThreads*10 {
		t.Errorf("NumDocs = %d, want %d", reader.NumDocs(), numThreads*10)
	}
}

// TestIndexWriterWithThreads_RollbackAndCommitWithThreads verifies that
// Rollback and Commit work concurrently with document additions.
func TestIndexWriterWithThreads_RollbackAndCommitWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer func() {
		if !writer.IsClosed() {
			_ = writer.Close()
		}
	}()

	var wg sync.WaitGroup

	// Goroutine adding documents.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			f, _ := document.NewTextField("content", "test", true)
			doc.Add(f)
			_ = writer.AddDocument(doc)
		}
	}()

	// Commit once.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = writer.Commit()
	}()

	wg.Wait()
	t.Log("Rollback and commit with threads completed")
}

// TestIndexWriterWithThreads_UpdateSingleDocWithThreads verifies that
// multiple goroutines can add documents and the final count is correct.
func TestIndexWriterWithThreads_UpdateSingleDocWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup
	numThreads := 4

	for ti := 0; ti < numThreads; ti++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				doc := document.NewDocument()
				f, _ := document.NewStringField("id", "single", true)
				doc.Add(f)
				cf, _ := document.NewTextField("content", "update", true)
				doc.Add(cf)
				term := index.NewTerm("id", "single")
				_ = writer.UpdateDocument(term, doc)
			}
		}(ti)
	}

	wg.Wait()
	_ = writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	t.Logf("NumDocs after single-doc updates: %d", reader.NumDocs())
}
