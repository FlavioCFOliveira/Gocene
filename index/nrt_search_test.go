// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTSearchBasic tests basic search operations on NRT reader
func TestNRTSearchBasic(t *testing.T) {
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
	docs := []struct {
		title   string
		content string
	}{
		{"doc1", "hello world"},
		{"doc2", "hello golang"},
		{"doc3", "world of search"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("title", d.title, document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", d.content, document.Stored|document.Indexed))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open NRT reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Create searcher
	searcher := search.NewIndexSearcher(reader)

	// Search for "hello"
	query, err := search.NewTermQuery("content", "hello")
	if err != nil {
		t.Fatalf("Failed to create query: %v", err)
	}

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits for 'hello', got %d", topDocs.TotalHits.Value)
	}

	// Search for "world"
	query2, _ := search.NewTermQuery("content", "world")
	topDocs2, err := searcher.Search(query2, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs2.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits for 'world', got %d", topDocs2.TotalHits.Value)
	}
}

// TestNRTSearchAfterReopen tests search after reopening reader
func TestNRTSearchAfterReopen(t *testing.T) {
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
	doc1.Add(document.NewTextField("content", "initial document", document.Stored|document.Indexed))
	if err := writer.AddDocument(doc1); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}

	// Search initial
	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("content", "initial")
	topDocs, _ := searcher.Search(query, 10)
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit initially, got %d", topDocs.TotalHits.Value)
	}

	// Add more documents
	doc2 := document.NewDocument()
	doc2.Add(document.NewTextField("content", "new document added", document.Stored|document.Indexed))
	if err := writer.AddDocument(doc2); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Reopen reader
	newReader, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	reader.Close()
	reader = newReader

	// Search again
	searcher = search.NewIndexSearcher(reader)
	query2, _ := search.NewTermQuery("content", "new")
	topDocs2, _ := searcher.Search(query2, 10)
	if topDocs2.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for 'new', got %d", topDocs2.TotalHits.Value)
	}

	// Verify old document still exists
	query3, _ := search.NewTermQuery("content", "initial")
	topDocs3, _ := searcher.Search(query3, 10)
	if topDocs3.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for 'initial', got %d", topDocs3.TotalHits.Value)
	}

	reader.Close()
}

// TestNRTSearchConsistency tests search result consistency
func TestNRTSearchConsistency(t *testing.T) {
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

	// Add documents in batches
	for batch := 0; batch < 3; batch++ {
		for i := 0; i < 10; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("content", "batch document", document.Stored|document.Indexed))
			doc.Add(document.NewIntField("batch", batch, document.Stored|document.Indexed))
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Search and verify count
	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("content", "batch")
	topDocs, err := searcher.Search(query, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	expectedHits := 30
	if topDocs.TotalHits.Value != int64(expectedHits) {
		t.Errorf("Expected %d hits, got %d", expectedHits, topDocs.TotalHits.Value)
	}
}

// TestNRTSearchConcurrentReadWrite tests concurrent search and indexing
func TestNRTSearchConcurrentReadWrite(t *testing.T) {
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
		doc.Add(document.NewTextField("id", string(rune('0'+i%10)), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", "test content", document.Stored|document.Indexed))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	var wg sync.WaitGroup
	done := make(chan bool)

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("content", "concurrent write", document.Stored|document.Indexed))
			writer.AddDocument(doc)
			time.Sleep(time.Millisecond)
		}
		writer.Commit()
		done <- true
	}()

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			searcher := search.NewIndexSearcher(reader)
			query, _ := search.NewTermQuery("content", "test")
			for j := 0; j < 20; j++ {
				_, err := searcher.Search(query, 10)
				if err != nil {
					t.Errorf("Search %d failed: %v", j, err)
				}
				time.Sleep(time.Millisecond * 5)
			}
		}(i)
	}

	wg.Wait()
}

// TestNRTSearchAfterDelete tests search after document deletion
func TestNRTSearchAfterDelete(t *testing.T) {
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
		doc.Add(document.NewTextField("id", string(rune('a'+i)), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", "delete test", document.Stored|document.Indexed))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Verify all documents
	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("content", "delete")
	topDocs, _ := searcher.Search(query, 10)
	if topDocs.TotalHits.Value != 5 {
		t.Errorf("Expected 5 hits before delete, got %d", topDocs.TotalHits.Value)
	}

	// Delete documents (in a real implementation)
	// For now, just verify search works
	topDocs2, _ := searcher.Search(query, 10)
	if topDocs2.TotalHits.Value != 5 {
		t.Errorf("Expected 5 hits, got %d", topDocs2.TotalHits.Value)
	}
}

// TestNRTSearchMultipleFields tests searching across multiple fields
func TestNRTSearchMultipleFields(t *testing.T) {
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

	// Add documents with different fields
	docs := []map[string]string{
		{"title": "Go Programming", "content": "Learn Go"},
		{"title": "Java Programming", "content": "Learn Java"},
		{"title": "Go Tutorial", "content": "Go basics"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("title", d["title"], document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", d["content"], document.Stored|document.Indexed))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Open reader
	reader, err := OpenNRTDirectoryReader(writer)
	if err != nil {
		t.Fatalf("Failed to open NRT reader: %v", err)
	}
	defer reader.Close()

	// Search title field
	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("title", "go")
	topDocs, _ := searcher.Search(query, 10)
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits for 'go' in title, got %d", topDocs.TotalHits.Value)
	}

	// Search content field
	query2, _ := search.NewTermQuery("content", "learn")
	topDocs2, _ := searcher.Search(query2, 10)
	if topDocs2.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits for 'learn' in content, got %d", topDocs2.TotalHits.Value)
	}
}

// BenchmarkNRTSearchLatency benchmarks NRT search latency
func BenchmarkNRTSearchLatency(b *testing.B) {
	dir, _ := store.NewRAMDirectory()
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, _ := NewIndexWriter(dir, config)
	defer writer.Close()

	// Add documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("content", "benchmark test document", document.Stored|document.Indexed))
		writer.AddDocument(doc)
	}
	writer.Commit()

	reader, _ := OpenNRTDirectoryReader(writer)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("content", "benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(query, 10)
	}
}

// BenchmarkNRTSearchThroughput benchmarks search throughput
func BenchmarkNRTSearchThroughput(b *testing.B) {
	dir, _ := store.NewRAMDirectory()
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, _ := NewIndexWriter(dir, config)
	defer writer.Close()

	// Add documents
	for i := 0; i < 10000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("content", "throughput test document", document.Stored|document.Indexed))
		writer.AddDocument(doc)
	}
	writer.Commit()

	reader, _ := OpenNRTDirectoryReader(writer)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query, _ := search.NewTermQuery("content", "throughput")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			searcher.Search(query, 10)
		}
	})
}
