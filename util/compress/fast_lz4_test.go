// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0 tests:
//
//   - org.apache.lucene.util.compress.TestFastLZ4
//   - org.apache.lucene.util.compress.LZ4TestCase (specialised here for
//     FastCompressionHashTable since Java's class-hierarchy reuse pattern
//     does not translate cleanly to Go).
//
// Where Lucene relies on infrastructure that does not exist in Gocene
// (LuceneTestCase, randomizedtesting helpers, ByteBuffersDataOutput-backed
// IO scaffolding), we follow Sprint 55 option c: a Go-native, seeded,
// deterministic equivalent that exercises the same logical paths. The
// LUCENE5201 corpus is ported verbatim because it is a regression fixture
// for a real bug — randomising it would lose its diagnostic value.
//
// The assertingHashTable decorator, signed() helper and lucene5201Data()
// fixture are declared in high_lz4_test.go (same package) and reused here
// verbatim to avoid duplication.

package compress

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newFastHashTable is the Gocene equivalent of TestFastLZ4.newHashTable:
// a FastCompressionHashTable wrapped in the asserting decorator.
func newFastHashTable() *assertingHashTable {
	return newAssertingHashTable(NewFastCompressionHashTable())
}

// doTestFast mirrors LZ4TestCase.doTest(byte[], HashTable). The
// FastCompressionHashTable has a uniform Reset (no 64kB ring-boundary
// special case), but we keep the same offset-picking logic as the High
// variant for byte-level parity with the shared Java LZ4TestCase.
func doTestFast(t *testing.T, data []byte, ht *assertingHashTable) {
	t.Helper()
	seed := int64(len(data))
	for i, b := range data {
		seed = seed*1315423911 ^ int64(b)<<uint(i%24)
	}
	r := rand.New(rand.NewSource(seed))

	var offset int
	if len(data) >= (1<<16) || r.Intn(2) == 0 {
		offset = r.Intn(10)
	} else {
		offset = (1 << 16) - len(data)/2
	}
	trailing := r.Intn(10)
	buf := make([]byte, len(data)+offset+trailing)
	copy(buf[offset:], data)
	doTestFastAt(t, buf, offset, len(data), ht)
}

