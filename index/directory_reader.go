// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
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
	coreCacheKey *CacheKey

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewLeafReader creates a new LeafReader.
func NewLeafReader(segmentInfo *SegmentInfo) *LeafReader {
	return &LeafReader{
		IndexReader:  NewIndexReader(),
		segmentInfo:  segmentInfo,
		coreCacheKey: NewCacheKey(),
	}
}

// NewLeafReaderWithFieldInfos creates a new LeafReader with FieldInfos.
func NewLeafReaderWithFieldInfos(segmentInfo *SegmentInfo, fieldInfos *FieldInfos) *LeafReader {
	lr := NewLeafReader(segmentInfo)
	lr.SetFieldInfos(fieldInfos)
	return lr
}

// GetCoreCacheKey returns the cache key for this reader.
// Used for caching per-segment data structures.
func (r *LeafReader) GetCoreCacheKey() *CacheKey {
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
		return r.segmentInfo.DocCount()
	}
	return r.IndexReader.DocCount()
}

// NumDocs returns the number of live documents in this segment.
func (r *LeafReader) NumDocs() int {
	// Use the IndexReader's numDocs which accounts for deletions
	return r.IndexReader.NumDocs()
}

// MaxDoc returns the maximum document ID (one past the last doc).
func (r *LeafReader) MaxDoc() int {
	return r.DocCount()
}

// HasDeletions returns true if this reader has deleted documents.
func (r *LeafReader) HasDeletions() bool {
	return r.IndexReader.HasDeletions()
}

// NumDeletedDocs returns the number of deleted documents.
func (r *LeafReader) NumDeletedDocs() int {
	return r.IndexReader.NumDeletedDocs()
}

// EnsureOpen throws an error if the reader is closed.
func (r *LeafReader) EnsureOpen() error {
	return r.IndexReader.EnsureOpen()
}

// IncRef increments the reference count.
func (r *LeafReader) IncRef() error {
	return r.IndexReader.IncRef()
}

// DecRef decrements the reference count.
func (r *LeafReader) DecRef() error {
	return r.IndexReader.DecRef()
}

// TryIncRef tries to increment the reference count.
func (r *LeafReader) TryIncRef() bool {
	return r.IndexReader.TryIncRef()
}

// GetRefCount returns the current reference count.
func (r *LeafReader) GetRefCount() int32 {
	return r.IndexReader.GetRefCount()
}

// GetTermVectors returns the term vectors for a document.
// This method should be overridden by SegmentReader which has access to the codec readers.
func (r *LeafReader) GetTermVectors(docID int) (Fields, error) {
	// Base implementation returns nil - SegmentReader will override this
	return nil, nil
}

// Terms returns the Terms for a field.
// This method should be overridden by SegmentReader which has access to the codec readers.
func (r *LeafReader) Terms(field string) (Terms, error) {
	// Base implementation returns nil - SegmentReader will override this
	return nil, nil
}

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *LeafReader) StoredFields() (StoredFields, error) {
	return nil, fmt.Errorf("StoredFields must be implemented by subclass")
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *LeafReader) TermVectors() (TermVectors, error) {
	return nil, fmt.Errorf("TermVectors must be implemented by subclass")
}

// GetContext returns the reader context for this leaf reader.
func (r *LeafReader) GetContext() (IndexReaderContext, error) {
	return NewLeafReaderContext(r, nil, 0, 0), nil
}

// Leaves returns all leaf reader contexts (just this one for a leaf).
func (r *LeafReader) Leaves() ([]*LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	return []*LeafReaderContext{ctx.(*LeafReaderContext)}, nil
}

// SegmentCoreReadersHolder holds a reference to SegmentCoreReaders.
// This is used by SegmentReader to access codec readers.
type SegmentCoreReadersHolder interface {
	// GetCoreReaders returns the SegmentCoreReaders.
	GetCoreReaders() *SegmentCoreReaders
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

	// readerContext is the context for this reader
	readerContext IndexReaderContext
}

// SegmentReader is a LeafReader for a specific segment.
type SegmentReader struct {
	*LeafReader
	segmentCommitInfo *SegmentCommitInfo
	coreReaders       *SegmentCoreReaders
	fieldInfos        *FieldInfos
	codec             Codec
}

