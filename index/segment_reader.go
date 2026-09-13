// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// SegmentReader is a LeafReader for a specific segment.
// Mirrors org.apache.lucene.index.SegmentReader from Apache Lucene 10.5.0.
type SegmentReader struct {
	segmentCommitInfo *SegmentCommitInfo
	coreReaders       *SegmentCoreReaders
	fieldInfos        *FieldInfos
	codec             Codec
	// directory is the source directory; used to look up in-memory postings
	// from the package-level registry when coreReaders is nil.
	directory store.Directory

	liveDocs     util.Bits
	hardLiveDocs util.Bits
	isNRT        bool
	numDocs      int
}

// NewSegmentReader creates a new SegmentReader.
// If the SegmentCommitInfo carries in-memory FieldInfos (written by Commit),
// they are used so that CheckIndex and other readers can enumerate fields
// without codec infrastructure.
func NewSegmentReader(segmentCommitInfo *SegmentCommitInfo) *SegmentReader {
	sr := &SegmentReader{
		segmentCommitInfo: segmentCommitInfo,
		fieldInfos:        segmentCommitInfo.GetInMemoryFieldInfos(),
	}
	sr.initLiveDocs()
	return sr
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

func (r *SegmentReader) initLiveDocs() {
	if r.segmentCommitInfo == nil || r.directory == nil {
		return
	}
	if !r.segmentCommitInfo.HasDeletions() {
		return
	}
	segInfo := r.segmentCommitInfo.SegmentInfo()
	maxDoc := segInfo.MaxDoc()
	if maxDoc <= 0 {
		return
	}
	// Mirrors SegmentReader's constructor, which reads the segment's .liv
	// through codec.liveDocsFormat().readLiveDocs(...). Gocene serves the
	// Lucene90 live-docs format directly from readLiveDocs (live_docs.go);
	// it is the only live-docs format Lucene 10.5.0 defines.
	liveDocs, err := readLiveDocs(r.directory, segInfo.Name(), segInfo.GetID(), r.segmentCommitInfo.DelGen(), maxDoc)
	if err != nil {
		panic(fmt.Sprintf("failed to read live docs for seg=%s: %v", r.segmentCommitInfo, err))
	}
	if liveDocs == nil {
		return
	}
	r.liveDocs = liveDocs
	r.hardLiveDocs = liveDocs
}

// GetNormsProducer returns the norms producer for this segment.
func (r *SegmentReader) GetNormsProducer() NormsProducer {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetNormsProducer()
}

// GetSegmentCommitInfo returns the SegmentCommitInfo for this reader. This is
// the Go counterpart of org.apache.lucene.index.SegmentReader.getSegmentInfo(),
// which returns the commit-level info; GetSegmentInfo is reserved for the
// per-segment .si metadata the CodecReader contract exposes.
func (r *SegmentReader) GetSegmentCommitInfo() *SegmentCommitInfo {
	return r.segmentCommitInfo
}

// GetSegmentInfo returns the per-segment .si metadata backing this reader, as
// required by the CodecReader contract.
func (r *SegmentReader) GetSegmentInfo() *SegmentInfo {
	if r.segmentCommitInfo == nil {
		return nil
	}
	return r.segmentCommitInfo.SegmentInfo()
}

// GetSegmentName returns the name of the segment this reader reads.
func (r *SegmentReader) GetSegmentName() string {
	if r.segmentCommitInfo == nil {
		return ""
	}
	return r.segmentCommitInfo.SegmentInfo().Name()
}

// IsNRT reports whether this reader was opened over live-docs carried in RAM
// rather than read from a committed .liv file. Mirrors the package-private
// SegmentReader.isNRT flag.
func (r *SegmentReader) IsNRT() bool {
	return r.isNRT
}

// GetCoreReaders returns the SegmentCoreReaders for this reader.
func (r *SegmentReader) GetCoreReaders() *SegmentCoreReaders {
	return r.coreReaders
}

// GetFieldsReader returns the codec's stored-fields reader, or nil when the
// segment has no core readers. Mirrors SegmentReader.getFieldsReader().
func (r *SegmentReader) GetFieldsReader() StoredFieldsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetStoredFieldsReader()
}

