// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compress

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// roundTrip compresses src with ht then decompresses it back, asserting
// that the recovered bytes equal src. Returns the size of the encoded
// stream for sanity checks by callers.
func roundTrip(t *testing.T, src []byte, ht HashTable) int {
	t.Helper()
	out := store.NewByteArrayDataOutput(len(src) + 16)
	if err := LZ4Compress(src, 0, len(src), out, ht); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	encoded := out.GetBytes()

	in := store.NewByteArrayDataInput(encoded)
	dest := make([]byte, len(src))
	n, err := LZ4Decompress(in, len(src), dest, 0)
	if err != nil {
		t.Fatalf("LZ4Decompress: %v", err)
	}
	if n != len(src) {
		t.Errorf("LZ4Decompress wrote %d bytes, want %d", n, len(src))
	}
	if !bytes.Equal(dest, src) {
		// Truncate output in error messages to keep things readable.
		maxShow := 64
		want := src
		got := dest
		if len(want) > maxShow {
			want = want[:maxShow]
			got = got[:maxShow]
		}
		t.Errorf("round-trip mismatch:\n  src len=%d\n  want %x\n  got  %x", len(src), want, got)
	}
	return len(encoded)
}

func TestLZ4_RoundTrip_KnownFixture_FastHashTable(t *testing.T) {
	t.Parallel()
	// "the quick brown fox..." repeated 8 times. Plenty of repetition so
	// the encoder finds matches and we exercise both literal and match
	// encoding paths.
	chunk := "the quick brown fox jumps over the lazy dog "
	src := []byte(strings.Repeat(chunk, 8))

	encodedLen := roundTrip(t, src, NewFastCompressionHashTable())
	if encodedLen >= len(src) {
		t.Errorf("repetitive input not compressed: encoded=%d input=%d", encodedLen, len(src))
	}
}

func TestLZ4_RoundTrip_KnownFixture_HighCompressionHashTable(t *testing.T) {
	t.Parallel()
	chunk := "the quick brown fox jumps over the lazy dog "
	src := []byte(strings.Repeat(chunk, 8))

	encodedLen := roundTrip(t, src, NewHighCompressionHashTable())
	if encodedLen >= len(src) {
		t.Errorf("repetitive input not compressed: encoded=%d input=%d", encodedLen, len(src))
	}
}

func TestLZ4_RoundTrip_HighRepetition_FastHashTable(t *testing.T) {
	t.Parallel()
	// Synthetic data with very high 4-byte sequence repetition: alternate
	// among a small set of 4-byte words so the hash table is full of
	// useful entries.
	words := [][]byte{
		{0x01, 0x02, 0x03, 0x04},
		{0x05, 0x06, 0x07, 0x08},
		{0x09, 0x0A, 0x0B, 0x0C},
		{0x0D, 0x0E, 0x0F, 0x10},
	}
	src := make([]byte, 0, 4096)
	r := rand.New(rand.NewSource(0xC0FFEE))
	for len(src) < 4096 {
		src = append(src, words[r.Intn(len(words))]...)
	}

	encodedLen := roundTrip(t, src, NewFastCompressionHashTable())
	if encodedLen >= len(src) {
		t.Errorf("repetitive synthetic data not compressed: encoded=%d input=%d", encodedLen, len(src))
	}
}

func TestLZ4_RoundTrip_HighRepetition_HighCompressionHashTable(t *testing.T) {
	t.Parallel()
	words := [][]byte{
		{0x01, 0x02, 0x03, 0x04},
		{0x05, 0x06, 0x07, 0x08},
		{0x09, 0x0A, 0x0B, 0x0C},
		{0x0D, 0x0E, 0x0F, 0x10},
	}
	src := make([]byte, 0, 4096)
	r := rand.New(rand.NewSource(0xBADF00D))
	for len(src) < 4096 {
		src = append(src, words[r.Intn(len(words))]...)
	}

	encodedLen := roundTrip(t, src, NewHighCompressionHashTable())
	if encodedLen >= len(src) {
		t.Errorf("repetitive synthetic data not compressed: encoded=%d input=%d", encodedLen, len(src))
	}
}

func TestLZ4_RoundTrip_Incompressible_FastHashTable(t *testing.T) {
	t.Parallel()
	// Random data: the encoder cannot find useful matches, so we don't
	// require any compression ratio — only that the round-trip is exact.
	src := make([]byte, 2048)
	r := rand.New(rand.NewSource(0xDEADBEEF))
	for i := range src {
		src[i] = byte(r.Intn(256))
	}
	roundTrip(t, src, NewFastCompressionHashTable())
}

