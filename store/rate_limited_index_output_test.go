// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestRateLimiter.java
// covers RateLimitedIndexOutput indirectly; the tests below validate
// throttle invocation, threshold refresh, and forwarding semantics directly.

package store

import (
	"sync/atomic"
	"testing"
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
	*BaseIndexOutput
	buf []byte
}

func (c *capturingOutput) WriteByte(b byte) error    { c.buf = append(c.buf, b); return nil }
func (c *capturingOutput) WriteBytes(b []byte) error { c.buf = append(c.buf, b...); return nil }
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

func TestRateLimitedIndexOutput_PausesAtThreshold(t *testing.T) {
	rl := &recordingRateLimiter{mbPerSec: 1.0, minPauseBytes: 16}
	wrapped := &capturingOutput{BaseIndexOutput: NewBaseIndexOutput("test")}
	out := NewRateLimitedIndexOutput(rl, wrapped)
	if err := out.WriteBytes(make([]byte, 32)); err != nil {
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
	wrapped := &capturingOutput{BaseIndexOutput: NewBaseIndexOutput("test")}
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
	wrapped := &capturingOutput{BaseIndexOutput: NewBaseIndexOutput("test")}
	out := NewRateLimitedIndexOutput(rl, wrapped)
	data := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if err := out.WriteBytes(data); err != nil {
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
