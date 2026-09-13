// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// SortingCodecReader is a CodecReader which supports sorting documents by a given Sort.
// Mirrors org.apache.lucene.index.SortingCodecReader from Apache Lucene 10.5.0.
type SortingCodecReader struct {
	*FilterCodecReader
	docMap   SorterDocMap
	metaData *LeafMetaData

	// cache for last used DV or Norms instance
	mu           sync.Mutex
	cachedField  string
	cachedObject interface{}
	cacheIsNorms bool
	cacheValid   bool
}

func NewSortingCodecReader(in CodecReader, docMap SorterDocMap, metaData *LeafMetaData) *SortingCodecReader {
	r := &SortingCodecReader{
		FilterCodecReader: NewFilterCodecReader(in),
		docMap:            docMap,
		metaData:          metaData,
	}
	// Register the outermost reader so the accessors CodecReader derives from
	// the producers (getNumericDocValues, getPointValues, searchNearestVectors,
	// ...) see this reader's sorting overrides of getDocValuesReader,
	// getPointsReader and getVectorReader. In Java that follows from virtual
	// dispatch; Go embedding needs the back-reference.
	r.SetSelf(r)
	return r
}

// WrapSortingCodecReader returns a sorted view of reader according to the
// order defined by sort. Mirrors SortingCodecReader.wrap(CodecReader, Sort),
// which computes the doc map with `new Sorter(sort).sort(reader)`.
//
// PORT NOTE: Sort is taken by pointer rather than by value. Java passes the
// Sort by reference and Gocene's Sort carries a slice, so a pointer keeps the
// call sites free of defensive copies.
func WrapSortingCodecReader(reader CodecReader, sort *Sort) (CodecReader, error) {
	docMap, err := sorterSort(sort, reader)
	if err != nil {
		return nil, err
	}
	return WrapWithDocMap(reader, docMap, sort)
}

// WrapWithDocMap is the expert version of Wrap that operates directly on a SorterDocMap.
func WrapWithDocMap(reader CodecReader, docMap SorterDocMap, sort *Sort) (CodecReader, error) {
	metaData := leafMetaDataOf(reader)
	if metaData == nil {
		return nil, fmt.Errorf("index: SortingCodecReader: reader %T does not expose LeafMetaData", reader)
	}
	newMetaData, err := NewLeafMetaData(
		metaData.CreatedVersionMajor,
		metaData.MinVersion,
		sort,
		metaData.HasBlocks,
	)
	if err != nil {
		return nil, err
	}
	if docMap == nil {
		// the reader is already sorted
		view := &sortedViewReader{
			FilterCodecReader: NewFilterCodecReader(reader),
			metaData:          newMetaData,
		}
		view.SetSelf(view)
		return view, nil
	}
	return NewSortingCodecReader(reader, docMap, newMetaData), nil
}

// sorterSort is the port of org.apache.lucene.index.Sorter.sort(LeafReader):
// it returns the doc map that reorders reader's documents into sort order, or
// nil when the documents are already in that order (Java returns null for
// that case, and SortingCodecReader.wrap treats it as "already sorted").
//
// Lucene builds one IndexSorter.DocComparator per sort field and then walks
// the adjacent pairs to detect an already-sorted reader; Gocene's IndexSorter
// already composes the per-field comparators and performs the stable sort, so
// the identity permutation is the equivalent "already sorted" signal.
func sorterSort(sort *Sort, reader CodecReader) (SorterDocMap, error) {
	if sort == nil {
		return nil, fmt.Errorf("index: Sorter: sort is nil")
	}
	oldToNew, err := NewIndexSorter(sort).SortSegment(reader)
	if err != nil {
		return nil, err
	}
	sorted := true
	for old, current := range oldToNew {
		if old != current {
			sorted = false
			break
		}
	}
	if sorted {
		return nil, nil
	}
	newToOld := make([]int, len(oldToNew))
	for old, current := range oldToNew {
		newToOld[current] = old
	}
	return &sorterDocMap{oldToNew: oldToNew, newToOld: newToOld}, nil
}