func TestLZ4_RoundTrip_Incompressible_HighCompressionHashTable(t *testing.T) {
	t.Parallel()
	src := make([]byte, 2048)
	r := rand.New(rand.NewSource(0x1234ABCD))
	for i := range src {
		src[i] = byte(r.Intn(256))
	}
	roundTrip(t, src, NewHighCompressionHashTable())
}

func TestLZ4_RoundTrip_Empty(t *testing.T) {
	t.Parallel()
	// Empty input: no main loop, just an empty last-literals token (0x00).
	src := []byte{}
	out := store.NewByteArrayDataOutput(8)
	if err := LZ4Compress(src, 0, 0, out, NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress empty: %v", err)
	}
	if got := out.GetBytes(); len(got) != 1 || got[0] != 0x00 {
		t.Errorf("empty input: want [00], got %x", got)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	dest := make([]byte, 0)
	n, err := LZ4Decompress(in, 0, dest, 0)
	if err != nil {
		t.Fatalf("LZ4Decompress empty: %v", err)
	}
	if n != 0 {
		t.Errorf("empty decompress wrote %d bytes", n)
	}
}

func TestLZ4_RoundTrip_SingleByte(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0x00, 0x42, 0xFF} {
		src := []byte{b}
		out := store.NewByteArrayDataOutput(4)
		if err := LZ4Compress(src, 0, 1, out, NewFastCompressionHashTable()); err != nil {
			t.Fatalf("LZ4Compress %x: %v", src, err)
		}
		// Expect: token = (1<<4) = 0x10, then the literal byte.
		want := []byte{0x10, b}
		if got := out.GetBytes(); !bytes.Equal(got, want) {
			t.Errorf("single byte %x: want %x, got %x", src, want, got)
		}

		in := store.NewByteArrayDataInput(out.GetBytes())
		dest := make([]byte, 1)
		n, err := LZ4Decompress(in, 1, dest, 0)
		if err != nil {
			t.Fatalf("LZ4Decompress %x: %v", src, err)
		}
		if n != 1 || dest[0] != b {
			t.Errorf("single byte round-trip: src=%x got=%x", src, dest)
		}
	}
}

