// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecbridge

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// PostingsFormat
// ---------------------------------------------------------------------------

type postingsFormatAdapter struct {
	inner codecs.PostingsFormat
}

var _ index.PostingsFormat = (*postingsFormatAdapter)(nil)

func (a *postingsFormatAdapter) Name() string {
	return a.inner.Name()
}

func (a *postingsFormatAdapter) FieldsConsumer(state *index.SegmentWriteState) (index.FieldsConsumer, error) {
	cons, err := a.inner.FieldsConsumer(toCodecsWriteState(state))
	if err != nil {
		return nil, err
	}
	return &fieldsConsumerAdapter{inner: cons}, nil
}

func (a *postingsFormatAdapter) FieldsProducer(state *index.SegmentReadState) (index.FieldsProducer, error) {
	prod, err := a.inner.FieldsProducer(toCodecsReadState(state))
	if err != nil {
		return nil, err
	}
	return &fieldsProducerAdapter{inner: prod}, nil
}

type fieldsConsumerAdapter struct {
	inner codecs.FieldsConsumer
}

func (a *fieldsConsumerAdapter) Write(field string, terms index.Terms) error {
	// codecs.FieldsConsumer.Write also accepts index.Terms — both sides
	// share the same concrete Terms type, so no further adaptation is
	// required.
	return a.inner.Write(field, terms)
}

func (a *fieldsConsumerAdapter) Close() error {
	return a.inner.Close()
}

type fieldsProducerAdapter struct {
	inner codecs.FieldsProducer
}

func (a *fieldsProducerAdapter) Terms(field string) (index.Terms, error) {
	return a.inner.Terms(field)
}

func (a *fieldsProducerAdapter) Close() error {
	return a.inner.Close()
}

// ---------------------------------------------------------------------------
// StoredFieldsFormat
// ---------------------------------------------------------------------------

type storedFieldsFormatAdapter struct {
	inner codecs.StoredFieldsFormat
}

var _ index.StoredFieldsFormat = (*storedFieldsFormatAdapter)(nil)

func (a *storedFieldsFormatAdapter) Name() string {
	return a.inner.Name()
}

func (a *storedFieldsFormatAdapter) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (index.StoredFieldsReader, error) {
	r, err := a.inner.FieldsReader(dir, segmentInfo, fieldInfos, context)
	if err != nil {
		return nil, err
	}
	return &storedFieldsReaderAdapter{inner: r}, nil
}

func (a *storedFieldsFormatAdapter) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (index.StoredFieldsWriter, error) {
	w, err := a.inner.FieldsWriter(dir, segmentInfo, context)
	if err != nil {
		return nil, err
	}
	return &storedFieldsWriterAdapter{inner: w}, nil
}

type storedFieldsReaderAdapter struct {
	inner codecs.StoredFieldsReader
}

func (a *storedFieldsReaderAdapter) VisitDocument(docID int, visitor index.StoredFieldVisitor) error {
	return a.inner.VisitDocument(docID, &visitorAdapter{inner: visitor})
}

func (a *storedFieldsReaderAdapter) Close() error {
	return a.inner.Close()
}

// visitorAdapter exposes an index.StoredFieldVisitor as a codecs.StoredFieldVisitor.
// The two interfaces are structurally identical so dispatch is straight delegation.
type visitorAdapter struct {
	inner index.StoredFieldVisitor
}

func (v *visitorAdapter) StringField(field string, value string) { v.inner.StringField(field, value) }
func (v *visitorAdapter) BinaryField(field string, value []byte) { v.inner.BinaryField(field, value) }
func (v *visitorAdapter) IntField(field string, value int)       { v.inner.IntField(field, value) }
func (v *visitorAdapter) LongField(field string, value int64)    { v.inner.LongField(field, value) }
func (v *visitorAdapter) FloatField(field string, value float32) { v.inner.FloatField(field, value) }
func (v *visitorAdapter) DoubleField(field string, value float64) {
	v.inner.DoubleField(field, value)
}

type storedFieldsWriterAdapter struct {
	inner codecs.StoredFieldsWriter
}

func (a *storedFieldsWriterAdapter) StartDocument() error  { return a.inner.StartDocument() }
func (a *storedFieldsWriterAdapter) FinishDocument() error { return a.inner.FinishDocument() }

func (a *storedFieldsWriterAdapter) WriteField(field index.IndexableField) error {
	return a.inner.WriteField(adaptIndexableField(field))
}

func (a *storedFieldsWriterAdapter) Finish(numDocs int) error {
	// codecs.StoredFieldsWriter does not expose Finish in its current
	// minimum surface (the production Lucene104 implementation flushes
	// in Close). Map Finish to a no-op; the on-disk file is produced
	// when Close runs immediately afterwards.
	return nil
}

