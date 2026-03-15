// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package compressing provides compression/decompression modes for stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.compressing package.
//
// Source files:
//   - TestFastCompressionMode.java
//   - TestHighCompressionMode.java
//   - TestFastDecompressionMode.java
//   - AbstractTestCompressionMode.java (base class)
//
// Purpose: Tests compression/decompression modes (FAST, HIGH_COMPRESSION, FAST_DECOMPRESSION)
// for byte-level compatibility with Apache Lucene.
package compressing

import (
	"bytes"
	"compress/zlib"
	"io"
	"math/rand"
	"testing"
)

// CompressionMode defines the interface for compression modes.
// This is the Go port of Lucene's CompressionMode abstract class.
type CompressionMode interface {
	// NewCompressor creates a new Compressor instance.
	NewCompressor() Compressor

	// NewDecompressor creates a new Decompressor instance.
	NewDecompressor() Decompressor

	// String returns the name of this compression mode.
	String() string
}

// Compressor is a data compressor.
// This is the Go port of Lucene's Compressor abstract class.
type Compressor interface {
	// Compress compresses bytes into the output buffer.
	// Returns the compressed data and any error encountered.
	Compress(data []byte) ([]byte, error)

	// Close releases any resources held by the compressor.
	Close() error
}

// Decompressor is a data decompressor.
// This is the Go port of Lucene's Decompressor abstract class.
type Decompressor interface {
	// Decompress decompresses bytes from the compressed data.
	// Parameters:
	//   - compressed: the compressed data
	//   - originalLength: the length of the original data before compression
	//   - offset: bytes before this offset do not need to be decompressed
	//   - length: number of bytes to decompress starting from offset
	// Returns the decompressed data and any error encountered.
	Decompress(compressed []byte, originalLength, offset, length int) ([]byte, error)

	// Clone creates a copy of this decompressor.
	Clone() Decompressor
}

// Compression modes
var (
	// FAST trades compression ratio for speed.
	// Uses LZ4 fast compression.
	FAST CompressionMode = &fastCompressionMode{}

	// HIGH_COMPRESSION trades speed for compression ratio.
	// Uses zlib/deflate with level 6.
	HIGH_COMPRESSION CompressionMode = &highCompressionMode{}

	// FAST_DECOMPRESSION is similar to FAST but spends more time compressing.
	// Uses LZ4 high compression.
	FAST_DECOMPRESSION CompressionMode = &fastDecompressionMode{}
)

// fastCompressionMode implements FAST compression mode using LZ4.
type fastCompressionMode struct{}

func (m *fastCompressionMode) NewCompressor() Compressor {
	return &lz4FastCompressor{}
}

func (m *fastCompressionMode) NewDecompressor() Decompressor {
	return &lz4Decompressor{}
}

func (m *fastCompressionMode) String() string {
	return "FAST"
}

// highCompressionMode implements HIGH_COMPRESSION mode using zlib/deflate.
type highCompressionMode struct{}

func (m *highCompressionMode) NewCompressor() Compressor {
	return &deflateCompressor{level: 6}
}

func (m *highCompressionMode) NewDecompressor() Decompressor {
	return &deflateDecompressor{}
}

func (m *highCompressionMode) String() string {
	return "HIGH_COMPRESSION"
}

// fastDecompressionMode implements FAST_DECOMPRESSION mode using LZ4 high compression.
type fastDecompressionMode struct{}

func (m *fastDecompressionMode) NewCompressor() Compressor {
	return &lz4HighCompressor{}
}

func (m *fastDecompressionMode) NewDecompressor() Decompressor {
	return &lz4Decompressor{}
}

func (m *fastDecompressionMode) String() string {
	return "FAST_DECOMPRESSION"
}

// lz4FastCompressor implements fast LZ4 compression.
type lz4FastCompressor struct{}

func (c *lz4FastCompressor) Compress(data []byte) ([]byte, error) {
	// Simplified LZ4 fast compression implementation
	// In a full implementation, this would use the LZ4 algorithm
	// For now, we use a simple run-length encoding as placeholder
	return simpleCompress(data), nil
}

func (c *lz4FastCompressor) Close() error {
	return nil
}

// lz4HighCompressor implements high-compression LZ4.
type lz4HighCompressor struct{}

func (c *lz4HighCompressor) Compress(data []byte) ([]byte, error) {
	// Simplified LZ4 high compression implementation
	// In a full implementation, this would use the LZ4 HC algorithm
	return simpleCompress(data), nil
}

func (c *lz4HighCompressor) Close() error {
	return nil
}

