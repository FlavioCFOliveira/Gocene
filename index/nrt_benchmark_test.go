// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BenchmarkNRTIndexing benchmarks NRT indexing performance
func BenchmarkNRTIndexing(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Open initial reader
	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("This is the content of document %d with some text", i), document.Stored|document.Indexed))

		err := writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}

		if i%100 == 0 {
			writer.Commit()
		}
	}
}

// BenchmarkNRTReopen benchmarks NRT reopen performance
func BenchmarkNRTReopen(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		b.Fatalf("Failed to commit: %v", err)
	}

	// Open initial reader
	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		newReader, err := reader.Reopen()
		if err != nil {
			b.Fatalf("Failed to reopen: %v", err)
		}
		oldReader := reader
		reader = newReader
		oldReader.Close()
	}

	reader.Close()
}

// BenchmarkNRTReopenWithChanges benchmarks NRT reopen with new changes
func BenchmarkNRTReopenWithChanges(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		b.Fatalf("Failed to commit: %v", err)
	}

	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Add a few documents
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("newdoc%d", i), document.Stored|document.Indexed))
		writer.AddDocument(doc)
		writer.Commit()

		// Reopen reader
		newReader, err := reader.Reopen()
		if err != nil {
			b.Fatalf("Failed to reopen: %v", err)
		}
		oldReader := reader
		reader = newReader
		oldReader.Close()
	}

	reader.Close()
}

// BenchmarkNRTConcurrentIndexing benchmarks concurrent NRT indexing
func BenchmarkNRTConcurrentIndexing(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
			doc.Add(document.NewTextField("content", "test content", document.Stored|document.Indexed))

			writer.AddDocument(doc)
			i++
		}
	})

	writer.Commit()
}

// BenchmarkNRTReaderCreation benchmarks reader creation
func BenchmarkNRTReaderCreation(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		b.Fatalf("Failed to commit: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader, err := writer.GetReader()
		if err != nil {
			b.Fatalf("Failed to open reader: %v", err)
		}
		reader.Close()
	}
}

// BenchmarkNRTDocumentThroughput measures document throughput
func BenchmarkNRTDocumentThroughput(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetRAMBufferSizeMB(64.0)
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Open reader
	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", fmt.Sprintf("Content for document %d", i), document.Stored|document.Indexed))

		err := writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()

	elapsed := time.Since(start)
	docsPerSec := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(docsPerSec, "docs/sec")
}

// BenchmarkNRTReopenLatency benchmarks reopen latency
func BenchmarkNRTReopenLatency(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 10000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		err = writer.AddDocument(doc)
		if err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}
	}

	err = writer.Commit()
	if err != nil {
		b.Fatalf("Failed to commit: %v", err)
	}

	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		newReader, err := reader.Reopen()
		if err != nil {
			b.Fatalf("Failed to reopen: %v", err)
		}
		oldReader := reader
		reader = newReader
		oldReader.Close()
	}

	reader.Close()
}

// BenchmarkNRTVsNonNRT compares NRT vs non-NRT performance
func BenchmarkNRTVsNonNRT(b *testing.B) {
	b.Run("NRT", func(b *testing.B) {
		dir, _ := store.NewRAMDirectory()
		defer dir.Close()

		config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
		writer, _ := NewIndexWriter(dir, config)
		defer writer.Close()

		reader, _ := writer.GetReader()
		defer reader.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
			writer.AddDocument(doc)

			if i%100 == 0 {
				writer.Commit()
				newReader, _ := reader.Reopen()
				reader.Close()
				reader = newReader
			}
		}
	})

	b.Run("NonNRT", func(b *testing.B) {
		dir, _ := store.NewRAMDirectory()
		defer dir.Close()

		config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
		writer, _ := NewIndexWriter(dir, config)
		defer writer.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
			writer.AddDocument(doc)

			if i%100 == 0 {
				writer.Commit()
			}
		}
	})
}

// BenchmarkNRTMemoryUsage benchmarks memory usage
func BenchmarkNRTMemoryUsage(b *testing.B) {
	dir, err := store.NewRAMDirectory()
	if err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetRAMBufferSizeMB(16.0)
	writer, err := NewIndexWriter(dir, config)
	if err != nil {
		b.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	reader, err := writer.GetReader()
	if err != nil {
		b.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", "test content", document.Stored|document.Indexed))
		writer.AddDocument(doc)

		if i%1000 == 0 {
			writer.Commit()
			newReader, _ := reader.Reopen()
			reader.Close()
			reader = newReader
		}
	}
}