// doTestFastAt mirrors LZ4TestCase.doTest(byte[], int, int, HashTable):
// compress, walk the encoded stream sanity-checking the LZ4 wire format
// byte by byte, compress again with the same hash table to confirm
// reusability, then decompress at offset 0 and at a non-zero offset.
func doTestFastAt(t *testing.T, data []byte, offset, length int, ht *assertingHashTable) {
	t.Helper()

	out := store.NewByteArrayDataOutput(length + 16)
	if err := LZ4Compress(data, offset, length, out, ht); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	compressed := append([]byte(nil), out.GetBytes()...)

	// Walk the encoded stream the same way LZ4TestCase.doTest does, so we
	// catch wire-format drift (literal-len continuation runs, 16-bit LE
	// match offset, match-len continuation runs).
	off := 0
	decompressedOff := 0
	for {
		token := int(compressed[off]) & 0xFF
		off++
		literalLen := token >> 4
		if literalLen == 0x0F {
			for compressed[off] == 0xFF {
				literalLen += 0xFF
				off++
			}
			literalLen += int(compressed[off]) & 0xFF
			off++
		}
		off += literalLen
		decompressedOff += literalLen

		if off == len(compressed) {
			if decompressedOff != length {
				t.Fatalf("trailer mismatch: decompressedOff=%d length=%d", decompressedOff, length)
			}
			if literalLen < lastLiterals && literalLen != length {
				t.Fatalf("lastLiterals=%d bytes=%d (must be >= %d or == total length)",
					literalLen, length, lastLiterals)
			}
			break
		}

		matchDec := int(compressed[off])&0xFF | (int(compressed[off+1])&0xFF)<<8
		off += 2
		if matchDec <= 0 || matchDec > decompressedOff {
			t.Fatalf("matchDec=%d decompressedOff=%d (out of range)", matchDec, decompressedOff)
		}

		matchLen := token & 0x0F
		if matchLen == 0x0F {
			for compressed[off] == 0xFF {
				matchLen += 0xFF
				off++
			}
			matchLen += int(compressed[off]) & 0xFF
			off++
		}
		matchLen += minMatch

		// LZ4TestCase's "wasted-space" invariant: if a match ends before
		// the final lastLiterals window and the next byte after the
		// match still matches data[off-matchDec+matchLen], the next
		// sequence must not carry any literals — otherwise the encoder
		// has emitted a shorter match than it could have.
		if decompressedOff+matchLen < length-lastLiterals {
			moreCommon := data[offset+decompressedOff+matchLen] ==
				data[offset+decompressedOff-matchDec+matchLen]
			nextHasLiterals := ((int(compressed[off]) & 0xFF) >> 4) != 0
			if moreCommon && nextHasLiterals {
				t.Fatalf("wasted match: extension available at decompressedOff=%d", decompressedOff)
			}
		}

		decompressedOff += matchLen
	}
	if decompressedOff != length {
		t.Fatalf("after walk: decompressedOff=%d length=%d", decompressedOff, length)
	}

	// Compress again with the same hash table — must produce identical bytes.
	out2 := store.NewByteArrayDataOutput(length + 16)
	if err := LZ4Compress(data, offset, length, out2, ht); err != nil {
		t.Fatalf("second LZ4Compress: %v", err)
	}
	if !bytes.Equal(compressed, out2.GetBytes()) {
		t.Fatalf("reused hash table produced different output:\n  first=%x\n  second=%x",
			compressed, out2.GetBytes())
	}

	// Restore at offset 0.
	restored := make([]byte, length+3)
	in := store.NewByteArrayDataInput(compressed)
	end, err := LZ4Decompress(in, length, restored, 0)
	if err != nil {
		t.Fatalf("LZ4Decompress @0: %v", err)
	}
	if end != length {
		t.Fatalf("decompress @0 wrote to %d, want %d", end, length)
	}
	if !bytes.Equal(restored[:length], data[offset:offset+length]) {
		t.Fatalf("round-trip @0 mismatch")
	}

	// Restore at a non-zero offset.
	restoreOffset := 1 + (length % 9) // deterministic, 1..9 per LZ4TestCase
	restored = make([]byte, restoreOffset+length+3)
	in = store.NewByteArrayDataInput(compressed)
	end, err = LZ4Decompress(in, length, restored, restoreOffset)
	if err != nil {
		t.Fatalf("LZ4Decompress @%d: %v", restoreOffset, err)
	}
	if end-restoreOffset != length {
		t.Fatalf("decompress @%d wrote %d, want %d", restoreOffset, end-restoreOffset, length)
	}
	if !bytes.Equal(restored[restoreOffset:restoreOffset+length], data[offset:offset+length]) {
		t.Fatalf("round-trip @%d mismatch", restoreOffset)
	}
}

// doTestFastWithDictionary mirrors LZ4TestCase.doTestWithDictionary.
func doTestFastWithDictionary(t *testing.T, data []byte, ht *assertingHashTable) {
	t.Helper()
	r := rand.New(rand.NewSource(int64(len(data)) ^ 0x5A5A5A5A))

	dictOff := r.Intn(11)
	var dictBuf bytes.Buffer
	dictBuf.Write(make([]byte, dictOff))

	dictLen := 0
	i := r.Intn(len(data) + 1)
	for i < len(data) && dictLen < MaxDistance {
		l := 1 + r.Intn(32)
		if i+l > len(data) {
			l = len(data) - i
		}
		if dictLen+l > MaxDistance {
			l = MaxDistance - dictLen
		}
		dictBuf.Write(data[i : i+l])
		dictLen += l
		i += l
		i += 1 + r.Intn(32)
	}

	dictBuf.Write(data)
	dictBuf.Write(make([]byte, r.Intn(10)))

	doTestFastWithDictionaryAt(t, dictBuf.Bytes(), dictOff, dictLen, len(data), ht)
}

func doTestFastWithDictionaryAt(t *testing.T, data []byte, dictOff, dictLen, length int, ht *assertingHashTable) {
	t.Helper()

	out := store.NewByteArrayDataOutput(length + 32)
	if err := LZ4CompressWithDictionary(data, dictOff, dictLen, length, out, ht); err != nil {
		t.Fatalf("LZ4CompressWithDictionary: %v", err)
	}
	compressed := append([]byte(nil), out.GetBytes()...)

	out2 := store.NewByteArrayDataOutput(length + 32)
	if err := LZ4CompressWithDictionary(data, dictOff, dictLen, length, out2, ht); err != nil {
		t.Fatalf("second LZ4CompressWithDictionary: %v", err)
	}
	if !bytes.Equal(compressed, out2.GetBytes()) {
		t.Fatalf("reused hash table produced different dict-compressed output")
	}

	restoreOffset := 1 + (length % 9)
	restored := make([]byte, restoreOffset+dictLen+length+3)
	copy(restored[restoreOffset:], data[dictOff:dictOff+dictLen])

	in := store.NewByteArrayDataInput(compressed)
	if _, err := LZ4Decompress(in, length, restored, dictLen+restoreOffset); err != nil {
		t.Fatalf("LZ4Decompress with dict: %v", err)
	}
	got := restored[restoreOffset+dictLen : restoreOffset+dictLen+length]
	want := data[dictOff+dictLen : dictOff+dictLen+length]
	if !bytes.Equal(got, want) {
		t.Fatalf("dict round-trip mismatch:\n  want %x\n  got  %x", want, got)
	}
}

