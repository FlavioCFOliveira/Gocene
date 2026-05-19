// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0 tests:
//
//   - org.apache.lucene.util.compress.TestHighLZ4
//   - org.apache.lucene.util.compress.LZ4TestCase (specialised here for
//     HighCompressionHashTable since Java's class-hierarchy reuse pattern
//     does not translate cleanly to Go).
//
// Where Lucene relies on infrastructure that does not exist in Gocene
// (LuceneTestCase, randomizedtesting helpers, ByteBuffersDataOutput-backed
// IO scaffolding), we follow Sprint 55 option c: a Go-native, seeded,
// deterministic equivalent that exercises the same logical paths. The
// LUCENE5201 corpus is ported verbatim because it is a regression fixture
// for a real bug — randomising it would lose its diagnostic value.

package compress

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// assertingHashTable wraps a HashTable and panics if the inner table's
// assertReset invariant is violated at the moments LZ4TestCase checks it:
// immediately after Reset, and just before InitDictionary. This mirrors
// org.apache.lucene.util.compress.LZ4TestCase.AssertingHashTable.
type assertingHashTable struct {
	in HashTable
}

func newAssertingHashTable(in HashTable) *assertingHashTable {
	return &assertingHashTable{in: in}
}

func (a *assertingHashTable) Reset(b []byte, off, length int) {
	a.in.Reset(b, off, length)
	if !a.in.assertReset() {
		panic("assertingHashTable: inner hash table is not reset after Reset")
	}
}

func (a *assertingHashTable) InitDictionary(dictLen int) {
	if !a.in.assertReset() {
		panic("assertingHashTable: inner hash table is not reset before InitDictionary")
	}
	a.in.InitDictionary(dictLen)
}

func (a *assertingHashTable) Get(off int) int      { return a.in.Get(off) }
func (a *assertingHashTable) Previous(off int) int { return a.in.Previous(off) }
func (a *assertingHashTable) assertReset() bool {
	// Java: throw new UnsupportedOperationException. The wrapper is purely
	// for use by the test scaffold; nobody should call assertReset on it.
	panic("assertingHashTable.assertReset: unsupported")
}

// newHighHashTable is the Gocene equivalent of TestHighLZ4.newHashTable:
// a HighCompressionHashTable wrapped in the asserting decorator.
func newHighHashTable() *assertingHashTable {
	return newAssertingHashTable(NewHighCompressionHashTable())
}

// doTestHigh mirrors LZ4TestCase.doTest(byte[], HashTable): it picks an
// offset designed to trigger HighCompressionHashTable.Reset's special
// "previous extent < 64kB" branch when the input is small enough, then
// dispatches to the fixed-offset variant.
//
// Java picks the offset from `random()`. We seed an rng from the data
// fingerprint so the test is deterministic but the choice still varies
// across inputs (avoids always exercising the same branch).
func doTestHigh(t *testing.T, data []byte, ht *assertingHashTable) {
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
		// Trigger the "special reset logic for high compression": the
		// previous run's [base, end) wraps past the chain-table's ring
		// boundary so Reset has to clear the table in two segments.
		offset = (1 << 16) - len(data)/2
	}
	trailing := r.Intn(10)
	buf := make([]byte, len(data)+offset+trailing)
	copy(buf[offset:], data)
	doTestHighAt(t, buf, offset, len(data), ht)
}

