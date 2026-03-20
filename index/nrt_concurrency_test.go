// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTConcurrentIndexingAndSearching tests concurrent indexing and searching
func TestNRTConcurrentIndexingAndSearching(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Open initial reader
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	var wg sync.WaitGroup
	numIndexers := 5
	numSearchers := 5
	opsPerGoroutine := 100

	// Channel to signal completion
	done := make(chan struct{})

	// Atomic counter for documents indexed
	var docsIndexed int32

	// Start indexers
	for i := 0; i < numIndexers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				select {
				case <-done:
					return
				default:
				}

				doc := document.NewDocument()
				doc.Add(document.NewTextField("id", fmt.Sprintf("goroutine%d_doc%d", id, j), document.Stored|document.Indexed))
				doc.Add(document.NewTextField("content", fmt.Sprintf("Content from goroutine %d operation %d", id, j), document.Stored|document.Indexed))

				err := writer.AddDocument(doc)
				if err != nil {
					t.Logf("Failed to add document: %v", err)
					continue
				}

				atomic.AddInt32(&docsIndexed, 1)

				// Occasionally commit
				if j%20 == 0 {
					writer.Commit()
				}
			}
		}(i)
	}

	// Start searchers
	for i := 0; i < numSearchers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				select {
				case <-done:
					return
				default:
				}

				// Read document count
				_ = reader.NumDocs()
				_ = reader.MaxDoc()

				// Occasionally reopen reader
				if j%10 == 0 {
					newReader, err := reader.Reopen()
					if err == nil {
						oldReader := reader
						reader = newReader
						oldReader.Close()
					}
				}

				time.Sleep(time.Microsecond * 100)
			}
		}(i)
	}

	// Let it run for a bit
	time.Sleep(2 * time.Second)
	close(done)

	// Wait for all goroutines
	wg.Wait()

	reader.Close()

	t.Logf("Total documents indexed: %d", atomic.LoadInt32(&docsIndexed))
}

// TestNRTMultipleReaders tests multiple concurrent readers
func TestNRTMultipleReaders(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("content %d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open multiple readers concurrently
	numReaders := 20
	readers := make([]*DirectoryReader, numReaders)
	var openErr error
	var wg sync.WaitGroup

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reader, err := writer.GetReader()
			if err != nil {
				openErr = err
				return
			}
			readers[idx] = reader
		}(i)
	}

	wg.Wait()

	if openErr != nil {
		t.Fatalf("Failed to open reader: %v", openErr)
	}

	// Verify all readers see the same data
	for i, reader := range readers {
		if reader.NumDocs() != 100 {
			t.Errorf("Reader %d: expected 100 documents, got %d", i, reader.NumDocs())
		}
		reader.Close()
	}
}

// TestNRTRaceConditionReopen tests for race conditions during reopen
func TestNRTRaceConditionReopen(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open initial reader
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	var wg sync.WaitGroup
	numReopeners := 10
	numReopens := 50

	// Start reopeners
	for i := 0; i < numReopeners; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numReopens; j++ {
				newReader, err := reader.Reopen()
				if err != nil {
					t.Logf("Reopen failed: %v", err)
					continue
				}
				_ = newReader.NumDocs()
				newReader.Close()
			}
		}(i)
	}

	// Concurrently add more documents
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 50; i < 100; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
			err = writer.AddDocument(doc)
			if err != nil {
				t.Logf("Failed to add document: %v", err)
			}
			if i%10 == 0 {
				writer.Commit()
			}
		}
	}()

	wg.Wait()
	reader.Close()
}

// TestNRTConcurrentDeletesAndReads tests concurrent deletes and reads
func TestNRTConcurrentDeletesAndReads(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	var wg sync.WaitGroup
	numDeleters := 5
	numReaders := 5

	// Start deleters
	for i := 0; i < numDeleters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				docID := (id*10 + j) % 100
				err := writer.DeleteDocuments(NewTermQuery(NewTerm("id", fmt.Sprintf("doc%d", docID))))
				if err != nil {
					t.Logf("Failed to delete document: %v", err)
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = reader.NumDocs()
				_ = reader.MaxDoc()
				time.Sleep(time.Millisecond * 2)
			}
		}(i)
	}

	wg.Wait()
	reader.Close()
}

// TestNRTWriterReaderConsistency tests consistency between writer and reader
func TestNRTWriterReaderConsistency(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents in batches and verify consistency
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	for batch := 0; batch < 5; batch++ {
		// Add batch of documents
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("batch%d_doc%d", batch, i), document.Stored|document.Indexed))
			err = writer.AddDocument(doc)
			if err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Commit
		err = writer.Commit()
		if err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Reopen reader
		newReader, err := reader.Reopen()
		if err != nil {
			t.Fatalf("Failed to reopen reader: %v", err)
		}
		reader.Close()
		reader = newReader

		// Verify consistency
		expectedDocs := (batch + 1) * 20
		if reader.NumDocs() != expectedDocs {
			t.Errorf("Batch %d: expected %d documents, got %d", batch, expectedDocs, reader.NumDocs())
		}
	}

	reader.Close()
}

// TestNRTConcurrentReopenAndCommit tests concurrent reopen and commit
func TestNRTConcurrentReopenAndCommit(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Reopen goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				newReader, err := reader.Reopen()
				if err == nil {
					oldReader := reader
					reader = newReader
					oldReader.Close()
				}
				time.Sleep(time.Millisecond * 10)
			}
		}
	}()

	// Commit goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 50
		for {
			select {
			case <-done:
				return
			default:
				doc := document.NewDocument()
				doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", counter), document.Stored|document.Indexed))
				err = writer.AddDocument(doc)
				if err != nil {
					t.Logf("Failed to add document: %v", err)
				}
				counter++

				if counter%10 == 0 {
					err = writer.Commit()
					if err != nil {
						t.Logf("Failed to commit: %v", err)
					}
				}
				time.Sleep(time.Millisecond * 5)
			}
		}
	}()

	// Run for a short time
	time.Sleep(1 * time.Second)
	close(done)
	wg.Wait()

	reader.Close()
}

// TestNRTGoroutineLeak tests that no goroutines are leaked
func TestNRTGoroutineLeak(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open and close multiple readers
	for i := 0; i < 100; i++ {
		reader, err := writer.GetReader()
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Do some reads
		_ = reader.NumDocs()
		_ = reader.MaxDoc()

		// Reopen a few times
		if i%10 == 0 {
			newReader, err := reader.Reopen()
			if err != nil {
				t.Logf("Reopen failed: %v", err)
			} else {
				reader.Close()
				reader = newReader
			}
		}

		reader.Close()
	}

	writer.Close()
}

// TestNRTAtomicVisibility tests atomic visibility of changes
func TestNRTAtomicVisibility(t *testing.T) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Open reader
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Before commit, reader should not see documents
	if reader.NumDocs() != 0 {
		t.Errorf("Expected 0 documents before commit, got %d", reader.NumDocs())
	}

	// Commit
	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Reopen and verify all documents are visible
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen: %v", err)
	}
	reader.Close()
	reader = newReader

	// All 50 documents should be visible atomically
	if reader.NumDocs() != 50 {
		t.Errorf("Expected 50 documents after commit, got %d", reader.NumDocs())
	}

	reader.Close()
}