// sorterDocMap is the permutation produced by sorterSort. Mirrors the
// anonymous Sorter.DocMap returned by Sorter.sort(int, DocComparator).
type sorterDocMap struct {
	oldToNew []int
	newToOld []int
}

func (m *sorterDocMap) OldToNew(oldDocID int) int {
	if oldDocID < 0 || oldDocID >= len(m.oldToNew) {
		return -1
	}
	return m.oldToNew[oldDocID]
}

func (m *sorterDocMap) NewToOld(newDocID int) int {
	if newDocID < 0 || newDocID >= len(m.newToOld) {
		return -1
	}
	return m.newToOld[newDocID]
}

func (m *sorterDocMap) Size() int { return len(m.oldToNew) }

type sortedViewReader struct {
	*FilterCodecReader
	metaData *LeafMetaData
}

// GetLeafMetaData returns the sorted view's leaf metadata. See the PORT NOTE
// on leafMetaDataProvider in multi_sorter.go for why Lucene's
// LeafReader.getMetaData payload is reached through its own accessor.
func (s *sortedViewReader) GetLeafMetaData() *LeafMetaData {
	return s.metaData
}

func (s *sortedViewReader) String() string {
	return fmt.Sprintf("SortingCodecReader(%v)", s.FilterCodecReader.in)
}

// Implementation of the various producers

func (s *SortingCodecReader) GetPostingsReader() FieldsProducer {
	delegate := s.FilterCodecReader.in.GetPostingsReader()
	if delegate == nil {
		return nil
	}
	return &sortingFieldsProducer{
		delegate:   delegate,
		docMap:     s.docMap,
		fieldInfos: s.in.GetFieldInfos(),
	}
}

type sortingFieldsProducer struct {
	delegate   FieldsProducer
	docMap     SorterDocMap
	fieldInfos *FieldInfos
}

func (p *sortingFieldsProducer) Close() error {
	return p.delegate.Close()
}

func (p *sortingFieldsProducer) CheckIntegrity() error {
	return p.delegate.CheckIntegrity()
}

func (p *sortingFieldsProducer) Terms(field string) (Terms, error) {
	terms, err := p.delegate.Terms(field)
	if err != nil || terms == nil {
		return nil, err
	}
	fi := p.fieldInfos.FieldInfoByName(field)
	if fi == nil {
		return nil, nil
	}
	return NewSortingTerms(terms, fi.IndexOptions(), p.docMap), nil
}

// Size returns the number of fields the delegate exposes, or -1 when that
// count is unknown. Mirrors Fields.size(), which SortingCodecReader forwards
// to the delegate.
//
// PORT NOTE: spi.FieldsProducer does not declare Size (Lucene's
// FieldsProducer extends Fields, which does), so the count is recovered by
// assertion; -1 is the value Lucene's Fields.size() contract reserves for
// "unknown".
func (p *sortingFieldsProducer) Size() int {
	if sized, ok := p.delegate.(interface{ Size() int }); ok {
		return sized.Size()
	}
	return -1
}

func (s *SortingCodecReader) GetFieldsReader() StoredFieldsReader {
	delegate := s.FilterCodecReader.in.GetFieldsReader()
	if delegate == nil {
		return nil
	}
	return &sortingStoredFieldsReader{
		delegate: delegate,
		docMap:   s.docMap,
	}
}

type sortingStoredFieldsReader struct {
	delegate StoredFieldsReader
	docMap   SorterDocMap
}

// Prefetch forwards the prefetch hint for the source document backing docID.
// spi.StoredFieldsReader does not declare Prefetch (Lucene's default is a
// no-op), so the delegate's override is recovered by assertion, exactly as
// baseCodecReader does in codec_reader.go.
func (r *sortingStoredFieldsReader) Prefetch(docID int) error {
	if pf, ok := r.delegate.(interface{ Prefetch(docID int) error }); ok {
		return pf.Prefetch(r.docMap.NewToOld(docID))
	}
	return nil
}

