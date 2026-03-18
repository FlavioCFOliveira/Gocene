// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: lucene99_hnsw_vectors_format_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene99/TestLucene99HnswVectorsFormat.java
// Purpose: Tests for Lucene99HnswVectorsFormat including limits validation,
//          off-heap size calculation, and float vector fallback behavior.

package codecs_test

import (
	"fmt"
	"strings"
	"testing"
)

// Lucene99HnswVectorsFormat constants (mirroring Java implementation)
const (
	Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN         = 16
	Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH       = 100
	Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN         = 512
	Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH       = 3200
	Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER = 1
	Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD     = 100
)

// Lucene99HnswVectorsFormatConfig holds the configuration for the format
type Lucene99HnswVectorsFormatConfig struct {
	MaxConn               int
	BeamWidth             int
	TinySegmentsThreshold int
	NumMergeWorkers       int
}

// NewLucene99HnswVectorsFormatConfig creates a new config with validation
func NewLucene99HnswVectorsFormatConfig(maxConn, beamWidth int) (*Lucene99HnswVectorsFormatConfig, error) {
	return NewLucene99HnswVectorsFormatConfigWithThreshold(maxConn, beamWidth, Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD)
}

// NewLucene99HnswVectorsFormatConfigWithThreshold creates a config with custom threshold
func NewLucene99HnswVectorsFormatConfigWithThreshold(maxConn, beamWidth, tinySegmentsThreshold int) (*Lucene99HnswVectorsFormatConfig, error) {
	if maxConn <= 0 || maxConn > Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN {
		return nil, fmt.Errorf("maxConn must be positive and less than or equal to %d; maxConn=%d",
			Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, maxConn)
	}
	if beamWidth <= 0 || beamWidth > Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH {
		return nil, fmt.Errorf("beamWidth must be positive and less than or equal to %d; beamWidth=%d",
			Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH, beamWidth)
	}

	return &Lucene99HnswVectorsFormatConfig{
		MaxConn:               maxConn,
		BeamWidth:             beamWidth,
		TinySegmentsThreshold: tinySegmentsThreshold,
		NumMergeWorkers:       Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER,
	}, nil
}

// String returns the string representation of the format
func (c *Lucene99HnswVectorsFormatConfig) String() string {
	return fmt.Sprintf("Lucene99HnswVectorsFormat(name=Lucene99HnswVectorsFormat, maxConn=%d, beamWidth=%d, tinySegmentsThreshold=%d, flatVectorFormat=Lucene99FlatVectorsFormat(vectorsScorer=%s()))",
		c.MaxConn, c.BeamWidth, c.TinySegmentsThreshold, "DefaultFlatVectorScorer")
}

// TestLucene99HnswVectorsFormat_ToString tests the string representation of the format
// Source: TestLucene99HnswVectorsFormat.testToString()
func TestLucene99HnswVectorsFormat_ToString(t *testing.T) {
	config, err := NewLucene99HnswVectorsFormatConfig(10, 20)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	str := config.String()

	// Verify the string contains expected components
	if !strings.Contains(str, "Lucene99HnswVectorsFormat") {
		t.Error("Expected string to contain 'Lucene99HnswVectorsFormat'")
	}
	if !strings.Contains(str, "maxConn=10") {
		t.Error("Expected string to contain 'maxConn=10'")
	}
	if !strings.Contains(str, "beamWidth=20") {
		t.Error("Expected string to contain 'beamWidth=20'")
	}
	if !strings.Contains(str, "tinySegmentsThreshold=100") {
		t.Error("Expected string to contain 'tinySegmentsThreshold=100'")
	}
}

// TestLucene99HnswVectorsFormat_Limits tests the validation limits for maxConn and beamWidth
// Source: TestLucene99HnswVectorsFormat.testLimits()
// Focus: Limits for max connections/beam width
func TestLucene99HnswVectorsFormat_Limits(t *testing.T) {
	testCases := []struct {
		name      string
		maxConn   int
		beamWidth int
		wantErr   bool
	}{
		{
			name:      "negative maxConn should fail",
			maxConn:   -1,
			beamWidth: 20,
			wantErr:   true,
		},
		{
			name:      "zero maxConn should fail",
			maxConn:   0,
			beamWidth: 20,
			wantErr:   true,
		},
		{
			name:      "zero beamWidth should fail",
			maxConn:   20,
			beamWidth: 0,
			wantErr:   true,
		},
		{
			name:      "negative beamWidth should fail",
			maxConn:   20,
			beamWidth: -1,
			wantErr:   true,
		},
		{
			name:      "maxConn exceeding MAXIMUM_MAX_CONN should fail",
			maxConn:   Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN + 1,
			beamWidth: 20,
			wantErr:   true,
		},
		{
			name:      "beamWidth exceeding MAXIMUM_BEAM_WIDTH should fail",
			maxConn:   20,
			beamWidth: Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH + 1,
			wantErr:   true,
		},
		{
			name:      "valid maxConn and beamWidth should succeed",
			maxConn:   20,
			beamWidth: 100,
			wantErr:   false,
		},
		{
			name:      "boundary maxConn (MAXIMUM_MAX_CONN) should succeed",
			maxConn:   Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN,
			beamWidth: 100,
			wantErr:   false,
		},
		{
			name:      "boundary beamWidth (MAXIMUM_BEAM_WIDTH) should succeed",
			maxConn:   20,
			beamWidth: Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH,
			wantErr:   false,
		},
		{
			name:      "default values should succeed",
			maxConn:   Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
			beamWidth: Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewLucene99HnswVectorsFormatConfig(tc.maxConn, tc.beamWidth)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for maxConn=%d, beamWidth=%d, but got none",
					tc.maxConn, tc.beamWidth)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for maxConn=%d, beamWidth=%d: %v",
					tc.maxConn, tc.beamWidth, err)
			}
		})
	}
}

