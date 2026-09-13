// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FilterCodecReader contains another CodecReader, which it uses as its basic source
// of data, possibly transforming the data along the way or providing additional functionality.
//
// This is the Go port of Lucene's org.apache.lucene.index.FilterCodecReader.
type FilterCodecReader struct {
	in CodecReader

	// self is the outermost reader of the embedding chain, when there is one.
	//
	// PORT NOTE: Lucene's CodecReader implements getNumericDocValues, postings,
	// getPointValues and the rest as final methods written in terms of
	// getDocValuesReader()/getPostingsReader()/..., so a subclass that overrides
	// only a producer accessor (SortingCodecReader does exactly that) changes
	// every derived accessor with it. Go embedding has no virtual dispatch: a
	// method promoted from *FilterCodecReader would keep calling
	// FilterCodecReader's own accessor and silently bypass the override. The
	// embedder therefore registers itself through SetSelf, and the derived
	// methods below route through codecSelf() to reproduce Java's dispatch.
	self CodecReader

	// Overrides for WrapLiveDocs
	liveDocsOverride util.Bits
	numDocsOverride  *int
}

// NewFilterCodecReader creates a new FilterCodecReader wrapping the given reader.
func NewFilterCodecReader(in CodecReader) *FilterCodecReader {
	return &FilterCodecReader{
		in: in,
	}
}

// Unwrap returns the wrapped instance by reader as long as this reader is an instance of FilterCodecReader.
func Unwrap(reader LeafReader) LeafReader {
	for {
		if delegate, ok := reader.(interface{ GetDelegate() LeafReader }); ok {
			reader = delegate.GetDelegate()
		} else {
			break
		}
	}
	return reader
}

// GetDelegate returns the wrapped CodecReader.
func (r *FilterCodecReader) GetDelegate() LeafReader {
	return r.in
}

// WrapLiveDocs returns a filtered codec reader with the given live docs and numDocs.
func WrapLiveDocs(reader CodecReader, liveDocs util.Bits, numDocs int) *FilterCodecReader {
	return &FilterCodecReader{
		in:               reader,
		liveDocsOverride: liveDocs,
		numDocsOverride:  &numDocs,
	}
}

// SetSelf registers the outermost reader embedding this FilterCodecReader so
// that the derived accessors dispatch through its producer overrides. It is a
// no-op to call it with nil.
func (r *FilterCodecReader) SetSelf(self CodecReader) {
	r.self = self
}

// codecSelf returns the reader whose producer accessors the derived methods
// must consult: the registered outermost embedder, or this reader itself.
func (r *FilterCodecReader) codecSelf() CodecReader {
	if r.self != nil {
		return r.self
	}
	return r
}

// docValuesDelegate resolves the doc-values producer down to the accessor
// surface, the same optional-interface pattern index/segment_reader.go uses.
func (r *FilterCodecReader) docValuesDelegate() docValuesProducerDelegate {
	dvp := r.codecSelf().GetDocValuesReader()
	if dvp == nil {
		return nil
	}
	d, ok := dvp.(docValuesProducerDelegate)
	if !ok {
		return nil
	}
	return d
}

func (r *FilterCodecReader) normsDelegate() normsProducerDelegate {
	np := r.codecSelf().GetNormsReader()
	if np == nil {
		return nil
	}
	d, ok := np.(normsProducerDelegate)
	if !ok {
		return nil
	}
	return d
}

func (r *FilterCodecReader) pointsDelegate() pointsReaderDelegate {
	pr := r.codecSelf().GetPointsReader()
	if pr == nil {
		return nil
	}
	d, ok := pr.(pointsReaderDelegate)
	if !ok {
		return nil
	}
	return d
}

func (r *FilterCodecReader) vectorsDelegate() knnVectorsReaderDelegate {
	vr := r.codecSelf().GetVectorReader()
	if vr == nil {
		return nil
	}
	d, ok := vr.(knnVectorsReaderDelegate)
	if !ok {
		return nil
	}
	return d
}

// dvFieldInfo returns the FieldInfo for field when it carries doc values, and
// nil otherwise. Port of the private CodecReader.getDVField check.
func (r *FilterCodecReader) dvFieldInfo(field string) *FieldInfo {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || !fi.DocValuesType().HasDocValues() {
		return nil
	}
	return fi
}

// DocID mirrors the doc-ID position of a reader, which is always 0 for a
// non-iterating reader.
func (r *FilterCodecReader) DocID() int {
	return 0
}

