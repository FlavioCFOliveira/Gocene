// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// stubTVPerField is a minimal TermVectorsPerFieldHandle for unit tests:
// it records FinishDocument invocations and provides a deterministic
// CompareName comparator backed by strings.Compare.
type stubTVPerField struct {
	name        string
	finishCalls int
	finishErr   error
}

func newStubTVPerField(name string) *stubTVPerField {
	return &stubTVPerField{name: name}
}

func (s *stubTVPerField) CompareName(other TermVectorsPerFieldHandle) int {
	o, ok := other.(*stubTVPerField)
	if !ok {
		return 0
	}
	return strings.Compare(s.name, o.name)
}

func (s *stubTVPerField) FinishDocument() error {
	s.finishCalls++
	return s.finishErr
}

// recordingTVWriter is a TermVectorsWriter that records every call so
// tests can assert the sequence the consumer issues.
type recordingTVWriter struct {
	startDoc  []int
	startFld  int
	startTerm int
	addPos    int
	finishTrm int
	finishFld int
	finishDoc int
	closed    bool
	closeErr  error
}

func (w *recordingTVWriter) StartDocument(numFields int) error {
	w.startDoc = append(w.startDoc, numFields)
	return nil
}
func (w *recordingTVWriter) StartField(fi *FieldInfo, numTerms int, p, o, pl bool) error {
	w.startFld++
	return nil
}
func (w *recordingTVWriter) StartTerm(term []byte) error               { w.startTerm++; return nil }
func (w *recordingTVWriter) AddPosition(pos, s, e int, p []byte) error { w.addPos++; return nil }
func (w *recordingTVWriter) FinishTerm() error                         { w.finishTrm++; return nil }
func (w *recordingTVWriter) FinishField() error                        { w.finishFld++; return nil }
func (w *recordingTVWriter) FinishDocument() error                     { w.finishDoc++; return nil }
func (w *recordingTVWriter) Close() error                              { w.closed = true; return w.closeErr }

// recordingTVFormat hands out one recordingTVWriter per VectorsWriter
// call so tests can read the writer after the consumer flushes it.
type recordingTVFormat struct {
	last *recordingTVWriter
	err  error
}

func (f *recordingTVFormat) Name() string { return "recording" }
func (f *recordingTVFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.last = &recordingTVWriter{}
	return f.last, nil
}
func (f *recordingTVFormat) VectorsReader(_ store.Directory, _ *SegmentInfo, _ *FieldInfos, _ store.IOContext) (TermVectorsReader, error) {
	return nil, errors.New("recording: no reader")
}

// recordingCodec wraps recordingTVFormat as the term-vectors format.
type recordingCodec struct {
	tv *recordingTVFormat
}

func (c *recordingCodec) Name() string                           { return "recording-codec" }
func (c *recordingCodec) PostingsFormat() PostingsFormat         { return nil }
func (c *recordingCodec) StoredFieldsFormat() StoredFieldsFormat { return nil }
func (c *recordingCodec) FieldInfosFormat() FieldInfosFormat     { return nil }
func (c *recordingCodec) SegmentInfosFormat() SegmentInfosFormat { return nil }
func (c *recordingCodec) SegmentInfoFormat() SegmentInfoFormat   { return nil }
func (c *recordingCodec) TermVectorsFormat() TermVectorsFormat {
	if c.tv == nil {
		return nil
	}
	return c.tv
}

func newTestTermVectorsConsumer(t *testing.T, docs int) (*TermVectorsConsumer, *recordingTVFormat) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", docs, dir)
	format := &recordingTVFormat{}
	codec := &recordingCodec{tv: format}
	return NewTermVectorsConsumer(dir, info, codec), format
}

func TestTermVectorsConsumer_NilInfoReturnsNil(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	if got := NewTermVectorsConsumer(dir, nil, &recordingCodec{tv: &recordingTVFormat{}}); got != nil {
		t.Fatalf("NewTermVectorsConsumer(nil info) = %v, want nil", got)
	}
}

func TestTermVectorsConsumer_InitialState(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 3)
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	if c.HasVectors() {
		t.Fatal("HasVectors must be false on a fresh consumer")
	}
	if c.NumVectorFields() != 0 {
		t.Fatalf("NumVectorFields = %d, want 0", c.NumVectorFields())
	}
	if c.Writer != nil {
		t.Fatal("Writer must be nil before InitTermVectorsWriter")
	}
	if c.FlushTerm == nil || c.VectorSliceReaderPos == nil || c.VectorSliceReaderOff == nil {
		t.Fatal("scratch fields must be pre-allocated")
	}
	if c.RamBytesUsed() != 0 {
		t.Fatalf("RamBytesUsed = %d, want 0 before InitTermVectorsWriter", c.RamBytesUsed())
	}
}