// NewSegmentReader creates a new SegmentReader.
func NewSegmentReader(segmentCommitInfo *SegmentCommitInfo) *SegmentReader {
	return &SegmentReader{
		LeafReader:        NewLeafReader(segmentCommitInfo.SegmentInfo()),
		segmentCommitInfo: segmentCommitInfo,
	}
}

// NewSegmentReaderWithCore creates a new SegmentReader with core readers.
func NewSegmentReaderWithCore(
	segmentCommitInfo *SegmentCommitInfo,
	coreReaders *SegmentCoreReaders,
	fieldInfos *FieldInfos,
	codec Codec,
) *SegmentReader {
	return &SegmentReader{
		LeafReader:        NewLeafReader(segmentCommitInfo.SegmentInfo()),
		segmentCommitInfo: segmentCommitInfo,
		coreReaders:       coreReaders,
		fieldInfos:        fieldInfos,
		codec:             codec,
	}
}

// GetSegmentCommitInfo returns the SegmentCommitInfo for this reader.
func (r *SegmentReader) GetSegmentCommitInfo() *SegmentCommitInfo {
	return r.segmentCommitInfo
}

// GetCoreReaders returns the SegmentCoreReaders for this reader.
func (r *SegmentReader) GetCoreReaders() *SegmentCoreReaders {
	return r.coreReaders
}

// GetFieldInfos returns the FieldInfos for this reader.
func (r *SegmentReader) GetFieldInfos() *FieldInfos {
	return r.fieldInfos
}

// NumDocs returns the number of live documents in this segment.
func (r *SegmentReader) NumDocs() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.NumDocs()
}

// MaxDoc returns the maximum document ID (one past the last doc) for this segment.
func (r *SegmentReader) MaxDoc() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.SegmentInfo().DocCount()
}

// GetTermVectors returns the term vectors for a document.
// Implements LeafReader.GetTermVectors by delegating to the TermVectorsReader.
func (r *SegmentReader) GetTermVectors(docID int) (Fields, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized: core readers are nil")
	}

	tvReader := r.coreReaders.GetTermVectorsReader()
	if tvReader == nil {
		// No term vectors stored for this segment
		return nil, nil
	}

	// Validate document ID
	if docID < 0 || docID >= r.DocCount() {
		return nil, fmt.Errorf("document ID %d out of range [0, %d)", docID, r.DocCount())
	}

	return tvReader.Get(docID)
}

// Terms returns the Terms for a field.
// Implements LeafReader.Terms by delegating to the FieldsProducer.
func (r *SegmentReader) Terms(field string) (Terms, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized: core readers are nil")
	}

	fields := r.coreReaders.GetFields()
	if fields == nil {
		// No indexed fields in this segment
		return nil, nil
	}

	return fields.Terms(field)
}

// Close closes the SegmentReader and releases resources.
func (r *SegmentReader) Close() error {
	var lastErr error

	if r.coreReaders != nil {
		if err := r.coreReaders.DecRef(); err != nil {
			lastErr = err
		}
		r.coreReaders = nil
	}

	return lastErr
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
	if commit == nil {
		return nil, fmt.Errorf("commit cannot be nil")
	}

	// Get the SegmentInfos from the commit
	segmentInfos := commit.GetSegmentInfos()
	if segmentInfos == nil {
		return nil, fmt.Errorf("commit has no segment infos")
	}

	// Create the DirectoryReader with the commit's segment infos
	reader := &DirectoryReader{
		LeafReader:   NewLeafReader(nil),
		directory:    directory,
		segmentInfos: segmentInfos,
		readers:      make([]*SegmentReader, 0, segmentInfos.Size()),
	}

	// Create a reader for each segment in the commit
	for i := 0; i < segmentInfos.Size(); i++ {
		segmentCommitInfo := segmentInfos.Get(i)
		segmentReader := NewSegmentReader(segmentCommitInfo)
		reader.readers = append(reader.readers, segmentReader)
	}

	return reader, nil
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
	return segmentInfos.Generation() == r.segmentInfos.Generation(), nil
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

// GetIndexCommit returns the IndexCommit that this reader is reading from.
func (r *DirectoryReader) GetIndexCommit() *IndexCommit {
	if r.segmentInfos == nil {
		return nil
	}
	commit := NewIndexCommit(r.segmentInfos)
	commit.SetDirectory(r.directory)
	return commit
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
	var lastErr error
	for _, reader := range r.readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}
	r.readers = nil
	return lastErr
}

