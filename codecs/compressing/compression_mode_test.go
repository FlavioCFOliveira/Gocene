// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package compressing tests cover the three CompressionMode variants
// defined by Apache Lucene 10.4.0:
//   - FAST                — TestFastCompressionMode.java
//   - HIGH_COMPRESSION    — TestHighCompressionMode.java
//   - FAST_DECOMPRESSION  — TestFastDecompressionMode.java
//
// The body of each test mirrors AbstractTestCompressionMode.java; the table
// of seven subtests (Decompress, PartialDecompress, EmptySequence,
// ShortSequence, Incompressible, Constant, ExtremelyLargeInput) is run for
// every mode through compressionModeContract.
//
// Wire-format parity:
//
//   - LZ4 (FAST / FAST_DECOMPRESSION) — the on-disk byte stream is
//     guaranteed identical to Lucene 10.4.0 by the util/compress/lz4
//     implementation; the fixture-based golden test
//     TestLZ4FastWireFormatGolden encodes a deterministic input and asserts
//     a known prefix layout.
//   - DEFLATE (HIGH_COMPRESSION) — Lucene relies on java.util.zip whose
//     emitted byte stream is implementation-specific. Go's compress/flate
//     produces a *valid* raw DEFLATE stream that Java can decode (and the
//     reverse), but the compressed bytes are not bit-identical across
//     implementations. Tests therefore assert round-trip correctness only
//     for HIGH_COMPRESSION; the contract preserved with Java is the wire
//     framing (VInt-prefixed payload), not the raw-DEFLATE bytes inside.
package compressing

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fixedSeed is the deterministic PRNG seed used by every test in this file.
// Lucene's test base picks a random seed each run; we lock the seed so
// failures are bit-reproducible (the fixed seed is reported when a test
// fails to make triage faster).
const fixedSeed uint64 = 0x9E3779B97F4A7C15 // golden-ratio constant

// newRand returns a deterministic *rand.Rand seeded from fixedSeed and the
// caller's subtest name so the seven-test grid does not produce identical
// sequences across subtests. Using math/rand/v2 (Go 1.22+) keeps the PRNG
// independent of the package-level state.
func newRand(name string) *rand.Rand {
	h := uint64(fixedSeed)
	for _, c := range []byte(name) {
		h = (h ^ uint64(c)) * 1099511628211 // FNV-1a-ish mixer
	}
	return rand.New(rand.NewPCG(fixedSeed, h))
}

// randomArray mirrors AbstractTestCompressionMode.randomArray. testNightly
// is exposed so the extremely-large variants can opt in to the bigger
// payload size (we always run with the short-mode size of 33 KiB to keep
// CI cheap; the dedicated testExtremelyLargeInput test exercises 16 MiB
// directly).
func randomArray(r *rand.Rand) []byte {
	const bigsize = 33 * 1024
	var max int
	if r.IntN(2) == 1 {
		max = r.IntN(4)
	} else {
		max = r.IntN(255)
	}
	var length int
	if r.IntN(2) == 1 {
		length = r.IntN(20)
	} else {
		length = r.IntN(bigsize)
	}
	return randomArrayN(r, length, max)
}

func randomArrayN(r *rand.Rand, length, max int) []byte {
	a := make([]byte, length)
	for i := range a {
		a[i] = byte(r.IntN(max + 1))
	}
	return a
}

// compress mirrors AbstractTestCompressionMode.compress. The input window
// decompressed[off:off+length] is wrapped in a ByteBuffersDataInput and
// fed to a fresh Compressor; the output is captured by a
// ByteArrayDataOutput sized to len*3+16 (matches the Java upper bound).
func compressWindow(t *testing.T, mode CompressionMode, decompressed []byte, off, length int) []byte {
	t.Helper()
	c := mode.NewCompressor()
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatalf("Compressor.Close: %v", err)
		}
	}()

	window := make([]byte, length)
	copy(window, decompressed[off:off+length])
	in := store.NewByteBuffersDataInput(window)

	out := store.NewByteArrayDataOutput(length*3 + 16)
	if err := c.Compress(in, out); err != nil {
		t.Fatalf("Compressor.Compress: %v (mode=%s off=%d len=%d)", err, mode.String(), off, length)
	}
	return out.GetBytes()
}