func (a *storedFieldsWriterAdapter) Close() error { return a.inner.Close() }

// ---------------------------------------------------------------------------
// FieldInfosFormat
// ---------------------------------------------------------------------------

type fieldInfosFormatAdapter struct {
	inner codecs.FieldInfosFormat
}

var _ index.FieldInfosFormat = (*fieldInfosFormatAdapter)(nil)

func (a *fieldInfosFormatAdapter) Name() string {
	return a.inner.Name()
}

// Read adapts the index-package signature (no segmentSuffix) to the codecs
// signature (carries a segmentSuffix string). Bridge callers do not yet
// split field infos by suffix; pass the empty suffix to match historical
// IndexWriter behaviour.
func (a *fieldInfosFormatAdapter) Read(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (*index.FieldInfos, error) {
	return a.inner.Read(dir, segmentInfo, "", context)
}

// Write adapts the index signature (no segmentSuffix) to the codecs one.
func (a *fieldInfosFormatAdapter) Write(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) error {
	return a.inner.Write(dir, segmentInfo, "", fieldInfos, context)
}

// ---------------------------------------------------------------------------
// SegmentInfosFormat
// ---------------------------------------------------------------------------

type segmentInfosFormatAdapter struct {
	inner codecs.SegmentInfosFormat
}

var _ index.SegmentInfosFormat = (*segmentInfosFormatAdapter)(nil)

func (a *segmentInfosFormatAdapter) Name() string {
	return a.inner.Name()
}

// Read drops the IOContext parameter because the codecs-side
// SegmentInfosFormat.Read does not accept one (the file is opened with
// IOContextRead internally).
func (a *segmentInfosFormatAdapter) Read(dir store.Directory, context store.IOContext) (*index.SegmentInfos, error) {
	return a.inner.Read(dir)
}

// Write drops the IOContext parameter similarly. The codecs-side writer
// opens segments_N with IOContextWrite internally.
func (a *segmentInfosFormatAdapter) Write(dir store.Directory, segmentInfos *index.SegmentInfos, context store.IOContext) error {
	return a.inner.Write(dir, segmentInfos)
}

// ---------------------------------------------------------------------------
// SegmentInfoFormat (.si)
// ---------------------------------------------------------------------------

type segmentInfoFormatAdapter struct {
	inner codecs.SegmentInfoFormat
}

var _ index.SegmentInfoFormat = (*segmentInfoFormatAdapter)(nil)

func (a *segmentInfoFormatAdapter) Write(dir store.Directory, info *index.SegmentInfo, context store.IOContext) error {
	return a.inner.Write(dir, info, context)
}

func (a *segmentInfoFormatAdapter) Read(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*index.SegmentInfo, error) {
	return a.inner.Read(dir, segmentName, segmentID, context)
}

// ---------------------------------------------------------------------------
// TermVectorsFormat
// ---------------------------------------------------------------------------

type termVectorsFormatAdapter struct {
	inner codecs.TermVectorsFormat
}

var _ index.TermVectorsFormat = (*termVectorsFormatAdapter)(nil)

func (a *termVectorsFormatAdapter) Name() string {
	return a.inner.Name()
}

func (a *termVectorsFormatAdapter) VectorsWriter(state *index.SegmentWriteState) (index.TermVectorsWriter, error) {
	w, err := a.inner.VectorsWriter(toCodecsWriteState(state))
	if err != nil {
		return nil, err
	}
	return &termVectorsWriterAdapter{inner: w}, nil
}

func (a *termVectorsFormatAdapter) VectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (index.TermVectorsReader, error) {
	r, err := a.inner.VectorsReader(dir, segmentInfo, fieldInfos, context)
	if err != nil {
		return nil, err
	}
	return &termVectorsReaderAdapter{inner: r}, nil
}

type termVectorsWriterAdapter struct {
	inner codecs.TermVectorsWriter
}

func (a *termVectorsWriterAdapter) StartDocument(numFields int) error {
	return a.inner.StartDocument(numFields)
}

func (a *termVectorsWriterAdapter) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	return a.inner.StartField(fieldInfo, numTerms, hasPositions, hasOffsets, hasPayloads)
}

