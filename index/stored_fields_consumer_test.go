// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakeStoredValue is a minimal StoredValue used by the StoredFieldsConsumer
// tests. It carries exactly one typed payload selected by kind; the other
// accessors return their zero value, exactly as Lucene's per-variant getters
// behave. The package-level fakeCodec / fakeStoredFieldsFormat helpers from
// sorting_stored_fields_consumer_test.go are reused for the codec wiring.
type fakeStoredValue struct {
	kind StoredValueType
	str  string
	bin  []byte
	i32  int32
	i64  int64
	f32  float32
	f64  float64
}

func (v fakeStoredValue) Type() StoredValueType { return v.kind }
func (v fakeStoredValue) IntValue() int32       { return v.i32 }
func (v fakeStoredValue) LongValue() int64      { return v.i64 }
func (v fakeStoredValue) FloatValue() float32   { return v.f32 }
func (v fakeStoredValue) DoubleValue() float64  { return v.f64 }
func (v fakeStoredValue) BinaryValue() []byte   { return v.bin }
func (v fakeStoredValue) StringValue() string   { return v.str }

var _ StoredValue = fakeStoredValue{}

// newConsumerFixture builds a StoredFieldsConsumer for a segment of maxDoc
// documents, returning the consumer and the format that records writer calls.
func newConsumerFixture(t *testing.T, maxDoc int) (*StoredFieldsConsumer, *fakeStoredFieldsFormat) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", maxDoc, dir)
	format := newFakeStoredFieldsFormat("sfc")
	codec := &fakeCodec{stored: format}
	return NewStoredFieldsConsumer(codec, dir, info), format
}

func storedField(t *testing.T, name string) *FieldInfo {
	t.Helper()
	return NewFieldInfo(name, 0, DefaultFieldInfoOptions())
}

func TestStoredFieldsConsumer_WriterIsLazy(t *testing.T) {
	c, format := newConsumerFixture(t, 1)
	if c.Writer() != nil {
		t.Fatalf("Writer must be nil before any document is started")
	}
	if format.writes != 0 {
		t.Fatalf("no FieldsWriter call expected yet, got %d", format.writes)
	}
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if c.Writer() == nil {
		t.Fatalf("Writer must be non-nil after the first StartDocument")
	}
	if format.writes != 1 {
		t.Fatalf("exactly one FieldsWriter call expected, got %d", format.writes)
	}
}

func TestStoredFieldsConsumer_InitIsIdempotent(t *testing.T) {
	c, format := newConsumerFixture(t, 1)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	w := c.Writer()
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if c.Writer() != w {
		t.Fatalf("second init must not replace the writer")
	}
	if format.writes != 1 {
		t.Fatalf("second init must not allocate a new writer, got %d writes", format.writes)
	}
}

func TestStoredFieldsConsumer_StartDocumentFillsGaps(t *testing.T) {
	c, format := newConsumerFixture(t, 5)
	// Jump straight to docID 3: docs 0,1,2 must be emitted as empty fillers.
	if err := c.StartDocument(3); err != nil {
		t.Fatalf("StartDocument(3): %v", err)
	}
	if err := c.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if got := len(*format.docs); got != 4 {
		t.Fatalf("expected 4 docs (3 fillers + doc 3), got %d", got)
	}
	for i := 0; i < 4; i++ {
		if got := len((*format.docs)[i]); got != 0 {
			t.Errorf("doc %d must be empty, got %d fields", i, got)
		}
	}
}

func TestStoredFieldsConsumer_StartDocumentRejectsBackwardsDocID(t *testing.T) {
	c, _ := newConsumerFixture(t, 5)
	if err := c.StartDocument(2); err != nil {
		t.Fatalf("StartDocument(2): %v", err)
	}
	if err := c.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if err := c.StartDocument(2); err == nil {
		t.Fatalf("StartDocument with a non-increasing docID must error")
	}
	if err := c.StartDocument(1); err == nil {
		t.Fatalf("StartDocument with a backwards docID must error")
	}
}

