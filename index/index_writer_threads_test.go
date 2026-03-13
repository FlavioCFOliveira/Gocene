// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterWithThreads tests IndexWriter with multiple threads.
// Ported from: TestIndexWriter.testAddDocumentWithThreads()
func TestIndexWriterWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	numThreads := 4
	numDocsPerThread := 100
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < numDocsPerThread; j++ {
				doc := &testDocument{fields: []interface{}{}}
				if err := writer.AddDocument(doc); err != nil {
					t.Errorf("Thread %d failed to add document %d: %v", threadID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	if writer.NumDocs() != numThreads*numDocsPerThread {
		t.Errorf("NumDocs() = %d, want %d", writer.NumDocs(), numThreads*numDocsPerThread)
	}

	if err := writer.Commit(); err != nil {
		t.Errorf("Commit() error = %v", err)
	}

	if writer.NumDocs() != numThreads*numDocsPerThread {
		t.Errorf("NumDocs() after commit = %d, want %d", writer.NumDocs(), numThreads*numDocsPerThread)
	}

	writer.Close()
}

// TestIndexWriter_ConcurrentCommits tests concurrent commit operations.
func TestIndexWriter_ConcurrentCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	numThreads := 3
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				doc := &testDocument{fields: []interface{}{}}
				writer.AddDocument(doc)
				if err := writer.Commit(); err != nil {
					t.Errorf("Thread %d failed to commit: %v", threadID, err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestIndexWriter_ConcurrentClose tests concurrent close calls.
func TestIndexWriter_ConcurrentClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)

	numThreads := 5
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer.Close()
		}()
	}

	wg.Wait()

	if !writer.IsClosed() {
		t.Error("writer should be closed")
	}
}

// TestIndexWriter_ConcurrentCloseDuringIndexing tests calling Close() while threads are indexing.
func TestIndexWriter_ConcurrentCloseDuringIndexing(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)

	numThreads := 4
	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startSignal
			for {
				doc := &testDocument{fields: []interface{}{}}
				err := writer.AddDocument(doc)
				if err != nil {
					// Expect AlreadyClosedException eventually
					return
				}
			}
		}()
	}

	close(startSignal)
	// Let them index for a bit
	time.Sleep(10 * time.Millisecond)

	// Close while they are indexing
	if err := writer.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	wg.Wait()

	if !writer.IsClosed() {
		t.Error("Writer should be closed")
	}
}

// TestIndexWriter_UpdateDocumentsWithThreads tests concurrent updates.
func TestIndexWriter_UpdateDocumentsWithThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	numThreads := 4
	numDocsPerThread := 50
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < numDocsPerThread; j++ {
				term := index.NewTerm("id", fmt.Sprintf("%d-%d", threadID, j))
				doc := &testDocument{fields: []interface{}{}}
				if err := writer.UpdateDocument(term, doc); err != nil {
					t.Errorf("Thread %d failed to update document %d: %v", threadID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
}
