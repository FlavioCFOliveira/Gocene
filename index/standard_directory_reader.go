// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// StandardDirectoryReader is the standard implementation of DirectoryReader.
// This is the Go port of Lucene's org.apache.lucene.index.StandardDirectoryReader.
//
// StandardDirectoryReader provides a complete implementation for reading indexes
// from a Directory, managing multiple SegmentReaders and providing a unified
// view of the index.
type StandardDirectoryReader struct {
	*BaseCompositeReader

	// directory is the source directory
	directory store.Directory

	// segmentInfos contains information about all segments
	segmentInfos *SegmentInfos

	// readers holds readers for each segment
	readers []*SegmentReader

	// lastCommittedInfos holds the last committed segment infos
	lastCommittedInfos *SegmentInfos

	// isCurrent indicates if this reader is up to date
	isCurrent bool

	// mu protects mutable fields
	mu sync.RWMutex

	// readerContext is the context for this reader
	readerContext IndexReaderContext
}

// NewStandardDirectoryReader creates a new StandardDirectoryReader.
func NewStandardDirectoryReader(directory store.Directory, readers []*SegmentReader, segmentInfos *SegmentInfos, lastCommittedInfos *SegmentInfos, isCurrent bool) (*StandardDirectoryReader, error) {
	if len(readers) == 0 {
		return nil, fmt.Errorf("readers array must be non-empty")
	}

	// Convert SegmentReaders to IndexReaderInterface
	subReaders := make([]IndexReaderInterface, len(readers))
	for i, reader := range readers {
		subReaders[i] = reader
	}

	baseReader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		return nil, err
	}

	reader := &StandardDirectoryReader{
		BaseCompositeReader: baseReader,
		directory:           directory,
		segmentInfos:        segmentInfos,
		readers:             readers,
		lastCommittedInfos:  lastCommittedInfos,
		isCurrent:           isCurrent,
	}

	return reader, nil
}

// Open opens a StandardDirectoryReader for the given directory.
func OpenStandardDirectoryReader(directory store.Directory) (*StandardDirectoryReader, error) {
	// Read segment infos
	segmentInfos, err := ReadSegmentInfos(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read segment infos: %w", err)
	}

	return OpenStandardDirectoryReaderWithInfos(directory, segmentInfos)
}

// OpenStandardDirectoryReaderWithInfos opens a StandardDirectoryReader with existing SegmentInfos.
func OpenStandardDirectoryReaderWithInfos(directory store.Directory, segmentInfos *SegmentInfos) (*StandardDirectoryReader, error) {
	// Create readers for each segment
	readers := make([]*SegmentReader, 0, segmentInfos.Size())
	for i := 0; i < segmentInfos.Size(); i++ {
		segmentCommitInfo := segmentInfos.Get(i)
		segmentReader := NewSegmentReader(segmentCommitInfo)
		readers = append(readers, segmentReader)
	}

	return NewStandardDirectoryReader(directory, readers, segmentInfos, segmentInfos, true)
}

// OpenIfChanged reopens the index if there have been changes.
// Returns the new reader if changed, or the same reader if unchanged.
func (r *StandardDirectoryReader) OpenIfChanged() (*StandardDirectoryReader, error) {
	r.mu.RLock()
	if r.isCurrent {
		r.mu.RUnlock()
		return r, nil
	}
	r.mu.RUnlock()

	// Check for changes
	current, err := r.IsCurrent()
	if err != nil {
		return nil, err
	}
	if current {
		return r, nil
	}

	// Open new reader
	return OpenStandardDirectoryReader(r.directory)
}

// GetDirectory returns the directory being read.
func (r *StandardDirectoryReader) GetDirectory() store.Directory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.directory
}

// GetSegmentInfos returns the SegmentInfos for this reader.
func (r *StandardDirectoryReader) GetSegmentInfos() *SegmentInfos {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.segmentInfos
}

// GetIndexCommit returns the IndexCommit that this reader is reading from.
func (r *StandardDirectoryReader) GetIndexCommit() *IndexCommit {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.segmentInfos == nil {
		return nil
	}

	commit := NewIndexCommit(r.segmentInfos)
	commit.SetDirectory(r.directory)
	return commit
}