func TestTermVectorsConsumer_SetHasVectors(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	if !c.HasVectors() {
		t.Fatal("HasVectors() = false after SetHasVectors()")
	}
}

func TestTermVectorsConsumer_InitTermVectorsWriterIdempotent(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	first := c.Writer
	if first == nil {
		t.Fatal("Writer still nil after init")
	}
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if c.Writer != first {
		t.Fatal("InitTermVectorsWriter must be idempotent")
	}
	if fmt.last == nil {
		t.Fatal("format never asked for a writer")
	}
}

func TestTermVectorsConsumer_InitTermVectorsWriterErrors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)

	t.Run("no codec", func(t *testing.T) {
		c := NewTermVectorsConsumer(dir, info, nil)
		if err := c.InitTermVectorsWriter(); err == nil {
			t.Fatal("expected error for nil Codec")
		}
	})

	t.Run("codec without format", func(t *testing.T) {
		c := NewTermVectorsConsumer(dir, info, &recordingCodec{tv: nil})
		if err := c.InitTermVectorsWriter(); err == nil {
			t.Fatal("expected error for nil TermVectorsFormat")
		}
	})

	t.Run("format returns error", func(t *testing.T) {
		boom := errors.New("nope")
		c := NewTermVectorsConsumer(dir, info, &recordingCodec{tv: &recordingTVFormat{err: boom}})
		if err := c.InitTermVectorsWriter(); err == nil || !errors.Is(err, boom) {
			t.Fatalf("err = %v, want wrapping of %v", err, boom)
		}
	})
}

func TestTermVectorsConsumer_AddFieldToFlushGrows(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	const n = 10 // starting capacity is 1, so this forces several oversizes
	for i := 0; i < n; i++ {
		c.AddFieldToFlush(newStubTVPerField("f"))
	}
	if c.NumVectorFields() != n {
		t.Fatalf("NumVectorFields = %d, want %d", c.NumVectorFields(), n)
	}
	if cap(c.perFields) < n {
		t.Fatalf("perFields cap = %d, want >= %d", cap(c.perFields), n)
	}
}

func TestTermVectorsConsumer_StartDocumentResets(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	c.AddFieldToFlush(newStubTVPerField("a"))
	c.AddFieldToFlush(newStubTVPerField("b"))
	if c.NumVectorFields() != 2 {
		t.Fatalf("setup: NumVectorFields=%d", c.NumVectorFields())
	}
	c.StartDocument()
	if c.NumVectorFields() != 0 {
		t.Fatalf("after StartDocument NumVectorFields = %d, want 0", c.NumVectorFields())
	}
	for i, f := range c.perFields {
		if f != nil {
			t.Fatalf("perFields[%d] = %v, want nil after StartDocument", i, f)
		}
	}
}

func TestTermVectorsConsumer_ResetFieldsClearsRefs(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	c.AddFieldToFlush(newStubTVPerField("a"))
	c.ResetFields()
	if c.NumVectorFields() != 0 {
		t.Fatalf("NumVectorFields = %d, want 0", c.NumVectorFields())
	}
	for i, f := range c.perFields {
		if f != nil {
			t.Fatalf("perFields[%d] = %v, want nil after ResetFields", i, f)
		}
	}
}

func TestTermVectorsConsumer_FinishDocumentNoVectorsIsNoop(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	// HasVectors is false → must return nil and never touch the writer.
	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if fmt.last != nil {
		t.Fatal("writer must not be initialised when HasVectors is false")
	}
}

func TestTermVectorsConsumer_FinishDocumentSortsAndFlushes(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()

	// Add fields in reverse-name order so the sort step must actually
	// reorder them before FinishDocument is dispatched.
	beta := newStubTVPerField("beta")
	alpha := newStubTVPerField("alpha")
	c.AddFieldToFlush(beta)
	c.AddFieldToFlush(alpha)

	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if fmt.last == nil {
		t.Fatal("writer never initialised")
	}
	if len(fmt.last.startDoc) != 1 || fmt.last.startDoc[0] != 2 {
		t.Fatalf("StartDocument calls = %v, want [2]", fmt.last.startDoc)
	}
	if fmt.last.finishDoc != 1 {
		t.Fatalf("FinishDocument writer calls = %d, want 1", fmt.last.finishDoc)
	}
	if alpha.finishCalls != 1 || beta.finishCalls != 1 {
		t.Fatalf("per-field finish calls: alpha=%d beta=%d, want 1/1", alpha.finishCalls, beta.finishCalls)
	}
	if c.LastDocID != 1 {
		t.Fatalf("LastDocID = %d, want 1", c.LastDocID)
	}
	// numVectorFields must be reset after FinishDocument.
	if c.NumVectorFields() != 0 {
		t.Fatalf("NumVectorFields = %d after FinishDocument, want 0", c.NumVectorFields())
	}
}

