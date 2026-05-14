// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestCharsRefBuilder_CopyUTF8Bytes_ASCII(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	src := []byte("hello")
	b.CopyUTF8Bytes(src, 0, len(src))
	if b.String() != "hello" {
		t.Errorf("CopyUTF8Bytes(ASCII) = %q, want %q", b.String(), "hello")
	}
	if b.Length() != 5 {
		t.Errorf("Length = %d, want 5", b.Length())
	}
}

func TestCharsRefBuilder_CopyUTF8Bytes_Multibyte(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	// "héllo" — é is 0xC3 0xA9 in UTF-8 (1 rune, 2 bytes).
	src := []byte("héllo")
	b.CopyUTF8Bytes(src, 0, len(src))
	if b.String() != "héllo" {
		t.Errorf("CopyUTF8Bytes(multibyte) = %q, want %q", b.String(), "héllo")
	}
	// 5 runes
	if b.Length() != 5 {
		t.Errorf("Length = %d, want 5 runes", b.Length())
	}
}

func TestCharsRefBuilder_CopyUTF8Bytes_OffsetLength(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	full := []byte("XXhelloYY")
	b.CopyUTF8Bytes(full, 2, 5)
	if b.String() != "hello" {
		t.Errorf("CopyUTF8Bytes(offset,length) = %q, want %q", b.String(), "hello")
	}
}

func TestCharsRefBuilder_CopyUTF8BytesRef(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	br := &BytesRef{Bytes: []byte("..café.."), Offset: 2, Length: 5}
	b.CopyUTF8BytesRef(br)
	if b.String() != "café" {
		t.Errorf("CopyUTF8BytesRef = %q, want %q", b.String(), "café")
	}
}

func TestCharsRefBuilder_CopyUTF8BytesRef_Nil(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	b.AppendChar('x')
	b.CopyUTF8BytesRef(nil)
	if b.Length() != 0 {
		t.Errorf("after nil, Length = %d, want 0", b.Length())
	}
}

func TestCharsRefBuilder_CopyUTF8Bytes_InvalidSequence(t *testing.T) {
	t.Parallel()

	b := NewCharsRefBuilder()
	// 0xC3 alone is the start of a 2-byte sequence; without a continuation
	// byte it must decode to U+FFFD per the standard substitution rule.
	src := []byte{0xC3}
	b.CopyUTF8Bytes(src, 0, 1)
	if b.Length() != 1 {
		t.Fatalf("Length = %d, want 1", b.Length())
	}
	if b.CharAt(0) != '�' {
		t.Errorf("CharAt(0) = %U, want U+FFFD", b.CharAt(0))
	}
}
