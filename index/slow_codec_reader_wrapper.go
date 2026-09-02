// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
)

// SlowCodecReaderWrapper adapts an arbitrary LeafReader to the CodecReader
// surface by delegating codec-reader methods through the LeafReader's public
// API. Mirrors org.apache.lucene.index.SlowCodecReaderWrapper from Apache
// Lucene 10.4.0.
//
// # Deviation from Lucene 10.4.0
//
// Lucene's SlowCodecReaderWrapper.wrap() is a static factory that returns a
// CodecReader (the abstract base type). Go does not have abstract classes, and
// Gocene's CodecReader is a concrete struct backed by SegmentCoreReaders.
// Wrapping an arbitrary LeafReader therefore cannot produce a *CodecReader
// directly. WrapLeafReader returns a *SlowLeafCodecReader, which exposes the
// same per-field accessor surface (GetTermVectorsReader, GetStoredFieldsReader,
// GetPostingsReader, GetDocValuesReader, GetNormsReader, GetPointsReader,
// GetFieldInfos, GetLiveDocs) via adapters that delegate to the LeafReader API.
//
// Callers that require a *CodecReader (e.g. SlowCompositeCodecReaderWrapper)
// should use WrapSlowCompositeCodecReader instead, which accepts *CodecReader
// inputs directly.
type SlowCodecReaderWrapper struct{}

// WrapLeafReader adapts reader to the codec reader surface. If reader is
// already a *CodecReader it is returned as-is (zero overhead). Otherwise a
// SlowLeafCodecReader adapter is returned.
//
// Mirrors SlowCodecReaderWrapper.wrap(LeafReader) in Lucene 10.4.0, with the
// return type changed from CodecReader to *SlowLeafCodecReader to remain
// idiomatic in Go (CodecReader is a concrete struct, not an interface).
func WrapLeafReader(reader *LeafReader) (*SlowLeafCodecReader, error) {
	if reader == nil {
		return nil, errors.New("SlowCodecReaderWrapper: reader must not be nil")
	}
	if err := reader.CheckIntegrity(); err != nil {
		return nil, fmt.Errorf("SlowCodecReaderWrapper: checkIntegrity: %w", err)
	}
	return &SlowLeafCodecReader{delegate: reader}, nil
}

// SlowLeafCodecReader wraps a *LeafReader and exposes the per-field codec
// reader surface by delegating through the LeafReader's public API. It is
// intentionally slow: every Get call fans out to the leaf, which may be an
// in-memory reader without pre-paged data. Use FilterCodecReader or a real
// SegmentReader where performance matters.
//
// This type mirrors the anonymous CodecReader subclass constructed inside
// SlowCodecReaderWrapper.wrap() in Apache Lucene 10.4.0.
type SlowLeafCodecReader struct {
	delegate *LeafReader
}

// GetDelegate returns the underlying LeafReader.
func (s *SlowLeafCodecReader) GetDelegate() *LeafReader { return s.delegate }

// GetFieldInfos returns the FieldInfos from the delegate.
func (s *SlowLeafCodecReader) GetFieldInfos() *FieldInfos {
	return s.delegate.IndexReader.GetFieldInfos()
}

// GetLiveDocs returns the live docs from the delegate. Returns nil when all
// documents are live (no deletions).
func (s *SlowLeafCodecReader) GetLiveDocs() interface{} {
	// LeafReader does not expose a typed live-docs accessor at this layer;
	// return nil (no deletions visible through the slow wrapper).
	return nil
}

// NumDocs returns the number of live documents.
func (s *SlowLeafCodecReader) NumDocs() int { return s.delegate.NumDocs() }

// MaxDoc returns the maximum document ID plus one.
func (s *SlowLeafCodecReader) MaxDoc() int { return s.delegate.MaxDoc() }

// CheckIntegrity verifies the underlying reader's integrity. Mirrors Lucene's
// delegating checkIntegrity call inside the anonymous CodecReader.
func (s *SlowLeafCodecReader) CheckIntegrity() error { return s.delegate.CheckIntegrity() }

// GetTermVectorsReader returns a TermVectorsReader that delegates to
// delegate.TermVectors(). Mirrors readerToTermVectorsReader() in Lucene.
func (s *SlowLeafCodecReader) GetTermVectorsReader() TermVectorsReader {
	tv, err := s.delegate.TermVectors()
	if err != nil || tv == nil {
		return nil
	}
	return &slowTermVectorsReader{tv: tv, delegate: s.delegate}
}

// GetStoredFieldsReader returns a StoredFieldsReader backed by the delegate's
// stored fields. Mirrors readerToStoredFieldsReader() in Lucene.
func (s *SlowLeafCodecReader) GetStoredFieldsReader() StoredFieldsReader {
	sf, err := s.delegate.StoredFields()
	if err != nil || sf == nil {
		return nil
	}
	return &slowStoredFieldsReader{sf: sf, delegate: s.delegate}
}

// GetPostingsReader returns a FieldsProducer backed by the delegate's terms.
// Mirrors readerToFieldsProducer() in Lucene.
func (s *SlowLeafCodecReader) GetPostingsReader() (FieldsProducer, error) {
	fi := s.GetFieldInfos()
	if fi == nil {
		return &slowFieldsProducer{delegate: s.delegate, fields: nil}, nil
	}
	var indexedFields []string
	it := fi.Iterator()
	for {
		info := it.Next()
		if info == nil {
			break
		}
		if info.IndexOptions() != IndexOptionsNone {
			indexedFields = append(indexedFields, info.Name())
		}
	}
	sort.Strings(indexedFields)
	return &slowFieldsProducer{delegate: s.delegate, fields: indexedFields}, nil
}