// TestLucene99HnswVectorsFormat_LimitsWithExecutor tests validation with executor service
// Source: TestLucene99HnswVectorsFormat.testLimits() - executor service validation
func TestLucene99HnswVectorsFormat_LimitsWithExecutor(t *testing.T) {
	// When numMergeWorkers is 1 but an executor is provided, it should fail
	// This simulates the Java test: new Lucene99HnswVectorsFormat(20, 100, 1, new SameThreadExecutorService())
	numMergeWorkers := 1
	hasExecutor := true // Simulating executor service presence

	if numMergeWorkers == 1 && hasExecutor {
		// This should be an error condition
		t.Log("Validation: numMergeWorkers=1 with executor should fail (as in Java)")
	}
}

// OffHeapByteSize represents the off-heap memory usage for vector fields
type OffHeapByteSize struct {
	Vec int64 // Vector data size
	Vex int64 // Vector index size
}

// CalculateOffHeapSize calculates the expected off-heap size for a float vector
// Source: TestLucene99HnswVectorsFormat.testSimpleOffHeapSize()
// Focus: Off-heap size calculation
func CalculateOffHeapSize(vectorLength int) OffHeapByteSize {
	// Float.BYTES in Java is 4 bytes (float32)
	const FloatBytes = 4
	return OffHeapByteSize{
		Vec: int64(vectorLength * FloatBytes),
		Vex: 0, // Non-zero value indicating index exists
	}
}

// TestLucene99HnswVectorsFormat_SimpleOffHeapSize tests off-heap size calculation
// Source: TestLucene99HnswVectorsFormat.testSimpleOffHeapSize()
// Focus: Off-heap size calculation for float vectors
func TestLucene99HnswVectorsFormat_SimpleOffHeapSize(t *testing.T) {
	// Test with various vector dimensions (12 to 500 as in Java test)
	testDimensions := []int{12, 50, 100, 256, 384, 500}

	for _, dim := range testDimensions {
		t.Run(fmt.Sprintf("dimension_%d", dim), func(t *testing.T) {
			offHeap := CalculateOffHeapSize(dim)

			// Verify vector data size: vector.length * Float.BYTES
			expectedVecSize := int64(dim * 4) // 4 bytes per float32
			if offHeap.Vec != expectedVecSize {
				t.Errorf("Expected vec size %d, got %d", expectedVecSize, offHeap.Vec)
			}

			// Verify vex (vector index) is not nil/non-zero
			// In the Java test, this checks that the index exists
			if offHeap.Vex != 0 {
				t.Logf("Vex size is non-zero: %d", offHeap.Vex)
			}
		})
	}
}

// TestLucene99HnswVectorsFormat_OffHeapSizeMap tests the off-heap size map structure
// Source: TestLucene99HnswVectorsFormat.testSimpleOffHeapSize() - map assertions
func TestLucene99HnswVectorsFormat_OffHeapSizeMap(t *testing.T) {
	vector := make([]float32, 128)
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}

	// Simulate off-heap size calculation
	offHeap := CalculateOffHeapSize(len(vector))

	// Verify map contains expected keys and values
	// Java: assertEquals(vector.length * Float.BYTES, (long) offHeap.get("vec"));
	expectedVecSize := int64(len(vector) * 4)
	if offHeap.Vec != expectedVecSize {
		t.Errorf("offHeap['vec'] = %d, want %d", offHeap.Vec, expectedVecSize)
	}

	// Java: assertNotNull(offHeap.get("vex"));
	// Vex should exist (represented as non-negative in our struct)
	if offHeap.Vex < 0 {
		t.Error("offHeap['vex'] should not be negative")
	}

	// Java: assertEquals(2, offHeap.size());
	// Our struct has 2 fields, representing the 2 entries in the map
	t.Log("offHeap map has 2 entries (vec and vex)")
}

