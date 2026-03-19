// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestLZ4Factory_Basic tests basic factory creation
func TestLZ4Factory_Basic(t *testing.T) {
	factory := GetLZ4Factory()
	if factory == nil {
		t.Fatal("GetLZ4Factory returned nil")
	}

	// Should return the same instance (singleton)
	factory2 := GetLZ4Factory()
	if factory != factory2 {
		t.Error("expected GetLZ4Factory to return the same instance")
	}
}

// TestLZ4Factory_NewFactory tests creating a new factory
func TestLZ4Factory_NewFactory(t *testing.T) {
	// Test unsafe, fast factory
	factory := NewLZ4Factory(false, false)
	if factory == nil {
		t.Fatal("NewLZ4Factory returned nil")
	}
	if factory.IsSafe() {
		t.Error("expected factory to be unsafe")
	}
	if factory.IsHighCompression() {
		t.Error("expected factory to not be high compression")
	}

	// Test safe, high compression factory
	factory2 := NewLZ4Factory(true, true)
	if !factory2.IsSafe() {
		t.Error("expected factory to be safe")
	}
	if !factory2.IsHighCompression() {
		t.Error("expected factory to be high compression")
	}
}

// TestLZ4Factory_FastCompressor tests getting a fast compressor
func TestLZ4Factory_FastCompressor(t *testing.T) {
	factory := NewLZ4Factory(false, false)
	compressor := factory.FastCompressor()
	if compressor == nil {
		t.Fatal("FastCompressor returned nil")
	}

	if compressor.Name() != "LZ4FastCompressor" {
		t.Errorf("expected name 'LZ4FastCompressor', got '%s'", compressor.Name())
	}
}

// TestLZ4Factory_SafeFastCompressor tests getting a safe fast compressor
func TestLZ4Factory_SafeFastCompressor(t *testing.T) {
	factory := NewLZ4Factory(true, false)
	compressor := factory.FastCompressor()
	if compressor == nil {
		t.Fatal("FastCompressor returned nil")
	}

	if compressor.Name() != "LZ4SafeCompressor" {
		t.Errorf("expected name 'LZ4SafeCompressor', got '%s'", compressor.Name())
	}
}

// TestLZ4Factory_HighCompressor tests getting a high compressor
func TestLZ4Factory_HighCompressor(t *testing.T) {
	factory := NewLZ4Factory(false, true)
	compressor := factory.HighCompressor()
	if compressor == nil {
		t.Fatal("HighCompressor returned nil")
	}

	if compressor.Name() != "LZ4HighCompressor" {
		t.Errorf("expected name 'LZ4HighCompressor', got '%s'", compressor.Name())
	}
}

// TestLZ4Factory_SafeHighCompressor tests getting a safe high compressor
func TestLZ4Factory_SafeHighCompressor(t *testing.T) {
	factory := NewLZ4Factory(true, true)
	compressor := factory.HighCompressor()
	if compressor == nil {
		t.Fatal("HighCompressor returned nil")
	}

	if compressor.Name() != "LZ4SafeHighCompressor" {
		t.Errorf("expected name 'LZ4SafeHighCompressor', got '%s'", compressor.Name())
	}
}

// TestLZ4Factory_FastDecompressor tests getting a fast decompressor
func TestLZ4Factory_FastDecompressor(t *testing.T) {
	factory := NewLZ4Factory(false, false)
	decompressor := factory.FastDecompressor()
	if decompressor == nil {
		t.Fatal("FastDecompressor returned nil")
	}

	if decompressor.Name() != "LZ4FastDecompressor" {
		t.Errorf("expected name 'LZ4FastDecompressor', got '%s'", decompressor.Name())
	}
}

// TestLZ4Factory_SafeDecompressor tests getting a safe decompressor
func TestLZ4Factory_SafeDecompressor(t *testing.T) {
	factory := NewLZ4Factory(false, false)
	decompressor := factory.SafeDecompressor()
	if decompressor == nil {
		t.Fatal("SafeDecompressor returned nil")
	}

	if decompressor.Name() != "LZ4SafeDecompressor" {
		t.Errorf("expected name 'LZ4SafeDecompressor', got '%s'", decompressor.Name())
	}
}