func TestTermVectorsConsumer_FinishDocumentFillsGap(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))

	// docID = 3 → Fill emits 3 empty docs (0..2) before the real doc.
	if err := c.FinishDocument(3); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	// 3 empty StartDocument(0) + 1 real StartDocument(1) = 4 calls.
	if len(fmt.last.startDoc) != 4 {
		t.Fatalf("StartDocument calls = %d, want 4", len(fmt.last.startDoc))
	}
	for i := 0; i < 3; i++ {
		if fmt.last.startDoc[i] != 0 {
			t.Fatalf("Fill StartDocument[%d] numFields = %d, want 0", i, fmt.last.startDoc[i])
		}
	}
	if fmt.last.startDoc[3] != 1 {
		t.Fatalf("real StartDocument numFields = %d, want 1", fmt.last.startDoc[3])
	}
	if c.LastDocID != 4 {
		t.Fatalf("LastDocID = %d, want 4", c.LastDocID)
	}
}

func TestTermVectorsConsumer_FinishDocumentDocIDMismatch(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))
	// Init the writer first so LastDocID stops being reset by Init.
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}
	c.LastDocID = 5
	// Now Fill is a no-op (5 < 0 is false) and the post-flush
	// assertion (lastDocID == docID) fails.
	if err := c.FinishDocument(0); err == nil {
		t.Fatal("expected lastDocID/docID mismatch error")
	}
}

func TestTermVectorsConsumer_FinishDocumentPropagatesPerFieldError(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	boom := errors.New("field boom")
	bad := newStubTVPerField("bad")
	bad.finishErr = boom
	c.AddFieldToFlush(bad)
	err := c.FinishDocument(0)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrap of %v", err, boom)
	}
}

func TestTermVectorsConsumer_FlushWithoutInitIsNoop(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	// Writer never installed → flush is a no-op (matches Lucene's
	// `if (writer != null)` guard).
	state := &SegmentWriteState{Directory: c.Directory, SegmentInfo: c.Info}
	if err := c.Flush(state, nil); err != nil {
		t.Fatalf("Flush no-op: %v", err)
	}
}

func TestTermVectorsConsumer_FlushFillsAndCloses(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 5)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))
	// Push a single document at docID=0; LastDocID becomes 1.
	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if c.Writer == nil || fmt.last == nil {
		t.Fatal("writer must be installed by now")
	}

	state := &SegmentWriteState{Directory: c.Directory, SegmentInfo: c.Info}
	if err := c.Flush(state, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !fmt.last.closed {
		t.Fatal("Flush must close the writer")
	}
	if c.Writer != nil {
		t.Fatal("Flush must clear the writer reference")
	}
	// numDocs=5, LastDocID was 1 after the one real doc, Fill emits 4
	// empty docs to reach numDocs=5.
	emptyAfterReal := 0
	for _, n := range fmt.last.startDoc[1:] {
		if n == 0 {
			emptyAfterReal++
		}
	}
	if emptyAfterReal != 4 {
		t.Fatalf("Fill-to-numDocs empty starts = %d, want 4", emptyAfterReal)
	}
}

func TestTermVectorsConsumer_FlushRejectsBadState(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 5)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))
	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}

	if err := c.Flush(nil, nil); err == nil {
		t.Fatal("nil state must error")
	}
	if err := c.Flush(&SegmentWriteState{}, nil); err == nil {
		t.Fatal("state with nil SegmentInfo must error")
	}

	// numDocs = 0 also rejected (matches Lucene's `assert numDocs > 0`).
	zeroDir := store.NewByteBuffersDirectory()
	zeroInfo := NewSegmentInfo("zero", 0, zeroDir)
	if err := c.Flush(&SegmentWriteState{Directory: zeroDir, SegmentInfo: zeroInfo}, nil); err == nil {
		t.Fatal("numDocs=0 must error")
	}
}

func TestTermVectorsConsumer_AbortClosesAndClears(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))
	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}

	c.Abort()
	if !fmt.last.closed {
		t.Fatal("Abort must close the writer")
	}
	if c.Writer != nil {
		t.Fatal("Abort must clear the writer ref")
	}
}

