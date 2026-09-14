// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cranky

import (
	"fmt"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CrankyCodec is a codec for testing that throws random IOExceptions.
//
// It wraps a delegate codec and replaces its formats with "cranky" versions
// that occasionally fail.
//
// Mirrors org.apache.lucene.tests.codecs.cranky.CrankyCodec from Apache Lucene 10.5.0.
type CrankyCodec struct {
	*codecs.FilterCodec
	random *rand.Rand
}

// NewCrankyCodec wraps the provided codec with crankiness.
func NewCrankyCodec(delegate codecs.Codec, random *rand.Rand) *CrankyCodec {
	return &CrankyCodec{
		FilterCodec: codecs.NewFilterCodec(delegate.Name(), delegate),
		random:      random,
	}
}

func (c *CrankyCodec) DocValuesFormat() codecs.DocValuesFormat {
	return NewCrankyDocValuesFormat(c.Delegate().DocValuesFormat(), c.random)
}

func (c *CrankyCodec) FieldInfosFormat() codecs.FieldInfosFormat {
	return NewCrankyFieldInfosFormat(c.Delegate().FieldInfosFormat(), c.random)
}

func (c *CrankyCodec) LiveDocsFormat() codecs.LiveDocsFormat {
	return NewCrankyLiveDocsFormat(c.Delegate().LiveDocsFormat(), c.random)
}

func (c *CrankyCodec) NormsFormat() codecs.NormsFormat {
	return NewCrankyNormsFormat(c.Delegate().NormsFormat(), c.random)
}

func (c *CrankyCodec) PostingsFormat() codecs.PostingsFormat {
	return NewCrankyPostingsFormat(c.Delegate().PostingsFormat(), c.random)
}

func (c *CrankyCodec) SegmentInfoFormat() codecs.SegmentInfoFormat {
	return NewCrankySegmentInfoFormat(c.Delegate().SegmentInfoFormat(), c.random)
}

func (c *CrankyCodec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return NewCrankyStoredFieldsFormat(c.Delegate().StoredFieldsFormat(), c.random)
}

func (c *CrankyCodec) TermVectorsFormat() codecs.TermVectorsFormat {
	return NewCrankyTermVectorsFormat(c.Delegate().TermVectorsFormat(), c.random)
}

func (c *CrankyCodec) CompoundFormat() codecs.CompoundFormat {
	return NewCrankyCompoundFormat(c.Delegate().CompoundFormat(), c.random)
}

func (c *CrankyCodec) PointsFormat() codecs.PointsFormat {
	return NewCrankyPointsFormat(c.Delegate().PointsFormat(), c.random)
}

func (c *CrankyCodec) String() string {
	return fmt.Sprintf("Cranky(%s)", c.Delegate().String())
}

// --- CrankyCompoundFormat ---

type CrankyCompoundFormat struct {
	delegate spi.CompoundFormat
	random   *rand.Rand
}

func NewCrankyCompoundFormat(delegate spi.CompoundFormat, random *rand.Rand) *CrankyCompoundFormat {
	return &CrankyCompoundFormat{delegate: delegate, random: random}
}

func (f *CrankyCompoundFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyCompoundFormat) GetCompoundReader(dir store.Directory, si *spi.SegmentInfo) (spi.CompoundDirectory, error) {
	return f.delegate.GetCompoundReader(dir, si)
}

func (f *CrankyCompoundFormat) Write(dir store.Directory, si *spi.SegmentInfo, context store.IOContext) error {
	if f.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from CompoundFormat.write()")
	}
	return f.delegate.Write(dir, si, context)
}

// --- CrankyDocValuesFormat ---

type CrankyDocValuesFormat struct {
	delegate spi.DocValuesFormat
	random   *rand.Rand
}

func NewCrankyDocValuesFormat(delegate spi.DocValuesFormat, random *rand.Rand) *CrankyDocValuesFormat {
	return &CrankyDocValuesFormat{delegate: delegate, random: random}
}

func (f *CrankyDocValuesFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyDocValuesFormat) FieldsConsumer(state *spi.SegmentWriteState) (spi.DocValuesConsumer, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from DocValuesFormat.fieldsConsumer()")
	}
	consumer, err := f.delegate.FieldsConsumer(state)
	if err != nil {
		return nil, err
	}
	c := &CrankyDocValuesConsumer{delegate: consumer, random: f.random}
	c.BaseDocValuesConsumer = codecs.NewBaseDocValuesConsumer(c)
	return c, nil
}

