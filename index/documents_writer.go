// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
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

	// codec is the codec for encoding/decoding index data
	codec Codec

	// config is the IndexWriter configuration
	config *IndexWriterConfig

	// flushPolicy controls when to flush
	flushPolicy FlushPolicy

	// perThreadPool manages per-thread writers
	perThreadPool []*DocumentsWriterPerThread

	// threadLock protects perThreadPool access
	threadLock sync.RWMutex

	// numDocsInRAM tracks documents in memory across all threads
	numDocsInRAM int

	// numDocs tracks total documents processed
	numDocs int

	// bytesUsed tracks memory usage
	bytesUsed int64

	// closed indicates if the writer is closed
	closed bool

	// mu protects mutable fields
	mu sync.RWMutex

	// segmentNameCounter is used to generate segment names
	segmentNameCounter int64
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

// NewDocumentsWriter creates a new DocumentsWriter.
func NewDocumentsWriter(directory store.Directory, config *IndexWriterConfig) (*DocumentsWriter, error) {
	dw := &DocumentsWriter{
		directory:         directory,
		analyzer:          config.analyzer,
		config:            config,
		perThreadPool:     make([]*DocumentsWriterPerThread, 0),
		flushPolicy:       NewDefaultFlushPolicy(config.maxBufferedDocs, config.ramBufferSizeMB),
		segmentNameCounter: 0,
	}

	return dw, nil
}

// SetCodec sets the codec for this writer.
func (dw *DocumentsWriter) SetCodec(codec Codec) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.codec = codec
}

// UpdateDocument updates a document (adds a new document, optionally deleting an old one).
func (dw *DocumentsWriter) UpdateDocument(doc Document, analyzer analysis.Analyzer, term *Term) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// Get a per-thread writer
	dwpt := dw.getPerThreadWriter()

	// Use the provided analyzer or the default one
	if analyzer == nil {
		analyzer = dw.analyzer
	}

	// Process the document
	if err := dwpt.ProcessDocument(doc); err != nil {
		return err
	}

	dw.numDocsInRAM++
	dw.numDocs++

	// Update memory tracking
	dw.bytesUsed += dwpt.GetBytesUsed()

	// Check if flush is needed
	if dw.flushPolicy.ShouldFlush(dw.numDocsInRAM, dw.bytesUsed) {
		return dw.flush()
	}

	return nil
}

// AddDocument adds a document to the index.
// This is equivalent to UpdateDocument with term=nil.
func (dw *DocumentsWriter) AddDocument(doc Document, analyzer analysis.Analyzer) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// Get a per-thread writer
	dwpt := dw.getPerThreadWriter()

	// Use the provided analyzer or the default one
	if analyzer == nil {
		analyzer = dw.analyzer
	}

	// Process the document
	if err := dwpt.ProcessDocument(doc); err != nil {
		return err
	}

	dw.numDocsInRAM++
	dw.numDocs++

	// Update memory tracking
	dw.bytesUsed += dwpt.GetBytesUsed()

	// Check if flush is needed
	if dw.flushPolicy.ShouldFlush(dw.numDocsInRAM, dw.bytesUsed) {
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
// This method should be called with the lock held.
func (dw *DocumentsWriter) getPerThreadWriter() *DocumentsWriterPerThread {
	dw.threadLock.Lock()
	defer dw.threadLock.Unlock()

	// For now, create a new one each time
	// In production, this would use thread-local storage or a pool
	dwpt := NewDocumentsWriterPerThread(dw)
	dw.perThreadPool = append(dw.perThreadPool, dwpt)
	return dwpt
}

// ramUsed returns the estimated RAM usage in bytes.
func (dw *DocumentsWriter) ramUsed() int64 {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.bytesUsed
}

// flush flushes documents to disk.
// This method should be called with the lock held.
func (dw *DocumentsWriter) flush() error {
	if dw.numDocsInRAM == 0 {
		return nil // Nothing to flush
	}

	if dw.codec == nil {
		// No codec set, cannot flush
		dw.numDocsInRAM = 0
		return nil
	}

	// Generate a new segment name
	segmentName := dw.nextSegmentName()

	// Get all per-thread writers
	dw.threadLock.RLock()
	dwpts := make([]*DocumentsWriterPerThread, len(dw.perThreadPool))
	copy(dwpts, dw.perThreadPool)
	dw.threadLock.RUnlock()

	// Flush each DWPT and collect segment infos
	var segments []*SegmentInfo
	var totalDocsFlushed int

	for _, dwpt := range dwpts {
		if dwpt.GetNumDocs() == 0 {
			continue // Nothing to flush in this DWPT
		}

		// Flush the DWPT
		segmentInfo, err := dwpt.Flush(dw.directory, dw.codec, segmentName)
		if err != nil {
			return fmt.Errorf("failed to flush segment %s: %w", segmentName, err)
		}

		if segmentInfo != nil {
			segments = append(segments, segmentInfo)
			totalDocsFlushed += segmentInfo.DocCount()

			// Generate next segment name for subsequent segments
			segmentName = dw.nextSegmentName()
		}
	}

	// Create segment commit infos for the flushed segments
	for _, si := range segments {
		// Write segment info to directory
		if err := WriteSegmentInfo(si, dw.directory); err != nil {
			return fmt.Errorf("failed to write segment info: %w", err)
		}
	}

	// Reset counters
	dw.numDocsInRAM = 0
	dw.bytesUsed = 0

	// Reset all DWPTs
	for _, dwpt := range dwpts {
		dwpt.Reset()
	}

	return nil
}

// Flush explicitly flushes all pending documents to disk.
func (dw *DocumentsWriter) Flush() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if dw.closed {
		return fmt.Errorf("DocumentsWriter is closed")
	}

	return dw.flush()
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

// nextSegmentName generates the next segment name.
func (dw *DocumentsWriter) nextSegmentName() string {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	name := fmt.Sprintf("_%d", dw.segmentNameCounter)
	dw.segmentNameCounter++
	return name
}

// WriteSegmentInfo writes a SegmentInfo to the directory.
func WriteSegmentInfo(si *SegmentInfo, dir store.Directory) error {
	// Create segment info file
	fileName := si.Name() + ".si"
	out, err := dir.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write header
	if err := store.WriteInt32(out, 0x3d767); err != nil { // Magic number
		return err
	}

	// Write segment info
	if err := store.WriteString(out, si.Name()); err != nil {
		return err
	}
	if err := store.WriteInt32(out, int32(si.DocCount())); err != nil {
		return err
	}

	// Write codec name
	if err := store.WriteString(out, si.Codec()); err != nil {
		return err
	}

	// Write ID
	id := si.GetID()
	if err := out.WriteBytes(id); err != nil {
		return err
	}

	return nil
}

