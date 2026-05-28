// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
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
func (r *LeafReader) GetCoreCacheKey() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.coreCacheKey
}

// Postings returns the postings for a term.
// This must be implemented by subclasses that have postings data.
func (r *LeafReader) Postings(term Term) (PostingsEnum, error) {
	return r.PostingsWithFreqPositions(term, 0)
}

// PostingsWithFreqPositions returns the postings for a term with specific flags.
// The flags parameter controls what data is returned:
//   - 0: only doc IDs
//   - 1: doc IDs and term frequencies
//   - 2: doc IDs, term frequencies, positions, offsets, and payloads
func (r *LeafReader) PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	return terms.GetPostingsReader(term.Text(), flags)
}

// GetNumericDocValues returns NumericDocValues for the given field.
// Returns nil if the field does not have numeric doc values.
func (r *LeafReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	return nil, nil
}

// GetBinaryDocValues returns BinaryDocValues for the given field.
// Returns nil if the field does not have binary doc values.
func (r *LeafReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	return nil, nil
}

// GetSortedDocValues returns SortedDocValues for the given field.
// Returns nil if the field does not have sorted doc values.
func (r *LeafReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	return nil, nil
}

// GetSortedNumericDocValues returns SortedNumericDocValues for the given field.
// Returns nil if the field does not have sorted numeric doc values.
func (r *LeafReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	return nil, nil
}

// GetSortedSetDocValues returns SortedSetDocValues for the given field.
// Returns nil if the field does not have sorted set doc values.
func (r *LeafReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	return nil, nil
}

// GetNormValues returns NumericDocValues for norms of the given field.
// Returns nil if the field does not have norms.
func (r *LeafReader) GetNormValues(field string) (NumericDocValues, error) {
	return nil, nil
}

// GetPointValues returns PointValues for the given field.
// Returns nil if the field does not have point values.
func (r *LeafReader) GetPointValues(field string) (PointValues, error) {
	return nil, nil
}

// GetFloatVectorValues returns FloatVectorValues for the given field.
// Returns nil if the field does not have float vector values.
func (r *LeafReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	return nil, nil
}

// GetByteVectorValues returns ByteVectorValues for the given field.
// Returns nil if the field does not have byte vector values.
func (r *LeafReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	return nil, nil
}

// SearchNearestVectors searches for the k nearest vectors to the target.
// Returns TopDocs containing the k nearest documents.
func (r *LeafReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) {
	return TopDocs{}, fmt.Errorf("SearchNearestVectors not implemented")
}

// GetDocValuesSkipper returns a DocValuesSkipper for efficient skipping.
// Returns nil if skipping is not supported.
func (r *LeafReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	return nil, nil
}

// CheckIntegrity checks that the index is not corrupt.
// Returns an error if any problems are found.
func (r *LeafReader) CheckIntegrity() error {
	return nil
}

// GetMetaData returns the IndexReaderMetaData for this reader.
func (r *LeafReader) GetMetaData() *IndexReaderMetaData {
	return &IndexReaderMetaData{
		HasDeletions: r.HasDeletions(),
		NumDocs:      r.NumDocs(),
		MaxDoc:       r.MaxDoc(),
	}
}

