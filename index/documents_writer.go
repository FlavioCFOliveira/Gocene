// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
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
	analyzer api.Analyzer

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

	// deleteQueue holds pending delete operations
	deleteQueue *DocumentsWriterDeleteQueue

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
// Optimized for branch prediction: combines checks to reduce branch mispredictions.
func (p *DefaultFlushPolicy) ShouldFlush(numDocs int, ramUsed int64) bool {
	// Check document count limit first (cheapest operation)
	if p.maxBufferedDocs > 0 && numDocs >= p.maxBufferedDocs {
		return true
	}
	// Check RAM limit - pre-compute maxBytes to avoid repeated multiplication
	maxBytes := int64(p.maxRAMBufferMB * 1024 * 1024)
	return p.maxRAMBufferMB > 0 && ramUsed >= maxBytes
}

// NewDocumentsWriter creates a new DocumentsWriter.
func NewDocumentsWriter(
	notifications *FlushNotifications,
	version int,
	pendingNumDocs *atomic.Int64,
	useCompoundFile bool,
	newSegmentName func() string,
	config *IndexWriterConfig,
	dirOrig store.Directory,
	dir store.Directory,
	fnm *FieldNumbers,
	infoStream InfoStream,
) (*DocumentsWriter, error) {
	dw := &DocumentsWriter{
		directory:          dir,
		analyzer:           config.analyzer,
		config:             config,
		codec:              config.Codec(),
		perThreadPool:      make([]*DocumentsWriterPerThread, 0),
		flushPolicy:        NewDefaultFlushPolicy(config.maxBufferedDocs, config.ramBufferSizeMB),
		segmentNameCounter: 0,
		deleteQueue:        NewDocumentsWriterDeleteQueue(infoStream),
	}

	return dw, nil
}

// SetCodec sets the codec for this writer.
func (dw *DocumentsWriter) SetCodec(codec Codec) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.codec = codec
}

// ShouldFlush reports whether the in-memory state has crossed the configured
// flush threshold. It mirrors Lucene's DocumentsWriter#doFlush internal
// flush-trigger check and is safe for concurrent use.
func (dw *DocumentsWriter) ShouldFlush() bool {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	if dw.flushPolicy == nil {
		return false
	}
	return dw.flushPolicy.ShouldFlush(dw.numDocsInRAM, dw.bytesUsed)
}

// DeleteQueries deletes documents matching the given queries.
func (dw *DocumentsWriter) DeleteQueries(queries []Query) int64 {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if len(queries) == 0 {
		return dw.GetNextSequenceNumber()
	}

	var node Node
	if len(queries) == 1 {
		node = NewQueryNode(queries[0])
	} else {
		node = &queryArrayNode{
			queries: queries,
		}
	}

	return dw.deleteQueue.Add(node)
}

// GetNextSequenceNumber returns the next sequence number from the delete queue.
func (dw *DocumentsWriter) GetNextSequenceNumber() int64 {
	return dw.deleteQueue.GetNextSequenceNumber()
}

// UpdateDocument updates a document (adds a new document, optionally deleting an old one).
//
// Note: this method does NOT trigger an auto-flush; the IndexWriter is
// responsible for all flush coordination (see AddDocument doc comment).
func (dw *DocumentsWriter) UpdateDocument(doc Document, analyzer api.Analyzer, term *Term) (int64, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// If term is provided, add a delete operation to the queue
	var seqNo int64
	if term != nil {
		node := NewTermNode(*term)
		seqNo = dw.deleteQueue.Add(node)
	}

	// Get a per-thread writer
	dwpt := dw.getPerThreadWriter()

	// Use the provided analyzer or the default one
	if analyzer == nil {
		analyzer = dw.analyzer
	}

	// Process the document
	before := dwpt.GetBytesUsed()
	if err := dwpt.ProcessDocument(doc); err != nil {
		return 0, err
	}

	dw.numDocsInRAM++
	dw.numDocs++

	// Update memory tracking: only count the bytes added by this document,
	// not the entire accumulated DWPT total.
	dw.bytesUsed += dwpt.GetBytesUsed() - before

	// If no term was provided, we still need a sequence number for the addition
	if term == nil {
		seqNo = dw.GetNextSequenceNumber()
	}

	return seqNo, nil
}

