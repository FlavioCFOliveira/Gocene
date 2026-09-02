//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortingCodecReader is a CodecReader which supports sorting documents by a given Sort.
// Mirrors org.apache.lucene.index.SortingCodecReader from Apache Lucene 10.5.0.
type SortingCodecReader struct {
	*FilterCodecReader
	docMap   *SorterDocMap
	metaData *LeafMetaData

	// cache for last used DV or Norms instance
	mu            sync.Mutex
	cachedField   string
	cachedObject  interface{}
	cacheIsNorms  bool
}

func NewSortingCodecReader(in CodecReader, docMap *SorterDocMap, metaData *LeafMetaData) *SortingCodecReader {
	return &SortingCodecReader{
		FilterCodecReader: NewFilterCodecReader(in),
		docMap:            docMap,
		metaData:          metaData,
	}
}

// Wrap returns a sorted view of reader according to the order defined by sort.
func Wrap(reader CodecReader, sort Sort) (CodecReader, error) {
	sorter := NewSorter(sort)
	docMap := sorter.Sort(reader)
	return WrapWithDocMap(reader, docMap, sort)
}

// WrapWithDocMap is the expert version of Wrap that operates directly on a SorterDocMap.
func WrapWithDocMap(reader CodecReader, docMap *SorterDocMap, sort Sort) CodecReader {
	metaData := reader.GetMetaData()
	newMetaData := NewLeafMetaData(
		metaData.CreatedVersionMajor(),
		metaData.MinVersion(),
		sort,
		metaData.HasBlocks(),
	)
	if docMap == nil {
		// the reader is already sorted
		return &sortedViewReader{
			FilterCodecReader: NewFilterCodecReader(reader),
			metaData:          newMetaData,
		}
	}
	return NewSortingCodecReader(reader, docMap, newMetaData)
}

type sortedViewReader struct {
	*FilterCodecReader
	metaData *LeafMetaData
}

func (s *sortedViewReader) GetMetaData() *LeafMetaData {
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
		delegate: delegate,
		docMap:   s.docMap,
		fieldInfos: s.in.GetFieldInfos(),
	}
}

type sortingFieldsProducer struct {
	delegate   FieldsProducer
	docMap     *SorterDocMap
	fieldInfos *FieldInfos
}

func (p *sortingFieldsProducer) Close() error {
	return p.delegate.Close()
}

func (p *sortingFieldsProducer) CheckIntegrity() error {
	return p.delegate.CheckIntegrity()
}

func (p *sortingFieldsProducer) Iterator() iterator.Iterator {
	return p.delegate.Iterator()
}

func (p *sortingFieldsProducer) Terms(field string) (Terms, error) {
	terms := p.delegate.Terms(field)
	if terms == nil {
		return nil, nil
	}
	return NewSortingTerms(terms, p.fieldInfos.FieldInfo(field).GetIndexOptions(), p.docMap), nil
}

func (p *sortingFieldsProducer) Size() int {
	return p.delegate.Size()
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
	docMap   *SorterDocMap
}

func (r *sortingStoredFieldsReader) Prefetch(docID int) error {
	return r.delegate.Prefetch(r.docMap.NewToOld(docID))
}

func (r *sortingStoredFieldsReader) Document(docID int, visitor StoredFieldVisitor) error {
	return r.delegate.Document(r.docMap.NewToOld(docID), visitor)
}