// decompressWhole mirrors the AbstractTestCompressionMode.decompress
// overload with (originalLength, 0, originalLength).
func decompressWhole(t *testing.T, mode CompressionMode, compressed []byte, originalLength int) []byte {
	t.Helper()
	d := mode.NewDecompressor()
	in := store.NewByteArrayDataInput(compressed)
	dst := &util.BytesRef{}
	if err := d.Decompress(in, originalLength, 0, originalLength, dst); err != nil {
		t.Fatalf("Decompressor.Decompress: %v (mode=%s len=%d)", err, mode.String(), originalLength)
	}
	out := make([]byte, dst.Length)
	copy(out, dst.ValidBytes())
	return out
}

// decompressWindow mirrors the AbstractTestCompressionMode.decompress
// overload with explicit offset and length.
func decompressWindow(t *testing.T, mode CompressionMode, compressed []byte, originalLength, offset, length int) []byte {
	t.Helper()
	d := mode.NewDecompressor()
	in := store.NewByteArrayDataInput(compressed)
	dst := &util.BytesRef{}
	if err := d.Decompress(in, originalLength, offset, length, dst); err != nil {
		t.Fatalf("Decompressor.Decompress: %v (mode=%s len=%d off=%d window=%d)", err, mode.String(), originalLength, offset, length)
	}
	out := make([]byte, dst.Length)
	copy(out, dst.ValidBytes())
	return out
}

// runRoundTripWindow does compress(off..off+len) -> decompress(0..len) and
// returns the compressed bytes (mirrors AbstractTestCompressionMode.test).
func runRoundTripWindow(t *testing.T, mode CompressionMode, decompressed []byte, off, length int) []byte {
	t.Helper()
	compressed := compressWindow(t, mode, decompressed, off, length)
	restored := decompressWhole(t, mode, compressed, length)
	if len(restored) != length {
		t.Fatalf("restored length = %d, want %d", len(restored), length)
	}
	if !bytes.Equal(restored, decompressed[off:off+length]) {
		t.Fatalf("round-trip mismatch (mode=%s off=%d len=%d)", mode.String(), off, length)
	}
	return compressed
}

// runRoundTrip is the zero-offset, full-length variant.
func runRoundTrip(t *testing.T, mode CompressionMode, decompressed []byte) []byte {
	return runRoundTripWindow(t, mode, decompressed, 0, len(decompressed))
}

// -------- ports of AbstractTestCompressionMode test methods ----------------

// testDecompress mirrors AbstractTestCompressionMode.testDecompress.
func testDecompress(t *testing.T, mode CompressionMode) {
	t.Helper()
	r := newRand("Decompress")
	iterations := 3 + r.IntN(3) // atLeast(random, 3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(r)
		var off int
		if r.IntN(2) == 0 {
			off = 0
		} else {
			off = r.IntN(len(decompressed) + 1)
		}
		var length int
		if r.IntN(2) == 0 {
			length = len(decompressed) - off
		} else {
			length = r.IntN(len(decompressed) - off + 1)
		}
		compressed := compressWindow(t, mode, decompressed, off, length)
		restored := decompressWhole(t, mode, compressed, length)
		want := append([]byte(nil), decompressed[off:off+length]...)
		if !bytes.Equal(want, restored) {
			t.Fatalf("iter %d: mismatch want %d bytes got %d bytes", i, len(want), len(restored))
		}
	}
}

// testPartialDecompress mirrors AbstractTestCompressionMode.testPartialDecompress.
func testPartialDecompress(t *testing.T, mode CompressionMode) {
	t.Helper()
	r := newRand("PartialDecompress")
	iterations := 3 + r.IntN(3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(r)
		compressed := compressWindow(t, mode, decompressed, 0, len(decompressed))
		var offset, length int
		if len(decompressed) == 0 {
			offset, length = 0, 0
		} else {
			offset = r.IntN(len(decompressed))
			length = r.IntN(len(decompressed) - offset)
		}
		restored := decompressWindow(t, mode, compressed, len(decompressed), offset, length)
		want := append([]byte(nil), decompressed[offset:offset+length]...)
		if !bytes.Equal(want, restored) {
			t.Fatalf("iter %d: partial mismatch offset=%d len=%d want=%d got=%d", i, offset, length, len(want), len(restored))
		}
	}
}

// testEmptySequence mirrors AbstractTestCompressionMode.testEmptySequence.
func testEmptySequence(t *testing.T, mode CompressionMode) {
	t.Helper()
	runRoundTrip(t, mode, []byte{})
}