// AddDocument adds a document to the index.
// This is equivalent to UpdateDocument with term=nil.
//
// Note: this method does NOT trigger an auto-flush of the DocumentsWriter's
// internal DWPT. All flush coordination is the responsibility of the
// IndexWriter, which calls flushPendingDocsLocked (and ultimately Commit)
// to materialise segments. The old DocumentsWriter-level auto-flush wrote
// segment files directly to disk without registering them in the SegmentInfos,
// causing "file already exists" errors when Commit later tried to create
// segments under the same names.
func (dw *DocumentsWriter) AddDocument(doc Document, analyzer api.Analyzer) (int64, error) {
	return dw.UpdateDocument(doc, analyzer, nil)
}

// UpdateDocuments updates multiple documents.
func (dw *DocumentsWriter) UpdateDocuments(docs []Document, analyzer api.Analyzer, term *Term) (int64, error) {
	var lastSeqNo int64
	for _, doc := range docs {
		seqNo, err := dw.UpdateDocument(doc, analyzer, term)
		if err != nil {
			return 0, err
		}
		lastSeqNo = seqNo
	}
	return lastSeqNo, nil
}

// getPerThreadWriter returns a per-thread writer.
func (dw *DocumentsWriter) getPerThreadWriter() *DocumentsWriterPerThread {
	dw.threadLock.Lock()
	defer dw.threadLock.Unlock()

	if len(dw.perThreadPool) > 0 {
		return dw.perThreadPool[0]
	}
	// Reserve a segment name up front, matching Lucene's DWPT constructor.
	// This lets lazily-initialized codec writers (in particular the term-vectors
	// writer) create their on-disk files under the correct segment name.
	segmentName := dw.nextSegmentName()
	dwpt := NewDocumentsWriterPerThread(dw, segmentName)
	dw.perThreadPool = append(dw.perThreadPool, dwpt)
	return dwpt
}

// GetPerThreadPool returns the current DWPT pool.
func (dw *DocumentsWriter) GetPerThreadPool() []*DocumentsWriterPerThread {
	dw.threadLock.RLock()
	defer dw.threadLock.RUnlock()
	return dw.perThreadPool
}