// doTestHighAt mirrors LZ4TestCase.doTest(byte[], int, int, HashTable):
// compress, walk the encoded stream sanity-checking the LZ4 wire format
// byte by byte, compress again with the same hash table to confirm
// reusability, then decompress at offset 0 and at a non-zero offset.
func doTestHighAt(t *testing.T, data []byte, offset, length int, ht *assertingHashTable) {
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

// doTestHighWithDictionary mirrors LZ4TestCase.doTestWithDictionary.
// Builds a synthetic dictionary out of substrings of the input, compresses
// with the dictionary, then decompresses with the dictionary prefilled
// into the destination buffer.
func doTestHighWithDictionary(t *testing.T, data []byte, ht *assertingHashTable) {
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

	doTestHighWithDictionaryAt(t, dictBuf.Bytes(), dictOff, dictLen, len(data), ht)
}

func doTestHighWithDictionaryAt(t *testing.T, data []byte, dictOff, dictLen, length int, ht *assertingHashTable) {
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

// --- The LZ4TestCase test matrix, specialised for HighCompressionHashTable.

func TestHighLZ4_Empty(t *testing.T) {
	t.Parallel()
	doTestHigh(t, []byte{}, newHighHashTable())
}

func TestHighLZ4_ShortLiteralsAndMatchs(t *testing.T) {
	t.Parallel()
	data := []byte("1234562345673456745678910123")
	doTestHigh(t, data, newHighHashTable())
	doTestHighWithDictionary(t, data, newHighHashTable())
}

func TestHighLZ4_LongMatchs(t *testing.T) {
	t.Parallel()
	// Deterministic equivalent of randomIntBetween(300, 1024): pick a size
	// solidly inside the range that exercises long-match encoding.
	data := make([]byte, 600)
	for i := range data {
		data[i] = byte(i)
	}
	doTestHigh(t, data, newHighHashTable())
}

func TestHighLZ4_LongLiterals(t *testing.T) {
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
	doTestHigh(t, data, newHighHashTable())
}

func TestHighLZ4_MatchRightBeforeLastLiterals(t *testing.T) {
	t.Parallel()
	doTestHigh(t, []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 5}, newHighHashTable())
}

func TestHighLZ4_IncompressibleRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xDEADBEEF))
	// ∈ [1, 1<<18]. Pick a size that is non-trivial and crosses the
	// 64kB chain-table boundary so we exercise the Table32/HC long path.
	data := make([]byte, 80*1024)
	for i := range data {
		data[i] = byte(r.Intn(256))
	}
	doTestHigh(t, data, newHighHashTable())
	doTestHighWithDictionary(t, data, newHighHashTable())
}

func TestHighLZ4_CompressibleRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0x1234_ABCD))
	data := make([]byte, 80*1024)
	base := r.Intn(256)
	maxDelta := 1 + r.Intn(8)
	for i := range data {
		data[i] = byte(base + r.Intn(maxDelta))
	}
	doTestHigh(t, data, newHighHashTable())
	doTestHighWithDictionary(t, data, newHighHashTable())
}

func TestHighLZ4_LUCENE5201(t *testing.T) {
	t.Parallel()
	// Ported verbatim from
	// org.apache.lucene.util.compress.LZ4TestCase#testLUCENE5201.
	// Java signed bytes -> Go uint8 (two's-complement-equivalent).
	data := lucene5201Data()
	doTestHighAt(t, data, 9, len(data)-9, newHighHashTable())
}

func TestHighLZ4_UseDictionary(t *testing.T) {
	t.Parallel()
	data := []byte{
		1, 2, 3, 4, 5, 6, // dictionary
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
	}
	const dictOff, dictLen = 0, 6
	length := len(data) - dictLen

	doTestHighWithDictionaryAt(t, data, dictOff, dictLen, length, newHighHashTable())

	// "compressed output is smaller than the original input despite being
	// incompressible on its own".
	out := store.NewByteArrayDataOutput(32)
	if err := LZ4CompressWithDictionary(data, dictOff, dictLen, length, out, newHighHashTable()); err != nil {
		t.Fatalf("LZ4CompressWithDictionary: %v", err)
	}
	if got := len(out.GetBytes()); got >= length {
		t.Errorf("dictionary did not shrink output: got %d bytes, length %d", got, length)
	}
}

// signed converts a Java-signed byte literal (used in the verbatim
// LUCENE5201 corpus) to its uint8 two's-complement equivalent. Inlined
// rather than imported to keep the test file self-contained.
func signed(v int) byte { return byte(int8(v)) }