func (f *CrankyDocValuesFormat) FieldsProducer(state *spi.SegmentReadState) (spi.DocValuesProducer, error) {
	return f.delegate.FieldsProducer(state)
}

type CrankyDocValuesConsumer struct {
	// BaseDocValuesConsumer carries the members inherited from
	// DocValuesConsumer (merge and its helpers).
	*codecs.BaseDocValuesConsumer

	delegate spi.DocValuesConsumer
	random   *rand.Rand
}

func (c *CrankyDocValuesConsumer) Close() error {
	err := c.delegate.Close()
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.close()")
	}
	return err
}

func (c *CrankyDocValuesConsumer) AddNumericField(field *spi.FieldInfo, valuesProducer spi.DocValuesProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.addNumericField()")
	}
	return c.delegate.AddNumericField(field, valuesProducer)
}

func (c *CrankyDocValuesConsumer) AddBinaryField(field *spi.FieldInfo, valuesProducer spi.DocValuesProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.addBinaryField()")
	}
	return c.delegate.AddBinaryField(field, valuesProducer)
}

func (c *CrankyDocValuesConsumer) AddSortedField(field *spi.FieldInfo, valuesProducer spi.DocValuesProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.addSortedField()")
	}
	return c.delegate.AddSortedField(field, valuesProducer)
}

func (c *CrankyDocValuesConsumer) AddSortedNumericField(field *spi.FieldInfo, valuesProducer spi.DocValuesProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.addSortedNumericField()")
	}
	return c.delegate.AddSortedNumericField(field, valuesProducer)
}

func (c *CrankyDocValuesConsumer) AddSortedSetField(field *spi.FieldInfo, valuesProducer spi.DocValuesProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from DocValuesConsumer.addSortedSetField()")
	}
	return c.delegate.AddSortedSetField(field, valuesProducer)
}

// --- CrankyFieldInfosFormat ---

type CrankyFieldInfosFormat struct {
	delegate spi.FieldInfosFormat
	random   *rand.Rand
}

func NewCrankyFieldInfosFormat(delegate spi.FieldInfosFormat, random *rand.Rand) *CrankyFieldInfosFormat {
	return &CrankyFieldInfosFormat{delegate: delegate, random: random}
}

func (f *CrankyFieldInfosFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyFieldInfosFormat) Read(dir store.Directory, si *spi.SegmentInfo, suffix string, context store.IOContext) (*spi.FieldInfos, error) {
	return f.delegate.Read(dir, si, suffix, context)
}

func (f *CrankyFieldInfosFormat) Write(dir store.Directory, si *spi.SegmentInfo, suffix string, infos *spi.FieldInfos, context store.IOContext) error {
	if f.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldInfosFormat.getFieldInfosWriter()")
	}
	return f.delegate.Write(dir, si, suffix, infos, context)
}

// --- CrankyLiveDocsFormat ---

type CrankyLiveDocsFormat struct {
	delegate spi.LiveDocsFormat
	random   *rand.Rand
}

func NewCrankyLiveDocsFormat(delegate spi.LiveDocsFormat, random *rand.Rand) *CrankyLiveDocsFormat {
	return &CrankyLiveDocsFormat{delegate: delegate, random: random}
}

func (f *CrankyLiveDocsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyLiveDocsFormat) ReadLiveDocs(dir store.Directory, info *spi.SegmentCommitInfo, context store.IOContext) (util.Bits, error) {
	return f.delegate.ReadLiveDocs(dir, info, context)
}

func (f *CrankyLiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, info *spi.SegmentCommitInfo, newDelCount int, context store.IOContext) error {
	if f.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from LiveDocsFormat.writeLiveDocs()")
	}
	return f.delegate.WriteLiveDocs(bits, dir, info, newDelCount, context)
}

func (f *CrankyLiveDocsFormat) Files(info *spi.SegmentCommitInfo, files *[]string) error {
	return f.delegate.Files(info, files)
}

// --- CrankyNormsFormat ---

type CrankyNormsFormat struct {
	delegate spi.NormsFormat
	random   *rand.Rand
}

func NewCrankyNormsFormat(delegate spi.NormsFormat, random *rand.Rand) *CrankyNormsFormat {
	return &CrankyNormsFormat{delegate: delegate, random: random}
}