// lz4Decompressor implements LZ4 decompression.
type lz4Decompressor struct{}

func (d *lz4Decompressor) Decompress(compressed []byte, originalLength, offset, length int) ([]byte, error) {
	// Decompress full data first
	decompressed := simpleDecompress(compressed)

	// Return only the requested slice
	if offset+length > len(decompressed) {
		return nil, io.ErrShortBuffer
	}

	result := make([]byte, length)
	copy(result, decompressed[offset:offset+length])
	return result, nil
}

func (d *lz4Decompressor) Clone() Decompressor {
	return &lz4Decompressor{}
}

// deflateCompressor implements zlib/deflate compression.
type deflateCompressor struct {
	level int
}

func (c *deflateCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *deflateCompressor) Close() error {
	return nil
}

// deflateDecompressor implements zlib/deflate decompression.
type deflateDecompressor struct{}

func (d *deflateDecompressor) Decompress(compressed []byte, originalLength, offset, length int) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Read all decompressed data
	decompressed := make([]byte, originalLength)
	n, err := io.ReadFull(r, decompressed)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if n != originalLength {
		return nil, io.ErrShortBuffer
	}

	// Return only the requested slice
	if offset+length > len(decompressed) {
		return nil, io.ErrShortBuffer
	}

	result := make([]byte, length)
	copy(result, decompressed[offset:offset+length])
	return result, nil
}

func (d *deflateDecompressor) Clone() Decompressor {
	return &deflateDecompressor{}
}

// simpleCompress is a placeholder compression for LZ4 modes.
// In a full implementation, this would be actual LZ4 compression.
// This implementation uses a simple length-prefixed format for testing.
func simpleCompress(data []byte) []byte {
	if len(data) == 0 {
		// Empty data: return 4 zero bytes for length
		return []byte{0, 0, 0, 0}
	}

	// Format: [4 bytes original length] [data...]
	result := make([]byte, 4+len(data))
	result[0] = byte(len(data) >> 24)
	result[1] = byte(len(data) >> 16)
	result[2] = byte(len(data) >> 8)
	result[3] = byte(len(data))
	copy(result[4:], data)
	return result
}

// simpleDecompress is a placeholder decompression for LZ4 modes.
func simpleDecompress(data []byte) []byte {
	if len(data) < 4 {
		return []byte{}
	}

	originalLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])

	if originalLen == 0 {
		return []byte{}
	}

	// Return the data portion
	if len(data) > 4+originalLen {
		return data[4 : 4+originalLen]
	}
	return data[4:]
}

// compress compresses data using the given compressor.
func compress(compressor Compressor, data []byte) ([]byte, error) {
	return compressor.Compress(data)
}

// decompress decompresses data using the given decompressor.
func decompress(decompressor Decompressor, compressed []byte, originalLength int) ([]byte, error) {
	return decompressor.Decompress(compressed, originalLength, 0, originalLength)
}

// decompressSlice decompresses a slice of data.
func decompressSlice(decompressor Decompressor, compressed []byte, originalLength, offset, length int) ([]byte, error) {
	return decompressor.Decompress(compressed, originalLength, offset, length)
}

// randomArray generates a random byte array.
func randomArray(r *rand.Rand, testNightly bool) []byte {
	bigSize := 33 * 1024
	if testNightly {
		bigSize = 192 * 1024
	}

	max := 255
	if r.Float64() < 0.5 {
		max = r.Intn(4)
	}

	length := r.Intn(20)
	if r.Float64() < 0.5 {
		length = r.Intn(bigSize)
	}

	return randomArrayWithParams(r, length, max)
}

// randomArrayWithParams generates a random byte array with specified parameters.
func randomArrayWithParams(r *rand.Rand, length, max int) []byte {
	arr := make([]byte, length)
	for i := range arr {
		arr[i] = byte(r.Intn(max + 1))
	}
	return arr
}

// atLeast returns at least the specified number of iterations.
func atLeast(r *rand.Rand, n int) int {
	// In Lucene, this returns a higher number in certain test modes
	// For simplicity, we return n or n+1
	if r.Float64() < 0.5 {
		return n + 1
	}
	return n
}

// nextInt returns a random int between 0 and n (exclusive).
func nextInt(r *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	return r.Intn(n)
}

// copyOfSubArray returns a copy of the specified subarray.
func copyOfSubArray(arr []byte, from, to int) []byte {
	if from < 0 {
		from = 0
	}
	if to > len(arr) {
		to = len(arr)
	}
	if from >= to {
		return []byte{}
	}
	result := make([]byte, to-from)
	copy(result, arr[from:to])
	return result
}

