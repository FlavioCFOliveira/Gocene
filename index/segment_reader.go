// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// SegmentReader is a LeafReader for a specific segment.
type SegmentReader struct {
	segmentCommitInfo *SegmentCommitInfo
	coreReaders       *SegmentCoreReaders
	fieldInfos        *FieldInfos
	codec             Codec
	// directory is the source directory; used to look up in-memory postings
	// from the package-level registry when coreReaders is nil.
	directory store.Directory
	liveDocs          util.Bits
	hardLiveDocs      util.Bits
	isNRT             bool
	numDocs           int
}

// NewSegmentReader creates a new SegmentReader.
func NewSegmentReader(segmentCommitInfo *SegmentCommitInfo) *SegmentReader {
	sr := &SegmentReader{
		segmentCommitInfo: segmentCommitInfo,
		fieldInfos:        segmentCommitInfo.GetInMemoryFieldInfos(),
	}
	sr.initLiveDocs()
	return sr
}

func (r *SegmentReader) initLiveDocs() {
	if r.segmentCommitInfo == nil {
		return
	}
	if r.segmentCommitInfo.HasDeletions() {
		// Read live docs from codec
		liveDocs, err := r.codec.LiveDocsFormat().ReadLiveDocs(r.directory, r.segmentCommitInfo, store.IOContextReadOnce)
		if err != nil {
			panic(fmt.Sprintf("failed to read live docs for seg=%s: %v", r.segmentCommitInfo, err))
		}
		r.liveDocs = liveDocs
		r.hardLiveDocs = liveDocs
	}
	r.numDocs = r.segmentCommitInfo.SegmentInfo().DocCount() - r.segmentCommitInfo.GetDelCount()
}

// NewSegmentReaderWithCore creates a new SegmentReader with core readers.
func NewSegmentReaderWithCore(
	segmentCommitInfo *SegmentCommitInfo,
	coreReaders *SegmentCoreReaders,
	fieldInfos *FieldInfos,
	codec Codec,
) *SegmentReader {
	sr := &SegmentReader{
		segmentCommitInfo: segmentCommitInfo,
		coreReaders:       coreReaders,
		fieldInfos:        fieldInfos,
		codec:             codec,
	}
	sr.initLiveDocs()
	return sr
}

// NewSegmentReaderClone creates a new SegmentReader sharing core from a previous SegmentReader
// and using the provided liveDocs, and recording whether those liveDocs were carried in ram (isNRT=true).
func NewSegmentReaderClone(
	si *SegmentCommitInfo,
	sr *SegmentReader,
	liveDocs util.Bits,
	hardLiveDocs util.Bits,
	numDocs int,
	isNRT bool,
) *SegmentReader {
	return &SegmentReader{
		segmentCommitInfo: si,
		coreReaders:       sr.coreReaders,
		fieldInfos:        sr.fieldInfos,
		codec:             sr.codec,
		directory:         sr.directory,
		liveDocs:          liveDocs,
		hardLiveDocs:      hardLiveDocs,
		isNRT:             isNRT,
		numDocs:           numDocs,
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
// segment, taken from the SegmentCommitInfo so it stays consistent with NumDocs.
func (r *SegmentReader) NumDeletedDocs() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.DelCount() + r.segmentCommitInfo.SoftDelCount()
}

// HasDeletions reports whether this segment carries any deleted documents.
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

// DocID returns the first document ID in this segment.
// For a SegmentReader, this is always 0.
func (r *SegmentReader) DocID() int {
	return 0
}

// DocFreq returns the number of documents containing the term.
func (r *SegmentReader) DocFreq(term Term) (int, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te := terms.Iterator()
	if te.SeekExact(term.Text()) {
		return te.DocFreq(), nil
	}
	return 0, nil
}

// TotalTermFreq returns the total number of occurrences of the term.
func (r *SegmentReader) TotalTermFreq(term Term) (int64, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te := terms.Iterator()
	if te.SeekExact(term.Text()) {
		return te.TotalTermFreq(), nil
	}
	return 0, nil
}

// GetTermVectors returns the term vectors for a document.
func (r *SegmentReader) GetTermVectors(docID int) (Fields, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized: core readers are nil")
	}

	tvReader := r.coreReaders.GetTermVectorsReader()
	if tvReader == nil {
		return nil, nil
	}

	if docID < 0 || docID >= r.DocCount() {
		return nil, fmt.Errorf("document ID %d out of range [0, %d)", docID, r.DocCount())
	}

	return tvReader.Get(docID)
}