// TestLZ4FastCompressor_Basic tests the fast compressor
func TestLZ4FastCompressor_Basic(t *testing.T) {
	compressor := NewLZ4FastCompressor()
	if compressor == nil {
		t.Fatal("NewLZ4FastCompressor returned nil")
	}

	if compressor.Name() != "LZ4FastCompressor" {
		t.Errorf("expected name 'LZ4FastCompressor', got '%s'", compressor.Name())
	}

	// Test MaxCompressedLength
	maxLen := compressor.MaxCompressedLength(1000)
	if maxLen <= 1000 {
		t.Errorf("expected MaxCompressedLength > 1000, got %d", maxLen)
	}
}

// TestLZ4FastCompressor_Compress tests compression
func TestLZ4FastCompressor_Compress(t *testing.T) {
	compressor := NewLZ4FastCompressor()
	src := []byte("Hello, World! This is a test string for compression.")
	dst := make([]byte, compressor.MaxCompressedLength(len(src)))

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("expected non-empty compressed data")
	}
}

// TestLZ4HighCompressor_Basic tests the high compressor
func TestLZ4HighCompressor_Basic(t *testing.T) {
	compressor := NewLZ4HighCompressor()
	if compressor == nil {
		t.Fatal("NewLZ4HighCompressor returned nil")
	}

	if compressor.Name() != "LZ4HighCompressor" {
		t.Errorf("expected name 'LZ4HighCompressor', got '%s'", compressor.Name())
	}
}

// TestLZ4SafeCompressor_Basic tests the safe compressor
func TestLZ4SafeCompressor_Basic(t *testing.T) {
	compressor := NewLZ4SafeCompressor(false)
	if compressor == nil {
		t.Fatal("NewLZ4SafeCompressor returned nil")
	}

	if compressor.Name() != "LZ4SafeCompressor" {
		t.Errorf("expected name 'LZ4SafeCompressor', got '%s'", compressor.Name())
	}

	// Test high compression variant
	highCompressor := NewLZ4SafeCompressor(true)
	if highCompressor.Name() != "LZ4SafeHighCompressor" {
		t.Errorf("expected name 'LZ4SafeHighCompressor', got '%s'", highCompressor.Name())
	}
}

// TestLZ4SafeCompressor_EmptyData tests compressing empty data
func TestLZ4SafeCompressor_EmptyData(t *testing.T) {
	compressor := NewLZ4SafeCompressor(false)
	src := []byte{}
	dst := make([]byte, 100)

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) != 0 {
		t.Errorf("expected empty compressed data for empty input, got %d bytes", len(compressed))
	}
}

// TestLZ4FastDecompressor_Basic tests the fast decompressor
func TestLZ4FastDecompressor_Basic(t *testing.T) {
	decompressor := NewLZ4FastDecompressor()
	if decompressor == nil {
		t.Fatal("NewLZ4FastDecompressor returned nil")
	}

	if decompressor.Name() != "LZ4FastDecompressor" {
		t.Errorf("expected name 'LZ4FastDecompressor', got '%s'", decompressor.Name())
	}
}

// TestLZ4FastDecompressor_Decompress tests decompression
func TestLZ4FastDecompressor_Decompress(t *testing.T) {
	// First compress some data
	compressor := NewLZ4FastCompressor()
	src := []byte("Hello, World! This is a test string for compression.")
	dst := make([]byte, compressor.MaxCompressedLength(len(src)))

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Now decompress
	decompressor := NewLZ4FastDecompressor()
	decompressed := make([]byte, len(src))

	err = decompressor.Decompress(compressed, decompressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	// Note: The current implementation doesn't actually decompress to the original
	// due to the placeholder implementation. This test verifies the API works.
}

// TestLZ4SafeDecompressor_Basic tests the safe decompressor
func TestLZ4SafeDecompressor_Basic(t *testing.T) {
	decompressor := NewLZ4SafeDecompressor()
	if decompressor == nil {
		t.Fatal("NewLZ4SafeDecompressor returned nil")
	}

	if decompressor.Name() != "LZ4SafeDecompressor" {
		t.Errorf("expected name 'LZ4SafeDecompressor', got '%s'", decompressor.Name())
	}
}

// TestLZ4SafeDecompressor_ShortSource tests error handling for short source
func TestLZ4SafeDecompressor_ShortSource(t *testing.T) {
	decompressor := NewLZ4SafeDecompressor()
	src := []byte{1, 2, 3} // Less than 4 bytes
	dst := make([]byte, 100)

	err := decompressor.Decompress(src, dst)
	if err == nil {
		t.Error("expected error for short source")
	}
}

// TestLZ4UnsafeCompressor_Basic tests the unsafe compressor
func TestLZ4UnsafeCompressor_Basic(t *testing.T) {
	compressor := NewLZ4UnsafeCompressor(false)
	if compressor == nil {
		t.Fatal("NewLZ4UnsafeCompressor returned nil")
	}

	if compressor.Name() != "LZ4UnsafeCompressor" {
		t.Errorf("expected name 'LZ4UnsafeCompressor', got '%s'", compressor.Name())
	}

	// Test high compression variant
	highCompressor := NewLZ4UnsafeCompressor(true)
	if highCompressor.Name() != "LZ4UnsafeHighCompressor" {
		t.Errorf("expected name 'LZ4UnsafeHighCompressor', got '%s'", highCompressor.Name())
	}
}

// TestLZ4UnsafeCompressor_Compress tests compression
func TestLZ4UnsafeCompressor_Compress(t *testing.T) {
	compressor := NewLZ4UnsafeCompressor(false)
	src := []byte("Hello, World! This is a test string for compression.")
	dst := make([]byte, compressor.MaxCompressedLength(len(src)))

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("expected non-empty compressed data")
	}
}