// TakePerThreadPool returns the current DWPT pool and resets it.
// Called by IndexWriter.flushPendingDocsLocked() to snapshot postings before
// clearing the pending state.
func (dw *DocumentsWriter) TakePerThreadPool() []*DocumentsWriterPerThread {
	dw.threadLock.Lock()
	defer dw.threadLock.Unlock()
	pool := dw.perThreadPool
	dw.perThreadPool = make([]*DocumentsWriterPerThread, 0)
	dw.numDocsInRAM = 0
	dw.bytesUsed = 0
	return pool
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
		return fmt.Errorf("documents_writer: cannot flush %d buffered documents: no codec configured", dw.numDocsInRAM)
	}

	// Get all per-thread writers
	dw.threadLock.RLock()
	dwpts := make([]*DocumentsWriterPerThread, len(dw.perThreadPool))
	copy(dwpts, dw.perThreadPool)
	dw.threadLock.RUnlock()

	// Flush each DWPT and collect segment infos.  Each DWPT already reserved
	// its segment name when it was obtained from the pool, so there is no need
	// to generate fresh names here.
	var segments []*SegmentInfo
	var totalDocsFlushed int

	for _, dwpt := range dwpts {
		if dwpt.GetNumDocs() == 0 {
			continue // Nothing to flush in this DWPT
		}

		segmentName := dwpt.SegmentName()
		// Flush the DWPT
		segmentInfo, err := dwpt.Flush(dw.directory, dw.codec, segmentName)
		if err != nil {
			return fmt.Errorf("failed to flush segment %s: %w", segmentName, err)
		}

		if segmentInfo != nil {
			segments = append(segments, segmentInfo)
			totalDocsFlushed += segmentInfo.DocCount()
		}
	}

	// Create segment commit infos for the flushed segments
	for _, si := range segments {
		// Write segment info to directory using the codec's segment-info format
		// so that the .si file is byte-compatible with Apache Lucene 10.4.0.
		if err := WriteSegmentInfo(si, dw.directory, dw.codec); err != nil {
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

// FlushNextBuffer flushes the next available per-thread writer.
// Returns true if a buffer was flushed, false otherwise.
func (dw *DocumentsWriter) FlushNextBuffer() bool {
	dw.threadLock.Lock()
	if len(dw.perThreadPool) == 0 {
		dw.threadLock.Unlock()
		return false
	}
	dwpt := dw.perThreadPool[0]
	dw.perThreadPool = dw.perThreadPool[1:]
	dw.threadLock.Unlock()

	if dwpt.GetNumDocs() == 0 {
		return false
	}

	segmentName := dwpt.SegmentName()
	segmentInfo, err := dwpt.Flush(dw.directory, dw.codec, segmentName)
	if err != nil {
		panic(fmt.Sprintf("failed to flush segment %s: %v", segmentName, err))
	}

	if segmentInfo != nil {
		if err := WriteSegmentInfo(segmentInfo, dw.directory, dw.codec); err != nil {
			panic(fmt.Sprintf("failed to write segment info: %v", err))
		}
	}
	return true
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
// Must be called with dw.mu held (write lock).
func (dw *DocumentsWriter) nextSegmentName() string {
	name := fmt.Sprintf("_%d", dw.segmentNameCounter)
	dw.segmentNameCounter++
	return name
}

// SyncSegmentNameCounter advances the segment-name counter so it is strictly
// greater than any existing segment name in the directory.  This must be called
// after external operations (commits, merges, addIndexes) that may introduce
// segment names the DocumentsWriter has not yet seen, preventing name collisions
// when the next flush runs.
func (dw *DocumentsWriter) SyncSegmentNameCounter() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	files, err := dw.directory.ListAll()
	if err != nil {
		return
	}
	var max int64 = -1
	for _, f := range files {
		if len(f) > 1 && f[0] == '_' {
			var n int64
			// Parse the numeric part; stop at the first non-digit.
			for i := 1; i < len(f); i++ {
				if f[i] < '0' || f[i] > '9' {
					break
				}
				n = n*10 + int64(f[i]-'0')
			}
			if n > max {
				max = n
			}
		}
	}
	if max >= 0 && max+1 > dw.segmentNameCounter {
		dw.segmentNameCounter = max + 1
	}
}

// DeleteTerms deletes documents matching the given terms.
func (dw *DocumentsWriter) DeleteTerms(terms []Term) (int64, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if len(terms) == 0 {
		return dw.GetNextSequenceNumber(), nil
	}

	var node Node
	if len(terms) == 1 {
		node = NewTermNode(terms[0])
	} else {
		// Use a term array node for multiple terms
		node = &termArrayNode{
			terms: terms,
		}
	}

	seqNo := dw.deleteQueue.Add(node)
	return seqNo, nil
}

// WriteSegmentInfo writes a SegmentInfo to the directory.

// When codec is non-nil, it delegates to codec.SegmentInfoFormat().Write so
// the .si file is byte-compatible with Apache Lucene.  When codec is nil, a
// minimal fallback format is used (structural-test path only).
func WriteSegmentInfo(si *SegmentInfo, dir store.Directory, codec Codec) error {
	if codec != nil {
		if format := codec.SegmentInfoFormat(); format != nil {
			return format.Write(dir, si, store.IOContextWrite)
		}
	}

	// Fallback: minimal custom format for tests that do not wire a codec.
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