// GetTermVectorsReader returns the codec's term-vectors reader, or nil when the
// segment stores none. Mirrors SegmentReader.getTermVectorsReader().
func (r *SegmentReader) GetTermVectorsReader() TermVectorsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetTermVectorsReader()
}

// GetNormsReader returns the codec's norms producer. Mirrors
// SegmentReader.getNormsReader().
func (r *SegmentReader) GetNormsReader() NormsProducer {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetNormsProducer()
}

// GetDocValuesReader returns the doc-values producer serving this reader.
// Mirrors SegmentReader.getDocValuesReader().
func (r *SegmentReader) GetDocValuesReader() DocValuesProducer {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetDocValuesProducer()
}

// GetPostingsReader returns the codec's postings producer. Mirrors
// SegmentReader.getPostingsReader().
func (r *SegmentReader) GetPostingsReader() FieldsProducer {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetFields()
}

// GetPointsReader returns the codec's points reader. Mirrors
// SegmentReader.getPointsReader().
func (r *SegmentReader) GetPointsReader() PointsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetPointsReader()
}

// GetVectorReader returns the codec's KNN vectors reader. Mirrors
// SegmentReader.getVectorReader().
func (r *SegmentReader) GetVectorReader() KnnVectorsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetVectorReader()
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
// (i.e. equals MaxDoc-NumDocs). Without this override, callers relying on a
// generic zero-value would get a value wrong for a committed segment carrying
// .liv deletions (rmp #12).
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

// DocCount returns the number of documents in this segment (same as MaxDoc
// for a segment without points, mirrors CodecReader.getDocCount).
func (r *SegmentReader) DocCount() int {
	if r.segmentCommitInfo == nil {
		return 0
	}
	return r.segmentCommitInfo.SegmentInfo().DocCount()
}

// DocID returns the first document ID in this segment (always 0 for a leaf).
func (r *SegmentReader) DocID() int {
	return 0
}