// GetTermVectors returns term vectors for the given document across all segments.
// Note: This requires mapping the document ID to the correct segment.
// For a DirectoryReader, document IDs are sequential across segments.
func (r *DirectoryReader) GetTermVectors(docID int) (Fields, error) {
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
func (r *DirectoryReader) Terms(field string) (Terms, error) {
	// Collect Terms from all segments that have the field
	// For simplicity, return the Terms from the first segment that has it
	// A full implementation would merge Terms from all segments
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

// GetLiveDocs returns a bitset of live documents.
// Returns nil if there are no deletions.
func (r *DirectoryReader) GetLiveDocs() []bool {
	// For now, return nil (all documents are live)
	// A full implementation would track deleted documents
	return nil
}

// GetSequentialSubReaders returns the sequential sub-readers.
func (r *DirectoryReader) GetSequentialSubReaders() []*SegmentReader {
	return r.readers
}

// GetReaderCount returns the number of segment readers.
func (r *DirectoryReader) GetReaderCount() int {
	return len(r.readers)
}

// GetLastCommit returns the last committed segment infos.
func (r *DirectoryReader) GetLastCommit() *SegmentInfos {
	return r.lastCommittedInfos
}

// HasDeletions returns true if any segment has deleted documents.
func (r *DirectoryReader) HasDeletions() bool {
	for _, reader := range r.readers {
		if reader.HasDeletions() {
			return true
		}
	}
	return false
}

// NumDeletedDocs returns the total number of deleted documents across all segments.
func (r *DirectoryReader) NumDeletedDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDeletedDocs()
	}
	return total
}

// EnsureOpen throws an error if the reader is closed.
func (r *DirectoryReader) EnsureOpen() error {
	if r.closed.Load() {
		return ErrAlreadyClosed
	}
	return nil
}

// IncRef increments the reference count on all sub-readers.
func (r *DirectoryReader) IncRef() error {
	if err := r.EnsureOpen(); err != nil {
		return err
	}
	for _, reader := range r.readers {
		if err := reader.IncRef(); err != nil {
			return err
		}
	}
	return nil
}

// DecRef decrements the reference count on all sub-readers.
func (r *DirectoryReader) DecRef() error {
	var lastErr error
	for _, reader := range r.readers {
		if err := reader.DecRef(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// TryIncRef tries to increment the reference count.
func (r *DirectoryReader) TryIncRef() bool {
	if r.closed.Load() {
		return false
	}
	for _, reader := range r.readers {
		if !reader.TryIncRef() {
			// Rollback
			// Note: This is simplified; a full implementation would track which refs were incremented
			return false
		}
	}
	return true
}

// GetRefCount returns the minimum reference count across all sub-readers.
func (r *DirectoryReader) GetRefCount() int32 {
	if len(r.readers) == 0 {
		return 0
	}
	minRefCount := r.readers[0].GetRefCount()
	for _, reader := range r.readers[1:] {
		rc := reader.GetRefCount()
		if rc < minRefCount {
			minRefCount = rc
		}
	}
	return minRefCount
}

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *DirectoryReader) StoredFields() (StoredFields, error) {
	// For a DirectoryReader, this would need to aggregate across segments
	// Return a wrapper that delegates to the appropriate segment
	return &directoryStoredFields{reader: r}, nil
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *DirectoryReader) TermVectors() (TermVectors, error) {
	// For a DirectoryReader, this would need to aggregate across segments
	// Return a wrapper that delegates to the appropriate segment
	return &directoryTermVectors{reader: r}, nil
}

// GetContext returns the reader context for this directory reader.
func (r *DirectoryReader) GetContext() (IndexReaderContext, error) {
	if r.readerContext == nil {
		ctx, err := buildDirectoryReaderContext(r, nil)
		if err != nil {
			return nil, err
		}
		r.readerContext = ctx
	}
	return r.readerContext, nil
}

// Leaves returns all leaf reader contexts from all segments.
func (r *DirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	return ctx.(*CompositeReaderContext).Leaves(), nil
}

// buildDirectoryReaderContext builds the context hierarchy for a DirectoryReader.
func buildDirectoryReaderContext(reader *DirectoryReader, parent IndexReaderContext) (*CompositeReaderContext, error) {
	children := make([]IndexReaderContext, len(reader.readers))
	leaves := make([]*LeafReaderContext, 0)
	docBase := 0

	for i, subReader := range reader.readers {
		leafCtx := NewLeafReaderContext(subReader.LeafReader, parent, i, docBase)
		children[i] = leafCtx
		leaves = append(leaves, leafCtx)
		docBase += subReader.MaxDoc()
	}

	return NewCompositeReaderContextWithChildren(reader, parent, children, leaves), nil
}

// directoryStoredFields wraps a DirectoryReader to provide StoredFields access.
type directoryStoredFields struct {
	reader *DirectoryReader
}

// Prefetch prefetches stored fields for the given document IDs.
func (dsf *directoryStoredFields) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Document retrieves the stored fields for a document using the visitor pattern.
func (dsf *directoryStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	remainingDocID := docID
	for _, subReader := range dsf.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			sf, err := subReader.StoredFields()
			if err != nil {
				return err
			}
			if sf == nil {
				return nil
			}
			return sf.Document(remainingDocID, visitor)
		}
		remainingDocID -= maxDoc
	}
	return fmt.Errorf("document ID %d out of range", docID)
}

