// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakeFlushWriter is a minimal in-package concrete used to drive the
// abstract base through its full lifecycle. The flusher emits a single
// token byte (the buffered length) followed by the packed values at the
// supplied bitsPerValue. It is deliberately schema-less: tests inspect
// the raw output, not its semantic meaning.
type fakeFlushWriter struct {
	abstractBlockPackedWriter
	bitsPerValue int
	flushes      int
}

func newFakeFlushWriter(out store.DataOutput, blockSize, bpv int) (*fakeFlushWriter, error) {
	w := &fakeFlushWriter{bitsPerValue: bpv}
	if err := w.init(out, blockSize, w.flush); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *fakeFlushWriter) flush() error {
	w.flushes++
	if err := w.out.WriteByte(byte(w.off)); err != nil {
		return err
	}
	if err := w.writeValues(w.bitsPerValue); err != nil {
		return err
	}
	w.off = 0
	return nil
}

func TestAbstractBlockPackedWriter_AddAndFinish(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(256)
	w, err := newFakeFlushWriter(out, 64, 8)
	if err != nil {
		t.Fatal(err)
	}
	// One full block + 3 trailing values triggers exactly two flushes.
	for i := 0; i < 67; i++ {
		if err := w.add(int64(i & 0xFF)); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if got := w.ordinal(); got != 67 {
		t.Errorf("ordinal after add: got %d want 67", got)
	}
	if err := w.finish(); err != nil {
		t.Fatal(err)
	}
	if w.flushes != 2 {
		t.Errorf("flushes: got %d want 2", w.flushes)
	}
}

func TestAbstractBlockPackedWriter_FinishOnEmptyDoesNotFlush(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(16)
	w, err := newFakeFlushWriter(out, 64, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.finish(); err != nil {
		t.Fatal(err)
	}
	if w.flushes != 0 {
		t.Errorf("flushes: got %d want 0", w.flushes)
	}
	if w.ordinal() != 0 {
		t.Errorf("ordinal: got %d want 0", w.ordinal())
	}
}

func TestAbstractBlockPackedWriter_AddAfterFinishRejected(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(16)
	w, err := newFakeFlushWriter(out, 64, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.finish(); err != nil {
		t.Fatal(err)
	}
	if err := w.add(1); err == nil {
		t.Fatal("expected error on add after finish")
	}
	if err := w.finish(); err == nil {
		t.Fatal("expected error on double finish")
	}
	if err := w.addBlockOfZeros(); err == nil {
		t.Fatal("expected error on addBlockOfZeros after finish")
	}
}

func TestAbstractBlockPackedWriter_AddBlockOfZeros(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(256)
	w, err := newFakeFlushWriter(out, 64, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.addBlockOfZeros(); err != nil {
		t.Fatal(err)
	}
	if w.ordinal() != 64 {
		t.Errorf("ordinal after block-of-zeros: got %d want 64", w.ordinal())
	}
	// A second call must flush the first block first and queue another.
	if err := w.addBlockOfZeros(); err != nil {
		t.Fatal(err)
	}
	if w.flushes != 1 {
		t.Errorf("flushes after two block-of-zeros: got %d want 1", w.flushes)
	}
	// Force a mid-block state and verify the guard fires.
	if err := w.finish(); err != nil {
		t.Fatal(err)
	}
}

func TestAbstractBlockPackedWriter_AddBlockOfZerosMidBlockRejected(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(16)
	w, err := newFakeFlushWriter(out, 64, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.add(1); err != nil {
		t.Fatal(err)
	}
	if err := w.addBlockOfZeros(); err == nil {
		t.Fatal("expected error when buffer is mid-block")
	}
}

func TestAbstractBlockPackedWriter_ResetClearsState(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(64)
	w, err := newFakeFlushWriter(out, 64, 4)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := w.add(int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.finish(); err != nil {
		t.Fatal(err)
	}
	out2 := store.NewByteArrayDataOutput(64)
	w.reset(out2)
	if w.off != 0 || w.ord != 0 || w.finished {
		t.Fatalf("reset left dirty state: off=%d ord=%d finished=%v", w.off, w.ord, w.finished)
	}
	if err := w.add(42); err != nil {
		t.Fatal(err)
	}
	if w.ordinal() != 1 {
		t.Errorf("ordinal after reset+add: got %d want 1", w.ordinal())
	}
}

func TestAbstractBlockPackedWriter_InitRejectsInvalidBlockSize(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(16)
	if _, err := newFakeFlushWriter(out, 63, 4); err == nil {
		t.Fatal("expected error for block size below minimum")
	}
	if _, err := newFakeFlushWriter(out, 1<<28, 4); err == nil {
		t.Fatal("expected error for block size above maximum")
	}
}

func TestAbstractBlockPackedWriter_InitRejectsNilFlusher(t *testing.T) {
	t.Parallel()
	w := &abstractBlockPackedWriter{}
	if err := w.init(store.NewByteArrayDataOutput(16), 64, nil); err == nil {
		t.Fatal("expected error for nil flusher")
	}
}

func TestAbstractBlockPackedWriter_ResetNilPanics(t *testing.T) {
	t.Parallel()
	w := &abstractBlockPackedWriter{}
	if err := w.init(store.NewByteArrayDataOutput(16), 64, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on reset(nil)")
		}
	}()
	w.reset(nil)
}

func TestAbstractBlockPackedWriter_FlushErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("flush boom")
	w := &abstractBlockPackedWriter{}
	if err := w.init(store.NewByteArrayDataOutput(16), 64, func() error { return sentinel }); err != nil {
		t.Fatal(err)
	}
	// Fill the buffer; the next add triggers the failing flush.
	for i := 0; i < 64; i++ {
		if err := w.add(int64(i)); err != nil {
			t.Fatalf("unexpected error before flush: %v", err)
		}
	}
	if err := w.add(0); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel from add, got %v", err)
	}
	// finish should also surface the sentinel because off > 0.
	if err := w.finish(); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel from finish, got %v", err)
	}
}

func TestAbstractWriteVLong(t *testing.T) {
	t.Parallel()
	cases := []int64{0, 1, 127, 128, 16383, 16384, -1, -1 << 32, 1 << 60}
	for _, v := range cases {
		out := store.NewByteArrayDataOutput(16)
		if err := abstractWriteVLong(out, v); err != nil {
			t.Fatalf("write %d: %v", v, err)
		}
		// blockPackedWriteVLong is the byte-identical pre-existing helper.
		out2 := store.NewByteArrayDataOutput(16)
		if err := blockPackedWriteVLong(out2, v); err != nil {
			t.Fatalf("legacy write %d: %v", v, err)
		}
		got, want := out.GetBytes(), out2.GetBytes()
		if len(got) != len(want) {
			t.Errorf("v=%d: byte length got %d want %d", v, len(got), len(want))
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("v=%d byte %d: got %#x want %#x", v, i, got[i], want[i])
			}
		}
	}
}

func TestAbstractBlockPackedWriter_ConstantsMatchLucene(t *testing.T) {
	t.Parallel()
	if abpwMinBlockSize != 64 {
		t.Errorf("abpwMinBlockSize: got %d want 64", abpwMinBlockSize)
	}
	if abpwMaxBlockSize != 1<<(30-3) {
		t.Errorf("abpwMaxBlockSize: got %d want %d", abpwMaxBlockSize, 1<<(30-3))
	}
	if abpwMinValueEqualsZero != 1 {
		t.Errorf("abpwMinValueEqualsZero: got %d want 1", abpwMinValueEqualsZero)
	}
	if abpwBpvShift != 1 {
		t.Errorf("abpwBpvShift: got %d want 1", abpwBpvShift)
	}
}
