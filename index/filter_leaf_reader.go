package index

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterLeafReader contains another LeafReader, which it uses as its basic source of
// data, possibly transforming the data along the way or providing additional functionality.
// This is the Go port of Lucene's org.apache.lucene.index.FilterLeafReader.
type FilterLeafReader struct {
	in LeafReader
}

// NewFilterLeafReader constructs a FilterLeafReader based on the specified base reader.
func NewFilterLeafReader(in LeafReader) *FilterLeafReader {
	if in == nil {
		panic("incoming LeafReader must not be null")
	}
	return &FilterLeafReader{in: in}
}

// Unwrap returns the wrapped instance by reader as long as this reader is an instance of FilterLeafReader.
func UnwrapLeafReader(reader LeafReader) LeafReader {
	for {
		if flr, ok := reader.(*FilterLeafReader); ok {
			reader = flr.GetDelegate()
		} else {
			break
		}
	}
	return reader
}

// GetDelegate returns the wrapped LeafReader.
func (f *FilterLeafReader) GetDelegate() LeafReader {
	return f.in
}

// --- Implementation of LeafReader interface by delegation ---

func (f *FilterLeafReader) DocID() int {
	return f.in.DocID()
}

func (f *FilterLeafReader) MaxDoc() int {
	return f.in.MaxDoc()
}

func (f *FilterLeafReader) NumDocs() int {
	return f.in.NumDocs()
}

func (f *FilterLeafReader) DocFreq(term Term) (int, error) {
	return f.in.DocFreq(term)
}

func (f *FilterLeafReader) TotalTermFreq(term Term) (int64, error) {
	return f.in.TotalTermFreq(term)
}

func (f *FilterLeafReader) Terms(field string) (Terms, error) {
	return f.in.Terms(field)
}

func (f *FilterLeafReader) Postings(term Term, flags int) (PostingsEnum, error) {
	return f.in.Postings(term, flags)
}

func (f *FilterLeafReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	return f.in.GetNumericDocValues(field)
}

func (f *FilterLeafReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	return f.in.GetBinaryDocValues(field)
}

func (f *FilterLeafReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	return f.in.GetSortedDocValues(field)
}

func (f *FilterLeafReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	return f.in.GetSortedNumericDocValues(field)
}

func (f *FilterLeafReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	return f.in.GetSortedSetDocValues(field)
}

func (f *FilterLeafReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	return f.in.GetDocValuesSkipper(field)
}

func (f *FilterLeafReader) GetNormValues(field string) (NumericDocValues, error) {
	return f.in.GetNormValues(field)
}

func (f *FilterLeafReader) GetMetaData() *IndexReaderMetaData {
	return f.in.GetMetaData()
}

func (f *FilterLeafReader) CheckIntegrity() error {
	return f.in.CheckIntegrity()
}

func (f *FilterLeafReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	return f.in.GetFloatVectorValues(field)
}

func (f *FilterLeafReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	return f.in.GetByteVectorValues(field)
}