// TestLZ4_Fixture_LiteralsOnlyShort verifies the exact byte format of an
// input that is entirely encoded as last-literals with no continuation byte
// for the length. 10 distinct bytes -> 1 token (0xA0) + 10 raw bytes.
func TestLZ4_Fixture_LiteralsOnlyShort(t *testing.T) {
	t.Parallel()
	src := []byte("abcdefghij") // 10 bytes, all distinct
	want := []byte{
		0xA0, // token: high nibble = literal len = 10, low nibble = match nibble = 0 (unused)
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j',
	}

	out := store.NewByteArrayDataOutput(16)
	if err := LZ4Compress(src, 0, len(src), out, NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	got := out.GetBytes()
	if !bytes.Equal(got, want) {
		t.Errorf("byte-format drift:\n  want %s\n  got  %s",
			hex.EncodeToString(want), hex.EncodeToString(got))
	}
}

// TestLZ4_Fixture_LiteralsOnlyLong verifies the literal-length continuation
// byte path. 20 distinct bytes -> token 0xF0 + continuation 0x05 + 20 raw
// bytes (the LZ4 encoding writes (length - 0x0F) as the continuation
// payload).
func TestLZ4_Fixture_LiteralsOnlyLong(t *testing.T) {
	t.Parallel()
	src := []byte("abcdefghijklmnopqrst") // 20 bytes, all distinct
	want := []byte{
		0xF0, // token: high nibble = 0x0F (>=15 sentinel), low nibble = 0
		0x05, // continuation byte: 20 - 15 = 5
	}
	want = append(want, src...)

	out := store.NewByteArrayDataOutput(32)
	if err := LZ4Compress(src, 0, len(src), out, NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	got := out.GetBytes()
	if !bytes.Equal(got, want) {
		t.Errorf("byte-format drift:\n  want %s\n  got  %s",
			hex.EncodeToString(want), hex.EncodeToString(got))
	}
}

// TestLZ4_Fixture_LiteralsContinuationFull255 verifies the path where the
// literal length is exactly 0x0F + 0xFF, i.e. one 0xFF run-byte followed by
// 0x00 as the final residue.
func TestLZ4_Fixture_LiteralsContinuation255(t *testing.T) {
	t.Parallel()
	// 270 distinct-ish bytes: cycle 256..256+13 (mod 256) to make all
	// 4-byte sequences unique by including the position in the value.
	src := make([]byte, 270)
	for i := range src {
		src[i] = byte((i * 17) ^ 0x55) // pseudo-distinct but deterministic
	}
	// Quick sanity: ensure no 4-byte sequence collides within the first
	// 266 positions (would otherwise let the encoder emit a match).
	seen := map[uint32]int{}
	collision := false
	for i := 0; i+4 <= len(src); i++ {
		key := uint32(src[i]) | uint32(src[i+1])<<8 | uint32(src[i+2])<<16 | uint32(src[i+3])<<24
		if _, ok := seen[key]; ok {
			collision = true
			break
		}
		seen[key] = i
	}
	if collision {
		t.Fatal("synthetic input happens to contain a duplicate 4-byte sequence; skip fixture")
	}

	// Expected: token=0xF0, then encodeLen(270-15=255): 0xFF 0x00, then 270 bytes.
	want := []byte{0xF0, 0xFF, 0x00}
	want = append(want, src...)

	out := store.NewByteArrayDataOutput(300)
	if err := LZ4Compress(src, 0, len(src), out, NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	got := out.GetBytes()
	if !bytes.Equal(got, want) {
		t.Errorf("byte-format drift:\n  want %s\n  got  %s",
			hex.EncodeToString(want), hex.EncodeToString(got))
	}
}

// TestLZ4_Fixture_SingleMatch_FastHashTable verifies the byte format of an
// input that produces exactly one match. Input is 22 bytes:
//
//	"abcdefghABCDabcdefghij"
//
// matchLimit = len - LAST_LITERALS - MIN_MATCH = 22 - 5 - 4 = 13, so the
// encoder is allowed to inspect offset 12 (which is < 13). At off=12 it
// finds the second occurrence of "abcd" matching off=0. The match can
// extend one byte beyond the 4-byte prefix because limit (= end -
// LAST_LITERALS = 17) leaves exactly one byte of slack ('e' at off=4 vs
// 'e' at off=16). Output layout:
//
//	C1                            token: 12 literals | matchLen-4 = 1
//	61 62 63 64 65 66 67 68       "abcdefgh"
//	41 42 43 44                   "ABCD"
//	0C 00                         matchDec = 12 (little-endian short)
//	50                            last-literals token (5 literals)
//	66 67 68 69 6A                "fghij"
//
// This is the most discriminating fixture in the test suite: it asserts
// (a) the 4-bit token packing for both nibbles, (b) the little-endian
// match offset, and (c) the last-literals trailer.
func TestLZ4_Fixture_SingleMatch_FastHashTable(t *testing.T) {
	t.Parallel()
	src := []byte("abcdefghABCDabcdefghij") // 22 bytes
	want := []byte{
		0xC1,
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h',
		'A', 'B', 'C', 'D',
		0x0C, 0x00,
		0x50,
		'f', 'g', 'h', 'i', 'j',
	}

	out := store.NewByteArrayDataOutput(32)
	if err := LZ4Compress(src, 0, len(src), out, NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	got := out.GetBytes()
	if !bytes.Equal(got, want) {
		t.Errorf("byte-format drift:\n  want %s\n  got  %s",
			hex.EncodeToString(want), hex.EncodeToString(got))
	}

	// Round-trip it for safety.
	in := store.NewByteArrayDataInput(got)
	dest := make([]byte, len(src))
	n, err := LZ4Decompress(in, len(src), dest, 0)
	if err != nil {
		t.Fatalf("LZ4Decompress: %v", err)
	}
	if n != len(src) || !bytes.Equal(dest, src) {
		t.Errorf("fixture round-trip mismatch: want %q got %q", src, dest)
	}
}

// TestLZ4_HashTable_Reset verifies that both hash-table implementations can
// be reused across multiple Compress calls and that the second result is
// identical to a fresh compression with a brand-new hash table.
func TestLZ4_HashTable_Reset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		makeHT func() HashTable
	}{
		{"FastCompressionHashTable", func() HashTable { return NewFastCompressionHashTable() }},
		{"HighCompressionHashTable", func() HashTable { return NewHighCompressionHashTable() }},
	}

	src1 := []byte(strings.Repeat("the quick brown fox jumps. ", 5))
	src2 := []byte(strings.Repeat("0123456789abcdef", 32))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// First, compress src1 then src2 with a single hash table.
			shared := tc.makeHT()
			outShared := store.NewByteArrayDataOutput(len(src1) + 8)
			if err := LZ4Compress(src1, 0, len(src1), outShared, shared); err != nil {
				t.Fatalf("first compress: %v", err)
			}
			outSharedSrc1 := append([]byte(nil), outShared.GetBytes()...)

			outShared2 := store.NewByteArrayDataOutput(len(src2) + 8)
			if err := LZ4Compress(src2, 0, len(src2), outShared2, shared); err != nil {
				t.Fatalf("second compress (reused HT): %v", err)
			}
			outSharedSrc2 := append([]byte(nil), outShared2.GetBytes()...)

			// Now compress each source with a fresh table; the outputs must match
			// byte-for-byte (Reset must fully isolate the second invocation).
			fresh1 := tc.makeHT()
			outFresh1 := store.NewByteArrayDataOutput(len(src1) + 8)
			if err := LZ4Compress(src1, 0, len(src1), outFresh1, fresh1); err != nil {
				t.Fatalf("fresh compress 1: %v", err)
			}

			fresh2 := tc.makeHT()
			outFresh2 := store.NewByteArrayDataOutput(len(src2) + 8)
			if err := LZ4Compress(src2, 0, len(src2), outFresh2, fresh2); err != nil {
				t.Fatalf("fresh compress 2: %v", err)
			}

			if !bytes.Equal(outSharedSrc1, outFresh1.GetBytes()) {
				t.Errorf("first compress with reused HT differs from fresh:\n  shared=%x\n  fresh =%x",
					outSharedSrc1, outFresh1.GetBytes())
			}
			if !bytes.Equal(outSharedSrc2, outFresh2.GetBytes()) {
				t.Errorf("second compress with reused HT differs from fresh:\n  shared=%x\n  fresh =%x",
					outSharedSrc2, outFresh2.GetBytes())
			}

			// And round-trip the second compressed stream to ensure decode works.
			in := store.NewByteArrayDataInput(outSharedSrc2)
			dest := make([]byte, len(src2))
			if _, err := LZ4Decompress(in, len(src2), dest, 0); err != nil {
				t.Fatalf("decompress reused-HT output: %v", err)
			}
			if !bytes.Equal(dest, src2) {
				t.Errorf("reused-HT round-trip mismatch")
			}
		})
	}
}

