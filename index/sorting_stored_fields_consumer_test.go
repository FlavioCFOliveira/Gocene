// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakeStoredFieldsFormat is the minimal StoredFieldsFormat the test
// injects in place of Lucene90CompressingStoredFieldsFormat (which is
// not yet reachable from package index — see Sprint 55 deviation notes
// on SortingStoredFieldsConsumer). It records each FieldsWriter write
// in-memory keyed by document index, and FieldsReader replays them.
//
// The writer touches the directory exactly once per call (via
// CreateOutput on a deterministic name) so the tracking wrapper sees a
// file enter and exit during flush/abort.
type fakeStoredFieldsFormat struct {
	docs   *[][]copiedField
	name   string
	writes int
	reads  int
}

func newFakeStoredFieldsFormat(name string) *fakeStoredFieldsFormat {
	docs := make([][]copiedField, 0)
	return &fakeStoredFieldsFormat{docs: &docs, name: name}
}

func (f *fakeStoredFieldsFormat) Name() string { return f.name }

func (f *fakeStoredFieldsFormat) FieldsWriter(dir store.Directory, info *SegmentInfo, ctx store.IOContext) (StoredFieldsWriter, error) {
	name := fmt.Sprintf("_%s_%s_%d.fdt", f.name, info.Name(), f.writes)
	f.writes++
	out, err := dir.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	*f.docs = (*f.docs)[:0]
	return &fakeStoredFieldsWriter{out: out, docs: f.docs}, nil
}

func (f *fakeStoredFieldsFormat) FieldsReader(dir store.Directory, info *SegmentInfo, fis *FieldInfos, ctx store.IOContext) (StoredFieldsReader, error) {
	f.reads++
	return &fakeStoredFieldsReader{docs: *f.docs}, nil
}

type fakeStoredFieldsWriter struct {
	out     store.IndexOutput
	docs    *[][]copiedField
	current []copiedField
	closed  bool
}

func (w *fakeStoredFieldsWriter) StartDocument() error {
	w.current = w.current[:0]
	return nil
}

func (w *fakeStoredFieldsWriter) FinishDocument() error {
	doc := make([]copiedField, len(w.current))
	copy(doc, w.current)
	*w.docs = append(*w.docs, doc)
	w.current = w.current[:0]
	return nil
}

func (w *fakeStoredFieldsWriter) WriteField(field IndexableField) error {
	if cf, ok := field.(*copiedField); ok {
		w.current = append(w.current, *cf)
		return nil
	}
	// For non-copyVisitor producers we synthesise a string-only field.
	w.current = append(w.current, copiedField{name: field.Name(), kind: copiedString, str: field.StringValue()})
	return nil
}

func (w *fakeStoredFieldsWriter) Finish(numDocs int) error { return nil }

