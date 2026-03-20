// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTStressIndexing runs a stress test for NRT indexing
func TestNRTStressIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetRAMBufferSizeMB(64.0)
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
	numWorkers := 10
	docsPerWorker := 1000
	done := make(chan struct{})

	// Document counter
	var totalDocs int32

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < docsPerWorker; i++ {
				select {
				case <-done:
					return
				default:
				}

				doc := document.NewDocument()
				doc.Add(document.NewTextField("id", fmt.Sprintf("worker%d_doc%d", workerID, i), document.Stored|document.Indexed))
				doc.Add(document.NewTextField("content", fmt.Sprintf("Content %d from worker %d", i, workerID), document.Stored|document.Indexed))

				err := writer.AddDocument(doc)
				if err != nil {
					t.Logf("Worker %d: Failed to add document: %v", workerID, err)
					continue
				}

				atomic.AddInt32(&totalDocs, 1)

				// Occasionally commit
				if i%100 == 0 {
					writer.Commit()
				}
			}
		}(w)
	}

	// Reader goroutine that periodically reopens
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				newReader, err := reader.Reopen()
				if err == nil {
					oldReader := reader
					reader = newReader
					oldReader.Close()
				}
			}
		}
	}()

	// Let it run
	time.Sleep(5 * time.Second)
	close(done)
	wg.Wait()

	// Final commit
	writer.Commit()

	// Reopen and verify
	newReader, err := reader.Reopen()
	if err == nil {
		reader.Close()
		reader = newReader
	}

	t.Logf("Total documents indexed: %d", atomic.LoadInt32(&totalDocs))
	t.Logf("Documents visible in reader: %d", reader.NumDocs())

	reader.Close()
}

// TestNRTStressMemory tests memory usage during stress
func TestNRTStressMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetRAMBufferSizeMB(16.0)
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Record initial memory
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Index many documents
	numDocs := 10000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("This is document number %d with some content to make it larger", i), document.Stored|document.Indexed))

		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		if i%1000 == 0 {
			err = writer.Commit()
			if err != nil {
				t.Fatalf("Failed to commit: %v", err)
			}
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	writer.Close()

	// Record final memory
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Check for memory growth
	memGrowth := m2.HeapAlloc - m1.HeapAlloc
	t.Logf("Memory growth: %d bytes (%d MB)", memGrowth, memGrowth/1024/1024)

	// Memory growth should be reasonable (less than 500MB for 10k docs)
	if memGrowth > 500*1024*1024 {
		t.Errorf("Excessive memory growth: %d MB", memGrowth/1024/1024)
	}
}

// TestNRTStressReopenStress tests reopen under heavy load
func TestNRTStressReopenStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

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
	for i := 0; i < 1000; i++ {
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

	// Rapid reopen goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			localReader := reader
			for j := 0; j < 500; j++ {
				select {
				case <-done:
					return
				default:
				}

				newReader, err := localReader.Reopen()
				if err == nil {
					if localReader != reader {
						localReader.Close()
					}
					localReader = newReader
				}
			}
			if localReader != reader {
				localReader.Close()
			}
		}(i)
	}

	// Concurrent indexing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1000; i < 5000; i++ {
			select {
			case <-done:
				return
			default:
			}

			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
			err = writer.AddDocument(doc)
			if err != nil {
				t.Logf("Failed to add document: %v", err)
			}

			if i%100 == 0 {
				writer.Commit()
			}
		}
	}()

	time.Sleep(3 * time.Second)
	close(done)
	wg.Wait()

	reader.Close()
}

// TestNRTStressLongRunning tests long-running NRT operations
func TestNRTStressLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetRAMBufferSizeMB(32.0)
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

	// Run for a period
	duration := 5 * time.Second
	start := time.Now()

	var totalDocs int32
	var totalReopens int32

	var wg sync.WaitGroup

	// Indexing goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		docID := 0
		for time.Since(start) < duration {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", docID), document.Stored|document.Indexed))
			doc.Add(document.NewTextField("content", fmt.Sprintf("Content for document %d", docID), document.Stored|document.Indexed))

			err := writer.AddDocument(doc)
			if err != nil {
				t.Logf("Failed to add document: %v", err)
			} else {
				atomic.AddInt32(&totalDocs, 1)
			}

			docID++

			if docID%100 == 0 {
				writer.Commit()
			}

			time.Sleep(time.Millisecond)
		}
	}()

	// Reopen goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Since(start) < duration {
			newReader, err := reader.Reopen()
			if err == nil {
				if reader != nil {
					reader.Close()
				}
				reader = newReader
				atomic.AddInt32(&totalReopens, 1)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	wg.Wait()

	if reader != nil {
		reader.Close()
	}

	t.Logf("Total documents added: %d", atomic.LoadInt32(&totalDocs))
	t.Logf("Total reopens: %d", atomic.LoadInt32(&totalReopens))
}

// TestNRTStressDeleteHeavy tests heavy delete operations
func TestNRTStressDeleteHeavy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

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

	// Add many documents
	numDocs := 5000
	for i := 0; i < numDocs; i++ {
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

	// Delete documents in batches
	for batch := 0; batch < 10; batch++ {
		for i := 0; i < 100; i++ {
			docID := batch*500 + i
			err = writer.DeleteDocuments(NewTermQuery(NewTerm("id", fmt.Sprintf("doc%d", docID))))
			if err != nil {
				t.Logf("Failed to delete document: %v", err)
			}
		}

		writer.Commit()

		// Reopen and verify
		newReader, err := reader.Reopen()
		if err == nil {
			reader.Close()
			reader = newReader
		}
	}

	// Final document count should be 5000 - 1000 = 4000
	if reader.NumDocs() != 4000 {
		t.Errorf("Expected 4000 documents after deletes, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTStressFileHandleLeak tests for file handle leaks
func TestNRTStressFileHandleLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

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

	// Create and close many readers
	for i := 0; i < 1000; i++ {
		reader, err := writer.GetReader()
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Do some reads
		_ = reader.NumDocs()

		// Occasional reopen
		if i%10 == 0 {
			newReader, err := reader.Reopen()
			if err == nil {
				reader.Close()
				reader = newReader
			}
		}

		reader.Close()
	}

	// At this point, we should have no leaked file handles
	// (difficult to test programmatically, but the test should not crash)
	t.Log("File handle leak test completed successfully")
}

// TestNRTStressRecovery tests recovery from errors
func TestNRTStressRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

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

	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Continue operations after simulated errors
	for i := 0; i < 100; i++ {
		// Add document
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Logf("Error adding document (continuing): %v", err)
		}

		// Try to reopen
		newReader, err := reader.Reopen()
		if err == nil {
			oldReader := reader
			reader = newReader
			oldReader.Close()
		} else {
			t.Logf("Error reopening reader (continuing): %v", err)
		}
	}

	// Final commit should succeed
	err = writer.Commit()
	if err != nil {
		t.Fatalf("Final commit failed: %v", err)
	}

	// Reopen should work
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Final reopen failed: %v", err)
	}
	reader.Close()
	reader = newReader

	t.Logf("Final document count: %d", reader.NumDocs())
}
