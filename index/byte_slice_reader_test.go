// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// sliceLevelSizes mirrors ByteSlicePool.LEVEL_SIZE_ARRAY (used by the test writer).
var sliceLevelSizes = [...]int{5, 14, 20, 30, 40, 40, 80, 80, 120, 200}

// sliceNextLevel mirrors ByteSlicePool.NEXT_LEVEL_ARRAY (used by the test writer).
var sliceNextLevel = [...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 9}

// testSliceWriter is a minimal port of ByteSlicePool sufficient to lay out
// chained slices inside a ByteBlockPool so ByteSliceReader can be exercised.
// It mirrors newSlice and allocSlice from Lucene's ByteSlicePool.
type testSliceWriter struct {
	pool *util.ByteBlockPool

	currentBuf   []byte
	currentLevel int
	currentUpto  int // local upto inside currentBuf
	currentLimit int // local end of slice inside currentBuf (sentinel index + 1)
	written      int // total bytes written into the chain (payload only)

	startOffset int // absolute start of the very first slice
	endOffset   int // absolute end (== bytes written + final consumed sentinels)
}

// newTestSliceWriter starts a fresh chain at the current pool position.
func newTestSliceWriter(t *testing.T, pool *util.ByteBlockPool) *testSliceWriter {
	t.Helper()
	if pool.Buffer == nil {
		pool.NextBuffer()
	}
	size := sliceLevelSizes[0]
	if pool.ByteUpto > util.ByteBlockSize-size {
		pool.NextBuffer()
	}
	upto := pool.ByteUpto
	pool.ByteUpto += size
	pool.Buffer[pool.ByteUpto-1] = 16 // level-0 sentinel

	w := &testSliceWriter{
		pool:         pool,
		currentBuf:   pool.Buffer,
		currentLevel: 0,
		currentUpto:  upto,
		currentLimit: upto + size,
		startOffset:  pool.ByteOffset + upto,
	}
	return w
}

// allocNext allocates a continuation slice and rewrites the trailing 4 bytes of
// the current slice as a forwarding address. Mirrors ByteSlicePool.allocSlice.
func (w *testSliceWriter) allocNext() {
	level := int(w.currentBuf[w.currentLimit-1] & 15)
	newLevel := sliceNextLevel[level]
	newSize := sliceLevelSizes[newLevel]

	if w.pool.ByteUpto > util.ByteBlockSize-newSize {
		w.pool.NextBuffer()
	}
	newUpto := w.pool.ByteUpto
	offset := newUpto + w.pool.ByteOffset
	w.pool.ByteUpto += newSize

	// Past 3 bytes preceding the sentinel position; preserve them at the head
	// of the new slice. Equivalent to BitUtil.VH_LE_INT.get(slice, upto - 3)
	// masked to 24 bits.
	past3 := uint32(w.currentBuf[w.currentLimit-4]) |
		uint32(w.currentBuf[w.currentLimit-3])<<8 |
		uint32(w.currentBuf[w.currentLimit-2])<<16
	binary.LittleEndian.PutUint32(w.pool.Buffer[newUpto:newUpto+4], past3)

	// Forwarding address occupies the last 4 bytes of the old slice
	// (overwriting positions limit-4..limit-1).
	binary.LittleEndian.PutUint32(w.currentBuf[w.currentLimit-4:w.currentLimit], uint32(offset))

	// Level sentinel for the new slice.
	w.pool.Buffer[w.pool.ByteUpto-1] = byte(16 | newLevel)

	w.currentBuf = w.pool.Buffer
	w.currentLevel = newLevel
	// First 3 bytes already hold the carried-over payload.
	w.currentUpto = newUpto + 3
	w.currentLimit = newUpto + newSize
}

// writeByte appends one byte to the chain, hopping to a new slice when needed.
func (w *testSliceWriter) writeByte(b byte) {
	if w.currentBuf[w.currentUpto] != 0 {
		w.allocNext()
	}
	w.currentBuf[w.currentUpto] = b
	w.currentUpto++
	w.written++
}

// writeBytes appends bs to the chain.
func (w *testSliceWriter) writeBytes(bs []byte) {
	for _, b := range bs {
		w.writeByte(b)
	}
}

// Finish computes the absolute end offset for the reader (one past the last
// payload byte written).
func (w *testSliceWriter) Finish() (start, end int) {
	w.endOffset = w.pool.ByteOffset + w.currentUpto
	return w.startOffset, w.endOffset
}

func newTestPool() *util.ByteBlockPool {
	return util.NewByteBlockPool(util.NewDirectAllocator())
}