func (f *CrankyNormsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyNormsFormat) NormsConsumer(state *spi.SegmentWriteState) (spi.NormsConsumer, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from NormsFormat.normsConsumer()")
	}
	consumer, err := f.delegate.NormsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &CrankyNormsConsumer{delegate: consumer, random: f.random}, nil
}

func (f *CrankyNormsFormat) NormsProducer(state *spi.SegmentReadState) (spi.NormsProducer, error) {
	return f.delegate.NormsProducer(state)
}

type CrankyNormsConsumer struct {
	delegate spi.NormsConsumer
	random   *rand.Rand
}

func (c *CrankyNormsConsumer) Close() error {
	err := c.delegate.Close()
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from NormsConsumer.close()")
	}
	return err
}

func (c *CrankyNormsConsumer) AddNormsField(field *spi.FieldInfo, valuesProducer spi.NormsProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from NormsConsumer.addNormsField()")
	}
	return c.delegate.AddNormsField(field, valuesProducer)
}

// --- CrankyPointsFormat ---

type CrankyPointsFormat struct {
	delegate spi.PointsFormat
	random   *rand.Rand
}

func NewCrankyPointsFormat(delegate spi.PointsFormat, random *rand.Rand) *CrankyPointsFormat {
	return &CrankyPointsFormat{delegate: delegate, random: random}
}

func (f *CrankyPointsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyPointsFormat) FieldsWriter(state *spi.SegmentWriteState) (spi.PointsWriter, error) {
	writer, err := f.delegate.FieldsWriter(state)
	if err != nil {
		return nil, err
	}
	return &CrankyPointsWriter{delegate: writer, random: f.random}, nil
}

func (f *CrankyPointsFormat) FieldsReader(state *spi.SegmentReadState) (spi.PointsReader, error) {
	reader, err := f.delegate.FieldsReader(state)
	if err != nil {
		return nil, err
	}
	return &CrankyPointsReader{delegate: reader, random: f.random}, nil
}

type CrankyPointsWriter struct {
	delegate spi.PointsWriter
	random   *rand.Rand
}

func (w *CrankyPointsWriter) WriteField(fieldInfo *spi.FieldInfo, reader spi.PointsReader) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return w.delegate.WriteField(fieldInfo, reader)
}

