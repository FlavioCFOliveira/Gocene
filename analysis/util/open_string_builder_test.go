// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestOpenStringBuilder_BasicAppend verifies Append, AppendString, and String.
func TestOpenStringBuilder_BasicAppend(t *testing.T) {
	b := NewOpenStringBuilder()
	b.Append('H').Append('e').Append('l').Append('l').Append('o')
	if b.String() != "Hello" {
		t.Fatalf("got %q, want %q", b.String(), "Hello")
	}
	if b.Size() != 5 {
		t.Fatalf("Size() = %d, want 5", b.Size())
	}
}

// TestOpenStringBuilder_AppendString verifies multi-rune appends.
func TestOpenStringBuilder_AppendString(t *testing.T) {
	b := NewOpenStringBuilder()
	b.AppendString("héllo")
	if b.String() != "héllo" {
		t.Fatalf("got %q, want %q", b.String(), "héllo")
	}
}

// TestOpenStringBuilder_Grow verifies that the buffer grows beyond initial capacity.
func TestOpenStringBuilder_Grow(t *testing.T) {
	b := NewOpenStringBuilderWithSize(4)
	for i := 0; i < 100; i++ {
		b.Append(rune('a' + i%26))
	}
	if b.Size() != 100 {
		t.Fatalf("Size() = %d, want 100", b.Size())
	}
}

// TestOpenStringBuilder_Reset verifies Reset zeroes the length.
func TestOpenStringBuilder_Reset(t *testing.T) {
	b := NewOpenStringBuilder()
	b.AppendString("test")
	b.Reset()
	if b.Size() != 0 {
		t.Fatalf("after Reset, Size() = %d, want 0", b.Size())
	}
	if b.String() != "" {
		t.Fatalf("after Reset, String() = %q, want %q", b.String(), "")
	}
}

// TestOpenStringBuilder_SetRuneAt verifies in-place character replacement.
func TestOpenStringBuilder_SetRuneAt(t *testing.T) {
	b := NewOpenStringBuilder()
	b.AppendString("hello")
	b.SetRuneAt(0, 'H')
	if b.String() != "Hello" {
		t.Fatalf("got %q, want %q", b.String(), "Hello")
	}
}

// TestOpenStringBuilder_ToRuneSlice verifies a copy is returned.
func TestOpenStringBuilder_ToRuneSlice(t *testing.T) {
	b := NewOpenStringBuilder()
	b.AppendString("abc")
	s := b.ToRuneSlice()
	if string(s) != "abc" {
		t.Fatalf("ToRuneSlice = %q, want %q", string(s), "abc")
	}
	// Mutating the slice must not affect the builder.
	s[0] = 'X'
	if b.String() != "abc" {
		t.Fatalf("builder mutated after ToRuneSlice modification")
	}
}

// TestOpenStringBuilder_WriteSlice verifies partial-array writes.
func TestOpenStringBuilder_WriteSlice(t *testing.T) {
	b := NewOpenStringBuilder()
	runes := []rune("hello world")
	b.WriteSlice(runes, 6, 5) // "world"
	if b.String() != "world" {
		t.Fatalf("got %q, want %q", b.String(), "world")
	}
}

// TestOpenStringBuilder_SetLength verifies that SetLength truncates/extends.
func TestOpenStringBuilder_SetLength(t *testing.T) {
	b := NewOpenStringBuilder()
	b.AppendString("hello")
	b.SetLength(3)
	if b.String() != "hel" {
		t.Fatalf("got %q, want %q", b.String(), "hel")
	}
}