// TotalTermFreq ports the final method LeafReader.totalTermFreq(Term).
func (r *FilterCodecReader) TotalTermFreq(term Term) (int64, error) {
	terms, err := r.codecSelf().Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te, err := terms.GetIterator()
	if err != nil || te == nil {
		return 0, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return te.TotalTermFreq()
}

// Postings ports the final method LeafReader.postings(Term, int).
func (r *FilterCodecReader) Postings(term Term, flags int) (PostingsEnum, error) {
	terms, err := r.codecSelf().Terms(term.Field)
	if err != nil || terms == nil {
		return nil, err
	}
	te, err := terms.GetIterator()
	if err != nil || te == nil {
		return nil, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

// GetNumericDocValues ports the final method CodecReader.getNumericDocValues.
func (r *FilterCodecReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeNumeric {
		return nil, nil
	}
	return d.GetNumeric(fi)
}

// GetBinaryDocValues ports the final method CodecReader.getBinaryDocValues.
func (r *FilterCodecReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeBinary {
		return nil, nil
	}
	return d.GetBinary(fi)
}

// GetSortedDocValues ports the final method CodecReader.getSortedDocValues.
func (r *FilterCodecReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSorted {
		return nil, nil
	}
	return d.GetSorted(fi)
}

// GetSortedNumericDocValues ports the final method
// CodecReader.getSortedNumericDocValues.
func (r *FilterCodecReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedNumeric {
		return nil, nil
	}
	return d.GetSortedNumeric(fi)
}

// GetSortedSetDocValues ports the final method CodecReader.getSortedSetDocValues.
func (r *FilterCodecReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	d := r.docValuesDelegate()
	fi := r.dvFieldInfo(field)
	if d == nil || fi == nil || fi.DocValuesType() != DocValuesTypeSortedSet {
		return nil, nil
	}
	return d.GetSortedSet(fi)
}

// GetDocValuesSkipper ports the final method CodecReader.getDocValuesSkipper.
func (r *FilterCodecReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.DocValuesSkipIndexType() == spi.DocValuesSkipIndexTypeNone {
		return nil, nil
	}
	dvp := r.codecSelf().GetDocValuesReader()
	if dvp == nil {
		return nil, nil
	}
	return dvp.GetSkipper(fi)
}

// GetNormValues ports the final method CodecReader.getNormValues.
func (r *FilterCodecReader) GetNormValues(field string) (NumericDocValues, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || !fi.HasNorms() {
		return nil, nil
	}
	d := r.normsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.GetNorms(fi)
}

// GetPointValues ports the final method CodecReader.getPointValues.
func (r *FilterCodecReader) GetPointValues(field string) (spi.PointValues, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.PointDimensionCount() == 0 {
		return nil, nil
	}
	d := r.pointsDelegate()
	if d == nil {
		return nil, nil
	}
	return d.GetValues(field)
}

// GetFloatVectorValues ports the final method CodecReader.getFloatVectorValues.
func (r *FilterCodecReader) GetFloatVectorValues(field string) (spi.FloatVectorValues, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != util.VectorEncodingFloat32 {
		return nil, nil
	}
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

// GetByteVectorValues ports the final method CodecReader.getByteVectorValues.
func (r *FilterCodecReader) GetByteVectorValues(field string) (spi.ByteVectorValues, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != util.VectorEncodingByte {
		return nil, nil
	}
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

// SearchNearestVectors ports the final method
// CodecReader.searchNearestVectors(String, float[], KnnCollector, AcceptDocs).
func (r *FilterCodecReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return spi.TopDocs{}, nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != util.VectorEncodingFloat32 {
		return spi.TopDocs{}, nil
	}
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

// SearchNearestVectorsCollector ports the final method
// CodecReader.searchNearestVectors(String, float[], KnnCollector, AcceptDocs),
// resolved through this reader's own getVectorReader override.
func (r *FilterCodecReader) SearchNearestVectorsCollector(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != util.VectorEncodingFloat32 {
		// Field does not exist or does not index vectors
		return nil
	}
	d := r.vectorsDelegate()
	if d == nil {
		return nil
	}
	return d.SearchNearestFloatCollector(field, target, knnCollector, acceptDocs)
}

// SearchNearestVectorsByteCollector ports the final method
// CodecReader.searchNearestVectors(String, byte[], KnnCollector, AcceptDocs),
// resolved through this reader's own getVectorReader override.
func (r *FilterCodecReader) SearchNearestVectorsByteCollector(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	fis := r.codecSelf().GetFieldInfos()
	if fis == nil {
		return nil
	}
	fi := fis.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != util.VectorEncodingByte {
		// Field does not exist or does not index vectors
		return nil
	}
	d := r.vectorsDelegate()
	if d == nil {
		return nil
	}
	return d.SearchNearestByteCollector(field, target, knnCollector, acceptDocs)
}

// GetCoreCacheHelper ports FilterCodecReader.getCoreCacheHelper.
func (r *FilterCodecReader) GetCoreCacheHelper() CacheHelper {
	return r.in.GetCoreCacheHelper()
}

// GetReaderCacheHelper ports FilterCodecReader.getReaderCacheHelper.
func (r *FilterCodecReader) GetReaderCacheHelper() CacheHelper {
	return r.in.GetReaderCacheHelper()
}

// --- Delegation Methods ---

func (r *FilterCodecReader) Close() error {
	return r.in.Close()
}

func (r *FilterCodecReader) DocCount() int {
	return r.in.DocCount()
}

func (r *FilterCodecReader) NumDocs() int {
	if r.numDocsOverride != nil {
		return *r.numDocsOverride
	}
	return r.in.NumDocs()
}

func (r *FilterCodecReader) MaxDoc() int {
	return r.in.MaxDoc()
}

func (r *FilterCodecReader) HasDeletions() bool {
	return r.in.HasDeletions()
}

func (r *FilterCodecReader) NumDeletedDocs() int {
	return r.in.NumDeletedDocs()
}

func (r *FilterCodecReader) GetLiveDocs() util.Bits {
	if r.liveDocsOverride != nil {
		return r.liveDocsOverride
	}
	return r.in.GetLiveDocs()
}

func (r *FilterCodecReader) GetFieldInfos() *FieldInfos {
	return r.in.GetFieldInfos()
}

func (r *FilterCodecReader) Terms(field string) (Terms, error) {
	return r.in.Terms(field)
}

// GetTermVectors returns the term vectors of docID. Mirrors the Lucene
// convenience that reads through TermVectors().get(docID).
// DocFreq returns the number of documents containing term.
//
// Port of the final method org.apache.lucene.index.LeafReader.docFreq(Term),
// which FilterCodecReader inherits: it resolves the term through this reader's
// own Terms(field), so the filter's overrides are honoured. Terms.getTerms
// substitutes Terms.EMPTY for a missing field, whose iterator never matches —
// hence the zero returned here when the field is absent.
func (r *FilterCodecReader) DocFreq(term Term) (int, error) {
	terms, err := r.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	te, err := terms.GetIterator()
	if err != nil || te == nil {
		return 0, err
	}
	found, err := te.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return te.DocFreq()
}

func (r *FilterCodecReader) GetTermVectors(docID int) (Fields, error) {
	tv, err := r.in.TermVectors()
	if err != nil || tv == nil {
		return nil, err
	}
	return tv.Get(docID)
}

func (r *FilterCodecReader) StoredFields() (StoredFields, error) {
	return r.in.StoredFields()
}

func (r *FilterCodecReader) TermVectors() (TermVectors, error) {
	return r.in.TermVectors()
}

// GetCoreReaders returns the wrapped reader's shared core readers when it owns
// a segment core (a SegmentReader does), or nil otherwise. CodecReader is the
// codec-facing contract and does not declare the accessor, so it is recovered
// from the concrete delegate.
func (r *FilterCodecReader) GetCoreReaders() *SegmentCoreReaders {
	if c, ok := r.in.(interface{ GetCoreReaders() *SegmentCoreReaders }); ok {
		return c.GetCoreReaders()
	}
	return nil
}

// GetCoreCacheKey returns the delegate's core cache key, i.e. the CacheKey of
// its core CacheHelper (Lucene's IndexReader.getCoreCacheHelper().getKey()).
func (r *FilterCodecReader) GetCoreCacheKey() interface{} {
	if h := r.in.GetCoreCacheHelper(); h != nil {
		return h.CacheKey()
	}
	return nil
}

func (r *FilterCodecReader) GetTermVectorsReader() TermVectorsReader {
	return r.in.GetTermVectorsReader()
}

// GetStoredFieldsReader returns the delegate's stored-fields reader. Alias of
// [FilterCodecReader.GetFieldsReader], which carries the Lucene name.
func (r *FilterCodecReader) GetStoredFieldsReader() StoredFieldsReader {
	return r.in.GetFieldsReader()
}

// GetFieldsReader returns the delegate's stored-fields reader, mirroring
// org.apache.lucene.index.FilterCodecReader.getFieldsReader().
func (r *FilterCodecReader) GetFieldsReader() StoredFieldsReader {
	return r.in.GetFieldsReader()
}

func (r *FilterCodecReader) GetPostingsReader() FieldsProducer {
	return r.in.GetPostingsReader()
}

func (r *FilterCodecReader) GetDocValuesReader() DocValuesProducer {
	return r.in.GetDocValuesReader()
}

func (r *FilterCodecReader) GetNormsReader() NormsProducer {
	return r.in.GetNormsReader()
}

func (r *FilterCodecReader) GetPointsReader() PointsReader {
	return r.in.GetPointsReader()
}

func (r *FilterCodecReader) GetVectorReader() KnnVectorsReader {
	return r.in.GetVectorReader()
}

func (r *FilterCodecReader) IncRef() error {
	return r.in.IncRef()
}

func (r *FilterCodecReader) DecRef() error {
	return r.in.DecRef()
}

func (r *FilterCodecReader) TryIncRef() bool {
	return r.in.TryIncRef()
}

func (r *FilterCodecReader) GetRefCount() int32 {
	return r.in.GetRefCount()
}

func (r *FilterCodecReader) EnsureOpen() error {
	return r.in.EnsureOpen()
}

func (r *FilterCodecReader) GetContext() (IndexReaderContext, error) {
	return r.in.GetContext()
}

func (r *FilterCodecReader) Leaves() ([]*LeafReaderContext, error) {
	return r.in.Leaves()
}

func (r *FilterCodecReader) CheckIntegrity() error {
	return r.in.CheckIntegrity()
}

func (r *FilterCodecReader) GetMetaData() *IndexReaderMetaData {
	return r.in.GetMetaData()
}

func (r *FilterCodecReader) GetSegmentInfo() *SegmentInfo {
	return r.in.GetSegmentInfo()
}