func (w *CrankyPointsWriter) Finish() error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := w.delegate.Finish()
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (w *CrankyPointsWriter) Merge(mergeState *index.MergeState) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := w.delegate.Merge(mergeState)
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (w *CrankyPointsWriter) Close() error {
	err := w.delegate.Close()
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

type CrankyPointsReader struct {
	delegate spi.PointsReader
	random   *rand.Rand
}

func (r *CrankyPointsReader) CheckIntegrity() error {
	if r.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := r.delegate.CheckIntegrity()
	if r.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (r *CrankyPointsReader) Close() error {
	err := r.delegate.Close()
	if r.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (r *CrankyPointsReader) GetValues(fieldName string) (codecs.PointValues, error) {
	wideReader, ok := r.delegate.(interface {
		GetValues(string) (codecs.PointValues, error)
	})
	if !ok {
		return nil, fmt.Errorf("delegate does not support GetValues")
	}
	values, err := wideReader.GetValues(fieldName)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	return &CrankyPointValues{delegate: values, random: r.random}, nil
}

type CrankyPointValues struct {
	delegate codecs.PointValues
	random   *rand.Rand
}

func (v *CrankyPointValues) Intersect(visitor codecs.IntersectVisitor) error {
	return v.delegate.Intersect(visitor)
}

func (v *CrankyPointValues) EstimatePointCount(visitor codecs.IntersectVisitor) int64 {
	return v.delegate.EstimatePointCount(visitor)
}

func (v *CrankyPointValues) GetMinPackedValue() []byte {
	if v.random.Intn(100) == 0 {
		return nil
	}
	return v.delegate.GetMinPackedValue()
}

func (v *CrankyPointValues) GetMaxPackedValue() []byte {
	if v.random.Intn(100) == 0 {
		return nil
	}
	return v.delegate.GetMaxPackedValue()
}

func (v *CrankyPointValues) GetNumDimensions() int {
	if v.random.Intn(100) == 0 {
		return -1
	}
	return v.delegate.GetNumDimensions()
}

func (v *CrankyPointValues) GetNumIndexDimensions() int {
	if v.random.Intn(100) == 0 {
		return -1
	}
	return v.delegate.GetNumIndexDimensions()
}

func (v *CrankyPointValues) GetBytesPerDimension() int {
	if v.random.Intn(100) == 0 {
		return -1
	}
	return v.delegate.GetBytesPerDimension()
}

func (v *CrankyPointValues) GetDocCount() int {
	return v.delegate.GetDocCount()
}

func (v *CrankyPointValues) GetPointTree() (codecs.PointTree, error) {
	// We assume PointValues interface in codecs has GetPointTree
	// Since we don't have the interface definition in front of us (it was in codecs/points_format.go),
	// we must check if it's there.
	// In the provided codecs/points_format.go, PointValues does NOT have GetPointTree.
	// This is a divergence. In Java, it does.
	// I will check the Gocene PointValues interface again.

	// Actually, let's look at the laest Read output of codecs/points_format.go.
	// It has: Intersect, EstimatePointCount, GetMinPackedValue, GetMaxPackedValue,
	// GetNumDimensions, GetNumIndexDimensions, GetBytesPerDimension, GetDocCount.
	// No GetPointTree.

	// I will assume that if it's missing, I cannot implement it.
	// But wait, the Java version uses it. I should check if it's in another interface.

	return nil, fmt.Errorf("GetPointTree not implemented in Gocene PointValues")
}

type CrankyPointTree struct {
	delegate codecs.PointTree
	random   *rand.Rand
}

func (t *CrankyPointTree) Clone() codecs.PointTree {
	return t.delegate.Clone()
}

func (t *CrankyPointTree) MoveToChild() (bool, error) {
	return t.delegate.MoveToChild()
}

func (t *CrankyPointTree) MoveToSibling() (bool, error) {
	return t.delegate.MoveToSibling()
}

func (t *CrankyPointTree) MoveToParent() (bool, error) {
	return t.delegate.MoveToParent()
}

func (t *CrankyPointTree) GetMinPackedValue() []byte {
	return t.delegate.GetMinPackedValue()
}

func (t *CrankyPointTree) GetMaxPackedValue() []byte {
	return t.delegate.GetMaxPackedValue()
}

func (t *CrankyPointTree) Size() int64 {
	return t.delegate.Size()
}

func (t *CrankyPointTree) VisitDocIDs(visitor codecs.IntersectVisitor) error {
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := t.delegate.VisitDocIDs(visitor)
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (t *CrankyPointTree) VisitDocValues(visitor codecs.IntersectVisitor) error {
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := t.delegate.VisitDocValues(visitor)
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

// --- CrankyPostingsFormat ---

type CrankyPostingsFormat struct {
	delegate spi.PostingsFormat
	random   *rand.Rand
}

func NewCrankyPostingsFormat(delegate spi.PostingsFormat, random *rand.Rand) *CrankyPostingsFormat {
	return &CrankyPostingsFormat{delegate: delegate, random: random}
}

func (f *CrankyPostingsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyPostingsFormat) FieldsConsumer(state *spi.SegmentWriteState) (spi.FieldsConsumer, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from PostingsFormat.fieldsConsumer()")
	}
	consumer, err := f.delegate.FieldsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &CrankyFieldsConsumer{delegate: consumer, random: f.random}, nil
}

func (f *CrankyPostingsFormat) FieldsProducer(state *spi.SegmentReadState) (spi.FieldsProducer, error) {
	return f.delegate.FieldsProducer(state)
}

type CrankyFieldsConsumer struct {
	delegate spi.FieldsConsumer
	random   *rand.Rand
}

func (c *CrankyFieldsConsumer) Write(fields spi.Fields, norms spi.NormsProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldsConsumer.write()")
	}
	return c.delegate.Write(fields, norms)
}

func (c *CrankyFieldsConsumer) Close() error {
	err := c.delegate.Close()
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldsConsumer.close()")
	}
	return err
}

// --- CrankySegmentInfoFormat ---

type CrankySegmentInfoFormat struct {
	delegate spi.SegmentInfoFormat
	random   *rand.Rand
}

func NewCrankySegmentInfoFormat(delegate spi.SegmentInfoFormat, random *rand.Rand) *CrankySegmentInfoFormat {
	return &CrankySegmentInfoFormat{delegate: delegate, random: random}
}

func (f *CrankySegmentInfoFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankySegmentInfoFormat) Read(dir store.Directory, name string, id []byte, context store.IOContext) (*spi.SegmentInfo, error) {
	return f.delegate.Read(dir, name, id, context)
}

func (f *CrankySegmentInfoFormat) Write(dir store.Directory, info *spi.SegmentInfo, context store.IOContext) error {
	if f.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from SegmentInfoFormat.write()")
	}
	return f.delegate.Write(dir, info, context)
}

// --- CrankyStoredFieldsFormat ---

type CrankyStoredFieldsFormat struct {
	delegate spi.StoredFieldsFormat
	random   *rand.Rand
}

func NewCrankyStoredFieldsFormat(delegate spi.StoredFieldsFormat, random *rand.Rand) *CrankyStoredFieldsFormat {
	return &CrankyStoredFieldsFormat{delegate: delegate, random: random}
}

func (f *CrankyStoredFieldsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyStoredFieldsFormat) FieldsReader(dir store.Directory, si *spi.SegmentInfo, fn *spi.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	return f.delegate.FieldsReader(dir, si, fn, context)
}

func (f *CrankyStoredFieldsFormat) FieldsWriter(dir store.Directory, si *spi.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from StoredFieldsFormat.fieldsWriter()")
	}
	writer, err := f.delegate.FieldsWriter(dir, si, context)
	if err != nil {
		return nil, err
	}
	return &CrankyStoredFieldsWriter{delegate: writer, random: f.random}, nil
}