func TestLZ4_Decompress_InvalidOffsetZero(t *testing.T) {
	t.Parallel()
	// A handcrafted stream with: one literal "abcd", then a match token
	// declaring matchLen >= 4 but with matchDec=0 -> must error out.
	encoded := []byte{
		0x41,               // 4 literals, match-nibble 1 (matchLen = 5)
		'a', 'b', 'c', 'd', // literals
		0x00, 0x00, // matchDec = 0 (invalid)
	}
	in := store.NewByteArrayDataInput(encoded)
	dest := make([]byte, 64)
	if _, err := LZ4Decompress(in, 9, dest, 0); err == nil {
		t.Fatal("expected error for matchDec=0, got nil")
	}
}

func TestLZ4_RoundTrip_LargerThan64kB_FastHashTable(t *testing.T) {
	t.Parallel()
	// 100kB: exceeds 1<<16, exercises the Table32 path in
	// FastCompressionHashTable.Reset.
	const size = 100 * 1024
	src := make([]byte, size)
	r := rand.New(rand.NewSource(0xFEEDFACE))
	// Half-random half-repetitive payload: random words duplicated in
	// blocks to give the encoder things to find without being trivially
	// compressible.
	word := make([]byte, 64)
	for i := range word {
		word[i] = byte(r.Intn(256))
	}
	for i := 0; i < size; {
		chunkLen := 64 + r.Intn(64)
		if i+chunkLen > size {
			chunkLen = size - i
		}
		copy(src[i:i+chunkLen], word)
		i += chunkLen
		// Occasionally insert some noise so it's not 100% repetitive.
		if i < size {
			src[i] = byte(r.Intn(256))
			i++
		}
	}

	roundTrip(t, src, NewFastCompressionHashTable())
}