func (r *sortingStoredFieldsReader) Clone() StoredFieldsReader {
	return &sortingStoredFieldsReader{
		delegate: r.delegate.Clone(),
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
	docMap   *SorterDocMap
}

func (r *sortingPointsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

func (r *sortingPointsReader) GetValues(field string) (PointValues, error) {
	values, err := r.delegate.GetValues(field)
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
	docMap   *SorterDocMap
}

func (r *sortingKnnVectorsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

func (r *sortingKnnVectorsReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	delegate, err := r.delegate.GetFloatVectorValues(field)
	if err != nil || delegate == nil {
		return delegate, err
	}
	return NewSortingFloatVectorValues(delegate, r.docMap), nil
}

func (r *sortingKnnVectorsReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	delegate, err := r.delegate.GetByteVectorValues(field)
	if err != nil || delegate == nil {
		return delegate, err
	}
	return NewSortingByteVectorValues(delegate, r.docMap), nil
}

func (r *sortingKnnVectorsReader) Search(field string, target []float32, knnCollector KnnCollector, acceptDocs util.Bits) {
	// Not implemented as per Lucene's UnsupportedOperationException
	panic("not implemented")
}

func (r *sortingKnnVectorsReader) SearchBytes(field string, target []byte, knnCollector KnnCollector, acceptDocs util.Bits) {
	panic("not implemented")
}

func (r *sortingKnnVectorsReader) GetOffHeapByteSize(fieldInfo FieldInfo) map[string]int64 {
	return r.delegate.GetOffHeapByteSize(fieldInfo)
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
	return p.s.getOrCreate(field.Name, true, func() (interface{}, error) {
		return p.delegate.GetNorms(field)
	}).(NumericDocValues), nil
}

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
	return p.s.getOrCreate(field.Name, false, func() (interface{}, error) {
		return p.delegate.GetNumeric(field)
	}).(NumericDocValues), nil
}

func (p *sortingDocValuesProducer) GetBinary(field *FieldInfo) (BinaryDocValues, error) {
	return p.s.getOrCreate(field.Name, false, func() (interface{}, error) {
		return p.delegate.GetBinary(field)
	}).(BinaryDocValues), nil
}

func (p *sortingDocValuesProducer) GetSorted(field *FieldInfo) (SortedDocValues, error) {
	old := p.delegate.GetSorted(field)
	return NewSortingSortedDocValues(old, p.s.docMap), nil
}

func (p *sortingDocValuesProducer) GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error) {
	old := p.delegate.GetSortedNumeric(field)
	return NewSortingSortedNumericDocValues(old, p.s.docMap), nil
}

func (p *sortingDocValuesProducer) GetSortedSet(field *FieldInfo) (SortedSetDocValues, error) {
	old := p.delegate.GetSortedSet(field)
	return NewSortingSortedSetDocValues(old, p.s.docMap), nil
}

func (p *sortingDocValuesProducer) CheckIntegrity() error {
	return p.delegate.CheckIntegrity()
}

func (p *sortingDocValuesProducer) Close() error {
	return p.delegate.Close()
}

func (p *sortingDocValuesProducer) GetSkipper(field *FieldInfo) (DocValuesSkipper, error) {
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
	docMap   *SorterDocMap
}

func (r *sortingTermVectorsReader) Prefetch(doc int) error {
	return r.delegate.Prefetch(r.docMap.NewToOld(doc))
}

func (r *sortingTermVectorsReader) Get(doc int) (Fields, error) {
	return r.delegate.Get(r.docMap.NewToOld(doc))
}

func (r *sortingTermVectorsReader) CheckIntegrity() error {
	return r.delegate.CheckIntegrity()
}

func (r *sortingTermVectorsReader) Clone() TermVectorsReader {
	return &sortingTermVectorsReader{
		delegate: r.delegate.Clone(),
		docMap:   r.docMap,
	}
}

func (r *sortingTermVectorsReader) Close() error {
	return r.delegate.Close()
}

func (s *SortingCodecReader) String() string {
	return fmt.Sprintf("SortingCodecReader(%v)", s.FilterCodecReader.in)
}

func (s *SortingCodecReader) GetCoreCacheHelper() CacheHelper { return nil }
func (s *SortingCodecReader) GetReaderCacheHelper() CacheHelper { return nil }
func (s *SortingCodecReader) GetMetaData() *LeafMetaData { return s.metaData }

func (s *SortingCodecReader) getOrCreate(field string, norms bool, supplier func() (interface{}, error)) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedField == field && s.cacheIsNorms == norms {
		return s.cachedObject
	}

	obj, err := supplier()
	if err != nil {
		panic(err)
	}
	s.cachedObject = obj
	s.cachedField = field
	s.cacheIsNorms = norms
	return obj
}