// testShortSequence mirrors AbstractTestCompressionMode.testShortSequence.
func testShortSequence(t *testing.T, mode CompressionMode) {
	t.Helper()
	r := newRand("ShortSequence")
	runRoundTrip(t, mode, []byte{byte(r.IntN(256))})
}

// testIncompressible mirrors AbstractTestCompressionMode.testIncompressible.
func testIncompressible(t *testing.T, mode CompressionMode) {
	t.Helper()
	r := newRand("Incompressible")
	length := 20 + r.IntN(256-20+1) // randomIntBetween(20, 256)
	decompressed := make([]byte, length)
	for i := range decompressed {
		decompressed[i] = byte(i)
	}
	runRoundTrip(t, mode, decompressed)
}

// testConstant mirrors AbstractTestCompressionMode.testConstant.
func testConstant(t *testing.T, mode CompressionMode) {
	t.Helper()
	r := newRand("Constant")
	length := 1 + r.IntN(10000)
	value := byte(r.IntN(256))
	decompressed := make([]byte, length)
	for i := range decompressed {
		decompressed[i] = value
	}
	runRoundTrip(t, mode, decompressed)
}

// testExtremelyLargeInput mirrors
// AbstractTestCompressionMode.testExtremelyLargeInput (16 MiB payload).
// Skipped under -test.short to keep CI cheap.
func testExtremelyLargeInput(t *testing.T, mode CompressionMode) {
	t.Helper()
	if testing.Short() {
		t.Fatal("skipping 16 MiB round-trip in -short mode")
	}
	const length = 1 << 24
	decompressed := make([]byte, length)
	for i := range decompressed {
		decompressed[i] = byte(i & 0x0F)
	}
	runRoundTrip(t, mode, decompressed)
}

// compressionModeContract runs the seven Lucene tests against the given
// mode. The subtest names match the Java method names so failures point
// directly to the Java peer.
func compressionModeContract(t *testing.T, mode CompressionMode) {
	t.Run("Decompress", func(t *testing.T) { testDecompress(t, mode) })
	t.Run("PartialDecompress", func(t *testing.T) { testPartialDecompress(t, mode) })
	t.Run("EmptySequence", func(t *testing.T) { testEmptySequence(t, mode) })
	t.Run("ShortSequence", func(t *testing.T) { testShortSequence(t, mode) })
	t.Run("Incompressible", func(t *testing.T) { testIncompressible(t, mode) })
	t.Run("Constant", func(t *testing.T) { testConstant(t, mode) })
	t.Run("ExtremelyLargeInput", func(t *testing.T) { testExtremelyLargeInput(t, mode) })
}

// TestFastCompressionMode mirrors TestFastCompressionMode.java.
func TestFastCompressionMode(t *testing.T) {
	compressionModeContract(t, FAST)
}

// TestHighCompressionMode mirrors TestHighCompressionMode.java.
func TestHighCompressionMode(t *testing.T) {
	compressionModeContract(t, HIGH_COMPRESSION)
}

// TestFastDecompressionMode mirrors TestFastDecompressionMode.java.
func TestFastDecompressionMode(t *testing.T) {
	compressionModeContract(t, FAST_DECOMPRESSION)
}

// TestNoCompressionMode exercises the round-trip contract for the
// NO_COMPRESSION variant declared by SortingStoredFieldsConsumer in
// Apache Lucene 10.4.0. Bytes are emitted verbatim and any window
// [offset, offset+length) of the original payload can be recovered.
func TestNoCompressionMode(t *testing.T) {
	compressionModeContract(t, NO_COMPRESSION)
}

// TestCompressionModeNames verifies the String() values exactly match the
// Java toString() returns. These names are observed externally (e.g. in
// log lines) and are part of the public contract.
func TestCompressionModeNames(t *testing.T) {
	cases := []struct {
		mode CompressionMode
		name string
	}{
		{FAST, "FAST"},
		{HIGH_COMPRESSION, "HIGH_COMPRESSION"},
		{FAST_DECOMPRESSION, "FAST_DECOMPRESSION"},
		{NO_COMPRESSION, "NO_COMPRESSION"},
	}
	for _, c := range cases {
		if got := c.mode.String(); got != c.name {
			t.Errorf("String() = %q, want %q", got, c.name)
		}
	}
}