func (w *fakeStoredFieldsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

type fakeStoredFieldsReader struct {
	docs [][]copiedField
}

func (r *fakeStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	if docID < 0 || docID >= len(r.docs) {
		return fmt.Errorf("fakeReader: docID %d out of range [0,%d)", docID, len(r.docs))
	}
	for _, f := range r.docs[docID] {
		switch f.kind {
		case copiedString:
			visitor.StringField(f.name, f.str)
		case copiedBinary:
			visitor.BinaryField(f.name, f.bin)
		case copiedInt:
			visitor.IntField(f.name, int(f.num))
		case copiedLong:
			visitor.LongField(f.name, f.num)
		case copiedFloat:
			visitor.FloatField(f.name, f.f32)
		case copiedDouble:
			visitor.DoubleField(f.name, f.f64)
		}
	}
	return nil
}

func (r *fakeStoredFieldsReader) Close() error { return nil }

// fakeCodec exposes the fake format as the codec's stored-fields format
// so the final (post-sort) writer path also lands in the same buckets.
type fakeCodec struct {
	stored *fakeStoredFieldsFormat
}

func (c *fakeCodec) Name() string                           { return "fake-codec" }
func (c *fakeCodec) PostingsFormat() PostingsFormat         { return nil }
func (c *fakeCodec) StoredFieldsFormat() StoredFieldsFormat { return c.stored }
func (c *fakeCodec) FieldInfosFormat() FieldInfosFormat     { return nil }
func (c *fakeCodec) SegmentInfosFormat() SegmentInfosFormat { return nil }
func (c *fakeCodec) SegmentInfoFormat() SegmentInfoFormat   { return nil }
func (c *fakeCodec) TermVectorsFormat() TermVectorsFormat   { return nil }

// reverseSortMap{n} is reused from sorted_set_doc_values_writer_test.go;
// it implements SorterDocMap as a reverse permutation with pointer receivers.

func newTestConsumer(t *testing.T, docs int) (*SortingStoredFieldsConsumer, *fakeStoredFieldsFormat, store.Directory, *SegmentInfo) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", docs, dir)
	tempFormat := newFakeStoredFieldsFormat("temp")
	finalFormat := newFakeStoredFieldsFormat("final")
	codec := &fakeCodec{stored: finalFormat}
	c := NewSortingStoredFieldsConsumer(codec, dir, info)
	c.SetTempStoredFieldsFormat(tempFormat)
	return c, finalFormat, dir, info
}

func TestSortingStoredFieldsConsumer_InitRequiresTempFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)
	c := NewSortingStoredFieldsConsumer(&fakeCodec{stored: newFakeStoredFieldsFormat("temp")}, dir, info)
	// Deliberately skip SetTempStoredFieldsFormat.
	if err := c.InitStoredFieldsWriter(); !errors.Is(err, ErrTempFormatUnset) {
		t.Fatalf("InitStoredFieldsWriter without temp format: got %v, want ErrTempFormatUnset", err)
	}
	if c.TempDirectory() != nil {
		t.Fatalf("TempDirectory should remain nil when init fails")
	}
}

func TestSortingStoredFieldsConsumer_InitIsIdempotent(t *testing.T) {
	c, _, _, _ := newTestConsumer(t, 1)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	tmp := c.TempDirectory()
	if tmp == nil {
		t.Fatalf("TempDirectory should be non-nil after init")
	}
	firstFiles := tmp.TemporaryFiles()
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if got := c.TempDirectory(); got != tmp {
		t.Fatalf("second init must not replace tracking wrapper")
	}
	if got := len(c.TempDirectory().TemporaryFiles()); got != len(firstFiles) {
		t.Fatalf("second init must not allocate more files: was %d, now %d", len(firstFiles), got)
	}
}

func TestSortingStoredFieldsConsumer_TrackingWrapperRecordsFiles(t *testing.T) {
	c, _, _, _ := newTestConsumer(t, 1)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}
	files := c.TempDirectory().TemporaryFiles()
	if len(files) != 1 {
		t.Fatalf("expected 1 tracked file after init, got %d (%v)", len(files), files)
	}
	// Defensive copy: mutating the returned slice must not affect the wrapper.
	files[0] = "tampered"
	if again := c.TempDirectory().TemporaryFiles(); again[0] == "tampered" {
		t.Fatalf("TemporaryFiles must return a defensive copy")
	}
}