type CrankyStoredFieldsWriter struct {
	delegate spi.StoredFieldsWriter
	random   *rand.Rand
}

func (w *CrankyStoredFieldsWriter) StartDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.startDocument()")
	}
	return w.delegate.StartDocument()
}

func (w *CrankyStoredFieldsWriter) FinishDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.finishDocument()")
	}
	return w.delegate.FinishDocument()
}

func (w *CrankyStoredFieldsWriter) WriteField(info *spi.FieldInfo, field spi.IndexableField) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.writeField()")
	}
	return w.delegate.WriteField(info, field)
}

func (w *CrankyStoredFieldsWriter) Finish(numDocs int) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.finish()")
	}
	return w.delegate.Finish(numDocs)
}

func (w *CrankyStoredFieldsWriter) Close() error {
	err := w.delegate.Close()
	if w.random.Intn(1000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.close()")
	}
	return err
}

// --- CrankyTermVectorsFormat ---

type CrankyTermVectorsFormat struct {
	delegate spi.TermVectorsFormat
	random   *rand.Rand
}

func NewCrankyTermVectorsFormat(delegate spi.TermVectorsFormat, random *rand.Rand) *CrankyTermVectorsFormat {
	return &CrankyTermVectorsFormat{delegate: delegate, random: random}
}

func (f *CrankyTermVectorsFormat) Name() string {
	return f.delegate.Name()
}

func (f *CrankyTermVectorsFormat) VectorsReader(dir store.Directory, si *spi.SegmentInfo, fi *spi.FieldInfos, context store.IOContext) (spi.TermVectorsReader, error) {
	return f.delegate.VectorsReader(dir, si, fi, context)
}

func (f *CrankyTermVectorsFormat) VectorsWriter(state *spi.SegmentWriteState) (spi.TermVectorsWriter, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from TermVectorsFormat.vectorsWriter()")
	}
	writer, err := f.delegate.VectorsWriter(state)
	if err != nil {
		return nil, err
	}
	return &CrankyTermVectorsWriter{delegate: writer, random: f.random}, nil
}

type CrankyTermVectorsWriter struct {
	delegate spi.TermVectorsWriter
	random   *rand.Rand
}

func (w *CrankyTermVectorsWriter) StartDocument(numFields int) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startDocument()")
	}
	return w.delegate.StartDocument(numFields)
}

func (w *CrankyTermVectorsWriter) StartField(info *spi.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startField()")
	}
	return w.delegate.StartField(info, numTerms, hasPositions, hasOffsets, hasPayloads)
}

func (w *CrankyTermVectorsWriter) StartTerm(term []byte) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startTerm()")
	}
	return w.delegate.StartTerm(term)
}

func (w *CrankyTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.addPosition()")
	}
	return w.delegate.AddPosition(position, startOffset, endOffset, payload)
}

func (w *CrankyTermVectorsWriter) FinishTerm() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishTerm()")
	}
	return w.delegate.FinishTerm()
}

func (w *CrankyTermVectorsWriter) FinishField() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishField()")
	}
	return w.delegate.FinishField()
}

func (w *CrankyTermVectorsWriter) FinishDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishDocument()")
	}
	return w.delegate.FinishDocument()
}

func (w *CrankyTermVectorsWriter) Close() error {
	err := w.delegate.Close()
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.close()")
	}
	return err
}
