// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand/v2"
	"testing"
	"unicode/utf16"
)

func bytesRefFrom(b []byte) *BytesRef {
	return &BytesRef{Bytes: b, Offset: 0, Length: len(b)}
}

// TestUnicodeUtil_CodePointCount_Invalid mirrors the throwing branch of
// TestUnicodeUtil#testCodePointCount.
func TestUnicodeUtil_CodePointCount_Invalid(t *testing.T) {
	cases := [][]byte{
		{'z', 0x80, 'z', 'z', 'z'},
		{'z', 0xC0 - 1, 'z', 'z', 'z'},
		{'z', 0xF8, 'z', 'z', 'z'},
		{'z', 0xFC, 'z', 'z', 'z'},
	}
	for _, c := range cases {
		func(c []byte) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for %x", c)
				}
			}()
			_ = CodePointCount(bytesRefFrom(c))
		}(c)
	}
}

// TestUnicodeUtil_CodePointCount_Examples mirrors the success-path
// assertions of testCodePointCount.
func TestUnicodeUtil_CodePointCount_Examples(t *testing.T) {
	cases := []struct {
		in   []byte
		want int
	}{
		{[]byte{}, 0},
		{[]byte{'z', 'z', 'z'}, 3},
		{[]byte{'z', 0xC2, 0xA2}, 2},
		{[]byte{'z', 0xE2, 0x82, 0xAC}, 2},
		{[]byte{'z', 0xF0, 0xA4, 0xAD, 0xA2}, 2},
	}
	for _, c := range cases {
		if got := CodePointCount(bytesRefFrom(c.in)); got != c.want {
			t.Fatalf("CodePointCount(%x) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestUnicodeUtil_UTF16ToUTF8_Vectors verifies byte-for-byte parity with
// the Lucene reference for representative inputs across the four UTF-8
// sequence lengths plus a lone-surrogate fixture.
func TestUnicodeUtil_UTF16ToUTF8_Vectors(t *testing.T) {
	cases := []struct {
		name  string
		units []uint16
		want  []byte
	}{
		{"ascii", []uint16{'A', 'B', 'C'}, []byte{0x41, 0x42, 0x43}},
		{"two-byte", []uint16{0x00A2}, []byte{0xC2, 0xA2}},               // ¢
		{"three-byte", []uint16{0x20AC}, []byte{0xE2, 0x82, 0xAC}},       // €
		{"four-byte", []uint16{0xD83D, 0xDE00}, []byte{0xF0, 0x9F, 0x98, 0x80}}, // 😀
		{"lone high", []uint16{0xD83D}, []byte{0xEF, 0xBF, 0xBD}},
		{"lone low", []uint16{0xDC00}, []byte{0xEF, 0xBF, 0xBD}},
		{"out-of-order pair", []uint16{0xDC00, 0xD83D}, []byte{0xEF, 0xBF, 0xBD, 0xEF, 0xBF, 0xBD}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := make([]byte, MaxUTF8Length(len(c.units)))
			n := UTF16ToUTF8Chars(c.units, 0, len(c.units), out)
			got := out[:n]
			if !bytes.Equal(got, c.want) {
				t.Fatalf("got %x, want %x", got, c.want)
			}
		})
	}
}

// TestUnicodeUtil_UTF8ToUTF16_Vectors validates the reverse transformation,
// crucially including the supplementary path that re-emits a surrogate pair.
func TestUnicodeUtil_UTF8ToUTF16_Vectors(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []uint16
	}{
		{"ascii", []byte{'A', 'B', 'C'}, []uint16{'A', 'B', 'C'}},
		{"two-byte", []byte{0xC2, 0xA2}, []uint16{0x00A2}},
		{"three-byte", []byte{0xE2, 0x82, 0xAC}, []uint16{0x20AC}},
		{"four-byte", []byte{0xF0, 0x9F, 0x98, 0x80}, []uint16{0xD83D, 0xDE00}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := make([]uint16, len(c.in)+1)
			n := UTF8ToUTF16(c.in, 0, len(c.in), out)
			got := out[:n]
			if len(got) != len(c.want) {
				t.Fatalf("len = %d, want %d (got %v want %v)", len(got), len(c.want), got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("unit[%d] = 0x%X, want 0x%X", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestUnicodeUtil_RoundTrip verifies UTF-16 → UTF-8 → UTF-16 is the identity
// on well-formed inputs, against Go's stdlib utf16 reference.
func TestUnicodeUtil_RoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for trial := 0; trial < 1000; trial++ {
		n := r.IntN(20) + 1
		runes := make([]rune, n)
		for i := range runes {
			// Mix of BMP and supplementary code points; skip surrogate codes
			// because Go's UTF-8 decoder rejects them.
			var cp int
			for {
				cp = r.IntN(0x10FFFF)
				if cp < 0xD800 || cp > 0xDFFF {
					break
				}
			}
			runes[i] = rune(cp)
		}
		units := utf16.Encode(runes)
		out := make([]byte, MaxUTF8Length(len(units)))
		n8 := UTF16ToUTF8Chars(units, 0, len(units), out)
		// Reverse.
		decoded := make([]uint16, len(units)*2)
		n16 := UTF8ToUTF16(out, 0, n8, decoded)
		got := decoded[:n16]
		if len(got) != len(units) {
			t.Fatalf("trial %d: round-trip length differs: %d → %d", trial, len(units), len(got))
		}
		for i := range units {
			if got[i] != units[i] {
				t.Fatalf("trial %d: unit[%d] = 0x%X, want 0x%X", trial, i, got[i], units[i])
			}
		}
	}
}

// TestUnicodeUtil_CodePointAt walks a UTF-8 byte stream and confirms
// codePoint/numBytes for each step.
func TestUnicodeUtil_CodePointAt(t *testing.T) {
	in := []byte("Az¢€\U0001F600")
	want := []struct {
		cp int
		nb int
	}{
		{'A', 1}, {'z', 1}, {0xA2, 2}, {0x20AC, 3}, {0x1F600, 4},
	}
	var reuse *UTF8CodePoint
	pos := 0
	idx := 0
	for pos < len(in) {
		reuse = CodePointAt(in, pos, reuse)
		if reuse.CodePoint != want[idx].cp || reuse.NumBytes != want[idx].nb {
			t.Fatalf("step %d: got (cp=0x%X, nb=%d), want (cp=0x%X, nb=%d)",
				idx, reuse.CodePoint, reuse.NumBytes, want[idx].cp, want[idx].nb)
		}
		pos += reuse.NumBytes
		idx++
	}
	if idx != len(want) {
		t.Fatalf("decoded %d code points, want %d", idx, len(want))
	}
}

// TestUnicodeUtil_UTF8ToUTF32 mirrors testUTF8toUTF32: encode then decode
// returns the original code-point sequence.
func TestUnicodeUtil_UTF8ToUTF32(t *testing.T) {
	in := "abç€😀"
	units := stringToUTF16(in)
	out := make([]byte, MaxUTF8Length(len(units)))
	n := UTF16ToUTF8Chars(units, 0, len(units), out)
	utf32 := make([]int, n)
	count := UTF8ToUTF32(bytesRefFrom(out[:n]), utf32)
	want := []rune{'a', 'b', 0xE7, 0x20AC, 0x1F600}
	if count != len(want) {
		t.Fatalf("count = %d, want %d", count, len(want))
	}
	for i, r := range want {
		if utf32[i] != int(r) {
			t.Fatalf("utf32[%d] = 0x%X, want 0x%X", i, utf32[i], r)
		}
	}
}

// TestUnicodeUtil_CalcUTF16ToUTF8Length verifies the precomputed length
// matches the actual encoding length, mirroring testCalcUTF16toUTF8Length.
func TestUnicodeUtil_CalcUTF16ToUTF8Length(t *testing.T) {
	r := rand.New(rand.NewPCG(0xA, 0xB))
	for trial := 0; trial < 500; trial++ {
		n := r.IntN(30)
		runes := make([]rune, n)
		for i := range runes {
			for {
				cp := r.IntN(0x10FFFF)
				if cp >= 0xD800 && cp <= 0xDFFF {
					continue
				}
				runes[i] = rune(cp)
				break
			}
		}
		s := string(runes)
		units := stringToUTF16(s)
		out := make([]byte, MaxUTF8Length(len(units)))
		n8 := UTF16ToUTF8Chars(units, 0, len(units), out)
		got := CalcUTF16toUTF8Length(s, 0, len(units))
		if got != n8 {
			t.Fatalf("CalcUTF16toUTF8Length=%d, actual=%d", got, n8)
		}
	}
}

// TestUnicodeUtil_ValidUTF16String covers the success/failure paths.
func TestUnicodeUtil_ValidUTF16String(t *testing.T) {
	if !ValidUTF16Chars([]uint16{'A', 'B'}, 2) {
		t.Fatalf("ascii must be valid")
	}
	if !ValidUTF16Chars([]uint16{0xD83D, 0xDE00}, 2) {
		t.Fatalf("valid surrogate pair must be valid")
	}
	if ValidUTF16Chars([]uint16{0xD83D}, 1) {
		t.Fatalf("lone high must be invalid")
	}
	if ValidUTF16Chars([]uint16{0xDE00}, 1) {
		t.Fatalf("lone low must be invalid")
	}
	if ValidUTF16Chars([]uint16{0xD83D, 'A'}, 2) {
		t.Fatalf("unmatched high must be invalid")
	}
}

// TestUnicodeUtil_NewStringFromCodePoints mirrors a couple of the cases in
// testNewString that don't depend on Java-specific surrogate semantics.
func TestUnicodeUtil_NewStringFromCodePoints(t *testing.T) {
	cps := []int{'h', 'e', 'l', 'l', 'o'}
	got := NewStringFromCodePoints(cps, 0, len(cps))
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	cps = []int{0x1F600}
	got = NewStringFromCodePoints(cps, 0, 1)
	if got != "\U0001F600" {
		t.Fatalf("got %q, want %q", got, "\U0001F600")
	}
}

// TestUnicodeUtil_NewStringFromCodePoints_Invalid checks bounds and code-point
// validation.
func TestUnicodeUtil_NewStringFromCodePoints_Invalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for negative count")
		}
	}()
	_ = NewStringFromCodePoints([]int{0}, 0, -1)
}

// TestUnicodeUtil_MaxUTF8Length_Overflow exercises the overflow guard.
func TestUnicodeUtil_MaxUTF8Length_Overflow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on overflow")
		}
	}()
	_ = MaxUTF8Length(1<<30 + 1)
}

// TestUnicodeUtil_BigTerm is a tiny sanity check that the constant is
// byte-identical to Lucene's value.
func TestUnicodeUtil_BigTerm(t *testing.T) {
	want := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(BigTerm.ValidBytes(), want) {
		t.Fatalf("BigTerm bytes mismatch: %x", BigTerm.ValidBytes())
	}
}