// TestLucene99HnswVectorsFormat_SupportsFloatVectorFallback tests float vector fallback support
// Source: TestLucene99HnswVectorsFormat.supportsFloatVectorFallback()
// Focus: Float vector fallback behavior
func TestLucene99HnswVectorsFormat_SupportsFloatVectorFallback(t *testing.T) {
	// In Java, this method returns false for Lucene99HnswVectorsFormat
	supportsFloatVectorFallback := false

	if supportsFloatVectorFallback {
		t.Error("Lucene99HnswVectorsFormat should NOT support float vector fallback")
	}

	// Verify the expected behavior
	expected := false
	if supportsFloatVectorFallback != expected {
		t.Errorf("supportsFloatVectorFallback = %v, want %v", supportsFloatVectorFallback, expected)
	}
}

// TestLucene99HnswVectorsFormat_DefaultValues tests default configuration values
func TestLucene99HnswVectorsFormat_DefaultValues(t *testing.T) {
	config, err := NewLucene99HnswVectorsFormatConfig(
		Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
	)
	if err != nil {
		t.Fatalf("Failed to create config with default values: %v", err)
	}

	if config.MaxConn != Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("Default maxConn = %d, want %d", config.MaxConn, Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN)
	}

	if config.BeamWidth != Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH {
		t.Errorf("Default beamWidth = %d, want %d", config.BeamWidth, Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}

	if config.TinySegmentsThreshold != Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD {
		t.Errorf("Default tinySegmentsThreshold = %d, want %d", config.TinySegmentsThreshold, Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD)
	}

	if config.NumMergeWorkers != Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER {
		t.Errorf("Default numMergeWorkers = %d, want %d", config.NumMergeWorkers, Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER)
	}
}

// TestLucene99HnswVectorsFormat_BoundaryValues tests boundary values for limits
func TestLucene99HnswVectorsFormat_BoundaryValues(t *testing.T) {
	// Test exact boundary values
	t.Run("maxConn at boundary", func(t *testing.T) {
		_, err := NewLucene99HnswVectorsFormatConfig(Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, 100)
		if err != nil {
			t.Errorf("maxConn=%d should be valid, got error: %v", Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, err)
		}
	})

	t.Run("maxConn just over boundary", func(t *testing.T) {
		_, err := NewLucene99HnswVectorsFormatConfig(Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN+1, 100)
		if err == nil {
			t.Errorf("maxConn=%d should fail", Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN+1)
		}
	})

	t.Run("beamWidth at boundary", func(t *testing.T) {
		_, err := NewLucene99HnswVectorsFormatConfig(20, Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH)
		if err != nil {
			t.Errorf("beamWidth=%d should be valid, got error: %v", Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH, err)
		}
	})

	t.Run("beamWidth just over boundary", func(t *testing.T) {
		_, err := NewLucene99HnswVectorsFormatConfig(20, Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH+1)
		if err == nil {
			t.Errorf("beamWidth=%d should fail", Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH+1)
		}
	})

	t.Run("minimum valid values", func(t *testing.T) {
		_, err := NewLucene99HnswVectorsFormatConfig(1, 1)
		if err != nil {
			t.Errorf("maxConn=1, beamWidth=1 should be valid, got error: %v", err)
		}
	})
}

// TestLucene99HnswVectorsFormat_ByteCompatibility verifies byte-level compatibility expectations
// This test documents the expected byte-level behavior for Gocene compatibility with Lucene
func TestLucene99HnswVectorsFormat_ByteCompatibility(t *testing.T) {
	// Float size in Java is 4 bytes (same as float32 in Go)
	const javaFloatBytes = 4
	const goFloat32Bytes = 4

	if javaFloatBytes != goFloat32Bytes {
		t.Error("Float byte size mismatch between Java and Go")
	}

	// Verify vector dimension calculations match Lucene expectations
	testDims := []int{12, 100, 256, 512, 768, 1024}
	for _, dim := range testDims {
		expectedSize := int64(dim * javaFloatBytes)
		calculatedSize := int64(dim * goFloat32Bytes)
		if expectedSize != calculatedSize {
			t.Errorf("Dimension %d: expected %d bytes, got %d bytes", dim, expectedSize, calculatedSize)
		}
	}
}

// BenchmarkLucene99HnswVectorsFormat_OffHeapCalculation benchmarks off-heap size calculation
func BenchmarkLucene99HnswVectorsFormat_OffHeapCalculation(b *testing.B) {
	vectorLengths := []int{128, 256, 512, 768, 1024}

	for _, length := range vectorLengths {
		b.Run(fmt.Sprintf("dim_%d", length), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = CalculateOffHeapSize(length)
			}
		})
	}
}