func TestTermVectorsConsumer_AbortSwallowsCloseError(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	c.SetHasVectors()
	c.AddFieldToFlush(newStubTVPerField("only"))
	if err := c.FinishDocument(0); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	fmt.last.closeErr = errors.New("close boom")
	c.Abort() // must not panic, must not propagate
	if c.Writer != nil {
		t.Fatal("Abort must clear the writer ref even on close error")
	}
}

func TestTermVectorsConsumer_AddFieldRejectsNilBuild(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	if _, err := c.AddField(nil, nil, nil); err == nil {
		t.Fatal("nil build must error")
	}
}

func TestTermVectorsConsumer_AddFieldRejectsNilHandle(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	build := func(*FieldInvertState, *FieldInfo) TermVectorsPerFieldHandle { return nil }
	if _, err := c.AddField(nil, nil, build); err == nil {
		t.Fatal("nil handle must error")
	}
}

func TestTermVectorsConsumer_AddFieldPassesInputsToBuild(t *testing.T) {
	c, _ := newTestTermVectorsConsumer(t, 1)
	want := newStubTVPerField("x")
	inv := NewFieldInvertState(10, "x", IndexOptionsDocsAndFreqs)
	fi := &FieldInfo{}
	var gotInv *FieldInvertState
	var gotFi *FieldInfo
	build := func(i *FieldInvertState, f *FieldInfo) TermVectorsPerFieldHandle {
		gotInv = i
		gotFi = f
		return want
	}
	got, err := c.AddField(inv, fi, build)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if got != want {
		t.Fatalf("AddField handle = %v, want %v", got, want)
	}
	if gotInv != inv || gotFi != fi {
		t.Fatal("AddField did not forward inputs verbatim to build")
	}
}

func TestTermVectorsConsumer_RamBytesUsedTracksWriter(t *testing.T) {
	c, fmt := newTestTermVectorsConsumer(t, 1)
	if c.RamBytesUsed() != 0 {
		t.Fatalf("RamBytesUsed = %d before init, want 0", c.RamBytesUsed())
	}
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}
	// recordingTVWriter does not implement util.Accountable, so the
	// consumer's RamBytesUsed stays at 0; this guards the nil-check
	// branch on accountable.
	if got := c.RamBytesUsed(); got != 0 {
		t.Fatalf("RamBytesUsed = %d, want 0 for non-Accountable writer", got)
	}
	_ = fmt
}

// accountableTVWriter is a recordingTVWriter that also implements
// util.Accountable so RamBytesUsed has a non-zero path to exercise.
type accountableTVWriter struct {
	recordingTVWriter
	bytes int64
}

func (a *accountableTVWriter) RamBytesUsed() int64 { return a.bytes }

type accountableTVFormat struct {
	writer *accountableTVWriter
}

func (f *accountableTVFormat) Name() string { return "acc" }
func (f *accountableTVFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	return f.writer, nil
}
func (f *accountableTVFormat) VectorsReader(_ store.Directory, _ *SegmentInfo, _ *FieldInfos, _ store.IOContext) (TermVectorsReader, error) {
	return nil, errors.New("acc: no reader")
}

func TestTermVectorsConsumer_RamBytesUsedDelegatesToAccountableWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)
	writer := &accountableTVWriter{bytes: 4242}
	codec := &recordingCodec{} // tv unused — we install via accountableTVFormat below
	_ = codec
	c := NewTermVectorsConsumer(dir, info, &accountableCodec{tv: &accountableTVFormat{writer: writer}})
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := c.RamBytesUsed(); got != 4242 {
		t.Fatalf("RamBytesUsed = %d, want 4242", got)
	}
}

// accountableCodec is the codec wiring for accountableTVFormat.
type accountableCodec struct {
	tv *accountableTVFormat
}

func (c *accountableCodec) Name() string                           { return "acc-codec" }
func (c *accountableCodec) PostingsFormat() PostingsFormat         { return nil }
func (c *accountableCodec) StoredFieldsFormat() StoredFieldsFormat { return nil }
func (c *accountableCodec) FieldInfosFormat() FieldInfosFormat     { return nil }
func (c *accountableCodec) SegmentInfosFormat() SegmentInfosFormat { return nil }
func (c *accountableCodec) SegmentInfoFormat() SegmentInfoFormat   { return nil }
func (c *accountableCodec) TermVectorsFormat() TermVectorsFormat   { return c.tv }