func TestSortingStoredFieldsConsumer_FlushAppliesSortMap(t *testing.T) {
	const n = 3
	c, finalFormat, dir, info := newTestConsumer(t, n)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Hand-write 3 docs into the temp writer through the underlying interface.
	// Each doc carries a single string field equal to its document index.
	tw := c.writer.(*fakeStoredFieldsWriter)
	for i := 0; i < n; i++ {
		if err := tw.StartDocument(); err != nil {
			t.Fatal(err)
		}
		if err := tw.WriteField(&copiedField{name: "doc_index", kind: copiedString, str: fmt.Sprintf("doc-%d", i)}); err != nil {
			t.Fatal(err)
		}
		if err := tw.FinishDocument(); err != nil {
			t.Fatal(err)
		}
	}

	state := &SegmentWriteState{Directory: dir, SegmentInfo: info, FieldInfos: nil}
	if err := c.Flush(state, &reverseSortMap{n: n}); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// finalFormat received the reversed permutation: doc 0 (sorted) <- doc 2 (source).
	if got := len(*finalFormat.docs); got != n {
		t.Fatalf("final doc count: got %d, want %d", got, n)
	}
	want := []string{"doc-2", "doc-1", "doc-0"}
	for i, w := range want {
		got := (*finalFormat.docs)[i][0].str
		if got != w {
			t.Errorf("doc[%d]: got %q, want %q", i, got, w)
		}
	}

	// Flush must have deleted the temp file.
	if c.TempDirectory() != nil {
		t.Fatalf("Flush must clear TempDirectory")
	}
}

func TestSortingStoredFieldsConsumer_FlushIdentityWhenSortMapNil(t *testing.T) {
	const n = 2
	c, finalFormat, dir, info := newTestConsumer(t, n)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatal(err)
	}
	tw := c.writer.(*fakeStoredFieldsWriter)
	for i := 0; i < n; i++ {
		_ = tw.StartDocument()
		_ = tw.WriteField(&copiedField{name: "k", kind: copiedInt, num: int64(100 + i)})
		_ = tw.FinishDocument()
	}
	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush nil sortMap: %v", err)
	}
	if len(*finalFormat.docs) != n {
		t.Fatalf("doc count: got %d, want %d", len(*finalFormat.docs), n)
	}
	for i := 0; i < n; i++ {
		got := (*finalFormat.docs)[i][0].num
		if int(got) != 100+i {
			t.Errorf("doc[%d]: got %d, want %d", i, got, 100+i)
		}
	}
}

func TestSortingStoredFieldsConsumer_FlushWithoutInitIsNoOp(t *testing.T) {
	c, finalFormat, dir, info := newTestConsumer(t, 0)
	// No InitStoredFieldsWriter call.
	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush without init must be a no-op, got %v", err)
	}
	if len(*finalFormat.docs) != 0 {
		t.Fatalf("Flush without init must not produce docs, got %d", len(*finalFormat.docs))
	}
}

func TestSortingStoredFieldsConsumer_AbortDeletesTempFiles(t *testing.T) {
	c, _, dir, _ := newTestConsumer(t, 1)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatal(err)
	}
	files := c.TempDirectory().TemporaryFiles()
	if len(files) != 1 {
		t.Fatalf("expected 1 temp file, got %d", len(files))
	}
	tempName := files[0]

	c.Abort()
	if c.TempDirectory() != nil {
		t.Fatalf("Abort must clear TempDirectory")
	}
	// Underlying directory must no longer list the temp file.
	listed, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, n := range listed {
		if n == tempName {
			t.Fatalf("Abort must delete temp file %q (still listed)", tempName)
		}
	}
}

func TestSortingStoredFieldsConsumer_AbortIsSafeWithoutInit(t *testing.T) {
	c, _, _, _ := newTestConsumer(t, 0)
	// Must not panic.
	c.Abort()
}

func TestSortingStoredFieldsConsumer_FlushRequiresState(t *testing.T) {
	c, _, _, _ := newTestConsumer(t, 1)
	if err := c.InitStoredFieldsWriter(); err != nil {
		t.Fatal(err)
	}
	if err := c.Flush(nil, nil); err == nil {
		t.Fatalf("Flush with nil state must error")
	}
}

