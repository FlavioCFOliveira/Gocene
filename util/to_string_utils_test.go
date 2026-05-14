// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

// TestToStringByteArray verifies the Lucene format byte-for-byte including
// the signed-int rendering of bytes >= 0x80.
func TestToStringByteArray(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", nil, ""},
		{"single zero", []byte{0}, "b[0]=0"},
		{"three values", []byte{1, 2, 3}, "b[0]=1,b[1]=2,b[2]=3"},
		{"signed negative", []byte{0xFF, 0x80, 0x7F}, "b[0]=-1,b[1]=-128,b[2]=127"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			ToStringByteArray(&b, tc.in)
			if got := b.String(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestToStringLongHex verifies the leading-zero-preserving format including
// the all-zero and all-one edge cases.
func TestToStringLongHex(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0x0000000000000000"},
		{"one", 1, "0x0000000000000001"},
		{"ten", 0x0A, "0x000000000000000a"},
		{"max", 0x7FFFFFFFFFFFFFFF, "0x7fffffffffffffff"},
		{"min", -0x8000000000000000, "0x8000000000000000"},
		{"neg one", -1, "0xffffffffffffffff"},
		{"mid", 0x0123456789ABCDEF, "0x0123456789abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToStringLongHex(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestToStringBytesRef_NilAndEmpty verifies nil yields "null" and the
// empty-slice path produces "<empty-utf8> []" — i.e. " []".
func TestToStringBytesRef_NilAndEmpty(t *testing.T) {
	if got := ToStringBytesRef(nil); got != "null" {
		t.Fatalf("nil → %q, want %q", got, "null")
	}
	got := ToStringBytesRef(NewBytesRef(nil))
	want := " []"
	if got != want {
		t.Fatalf("empty BytesRef → %q, want %q", got, want)
	}
}

// TestToStringBytesRef_ValidUTF8 verifies the success path matches Lucene's
// "<utf8> <hex>" layout for ASCII input.
func TestToStringBytesRef_ValidUTF8(t *testing.T) {
	ref := NewBytesRef([]byte("hello"))
	got := ToStringBytesRef(ref)
	if !strings.HasPrefix(got, "hello ") {
		t.Fatalf("got %q, expected UTF-8 prefix 'hello '", got)
	}
	// Hex portion: BytesRef.String yields '[' hex... ']'.
	if !strings.Contains(got, "[") || !strings.Contains(got, "]") {
		t.Fatalf("got %q, expected hex brackets", got)
	}
}

// TestToStringBytesRef_InvalidUTF8 confirms the fallback path: only the hex
// representation is returned when the bytes are not valid UTF-8.
func TestToStringBytesRef_InvalidUTF8(t *testing.T) {
	// 0xFE / 0xFF are never valid UTF-8 lead bytes.
	ref := NewBytesRef([]byte{0xFE, 0xFF})
	got := ToStringBytesRef(ref)
	// Must equal the BytesRef's own hex dump — i.e. no leading UTF-8 segment.
	want := ref.ToHexString()
	if got != want {
		t.Fatalf("got %q, want %q (hex-only fallback)", got, want)
	}
}

// TestToStringBytesRefBuilder feeds the BytesRefBuilder overload.
func TestToStringBytesRefBuilder(t *testing.T) {
	b := NewBytesRefBuilder()
	b.CopyChars("abc")
	got := ToStringBytesRefBuilder(b)
	if !strings.HasPrefix(got, "abc ") {
		t.Fatalf("got %q, expected UTF-8 prefix 'abc '", got)
	}
	if got := ToStringBytesRefBuilder(nil); got != "null" {
		t.Fatalf("nil builder → %q, want %q", got, "null")
	}
}

// TestToStringBytes verifies the raw []byte overload routes through the
// BytesRef path.
func TestToStringBytes(t *testing.T) {
	got := ToStringBytes([]byte("xy"))
	if !strings.HasPrefix(got, "xy ") {
		t.Fatalf("got %q, expected UTF-8 prefix 'xy '", got)
	}
}

// TestIsValidUTF8 spot-checks the in-package validator with known good/bad
// sequences. The validator is exercised indirectly by ToStringBytesRef tests
// above; this guards it explicitly.
func TestIsValidUTF8(t *testing.T) {
	good := [][]byte{
		nil,
		[]byte(""),
		[]byte("ascii"),
		[]byte("café"),             // 2-byte UTF-8
		[]byte("日本語"),              // 3-byte UTF-8
		[]byte("\xF0\x9F\x98\x80"), // 4-byte UTF-8 (😀)
	}
	for i, p := range good {
		if !isValidUTF8(p) {
			t.Fatalf("good[%d] %x reported invalid", i, p)
		}
	}
	bad := [][]byte{
		{0xFF},
		{0xC0, 0x80},             // overlong NUL
		{0xED, 0xA0, 0x80},       // surrogate
		{0xF0, 0x80, 0x80, 0x80}, // overlong
		{0xC2},                   // truncated
	}
	for i, p := range bad {
		if isValidUTF8(p) {
			t.Fatalf("bad[%d] %x reported valid", i, p)
		}
	}
}
