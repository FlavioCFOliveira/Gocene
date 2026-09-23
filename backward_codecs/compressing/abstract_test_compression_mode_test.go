// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

// Port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/compressing/AbstractTestCompressionMode.java
// (Apache Lucene 10.5.0). The abstract test class is rendered as
// abstractTestCompressionMode, whose mode field is set by the setUp() of each
// concrete subclass (test_fast_compression_mode_test.go,
// test_fast_decompression_mode_test.go, test_high_compression_mode_test.go);
// runAbstractTestCompressionMode runs one inherited test method.

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type abstractTestCompressionMode struct {
	t    *testing.T
	r    *rand.Rand
	mode CompressionMode
}

func newAbstractTestCompressionMode(t *testing.T, mode CompressionMode) *abstractTestCompressionMode {
	t.Helper()
	seed := time.Now().UnixNano()
	t.Logf("random seed: %d", seed)
	return &abstractTestCompressionMode{t: t, r: rand.New(rand.NewSource(seed)), mode: mode}
}

// nextIntInclusive renders TestUtil.nextInt / RandomNumbers.randomIntBetween:
// both bounds inclusive.
func nextIntInclusive(r *rand.Rand, start, end int) int {
	return start + r.Intn(end-start+1)
}

func compressionAtLeast(r *rand.Rand, i int) int {
	return nextIntInclusive(r, i, i+i/2)
}

func randomArray(random *rand.Rand) []byte {
	bigsize := 33 * 1024 // TEST_NIGHTLY ? 192 * 1024 : 33 * 1024
	var maxV int
	if random.Intn(2) == 0 {
		maxV = random.Intn(4)
	} else {
		maxV = random.Intn(255)
	}
	var length int
	if random.Intn(2) == 0 {
		length = random.Intn(20)
	} else {
		length = random.Intn(bigsize)
	}
	return randomArrayOf(random, length, maxV)
}

func randomArrayOf(random *rand.Rand, length, maxV int) []byte {
	arr := make([]byte, length)
	for i := range arr {
		arr[i] = byte(nextIntInclusive(random, 0, maxV))
	}
	return arr
}

func (a *abstractTestCompressionMode) compress(decompressed []byte, off, length int) []byte {
	compressor := a.mode.NewCompressor()
	return compressWith(a.t, compressor, decompressed, off, length)
}

func compressWith(t *testing.T, compressor Compressor, decompressed []byte, off, length int) []byte {
	t.Helper()
	compressed := make([]byte, length*2+16) // should be enough
	out := store.NewByteArrayDataOutput(compressed)
	if err := compressor.Compress(decompressed, off, length, out); err != nil {
		t.Fatalf("compress: %v", err)
	}
	compressedLen := out.GetPosition()
	return bytes.Clone(compressed[:compressedLen])
}

func (a *abstractTestCompressionMode) decompress(compressed []byte, originalLength int) []byte {
	decompressor := a.mode.NewDecompressor()
	return decompressWith(a.t, decompressor, compressed, originalLength)
}

func decompressWith(t *testing.T, decompressor Decompressor, compressed []byte, originalLength int) []byte {
	t.Helper()
	b := util.NewBytesRefEmpty()
	if err := decompressor.Decompress(store.NewByteArrayDataInput(compressed), originalLength, 0, originalLength, b); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return bytes.Clone(b.ValidBytes()) // BytesRef.deepCopyOf(bytes).bytes
}

func (a *abstractTestCompressionMode) decompressPartial(compressed []byte, originalLength, offset, length int) []byte {
	decompressor := a.mode.NewDecompressor()
	b := util.NewBytesRefEmpty()
	if err := decompressor.Decompress(store.NewByteArrayDataInput(compressed), originalLength, offset, length, b); err != nil {
		a.t.Fatalf("decompress: %v", err)
	}
	return bytes.Clone(b.ValidBytes())
}

func assertArrayEquals(t *testing.T, expected, actual []byte) {
	t.Helper()
	if !bytes.Equal(expected, actual) {
		t.Fatalf("arrays differ: expected length %d, actual length %d", len(expected), len(actual))
	}
}

func (a *abstractTestCompressionMode) testDecompress() {
	random := a.r
	iterations := compressionAtLeast(random, 3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(random)
		off := 0
		if random.Intn(2) != 0 {
			off = nextIntInclusive(random, 0, len(decompressed))
		}
		var length int
		if random.Intn(2) == 0 {
			length = len(decompressed) - off
		} else {
			length = nextIntInclusive(random, 0, len(decompressed)-off)
		}
		compressed := a.compress(decompressed, off, length)
		restored := a.decompress(compressed, length)
		assertArrayEquals(a.t, decompressed[off:off+length], restored)
	}
}

func (a *abstractTestCompressionMode) testPartialDecompress() {
	random := a.r
	iterations := compressionAtLeast(random, 3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(random)
		compressed := a.compress(decompressed, 0, len(decompressed))
		var offset, length int
		if len(decompressed) == 0 {
			offset, length = 0, 0
		} else {
			offset = random.Intn(len(decompressed))
			length = random.Intn(len(decompressed) - offset)
		}
		restored := a.decompressPartial(compressed, len(decompressed), offset, length)
		assertArrayEquals(a.t, decompressed[offset:offset+length], restored)
	}
}

func (a *abstractTestCompressionMode) test(decompressed []byte) []byte {
	return a.testRange(decompressed, 0, len(decompressed))
}

func (a *abstractTestCompressionMode) testRange(decompressed []byte, off, length int) []byte {
	compressed := a.compress(decompressed, off, length)
	restored := a.decompress(compressed, length)
	if len(restored) != length {
		a.t.Fatalf("restored length %d, want %d", len(restored), length)
	}
	return compressed
}

func (a *abstractTestCompressionMode) testEmptySequence() {
	a.test([]byte{})
}

func (a *abstractTestCompressionMode) testShortSequence() {
	a.test([]byte{byte(a.r.Intn(256))})
}

func (a *abstractTestCompressionMode) testIncompressible() {
	decompressed := make([]byte, nextIntInclusive(a.r, 20, 256))
	for i := range decompressed {
		decompressed[i] = byte(i)
	}
	a.test(decompressed)
}

func (a *abstractTestCompressionMode) testConstant() {
	decompressed := make([]byte, nextIntInclusive(a.r, 1, 10000))
	v := byte(a.r.Int31())
	for i := range decompressed {
		decompressed[i] = v
	}
	a.test(decompressed)
}