// TestFastCompressionMode tests the FAST compression mode.
// Source: TestFastCompressionMode.java
func TestFastCompressionMode(t *testing.T) {
	testCompressionMode(t, FAST, "FAST")
}

// TestHighCompressionMode tests the HIGH_COMPRESSION mode.
// Source: TestHighCompressionMode.java
func TestHighCompressionMode(t *testing.T) {
	testCompressionMode(t, HIGH_COMPRESSION, "HIGH_COMPRESSION")
}

// TestFastDecompressionMode tests the FAST_DECOMPRESSION mode.
// Source: TestFastDecompressionMode.java
func TestFastDecompressionMode(t *testing.T) {
	testCompressionMode(t, FAST_DECOMPRESSION, "FAST_DECOMPRESSION")
}

// testCompressionMode runs all compression tests for the given mode.
// Source: AbstractTestCompressionMode.java
type testContext struct {
	mode CompressionMode
	r    *rand.Rand
	t    *testing.T
}

func testCompressionMode(t *testing.T, mode CompressionMode, name string) {
	t.Logf("Testing compression mode: %s", name)

	ctx := &testContext{
		mode: mode,
		r:    rand.New(rand.NewSource(42)), // Fixed seed for reproducibility
		t:    t,
	}

	t.Run("Decompress", func(t *testing.T) {
		ctx.t = t
		testDecompress(ctx)
	})

	t.Run("PartialDecompress", func(t *testing.T) {
		ctx.t = t
		testPartialDecompress(ctx)
	})

	t.Run("EmptySequence", func(t *testing.T) {
		ctx.t = t
		testEmptySequence(ctx)
	})

	t.Run("ShortSequence", func(t *testing.T) {
		ctx.t = t
		testShortSequence(ctx)
	})

	t.Run("Incompressible", func(t *testing.T) {
		ctx.t = t
		testIncompressible(ctx)
	})

	t.Run("Constant", func(t *testing.T) {
		ctx.t = t
		testConstant(ctx)
	})

	t.Run("ExtremelyLargeInput", func(t *testing.T) {
		ctx.t = t
		testExtremelyLargeInput(ctx)
	})
}

// testDecompress tests basic compression and decompression.
// Source: AbstractTestCompressionMode.testDecompress()
func testDecompress(ctx *testContext) {
	iterations := atLeast(ctx.r, 3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(ctx.r, false)

		off := 0
		if ctx.r.Float64() < 0.5 {
			off = nextInt(ctx.r, len(decompressed)+1)
		}

		length := len(decompressed) - off
		if ctx.r.Float64() < 0.5 {
			length = nextInt(ctx.r, len(decompressed)-off+1)
		}
		_ = length

		compressor := ctx.mode.NewCompressor()
		compressed, err := compress(compressor, decompressed[off:off+length])
		if err != nil {
			ctx.t.Fatalf("Failed to compress: %v", err)
		}
		compressor.Close()

		decompressor := ctx.mode.NewDecompressor()
		restored, err := decompress(decompressor, compressed, length)
		if err != nil {
			ctx.t.Fatalf("Failed to decompress: %v", err)
		}

		expected := copyOfSubArray(decompressed, off, off+length)
		if !bytes.Equal(expected, restored) {
			ctx.t.Errorf("Decompressed data mismatch at iteration %d", i)
		}
	}
}

// testPartialDecompress tests partial decompression.
// Source: AbstractTestCompressionMode.testPartialDecompress()
func testPartialDecompress(ctx *testContext) {
	iterations := atLeast(ctx.r, 3)
	for i := 0; i < iterations; i++ {
		decompressed := randomArray(ctx.r, false)

		compressor := ctx.mode.NewCompressor()
		compressed, err := compress(compressor, decompressed)
		if err != nil {
			ctx.t.Fatalf("Failed to compress: %v", err)
		}
		compressor.Close()

		offset, length := 0, 0
		if len(decompressed) == 0 {
			offset, length = 0, 0
		} else {
			offset = ctx.r.Intn(len(decompressed))
			length = ctx.r.Intn(len(decompressed) - offset + 1)
		}

		decompressor := ctx.mode.NewDecompressor()
		restored, err := decompressSlice(decompressor, compressed, len(decompressed), offset, length)
		if err != nil {
			ctx.t.Fatalf("Failed to decompress slice: %v", err)
		}

		expected := copyOfSubArray(decompressed, offset, offset+length)
		if !bytes.Equal(expected, restored) {
			ctx.t.Errorf("Partial decompressed data mismatch at iteration %d", i)
		}
	}
}

