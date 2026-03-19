// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"testing"
	"time"
)

// CompressionBenchmark runs compression benchmarks for all compression modes.
// This is the Go port of Lucene's compression benchmarking.
type CompressionBenchmark struct {
	results []BenchmarkResult
}

// BenchmarkResult holds the results of a single benchmark run.
type BenchmarkResult struct {
	Mode             CompressionMode
	DataSize         int
	CompressedSize   int
	CompressTime     time.Duration
	DecompressTime   time.Duration
	CompressionRatio float64
	ThroughputMBps   float64
}

// NewCompressionBenchmark creates a new compression benchmark runner.
func NewCompressionBenchmark() *CompressionBenchmark {
	return &CompressionBenchmark{
		results: make([]BenchmarkResult, 0),
	}
}

// RunAllBenchmarks runs benchmarks for all compression modes with various data sizes.
func (b *CompressionBenchmark) RunAllBenchmarks() ([]BenchmarkResult, error) {
	testData := b.generateTestData()

	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		for _, data := range testData {
			result, err := b.benchmarkMode(mode, data)
			if err != nil {
				return nil, fmt.Errorf("benchmark failed for %s: %w", mode.String(), err)
			}
			b.results = append(b.results, result)
		}
	}

	return b.results, nil
}

// benchmarkMode benchmarks a specific compression mode with given data.
func (b *CompressionBenchmark) benchmarkMode(mode CompressionMode, data []byte) (BenchmarkResult, error) {
	compressor := mode.compressor()
	decompressor := mode.decompressor()

	// Benchmark compression
	compressStart := time.Now()
	compressed, err := compressor(data)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("compression failed: %w", err)
	}
	compressTime := time.Since(compressStart)

	// Benchmark decompression
	decompressStart := time.Now()
	_, err = decompressor(compressed, len(data))
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("decompression failed: %w", err)
	}
	decompressTime := time.Since(decompressStart)

	dataSize := len(data)
	compressedSize := len(compressed)
	compressionRatio := float64(compressedSize) / float64(dataSize)
	throughputMBps := float64(dataSize) / (1024 * 1024) / compressTime.Seconds()

	return BenchmarkResult{
		Mode:             mode,
		DataSize:         dataSize,
		CompressedSize:   compressedSize,
		CompressTime:     compressTime,
		DecompressTime:   decompressTime,
		CompressionRatio: compressionRatio,
		ThroughputMBps:   throughputMBps,
	}, nil
}

// generateTestData generates test data of various sizes and patterns.
func (b *CompressionBenchmark) generateTestData() [][]byte {
	sizes := []int{
		1024,        // 1KB
		16 * 1024,   // 16KB
		64 * 1024,   // 64KB
		256 * 1024,  // 256KB
		1024 * 1024, // 1MB
	}

	var testData [][]byte

	for _, size := range sizes {
		// Random data (worst case for compression)
		randomData := make([]byte, size)
		for i := range randomData {
			randomData[i] = byte(i % 256)
		}
		testData = append(testData, randomData)

		// Repetitive data (best case for compression)
		repetitiveData := make([]byte, size)
		for i := range repetitiveData {
			repetitiveData[i] = byte(i % 16)
		}
		testData = append(testData, repetitiveData)

		// Text-like data (realistic case)
		textData := make([]byte, size)
		for i := range textData {
			// Printable ASCII characters
			textData[i] = byte(32 + (i % 95))
		}
		testData = append(testData, textData)
	}

	return testData
}

// PrintResults prints the benchmark results in a formatted table.
func (b *CompressionBenchmark) PrintResults() {
	fmt.Println("\nCompression Benchmark Results")
	fmt.Println("=============================")
	fmt.Printf("%-12s %-10s %-12s %-12s %-12s %-15s %-12s\n",
		"Mode", "DataSize", "CompSize", "CompTime", "DecompTime", "Ratio", "Throughput")
	fmt.Println("----------------------------------------------------------------------------------------")

	for _, r := range b.results {
		fmt.Printf("%-12s %-10d %-12d %-12s %-12s %-15.2f %-12.2f\n",
			r.Mode.String(),
			r.DataSize,
			r.CompressedSize,
			r.CompressTime,
			r.DecompressTime,
			r.CompressionRatio,
			r.ThroughputMBps,
		)
	}
}

// GetResults returns all benchmark results.
func (b *CompressionBenchmark) GetResults() []BenchmarkResult {
	return b.results
}

// BenchmarkCompression runs a benchmark for a specific compression mode.
// This is a standalone function for use in tests.
func BenchmarkCompression(b *testing.B, mode CompressionMode) {
	data := make([]byte, 64*1024) // 64KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressor := mode.compressor()
	decompressor := mode.decompressor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, err := compressor(data)
		if err != nil {
			b.Fatalf("compression failed: %v", err)
		}

		_, err = decompressor(compressed, len(data))
		if err != nil {
			b.Fatalf("decompression failed: %v", err)
		}
	}
}

// BenchmarkLZ4Fast benchmarks LZ4 fast compression.
func BenchmarkLZ4Fast(b *testing.B) {
	BenchmarkCompression(b, CompressionModeLZ4Fast)
}

// BenchmarkLZ4High benchmarks LZ4 high compression.
func BenchmarkLZ4High(b *testing.B) {
	BenchmarkCompression(b, CompressionModeLZ4High)
}

// BenchmarkDeflate benchmarks Deflate compression.
func BenchmarkDeflate(b *testing.B) {
	BenchmarkCompression(b, CompressionModeDeflate)
}

// RunCompressionBenchmark is a helper for running benchmarks in tests.
func RunCompressionBenchmark(t *testing.T) {
	benchmark := NewCompressionBenchmark()
	results, err := benchmark.RunAllBenchmarks()
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	// Verify results
	if len(results) == 0 {
		t.Error("expected benchmark results, got none")
	}

	// Check that all modes are represented
	modesFound := make(map[CompressionMode]bool)
	for _, r := range results {
		modesFound[r.Mode] = true
	}

	expectedModes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range expectedModes {
		if !modesFound[mode] {
			t.Errorf("expected results for %s mode", mode.String())
		}
	}

	// Print results for visibility
	benchmark.PrintResults()
}
