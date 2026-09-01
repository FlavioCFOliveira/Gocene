// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterLeafReader wraps another LeafReader and delegates most calls to it.
// Mirrors org.apache.lucene.index.FilterLeafReader from Apache Lucene 10.5.0.
type FilterLeafReader struct {
	in LeafReader
}

// NewFilterLeafReader constructs a FilterLeafReader.
func NewFilterLeafReader(in LeafReader) *FilterLeafReader {
	if in == nil {
		panic("incoming LeafReader must not be null")
	}
	return &FilterLeafReader{in: in}
}

// Unwrap returns the wrapped LeafReader, recursively unwrapping any FilterLeafReaders.
func Unwrap(reader LeafReader) LeafReader {
	for {
		if flr, ok := reader.(*FilterLeafReader); ok {
			reader = flr.GetDelegate()
		} else {
			break
		}
	}
	return reader
}

func (f *FilterLeafReader) GetDelegate() LeafReader {
	return f.in
}

// Delegate methods

func (f *FilterLeafReader) DocID() int                                  { return f.in.DocID() }
func (f *FilterLeafReader) MaxDoc() int                                { return f.in.MaxDoc() }
func (f *FilterLeafReader) NumDocs() int                                  { return f.in.NumDocs() }
func (f *FilterLeafReader) DocFreq(term Term) (int, error)             { return f.in.DocFreq(term) }
func (f *FilterLeafReader) TotalTermFreq(term Term) (int64, error)      { return f.in.TotalTermFreq(term) }
func (f *FilterLeafReader) Terms(field string) (Terms, error)           { return f.in.Terms(field) }
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
func (f *FilterLeafReader) GetNormValues(field string) (NumericDocValues, error) {
	return f.in.GetNormValues(field)
}
func (f *FilterLeafReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	return f.in.GetDocValuesSkipper(field)
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
func (f *FilterLeafReader) GetFieldInfos() *FieldInfos { return f.in.GetFieldInfos() }
func (f *FilterLeafReader) GetLiveDocs() util.Bits     { return f.in.GetLiveDocs() }
func (f *FilterLeafReader) GetPointValues(field string) (PointValues, error) {
	return f.in.GetPointValues(field)
}
func (f *FilterLeafReader) CheckIntegrity() error { return f.in.CheckIntegrity() }
func (f *FilterLeafReader) GetMetaData() *IndexReaderMetaData {
	return f.in.GetMetaData()
}
func (f *FilterLeafReader) GetContext() (IndexReaderContext, error) {
	return f.in.GetContext()
}
func (f *FilterLeafReader) Close() error { return f.in.Close() }
func (f *FilterLeafReader) IncRef() error { return f.in.IncRef() }
func (f *FilterLeafReader) DecRef() error { return f.in.DecRef() }
func (f *FilterLeafReader) TryIncRef() bool { return f.in.TryIncRef() }
func (f *FilterLeafReader) GetRefCount() int32 { return f.in.GetRefCount() }

func (f *FilterLeafReader) GetCoreCacheHelper() CacheHelper { return f.in.GetCoreCacheHelper() }
func (f *FilterLeafReader) GetReaderCacheHelper() CacheHelper { return f.in.GetReaderCacheHelper() }

func (f *FilterLeafReader) String() string {
	return fmt.Sprintf("FilterLeafReader(%v)", f.in)
}