// directoryTermVectors wraps a DirectoryReader to provide TermVectors access.
type directoryTermVectors struct {
	reader *DirectoryReader
}

// Prefetch prefetches term vectors for the given document IDs.
func (dtv *directoryTermVectors) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Get retrieves the term vectors for a document.
func (dtv *directoryTermVectors) Get(docID int) (Fields, error) {
	remainingDocID := docID
	for _, subReader := range dtv.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			tv, err := subReader.TermVectors()
			if err != nil {
				return nil, err
			}
			if tv == nil {
				return nil, nil
			}
			return tv.Get(remainingDocID)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// GetField retrieves the term vector for a specific field in a document.
func (dtv *directoryTermVectors) GetField(docID int, field string) (Terms, error) {
	remainingDocID := docID
	for _, subReader := range dtv.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			tv, err := subReader.TermVectors()
			if err != nil {
				return nil, err
			}
			if tv == nil {
				return nil, nil
			}
			return tv.GetField(remainingDocID, field)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// ListCommits returns all commits in the given directory.
// This is the Go equivalent of Lucene's DirectoryReader.listCommits().
func ListCommits(dir store.Directory) (IndexCommitList, error) {
	// Read the current segment infos to find all commits
	si, err := ReadSegmentInfos(dir)
	if err != nil {
		return nil, err
	}

	// For now, just return the current commit
	// In a full implementation, this would scan the directory for all
	// segments files and create an IndexCommit for each one
	commit := NewIndexCommit(si)
	commit.SetDirectory(dir)

	return IndexCommitList{commit}, nil
}

// Ensure DirectoryReader implements IndexReaderInterface
var _ IndexReaderInterface = (*DirectoryReader)(nil)

// SegmentReader additional methods

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *SegmentReader) StoredFields() (StoredFields, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized")
	}
	sfReader := r.coreReaders.GetStoredFieldsReader()
	if sfReader == nil {
		return NewEmptyStoredFields(), nil
	}
	liveDocs := r.GetLiveDocs()
	return NewStoredFields(sfReader, liveDocs), nil
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *SegmentReader) TermVectors() (TermVectors, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized")
	}
	tvReader := r.coreReaders.GetTermVectorsReader()
	if tvReader == nil {
		return NewEmptyTermVectors(), nil
	}
	liveDocs := r.GetLiveDocs()
	return NewTermVectors(tvReader, liveDocs), nil
}

// GetLiveDocs returns the live docs Bits for this segment.
func (r *SegmentReader) GetLiveDocs() util.Bits {
	// Check for deletions in the segment commit info
	if r.segmentCommitInfo != nil && r.segmentCommitInfo.HasDeletions() {
		// Return the live docs bitset
		// This would be populated when the segment is loaded
		return r.LeafReader.GetLiveDocs()
	}
	return nil
}

// Ensure SegmentReader implements IndexReaderInterface
var _ IndexReaderInterface = (*SegmentReader)(nil)
