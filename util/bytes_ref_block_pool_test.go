// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"strings"
	"testing"
)

// TestBytesRefBlockPool_EncodingShortLength verifies the single-byte length
// prefix used for terms < 128 bytes. Java writes the raw length byte, then
// the term bytes. Go must produce the same on-disk layout.
func TestBytesRefBlockPool_EncodingShortLength(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	term := []byte("lucene")
	off, err := rp.AddBytesRef(NewBytesRef(term))
	if err != nil {
		t.Fatalf("AddBytesRef: %v", err)
	}
	// First byte must equal len(term) with the high bit clear.
	if got := bp.ReadByteAt(int64(off)); got != byte(len(term)) {
		t.Fatalf("length prefix = 0x%X, want 0x%X", got, len(term))
	}
	// Bytes 1.. must match the term.
	buf := make([]byte, len(term))
	bp.ReadBytes(int64(off+1), buf, 0, len(term))
	if !bytes.Equal(buf, term) {
		t.Fatalf("payload = %x, want %x", buf, term)
	}
}

// TestBytesRefBlockPool_EncodingLongLength verifies the 2-byte big-endian
// length prefix used for terms >= 128 bytes. Java emits a single big-endian
// 16-bit value (length | 0x8000); Go writes two bytes manually but must
// produce the same byte stream.
func TestBytesRefBlockPool_EncodingLongLength(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	term := bytes.Repeat([]byte{0x41}, 200) // 0xC8 > 127
	off, err := rp.AddBytesRef(NewBytesRef(term))
	if err != nil {
		t.Fatalf("AddBytesRef: %v", err)
	}
	first := bp.ReadByteAt(int64(off))
	second := bp.ReadByteAt(int64(off + 1))
	// Java big-endian encoding of (200 | 0x8000) = 0x80C8:
	//   high byte = 0x80, low byte = 0xC8.
	if first != 0x80 {
		t.Fatalf("high byte = 0x%X, want 0x80", first)
	}
	if second != 0xC8 {
		t.Fatalf("low byte = 0x%X, want 0xC8", second)
	}
	buf := make([]byte, 200)
	bp.ReadBytes(int64(off+2), buf, 0, 200)
	if !bytes.Equal(buf, term) {
		t.Fatalf("payload differs")
	}
}

// TestBytesRefBlockPool_RoundTrip verifies AddBytesRef → FillBytesRef
// preserves the value across both length-prefix regimes.
func TestBytesRefBlockPool_RoundTrip(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	cases := []string{
		"",
		"a",
		"hello",
		strings.Repeat("x", 127), // boundary just below 1-byte prefix
		strings.Repeat("y", 128), // boundary at 2-byte prefix
		strings.Repeat("z", 1000),
	}
	offsets := make([]int, len(cases))
	for i, s := range cases {
		off, err := rp.AddBytesRef(NewBytesRef([]byte(s)))
		if err != nil {
			t.Fatalf("AddBytesRef(%q): %v", s, err)
		}
		offsets[i] = off
	}
	for i, s := range cases {
		var ref BytesRef
		rp.FillBytesRef(&ref, offsets[i])
		got := string(ref.Bytes[ref.Offset : ref.Offset+ref.Length])
		if got != s {
			t.Fatalf("round-trip[%d]: got %q, want %q", i, got, s)
		}
	}
}

// TestBytesRefBlockPool_Equals_AcrossLengths ensures Equals matches/rejects
// both length-prefix paths.
func TestBytesRefBlockPool_Equals_AcrossLengths(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	for _, term := range []string{"x", strings.Repeat("q", 250)} {
		off, err := rp.AddBytesRef(NewBytesRef([]byte(term)))
		if err != nil {
			t.Fatalf("AddBytesRef: %v", err)
		}
		if !rp.Equals(off, NewBytesRef([]byte(term))) {
			t.Fatalf("Equals must accept identical term %q", term)
		}
		if rp.Equals(off, NewBytesRef([]byte(term+"."))) {
			t.Fatalf("Equals must reject extended term")
		}
	}
}

// TestBytesRefBlockPool_Hash returns a non-zero hash for non-empty input
// and the hashes for distinct terms must differ for a reasonably-sized
// sample (this is a smoke test of the underlying murmur hash).
func TestBytesRefBlockPool_Hash(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	seen := make(map[int]string)
	for _, s := range []string{"abc", "xyz", "hello", "world", "lucene", "gocene"} {
		off, err := rp.AddBytesRef(NewBytesRef([]byte(s)))
		if err != nil {
			t.Fatalf("AddBytesRef: %v", err)
		}
		h := rp.Hash(off)
		if prev, ok := seen[h]; ok && prev != s {
			// Collisions are theoretically possible but extremely unlikely
			// for this tiny vocabulary.
			t.Fatalf("hash collision: %q and %q both produce 0x%X", prev, s, h)
		}
		seen[h] = s
	}
}

// TestBytesRefBlockPool_Reset returns the pool to a usable state.
func TestBytesRefBlockPool_Reset(t *testing.T) {
	bp := NewByteBlockPool(NewDirectAllocator())
	bp.NextBuffer()
	rp := NewBytesRefBlockPool(bp)
	_, _ = rp.AddBytesRef(NewBytesRef([]byte("first")))
	rp.Reset()
	bp.NextBuffer()
	if _, err := rp.AddBytesRef(NewBytesRef([]byte("after-reset"))); err != nil {
		t.Fatalf("AddBytesRef post-reset: %v", err)
	}
}