// GetSegmentReaders returns the SegmentReaders.
func (r *StandardDirectoryReader) GetSegmentReaders() []*SegmentReader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readers
}

// IsCurrent returns true if the reader is still up to date with the index.
func (r *StandardDirectoryReader) IsCurrent() (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read current segment infos
	segmentInfos, err := ReadSegmentInfos(r.directory)
	if err != nil {
		return false, err
	}

	return segmentInfos.Generation() == r.segmentInfos.Generation(), nil
}

// NumDocs returns the total number of live documents across all segments.
func (r *StandardDirectoryReader) NumDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDocs()
	}
	return total
}

// MaxDoc returns the maximum document ID across all segments.
func (r *StandardDirectoryReader) MaxDoc() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.MaxDoc()
	}
	return total
}

// DocCount returns the total document count across all segments.
func (r *StandardDirectoryReader) DocCount() int {
	return r.NumDocs()
}

// HasDeletions returns true if any segment has deleted documents.
func (r *StandardDirectoryReader) HasDeletions() bool {
	for _, reader := range r.readers {
		if reader.HasDeletions() {
			return true
		}
	}
	return false
}

// NumDeletedDocs returns the total number of deleted documents across all segments.
func (r *StandardDirectoryReader) NumDeletedDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDeletedDocs()
	}
	return total
}

// GetTermVectors returns term vectors for the given document across all segments.
func (r *StandardDirectoryReader) GetTermVectors(docID int) (Fields, error) {
	// Find the correct segment for this document ID
	remainingDocID := docID
	for _, reader := range r.readers {
		maxDoc := reader.MaxDoc()
		if remainingDocID < maxDoc {
			return reader.GetTermVectors(remainingDocID)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// Terms returns the Terms for a field, merging across all segments.
func (r *StandardDirectoryReader) Terms(field string) (Terms, error) {
	// Collect Terms from all segments that have the field
	for _, reader := range r.readers {
		terms, err := reader.Terms(field)
		if err != nil {
			return nil, err
		}
		if terms != nil {
			return terms, nil
		}
	}
	return nil, nil
}

// closeInternal closes the reader and all segment readers.
func (r *StandardDirectoryReader) closeInternal() error {
	var lastErr error
	for _, reader := range r.readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}
	r.readers = nil
	return lastErr
}

// Close closes the StandardDirectoryReader and all segment readers.
func (r *StandardDirectoryReader) Close() error {
	if err := r.closeInternal(); err != nil {
		return err
	}
	return r.BaseCompositeReader.closeInternal()
}

// GetSequentialSubReaders returns the sub-readers in sequential order.
func (r *StandardDirectoryReader) GetSequentialSubReaders() []IndexReaderInterface {
	subReaders := make([]IndexReaderInterface, len(r.readers))
	for i, reader := range r.readers {
		subReaders[i] = reader
	}
	return subReaders
}

// GetContext returns the reader context for this directory reader.
func (r *StandardDirectoryReader) GetContext() (IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}

	// Build context if not already built
	if r.readerContext == nil {
		ctx, err := r.buildContext()
		if err != nil {
			return nil, err
		}
		r.readerContext = ctx.(IndexReaderContext)
	}

	return r.readerContext, nil
}

// buildContext builds the context hierarchy for this reader.
func (r *StandardDirectoryReader) buildContext() (IndexReaderContext, error) {
	// Create leaf contexts for each segment
	leaves := make([]*LeafReaderContext, len(r.readers))
	docBase := 0
	for i, segmentReader := range r.readers {
		leaves[i] = NewLeafReaderContext(segmentReader.LeafReader, nil, i, docBase)
		docBase += segmentReader.MaxDoc()
	}

	// Create composite context
	return NewCompositeReaderContextWithChildren(r, nil, nil, leaves), nil
}

// Leaves returns all leaf reader contexts from all segments.
func (r *StandardDirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	compCtx, ok := ctx.(*CompositeReaderContext)
	if !ok {
		return nil, fmt.Errorf("context is not a CompositeReaderContext")
	}
	return compCtx.Leaves(), nil
}

// Ensure StandardDirectoryReader implements IndexReaderInterface
var _ IndexReaderInterface = (*StandardDirectoryReader)(nil)