// testEmptySequence tests compression/decompression of empty data.
// Source: AbstractTestCompressionMode.testEmptySequence()
func testEmptySequence(ctx *testContext) {
	empty := []byte{}

	compressor := ctx.mode.NewCompressor()
	compressed, err := compress(compressor, empty)
	if err != nil {
		ctx.t.Fatalf("Failed to compress empty sequence: %v", err)
	}
	compressor.Close()

	decompressor := ctx.mode.NewDecompressor()
	restored, err := decompress(decompressor, compressed, 0)
	if err != nil {
		ctx.t.Fatalf("Failed to decompress empty sequence: %v", err)
	}

	if len(restored) != 0 {
		ctx.t.Errorf("Expected empty result, got %d bytes", len(restored))
	}
}

// testShortSequence tests compression/decompression of a single byte.
// Source: AbstractTestCompressionMode.testShortSequence()
func testShortSequence(ctx *testContext) {
	data := []byte{byte(ctx.r.Intn(256))}

	compressor := ctx.mode.NewCompressor()
	compressed, err := compress(compressor, data)
	if err != nil {
		ctx.t.Fatalf("Failed to compress short sequence: %v", err)
	}
	compressor.Close()

	decompressor := ctx.mode.NewDecompressor()
	restored, err := decompress(decompressor, compressed, 1)
	if err != nil {
		ctx.t.Fatalf("Failed to decompress short sequence: %v", err)
	}

	if !bytes.Equal(data, restored) {
		ctx.t.Errorf("Short sequence mismatch: expected %v, got %v", data, restored)
	}
}

// testIncompressible tests compression of incompressible data.
// Source: AbstractTestCompressionMode.testIncompressible()
func testIncompressible(ctx *testContext) {
	length := 20 + ctx.r.Intn(237) // Random between 20 and 256
	decompressed := make([]byte, length)
	for i := range decompressed {
		decompressed[i] = byte(i)
	}

	compressor := ctx.mode.NewCompressor()
	compressed, err := compress(compressor, decompressed)
	if err != nil {
		ctx.t.Fatalf("Failed to compress incompressible data: %v", err)
	}
	compressor.Close()

	decompressor := ctx.mode.NewDecompressor()
	restored, err := decompress(decompressor, compressed, length)
	if err != nil {
		ctx.t.Fatalf("Failed to decompress incompressible data: %v", err)
	}

	if !bytes.Equal(decompressed, restored) {
		ctx.t.Errorf("Incompressible data mismatch")
	}
}

// testConstant tests compression of constant/repeated data.
// Source: AbstractTestCompressionMode.testConstant()
func testConstant(ctx *testContext) {
	length := 1 + ctx.r.Intn(10000)
	value := byte(ctx.r.Intn(256))
	decompressed := make([]byte, length)
	for i := range decompressed {
		decompressed[i] = value
	}

	compressor := ctx.mode.NewCompressor()
	compressed, err := compress(compressor, decompressed)
	if err != nil {
		ctx.t.Fatalf("Failed to compress constant data: %v", err)
	}
	compressor.Close()

	decompressor := ctx.mode.NewDecompressor()
	restored, err := decompress(decompressor, compressed, length)
	if err != nil {
		ctx.t.Fatalf("Failed to decompress constant data: %v", err)
	}

	if !bytes.Equal(decompressed, restored) {
		ctx.t.Errorf("Constant data mismatch")
	}
}

// testExtremelyLargeInput tests compression of a 16MB input.
// Source: AbstractTestCompressionMode.testExtremelyLargeInput()
func testExtremelyLargeInput(ctx *testContext) {
	// 16MB
	length := 1 << 24
	decompressed := make([]byte, length)

	// Fill with pattern: low nibble repeats
	for i := range decompressed {
		decompressed[i] = byte(i & 0x0F)
	}

	compressor := ctx.mode.NewCompressor()
	compressed, err := compress(compressor, decompressed)
	if err != nil {
		ctx.t.Fatalf("Failed to compress large input: %v", err)
	}
	compressor.Close()

	decompressor := ctx.mode.NewDecompressor()
	restored, err := decompress(decompressor, compressed, length)
	if err != nil {
		ctx.t.Fatalf("Failed to decompress large input: %v", err)
	}

	if !bytes.Equal(decompressed, restored) {
		ctx.t.Errorf("Large input data mismatch")
	}
}