func (f *FilterLeafReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (TopDocs, error) {
	return f.in.SearchNearestVectors(field, target, k, acceptDocs, visitedLimit)
}

func (f *FilterLeafReader) TermVectors() (TermVectors, error) {
	return f.in.TermVectors()
}

func (f *FilterLeafReader) StoredFields() (StoredFields, error) {
	return f.in.StoredFields()
}

func (f *FilterLeafReader) GetPointValues(field string) (PointValues, error) {
	return f.in.GetPointValues(field)
}

func (f *FilterLeafReader) GetFieldInfos() *FieldInfos {
	return f.in.GetFieldInfos()
}

func (f *FilterLeafReader) GetLiveDocs() util.Bits {
	return f.in.GetLiveDocs()
}

func (f *FilterLeafReader) Close() error {
	return f.in.Close()
}

func (f *FilterLeafReader) IncRef() error {
	return f.in.IncRef()
}

func (f *FilterLeafReader) DecRef() error {
	return f.in.DecRef()
}

func (f *FilterLeafReader) TryIncRef() bool {
	return f.in.TryIncRef()
}

func (f *FilterLeafReader) GetRefCount() int32 {
	return f.in.GetRefCount()
}

func (f *FilterLeafReader) GetCoreCacheHelper() CacheHelper {
	return f.in.GetCoreCacheHelper()
}

func (f *FilterLeafReader) GetReaderCacheHelper() CacheHelper {
	return f.in.GetReaderCacheHelper()
}

func (f *FilterLeafReader) String() string {
	return fmt.Sprintf("FilterLeafReader(%s)", f.in.String())
}

// --- Inner filter classes as separate types ---

// FilterFields is a base class for filtering Fields implementations.
type FilterFields struct {
	in Fields
}

func NewFilterFields(in Fields) *FilterFields {
	if in == nil {
		panic("incoming Fields must not be null")
	}
	return &FilterFields{in: in}
}

func (f *FilterFields) Iterator() []string {
	// Lucene returns an Iterator<String>. In Go, we typically return a slice or an iterator object.
	// Assuming Fields.Iterator() returns []string or similar.
	return f.in.Iterator()
}

func (f *FilterFields) Terms(field string) (Terms, error) {
	return f.in.Terms(field)
}

func (f *FilterFields) Size() int {
	return f.in.Size()
}

// FilterTerms is a base class for filtering Terms implementations.
type FilterTerms struct {
	in Terms
}

func NewFilterTerms(in Terms) *FilterTerms {
	if in == nil {
		panic("incoming Terms must not be null")
	}
	return &FilterTerms{in: in}
}

func (f *FilterTerms) Iterator() (TermsEnum, error) {
	return f.in.Iterator()
}

func (f *FilterTerms) Size() (int64, error) {
	return f.in.Size()
}

func (f *FilterTerms) GetSumTotalTermFreq() (int64, error) {
	return f.in.GetSumTotalTermFreq()
}

func (f *FilterTerms) GetSumDocFreq() (int64, error) {
	return f.in.GetSumDocFreq()
}

func (f *FilterTerms) GetDocCount() (int, error) {
	return f.in.GetDocCount()
}

func (f *FilterTerms) HasFreqs() bool {
	return f.in.HasFreqs()
}

func (f *FilterTerms) HasOffsets() bool {
	return f.in.HasOffsets()
}

func (f *FilterTerms) HasPositions() bool {
	return f.in.HasPositions()
}

func (f *FilterTerms) HasPayloads() bool {
	return f.in.HasPayloads()
}

func (f *FilterTerms) GetStats() (interface{}, error) {
	return f.in.GetStats()
}

// FilterTermsEnum is a base class for filtering TermsEnum implementations.
type FilterTermsEnum struct {
	in TermsEnum
}

func NewFilterTermsEnum(in TermsEnum) *FilterTermsEnum {
	if in == nil {
		panic("incoming TermsEnum must not be null")
	}
	return &FilterTermsEnum{in: in}
}

func (f *FilterTermsEnum) Attributes() util.AttributeSource {
	return f.in.Attributes()
}

func (f *FilterTermsEnum) SeekCeil(text *util.BytesRef) (SeekStatus, error) {
	return f.in.SeekCeil(text)
}

func (f *FilterTermsEnum) SeekExact(text *util.BytesRef) (bool, error) {
	return f.in.SeekExact(text)
}

func (f *FilterTermsEnum) SeekExactOrd(ord int64) error {
	return f.in.SeekExactOrd(ord)
}

func (f *FilterTermsEnum) Next() (*util.BytesRef, error) {
	return f.in.Next()
}

func (f *FilterTermsEnum) Term() (*util.BytesRef, error) {
	return f.in.Term()
}

func (f *FilterTermsEnum) Ord() (int64, error) {
	return f.in.Ord()
}

func (f *FilterTermsEnum) DocFreq() (int, error) {
	return f.in.DocFreq()
}

func (f *FilterTermsEnum) TotalTermFreq() (int64, error) {
	return f.in.TotalTermFreq()
}

func (f *FilterTermsEnum) Postings(reuse PostingsEnum, flags int) (PostingsEnum, error) {
	return f.in.Postings(reuse, flags)
}

func (f *FilterTermsEnum) Impacts(flags int) (ImpactsEnum, error) {
	return f.in.Impacts(flags)
}

func (f *FilterTermsEnum) SeekExactWithState(term *util.BytesRef, state *TermState) error {
	return f.in.SeekExactWithState(term, state)
}

func (f *FilterTermsEnum) PrepareSeekExact(text *util.BytesRef) (util.IOBooleanSupplier, error) {
	return f.in.PrepareSeekExact(text)
}

func (f *FilterTermsEnum) TermState() (*TermState, error) {
	return f.in.TermState()
}

// FilterPostingsEnum is a base class for filtering PostingsEnum implementations.
type FilterPostingsEnum struct {
	in PostingsEnum
}

func NewFilterPostingsEnum(in PostingsEnum) *FilterPostingsEnum {
	if in == nil {
		panic("incoming PostingsEnum must not be null")
	}
	return &FilterPostingsEnum{in: in}
}

func (f *FilterPostingsEnum) DocID() int {
	return f.in.DocID()
}

func (f *FilterPostingsEnum) Freq() (int, error) {
	return f.in.Freq()
}

func (f *FilterPostingsEnum) NextDoc() (int, error) {
	return f.in.NextDoc()
}

func (f *FilterPostingsEnum) Advance(target int) (int, error) {
	return f.in.Advance(target)
}

func (f *FilterPostingsEnum) NextPosition() (int, error) {
	return f.in.NextPosition()
}

func (f *FilterPostingsEnum) StartOffset() (int, error) {
	return f.in.StartOffset()
}

func (f *FilterPostingsEnum) EndOffset() (int, error) {
	return f.in.EndOffset()
}

func (f *FilterPostingsEnum) GetPayload() (*util.BytesRef, error) {
	return f.in.GetPayload()
}

func (f *FilterPostingsEnum) Cost() int64 {
	return f.in.Cost()
}

func (f *FilterPostingsEnum) Unwrap() PostingsEnum {
	return f.in
}