func TestStoredFieldsConsumer_WriteFieldDispatchesAllVariants(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	// Swap in a recording writer so the IndexableField the consumer produces
	// per variant is inspected directly, without the lossy synthesis the
	// fake stored-fields writer applies to non-copiedField inputs.
	rec := &recordingWriter{}
	c.writer = rec

	values := []fakeStoredValue{
		{kind: StoredValueTypeString, str: "hello"},
		{kind: StoredValueTypeBinary, bin: []byte{1, 2, 3}},
		{kind: StoredValueTypeInteger, i32: 42},
		{kind: StoredValueTypeLong, i64: 1 << 40},
		{kind: StoredValueTypeFloat, f32: 1.5},
		{kind: StoredValueTypeDouble, f64: 2.5},
	}
	for _, v := range values {
		if err := c.WriteField(storedField(t, v.kind.String()), v); err != nil {
			t.Fatalf("WriteField(%s): %v", v.kind, err)
		}
	}
	if len(rec.received) != len(values) {
		t.Fatalf("expected %d fields written, got %d", len(values), len(rec.received))
	}

	if got := rec.received[0].StringValue(); got != "hello" {
		t.Errorf("string field: got %q, want %q", got, "hello")
	}
	if got := rec.received[1].BinaryValue(); len(got) != 3 || got[0] != 1 {
		t.Errorf("binary field: got %v", got)
	}
	if got := rec.received[2].NumericValue(); got != int32(42) {
		t.Errorf("int field: got %v (%T), want int32(42)", got, got)
	}
	if got := rec.received[3].NumericValue(); got != int64(1)<<40 {
		t.Errorf("long field: got %v (%T), want %d", got, got, int64(1)<<40)
	}
	if got := rec.received[4].NumericValue(); got != float32(1.5) {
		t.Errorf("float field: got %v (%T), want float32(1.5)", got, got)
	}
	if got := rec.received[5].NumericValue(); got != float64(2.5) {
		t.Errorf("double field: got %v (%T), want float64(2.5)", got, got)
	}
	// The adapted field must report itself as stored-only. FieldType()
	// is no longer on the narrow spi.IndexableField surface, so probe
	// it via the legacy index-side projection.
	ft, ok := rec.received[0].(interface{ FieldType() FieldTypeInterface })
	if !ok {
		t.Fatalf("adapted field does not expose FieldType() FieldTypeInterface")
	}
	if !ft.FieldType().IsStored() || ft.FieldType().IsIndexed() {
		t.Errorf("adapted field must be stored-only")
	}
}

func TestStoredFieldsConsumer_WriteFieldRejectsNilValue(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if err := c.WriteField(storedField(t, "f"), nil); err == nil {
		t.Fatalf("WriteField with a nil value must error")
	}
}

func TestStoredFieldsConsumer_WriteFieldRejectsNilFieldInfo(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	v := fakeStoredValue{kind: StoredValueTypeString, str: "x"}
	if err := c.WriteField(nil, v); err == nil {
		t.Fatalf("WriteField with a nil FieldInfo must error")
	}
}

func TestStoredFieldsConsumer_WriteFieldRejectsUnknownVariant(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	bogus := fakeStoredValue{kind: StoredValueType(99)}
	if err := c.WriteField(storedField(t, "f"), bogus); err == nil {
		t.Fatalf("WriteField with an unknown StoredValue variant must error")
	}
}

func TestStoredFieldsConsumer_FinishEmitsTrailingFillers(t *testing.T) {
	c, format := newConsumerFixture(t, 4)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if err := c.WriteField(storedField(t, "f"), fakeStoredValue{kind: StoredValueTypeString, str: "v"}); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := c.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	// maxDoc 4: docs 1,2,3 carried no stored fields and must be emitted empty.
	if err := c.Finish(4); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if got := len(*format.docs); got != 4 {
		t.Fatalf("expected 4 docs after Finish, got %d", got)
	}
	for i := 1; i < 4; i++ {
		if got := len((*format.docs)[i]); got != 0 {
			t.Errorf("trailing doc %d must be empty, got %d fields", i, got)
		}
	}
}