// DocFreq returns the number of documents containing term.
func (r *SegmentReader) DocFreq(term Term) (int, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te, err := terms.Iterator()
	if err != nil || te == nil {
		return 0, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return te.DocFreq()
}

// Postings returns a PostingsEnum for the specified term, or nil if the term
// does not occur in this segment. Mirrors the default method
// org.apache.lucene.index.LeafReader.postings(Term, int).
func (r *SegmentReader) Postings(term Term, flags int) (PostingsEnum, error) {
	terms, err := r.Terms(term.Field)
	if err != nil || terms == nil {
		return nil, err
	}
	te, err := terms.Iterator()
	if err != nil || te == nil {
		return nil, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

// TotalTermFreq returns the total number of occurrences of term in this segment.
func (r *SegmentReader) TotalTermFreq(term Term) (int64, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te, err := terms.Iterator()
	if err != nil || te == nil {
		return 0, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return te.TotalTermFreq()
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

// GetFields returns the Fields for this segment.
func (r *SegmentReader) GetFields() Fields {
	if r.coreReaders != nil {
		fp := r.coreReaders.GetFields()
		if fp == nil {
			return nil
		}
		return asFields(fp)
	}

	if r.segmentCommitInfo != nil {
		if fp := r.segmentCommitInfo.GetInMemoryFields(); fp != nil {
			return asFields(fp)
		}
	}

	if r.directory != nil && r.segmentCommitInfo != nil {
		segName := r.segmentCommitInfo.SegmentInfo().Name()
		if fp := LookupInMemoryFields(r.directory, segName); fp != nil {
			return asFields(fp)
		}
	}

	return nil
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

// knnVectorsReaderDelegate is the per-encoding read surface exposed by the
// codec's KNN vectors reader (stored on SegmentCoreReaders.vectorsReader as
// an interface{}). The concrete reader — *codecs.PerFieldKnnVectorsReader
// wrapping *codecs.Lucene99HnswVectorsReader — satisfies it structurally.
//
// The contract uses only types the index package can name: the index-facing
// [FloatVectorValues] / [ByteVectorValues] interfaces (the codec adapters
// implement both their own and these) and [spi.TopDocs] (shared by
// index and codecs, which both import util/hnsw without a cycle).
type knnVectorsReaderDelegate interface {
	FloatVectorValues(field string) (FloatVectorValues, error)
	ByteVectorValues(field string) (ByteVectorValues, error)
	SearchNearestFloat(field string, target []float32, k int, acceptDocs util.Bits) (*spi.TopDocs, error)
	SearchNearestByte(field string, target []byte, k int, acceptDocs util.Bits) (*spi.TopDocs, error)
	SearchNearestFloatCollector(field string, target []float32, collector spi.KnnCollector, acceptDocs util.Bits) error
	SearchNearestByteCollector(field string, target []byte, collector spi.KnnCollector, acceptDocs util.Bits) error
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
func (r *SegmentReader) GetFloatVectorValues(field string) (spi.FloatVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	vv, err := d.FloatVectorValues(field)
	if err != nil || vv == nil {
		return nil, err
	}
	return newSPIFloatVectorValues(vv), nil
}

// GetByteVectorValues returns the byte vectors for field, delegating to the
// codec's KNN vectors reader. Implements LeafReader.GetByteVectorValues.
func (r *SegmentReader) GetByteVectorValues(field string) (spi.ByteVectorValues, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return nil, nil
	}
	vv, err := d.ByteVectorValues(field)
	if err != nil || vv == nil {
		return nil, err
	}
	return newSPIByteVectorValues(vv), nil
}

// SearchNearestVectors runs top-k nearest-neighbour float-vector search for
// target in field, bounded by visitedLimit visited vectors. Implements
// LeafReader.SearchNearestVectors.
//
// Mirrors org.apache.lucene.index.LeafReader.searchNearestVectors(String,
// float[], int, Bits, int), which wraps (k, visitedLimit) in a
// TopKnnCollector and drives the codec's collector-based search.
func (r *SegmentReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	d := r.vectorsDelegate()
	if d == nil {
		return spi.TopDocs{}, nil
	}
	collector := utilhnsw.NewTopKnnCollector(k, visitedLimit, nil)
	if err := d.SearchNearestFloatCollector(field, target, collector, acceptDocs); err != nil {
		return spi.TopDocs{}, err
	}
	td := collector.TopDocs()
	if td == nil {
		return spi.TopDocs{}, nil
	}
	return *td, nil
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
func (r *SegmentReader) SearchNearestVectorsCollector(field string, target []float32, collector spi.KnnCollector, acceptDocs util.Bits) error {
	d := r.vectorsDelegate()
	if d == nil {
		return nil
	}
	return d.SearchNearestFloatCollector(field, target, collector, acceptDocs)
}

// SearchNearestVectorsByteCollector is the byte-vector analogue of
// [SegmentReader.SearchNearestVectorsCollector].
func (r *SegmentReader) SearchNearestVectorsByteCollector(field string, target []byte, collector spi.KnnCollector, acceptDocs util.Bits) error {
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
// the index-facing [spi.PointValues] type, so the index package can name it
// without importing codecs.
type pointsReaderDelegate interface {
	GetValues(field string) (spi.PointValues, error)
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
// reader or the field has no indexed points. Mirrors
// org.apache.lucene.index.SegmentReader.getPointValues / CodecReader.
func (r *SegmentReader) GetPointValues(field string) (spi.PointValues, error) {
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

// dvFieldInfo resolves field to its FieldInfo from the segment's exposed
// FieldInfos, returning nil when the field is absent or carries no doc
// values. The codec producer keys on the FieldInfo (in particular its number),
// not the field name.  The exposed FieldInfos may be newer than the core
// readers' base FieldInfos after a doc-values update, so the lookup must use
// the segment-level view.
func (r *SegmentReader) dvFieldInfo(field string) *FieldInfo {
	fis := r.GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || !fi.DocValuesType().HasDocValues() {
		return nil
	}
	return fi
}

// GetDocValues returns the doc values for field as a generic spi.DocValues.
// Returns (nil, nil) when the segment has no doc values producer or the field
// has no doc values.
func (r *SegmentReader) GetDocValues(field *FieldInfo) (spi.DocValues, error) {
	if field == nil {
		return nil, fmt.Errorf("GetDocValues requires a non-nil FieldInfo")
	}
	switch field.DocValuesType() {
	case DocValuesTypeNumeric:
		return r.GetNumericDocValues(field.Name())
	case DocValuesTypeBinary:
		return r.GetBinaryDocValues(field.Name())
	case DocValuesTypeSorted:
		return r.GetSortedDocValues(field.Name())
	case DocValuesTypeSortedNumeric:
		return r.GetSortedNumericDocValues(field.Name())
	case DocValuesTypeSortedSet:
		return r.GetSortedSetDocValues(field.Name())
	default:
		return nil, nil
	}
}

// GetNumericDocValues returns the numeric doc values for field, delegating to
// the codec's doc-values producer. Returns (nil, nil) when the segment has no
// doc-values producer or the field has no numeric doc values. Mirrors
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
// the codec's doc-values producer.
func (r *SegmentReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeBinary {
		return nil, nil
	}
	return d.GetBinary(fi)
}

// GetSortedDocValues returns the sorted doc values for field, delegating to
// the codec's doc-values producer.
func (r *SegmentReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSorted {
		return nil, nil
	}
	return d.GetSorted(fi)
}

// GetSortedNumericDocValues returns the sorted-numeric doc values for field,
// delegating to the codec's doc-values producer.
func (r *SegmentReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedNumeric {
		return nil, nil
	}
	return d.GetSortedNumeric(fi)
}

// GetSortedSetDocValues returns the sorted-set doc values for field,
// delegating to the codec's doc-values producer.
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
	fi := fis.FieldInfoByName(field)
	if fi == nil || !fi.HasNorms() {
		return nil
	}
	return fi
}

// GetNormValues returns the per-document norms for field, delegating to the
// codec's norms producer. Returns (nil, nil) when the segment has no norms
// producer or the field has no norms. Mirrors org.apache.lucene.index
// .SegmentReader.getNormValues / CodecReader.getNormValues.
func (r *SegmentReader) GetNormValues(field string) (NumericDocValues, error) {
	d := r.normsDelegate()
	fi := r.normsFieldInfo(field)
	if d == nil || fi == nil {
		return nil, nil
	}
	return d.GetNorms(fi)
}

// GetDocValuesSkipper returns the DocValuesSkipper for field, or nil when the
// segment carries no skip index for it. Mirrors
// org.apache.lucene.index.CodecReader.getDocValuesSkipper, which asks the
// doc-values producer for the field's skipper when the field was indexed with
// a skip index.
func (r *SegmentReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
	fi := r.dvFieldInfo(field)
	if fi == nil || fi.DocValuesSkipIndexType() == spi.DocValuesSkipIndexTypeNone {
		return nil, nil
	}
	if r.coreReaders == nil {
		return nil, nil
	}
	dvp := r.coreReaders.GetDocValuesProducer()
	if dvp == nil {
		return nil, nil
	}
	return dvp.GetSkipper(fi)
}

// knnTopDocsToIndex converts a util/hnsw TopDocs (the codec search result)
// into the index-package TopDocs struct. The hnsw TopDocs is already
// score-descending; TotalHits is the visited-count lower bound, but the
// index TopDocs.TotalHits records the number of returned hits, matching how
// the index layer reports per-leaf vector results.
func knnTopDocsToIndex(td *spi.TopDocs) TopDocs {
	if td == nil {
		return TopDocs{}
	}
	scoreDocs := make([]ScoreDoc, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		scoreDocs[i] = ScoreDoc{Doc: sd.Doc, Score: sd.Score}
	}
	return TopDocs{TotalHits: len(scoreDocs), ScoreDocs: scoreDocs}
}

// CheckIntegrity verifies the checksums of all files backing this segment.
func (r *SegmentReader) CheckIntegrity() error {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.CheckIntegrity()
}

// GetMetaData returns the reader's metadata.
func (r *SegmentReader) GetMetaData() *IndexReaderMetaData {
	return &IndexReaderMetaData{
		HasDeletions: r.HasDeletions(),
		NumDocs:      r.NumDocs(),
		MaxDoc:       r.MaxDoc(),
	}
}

// EnsureOpen throws an error if the reader is closed.
func (r *SegmentReader) EnsureOpen() error {
	if r.coreReaders == nil {
		return spi.ErrAlreadyClosed
	}
	return nil
}

// Leaves returns all leaf reader contexts.
// Since SegmentReader is a leaf reader, it returns a slice containing only itself.
func (r *SegmentReader) Leaves() ([]*spi.LeafReaderContext, error) {
	ctx := spi.NewLeafReaderContextForReader(r)
	return []*spi.LeafReaderContext{ctx}, nil
}

// GetContext returns the reader context for this leaf reader.
func (r *SegmentReader) GetContext() (IndexReaderContext, error) {
	return spi.NewLeafReaderContextForReader(r), nil
}

// GetLiveDocs returns a Bits representing the live (non-deleted) documents in
// this segment. Returns nil when no documents are deleted.
// When the segment has in-memory deleted ordinals (tracked by Gocene's delete
// path), a dense boolean slice is built and returned as a liveDocs
// implementation; otherwise it falls back to the codec-backed liveDocs read
// by initLiveDocs at construction time (or set directly by
// NewSegmentReaderClone for an NRT snapshot).
func (r *SegmentReader) GetLiveDocs() util.Bits {
	if r.segmentCommitInfo == nil {
		return r.liveDocs
	}
	ords := r.segmentCommitInfo.GetDeletedOrdinals()
	if len(ords) == 0 {
		if r.segmentCommitInfo.HasDeletions() {
			return r.liveDocs
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

// GetHardLiveDocs returns the live-docs bits excluding documents that are not
// live due to soft-deletes.
func (r *SegmentReader) GetHardLiveDocs() util.Bits {
	return r.hardLiveDocs
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
	// SegmentReader doesn't have its own ref count; it delegates to the core readers.
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
	if r.coreReaders != nil {
		return r.coreReaders.GetRefCount()
	}
	return 1
}

// GetCoreCacheHelper returns the CacheHelper tied to this segment's core (codec) data.
func (r *SegmentReader) GetCoreCacheHelper() CacheHelper {
	if r.coreReaders != nil {
		return r.coreReaders.GetCacheHelper()
	}
	return nil
}

// GetReaderCacheHelper returns the CacheHelper for this reader instance.
func (r *SegmentReader) GetReaderCacheHelper() CacheHelper {
	return r.GetCoreCacheHelper()
}

// spiFloatVectorValues adapts the Lucene 10.5.0-shaped [FloatVectorValues] the
// codec KNN readers expose — ordinal-addressed vector values plus an explicit
// [util.DocIndexIterator], mirroring org.apache.lucene.index.FloatVectorValues
// — to the document-addressed [spi.FloatVectorValues] contract that
// [spi.LeafReader] declares.
type spiFloatVectorValues struct {
	values FloatVectorValues
	it     util.DocIndexIterator
}

func newSPIFloatVectorValues(values FloatVectorValues) *spiFloatVectorValues {
	return &spiFloatVectorValues{values: values, it: values.Iterator()}
}

// Get returns the vector of docID, or nil when that document carries none.
// The backing iterator only moves forward, so a request for an already-passed
// document restarts it, matching the random-access contract spi.LeafReader
// callers expect.
func (v *spiFloatVectorValues) Get(docID int) ([]float32, error) {
	if v.it.DocID() > docID {
		v.it = v.values.Iterator()
	}
	if v.it.DocID() < docID {
		if _, err := v.it.Advance(docID); err != nil {
			return nil, err
		}
	}
	if v.it.DocID() != docID {
		return nil, nil
	}
	return v.values.VectorValue(v.it.Index())
}

// Scorer forwards FloatVectorValues.scorer(float[]) to the wrapped ordinal-
// addressed values, narrowing the interface{} the index-side contract returns
// (it cannot name util.VectorScorer's search-side alias) back to the scorer
// contract spi.FloatVectorValues declares.
func (v *spiFloatVectorValues) Scorer(target []float32) (util.VectorScorer, error) {
	scorer, err := v.values.Scorer(target)
	if err != nil || scorer == nil {
		return nil, err
	}
	vs, ok := scorer.(util.VectorScorer)
	if !ok {
		return nil, fmt.Errorf("index: %T is not a util.VectorScorer", scorer)
	}
	return vs, nil
}

func (v *spiFloatVectorValues) Advance(target int) (int, error) { return v.it.Advance(target) }
func (v *spiFloatVectorValues) NextDoc() (int, error)           { return v.it.NextDoc() }
func (v *spiFloatVectorValues) DocID() int                      { return v.it.DocID() }
func (v *spiFloatVectorValues) Dimension() int                  { return v.values.Dimension() }
func (v *spiFloatVectorValues) Size() int                       { return v.values.Size() }

// spiByteVectorValues is the byte-vector counterpart of [spiFloatVectorValues].
type spiByteVectorValues struct {
	values ByteVectorValues
	it     util.DocIndexIterator
}

func newSPIByteVectorValues(values ByteVectorValues) *spiByteVectorValues {
	return &spiByteVectorValues{values: values, it: values.Iterator()}
}

// Get returns the vector of docID, or nil when that document carries none.
func (v *spiByteVectorValues) Get(docID int) ([]byte, error) {
	if v.it.DocID() > docID {
		v.it = v.values.Iterator()
	}
	if v.it.DocID() < docID {
		if _, err := v.it.Advance(docID); err != nil {
			return nil, err
		}
	}
	if v.it.DocID() != docID {
		return nil, nil
	}
	return v.values.VectorValue(v.it.Index())
}

// Scorer forwards ByteVectorValues.scorer(byte[]) to the wrapped ordinal-
// addressed values, narrowing the interface{} the index-side contract returns
// back to the scorer contract spi.ByteVectorValues declares.
func (v *spiByteVectorValues) Scorer(target []byte) (util.VectorScorer, error) {
	scorer, err := v.values.Scorer(target)
	if err != nil || scorer == nil {
		return nil, err
	}
	vs, ok := scorer.(util.VectorScorer)
	if !ok {
		return nil, fmt.Errorf("index: %T is not a util.VectorScorer", scorer)
	}
	return vs, nil
}

func (v *spiByteVectorValues) Advance(target int) (int, error) { return v.it.Advance(target) }
func (v *spiByteVectorValues) NextDoc() (int, error)           { return v.it.NextDoc() }
func (v *spiByteVectorValues) DocID() int                      { return v.it.DocID() }
func (v *spiByteVectorValues) Dimension() int                  { return v.values.Dimension() }
func (v *spiByteVectorValues) Size() int                       { return v.values.Size() }

var (
	_ spi.FloatVectorValues = (*spiFloatVectorValues)(nil)
	_ spi.ByteVectorValues  = (*spiByteVectorValues)(nil)
)

// Ensure SegmentReader implements IndexReaderInterface.
var _ IndexReaderInterface = (*SegmentReader)(nil)

// Ensure SegmentReader implements the leaf-reader contract every index-side
// consumer (LeafReaderContext, IndexWriter's per-segment views) relies on.
var _ LeafReader = (*SegmentReader)(nil)

// Ensure SegmentReader implements the codec-reader contract, as Lucene's
// SegmentReader extends CodecReader.
var _ CodecReader = (*SegmentReader)(nil)
