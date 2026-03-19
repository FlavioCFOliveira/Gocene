// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestCompressionBenchmark_Basic tests basic benchmark creation
func TestCompressionBenchmark_Basic(t *testing.T) {
	benchmark := NewCompressionBenchmark()
	if benchmark == nil {
		t.Fatal("NewCompressionBenchmark returned nil")
	}

	if benchmark.GetResults() == nil {
		t.Error("expected GetResults to return non-nil slice")
	}
}

// TestCompressionBenchmark_RunAll runs all benchmarks
func TestCompressionBenchmark_RunAll(t *testing.T) {
	RunCompressionBenchmark(t)
}

// TestCompressionBenchmark_SingleMode tests benchmarking a single mode
func TestCompressionBenchmark_SingleMode(t *testing.T) {
	benchmark := NewCompressionBenchmark()

	testData := []byte("This is a test string for compression benchmarking. " +
		"It should be compressed and decompressed to measure performance.")

	result, err := benchmark.benchmarkMode(CompressionModeLZ4Fast, testData)
	if err != nil {
		t.Fatalf("benchmarkMode failed: %v", err)
	}

	if result.Mode != CompressionModeLZ4Fast {
		t.Errorf("expected mode LZ4Fast, got %v", result.Mode)
	}

	if result.DataSize != len(testData) {
		t.Errorf("expected data size %d, got %d", len(testData), result.DataSize)
	}

	if result.CompressTime <= 0 {
		t.Error("expected positive compress time")
	}

	if result.DecompressTime <= 0 {
		t.Error("expected positive decompress time")
	}
}

// TestCompressionBenchmark_AllModes tests all compression modes
func TestCompressionBenchmark_AllModes(t *testing.T) {
	benchmark := NewCompressionBenchmark()

	testData := []byte("This is test data for benchmarking all compression modes.")

	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			result, err := benchmark.benchmarkMode(mode, testData)
			if err != nil {
				t.Fatalf("benchmarkMode failed: %v", err)
			}

			if result.Mode != mode {
				t.Errorf("expected mode %v, got %v", mode, result.Mode)
			}

			if result.CompressionRatio <= 0 {
				t.Error("expected positive compression ratio")
			}
		})
	}
}

// TestCompressionBenchmark_GenerateTestData tests test data generation
func TestCompressionBenchmark_GenerateTestData(t *testing.T) {
	benchmark := NewCompressionBenchmark()
	data := benchmark.generateTestData()

	if len(data) == 0 {
		t.Error("expected non-empty test data")
	}

	// Check that we have multiple sizes
	sizes := make(map[int]bool)
	for _, d := range data {
		sizes[len(d)] = true
	}

	// Should have at least a few different sizes
	if len(sizes) < 3 {
		t.Errorf("expected multiple data sizes, got %d", len(sizes))
	}
}

// BenchmarkLZ4Fast runs the LZ4Fast benchmark
func BenchmarkLZ4Fast_Compression(b *testing.B) {
	BenchmarkCompression(b, CompressionModeLZ4Fast)
}

// BenchmarkLZ4High runs the LZ4High benchmark
func BenchmarkLZ4High_Compression(b *testing.B) {
	BenchmarkCompression(b, CompressionModeLZ4High)
}

// BenchmarkDeflate runs the Deflate benchmark
func BenchmarkDeflate_Compression(b *testing.B) {
	BenchmarkCompression(b, CompressionModeDeflate)
}

// BenchmarkCompression_SmallData benchmarks compression with small data
func BenchmarkCompression_SmallData(b *testing.B) {
	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressor := CompressionModeLZ4Fast.compressor()
	decompressor := CompressionModeLZ4Fast.decompressor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, _ := compressor(data)
		decompressor(compressed, len(data))
	}
}

// BenchmarkCompression_LargeData benchmarks compression with large data
func BenchmarkCompression_LargeData(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressor := CompressionModeLZ4Fast.compressor()
	decompressor := CompressionModeLZ4Fast.decompressor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, _ := compressor(data)
		decompressor(compressed, len(data))
	}
}

// BenchmarkLZ4FastVsHigh compares LZ4Fast and LZ4High
func BenchmarkLZ4FastVsHigh(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB
	for i := range data {
		data[i] = byte(i % 16) // Repetitive data
	}

	b.Run("LZ4Fast", func(b *testing.B) {
		compressor := CompressionModeLZ4Fast.compressor()
		decompressor := CompressionModeLZ4Fast.decompressor()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compressed, _ := compressor(data)
			decompressor(compressed, len(data))
		}
	})

	b.Run("LZ4High", func(b *testing.B) {
		compressor := CompressionModeLZ4High.compressor()
		decompressor := CompressionModeLZ4High.decompressor()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compressed, _ := compressor(data)
			decompressor(compressed, len(data))
		}
	})

	b.Run("Deflate", func(b *testing.B) {
		compressor := CompressionModeDeflate.compressor()
		decompressor := CompressionModeDeflate.decompressor()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compressed, _ := compressor(data)
			decompressor(compressed, len(data))
		}
	})
}
