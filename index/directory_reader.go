// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// LeafReader is an IndexReader for a single segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.LeafReader.
//
// LeafReader provides access to the index data for a single segment.
// It extends IndexReader with segment-specific information.
type LeafReader struct {
	*IndexReader

	// segmentInfo holds information about this segment
	segmentInfo *SegmentInfo

	// coreCacheKey is the key for caching
	coreCacheKey string

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewLeafReader creates a new LeafReader.
func NewLeafReader(segmentInfo *SegmentInfo) *LeafReader {
	return &LeafReader{
		IndexReader:  NewIndexReader(),
		segmentInfo:  segmentInfo,
		coreCacheKey: generateCacheKey(segmentInfo),
	}
}

// GetCoreCacheKey returns the cache key for this reader.
// Used for caching per-segment data structures.
func (r *LeafReader) GetCoreCacheKey() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.coreCacheKey
}

// GetSegmentInfo returns the SegmentInfo for this reader.
func (r *LeafReader) GetSegmentInfo() *SegmentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.segmentInfo
}

// DocCount returns the number of documents in this segment.
func (r *LeafReader) DocCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.segmentInfo != nil {
		return r.segmentInfo.docCount
	}
	return 0
}

// NumDocs returns the number of live documents in this segment.
func (r *LeafReader) NumDocs() int {
	// For now, return total docs
	// In production, subtract deleted documents
	return r.DocCount()
}

// MaxDoc returns the maximum document ID (one past the last doc).
func (r *LeafReader) MaxDoc() int {
	return r.DocCount()
}

// GetTermVectors returns the term vectors for a document.
func (r *LeafReader) GetTermVectors(docID int) (Fields, error) {
	// TODO: Implement
	return nil, nil
}

// Terms returns the Terms for a field.
func (r *LeafReader) Terms(field string) (Terms, error) {
	// TODO: Implement
	return nil, nil
}

// generateCacheKey generates a cache key for a segment.
func generateCacheKey(segmentInfo *SegmentInfo) string {
	if segmentInfo == nil {
		return ""
	}
	return segmentInfo.name
}

// DirectoryReader is a LeafReader that reads from a Directory.
//
// This is the Go port of Lucene's org.apache.lucene.index.DirectoryReader.
//
// DirectoryReader is the main implementation of IndexReader for reading
// indexes stored in a Directory. It manages a collection of SegmentReaders
// (each wrapping a LeafReader) for all segments in the index.
type DirectoryReader struct {
	*LeafReader

	// directory is the source directory
	directory store.Directory

	// segmentInfos contains information about all segments
	segmentInfos *SegmentInfos

	// readers holds readers for each segment
	readers []*SegmentReader

	// lastCommittedInfos holds the last committed segment infos
	lastCommittedInfos *SegmentInfos
}

// SegmentReader is a LeafReader for a specific segment.
type SegmentReader struct {
	*LeafReader
	segmentCommitInfo *SegmentCommitInfo
}

// NewSegmentReader creates a new SegmentReader.
func NewSegmentReader(segmentCommitInfo *SegmentCommitInfo) *SegmentReader {
	return &SegmentReader{
		LeafReader:        NewLeafReader(segmentCommitInfo.segmentInfo),
		segmentCommitInfo: segmentCommitInfo,
	}
}

// GetSegmentCommitInfo returns the SegmentCommitInfo for this reader.
func (r *SegmentReader) GetSegmentCommitInfo() *SegmentCommitInfo {
	return r.segmentCommitInfo
}

// Open opens a DirectoryReader for the given directory.
func OpenDirectoryReader(directory store.Directory) (*DirectoryReader, error) {
	// Read segment infos
	segmentInfos, err := ReadSegmentInfos(directory)
	if err != nil {
		return nil, err
	}

	return OpenDirectoryReaderWithInfos(directory, segmentInfos)
}

// OpenDirectoryReaderWithInfos opens a DirectoryReader with existing SegmentInfos.
func OpenDirectoryReaderWithInfos(directory store.Directory, segmentInfos *SegmentInfos) (*DirectoryReader, error) {
	reader := &DirectoryReader{
		LeafReader:   NewLeafReader(nil),
		directory:    directory,
		segmentInfos: segmentInfos,
		readers:      make([]*SegmentReader, 0, segmentInfos.Size()),
	}

	// Create a reader for each segment
	for i := 0; i < segmentInfos.Size(); i++ {
		segmentCommitInfo := segmentInfos.Get(i)
		segmentReader := NewSegmentReader(segmentCommitInfo)
		reader.readers = append(reader.readers, segmentReader)
	}

	return reader, nil
}

// OpenDirectoryReaderFromCommit opens a DirectoryReader from a specific commit.
func OpenDirectoryReaderFromCommit(directory store.Directory, commit *IndexCommit) (*DirectoryReader, error) {
	// TODO: Implement reading from a specific commit
	return OpenDirectoryReader(directory)
}

// Reopen reopens the index to see if any changes have been made.
func (r *DirectoryReader) Reopen() (*DirectoryReader, error) {
	isCurrent, err := r.IsCurrent()
	if err != nil {
		return nil, err
	}
	if isCurrent {
		return r, nil
	}
	return OpenDirectoryReader(r.directory)
}

// IsCurrent returns true if the reader is still up to date with the index.
func (r *DirectoryReader) IsCurrent() (bool, error) {
	segmentInfos, err := ReadSegmentInfos(r.directory)
	if err != nil {
		return false, err
	}
	return segmentInfos.generation == r.segmentInfos.generation, nil
}

// ReopenFromCommit reopens the index from a specific commit.
func (r *DirectoryReader) ReopenFromCommit(commit *IndexCommit) (*DirectoryReader, error) {
	return OpenDirectoryReaderFromCommit(r.directory, commit)
}

// GetDirectory returns the directory being read.
func (r *DirectoryReader) GetDirectory() store.Directory {
	return r.directory
}

// GetSegmentInfos returns the SegmentInfos for this reader.
func (r *DirectoryReader) GetSegmentInfos() *SegmentInfos {
	return r.segmentInfos
}

// GetSegmentReaders returns the SegmentReaders.
func (r *DirectoryReader) GetSegmentReaders() []*SegmentReader {
	return r.readers
}

// NumDocs returns the total number of live documents across all segments.
func (r *DirectoryReader) NumDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDocs()
	}
	return total
}

// MaxDoc returns the maximum document ID across all segments.
func (r *DirectoryReader) MaxDoc() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.MaxDoc()
	}
	return total
}

// DocCount returns the total document count across all segments.
func (r *DirectoryReader) DocCount() int {
	return r.NumDocs()
}

// Close closes the DirectoryReader and all segment readers.
func (r *DirectoryReader) Close() error {
	for _, reader := range r.readers {
		reader.Close()
	}
	r.readers = nil
	return r.LeafReader.Close()
}