// Terms returns the Terms for a field.
func (r *SegmentReader) Terms(field string) (Terms, error) {
	if r.coreReaders != nil {
		fields := r.coreReaders.GetFields()
		if fields == nil {
			return nil, nil
		}
		return fields.Terms(field)
	}

	if r.segmentCommitInfo != nil {
		if fp := r.segmentCommitInfo.GetInMemoryFields(); fp != nil {
			return fp.Terms(field)
		}
	}

	if r.directory != nil && r.segmentCommitInfo != nil {
		segName := r.segmentCommitInfo.SegmentInfo().Name()
		if fp := LookupInMemoryFields(r.directory, segName); fp != nil {
			return fp.Terms(field)
		}
	}

	return nil, nil
}

// GetFloatVectorValues returns the float vectors for field.
func (r *SegmentReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.FloatVectorValues(field)
}

// GetByteVectorValues returns the byte vectors for field.
func (r *SegmentReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.ByteVectorValues(field)
}

// SearchNearestVectors searches for the k nearest float vectors to target.
func (r *SegmentReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (TopDocs, error) {
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

// GetPointValues returns the BKD point values for field.
func (r *SegmentReader) GetPointValues(field string) (PointValues, error) {
	d := r.pointsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.GetValues(field)
}

// GetNumericDocValues returns the numeric doc values for field.
func (r *SegmentReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeNumeric {
		return nil, nil
	}
	return d.GetNumeric(fi)
}

// GetBinaryDocValues returns the binary doc values for field.
func (r *SegmentReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeBinary {
		return nil, nil
	}
	return d.GetBinary(fi)
}

// GetSortedDocValues returns the sorted doc values for field.
func (r *SegmentReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSorted {
		return nil, nil
	}
	return d.GetSorted(fi)
}

// GetSortedNumericDocValues returns the sorted-numeric doc values for field.
func (r *SegmentReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedNumeric {
		return nil, nil
	}
	return d.GetSortedNumeric(fi)
}

// GetSortedSetDocValues returns the sorted-set doc values for field.
func (r *SegmentReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedSet {
		return nil, nil
	}
	return d.GetSortedSet(fi)
}

// GetNormValues returns the per-document norms for field.
func (r *SegmentReader) GetNormValues(field string) (NumericDocValues, error) {
	d := r.normsDelegate()
	fi := r.normsFieldInfo(field)
	if d == nil || fi == nil {
		return nil, nil
	}
	return d.GetNorms(fi)
}

// GetDocValuesSkipper returns a DocValuesSkipper for efficient skipping.
func (r *SegmentReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	// Base implementation returns nil - should be implemented by concrete readers
	return nil, nil
}

// CheckIntegrity checks that the index is not corrupt.
func (r *SegmentReader) CheckIntegrity() error {
	// Implementation deferred
	return nil
}

// GetMetaData returns metadata about this leaf.
func (r *SegmentReader) GetMetaData() *IndexReaderMetaData {
	return &IndexReaderMetaData{
		HasDeletions: r.HasDeletions(),
		NumDocs:      r.NumDocs(),
		MaxDoc:       r.MaxDoc(),
	}
}

// GetLiveDocs returns a bitset of live (not deleted) docs.
func (r *SegmentReader) GetLiveDocs() util.Bits {
	return r.liveDocs
}

// GetHardLiveDocs returns the live-docs bits excluding documents that are not live due to soft-deletes.
func (r *SegmentReader) GetHardLiveDocs() util.Bits {
	return r.hardLiveDocs
}

// GetContext returns the reader context for this leaf reader.
func (r *SegmentReader) GetContext() (IndexReaderContext, error) {
	return NewLeafReaderContext(r, nil, 0, 0), nil
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

// IncRef increments the reference count.
func (r *SegmentReader) IncRef() error {
	// SegmentReader doesn't have its own ref count, it delegates to the core readers
	if r.coreReaders != nil {
		return r.coreReaders.IncRef()
	}
	return nil
}

// DecRef decrements the reference count.
func (r *SegmentReader) DecRef() error {
	if r.coreReaders != nil {
		return r.coreReaders.DecRef()
	}
	return nil
}

// TryIncRef tries to increment the reference count.
func (r *SegmentReader) TryIncRef() bool {
	if r.coreReaders != nil {
		return r.coreReaders.TryIncRef()
	}
	return true
}

// GetRefCount returns the current reference count.
func (r *SegmentReader) GetRefCount() int32 {
n
	// GetCoreCacheHelper returns a CacheHelper for the core data of this leaf.
	func (r *SegmentReader) GetCoreCacheHelper() CacheHelper {
		if r.coreReaders != nil {
			return r.coreReaders.GetCacheHelper()
		}
		return nil
	}

	// GetReaderCacheHelper returns a CacheHelper for the reader.
	func (r *SegmentReader) GetReaderCacheHelper() CacheHelper {
		return r.GetCacheHelper()
	}
	if r.coreReaders != nil {
n
	// GetCoreCacheHelper returns a CacheHelper for the core data of this leaf.
	func (r *SegmentReader) GetCoreCacheHelper() CacheHelper {
		if r.coreReaders != nil {
			return r.coreReaders.GetCacheHelper()
		}
		return nil
	}

	// GetReaderCacheHelper returns a CacheHelper for the reader.
	func (r *SegmentReader) GetReaderCacheHelper() CacheHelper {
		return r.GetCacheHelper()
	}
		return r.coreReaders.GetRefCount()
n
	// GetCoreCacheHelper returns a CacheHelper for the core data of this leaf.
	func (r *SegmentReader) GetCoreCacheHelper() CacheHelper {
		if r.coreReaders != nil {
			return r.coreReaders.GetCacheHelper()
		}
		return nil
	}

	// GetReaderCacheHelper returns a CacheHelper for the reader.
	func (r *SegmentReader) GetReaderCacheHelper() CacheHelper {
		return r.GetCacheHelper()
	}
	}
n
	// GetCoreCacheHelper returns a CacheHelper for the core data of this leaf.
	func (r *SegmentReader) GetCoreCacheHelper() CacheHelper {
		if r.coreReaders != nil {
			return r.coreReaders.GetCacheHelper()
		}
		return nil
	}

	// GetReaderCacheHelper returns a CacheHelper for the reader.
	func (r *SegmentReader) GetReaderCacheHelper() CacheHelper {
		return r.GetCacheHelper()
	}
	return 1
}

// DocCount returns the number of documents in this segment.
func (r *SegmentReader) DocCount() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.SegmentInfo().DocCount()
}

// vectorsDelegate narrows the core readers' KNN vectors reader to the per-encoding read surface.
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

type knnVectorsReaderDelegate interface {
	FloatVectorValues(field string) (FloatVectorValues, error)
	ByteVectorValues(field string) (ByteVectorValues, error)
	SearchNearestFloat(field string, target []float32, k int, acceptDocs util.Bits) (*utilhnsw.TopDocs, error)
	SearchNearestByte(field string, target []byte, k int, acceptDocs util.Bits) (*utilhnsw.TopDocs, error)
	SearchNearestFloatCollector(field string, target []float32, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
	SearchNearestByteCollector(field string, target []byte, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
}

// pointsDelegate narrows the core readers' points reader to the wide getValues surface.
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

type pointsReaderDelegate interface {
	GetValues(field string) (PointValues, error)
}

// docValuesProducerDelegate is the read surface exposed by the codec's doc-values producer.
type docValuesProducerDelegate interface {
	GetNumeric(field *FieldInfo) (NumericDocValues, error)
	GetBinary(field *FieldInfo) (BinaryDocValues, error)
	GetSorted(field *FieldInfo) (SortedDocValues, error)
	GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error)
	GetSortedSet(field *FieldInfo) (SortedSetDocValues, error)
}

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

func (r *SegmentReader) dvFieldInfo(field string) *FieldInfo {
	fis := r.GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.GetByName(field)
	if fi == nil || !fi.DocValuesType().HasDocValues() {
		return nil
	}
	return fi
}

// normsProducerDelegate is the read surface exposed by the codec's norms producer.
type normsProducerDelegate interface {
	GetNorms(field *FieldInfo) (NumericDocValues, error)
}

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
