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

// pointsWriterMerger is the merge member of PointsWriter. The codec writers
// carry it through codecs.BasePointsWriter; spi.PointsWriter cannot declare it
// because MergeState lives in package index, which spi cannot import. This is
// the same narrow interface index.SegmentMerger.mergePoints uses to reach it.
type pointsWriterMerger interface {
	Merge(mergeState *index.MergeState) error
}

// CrankyPointsWriter is the Go port of
// org.apache.lucene.tests.codecs.cranky.CrankyPointsFormat.CrankyPointsWriter.
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

// Merge renders `public void merge(MergeState mergeState)`.
func (w *CrankyPointsWriter) Merge(mergeState *index.MergeState) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	merger, ok := w.delegate.(pointsWriterMerger)
	if !ok {
		return fmt.Errorf("cranky: PointsWriter %T does not carry merge(MergeState)", w.delegate)
	}
	err := merger.Merge(mergeState)
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

// CrankyPointsReader is the Go port of
// org.apache.lucene.tests.codecs.cranky.CrankyPointsFormat.CrankyPointsReader.
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

// GetValues renders `public PointValues getValues(String fieldName)`, which
// wraps the delegate's PointValues in the anonymous subclass rendered by
// [CrankyPointValues].
func (r *CrankyPointsReader) GetValues(fieldName string) (index.PointValues, error) {
	delegate, err := r.delegate.GetValues(fieldName)
	if err != nil {
		return nil, err
	}
	if delegate == nil {
		return nil, nil
	}
	return newCrankyPointValues(delegate, r.random), nil
}

// CrankyPointValues is the anonymous PointValues returned by
// CrankyPointsReader.getValues.
type CrankyPointValues struct {
	*spi.BasePointValues
	delegate index.PointValues
	random   *rand.Rand
}

func newCrankyPointValues(delegate index.PointValues, random *rand.Rand) *CrankyPointValues {
	v := &CrankyPointValues{delegate: delegate, random: random}
	v.BasePointValues = spi.NewBasePointValues(v)
	return v
}

// GetPointTree renders `public PointTree getPointTree()`, which wraps the
// delegate's tree in the anonymous PointTree rendered by [CrankyPointTree].
func (v *CrankyPointValues) GetPointTree() (index.PointTree, error) {
	pointTree, err := v.delegate.GetPointTree()
	if err != nil {
		return nil, err
	}
	return &CrankyPointTree{delegate: pointTree, random: v.random}, nil
}

func (v *CrankyPointValues) GetMinPackedValue() ([]byte, error) {
	if v.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException")
	}
	return v.delegate.GetMinPackedValue()
}

func (v *CrankyPointValues) GetMaxPackedValue() ([]byte, error) {
	if v.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException")
	}
	return v.delegate.GetMaxPackedValue()
}

func (v *CrankyPointValues) GetNumDimensions() (int, error) {
	if v.random.Intn(100) == 0 {
		return 0, fmt.Errorf("Fake IOException")
	}
	return v.delegate.GetNumDimensions()
}

func (v *CrankyPointValues) GetNumIndexDimensions() (int, error) {
	if v.random.Intn(100) == 0 {
		return 0, fmt.Errorf("Fake IOException")
	}
	return v.delegate.GetNumIndexDimensions()
}

func (v *CrankyPointValues) GetBytesPerDimension() (int, error) {
	if v.random.Intn(100) == 0 {
		return 0, fmt.Errorf("Fake IOException")
	}
	return v.delegate.GetBytesPerDimension()
}

// Size renders `public long size()`, whose body delegates without a fake
// failure.
func (v *CrankyPointValues) Size() int64 {
	return v.delegate.Size()
}

func (v *CrankyPointValues) GetDocCount() int {
	return v.delegate.GetDocCount()
}

// CrankyPointTree is the anonymous PointTree returned by the anonymous
// PointValues' getPointTree.
type CrankyPointTree struct {
	delegate index.PointTree
	random   *rand.Rand
}

// Clone renders `public PointTree clone()`, whose body is
// `return pointTree.clone()` — it returns the delegate's clone unwrapped, as
// Java does.
func (t *CrankyPointTree) Clone() index.PointTree {
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

func (t *CrankyPointTree) VisitDocIDs(visitor index.IntersectVisitor) error {
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := t.delegate.VisitDocIDs(visitor)
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

func (t *CrankyPointTree) VisitDocValues(visitor index.IntersectVisitor) error {
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	err := t.delegate.VisitDocValues(visitor)
	if t.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException")
	}
	return err
}

var (
	_ index.PointValues = (*CrankyPointValues)(nil)
	_ index.PointTree   = (*CrankyPointTree)(nil)
)

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