// VisitDocument decodes the source document backing docID. Mirrors
// SortingCodecReader's document(int, StoredFieldVisitor) override; Gocene's
// SPI names the method VisitDocument.
func (r *sortingStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	return r.delegate.VisitDocument(r.docMap.NewToOld(docID), visitor)
}

// Clone returns a sorted view over a clone of the delegate. When the delegate
// does not expose Clone (spi.StoredFieldsReader does not declare it), the
// delegate is shared, which is correct for the immutable readers that omit
// the hook.
func (r *sortingStoredFieldsReader) Clone() StoredFieldsReader {
	delegate := r.delegate
	if c, ok := delegate.(interface{ Clone() StoredFieldsReader }); ok {
		delegate = c.Clone()
	}
	return &sortingStoredFieldsReader{
		delegate: delegate,
		docMap:   r.docMap,
	}
}

func (r *sortingStoredFieldsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

func (r *sortingStoredFieldsReader) Close() error {
	return r.delegate.Close()
}

func (s *SortingCodecReader) GetLiveDocs() util.Bits {
	inLiveDocs := s.FilterCodecReader.in.GetLiveDocs()
	if inLiveDocs == nil {
		return nil
	}
	return NewSortingBits(inLiveDocs, s.docMap)
}

func (s *SortingCodecReader) GetPointsReader() PointsReader {
	delegate := s.FilterCodecReader.in.GetPointsReader()
	if delegate == nil {
		return nil
	}
	return &sortingPointsReader{
		delegate: delegate,
		docMap:   s.docMap,
	}
}

type sortingPointsReader struct {
	delegate PointsReader
	docMap   SorterDocMap
}

func (r *sortingPointsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

// GetValues returns the sorted view of the field's point values. The wide
// getValues surface of org.apache.lucene.codecs.PointsReader is recovered by
// assertion (see pointsReaderWithValues in codec_reader.go).
func (r *sortingPointsReader) GetValues(field string) (PointValues, error) {
	valuesReader, ok := r.delegate.(pointsReaderWithValues)
	if !ok {
		return nil, nil
	}
	values, err := valuesReader.GetValues(field)
	if err != nil || values == nil {
		return values, err
	}
	return NewSortingPointValues(values, r.docMap), nil
}

func (r *sortingPointsReader) Close() error {
	return r.delegate.Close()
}

func (s *SortingCodecReader) GetVectorReader() KnnVectorsReader {
	delegate := s.FilterCodecReader.in.GetVectorReader()
	if delegate == nil {
		return nil
	}
	return &sortingKnnVectorsReader{
		delegate: delegate,
		docMap:   s.docMap,
	}
}

type sortingKnnVectorsReader struct {
	delegate KnnVectorsReader
	docMap   SorterDocMap
}

func (r *sortingKnnVectorsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

// GetFloatVectorValues returns the sorted view of the field's float vectors.
// The wide read surface of org.apache.lucene.codecs.KnnVectorsReader is
// recovered by assertion (see knnVectorsReaderWithValues in codec_reader.go).
func (r *sortingKnnVectorsReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	wide, ok := r.delegate.(knnVectorsReaderWithValues)
	if !ok {
		return nil, nil
	}
	delegate, err := wide.GetFloatVectorValues(field)
	if err != nil || delegate == nil {
		return delegate, err
	}
	return NewSortingFloatVectorValues(delegate, r.docMap)
}

// GetByteVectorValues returns the sorted view of the field's byte vectors.
func (r *sortingKnnVectorsReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	wide, ok := r.delegate.(knnVectorsReaderWithValues)
	if !ok {
		return nil, nil
	}
	delegate, err := wide.GetByteVectorValues(field)
	if err != nil || delegate == nil {
		return delegate, err
	}
	return NewSortingByteVectorValues(delegate, r.docMap)
}

func (r *sortingKnnVectorsReader) Search(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) {
	// Not implemented as per Lucene's UnsupportedOperationException
	panic("not implemented")
}

func (r *sortingKnnVectorsReader) SearchBytes(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) {
	panic("not implemented")
}

// GetMergeInstance returns the receiver. SortingCodecReader's KnnVectorsReader
// overrides only getOffHeapByteSize in Apache Lucene 10.5.0, so it inherits the
// KnnVectorsReader default, which returns this.
func (r *sortingKnnVectorsReader) GetMergeInstance() (KnnVectorsReader, error) { return r, nil }

// FinishMerge does nothing. SortingCodecReader's KnnVectorsReader does not
// override finishMerge in Apache Lucene 10.5.0, so it inherits the
// KnnVectorsReader default, whose body is empty.
func (r *sortingKnnVectorsReader) FinishMerge() error { return nil }

// GetOffHeapByteSize forwards the delegate's off-heap accounting. When the
// delegate does not expose the hook, an empty map is returned, which is the
// default of org.apache.lucene.codecs.KnnVectorsReader.getOffHeapByteSize.
func (r *sortingKnnVectorsReader) GetOffHeapByteSize(fieldInfo *FieldInfo) map[string]int64 {
	if sized, ok := r.delegate.(interface {
		GetOffHeapByteSize(fieldInfo *FieldInfo) map[string]int64
	}); ok {
		return sized.GetOffHeapByteSize(fieldInfo)
	}
	return map[string]int64{}
}

func (r *sortingKnnVectorsReader) Close() error {
	return r.delegate.Close()
}

func (s *SortingCodecReader) GetNormsReader() NormsProducer {
	delegate := s.FilterCodecReader.in.GetNormsReader()
	if delegate == nil {
		return nil
	}
	return &sortingNormsProducer{
		delegate: delegate,
		s:        s,
	}
}

type sortingNormsProducer struct {
	delegate NormsProducer
	s        *SortingCodecReader
}

func (p *sortingNormsProducer) GetNorms(field *FieldInfo) (NumericDocValues, error) {
	obj, err := p.s.getOrCreate(field.Name(), true, func() (interface{}, error) {
		old, err := p.delegate.GetNorms(field)
		if err != nil {
			return nil, err
		}
		return p.s.sortNumericDocValues(old)
	})
	if err != nil {
		return nil, err
	}
	dvs, ok := obj.(*numericDVs)
	if !ok || dvs == nil {
		return nil, nil
	}
	return newSortingNumericDocValues(dvs), nil
}

// GetMergeInstance returns the receiver. SortingCodecReader's norms producer
// does not override getMergeInstance in Apache Lucene 10.5.0, so it inherits
// the NormsProducer default, which returns this.
func (p *sortingNormsProducer) GetMergeInstance() NormsProducer { return p }

func (p *sortingNormsProducer) CheckIntegrity() error {
	return p.delegate.CheckIntegrity()
}

func (p *sortingNormsProducer) Close() error {
	return p.delegate.Close()
}

func (s *SortingCodecReader) GetDocValuesReader() DocValuesProducer {
	delegate := s.FilterCodecReader.in.GetDocValuesReader()
	if delegate == nil {
		return nil
	}
	return &sortingDocValuesProducer{
		delegate: delegate,
		s:        s,
	}
}

type sortingDocValuesProducer struct {
	delegate DocValuesProducer
	s        *SortingCodecReader
}

func (p *sortingDocValuesProducer) GetNumeric(field *FieldInfo) (NumericDocValues, error) {
	obj, err := p.s.getOrCreate(field.Name(), false, func() (interface{}, error) {
		old, err := p.delegate.GetNumeric(field)
		if err != nil {
			return nil, err
		}
		return p.s.sortNumericDocValues(old)
	})
	if err != nil {
		return nil, err
	}
	dvs, ok := obj.(*numericDVs)
	if !ok || dvs == nil {
		return nil, nil
	}
	return newSortingNumericDocValues(dvs), nil
}

func (p *sortingDocValuesProducer) GetBinary(field *FieldInfo) (BinaryDocValues, error) {
	obj, err := p.s.getOrCreate(field.Name(), false, func() (interface{}, error) {
		old, err := p.delegate.GetBinary(field)
		if err != nil {
			return nil, err
		}
		if old == nil {
			return nil, nil
		}
		return NewBinaryDVs(p.s.MaxDoc(), p.s.docMap, old), nil
	})
	if err != nil {
		return nil, err
	}
	dvs, ok := obj.(BinaryDocValues)
	if !ok || dvs == nil {
		return nil, nil
	}
	return dvs, nil
}

func (p *sortingDocValuesProducer) GetSorted(field *FieldInfo) (SortedDocValues, error) {
	old, err := p.delegate.GetSorted(field)
	if err != nil || old == nil {
		return nil, err
	}
	obj, err := p.s.getOrCreate(field.Name(), false, func() (interface{}, error) {
		maxDoc := p.s.MaxDoc()
		ords := make([]int, maxDoc)
		for i := range ords {
			ords[i] = -1
		}
		for {
			docID, err := old.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID == NO_MORE_DOCS || docID < 0 || docID >= maxDoc {
				break
			}
			ord, err := old.OrdValue()
			if err != nil {
				return nil, err
			}
			ords[p.s.docMap.OldToNew(docID)] = ord
		}
		return ords, nil
	})
	if err != nil {
		return nil, err
	}
	ords, ok := obj.([]int)
	if !ok {
		return nil, nil
	}
	return newSortingSortedDocValues(old, ords), nil
}

func (p *sortingDocValuesProducer) GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error) {
	old, err := p.delegate.GetSortedNumeric(field)
	if err != nil || old == nil {
		return nil, err
	}
	obj, err := p.s.getOrCreate(field.Name(), false, func() (interface{}, error) {
		return newSortedNumericLongValues(p.s.MaxDoc(), p.s.docMap, old, packed.Fast)
	})
	if err != nil {
		return nil, err
	}
	values, ok := obj.(*sortedNumericLongValues)
	if !ok || values == nil {
		return nil, nil
	}
	return newSortingSortedNumericDocValues(old, values), nil
}

func (p *sortingDocValuesProducer) GetSortedSet(field *FieldInfo) (SortedSetDocValues, error) {
	old, err := p.delegate.GetSortedSet(field)
	if err != nil || old == nil {
		return nil, err
	}
	obj, err := p.s.getOrCreate(field.Name(), false, func() (interface{}, error) {
		return newDocOrds(p.s.MaxDoc(), p.s.docMap, old, packed.Fast, startBitsPerValue)
	})
	if err != nil {
		return nil, err
	}
	ords, ok := obj.(*docOrds)
	if !ok || ords == nil {
		return nil, nil
	}
	return newSortingSortedSetDocValues(old, ords), nil
}

// GetMergeInstance returns the receiver. The corresponding class in Apache
// Lucene 10.5.0 does not override getMergeInstance, so it inherits the
// DocValuesProducer default, which returns this.
func (p *sortingDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *sortingDocValuesProducer) CheckIntegrity() error {
	return p.delegate.CheckIntegrity()
}

func (p *sortingDocValuesProducer) Close() error {
	return p.delegate.Close()
}

// GetSkipper returns nil: min/max information cannot be reported once the doc
// IDs have been reordered. Mirrors SortingCodecReader's getSkipper override.
func (p *sortingDocValuesProducer) GetSkipper(field *FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

func (s *SortingCodecReader) GetTermVectorsReader() TermVectorsReader {
	delegate := s.FilterCodecReader.in.GetTermVectorsReader()
	if delegate == nil {
		return nil
	}
	return &sortingTermVectorsReader{
		delegate: delegate,
		docMap:   s.docMap,
	}
}

type sortingTermVectorsReader struct {
	delegate TermVectorsReader
	docMap   SorterDocMap
}

// Prefetch forwards the prefetch hint for the source document backing doc.
// spi.TermVectorsReader does not declare Prefetch (Lucene's default is a
// no-op), so the delegate's override is recovered by assertion.
func (r *sortingTermVectorsReader) Prefetch(doc int) error {
	if pf, ok := r.delegate.(interface{ Prefetch(docID int) error }); ok {
		return pf.Prefetch(r.docMap.NewToOld(doc))
	}
	return nil
}

func (r *sortingTermVectorsReader) Get(doc int) (Fields, error) {
	return r.delegate.Get(r.docMap.NewToOld(doc))
}

func (r *sortingTermVectorsReader) GetField(doc int, field string) (Terms, error) {
	return r.delegate.GetField(r.docMap.NewToOld(doc), field)
}

func (r *sortingTermVectorsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

// Clone returns a sorted view over a clone of the delegate. When the delegate
// does not expose Clone (spi.TermVectorsReader does not declare it), the
// delegate is shared, which is correct for the immutable readers that omit
// the hook.
func (r *sortingTermVectorsReader) Clone() TermVectorsReader {
	delegate := r.delegate
	if c, ok := delegate.(interface{ Clone() TermVectorsReader }); ok {
		delegate = c.Clone()
	}
	return &sortingTermVectorsReader{
		delegate: delegate,
		docMap:   r.docMap,
	}
}

func (r *sortingTermVectorsReader) Close() error {
	return r.delegate.Close()
}

func (s *SortingCodecReader) String() string {
	return fmt.Sprintf("SortingCodecReader(%v)", s.FilterCodecReader.in)
}

func (s *SortingCodecReader) GetCoreCacheHelper() CacheHelper   { return nil }
func (s *SortingCodecReader) GetReaderCacheHelper() CacheHelper { return nil }

// GetLeafMetaData returns the sorted reader's leaf metadata. See the PORT
// NOTE on leafMetaDataProvider in multi_sorter.go.
func (s *SortingCodecReader) GetLeafMetaData() *LeafMetaData { return s.metaData }

// sortNumericDocValues materialises oldNumerics in the sorted doc order.
// Mirrors SortingCodecReader.getNumericDocValues, which builds the
// NumericDocValuesWriter.NumericDVs backing the sorted view.
func (s *SortingCodecReader) sortNumericDocValues(oldNumerics NumericDocValues) (*numericDVs, error) {
	if oldNumerics == nil {
		return nil, nil
	}
	maxDoc := s.MaxDoc()
	docsWithField, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	values := make([]int64, maxDoc)
	for {
		docID, err := oldNumerics.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == NO_MORE_DOCS || docID < 0 || docID >= maxDoc {
			break
		}
		value, err := oldNumerics.LongValue()
		if err != nil {
			return nil, err
		}
		newDocID := s.docMap.OldToNew(docID)
		docsWithField.Set(newDocID)
		values[newDocID] = value
	}
	return newNumericDVs(values, docsWithField), nil
}

// getOrCreate returns the cached materialised doc-values (or norms) for field,
// building them through supplier on a miss. Mirrors
// SortingCodecReader.getOrCreate: exactly one field's values are held at a
// time, because the merge and flush paths consume the fields one after the
// other.
func (s *SortingCodecReader) getOrCreate(field string, norms bool, supplier func() (interface{}, error)) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cacheValid && s.cachedField == field && s.cacheIsNorms == norms {
		return s.cachedObject, nil
	}

	obj, err := supplier()
	if err != nil {
		return nil, err
	}
	s.cachedObject = obj
	s.cachedField = field
	s.cacheIsNorms = norms
	s.cacheValid = true
	return obj, nil
}
