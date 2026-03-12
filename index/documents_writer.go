// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DocumentsWriter handles per-thread document processing.
//
// This is the Go port of Lucene's org.apache.lucene.index.DocumentsWriter.
//
// DocumentsWriter is responsible for processing documents in parallel threads,
// building in-memory indices, and flushing them to disk as segments.
type DocumentsWriter struct {
	// directory is the directory for index storage
	directory store.Directory

	// analyzer is the analyzer for text processing
	analyzer analysis.Analyzer

	// config is the IndexWriter configuration
	config *IndexWriterConfig

	// flushPolicy controls when to flush
	flushPolicy FlushPolicy

	// perThreadPool manages per-thread writers
	perThreadPool []*DocumentsWriterPerThread

	// numDocsInRAM tracks documents in memory
	numDocsInRAM int

	// numDocs tracks total documents
	numDocs int

	// closed indicates if the writer is closed
	closed bool

	// mu protects mutable fields
	mu sync.RWMutex
}

// FlushPolicy controls when to flush documents to disk.
type FlushPolicy interface {
	// ShouldFlush returns true if a flush should occur.
	ShouldFlush(numDocs int, ramUsed int64) bool
}

// DefaultFlushPolicy is the default flush policy.
type DefaultFlushPolicy struct {
	maxBufferedDocs int
	maxRAMBufferMB  float64
}

// NewDefaultFlushPolicy creates a new DefaultFlushPolicy.
func NewDefaultFlushPolicy(maxBufferedDocs int, maxRAMBufferMB float64) *DefaultFlushPolicy {
	return &DefaultFlushPolicy{
		maxBufferedDocs: maxBufferedDocs,
		maxRAMBufferMB:  maxRAMBufferMB,
	}
}

// ShouldFlush returns true if a flush should occur.
func (p *DefaultFlushPolicy) ShouldFlush(numDocs int, ramUsed int64) bool {
	if p.maxBufferedDocs > 0 && numDocs >= p.maxBufferedDocs {
		return true
	}
	if p.maxRAMBufferMB > 0 {
		maxBytes := int64(p.maxRAMBufferMB * 1024 * 1024)
		if ramUsed >= maxBytes {
			return true
		}
	}
	return false
}

// DocumentsWriterPerThread handles document processing for a single thread.
type DocumentsWriterPerThread struct {
	// indexWriter is the parent DocumentsWriter
	indexWriter *DocumentsWriter

	// segmentInfo holds segment information
	segmentInfo *SegmentInfo

	// fieldInfos holds field information
	fieldInfos *FieldInfos

	// numDocs tracks documents processed
	numDocs int
}

// NewDocumentsWriter creates a new DocumentsWriter.
func NewDocumentsWriter(directory store.Directory, config *IndexWriterConfig) (*DocumentsWriter, error) {
	dw := &DocumentsWriter{
		directory:     directory,
		analyzer:      config.analyzer,
		config:        config,
		perThreadPool: make([]*DocumentsWriterPerThread, 0),
		flushPolicy:   NewDefaultFlushPolicy(config.maxBufferedDocs, config.ramBufferSizeMB),
	}

	return dw, nil
}

// UpdateDocument updates a document.
func (dw *DocumentsWriter) UpdateDocument(doc Document, analyzer analysis.Analyzer, term *Term) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// Get a per-thread writer
	dwpt := dw.getPerThreadWriter()

	// Process the document
	if err := dwpt.processDocument(doc); err != nil {
		return err
	}

	dw.numDocsInRAM++
	dw.numDocs++

	// Check if flush is needed
	if dw.flushPolicy.ShouldFlush(dw.numDocsInRAM, dw.ramUsed()) {
		return dw.flush()
	}

	return nil
}

// UpdateDocuments updates multiple documents.
func (dw *DocumentsWriter) UpdateDocuments(docs []Document, analyzer analysis.Analyzer, term *Term) error {
	for _, doc := range docs {
		if err := dw.UpdateDocument(doc, analyzer, term); err != nil {
			return err
		}
	}
	return nil
}

// getPerThreadWriter returns a per-thread writer.
func (dw *DocumentsWriter) getPerThreadWriter() *DocumentsWriterPerThread {
	// For now, create a new one each time
	// In production, this would use thread-local storage
	dwpt := &DocumentsWriterPerThread{
		indexWriter: dw,
		fieldInfos:  NewFieldInfos(),
	}
	dw.perThreadPool = append(dw.perThreadPool, dwpt)
	return dwpt
}

// ramUsed returns the estimated RAM usage in bytes.
func (dw *DocumentsWriter) ramUsed() int64 {
	// Simplified estimation
	return int64(dw.numDocsInRAM * 1024) // Estimate 1KB per document
}

// flush flushes documents to disk.
func (dw *DocumentsWriter) flush() error {
	// TODO: Implement flush to disk
	dw.numDocsInRAM = 0
	return nil
}

// Close closes the DocumentsWriter.
func (dw *DocumentsWriter) Close() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if dw.closed {
		return nil
	}

	// Flush any remaining documents
	if err := dw.flush(); err != nil {
		return err
	}

	dw.closed = true
	return nil
}

// GetNumDocs returns the total number of documents.
func (dw *DocumentsWriter) GetNumDocs() int {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.numDocs
}

// GetNumDocsInRAM returns the number of documents in RAM.
func (dw *DocumentsWriter) GetNumDocsInRAM() int {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.numDocsInRAM
}

// processDocument processes a single document.
func (dwpt *DocumentsWriterPerThread) processDocument(doc Document) error {
	dwpt.numDocs++
	// TODO: Implement actual document processing
	return nil
}
