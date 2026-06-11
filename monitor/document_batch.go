// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DocumentBatch holds one or more Documents and provides a LeafReader over them.
// It is used by Monitor to match queries against in-memory document content.
//
// This is the Go port of org.apache.lucene.monitor.DocumentBatch from
// Apache Lucene 10.4.0.
//
// Deviation from Lucene: The Java reference uses MemoryIndex for single-document
// batches as a performance optimisation. Gocene's MemoryIndex does not expose a
// LeafReader, so we use a single-segment in-memory Directory for both single-
// and multi-document cases. This is correct (same byte-level semantics) but
// may be slightly less efficient for single-document batches.
type DocumentBatch struct {
	reader  *index.LeafReader
	dir     store.Directory
	dirRdr  *index.DirectoryReader
	closed  bool
}

// NewDocumentBatch creates a DocumentBatch containing a single document.
//
// This is the Go equivalent of DocumentBatch.of(Analyzer, Document).
func NewDocumentBatch(analyzer analysis.Analyzer, doc *document.Document) (*DocumentBatch, error) {
	return newDocumentBatchFromDocs(analyzer, []*document.Document{doc})
}

// NewDocumentBatchFromDocs creates a DocumentBatch containing multiple documents.
//
// This is the Go equivalent of DocumentBatch.of(Analyzer, Document...).
// docs must contain at least one document.
func NewDocumentBatchFromDocs(analyzer analysis.Analyzer, docs []*document.Document) (*DocumentBatch, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("document batch: must contain at least one document")
	}
	return newDocumentBatchFromDocs(analyzer, docs)
}

// newDocumentBatchFromDocs builds a single-segment in-memory index from the
// given documents and returns a DocumentBatch backed by the resulting LeafReader.
func newDocumentBatchFromDocs(analyzer analysis.Analyzer, docs []*document.Document) (*DocumentBatch, error) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: create writer: %w", err)
	}

	// Convert []*document.Document to []index.Document for AddDocuments.
	idxDocs := make([]index.Document, len(docs))
	for i, d := range docs {
		idxDocs[i] = d
	}

	if err := writer.AddDocuments(idxDocs); err != nil {
		_ = writer.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: add documents: %w", err)
	}
	if err := writer.Commit(); err != nil {
		_ = writer.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: commit: %w", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		_ = writer.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: force merge: %w", err)
	}
	if err := writer.Close(); err != nil {
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: close writer: %w", err)
	}

	dirReader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: open reader: %w", err)
	}

	leaves, err := dirReader.Leaves()
	if err != nil {
		_ = dirReader.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: get leaves: %w", err)
	}

	if len(leaves) == 0 {
		_ = dirReader.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: no leaves in reader")
	}

	// Get the LeafReader from the first (and only) leaf context.
	// In Gocene, the leaf may be a *SegmentReader (which embeds *LeafReader)
	// or a raw *LeafReader.
	leafCtx := leaves[0]
	var leafReader *index.LeafReader

	switch r := leafCtx.Reader().(type) {
	case *index.LeafReader:
		leafReader = r
	default:
		_ = dirReader.Close()
		_ = dir.Close()
		return nil, fmt.Errorf("document batch: unexpected leaf reader type: %T", leafCtx.Reader())
	}

	return &DocumentBatch{
		reader: leafReader,
		dir:    dir,
		dirRdr: dirReader,
	}, nil
}

// GetReader returns the LeafReader for this batch. The reader is backed by a
// single-segment in-memory index containing all documents in the batch.
//
// This is the Go equivalent of Java's Supplier<LeafReader>.get().
func (b *DocumentBatch) GetReader() *index.LeafReader {
	return b.reader
}

// Close releases resources held by this DocumentBatch.
func (b *DocumentBatch) Close() error {
	if b.closed {
		return nil
	}
	b.closed = true
	var errs []error
	if b.dirRdr != nil {
		if err := b.dirRdr.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if b.dir != nil {
		if err := b.dir.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("document batch close: %v", errs)
	}
	return nil
}

// Ensure DocumentBatch implements io.Closer.
var _ io.Closer = (*DocumentBatch)(nil)