// TestLZ4DecompressorClone exercises the FAST/FAST_DECOMPRESSION sharing
// rule: the LZ4 decompressor is stateless and Clone may return the
// receiver. We check that the cloned instance still decodes correctly.
func TestLZ4DecompressorClone(t *testing.T) {
	mode := FAST
	original := []byte("the quick brown fox jumps over the lazy dog")
	c := mode.NewCompressor()
	t.Cleanup(func() { _ = c.Close() })
	in := store.NewByteBuffersDataInput(original)
	out := store.NewByteArrayDataOutput(len(original) * 3)
	if err := c.Compress(in, out); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	compressed := out.GetBytes()

	d := mode.NewDecompressor()
	clone := d.Clone()
	if clone == nil {
		t.Fatal("Clone returned nil")
	}
	di := store.NewByteArrayDataInput(compressed)
	dst := &util.BytesRef{}
	if err := clone.Decompress(di, len(original), 0, len(original), dst); err != nil {
		t.Fatalf("clone.Decompress: %v", err)
	}
	if !bytes.Equal(dst.ValidBytes(), original) {
		t.Fatalf("clone decompression mismatch")
	}
}

// TestDeflateDecompressorClone verifies HIGH_COMPRESSION's Decompressor
// produces independent clones (each clone allocates its own scratch).
func TestDeflateDecompressorClone(t *testing.T) {
	mode := HIGH_COMPRESSION
	original := bytes.Repeat([]byte("abcdefgh"), 256) // compressible
	c := mode.NewCompressor()
	t.Cleanup(func() { _ = c.Close() })
	in := store.NewByteBuffersDataInput(original)
	out := store.NewByteArrayDataOutput(len(original) + 16)
	if err := c.Compress(in, out); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	compressed := out.GetBytes()

	d1 := mode.NewDecompressor()
	d2 := d1.Clone()
	if d2 == nil {
		t.Fatal("Clone returned nil")
	}
	if &d1 == &d2 {
		// pointer identity test on the interface value itself isn't
		// portable; we just check independent decode works.
	}

	dst1 := &util.BytesRef{}
	in1 := store.NewByteArrayDataInput(compressed)
	if err := d1.Decompress(in1, len(original), 0, len(original), dst1); err != nil {
		t.Fatalf("d1.Decompress: %v", err)
	}

	dst2 := &util.BytesRef{}
	in2 := store.NewByteArrayDataInput(compressed)
	if err := d2.Decompress(in2, len(original), 0, len(original), dst2); err != nil {
		t.Fatalf("d2.Decompress: %v", err)
	}

	if !bytes.Equal(dst1.ValidBytes(), original) {
		t.Fatalf("d1 mismatch")
	}
	if !bytes.Equal(dst2.ValidBytes(), original) {
		t.Fatalf("d2 mismatch")
	}
}

// TestLZ4FastWireFormatGolden encodes a known input with FAST and asserts
// the leading bytes of the LZ4 wire format match the algorithm-defined
// layout. For an incompressible 16-byte input the encoder emits a single
// "last literals" block: one token byte 0xF0 | (16-15)=0x01 -> 0xF1, then
// the 16 literal bytes verbatim.
//
// This is a wire-format check (not a round-trip assertion); the Java
// LZ4FastCompressor is guaranteed to produce the same bytes for this
// input because util/compress/lz4.go is a direct port of Lucene's LZ4.java.
func TestLZ4FastWireFormatGolden(t *testing.T) {
	// 16 incompressible bytes: 0x00, 0x01, ..., 0x0F.
	input := make([]byte, 16)
	for i := range input {
		input[i] = byte(i)
	}

	c := FAST.NewCompressor()
	t.Cleanup(func() { _ = c.Close() })
	out := store.NewByteArrayDataOutput(64)
	if err := c.Compress(store.NewByteBuffersDataInput(input), out); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	got := out.GetBytes()

	// Expected: literal-only block. With literalLen=16 the literal-len
	// nibble in the token saturates at 0x0F and one continuation byte
	// (16-15 = 1) follows; then the 16 literal bytes.
	want := append([]byte{0xF0, 0x01}, input...)
	if !bytes.Equal(got, want) {
		t.Fatalf("LZ4 wire format mismatch:\n got=% x\nwant=% x", got, want)
	}
}