func TestByteSliceReader_SingleSliceRoundTrip(t *testing.T) {
	t.Parallel()

	payload := []byte("lucene-to-go")
	pool := newTestPool()
	w := newTestSliceWriter(t, pool)
	w.writeBytes(payload)
	start, end := w.Finish()

	r := &index.ByteSliceReader{}
	if err := r.Init(pool, start, end); err != nil {
		t.Fatalf("Init: %v", err)
	}
	got := make([]byte, len(payload))
	if err := r.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
	if !r.EOF() {
		t.Fatalf("expected EOF after consuming payload")
	}
}

func TestByteSliceReader_MultiSliceByteByByte(t *testing.T) {
	t.Parallel()

	// Far exceeds the first level (5 bytes) and chains through several levels.
	payload := make([]byte, 500)
	rng := rand.New(rand.NewSource(0xBEEF))
	for i := range payload {
		// Avoid zero bytes which would clash with the slice-end sentinel
		// detection in the writer (writer expects 0 == free, non-zero == end).
		payload[i] = byte(1 + rng.Intn(255))
	}

	pool := newTestPool()
	w := newTestSliceWriter(t, pool)
	w.writeBytes(payload)
	start, end := w.Finish()

	r := &index.ByteSliceReader{}
	if err := r.Init(pool, start, end); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i, want := range payload {
		got, err := r.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte[%d]: %v", i, err)
		}
		if got != want {
			t.Fatalf("byte[%d]: got %#x want %#x", i, got, want)
		}
	}
	if !r.EOF() {
		t.Fatalf("expected EOF after consuming all bytes")
	}
	if _, err := r.ReadByte(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadByte past EOF: got %v want io.EOF", err)
	}
}

func TestByteSliceReader_WriteToCopiesAllBytes(t *testing.T) {
	t.Parallel()

	payload := make([]byte, 1024)
	rng := rand.New(rand.NewSource(0xC0FFEE))
	for i := range payload {
		payload[i] = byte(1 + rng.Intn(255))
	}

	pool := newTestPool()
	w := newTestSliceWriter(t, pool)
	w.writeBytes(payload)
	start, end := w.Finish()

	r := &index.ByteSliceReader{}
	if err := r.Init(pool, start, end); err != nil {
		t.Fatalf("Init: %v", err)
	}

	sink := &capturingOutput{}
	n, err := r.WriteTo(sink)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("WriteTo size: got %d want %d", n, len(payload))
	}
	if !bytes.Equal(sink.buf, payload) {
		t.Fatalf("WriteTo output mismatch")
	}
}

// capturingOutput satisfies store.DataOutput.WriteBytes for round-trip tests.
// Other DataOutput methods are intentionally unused by ByteSliceReader.WriteTo.
type capturingOutput struct {
	buf []byte
}

func (c *capturingOutput) WriteByte(b byte) error {
	c.buf = append(c.buf, b)
	return nil
}
func (c *capturingOutput) WriteBytes(b []byte) error { c.buf = append(c.buf, b...); return nil }
func (c *capturingOutput) WriteBytesN(b []byte, n int) error {
	c.buf = append(c.buf, b[:n]...)
	return nil
}
func (c *capturingOutput) WriteShort(int16) error   { return errors.New("unused") }
func (c *capturingOutput) WriteInt(int32) error     { return errors.New("unused") }
func (c *capturingOutput) WriteLong(int64) error    { return errors.New("unused") }
func (c *capturingOutput) WriteString(string) error { return errors.New("unused") }

func TestByteSliceReader_SkipBytes(t *testing.T) {
	t.Parallel()

	payload := make([]byte, 300)
	for i := range payload {
		payload[i] = byte((i % 254) + 1)
	}
	pool := newTestPool()
	w := newTestSliceWriter(t, pool)
	w.writeBytes(payload)
	start, end := w.Finish()

	r := &index.ByteSliceReader{}
	if err := r.Init(pool, start, end); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := r.SkipBytes(100); err != nil {
		t.Fatalf("SkipBytes(100): %v", err)
	}
	got := make([]byte, 50)
	if err := r.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if !bytes.Equal(got, payload[100:150]) {
		t.Fatalf("post-skip mismatch: got %v want %v", got, payload[100:150])
	}

	if err := r.SkipBytes(-1); err == nil {
		t.Fatalf("SkipBytes(-1) should error")
	}
}

func TestByteSliceReader_InitValidation(t *testing.T) {
	t.Parallel()

	pool := newTestPool()
	pool.NextBuffer()
	r := &index.ByteSliceReader{}

	cases := []struct {
		name        string
		start, end  int
		wantErrPart string
	}{
		{"negative start", -1, 10, "startIndex"},
		{"negative end", 0, -1, "endIndex"},
		{"end before start", 10, 5, "endIndex"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := r.Init(pool, tc.start, tc.end)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

var _ store.DataOutput = (*capturingOutput)(nil)
