// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// fakeFieldWriter is the spi.KnnFieldVectorsWriter test double returned by
// fakeKnnWriter.AddField. It records every AddValue / Finish invocation so
// tests can assert the indexing chain wired the writer correctly.
type fakeFieldWriter struct {
	name     string
	added    []addValueCall
	finished bool
	bytes    int64
}

type addValueCall struct {
	docID int
	value any
}

func (w *fakeFieldWriter) AddValue(docID int, value any) error {
	w.added = append(w.added, addValueCall{docID: docID, value: value})
	return nil
}
func (w *fakeFieldWriter) RamBytesUsed() int64 { return w.bytes }
func (w *fakeFieldWriter) Finish() error {
	w.finished = true
	return nil
}

// fakeKnnWriter is the spi.KnnVectorsWriter test double. The contract is
// identical to the production wide writer (AddField / Flush / Finish /
// Close / RamBytesUsed plus the merge-side WriteField). Tests configure
// per-method errors to exercise the consumer's try/finally surface.
type fakeKnnWriter struct {
	bytes int64

	added    []*FieldInfo
	flushed  bool
	finished bool
	closed   bool

	addErr    error
	flushErr  error
	finishErr error
	closeErr  error

	flushMaxDoc  int
	flushSortMap spi.SorterDocMap
}

func (w *fakeKnnWriter) RamBytesUsed() int64 { return w.bytes }

func (w *fakeKnnWriter) AddField(fi *FieldInfo) (spi.KnnFieldVectorsWriter, error) {
	if w.addErr != nil {
		return nil, w.addErr
	}
	w.added = append(w.added, fi)
	return &fakeFieldWriter{name: fi.Name()}, nil
}

func (w *fakeKnnWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	w.flushed = true
	w.flushMaxDoc = maxDoc
	w.flushSortMap = sortMap
	return w.flushErr
}

// WriteField is required by spi.KnnVectorsWriter for the merge path. The
// consumer tests never exercise it, so it returns a sentinel error
// matching the buffering-writer convention.
func (w *fakeKnnWriter) WriteField(fi *FieldInfo, _ spi.KnnVectorsReader) error {
	return errors.New("fakeKnnWriter: WriteField not used")
}

func (w *fakeKnnWriter) Finish() error {
	w.finished = true
	return w.finishErr
}

func (w *fakeKnnWriter) Close() error {
	w.closed = true
	return w.closeErr
}

// fakeKnnFormat is the spi.KnnVectorsFormat test double. It hands out
// the configured writer once per FieldsWriter call so tests can verify
// the consumer caches the result.
type fakeKnnFormat struct {
	writer *fakeKnnWriter
	err    error
	calls  int
	state  *SegmentWriteState
}

func (f *fakeKnnFormat) Name() string { return "fake-knn-format" }

func (f *fakeKnnFormat) FieldsWriter(state *SegmentWriteState) (spi.KnnVectorsWriter, error) {
	f.calls++
	f.state = state
	if f.err != nil {
		return nil, f.err
	}
	return f.writer, nil
}

func (f *fakeKnnFormat) FieldsReader(_ *SegmentReadState) (spi.KnnVectorsReader, error) {
	return nil, errors.New("fakeKnnFormat: FieldsReader not used")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newConsumerWithFormat(t *testing.T, f spi.KnnVectorsFormat) (*vectorValuesConsumer, *SegmentInfo) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 7, dir)
	c := newVectorValuesConsumer(nil, dir, info, util.NoOpInfoStream)
	if f != nil {
		c.setKnnVectorsFormat(f)
	}
	return c, info
}

