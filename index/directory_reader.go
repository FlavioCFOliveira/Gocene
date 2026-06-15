// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strings"
	"strconv"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
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

	// nrtGen records the IndexWriter NRT generation at the moment this
	// reader was created from a writer.  Zero means commit-pinned reader.
	// Used by OpenIfChangedFromWriter for efficient change detection.
	nrtGen int64
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

// NumDeletedDocs returns the number of deleted documents (hard + soft) in this
// segment, taken from the SegmentCommitInfo so it stays consistent with NumDocs
// (i.e. equals MaxDoc-NumDocs). Without this override the embedded
// LeafReader.NumDeletedDocs would return the base IndexReader's 0, which is
// wrong for a committed segment carrying .liv deletions (rmp #12).
func (r *SegmentReader) NumDeletedDocs() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.DelCount() + r.segmentCommitInfo.SoftDelCount()
}

// HasDeletions reports whether this segment carries any deleted documents,
// consistent with NumDeletedDocs (rmp #12).
func (r *SegmentReader) HasDeletions() bool {
	return r.NumDeletedDocs() > 0
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

// knnVectorsReaderDelegate is the per-encoding read surface exposed by the
// codec's KNN vectors reader (stored on SegmentCoreReaders.vectorsReader as
// an interface{}). The concrete reader — *codecs.PerFieldKnnVectorsReader
// wrapping *codecs.Lucene99HnswVectorsReader — satisfies it structurally.
//
// The contract uses only types the index package can name: the index-facing
// [FloatVectorValues] / [ByteVectorValues] interfaces (the codec adapters
// implement both their own and these) and [utilhnsw.TopDocs] (shared by
// index and codecs, which both import util/hnsw without a cycle).
type knnVectorsReaderDelegate interface {
	FloatVectorValues(field string) (FloatVectorValues, error)
	ByteVectorValues(field string) (ByteVectorValues, error)
	SearchNearestFloat(field string, target []float32, k int, acceptDocs util.Bits) (*utilhnsw.TopDocs, error)
	SearchNearestByte(field string, target []byte, k int, acceptDocs util.Bits) (*utilhnsw.TopDocs, error)
	SearchNearestFloatCollector(field string, target []float32, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
	SearchNearestByteCollector(field string, target []byte, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
}

// vectorsDelegate narrows the core readers' KNN vectors reader to the
// per-encoding read surface, or returns nil when the segment has no vectors
// reader (e.g. no vector fields, or the codec-less test path).
func (r *SegmentReader) vectorsDelegate() knnVectorsReaderDelegate {
	if r.coreReaders == nil {
		return nil
	}
	vr := r.coreReaders.GetVectorReader()
	if vr == nil {
		return nil
	}
	d, ok := vr.(knnVectorsReaderDelegate)
	if !ok {
		return nil
	}
	return d
}

// GetFloatVectorValues returns the float vectors for field, delegating to
// the codec's KNN vectors reader. Returns (nil, nil) when the segment has no
// vectors reader or no delegate owns the field (matching the LeafReader
// contract). Implements LeafReader.GetFloatVectorValues.
func (r *SegmentReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.FloatVectorValues(field)
}

// GetByteVectorValues returns the byte vectors for field, delegating to the
// codec's KNN vectors reader. Implements LeafReader.GetByteVectorValues.
func (r *SegmentReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.ByteVectorValues(field)
}

// SearchNearestVectors searches for the k nearest float vectors to target in
// field, delegating to the codec's KNN vectors reader and translating the
// result to the index-package [TopDocs]. Returns an empty TopDocs when the
// segment has no vectors reader. Implements LeafReader.SearchNearestVectors.
//
// acceptDocs is applied as a query-time live-docs filter inside the codec
// reader, independent of whether the field is stored densely or sparsely.
func (r *SegmentReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return TopDocs{}, nil
	}
	td, err := d.SearchNearestFloat(field, target, k, acceptDocs)
	if err != nil {
		return TopDocs{}, err
	}
	return knnTopDocsToIndex(td), nil
}

// SearchNearestVectorsByte is the byte-vector analogue of
// SearchNearestVectors. It is not part of the LeafReader interface yet (no
// byte SearchNearestVectors method exists on the base type) but is exposed
// so byte KNN queries can reach the codec search path.
func (r *SegmentReader) SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return TopDocs{}, nil
	}
	td, err := d.SearchNearestByte(field, target, k, acceptDocs)
	if err != nil {
		return TopDocs{}, err
	}
	return knnTopDocsToIndex(td), nil
}