// --- The LZ4TestCase test matrix, specialised for FastCompressionHashTable.

func TestFastLZ4_Empty(t *testing.T) {
	t.Parallel()
	doTestFast(t, []byte{}, newFastHashTable())
}

func TestFastLZ4_ShortLiteralsAndMatchs(t *testing.T) {
	t.Parallel()
	data := []byte("1234562345673456745678910123")
	doTestFast(t, data, newFastHashTable())
	doTestFastWithDictionary(t, data, newFastHashTable())
}

func TestFastLZ4_LongMatchs(t *testing.T) {
	t.Parallel()
	// Deterministic equivalent of randomIntBetween(300, 1024): pick a size
	// solidly inside the range that exercises long-match encoding.
	data := make([]byte, 600)
	for i := range data {
		data[i] = byte(i)
	}
	doTestFast(t, data, newFastHashTable())
}

func TestFastLZ4_LongLiterals(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xC0FFEE_BEEF))
	data := make([]byte, 700) // ∈ [400, 1024]
	for i := range data {
		data[i] = byte(r.Intn(256))
	}
	matchRef := r.Intn(30)
	matchOff := len(data) - 30 - r.Intn(20)
	matchLength := 4 + r.Intn(7)
	copy(data[matchOff:matchOff+matchLength], data[matchRef:matchRef+matchLength])
	doTestFast(t, data, newFastHashTable())
}

func TestFastLZ4_MatchRightBeforeLastLiterals(t *testing.T) {
	t.Parallel()
	doTestFast(t, []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 5}, newFastHashTable())
}

func TestFastLZ4_IncompressibleRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xDEADBEEF))
	// ∈ [1, 1<<18]. Pick a size that is non-trivial and crosses the
	// 64kB chain-table boundary so we exercise the Table32 long path.
	data := make([]byte, 80*1024)
	for i := range data {
		data[i] = byte(r.Intn(256))
	}
	doTestFast(t, data, newFastHashTable())
	doTestFastWithDictionary(t, data, newFastHashTable())
}

func TestFastLZ4_CompressibleRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0x1234_ABCD))
	data := make([]byte, 80*1024)
	base := r.Intn(256)
	maxDelta := 1 + r.Intn(8)
	for i := range data {
		data[i] = byte(base + r.Intn(maxDelta))
	}
	doTestFast(t, data, newFastHashTable())
	doTestFastWithDictionary(t, data, newFastHashTable())
}

func TestFastLZ4_LUCENE5201(t *testing.T) {
	t.Parallel()
	// Ported verbatim from
	// org.apache.lucene.util.compress.LZ4TestCase#testLUCENE5201.
	// Fixture lives in high_lz4_test.go (same package).
	data := lucene5201Data()
	doTestFastAt(t, data, 9, len(data)-9, newFastHashTable())
}

func TestFastLZ4_UseDictionary(t *testing.T) {
	t.Parallel()
	data := []byte{
		1, 2, 3, 4, 5, 6, // dictionary
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
	}
	const dictOff, dictLen = 0, 6
	length := len(data) - dictLen

	doTestFastWithDictionaryAt(t, data, dictOff, dictLen, length, newFastHashTable())

	// "compressed output is smaller than the original input despite being
	// incompressible on its own".
	out := store.NewByteArrayDataOutput(32)
	if err := LZ4CompressWithDictionary(data, dictOff, dictLen, length, out, newFastHashTable()); err != nil {
		t.Fatalf("LZ4CompressWithDictionary: %v", err)
	}
	if got := len(out.GetBytes()); got >= length {
		t.Errorf("dictionary did not shrink output: got %d bytes, length %d", got, length)
	}
}
