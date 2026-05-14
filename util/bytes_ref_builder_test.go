// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestBytesRefBuilder_EmptyHasZeroLength verifies a freshly created
// builder reports length 0.
func TestBytesRefBuilder_EmptyHasZeroLength(t *testing.T) {
	b := NewBytesRefBuilder()
	if got := b.Length(); got != 0 {
		t.Errorf("Length = %d, want 0", got)
	}
	ref := b.Get()
	if ref == nil {
		t.Fatal("Get returned nil for empty builder")
	}
	if ref.Length != 0 {
		t.Errorf("Get().Length = %d, want 0", ref.Length)
	}
}

// TestBytesRefBuilder_AppendByte verifies a sequence of AppendByte
// calls produces the expected bytes.
func TestBytesRefBuilder_AppendByte(t *testing.T) {
	b := NewBytesRefBuilder()
	for _, v := range []byte{1, 2, 3, 4, 5} {
		b.AppendByte(v)
	}
	if got := b.Length(); got != 5 {
		t.Fatalf("Length = %d, want 5", got)
	}
	for i := 0; i < 5; i++ {
		if got := b.ByteAt(i); got != byte(i+1) {
			t.Errorf("ByteAt(%d) = %d, want %d", i, got, i+1)
		}
	}
}

// TestBytesRefBuilder_AppendBytes verifies the bulk append.
func TestBytesRefBuilder_AppendBytes(t *testing.T) {
	b := NewBytesRefBuilder()
	b.AppendBytes([]byte("xxhelloxx"), 2, 5)
	if b.Length() != 5 {
		t.Fatalf("Length = %d, want 5", b.Length())
	}
	got := b.Get().Utf8ToString()
	if got != "hello" {
		t.Errorf("Get().Utf8ToString = %q, want %q", got, "hello")
	}
}

// TestBytesRefBuilder_AppendBytesRef appends from a BytesRef and
// verifies the round-trip.
func TestBytesRefBuilder_AppendBytesRef(t *testing.T) {
	b := NewBytesRefBuilder()
	ref := NewBytesRef([]byte("world"))
	b.AppendBytesRef(ref)
	if b.Length() != 5 {
		t.Fatalf("Length = %d, want 5", b.Length())
	}
	if got := b.Get().Utf8ToString(); got != "world" {
		t.Errorf("Get = %q, want %q", got, "world")
	}
}

// TestBytesRefBuilder_AppendBuilder appends one builder onto another.
func TestBytesRefBuilder_AppendBuilder(t *testing.T) {
	first := NewBytesRefBuilder()
	first.CopyChars("hello")
	second := NewBytesRefBuilder()
	second.CopyChars(" world")

	first.AppendBuilder(second)
	if got := first.Get().Utf8ToString(); got != "hello world" {
		t.Errorf("AppendBuilder result = %q, want %q", got, "hello world")
	}
}

// TestBytesRefBuilder_CopyBytes verifies CopyBytes replaces the
// builder's content.
func TestBytesRefBuilder_CopyBytes(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("garbage")
	b.CopyBytes([]byte("xxhelloxx"), 2, 5)
	if got := b.Get().Utf8ToString(); got != "hello" {
		t.Errorf("CopyBytes result = %q, want %q", got, "hello")
	}
}

// TestBytesRefBuilder_Clear keeps storage and resets length to 0.
func TestBytesRefBuilder_Clear(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("hello world")
	b.Clear()
	if b.Length() != 0 {
		t.Errorf("after Clear, Length = %d, want 0", b.Length())
	}
}

// TestBytesRefBuilder_ToBytesRefIndependent verifies the BytesRef
// returned by ToBytesRef has its own backing slice.
func TestBytesRefBuilder_ToBytesRefIndependent(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("hello")
	ref := b.ToBytesRef()

	// Mutate the builder; the previously returned ref must not change.
	b.SetByteAt(0, 'X')
	if ref.Bytes[0] != 'h' {
		t.Errorf("ToBytesRef result not independent: ref.Bytes[0] = %c", ref.Bytes[0])
	}
}

// TestBytesRefBuilder_GetSharesUnderlyingBytes verifies Get returns a
// BytesRef referencing the same storage (Lucene contract).
func TestBytesRefBuilder_GetSharesUnderlyingBytes(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("hello")
	ref := b.Get()
	b.SetByteAt(0, 'X')
	if ref.Bytes[0] != 'X' {
		t.Errorf("Get should share storage with builder, ref.Bytes[0] = %c", ref.Bytes[0])
	}
}

// TestBytesRefBuilder_String returns the Lucene hex format.
func TestBytesRefBuilder_String(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("ab")
	want := "[61 62]"
	if got := b.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

// TestBytesRefBuilder_SetByteAt verifies direct byte assignment.
func TestBytesRefBuilder_SetByteAt(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("hello")
	b.SetByteAt(0, 'H')
	if got := b.ByteAt(0); got != 'H' {
		t.Errorf("ByteAt(0) = %c, want H", got)
	}
}
