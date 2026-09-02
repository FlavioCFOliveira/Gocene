// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesLeafReader is a LeafReader that exposes only doc values, leaving
// every other reader capability unsupported. It is the base for readers
// produced by doc-values-only update flows, where postings, term vectors,
// points, vectors, stored fields, norms and live docs have no meaning.
//
// This is the Go port of the package-private
// org.apache.lucene.index.DocValuesLeafReader from Apache Lucene 10.4.0.
//
// In Lucene every method below throws UnsupportedOperationException; the doc
// values accessors (GetNumericDocValues, GetBinaryDocValues, ...) stay
// abstract for concrete subclasses to implement. Gocene embeds *LeafReader,
// so the doc values accessors inherit the base no-op behaviour and concrete
// subclasses override them; the methods shadowed here panic with
// errDocValuesLeafReaderUnsupported, preserving Lucene's unconditional throw.
//
// Naming divergence: Lucene overrides getCoreCacheHelper and
// getReaderCacheHelper. Gocene's *LeafReader does not expose those; it exposes
// GetCoreCacheKey, which is the method shadowed here instead.
type DocValuesLeafReader struct {
	*LeafReader
}

// errDocValuesLeafReaderUnsupported is the sentinel used by every shadowed
// method, mirroring Lucene's UnsupportedOperationException.
var errDocValuesLeafReaderUnsupported = fmt.Errorf("operation not supported on DocValuesLeafReader")

// NewDocValuesLeafReader returns a DocValuesLeafReader for the given segment.
func NewDocValuesLeafReader(segmentInfo *SegmentInfo) *DocValuesLeafReader {
	return &DocValuesLeafReader{LeafReader: NewLeafReader(segmentInfo)}
}

// GetCoreCacheKey is unsupported. It panics, mirroring Lucene's
// getCoreCacheHelper throwing UnsupportedOperationException.
func (r *DocValuesLeafReader) GetCoreCacheKey() interface{} {
	panic(errDocValuesLeafReaderUnsupported)
}

// Terms is unsupported.
func (r *DocValuesLeafReader) Terms(string) (Terms, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetNormValues is unsupported.
func (r *DocValuesLeafReader) GetNormValues(string) (NumericDocValues, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetLiveDocs is unsupported. It panics, mirroring Lucene's getLiveDocs
// throwing UnsupportedOperationException.
func (r *DocValuesLeafReader) GetLiveDocs() util.Bits {
	panic(errDocValuesLeafReaderUnsupported)
}

// GetPointValues is unsupported.
func (r *DocValuesLeafReader) GetPointValues(string) (PointValues, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetFloatVectorValues is unsupported.
func (r *DocValuesLeafReader) GetFloatVectorValues(string) (FloatVectorValues, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetByteVectorValues is unsupported.
func (r *DocValuesLeafReader) GetByteVectorValues(string) (ByteVectorValues, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// SearchNearestVectors is unsupported.
func (r *DocValuesLeafReader) SearchNearestVectors(string, []float32, int, util.Bits) (TopDocs, error) {
	return TopDocs{}, errDocValuesLeafReaderUnsupported
}

// CheckIntegrity is unsupported.
func (r *DocValuesLeafReader) CheckIntegrity() error {
	return errDocValuesLeafReaderUnsupported
}

// GetMetaData is unsupported. It panics, mirroring Lucene's getMetaData
// throwing UnsupportedOperationException.
func (r *DocValuesLeafReader) GetMetaData() *IndexReaderMetaData {
	panic(errDocValuesLeafReaderUnsupported)
}

// TermVectors is unsupported.
func (r *DocValuesLeafReader) TermVectors() (TermVectors, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetTermVectors is unsupported.
func (r *DocValuesLeafReader) GetTermVectors(int) (Fields, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// NumDocs is unsupported. It panics, mirroring Lucene's numDocs throwing
// UnsupportedOperationException.
func (r *DocValuesLeafReader) NumDocs() int {
	panic(errDocValuesLeafReaderUnsupported)
}

// MaxDoc is unsupported. It panics, mirroring Lucene's maxDoc throwing
// UnsupportedOperationException.
func (r *DocValuesLeafReader) MaxDoc() int {
	panic(errDocValuesLeafReaderUnsupported)
}

// StoredFields is unsupported.
func (r *DocValuesLeafReader) StoredFields() (StoredFields, error) {
	return nil, errDocValuesLeafReaderUnsupported
}

// GetDocValuesSkipper is unsupported.
func (r *DocValuesLeafReader) GetDocValuesSkipper(string) (DocValuesSkipper, error) {
	return nil, errDocValuesLeafReaderUnsupported
}