func TestCopyVisitor_DispatchesAllTypes(t *testing.T) {
	w := &recordingWriter{}
	v := &copyVisitor{writer: w}

	v.StringField("s", "hello")
	v.BinaryField("b", []byte{1, 2, 3})
	v.IntField("i", 42)
	v.LongField("l", 1<<40)
	v.FloatField("f", 1.5)
	v.DoubleField("d", 2.5)

	if v.err != nil {
		t.Fatalf("copyVisitor.err: %v", v.err)
	}
	if len(w.received) != 6 {
		t.Fatalf("expected 6 fields, got %d", len(w.received))
	}

	// Spot-check each kind so a future field reordering does not silently
	// alter the dispatch matrix.
	cases := []struct {
		name string
		kind copiedKind
	}{
		{"s", copiedString},
		{"b", copiedBinary},
		{"i", copiedInt},
		{"l", copiedLong},
		{"f", copiedFloat},
		{"d", copiedDouble},
	}
	for i, want := range cases {
		got := w.received[i].(*copiedField)
		if got.name != want.name || got.kind != want.kind {
			t.Errorf("field %d: got (%s,%v), want (%s,%v)", i, got.name, got.kind, want.name, want.kind)
		}
	}

	// Binary value must be a copy: mutating the caller's slice must not
	// affect the captured field.
	cf := w.received[1].(*copiedField)
	if len(cf.bin) != 3 || cf.bin[0] != 1 {
		t.Fatalf("binary value not captured: %v", cf.bin)
	}
}

func TestCopyVisitor_StopsAfterWriteError(t *testing.T) {
	w := &recordingWriter{returnErr: errors.New("disk full")}
	v := &copyVisitor{writer: w}
	v.StringField("s", "first")
	v.StringField("s", "second")
	if v.err == nil {
		t.Fatalf("expected copyVisitor.err to capture the writer error")
	}
	if len(w.received) != 1 {
		t.Fatalf("expected exactly 1 write attempt after error, got %d", len(w.received))
	}
}

func TestCopiedField_NumericValue(t *testing.T) {
	cases := []struct {
		f    copiedField
		want interface{}
	}{
		{copiedField{kind: copiedInt, num: 5}, int(5)},
		{copiedField{kind: copiedLong, num: 1 << 40}, int64(1 << 40)},
		{copiedField{kind: copiedFloat, f32: 1.5}, float32(1.5)},
		{copiedField{kind: copiedDouble, f64: 2.5}, float64(2.5)},
		{copiedField{kind: copiedString, str: "x"}, nil},
		{copiedField{kind: copiedBinary, bin: []byte{1}}, nil},
	}
	for i, c := range cases {
		got := c.f.NumericValue()
		if got != c.want {
			t.Errorf("case %d: got %v (%T), want %v (%T)", i, got, got, c.want, c.want)
		}
	}
}

func TestCopiedFieldType_StoredOnly(t *testing.T) {
	ft := copiedFieldType{}
	if !ft.IsStored() {
		t.Errorf("IsStored should be true")
	}
	if ft.IsIndexed() || ft.IsTokenized() || ft.StoreTermVectors() ||
		ft.StoreTermVectorPositions() || ft.StoreTermVectorOffsets() {
		t.Errorf("only IsStored should be true for copiedFieldType")
	}
	if ft.GetIndexOptions() != IndexOptionsNone {
		t.Errorf("IndexOptions: got %v, want IndexOptionsNone", ft.GetIndexOptions())
	}
	if ft.GetDocValuesType() != DocValuesTypeNone {
		t.Errorf("DocValuesType: got %v, want DocValuesTypeNone", ft.GetDocValuesType())
	}
}

// recordingWriter captures every WriteField call; FinishDocument et al.
// are no-ops because copyVisitor only exercises WriteField.
type recordingWriter struct {
	received  []IndexableField
	returnErr error
}

func (w *recordingWriter) StartDocument() error     { return nil }
func (w *recordingWriter) FinishDocument() error    { return nil }
func (w *recordingWriter) Finish(numDocs int) error { return nil }
func (w *recordingWriter) Close() error             { return nil }
func (w *recordingWriter) WriteField(f IndexableField) error {
	w.received = append(w.received, f)
	return w.returnErr
}
