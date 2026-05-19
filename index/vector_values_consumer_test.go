// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type fakeFieldWriter struct{ name string }

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
	flushSortMap SorterDocMap
}

func (w *fakeKnnWriter) RamBytesUsed() int64 { return w.bytes }

func (w *fakeKnnWriter) AddField(fi *FieldInfo) (any, error) {
	if w.addErr != nil {
		return nil, w.addErr
	}
	w.added = append(w.added, fi)
	return &fakeFieldWriter{name: fi.Name()}, nil
}

func (w *fakeKnnWriter) Flush(maxDoc int, sortMap SorterDocMap) error {
	w.flushed = true
	w.flushMaxDoc = maxDoc
	w.flushSortMap = sortMap
	return w.flushErr
}

func (w *fakeKnnWriter) Finish() error {
	w.finished = true
	return w.finishErr
}

func (w *fakeKnnWriter) Close() error {
	w.closed = true
	return w.closeErr
}

type fakeKnnFormat struct {
	writer *fakeKnnWriter
	err    error
	calls  int
	state  *SegmentWriteState
}

func (f *fakeKnnFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsConsumerWriter, error) {
	f.calls++
	f.state = state
	if f.err != nil {
		return nil, f.err
	}
	return f.writer, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newConsumerWithFormat(t *testing.T, f KnnVectorsFormatFactory) (*vectorValuesConsumer, *SegmentInfo) {
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
