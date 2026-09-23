// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestRateLimiter.java
// covers RateLimitedIndexOutput indirectly; the tests below validate
// throttle invocation, threshold refresh, and forwarding semantics directly.

package store

import (
	"errors"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// recordingRateLimiter is a RateLimiter that records every Pause call.
type recordingRateLimiter struct {
	pauses        atomic.Int32
	pausedBytes   atomic.Int64
	mbPerSec      float64
	minPauseBytes int64
}

func (r *recordingRateLimiter) GetMBPerSec() float64         { return r.mbPerSec }
func (r *recordingRateLimiter) SetMBPerSec(v float64)        { r.mbPerSec = v }
func (r *recordingRateLimiter) GetMinPauseCheckBytes() int64 { return r.minPauseBytes }
func (r *recordingRateLimiter) Pause(bytes int64) int64 {
	r.pauses.Add(1)
	r.pausedBytes.Add(bytes)
	return 0
}

type capturingOutput struct {
	*spi.BaseIndexOutput
	buf []byte
}

func (c *capturingOutput) WriteByte(b byte) error { c.buf = append(c.buf, b); return nil }
func (c *capturingOutput) WriteBytes(b []byte, _ int, _ int) error {
	c.buf = append(c.buf, b...)
	return nil
}
func (c *capturingOutput) WriteBytesN(b []byte, n int) error {
	c.buf = append(c.buf, b[:n]...)
	return nil
}
func (c *capturingOutput) WriteShort(int16) error   { return nil }
func (c *capturingOutput) WriteInt(int32) error     { return nil }
func (c *capturingOutput) WriteLong(int64) error    { return nil }
func (c *capturingOutput) WriteString(string) error { return nil }
func (c *capturingOutput) Close() error             { return nil }
func (c *capturingOutput) SetPosition(int64) error  { return nil }
func (c *capturingOutput) Length() int64            { return int64(len(c.buf)) }

// CopyBytes carries the default body Lucene gives IndexOutput.CopyBytes.
func (c *capturingOutput) CopyBytes(input spi.DataInput, numBytes int64) error {
	buf := make([]byte, 16384)
	for left := numBytes; left > 0; {
		n := len(buf)
		if left < int64(n) {
			n = int(left)
		}
		if err := input.ReadBytes(buf, 0, n); err != nil {
			return err
		}
		if err := c.WriteBytes(buf, 0, n); err != nil {
			return err
		}
		left -= int64(n)
	}
	return nil
}

// WriteGroupVInts is abstract in Lucene's IndexOutput; this double does not support it.
func (c *capturingOutput) WriteGroupVInts(values []int32, limit int) error {
	return errors.New("capturingOutput.WriteGroupVInts: unsupported operation")
}

// WriteMapOfStrings carries the default body Lucene gives IndexOutput.WriteMapOfStrings.
func (c *capturingOutput) WriteMapOfStrings(m map[string]string) error {
	if err := c.WriteVInt(int32(len(m))); err != nil {
		return err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := c.WriteString(k); err != nil {
			return err
		}
		if err := c.WriteString(m[k]); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetOfStrings carries the default body Lucene gives IndexOutput.WriteSetOfStrings.
func (c *capturingOutput) WriteSetOfStrings(s []string) error {
	if err := c.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	for _, v := range s {
		if err := c.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

// WriteVInt carries the default body Lucene gives IndexOutput.WriteVInt.
func (c *capturingOutput) WriteVInt(i int32) error {
	v := uint32(i)
	for v >= 0x80 {
		if err := c.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return c.WriteByte(byte(v))
}

// WriteVLong carries the default body Lucene gives IndexOutput.WriteVLong.
func (c *capturingOutput) WriteVLong(i int64) error {
	v := uint64(i)
	for v >= 0x80 {
		if err := c.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return c.WriteByte(byte(v))
}

// WriteZInt carries the default body Lucene gives IndexOutput.WriteZInt.
func (c *capturingOutput) WriteZInt(i int32) error {
	return c.WriteVInt((i >> 31) ^ (i << 1))
}

// WriteZLong carries the default body Lucene gives IndexOutput.WriteZLong.
func (c *capturingOutput) WriteZLong(i int64) error {
	return c.WriteVLong((i >> 63) ^ (i << 1))
}

func TestRateLimitedIndexOutput_PausesAtThreshold(t *testing.T) {
	rl := &recordingRateLimiter{mbPerSec: 1.0, minPauseBytes: 16}
	wrapped := &capturingOutput{BaseIndexOutput: spi.NewBaseIndexOutput("test")}
	out := NewRateLimitedIndexOutput(rl, wrapped)
	if err := out.WriteBytes(make([]byte, 32), 0, len(make([]byte, 32))); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if rl.pauses.Load() != 1 {
		t.Fatalf("expected 1 pause after exceeding threshold, got %d", rl.pauses.Load())
	}
	if rl.pausedBytes.Load() != 32 {
		t.Fatalf("expected 32 bytes paused, got %d", rl.pausedBytes.Load())
	}
}

func TestRateLimitedIndexOutput_NoPauseBelowThreshold(t *testing.T) {
	rl := &recordingRateLimiter{mbPerSec: 1.0, minPauseBytes: 100}
	wrapped := &capturingOutput{BaseIndexOutput: spi.NewBaseIndexOutput("test")}
	out := NewRateLimitedIndexOutput(rl, wrapped)
	for i := 0; i < 10; i++ {
		if err := out.WriteByte(byte(i)); err != nil {
			t.Fatalf("WriteByte: %v", err)
		}
	}
	if rl.pauses.Load() != 0 {
		t.Fatalf("expected no pauses, got %d", rl.pauses.Load())
	}
}

func TestRateLimitedIndexOutput_DataForwarded(t *testing.T) {
	rl := &recordingRateLimiter{mbPerSec: 1.0, minPauseBytes: 1 << 30}
	wrapped := &capturingOutput{BaseIndexOutput: spi.NewBaseIndexOutput("test")}
	out := NewRateLimitedIndexOutput(rl, wrapped)
	data := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if err := out.WriteBytes(data, 0, len(data)); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if len(wrapped.buf) != 4 {
		t.Fatalf("wrapped buf len = %d, want 4", len(wrapped.buf))
	}
	for i, b := range wrapped.buf {
		if b != data[i] {
			t.Fatalf("wrapped buf[%d] = %#x, want %#x", i, b, data[i])
		}
	}
}