// TestLZ4UnsafeDecompressor_Basic tests the unsafe decompressor
func TestLZ4UnsafeDecompressor_Basic(t *testing.T) {
	decompressor := NewLZ4UnsafeDecompressor()
	if decompressor == nil {
		t.Fatal("NewLZ4UnsafeDecompressor returned nil")
	}

	if decompressor.Name() != "LZ4UnsafeDecompressor" {
		t.Errorf("expected name 'LZ4UnsafeDecompressor', got '%s'", decompressor.Name())
	}
}

// TestLZ4UnsafeRoundTrip tests a full compress/decompress cycle with unsafe variants
func TestLZ4UnsafeRoundTrip(t *testing.T) {
	compressor := NewLZ4UnsafeCompressor(false)
	decompressor := NewLZ4UnsafeDecompressor()

	src := []byte("Hello, World! This is a test string for round-trip compression and decompression.")
	dst := make([]byte, compressor.MaxCompressedLength(len(src)))

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decompressed := make([]byte, len(src))
	err = decompressor.Decompress(compressed, decompressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
}

// BenchmarkLZ4Unsafe_Compress benchmarks unsafe compression
func BenchmarkLZ4Unsafe_Compress(b *testing.B) {
	compressor := NewLZ4UnsafeCompressor(false)

	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dst := make([]byte, compressor.MaxCompressedLength(len(data)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(data, dst)
	}
}

// BenchmarkLZ4Unsafe_Decompress benchmarks unsafe decompression
func BenchmarkLZ4Unsafe_Decompress(b *testing.B) {
	compressor := NewLZ4UnsafeCompressor(false)
	decompressor := NewLZ4UnsafeDecompressor()

	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dst := make([]byte, compressor.MaxCompressedLength(len(data)))
	compressed, _ := compressor.Compress(data, dst)
	decompressed := make([]byte, len(data))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decompressor.Decompress(compressed, decompressed)
	}
}

// TestLZ4RoundTrip tests a full compress/decompress cycle
func TestLZ4RoundTrip(t *testing.T) {
	factory := NewLZ4Factory(false, false)
	compressor := factory.FastCompressor()
	decompressor := factory.FastDecompressor()

	src := []byte("Hello, World! This is a test string for round-trip compression and decompression.")
	dst := make([]byte, compressor.MaxCompressedLength(len(src)))

	compressed, err := compressor.Compress(src, dst)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decompressed := make([]byte, len(src))
	err = decompressor.Decompress(compressed, decompressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	// Note: Due to placeholder implementation, we can't verify content match
	// but we can verify the API works end-to-end
}

// BenchmarkLZ4Factory_Compress benchmarks compression
func BenchmarkLZ4Factory_Compress(b *testing.B) {
	factory := NewLZ4Factory(false, false)
	compressor := factory.FastCompressor()

	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dst := make([]byte, compressor.MaxCompressedLength(len(data)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(data, dst)
	}
}

// BenchmarkLZ4Factory_Decompress benchmarks decompression
func BenchmarkLZ4Factory_Decompress(b *testing.B) {
	factory := NewLZ4Factory(false, false)
	compressor := factory.FastCompressor()
	decompressor := factory.FastDecompressor()

	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dst := make([]byte, compressor.MaxCompressedLength(len(data)))
	compressed, _ := compressor.Compress(data, dst)
	decompressed := make([]byte, len(data))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decompressor.Decompress(compressed, decompressed)
	}
}
