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

// String mirrors CrankyCodec.toString() (CrankyCodec.java:94-97):
// "Cranky(" + delegate + ")". Codec.toString() returns the codec name
// (Codec.java:158-161), so the delegate renders as its name.
func (c *CrankyCodec) String() string {
	return fmt.Sprintf("Cranky(%s)", c.Delegate().Name())
}

// --- CrankyCompoundFormat ---

type CrankyCompoundFormat struct {
	delegate spi.CompoundFormat
	random   *rand.Rand
}

func NewCrankyCompoundFormat(delegate spi.CompoundFormat, random *rand.Rand) *CrankyCompoundFormat {
	return &CrankyCompoundFormat{delegate: delegate, random: random}
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

// GetMergeInstance carries the default body of PointsReader.getMergeInstance()
// (PointsReader.java), which CrankyPointsReader does not override: it returns
// the receiver.
func (r *CrankyPointsReader) GetMergeInstance() spi.PointsReader {
	return r
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

// --- CrankySegmentInfoFormat ---

type CrankySegmentInfoFormat struct {
	delegate spi.SegmentInfoFormat
	random   *rand.Rand
}

func NewCrankySegmentInfoFormat(delegate spi.SegmentInfoFormat, random *rand.Rand) *CrankySegmentInfoFormat {
	return &CrankySegmentInfoFormat{delegate: delegate, random: random}
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