// SearchNearestVectorsCollector runs collector-driven nearest-neighbour
// float-vector search for target in field, driving the caller-supplied
// collector through the codec's HNSW traversal instead of an internally
// created top-k collector. The collector observes leaf-local document ids
// and is responsible for any further result shaping (e.g. parent-block
// diversification).
//
// It is a no-op (leaves the collector empty, returns nil) when the segment
// has no vectors reader. Mirrors LeafReader.searchNearestVectors(field,
// target, KnnCollector, acceptDocs) in Lucene, which delegates straight to
// the codec KnnVectorsReader.
func (r *SegmentReader) SearchNearestVectorsCollector(field string, target []float32, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error {
	d := r.vectorsDelegate()
	if d == nil {
		return nil
	}
	return d.SearchNearestFloatCollector(field, target, collector, acceptDocs)
}

// SearchNearestVectorsByteCollector is the byte-vector analogue of
// [SegmentReader.SearchNearestVectorsCollector].
func (r *SegmentReader) SearchNearestVectorsByteCollector(field string, target []byte, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error {
	d := r.vectorsDelegate()
	if d == nil {
		return nil
	}
	return d.SearchNearestByteCollector(field, target, collector, acceptDocs)
}

// pointsReaderDelegate is the wide read surface exposed by the codec's points
// reader (stored on SegmentCoreReaders.pointsReader as an interface{}). The
// concrete reader — the BKD-backed reader from the codecs/lucene90 sub-package
// — satisfies it structurally via its GetValues accessor (the Go counterpart
// of org.apache.lucene.codecs.PointsReader.getValues). The contract uses only
// the index-facing [PointValues] type, so the index package can name it
// without importing codecs.
type pointsReaderDelegate interface {
	GetValues(field string) (PointValues, error)
}

// pointsDelegate narrows the core readers' points reader to the wide
// getValues surface, or returns nil when the segment has no points reader
// (e.g. no point fields, or the codec-less test path).
func (r *SegmentReader) pointsDelegate() pointsReaderDelegate {
	if r.coreReaders == nil {
		return nil
	}
	pr := r.coreReaders.GetPointsReader()
	if pr == nil {
		return nil
	}
	d, ok := pr.(pointsReaderDelegate)
	if !ok {
		return nil
	}
	return d
}

// GetPointValues returns the BKD point values for field, delegating to the
// codec's points reader. Returns (nil, nil) when the segment has no points
// reader or the field has no indexed points (matching the LeafReader
// contract). This overrides the embedded LeafReader.GetPointValues, which
// unconditionally returns (nil, nil). Mirrors
// org.apache.lucene.index.SegmentReader.getPointValues / CodecReader.
func (r *SegmentReader) GetPointValues(field string) (PointValues, error) {
	d := r.pointsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.GetValues(field)
}

// docValuesProducerDelegate is the read surface exposed by the codec's
// doc-values producer (stored on SegmentCoreReaders.docValuesProducer as an
// interface{}). The concrete producer — Lucene90DocValuesProducer from the
// codecs package — satisfies it structurally via its Get* accessors (the Go
// counterpart of org.apache.lucene.codecs.DocValuesProducer.getNumeric etc.).
// The contract names only the index-facing doc-values value types (themselves
// aliases of the spi types) and *FieldInfo (an alias of schema.FieldInfo), so
// the index package can name it without importing codecs.
type docValuesProducerDelegate interface {
	GetNumeric(field *FieldInfo) (NumericDocValues, error)
	GetBinary(field *FieldInfo) (BinaryDocValues, error)
	GetSorted(field *FieldInfo) (SortedDocValues, error)
	GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error)
	GetSortedSet(field *FieldInfo) (SortedSetDocValues, error)
}

// docValuesDelegate narrows the core readers' doc-values producer to the
// Get* surface, or returns nil when the segment has no doc-values producer
// (e.g. no doc-values fields, or the codec-less test path).
func (r *SegmentReader) docValuesDelegate() docValuesProducerDelegate {
	if r.coreReaders == nil {
		return nil
	}
	dv := r.coreReaders.GetDocValuesProducer()
	if dv == nil {
		return nil
	}
	d, ok := dv.(docValuesProducerDelegate)
	if !ok {
		return nil
	}
	return d
}

// dvFieldInfo resolves field to its FieldInfo from the core readers, returning
// nil when the field is absent or carries no doc values. The codec producer
// keys on the FieldInfo (in particular its number), not the field name.
func (r *SegmentReader) dvFieldInfo(field string) *FieldInfo {
	if r.coreReaders == nil {
		return nil
	}
	fis := r.coreReaders.GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.GetByName(field)
	if fi == nil || !fi.DocValuesType().HasDocValues() {
		return nil
	}
	return fi
}

// GetNumericDocValues returns the numeric doc values for field, delegating to
// the codec's doc-values producer. Returns (nil, nil) when the segment has no
// doc-values producer or the field has no numeric doc values (matching the
// LeafReader contract). Overrides the embedded LeafReader.GetNumericDocValues,
// which unconditionally returns (nil, nil). Mirrors
// org.apache.lucene.index.SegmentReader.getNumericDocValues / CodecReader.
func (r *SegmentReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeNumeric {
		return nil, nil
	}
	return d.GetNumeric(fi)
}

// GetBinaryDocValues returns the binary doc values for field, delegating to
// the codec's doc-values producer. Overrides LeafReader.GetBinaryDocValues.
func (r *SegmentReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeBinary {
		return nil, nil
	}
	return d.GetBinary(fi)
}

// GetSortedDocValues returns the sorted doc values for field, delegating to
// the codec's doc-values producer. Overrides LeafReader.GetSortedDocValues.
func (r *SegmentReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSorted {
		return nil, nil
	}
	return d.GetSorted(fi)
}

// GetSortedNumericDocValues returns the sorted-numeric doc values for field,
// delegating to the codec's doc-values producer. Overrides
// LeafReader.GetSortedNumericDocValues.
func (r *SegmentReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedNumeric {
		return nil, nil
	}
	return d.GetSortedNumeric(fi)
}

// GetSortedSetDocValues returns the sorted-set doc values for field,
// delegating to the codec's doc-values producer. Overrides
// LeafReader.GetSortedSetDocValues.
func (r *SegmentReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedSet {
		return nil, nil
	}
	return d.GetSortedSet(fi)
}

// normsProducerDelegate is the read surface exposed by the codec's norms
// producer (stored on SegmentCoreReaders.normsProducer as an interface{}).
// The concrete producer — Lucene90NormsProducer from the codecs package —
// satisfies it structurally via GetNorms (the Go counterpart of
// org.apache.lucene.codecs.NormsProducer.getNorms). The contract names only
// the index-facing NumericDocValues (an alias of the spi type) and *FieldInfo
// (an alias of schema.FieldInfo), so the index package can name it without
// importing codecs — the same pattern as docValuesProducerDelegate.
type normsProducerDelegate interface {
	GetNorms(field *FieldInfo) (NumericDocValues, error)
}

// normsDelegate narrows the core readers' norms producer to the GetNorms
// surface, or returns nil when the segment has no norms producer (e.g. no
// fields with norms, or the codec-less test path).
func (r *SegmentReader) normsDelegate() normsProducerDelegate {
	if r.coreReaders == nil {
		return nil
	}
	np := r.coreReaders.GetNormsProducer()
	if np == nil {
		return nil
	}
	d, ok := np.(normsProducerDelegate)
	if !ok {
		return nil
	}
	return d
}

// normsFieldInfo resolves field to its FieldInfo from the core readers,
// returning nil when the field is absent or omits norms. The codec producer
// keys on the FieldInfo (in particular its number), not the field name.
func (r *SegmentReader) normsFieldInfo(field string) *FieldInfo {
	if r.coreReaders == nil {
		return nil
	}
	fis := r.coreReaders.GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.GetByName(field)
	if fi == nil || !fi.HasNorms() {
		return nil
	}
	return fi
}

// GetNormValues returns the per-document norms for field, delegating to the
// codec's norms producer. Returns (nil, nil) when the segment has no norms
// producer or the field has no norms (matching the LeafReader contract).
// Overrides the embedded LeafReader.GetNormValues, which unconditionally
// returns (nil, nil). Mirrors org.apache.lucene.index.SegmentReader
// .getNormValues / CodecReader.getNormValues.
func (r *SegmentReader) GetNormValues(field string) (NumericDocValues, error) {
	d := r.normsDelegate()
	fi := r.normsFieldInfo(field)
	if d == nil || fi == nil {
		return nil, nil
	}
	return d.GetNorms(fi)
}

// knnTopDocsToIndex converts a util/hnsw TopDocs (the codec search result)
// into the index-package TopDocs struct. The hnsw TopDocs is already
// score-descending; TotalHits is the visited-count lower bound, but the
// index TopDocs.TotalHits records the number of returned hits, matching how
// the index layer reports per-leaf vector results.
func knnTopDocsToIndex(td *utilhnsw.TopDocs) TopDocs {
	if td == nil {
		return TopDocs{}
	}
	scoreDocs := make([]ScoreDoc, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		scoreDocs[i] = ScoreDoc{Doc: sd.Doc, Score: sd.Score}
	}
	return TopDocs{TotalHits: len(scoreDocs), ScoreDocs: scoreDocs}
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

	// Load the full SegmentInfo from the .si file to obtain metadata not present
	// in the segments_N entry (isCompoundFile, file set, docCount).  Fall back
	// to the segments_N-constructed SegmentInfo when the .si is absent
	// (in-memory segments that were never flushed to disk).
	segInfo := sci.SegmentInfo()
	if codec != nil {
		if sif := codec.SegmentInfoFormat(); sif != nil {
			if fullSegInfo, err := sif.Read(directory, segInfo.Name(), segInfo.GetID(), store.IOContextRead); err == nil {
				segInfo = fullSegInfo
			}
		}
	}

	// Resolve FieldInfos: prefer in-memory (carried by a freshly-written
	// in-memory segment), otherwise read the authoritative .fnm from disk via
	// the codec's FieldInfosFormat (rmp #4785).
	fi := sci.GetInMemoryFieldInfos()
	if fi == nil && codec != nil {
		fi = readFieldInfosFromDisk(directory, codec, segInfo)
	}
	if fi == nil {
		fi = NewFieldInfos()
	}

	// When the segment has an updated FieldInfos generation (e.g. after a
	// doc-values update), read the newer .fnm so that GetNumericDocValues
	// and other doc-values accessors resolve the correct fields. The codec
	// producers (FieldsProducer, StoredFieldsReader, etc.) were written
	// against the base FieldInfos and stay wired to the core created below;
	// the SegmentReader's exposed fieldInfos may be newer. Mirrors Java
	// SegmentReader.initFieldInfos().
	var updatedFi *FieldInfos
	if fi != nil && sci.HasFieldInfosGen() {
		updatedFi = readFieldInfosWithGen(directory, codec, segInfo, sci.FieldInfosGen())
	}
	if updatedFi != nil {
		fi = updatedFi
	}

	// Construct SegmentCoreReaders when a codec is available. This wires the
	// StoredFieldsReader, FieldsProducer (postings), and TermVectorsReader so
	// that IndexSearcher.Doc and term-based searches work end-to-end.
	if codec != nil {
		core, err := NewSegmentCoreReaders(directory, segInfo, fi, codec, store.IOContextRead)
		if err == nil {
			sr := NewSegmentReaderWithCore(sci, core, fi, codec)
			sr.directory = directory

			// When any field carries a doc-values generation > -1 the base
			// producer loaded by NewSegmentCoreReaders reads the original files
			// (_0.dvd) but misses the updated ones (_0_1_Lucene90_0.dvd).
			// Overlay a SegmentDocValuesProducer that fans out to the correct
			// per-generation producer for every field.  Mirrors Java
			// SegmentReader's initFieldInfos() + SegmentCoreReaders wiring.
			if fi != nil && fi.HasDocValues() {
				var hasGen bool
				it := fi.Iterator()
				for it.HasNext() {
					if it.Next().DocValuesGen() > -1 {
						hasGen = true
						break
					}
				}
				if hasGen {
					coreInfos := readFieldInfosFromDisk(directory, codec, segInfo)
					if coreInfos == nil {
						coreInfos = NewFieldInfos()
					}
					factory := func(si *SegmentCommitInfo, dir store.Directory, gen int64, infos *FieldInfos) (DocValuesProducer, error) {
						suffix := ""
						if gen != -1 {
							suffix = strconv.FormatInt(gen, 36)
						}
						state := &SegmentReadState{
							Directory:     dir,
							SegmentInfo:   si.SegmentInfo(),
							FieldInfos:    infos,
							SegmentSuffix: suffix,
						}
						format := codec.DocValuesFormat()
						if format == nil {
							return nil, fmt.Errorf("no DocValuesFormat for codec %q", codec.Name())
						}
						return format.FieldsProducer(state)
					}
					sdv, err := NewSegmentDocValues(factory)
					if err != nil {
						_ = core.DecRef()
						return nil, fmt.Errorf("openSegmentReader: SegmentDocValues init failed: %w", err)
					}
					dvp, err := NewSegmentDocValuesProducer(sci, directory, coreInfos, fi, sdv)
					if err != nil {
						_ = core.DecRef()
						return nil, fmt.Errorf("openSegmentReader: SegmentDocValuesProducer init failed: %w", err)
					}
					if oldDvp, ok := core.docValuesProducer.(DocValuesProducer); ok && oldDvp != nil {
						_ = oldDvp.Close()
					}
					core.SetDocValuesProducer(dvp)
				}
			}

			loadLiveDocsFromDisk(directory, sci)
			return sr, nil
		}
		// Wiring the core readers failed. Distinguish a genuine failure from the
		// benign metadata-only segment case (rmp #4):
		//
		//   - If the segment owns per-format data files (a compound .cfs, or any
		//     non-metadata file), the codec readers MUST open; a failure here means
		//     a corrupt or truncated segment, so surface an explicit error instead
		//     of silently returning a data-less reader that would make DocValues
		//     read-back (and every other accessor) appear empty.
		//
		//   - If the segment carries only .si/.fnm it is a metadata-only segment:
		//     an AddIndexes-imported placeholder (backlog #2707) or a not-yet-
		//     data-merged ForceMerge result (the real merge that writes the merged
		//     postings/stored-fields/doc-values is tracked separately). For those we
		//     fall through to the FieldInfos-only reader, preserving the structural
		//     reopen until the merge write-path lands.
		if segmentHasDataFiles(segInfo) {
			return nil, fmt.Errorf("openSegmentReader: segment %q is codec-backed with data files but its core readers could not be opened: %w", segInfo.Name(), err)
		}
	}

	// Codec-less / data-less fallback: expose the .si docCount and the .fnm
	// FieldInfos without core readers.
	sr := &SegmentReader{
		LeafReader:        NewLeafReader(segInfo),
		segmentCommitInfo: sci,
		fieldInfos:        fi,
		directory:         directory,
	}
	loadLiveDocsFromDisk(directory, sci)
	return sr, nil
}

// segmentHasDataFiles reports whether segInfo owns any per-format document-data
// file (as opposed to only the .si segment-info and .fnm field-infos metadata).
// A compound segment always carries data (its .cfs). For a non-compound segment
// any file whose extension is not .si, .fnm or .cfe is a data file (postings,
// stored fields, doc values, points, vectors, norms, term vectors). Used by
// openSegmentReader to decide whether a core-readers wiring failure is a genuine
// corruption (data present but unreadable) or a benign metadata-only segment.
func segmentHasDataFiles(segInfo *SegmentInfo) bool {
	if segInfo.IsCompoundFile() {
		return true
	}
	for _, f := range segInfo.Files() {
		switch {
		case strings.HasSuffix(f, ".si"), strings.HasSuffix(f, ".fnm"), strings.HasSuffix(f, ".cfe"):
			// Metadata only: not document data.
		default:
			return true
		}
	}
	return false
}

// readFieldInfosFromDisk reads the authoritative .fnm FieldInfos for a segment
// from disk via the codec's FieldInfosFormat (rmp #4785). For a compound
// segment the .fnm lives inside the .cfs, so the read is routed through the
// compound directory; reading it from the top-level directory would find no
// .fnm and yield an empty FieldInfos, hiding every indexed field from the
// reopened reader. Returns nil when no codec FieldInfosFormat is available or
// the read fails (caller substitutes an empty FieldInfos).
func readFieldInfosFromDisk(directory store.Directory, codec Codec, segInfo *SegmentInfo) *FieldInfos {
	if codec == nil {
		return nil
	}
	fif := codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	fnmDir := directory
	var cfsReader store.Directory
	if segInfo.IsCompoundFile() {
		if cf := codec.CompoundFormat(); cf != nil {
			if r, err := cf.GetCompoundReader(directory, segInfo); err == nil {
				fnmDir = r
				cfsReader = r
			}
		}
	}
	if cfsReader != nil {
		defer func() {
			if closer, ok := cfsReader.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}()
	}
	fi, err := fif.Read(fnmDir, segInfo, "", store.IOContextRead)
	if err != nil {
		return nil
	}
	return fi
}

// readFieldInfosWithGen reads the .fnm FieldInfos for a segment at the given
// field-infos generation. When a doc-values update bumps the fieldInfosGen,
// Lucene writes a new _N_G.fnm file (where G is the generation in base-36);
// the base _N.fnm no longer reflects the current doc-values fields. This
// helper mirrors Java SegmentReader.initFieldInfos(). Returns nil when the
// read fails (caller falls back to the base FieldInfos).
func readFieldInfosWithGen(directory store.Directory, codec Codec, segInfo *SegmentInfo, gen int64) *FieldInfos {
	if codec == nil {
		return nil
	}
	fif := codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	suffix := strconv.FormatInt(gen, 36)
	fnmDir := directory
	var cfsReader store.Directory
	if segInfo.IsCompoundFile() {
		if cf := codec.CompoundFormat(); cf != nil {
			if r, err := cf.GetCompoundReader(directory, segInfo); err == nil {
				fnmDir = r
				cfsReader = r
			}
		}
	}
	if cfsReader != nil {
		defer func() {
			if closer, ok := cfsReader.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}()
	}
	fi, err := fif.Read(fnmDir, segInfo, suffix, store.IOContextRead)
	if err != nil {
		return nil
	}
	return fi
}

// loadLiveDocsFromDisk reads the segment's .liv file (when the segment has a
// non-default delGen) and records the deleted ordinals on the SegmentCommitInfo
// so SegmentReader.GetLiveDocs and SegmentCommitInfo.NumDocs reflect the
// on-disk deletions. This replaces the legacy _gocene_del_ userData round-trip
// (rmp #4785) by making the byte-faithful Lucene90 .liv file authoritative.
//
// Best-effort: a missing or unreadable .liv leaves the existing (possibly
// empty) deleted-ordinal state untouched, so segments with no deletions and
// codec-less in-memory segments are unaffected.
func loadLiveDocsFromDisk(directory store.Directory, sci *SegmentCommitInfo) {
	if sci == nil {
		return
	}
	// Skip when the segment already carries deleted ordinals (e.g. freshly
	// written in-memory state that has not yet round-tripped through disk) or
	// when there is no deletion generation to read.
	if len(sci.GetDeletedOrdinals()) > 0 {
		return
	}
	if sci.DelGen() < 0 || sci.DelCount() == 0 {
		return
	}
	segInfo := sci.SegmentInfo()
	maxDoc := segInfo.DocCount()
	if maxDoc <= 0 {
		return
	}
	bits, err := readLiveDocs(directory, segInfo.Name(), segInfo.GetID(), sci.DelGen(), maxDoc)
	if err != nil || bits == nil {
		return
	}
	ords := make([]int, 0, sci.DelCount())
	for doc := 0; doc < maxDoc; doc++ {
		if !bits.Get(doc) {
			ords = append(ords, doc)
		}
	}
	if len(ords) > 0 {
		sci.SetDeletedOrdinals(ords)
	}
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
		nrtGen:          0, // commit-pinned reader
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

// OpenDirectoryReaderFromWriter opens a near-real-time DirectoryReader
// directly from a live IndexWriter — the Go analogue of Lucene's
// DirectoryReader.open(IndexWriter). The returned reader reflects every
// document added to the writer so far, including documents still
// buffered and not yet durably committed.
//
// See IndexWriter.GetReader for the NRT contract and the (single) way
// this port currently differs from Lucene's pooled in-memory NRT path.
func OpenDirectoryReaderFromWriter(writer *IndexWriter) (*DirectoryReader, error) {
	if writer == nil {
		return nil, fmt.Errorf("OpenDirectoryReaderFromWriter: writer must not be nil")
	}
	return writer.GetReader()
}

// OpenIfChangedFromWriter reopens old against the current state of a live
// IndexWriter — the Go analogue of DirectoryReader.openIfChanged(reader,
// writer). It returns a fresh NRT reader when the writer holds changes
// not yet reflected by old, or (nil, nil) when old is already current
// (matching Lucene's null return). The caller retains ownership of old in
// both cases.
func OpenIfChangedFromWriter(old *DirectoryReader, writer *IndexWriter) (*DirectoryReader, error) {
	if writer == nil {
		return nil, fmt.Errorf("OpenIfChangedFromWriter: writer must not be nil")
	}
	if old != nil && old.nrtGen > 0 {
		// NRT reader path: compare the reader snapshot gen against the
		// writer current counter.  When both match and no uncommitted
		// changes exist, nothing happened since this reader was opened.
		if old.nrtGen == writer.GetNRTGeneration() && !writer.hasUncommittedChanges() {
			return nil, nil
		}
	}
	// Fall back: if writer has no uncommitted changes and committed gen
	// matches old, nothing changed.
	if old != nil && !writer.hasUncommittedChanges() {
		if cur, err := ReadSegmentInfos(writer.directory); err == nil &&
			old.segmentInfos != nil && cur.Generation() == old.segmentInfos.Generation() {
			return nil, nil
		}
	}
	return writer.GetReader()
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
	return compositeTermsForField(r.readers, field)
}

// compositeTermsForField returns a Terms view of field merged across every
// segment that contains it: a single sub is returned directly, two or more are
// aggregated through a MultiTerms whose ReaderSlices carry each segment's
// composite docID base so postings read back through the merged doc space.
// Returns (nil, nil) when no segment has the field.
//
// Previously DirectoryReader.Terms returned only the first segment's Terms,
// which silently hid every term unique to a later segment — breaking multi-term
// query rewrites (PrefixQuery/WildcardQuery/RangeQuery/...) over a multi-segment
// index (rmp #18 / #123).
func compositeTermsForField(readers []*SegmentReader, field string) (Terms, error) {
	var subs []Terms
	var slices []ReaderSlice
	docBase := 0
	for i, sr := range readers {
		if sr == nil {
			continue
		}
		maxDoc := sr.MaxDoc()
		terms, err := sr.Terms(field)
		if err != nil {
			return nil, err
		}
		if terms != nil {
			subs = append(subs, terms)
			slices = append(slices, ReaderSlice{Start: docBase, Length: maxDoc, ReaderIndex: i})
		}
		docBase += maxDoc
	}
	switch len(subs) {
	case 0:
		return nil, nil
	case 1:
		return subs[0], nil
	default:
		return NewMultiTermsForField(field, subs, slices)
	}
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
		gen := parseSegmentsFileGeneration(name)
		if gen <= 0 || gen >= currentGen {
			continue // not a segments file, or already captured as latest
		}
		sis, readErr := readSegmentInfosFileByGen(dir, name, gen)
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

// parseSegmentsFileGeneration extracts the generation from a segments_N
// file name. Returns 0 if the name does not match the "segments_<N>" pattern.
func parseSegmentsFileGeneration(name string) int64 {
	const prefix = "segments_"
	if !strings.HasPrefix(name, prefix) {
		return 0
	}
	gen, err := strconv.ParseInt(name[len(prefix):], 10, 64)
	if err != nil {
		return 0
	}
	return gen
}

// readSegmentInfosFileByGen reads a named segments_N file from dir with the
// given generation. Used by ListCommits to enumerate prior commits.
func readSegmentInfosFileByGen(dir store.Directory, name string, gen int64) (*SegmentInfos, error) {
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
