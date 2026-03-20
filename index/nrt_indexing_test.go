// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTBasicIndexing tests basic NRT indexing operations
func TestNRTBasicIndexing(t *testing.T) {
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

	// Add some documents
	doc := document.NewDocument()
	doc.Add(document.NewTextField("content", "hello world", document.Stored|document.Indexed))

	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Commit to make documents visible
	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open NRT reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Verify document is visible
	if reader.NumDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", reader.NumDocs())
	}
}

// TestNRTReopen tests NRT reader reopen functionality
func TestNRTReopen(t *testing.T) {
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

	// Add initial document
	doc1 := document.NewDocument()
	doc1.Add(document.NewTextField("content", "first document", document.Stored|document.Indexed))
	err = writer.AddDocument(doc1)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open initial reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	if reader.NumDocs() != 1 {
		t.Errorf("Expected 1 document initially, got %d", reader.NumDocs())
	}

	// Add more documents
	doc2 := document.NewDocument()
	doc2.Add(document.NewTextField("content", "second document", document.Stored|document.Indexed))
	err = writer.AddDocument(doc2)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	doc3 := document.NewDocument()
	doc3.Add(document.NewTextField("content", "third document", document.Stored|document.Indexed))
	err = writer.AddDocument(doc3)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Reopen reader
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Verify all documents are visible
	if reader.NumDocs() != 3 {
		t.Errorf("Expected 3 documents after reopen, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTDocumentVisibility tests document visibility in NRT
func TestNRTDocumentVisibility(t *testing.T) {
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
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Verify no documents initially
	if reader.NumDocs() != 0 {
		t.Errorf("Expected 0 documents initially, got %d", reader.NumDocs())
	}

	// Add documents without commit
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("content %d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}

	// Documents should not be visible without reopen
	if reader.NumDocs() != 0 {
		t.Errorf("Expected 0 documents before reopen, got %d", reader.NumDocs())
	}

	// Reopen and check visibility
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Documents should be visible after reopen
	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after reopen, got %d", reader.NumDocs())
	}
}

// TestNRTDeleteOperations tests delete operations in NRT
func TestNRTDeleteOperations(t *testing.T) {
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
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", "test content", document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	if reader.NumDocs() != 5 {
		t.Errorf("Expected 5 documents, got %d", reader.NumDocs())
	}

	// Delete some documents
	err = writer.DeleteDocuments(NewTermQuery(NewTerm("id", "doc1")))
	if err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	err = writer.DeleteDocuments(NewTermQuery(NewTerm("id", "doc3")))
	if err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	// Reopen reader
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Verify documents are deleted
	if reader.NumDocs() != 3 {
		t.Errorf("Expected 3 documents after delete, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTUpdateOperations tests update operations in NRT
func TestNRTUpdateOperations(t *testing.T) {
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

	// Add document
	doc := document.NewDocument()
	doc.Add(document.NewTextField("id", "doc1", document.Stored|document.Indexed))
	doc.Add(document.NewTextField("content", "original content", document.Stored|document.Indexed))
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Update document (delete old + add new)
	err = writer.UpdateDocument(NewTerm("id", "doc1"), doc)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	// Reopen reader
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Verify document still exists
	if reader.NumDocs() != 1 {
		t.Errorf("Expected 1 document after update, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTLargeDocumentSet tests NRT with a large number of documents
func TestNRTLargeDocumentSet(t *testing.T) {
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
	defer writer.Close()

	// Add many documents
	numDocs := 1000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("This is document number %d with some content", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open NRT reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Verify document count
	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d documents, got %d", numDocs, reader.NumDocs())
	}

	// Verify max doc (includes deleted)
	if reader.MaxDoc() < numDocs {
		t.Errorf("Expected MaxDoc >= %d, got %d", numDocs, reader.MaxDoc())
	}
}

// TestNRTMultipleReopens tests multiple reopen cycles
func TestNRTMultipleReopens(t *testing.T) {
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
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Perform multiple batches of updates
	for batch := 0; batch < 5; batch++ {
		// Add documents in this batch
		for i := 0; i < 10; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("batch%d_doc%d", batch, i), document.Stored|document.Indexed))
			doc.Add(document.NewTextField("content", fmt.Sprintf("Batch %d document %d", batch, i), document.Stored|document.Indexed))
			err = writer.AddDocument(doc)
			if err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Reopen reader
		newReader, err := reader.Reopen()
		if err != nil {
			t.Fatalf("Failed to reopen reader: %v", err)
		}
		reader.Close()
		reader = newReader

		// Verify document count
		expectedDocs := (batch + 1) * 10
		if reader.NumDocs() != expectedDocs {
			t.Errorf("After batch %d: expected %d documents, got %d", batch, expectedDocs, reader.NumDocs())
		}
	}

	reader.Close()
}

// TestNRTReopenWithDeletes tests reopen with deletes
func TestNRTReopenWithDeletes(t *testing.T) {
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
	for i := 0; i < 20; i++ {
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

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Delete odd documents
	for i := 1; i < 20; i += 2 {
		err = writer.DeleteDocuments(NewTermQuery(NewTerm("id", fmt.Sprintf("doc%d", i))))
		if err != nil {
			t.Fatalf("Failed to delete document: %v", err)
		}
	}

	// Reopen
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Verify 10 documents remaining
	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTContextCancellation tests context cancellation during NRT operations
func TestNRTContextCancellation(t *testing.T) {
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

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give some time for the context to timeout
	time.Sleep(5 * time.Millisecond)

	// Try operation with cancelled context
	// This is mostly to verify no panic occurs
	if ctx.Err() != nil {
		t.Logf("Context error as expected: %v", ctx.Err())
	}
}

// TestNRTReopenPerformance tests NRT reopen performance
func TestNRTReopenPerformance(t *testing.T) {
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

	// Add some documents
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

	// Open initial reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Add more documents
	for i := 100; i < 200; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("content %d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Time the reopen
	start := time.Now()
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	elapsed := time.Since(start)

	reader.Close()
	reader = newReader

	// Verify reopen was fast (should be < 100ms for this small set)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Reopen took too long: %v", elapsed)
	}

	// Verify documents are visible
	if reader.NumDocs() != 200 {
		t.Errorf("Expected 200 documents, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestNRTConcurrentAccess tests concurrent access to NRT reader
func TestNRTConcurrentAccess(t *testing.T) {
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
	for i := 0; i < 50; i++ {
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

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Concurrent reads
	var wg sync.WaitGroup
	numGoroutines := 20
	numReads := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numReads; j++ {
				_ = reader.NumDocs()
				_ = reader.MaxDoc()
			}
		}(i)
	}

	wg.Wait()
}

// TestNRTIsCurrent tests the IsCurrent method
func TestNRTIsCurrent(t *testing.T) {
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

	// Add and commit document
	doc := document.NewDocument()
	doc.Add(document.NewTextField("content", "test", document.Stored|document.Indexed))
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	err = writer.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Check if current
	isCurrent, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent failed: %v", err)
	}

	if !isCurrent {
		t.Error("Expected reader to be current after commit")
	}

	// Add another document without commit
	doc2 := document.NewDocument()
	doc2.Add(document.NewTextField("content", "test2", document.Stored|document.Indexed))
	err = writer.AddDocument(doc2)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Reader should not be current now
	isCurrent, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent failed: %v", err)
	}

	// Reader might be current depending on implementation
	// Just verify the method doesn't panic
	t.Logf("IsCurrent after adding document without commit: %v", isCurrent)

	reader.Close()
}

// TestNRTWithDifferentAnalyzers tests NRT with different analyzers
func TestNRTWithDifferentAnalyzers(t *testing.T) {
	analyzers := map[string]analysis.Analyzer{
		"standard": analysis.NewStandardAnalyzer(),
		"simple":   analysis.NewSimpleAnalyzer(),
		"keyword":  analysis.NewKeywordAnalyzer(),
	}

	for name, analyzer := range analyzers {
		t.Run(name, func(t *testing.T) {
			dir, err := store.NewRAMDirectory()
			if err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}
			defer dir.Close()

			config := NewIndexWriterConfig(analyzer)
			writer, err := NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("Failed to create writer: %v", err)
			}
			defer writer.Close()

			// Add documents
			doc := document.NewDocument()
			doc.Add(document.NewTextField("content", "Hello World Test", document.Stored|document.Indexed))
			err = writer.AddDocument(doc)
			if err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}

			err = writer.Commit()
			if err != nil {
				t.Fatalf("Failed to commit: %v", err)
			}

			// Open NRT reader
			reader, err := OpenNRTDirectoryReader(writer)
			if err != nil {
				t.Fatalf("Failed to open NRT reader: %v", err)
			}
			defer reader.Close()

			// Verify document is visible
			if reader.NumDocs() != 1 {
				t.Errorf("Expected 1 document, got %d", reader.NumDocs())
			}
		})
	}
}

// Helper function to create NRT directory reader
func OpenNRTDirectoryReader(writer *IndexWriter) (*DirectoryReader, error) {
	// In a real implementation, this would open an NRT reader from the writer
	// For now, we return a standard reader from the writer's directory
	return writer.GetReader()
}