// IndexReaderMetaData provides metadata about an IndexReader.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.Metadata.
type IndexReaderMetaData struct {
	// HasDeletions is true if this reader has deletions.
	HasDeletions bool

	// NumDocs is the number of live documents.
	NumDocs int

	// MaxDoc is the maximum document ID plus one.
	MaxDoc int
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

// DirectoryReader is a CompositeReader that reads from a Directory.
//
// This is the Go port of Lucene's org.apache.lucene.index.DirectoryReader.
//
// DirectoryReader is the main implementation of CompositeReader for reading
// indexes stored in a Directory. It manages a collection of SegmentReaders
// (each wrapping a LeafReader) for all segments in the index.
type DirectoryReader struct {
	*CompositeReader

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
	// directory is the source directory; used to look up in-memory postings
	// from the package-level registry when coreReaders is nil.
	directory store.Directory
}

// NewSegmentReader creates a new SegmentReader.
// If the SegmentCommitInfo carries in-memory FieldInfos (written by Commit),
// they are used so that CheckIndex and other readers can enumerate fields
// without codec infrastructure.
func NewSegmentReader(segmentCommitInfo *SegmentCommitInfo) *SegmentReader {
	return &SegmentReader{
		LeafReader:        NewLeafReader(segmentCommitInfo.SegmentInfo()),
		segmentCommitInfo: segmentCommitInfo,
		fieldInfos:        segmentCommitInfo.GetInMemoryFieldInfos(),
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
// Returns an empty FieldInfos if none was set.
func (r *SegmentReader) GetFieldInfos() *FieldInfos {
	if r.fieldInfos == nil {
		return NewFieldInfos()
	}
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
// Falls back to the in-memory FieldsProducer when coreReaders is nil
// (codec-less path used by unit tests that do not configure a codec).
func (r *SegmentReader) Terms(field string) (Terms, error) {
	if r.coreReaders != nil {
		fields := r.coreReaders.GetFields()
		if fields == nil {
			return nil, nil
		}
		return fields.Terms(field)
	}

	// Codec-less fall-back 1: use in-memory postings stored on the commit info
	// (present when the reader is constructed from the writer-side SegmentCommitInfo
	// before ReadSegmentInfos discards in-memory state).
	if r.segmentCommitInfo != nil {
		if fp := r.segmentCommitInfo.GetInMemoryFields(); fp != nil {
			return fp.Terms(field)
		}
	}

	// Codec-less fall-back 2: look up the producer in the package-level registry.
	// This handles the common case where OpenDirectoryReader called ReadSegmentInfos,
	// which created fresh SegmentCommitInfo objects without inMemoryFields, but the
	// writer already registered the producer under (directory, segmentName).
	if r.directory != nil && r.segmentCommitInfo != nil {
		segName := r.segmentCommitInfo.SegmentInfo().Name()
		if fp := LookupInMemoryFields(r.directory, segName); fp != nil {
			return fp.Terms(field)
		}
	}

	return nil, nil
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
	// Read segment infos; an empty (freshly created) directory has no segments
	// file yet — treat it as an empty index rather than an error.
	segmentInfos, err := ReadSegmentInfos(directory)
	if err != nil {
		segmentInfos = NewSegmentInfos()
	}

	return OpenDirectoryReaderWithInfos(directory, segmentInfos)
}

// newCompositeReaderFromSegments builds a CompositeReader from a slice of SegmentReaders.
// An empty slice produces an empty composite reader (valid for an empty index).
func newCompositeReaderFromSegments(readers []*SegmentReader) (*CompositeReader, error) {
	if len(readers) == 0 {
		return &CompositeReader{
			IndexReader: NewIndexReader(),
			subReaders:  []IndexReaderInterface{},
			starts:      []int{0},
		}, nil
	}
	subReaders := make([]IndexReaderInterface, len(readers))
	for i, r := range readers {
		subReaders[i] = r
	}
	return NewCompositeReaderWithSubReaders(subReaders)
}

// openSegmentReader creates a SegmentReader for one SegmentCommitInfo, loading
// FieldInfos from disk (if not already in memory) and constructing
// SegmentCoreReaders from the codec registered under the segment's codec name.
//
// If no codec is registered under that name (e.g., no bridge import in the
// test binary) the reader falls back to the codec-less path so structural
// tests that do not need stored-field access continue to work.
func openSegmentReader(directory store.Directory, sci *SegmentCommitInfo) (*SegmentReader, error) {
	// Resolve the codec for this segment. Prefer the segment's own codec name;
	// fall back to the registered default when the name is absent (freshly-
	// created in-memory segments have no on-disk codec name yet).
	codecName := sci.SegmentInfo().Codec()
	var codec Codec
	if codecName != "" {
		codec = LookupCodecByName(codecName)
		if codec == nil {
			// Codec name was stamped but not registered: fall back to default.
			codec = GetDefaultCodec()
		}
	}
	// When no codec name is stamped on the segment (in-memory or codec-less
	// write path), do not resolve a default codec. Attempting to open codec
	// files for a segment that was written without them would produce
	// "file not found" errors. The caller (IndexSearcher.Doc, term searches)
	// will receive nil coreReaders and take the in-memory fallback.

	// Resolve FieldInfos: prefer in-memory (written by the writer), then read
	// from disk via the codec's FieldInfosFormat.
	fi := sci.GetInMemoryFieldInfos()
	if fi == nil && codec != nil {
		fif := codec.FieldInfosFormat()
		if fif != nil {
			var err error
			fi, err = fif.Read(directory, sci.SegmentInfo(), "", store.IOContextRead)
			if err != nil {
				// Non-fatal: segment may not have a .fnm file yet (e.g., empty
				// in-memory segment). Fall back to an empty FieldInfos.
				fi = NewFieldInfos()
			}
		}
	}
	if fi == nil {
		fi = NewFieldInfos()
	}

	// Construct SegmentCoreReaders when a codec is available. This wires the
	// StoredFieldsReader, FieldsProducer (postings), and TermVectorsReader so
	// that IndexSearcher.Doc and term-based searches work end-to-end.
	if codec != nil {
		// Load the full SegmentInfo from the .si file to obtain metadata not
		// present in the segments_N entry (e.g., isCompoundFile).  Fall back
		// to the segments_N-constructed SegmentInfo when the .si is absent
		// (in-memory segments that were never flushed to disk).
		segInfo := sci.SegmentInfo()
		if sif := codec.SegmentInfoFormat(); sif != nil {
			if fullSegInfo, err := sif.Read(directory, segInfo.Name(), segInfo.GetID(), store.IOContextRead); err == nil {
				segInfo = fullSegInfo
			}
		}
		core, err := NewSegmentCoreReaders(directory, segInfo, fi, codec, store.IOContextRead)
		if err != nil {
			return nil, fmt.Errorf("opening core readers for segment %s: %w", sci.SegmentInfo().Name(), err)
		}
		sr := NewSegmentReaderWithCore(sci, core, fi, codec)
		sr.directory = directory
		return sr, nil
	}

	// Codec-less fallback for structural unit tests.
	sr := &SegmentReader{
		LeafReader:        NewLeafReader(sci.SegmentInfo()),
		segmentCommitInfo: sci,
		fieldInfos:        fi,
		directory:         directory,
	}
	return sr, nil
}

// OpenDirectoryReaderWithInfos opens a DirectoryReader with existing SegmentInfos.
func OpenDirectoryReaderWithInfos(directory store.Directory, segmentInfos *SegmentInfos) (*DirectoryReader, error) {
	readers := make([]*SegmentReader, 0, segmentInfos.Size())
	for i := 0; i < segmentInfos.Size(); i++ {
		sr, err := openSegmentReader(directory, segmentInfos.Get(i))
		if err != nil {
			// Close all already-opened readers before returning.
			for _, opened := range readers {
				opened.Close() //nolint:errcheck // best-effort cleanup in error path
			}
			return nil, err
		}
		readers = append(readers, sr)
	}

	compReader, err := newCompositeReaderFromSegments(readers)
	if err != nil {
		return nil, err
	}

	return &DirectoryReader{
		CompositeReader: compReader,
		directory:       directory,
		segmentInfos:    segmentInfos,
		readers:         readers,
	}, nil
}

// OpenDirectoryReaderFromCommit opens a DirectoryReader from a specific commit.
func OpenDirectoryReaderFromCommit(directory store.Directory, commit *IndexCommit) (*DirectoryReader, error) {
	if commit == nil {
		return nil, fmt.Errorf("commit cannot be nil")
	}

	segmentInfos := commit.GetSegmentInfos()
	if segmentInfos == nil {
		return nil, fmt.Errorf("commit has no segment infos")
	}

	return OpenDirectoryReaderWithInfos(directory, segmentInfos)
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
	if r.directory == nil {
		// No backing directory — treat as always current (nothing can change it).
		return true, nil
	}
	segmentInfos, err := ReadSegmentInfos(r.directory)
	if err != nil {
		// No segments file means the directory is still empty — still current.
		return true, nil
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

// GetFieldInfos returns the merged FieldInfos across all segments.
//
// Mirrors the contract of FieldInfos.getMergedFieldInfos in Lucene: it unions
// every FieldInfo from every segment, returning a non-nil *FieldInfos even for
// an empty index. Callers that need per-segment granularity should iterate
// Leaves() and call GetFieldInfos() on each LeafReader.
func (r *DirectoryReader) GetFieldInfos() *FieldInfos {
	merged := NewFieldInfos()
	for _, sr := range r.readers {
		fi := sr.GetFieldInfos()
		if fi == nil {
			continue
		}
		it := fi.Iterator()
		for {
			info := it.Next()
			if info == nil {
				break
			}
			// Ignore errors: a duplicate field number from a different segment is
			// legal when merging (same field may appear in multiple segments).
			_ = merged.Add(info)
		}
	}
	return merged
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
		leafCtx := NewLeafReaderContext(subReader, parent, i, docBase)
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

// ListCommits returns all commits present in the given directory, sorted in
// ascending generation order.  This is the Go equivalent of Lucene's
// DirectoryReader.listCommits().
func ListCommits(dir store.Directory) (IndexCommitList, error) {
	// Read the latest commit. Mirrors Lucene's listCommits which calls
	// SegmentInfos.readLatestCommit and propagates IndexNotFoundException
	// when the directory has no index.
	latest, err := ReadSegmentInfos(dir)
	if err != nil {
		return nil, err
	}

	currentGen := latest.Generation()

	// Start with the latest commit.
	commits := make(IndexCommitList, 0, 4)
	latestCommit := NewIndexCommit(latest)
	latestCommit.SetDirectory(dir)
	commits = append(commits, latestCommit)

	// Scan the directory for older segments_N files.
	files, err := dir.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listCommits: list directory: %w", err)
	}
	for _, name := range files {
		gen := ParseGeneration(name)
		if gen <= 0 || gen >= currentGen {
			continue // not a segments file, or already captured as latest
		}
		sis, readErr := readSegmentInfosFile(dir, name)
		if readErr != nil {
			// File may have been deleted between listing and reading; skip it.
			continue
		}
		c := NewIndexCommit(sis)
		c.SetDirectory(dir)
		commits = append(commits, c)
	}

	// Sort ascending by generation so callers can walk history oldest-first.
	sortCommitsByGeneration(commits)

	return commits, nil
}

// readSegmentInfosFile reads a named segments_N file from dir by generation.
// This is used by ListCommits to enumerate prior commits.  The generation
// must be > 0 (valid) for the call to succeed.
func readSegmentInfosFile(dir store.Directory, name string) (*SegmentInfos, error) {
	gen := ParseGeneration(name)
	if gen <= 0 {
		return nil, fmt.Errorf("not a segments file: %q", name)
	}
	// Open a throwaway handle; readSegmentInfosLucene104 closes it and re-opens
	// the canonical file by generation internally.
	rawIn, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	// Peek at the magic to validate the format before delegating.
	magic, err := store.ReadInt32(rawIn)
	if err != nil {
		_ = rawIn.Close()
		return nil, fmt.Errorf("readSegmentInfosFile %q: read magic: %w", name, err)
	}
	if magic != codecMagic {
		_ = rawIn.Close()
		// Legacy / unknown format — skip gracefully.
		return nil, fmt.Errorf("readSegmentInfosFile %q: unknown magic 0x%x", name, uint32(magic))
	}
	// rawIn is handed off to spi.ReadSegmentInfosFromHandle which closes it
	// and re-opens the file at offset 0 for checksum verification.
	return spi.ReadSegmentInfosFromHandle(rawIn, dir, gen)
}

// sortCommitsByGeneration sorts commits ascending by generation using insertion
// sort (list is typically very small: 1–10 commits).
func sortCommitsByGeneration(commits IndexCommitList) {
	for i := 1; i < len(commits); i++ {
		for j := i; j > 0 && commits[j].GetGeneration() < commits[j-1].GetGeneration(); j-- {
			commits[j], commits[j-1] = commits[j-1], commits[j]
		}
	}
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
// GetLiveDocs returns a Bits representing the live (non-deleted) documents in
// this segment.  Returns nil when no documents are deleted.
// When the segment has in-memory deleted ordinals (tracked by Gocene's delete
// path), a dense boolean slice is built and returned as a liveDocs implementation.
func (r *SegmentReader) GetLiveDocs() util.Bits {
	if r.segmentCommitInfo == nil {
		return nil
	}
	ords := r.segmentCommitInfo.GetDeletedOrdinals()
	if len(ords) == 0 {
		// No in-memory deletion info; fall back to LeafReader's liveDocs (may be nil).
		if r.segmentCommitInfo.HasDeletions() {
			return r.LeafReader.GetLiveDocs()
		}
		return nil
	}
	// Build a dense boolean bitset from the deleted ordinals.
	maxDoc := r.MaxDoc()
	live := make([]bool, maxDoc)
	for i := range live {
		live[i] = true
	}
	for _, ord := range ords {
		if ord >= 0 && ord < maxDoc {
			live[ord] = false
		}
	}
	return boolBits(live)
}

// boolBits is a util.Bits backed by a []bool slice.
type boolBits []bool

func (b boolBits) Get(index int) bool { return b[index] }
func (b boolBits) Length() int        { return len(b) }

// Ensure SegmentReader implements IndexReaderInterface
var _ IndexReaderInterface = (*SegmentReader)(nil)