// lucene5201Data returns the byte fixture for the testLUCENE5201
// regression test, ported verbatim from the Java source. Wrapped in a
// function so callers can mutate the returned slice without contaminating
// other subtests.
func lucene5201Data() []byte {
	src := []int{
		14, 72, 14, 85, 3, 72, 14, 85, 3, 72, 14, 72, 14, 72, 14, 85, 3, 72, 14, 72, 14, 72, 14,
		72, 14, 72, 14, 72, 14, 85, 3, 72,
		14, 85, 3, 72, 14, 85, 3, 72, 14, 85, 3, 72, 14, 85, 3, 72, 14, 85, 3, 72, 14, 50, 64, 0,
		46, -1, 0, 0, 0, 29, 3, 85,
		8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2,
		3, 85, 8, -113, 0, 68, -97, 3,
		0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113,
		0, 2, 3, -97, 6, 0, 68, -113,
		0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113,
		0, 2, 3, -97, 6, 0, 68, -113,
		0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 50, 64, 0, 47, -105, 0, 0,
		0, 30, 3, -97, 6, 0, 68, -113,
		0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, 85, 8, -113,
		0, 68, -97, 3, 0, 2, 3, 85,
		8, -113, 0, 68, -97, 3, 0, 2, -97, 6, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 120, 64, 0, 48, 4, 0, 0, 0, 31, 34, 72, 29, 72, 37, 72, 35, 72, 45, 72,
		23, 72, 46, 72, 20, 72, 40, 72,
		33, 72, 25, 72, 39, 72, 38, 72, 26, 72, 28, 72, 42, 72, 24, 72, 27, 72, 36, 72, 41, 72,
		32, 72, 18, 72, 30, 72, 22, 72, 31, 72,
		43, 72, 19, 72, 34, 72, 29, 72, 37, 72, 35, 72, 45, 72, 23, 72, 46, 72, 20, 72, 40, 72,
		33, 72, 25, 72, 39, 72, 38, 72, 26, 72,
		28, 72, 42, 72, 24, 72, 27, 72, 36, 72, 41, 72, 32, 72, 18, 72, 30, 72, 22, 72, 31, 72,
		43, 72, 19, 72, 34, 72, 29, 72, 37, 72,
		35, 72, 45, 72, 23, 72, 46, 72, 20, 72, 40, 72, 33, 72, 25, 72, 39, 72, 38, 72, 26, 72,
		28, 72, 42, 72, 24, 72, 27, 72, 36, 72,
		41, 72, 32, 72, 18, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0,
		0, 39, 24, 32, 34, 124, 0, 120, 64, 0, 48, 80, 0, 0, 0, 31, 30, 72, 22, 72, 31, 72, 43,
		72, 19, 72, 34, 72, 29, 72, 37, 72,
		35, 72, 45, 72, 23, 72, 46, 72, 20, 72, 40, 72, 33, 72, 25, 72, 39, 72, 38, 72, 26, 72,
		28, 72, 42, 72, 24, 72, 27, 72, 36, 72,
		41, 72, 32, 72, 18, 72, 30, 72, 22, 72, 31, 72, 43, 72, 19, 72, 34, 72, 29, 72, 37, 72,
		35, 72, 45, 72, 23, 72, 46, 72, 20, 72,
		40, 72, 33, 72, 25, 72, 39, 72, 38, 72, 26, 72, 28, 72, 42, 72, 24, 72, 27, 72, 36, 72,
		41, 72, 32, 72, 18, 72, 30, 72, 22, 72,
		31, 72, 43, 72, 19, 72, 34, 72, 29, 72, 37, 72, 35, 72, 45, 72, 23, 72, 46, 72, 20, 72,
		40, 72, 33, 72, 25, 72, 39, 72, 38, 72,
		26, 72, 28, 72, 42, 72, 24, 72, 27, 72, 36, 72, 41, 72, 32, 72, 18, 72, 30, 72, 22, 72,
		31, 72, 43, 72, 19, 72, 34, 72, 29, 72,
		37, 72, 35, 72, 45, 72, 23, 72, 46, 72, 20, 72, 40, 72, 33, 72, 25, 72, 39, 72, 38, 72,
		26, 72, 28, 72, 42, 72, 24, 72, 27, 72,
		36, 72, 41, 72, 32, 72, 18, 72, 30, 72, 22, 72, 31, 72, 43, 72, 19, 72, 34, 72, 29, 72,
		37, 72, 35, 72, 45, 72, 23, 72, 46, 72,
		20, 72, 40, 72, 33, 72, 25, 72, 39, 72, 38, 72, 26, 72, 28, 72, 42, 72, 24, 72, 27, 72,
		36, 72, 41, 72, 32, 72, 18, 72, 30, 72,
		22, 72, 31, 72, 43, 72, 19, 72, 34, 72, 29, 72, 37, 72, 35, 72, 45, 72, 23, 72, 46, 72,
		20, 72, 40, 72, 33, 72, 25, 72, 39, 72,
		38, 72, 26, 72, 28, 72, 42, 72, 24, 72, 27, 72, 36, 72, 41, 72, 32, 72, 18, 72, 30, 72,
		22, 72, 31, 72, 43, 72, 19, 72, 34, 72,
		29, 72, 37, 72, 35, 72, 45, 72, 23, 72, 46, 72, 20, 72, 40, 72, 33, 72, 25, 72, 39, 72,
		38, 72, 26, 72, 28, 72, 42, 72, 24, 72,
		27, 72, 36, 72, 41, 72, 32, 72, 18, 72, 30, 72, 22, 72, 31, 72, 43, 72, 19, 50, 64, 0, 49,
		20, 0, 0, 0, 32, 3, -97, 6, 0,
		68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2,
		3, -97, 6, 0, 68, -113, 0, 2,
		3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2,
		3, -97, 6, 0, 68, -113, 0, 2,
		3, -97, 6, 0, 50, 64, 0, 50, 53, 0, 0, 0, 34, 3, -97, 6, 0, 68, -113, 0, 2, 3, 85, 8,
		-113, 0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3,
		-97, 6, 0, 68, -113, 0, 2, 3,
		-97, 6, 0, 68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0,
		2, 3, 85, 8, -113, 0, 68, -97,
		3, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, 85, 8,
		-113, 0, 68, -97, 3, 0, 2, 3,
		85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0,
		2, 3, -97, 6, 0, 68, -113, 0,
		2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, 85, 8,
		-113, 0, 68, -97, 3, 0, 2, 3,
		-97, 6, 0, 50, 64, 0, 51, 85, 0, 0, 0, 36, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, -97, 6,
		0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, -97, 5,
		0, 2, 3, 85, 8, -113, 0, 68,
		-97, 3, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0,
		68, -113, 0, 2, 3, -97, 6, 0,
		68, -113, 0, 2, 3, -97, 6, 0, 50, -64, 0, 51, -45, 0, 0, 0, 37, 68, -113, 0, 2, 3, -97, 6,
		0, 68, -113, 0, 2, 3, -97, 6,
		0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, -97, 6, 0, 68, -113, 0, 2, 3, 85, 8,
		-113, 0, 68, -113, 0, 2, 3, -97,
		6, 0, 68, -113, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3, 0, 2, 3, 85, 8, -113, 0, 68, -97, 3,
		0, 120, 64, 0, 52, -88, 0, 0,
		0, 39, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 72, 13, 85, 5, 72, 13, 85, 5, 72,
		13, 85, 5, 72, 13, 85, 5, 72,
		13, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 72, 13, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13,
		85, 5, 72, 13, 85, 5, 72, 13, 85,
		5, 72, 13, 85, 5, 72, 13, 72, 13, 72, 13, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 72, 13,
		85, 5, 72, 13, 85, 5, 72, 13, 85,
		5, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85,
		5, 72, 13, 85, 5, 72, 13, 85,
		5, 72, 13, 85, 5, 72, 13, 72, 13, 72, 13, 72, 13, 85, 5, 72, 13, 85, 5, 72, 13, 85, 5, 72,
		13, 72, 13, 85, 5, 72, 13, 72,
		13, 85, 5, 72, 13, 72, 13, 85, 5, 72, 13, -19, -24, -101, -35,
	}
	out := make([]byte, len(src))
	for i, v := range src {
		out[i] = signed(v)
	}
	return out
}

// Compile-time guard that assertingHashTable implements HashTable.
var _ HashTable = (*assertingHashTable)(nil)