func newFloatField(t *testing.T, name string, dim int) *FieldInfo {
	t.Helper()
	b := NewFieldInfoBuilder(name, 0)
	b.SetVectorAttributes(dim, VectorEncodingFloat32, VectorSimilarityFunctionEuclidean)
	return b.Build()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestVectorValuesConsumer_AddFieldWithoutFormatReturnsSentinel(t *testing.T) {
	c, _ := newConsumerWithFormat(t, nil)
	_, err := c.AddField(newFloatField(t, "v", 4))
	if !errors.Is(err, ErrVectorValuesConsumerNoKnnFormat) {
		t.Fatalf("want ErrVectorValuesConsumerNoKnnFormat, got %v", err)
	}
}

func TestVectorValuesConsumer_AddFieldLazyInitAndReuse(t *testing.T) {
	w := &fakeKnnWriter{bytes: 1024}
	f := &fakeKnnFormat{writer: w}
	c, info := newConsumerWithFormat(t, f)

	fi1 := newFloatField(t, "v1", 3)
	fi2 := newFloatField(t, "v2", 3)
	if _, err := c.AddField(fi1); err != nil {
		t.Fatalf("AddField v1: %v", err)
	}
	if _, err := c.AddField(fi2); err != nil {
		t.Fatalf("AddField v2: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("FieldsWriter calls = %d, want 1 (lazy + cached)", f.calls)
	}
	if len(w.added) != 2 || w.added[0] != fi1 || w.added[1] != fi2 {
		t.Fatalf("AddField propagation broken: %+v", w.added)
	}
	if f.state == nil || f.state.SegmentInfo != info || f.state.Directory != info.Directory() {
		t.Fatalf("SegmentWriteState wiring wrong: %+v", f.state)
	}
	if got := c.GetAccountable().RamBytesUsed(); got != 1024 {
		t.Fatalf("Accountable RAM = %d, want 1024", got)
	}
}

func TestVectorValuesConsumer_AccountableDefaultZero(t *testing.T) {
	c, _ := newConsumerWithFormat(t, nil)
	if got := c.GetAccountable().RamBytesUsed(); got != 0 {
		t.Fatalf("default Accountable RAM = %d, want 0", got)
	}
}

func TestVectorValuesConsumer_FlushNoWriterIsNoop(t *testing.T) {
	c, info := newConsumerWithFormat(t, nil)
	if err := c.Flush(&SegmentWriteState{SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush without writer must be no-op, got %v", err)
	}
}

func TestVectorValuesConsumer_FlushHappyPath(t *testing.T) {
	w := &fakeKnnWriter{}
	f := &fakeKnnFormat{writer: w}
	c, info := newConsumerWithFormat(t, f)
	if _, err := c.AddField(newFloatField(t, "v", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if err := c.Flush(&SegmentWriteState{SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !w.flushed || !w.finished || !w.closed {
		t.Fatalf("expected flush+finish+close, got flush=%v finish=%v close=%v", w.flushed, w.finished, w.closed)
	}
	if w.flushMaxDoc != info.DocCount() {
		t.Fatalf("flush maxDoc = %d, want %d", w.flushMaxDoc, info.DocCount())
	}
	if c.writer != nil {
		t.Fatalf("writer must be cleared after Flush")
	}
}

func TestVectorValuesConsumer_FlushPropagatesFlushErrorAndStillCloses(t *testing.T) {
	flushErr := errors.New("boom")
	w := &fakeKnnWriter{flushErr: flushErr}
	f := &fakeKnnFormat{writer: w}
	c, info := newConsumerWithFormat(t, f)
	if _, err := c.AddField(newFloatField(t, "v", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	err := c.Flush(&SegmentWriteState{SegmentInfo: info}, nil)
	if !errors.Is(err, flushErr) {
		t.Fatalf("want flush error, got %v", err)
	}
	if w.finished {
		t.Fatalf("Finish must be skipped after Flush error")
	}
	if !w.closed {
		t.Fatalf("Close must always run, even after Flush error")
	}
}

func TestVectorValuesConsumer_FlushPropagatesFinishError(t *testing.T) {
	finishErr := errors.New("finish failed")
	w := &fakeKnnWriter{finishErr: finishErr}
	f := &fakeKnnFormat{writer: w}
	c, info := newConsumerWithFormat(t, f)
	if _, err := c.AddField(newFloatField(t, "v", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if err := c.Flush(&SegmentWriteState{SegmentInfo: info}, nil); !errors.Is(err, finishErr) {
		t.Fatalf("want finish error, got %v", err)
	}
	if !w.closed {
		t.Fatalf("Close must run after Finish error")
	}
}

func TestVectorValuesConsumer_FlushRequiresSegmentInfo(t *testing.T) {
	w := &fakeKnnWriter{}
	f := &fakeKnnFormat{writer: w}
	c, _ := newConsumerWithFormat(t, f)
	if _, err := c.AddField(newFloatField(t, "v", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if err := c.Flush(nil, nil); err == nil {
		t.Fatalf("Flush(nil) must return an error")
	}
	if err := c.Flush(&SegmentWriteState{}, nil); err == nil {
		t.Fatalf("Flush without SegmentInfo must return an error")
	}
}

func TestVectorValuesConsumer_AbortClosesWriter(t *testing.T) {
	w := &fakeKnnWriter{closeErr: errors.New("close failed")}
	f := &fakeKnnFormat{writer: w}
	c, _ := newConsumerWithFormat(t, f)
	if _, err := c.AddField(newFloatField(t, "v", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	c.Abort() // must swallow the close error
	if !w.closed {
		t.Fatalf("Abort must invoke Close")
	}
	if c.writer != nil {
		t.Fatalf("writer must be cleared after Abort")
	}
	// Abort on an empty consumer must be a no-op.
	c.Abort()
}

func TestVectorValuesConsumer_InitWriterPropagatesFormatError(t *testing.T) {
	wantErr := errors.New("format down")
	f := &fakeKnnFormat{err: wantErr}
	c, _ := newConsumerWithFormat(t, f)
	_, err := c.AddField(newFloatField(t, "v", 4))
	if !errors.Is(err, wantErr) {
		t.Fatalf("want format error, got %v", err)
	}
	if c.writer != nil {
		t.Fatalf("writer must remain nil when FieldsWriter fails")
	}
}

// ---------------------------------------------------------------------------
// Codec-based KNN format resolution
// ---------------------------------------------------------------------------

// fakeCodecWithKnn is a minimal Codec stub whose KnnVectorsFormat() method
// returns the injected spi.KnnVectorsFormat. All other methods return nil,
// which is acceptable for unit tests that only exercise the KNN path.
type fakeCodecWithKnn struct {
	knn spi.KnnVectorsFormat
}

func (c *fakeCodecWithKnn) Name() string                           { return "fake-knn-codec" }
func (c *fakeCodecWithKnn) PostingsFormat() PostingsFormat         { return nil }
func (c *fakeCodecWithKnn) StoredFieldsFormat() StoredFieldsFormat { return nil }
func (c *fakeCodecWithKnn) FieldInfosFormat() FieldInfosFormat     { return nil }
func (c *fakeCodecWithKnn) SegmentInfosFormat() SegmentInfosFormat { return nil }
func (c *fakeCodecWithKnn) SegmentInfoFormat() SegmentInfoFormat   { return nil }
func (c *fakeCodecWithKnn) TermVectorsFormat() TermVectorsFormat   { return nil }
func (c *fakeCodecWithKnn) CompoundFormat() CompoundFormat         { return nil }
func (c *fakeCodecWithKnn) KnnVectorsFormat() spi.KnnVectorsFormat { return c.knn }
func (c *fakeCodecWithKnn) DocValuesFormat() spi.DocValuesFormat   { return nil }
func (c *fakeCodecWithKnn) PointsFormat() spi.PointsFormat         { return nil }

// TestVectorValuesConsumer_CodecKnnFormatUsedWhenNoExplicitFormat verifies
// that when no explicit KnnVectorsFormat is injected via
// setKnnVectorsFormat, initKnnVectorsWriter falls back to
// codec.KnnVectorsFormat(). This is the production path used by IndexWriter
// when the default codec is configured.
func TestVectorValuesConsumer_CodecKnnFormatUsedWhenNoExplicitFormat(t *testing.T) {
	w := &fakeKnnWriter{bytes: 512}
	f := &fakeKnnFormat{writer: w}
	codec := &fakeCodecWithKnn{knn: f}

	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 3, dir)
	// Construct consumer with a codec but without calling setKnnVectorsFormat.
	c := newVectorValuesConsumer(codec, dir, info, util.NoOpInfoStream)

	fi := newFloatField(t, "vec", 4)
	if _, err := c.AddField(fi); err != nil {
		t.Fatalf("AddField via codec KNN path: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("FieldsWriter should have been called once, got %d", f.calls)
	}
	if len(w.added) != 1 || w.added[0] != fi {
		t.Fatalf("AddField not forwarded to underlying writer: %+v", w.added)
	}
	// Resolved format should be cached: a second AddField must not call FieldsWriter again.
	fi2 := newFloatField(t, "vec2", 4)
	if _, err := c.AddField(fi2); err != nil {
		t.Fatalf("second AddField: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("FieldsWriter called twice; expected caching: %d calls", f.calls)
	}
}

// TestVectorValuesConsumer_CodecKnnNilFallsBackToError verifies that when the
// codec returns a nil KnnVectorsFormat, AddField still returns
// ErrVectorValuesConsumerNoKnnFormat (matching the Java IllegalStateException
// path when codec.knnVectorsFormat() is null).
func TestVectorValuesConsumer_CodecKnnNilFallsBackToError(t *testing.T) {
	codec := &fakeCodecWithKnn{knn: nil}
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 3, dir)
	c := newVectorValuesConsumer(codec, dir, info, util.NoOpInfoStream)

	_, err := c.AddField(newFloatField(t, "vec", 4))
	if !errors.Is(err, ErrVectorValuesConsumerNoKnnFormat) {
		t.Fatalf("want ErrVectorValuesConsumerNoKnnFormat, got %v", err)
	}
}

// TestVectorValuesConsumer_ExplicitFormatOverridesCodec verifies that an
// explicit setKnnVectorsFormat call takes precedence over codec.KnnVectorsFormat().
func TestVectorValuesConsumer_ExplicitFormatOverridesCodec(t *testing.T) {
	// codecKnn would be used by the codec path if not overridden.
	codecKnnW := &fakeKnnWriter{}
	codecKnn := &fakeKnnFormat{writer: codecKnnW}
	codec := &fakeCodecWithKnn{knn: codecKnn}

	// explicit format replaces the codec path.
	explicitW := &fakeKnnWriter{}
	explicitFmt := &fakeKnnFormat{writer: explicitW}

	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 3, dir)
	c := newVectorValuesConsumer(codec, dir, info, util.NoOpInfoStream)
	c.setKnnVectorsFormat(explicitFmt) // explicit override

	if _, err := c.AddField(newFloatField(t, "vec", 4)); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if explicitFmt.calls != 1 {
		t.Fatalf("explicit FieldsWriter not called: got %d calls", explicitFmt.calls)
	}
	if codecKnn.calls != 0 {
		t.Fatalf("codec FieldsWriter should not be called when explicit format is set: got %d calls", codecKnn.calls)
	}
}