func TestStoredFieldsConsumer_FlushFinalizesAndClosesWriter(t *testing.T) {
	c, format := newConsumerFixture(t, 2)
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 2, dir)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if err := c.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	w := c.Writer().(*fakeStoredFieldsWriter)
	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !w.closed {
		t.Fatalf("Flush must close the writer")
	}
	if format.writes != 1 {
		t.Fatalf("Flush must not allocate a new writer, got %d writes", format.writes)
	}
}

func TestStoredFieldsConsumer_FlushWithoutWriterIsNoOp(t *testing.T) {
	c, _ := newConsumerFixture(t, 0)
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 0, dir)
	// No document started: writer is nil and Flush must not panic.
	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush without a writer must be a no-op, got %v", err)
	}
}

func TestStoredFieldsConsumer_FlushRequiresState(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	if err := c.Flush(nil, nil); err == nil {
		t.Fatalf("Flush with a nil state must error")
	}
	if err := c.Flush(&SegmentWriteState{}, nil); err == nil {
		t.Fatalf("Flush with a nil SegmentInfo must error")
	}
}

func TestStoredFieldsConsumer_AbortClosesWriter(t *testing.T) {
	c, _ := newConsumerFixture(t, 1)
	if err := c.StartDocument(0); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	w := c.Writer().(*fakeStoredFieldsWriter)
	c.Abort()
	if !w.closed {
		t.Fatalf("Abort must close the writer")
	}
}

func TestStoredFieldsConsumer_AbortWithoutWriterIsSafe(t *testing.T) {
	c, _ := newConsumerFixture(t, 0)
	// No document started: Abort on a nil writer must not panic.
	c.Abort()
}

func TestStoredFieldsConsumer_InitPropagatesFormatError(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)
	c := NewStoredFieldsConsumer(&failingCodec{}, dir, info)
	if err := c.InitStoredFieldsWriter(); err == nil {
		t.Fatalf("InitStoredFieldsWriter must propagate the format error")
	}
}

// failingCodec exposes a StoredFieldsFormat whose FieldsWriter always fails,
// so the error path of StoredFieldsConsumer.InitStoredFieldsWriter is covered.
type failingCodec struct{}

func (failingCodec) Name() string                              { return "failing-codec" }
func (failingCodec) PostingsFormat() PostingsFormat            { return nil }
func (failingCodec) StoredFieldsFormat() StoredFieldsFormat    { return failingStoredFieldsFormat{} }
func (failingCodec) FieldInfosFormat() FieldInfosFormat        { return nil }
func (failingCodec) SegmentInfosFormat() SegmentInfosFormat    { return nil }
func (failingCodec) SegmentInfoFormat() SegmentInfoFormat      { return nil }
func (failingCodec) TermVectorsFormat() TermVectorsFormat      { return nil }
func (failingCodec) CompoundFormat() CompoundFormat            { return nil }
func (failingCodec) KnnVectorsFormat() KnnVectorsFormatFactory { return nil }

// failingStoredFieldsFormat always fails FieldsWriter.
type failingStoredFieldsFormat struct{}

func (failingStoredFieldsFormat) Name() string { return "failing" }

func (failingStoredFieldsFormat) FieldsWriter(store.Directory, *SegmentInfo, store.IOContext) (StoredFieldsWriter, error) {
	return nil, errors.New("FieldsWriter denied")
}

func (failingStoredFieldsFormat) FieldsReader(store.Directory, *SegmentInfo, *FieldInfos, store.IOContext) (StoredFieldsReader, error) {
	return nil, errors.New("FieldsReader denied")
}