func (a *termVectorsWriterAdapter) StartTerm(term []byte) error { return a.inner.StartTerm(term) }
func (a *termVectorsWriterAdapter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	return a.inner.AddPosition(position, startOffset, endOffset, payload)
}
func (a *termVectorsWriterAdapter) FinishTerm() error     { return a.inner.FinishTerm() }
func (a *termVectorsWriterAdapter) FinishField() error    { return a.inner.FinishField() }
func (a *termVectorsWriterAdapter) FinishDocument() error { return a.inner.FinishDocument() }
func (a *termVectorsWriterAdapter) Close() error          { return a.inner.Close() }

type termVectorsReaderAdapter struct {
	inner codecs.TermVectorsReader
}

func (a *termVectorsReaderAdapter) Get(docID int) (index.Fields, error) {
	return a.inner.Get(docID)
}

func (a *termVectorsReaderAdapter) GetField(docID int, field string) (index.Terms, error) {
	return a.inner.GetField(docID, field)
}

func (a *termVectorsReaderAdapter) Close() error { return a.inner.Close() }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// toCodecsWriteState copies the four fields of index.SegmentWriteState
// into codecs.SegmentWriteState. The shared *index.SegmentInfo and
// *index.FieldInfos pointers are reused as-is — both sides reference the
// same concrete types defined in index/.
func toCodecsWriteState(state *index.SegmentWriteState) *codecs.SegmentWriteState {
	if state == nil {
		return nil
	}
	return &codecs.SegmentWriteState{
		Directory:     state.Directory,
		SegmentInfo:   state.SegmentInfo,
		FieldInfos:    state.FieldInfos,
		SegmentSuffix: state.SegmentSuffix,
	}
}

func toCodecsReadState(state *index.SegmentReadState) *codecs.SegmentReadState {
	if state == nil {
		return nil
	}
	return &codecs.SegmentReadState{
		Directory:     state.Directory,
		SegmentInfo:   state.SegmentInfo,
		FieldInfos:    state.FieldInfos,
		SegmentSuffix: state.SegmentSuffix,
	}
}

// ---------------------------------------------------------------------------
// IndexableField adaptation
// ---------------------------------------------------------------------------

// adaptIndexableField wraps an index.IndexableField (the narrow,
// codec-facing interface defined in index/) so it satisfies the wider
// document.IndexableField interface that codecs.StoredFieldsWriter.WriteField
// consumes.
//
// Reconstruction of the FieldType is the only lossy step: the inbound
// FieldTypeInterface exposes only the properties the codec layer needs
// (Indexed / Stored / Tokenized, IndexOptions, DocValuesType, term-vector
// triplet). Document-only properties not present on FieldTypeInterface —
// OmitNorms, StoreTermVectorPayloads, dimension/vector metadata,
// DocValuesSkipIndex, attributes — are reset to the document.FieldType
// zero values. This is acceptable for the stored-fields write path because
// the production Lucene104StoredFieldsWriter (and Gocene's port) only
// reads name, FieldType().IsStored(), and the StringValue/BinaryValue/
// NumericValue accessors when serialising the .fdt/.fdx pair. None of the
// dropped properties influence the on-disk stored-fields representation.
//
// If a future stored-fields format needs OmitNorms or the dimensional
// metadata, the lossy fields here will need to be threaded into
// index.FieldTypeInterface first.
func adaptIndexableField(f index.IndexableField) document.IndexableField {
	return &indexableFieldAdapter{inner: f}
}

type indexableFieldAdapter struct {
	inner index.IndexableField
}

var _ document.IndexableField = (*indexableFieldAdapter)(nil)

func (a *indexableFieldAdapter) Name() string {
	return a.inner.Name()
}

func (a *indexableFieldAdapter) FieldType() *document.FieldType {
	src := a.inner.FieldType()
	ft := document.NewFieldType()
	if src == nil {
		return ft
	}
	ft.Indexed = src.IsIndexed()
	ft.Stored = src.IsStored()
	ft.Tokenized = src.IsTokenized()
	ft.IndexOptions = src.GetIndexOptions()
	ft.DocValuesType = src.GetDocValuesType()
	ft.StoreTermVectors = src.StoreTermVectors()
	ft.StoreTermVectorPositions = src.StoreTermVectorPositions()
	ft.StoreTermVectorOffsets = src.StoreTermVectorOffsets()
	// Properties NOT present on index.FieldTypeInterface are intentionally
	// left at the document.FieldType zero value (see adaptIndexableField
	// docstring for the rationale).
	return ft
}

func (a *indexableFieldAdapter) StringValue() string    { return a.inner.StringValue() }
func (a *indexableFieldAdapter) BinaryValue() []byte    { return a.inner.BinaryValue() }
func (a *indexableFieldAdapter) NumericValue() any      { return a.inner.NumericValue() }
func (a *indexableFieldAdapter) ReaderValue() io.Reader { return nil }