// GetDocValuesReader returns a DocValuesProducer backed by the delegate.
// Mirrors readerToDocValuesProducer() in Lucene.
func (s *SlowLeafCodecReader) GetDocValuesReader() DocValuesProducer {
	return &slowDocValuesProducer{delegate: s.delegate}
}

// GetNormsReader returns a NormsProducer-compatible adapter backed by the
// delegate's getNormValues. Returns an opaque interface{} to avoid a cross-
// package import of codecs.NormsProducer (which Gocene's CodecReader also
// returns as interface{}).
func (s *SlowLeafCodecReader) GetNormsReader() interface{} {
	return &slowNormsProducer{delegate: s.delegate}
}

// GetPointsReader returns a PointsReader-compatible adapter backed by the
// delegate's getPointValues. Returns interface{} for the same reason as
// GetNormsReader.
func (s *SlowLeafCodecReader) GetPointsReader() interface{} {
	return &slowPointsReader{delegate: s.delegate}
}

// --- inner adapter types ---

// slowTermVectorsReader delegates TermVectorsReader to the LeafReader's
// TermVectors surface. Mirrors readerToTermVectorsReader() in Lucene.
type slowTermVectorsReader struct {
	tv       TermVectors
	delegate *LeafReader
}

func (r *slowTermVectorsReader) Get(docID int) (Fields, error) {
	return r.tv.Get(docID)
}

func (r *slowTermVectorsReader) GetField(docID int, field string) (Terms, error) {
	return r.tv.GetField(docID, field)
}

func (r *slowTermVectorsReader) Close() error { return nil }

// slowStoredFieldsReader delegates StoredFieldsReader to the LeafReader's
// StoredFields surface. Mirrors readerToStoredFieldsReader() in Lucene.
type slowStoredFieldsReader struct {
	sf       StoredFields
	delegate *LeafReader
}

func (r *slowStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	return r.sf.Document(docID, visitor)
}

func (r *slowStoredFieldsReader) Close() error { return nil }

// slowFieldsProducer delegates FieldsProducer to the LeafReader's Terms
// accessors. Mirrors readerToFieldsProducer() in Lucene.
type slowFieldsProducer struct {
	delegate *LeafReader
	fields   []string
}

func (p *slowFieldsProducer) Terms(field string) (Terms, error) {
	return p.delegate.Terms(field)
}

func (p *slowFieldsProducer) Close() error { return nil }

// slowDocValuesProducer delegates DocValuesProducer to the LeafReader's
// GetXxxDocValues accessors. Mirrors readerToDocValuesProducer() in Lucene.
type slowDocValuesProducer struct {
	delegate *LeafReader
}

func (p *slowDocValuesProducer) GetNumeric(field *FieldInfo) (NumericDocValues, error) {
	return p.delegate.GetNumericDocValues(field.Name())
}

func (p *slowDocValuesProducer) GetBinary(field *FieldInfo) (BinaryDocValues, error) {
	return p.delegate.GetBinaryDocValues(field.Name())
}

func (p *slowDocValuesProducer) GetSorted(field *FieldInfo) (SortedDocValues, error) {
	return p.delegate.GetSortedDocValues(field.Name())
}

func (p *slowDocValuesProducer) GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error) {
	return p.delegate.GetSortedNumericDocValues(field.Name())
}

func (p *slowDocValuesProducer) GetSortedSet(field *FieldInfo) (SortedSetDocValues, error) {
	return p.delegate.GetSortedSetDocValues(field.Name())
}

func (p *slowDocValuesProducer) GetSkipper(field *FieldInfo) (DocValuesSkipper, error) {
	return p.delegate.GetDocValuesSkipper(field.Name())
}

func (p *slowDocValuesProducer) CheckIntegrity() error { return nil }

func (p *slowDocValuesProducer) Close() error { return nil }

// slowNormsProducer delegates norm lookups to the LeafReader's GetNormValues.
// Returned as interface{} from SlowLeafCodecReader.GetNormsReader to avoid
// importing codecs.NormsProducer.
type slowNormsProducer struct {
	delegate *LeafReader
}

// GetNorms returns the NumericDocValues for the given FieldInfo's norms.
func (p *slowNormsProducer) GetNorms(field *FieldInfo) (NumericDocValues, error) {
	return p.delegate.GetNormValues(field.Name())
}

// CheckIntegrity is a no-op (the slow wrapper already called CheckIntegrity on
// construction, matching Lucene's comment).
func (p *slowNormsProducer) CheckIntegrity() error { return nil }

// Close is a no-op.
func (p *slowNormsProducer) Close() error { return nil }

// slowPointsReader delegates point-value lookups to the LeafReader's
// GetPointValues. Returned as interface{} from SlowLeafCodecReader.GetPointsReader
// to avoid importing codecs.PointsReader.
type slowPointsReader struct {
	delegate *LeafReader
}

// GetValues returns the PointValues for the given field.
func (p *slowPointsReader) GetValues(field string) (PointValues, error) {
	return p.delegate.GetPointValues(field)
}

// CheckIntegrity is a no-op.
func (p *slowPointsReader) CheckIntegrity() error { return nil }

// Close is a no-op.
func (p *slowPointsReader) Close() error { return nil }

// Compile-time assertions for the adapter types.
var _ TermVectorsReader = (*slowTermVectorsReader)(nil)
var _ StoredFieldsReader = (*slowStoredFieldsReader)(nil)
var _ FieldsProducer = (*slowFieldsProducer)(nil)
var _ DocValuesProducer = (*slowDocValuesProducer)(nil)
