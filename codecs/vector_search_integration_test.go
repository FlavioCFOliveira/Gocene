// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: vector_search_integration_test.go
// Purpose: Integration tests for vector search functionality

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestVectorSearchIntegration tests the complete vector search pipeline
func TestVectorSearchIntegration(t *testing.T) {
	// Test that all vector formats can be instantiated
	t.Run("HNSWFormat", func(t *testing.T) {
		format, err := codecs.NewLucene99HnswVectorsFormat()
		if err != nil {
			t.Fatalf("Failed to create HNSW format: %v", err)
		}
		if format == nil {
			t.Fatal("HNSW format should not be nil")
		}
		if format.Name() != "Lucene99HnswVectorsFormat" {
			t.Errorf("Expected name 'Lucene99HnswVectorsFormat', got '%s'", format.Name())
		}
	})

	t.Run("ScalarQuantizedFormat", func(t *testing.T) {
		format := codecs.NewLucene104ScalarQuantizedVectorsFormat()
		if format == nil {
			t.Fatal("Scalar quantized format should not be nil")
		}
		if format.Name() != "Lucene104ScalarQuantizedVectorsFormat" {
			t.Errorf("Expected name 'Lucene104ScalarQuantizedVectorsFormat', got '%s'", format.Name())
		}
	})

	t.Run("ScalarQuantizedWithEncoding", func(t *testing.T) {
		encodings := []codecs.ScalarEncoding{
			codecs.ScalarEncodingUnsignedByte,
			codecs.ScalarEncodingSevenBit,
			codecs.ScalarEncodingPackedNibble,
			codecs.ScalarEncodingSingleBitQueryNibble,
			codecs.ScalarEncodingDibitQueryNibble,
		}

		for _, enc := range encodings {
			format := codecs.NewLucene104ScalarQuantizedVectorsFormatWithEncoding(enc)
			if format == nil {
				t.Errorf("Format with encoding %s should not be nil", enc.String())
			}
			if format.Encoding() != enc {
				t.Errorf("Expected encoding %s, got %s", enc.String(), format.Encoding().String())
			}
		}
	})
}

// TestFlatVectorScorerIntegration tests the flat vector scorer
func TestFlatVectorScorerIntegration(t *testing.T) {
	scorer := codecs.NewDefaultFlatVectorScorer()
	if scorer == nil {
		t.Fatal("DefaultFlatVectorScorer should not be nil")
	}

	expectedName := "DefaultFlatVectorScorer()"
	if scorer.String() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, scorer.String())
	}
}

// TestVectorSimilarityFunctions tests all similarity function calculations
func TestVectorSimilarityFunctions(t *testing.T) {
	v1 := []float32{1.0, 2.0, 3.0}
	v2 := []float32{2.0, 3.0, 4.0}

	tests := []struct {
		name     string
		simFunc  codecs.VectorSimilarityFunction
		v1       []float32
		v2       []float32
		minScore float32
		maxScore float32
	}{
		{
			name:     "Euclidean",
			simFunc:  codecs.VectorSimilarityFunctionEuclidean,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
		{
			name:     "DotProduct",
			simFunc:  codecs.VectorSimilarityFunctionDotProduct,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
		{
			name:     "Cosine",
			simFunc:  codecs.VectorSimilarityFunctionCosine,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
		{
			name:     "MaximumInnerProduct",
			simFunc:  codecs.VectorSimilarityFunctionMaximumInnerProduct,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 100.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := codecs.ComputeSimilarity(tc.simFunc, tc.v1, tc.v2)
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Score %f out of range [%f, %f]", score, tc.minScore, tc.maxScore)
			}
		})
	}
}

// TestByteVectorSimilarityFunctions tests byte vector similarity calculations
func TestByteVectorSimilarityFunctions(t *testing.T) {
	v1 := []byte{10, 20, 30}
	v2 := []byte{20, 30, 40}

	tests := []struct {
		name     string
		simFunc  codecs.VectorSimilarityFunction
		v1       []byte
		v2       []byte
		minScore float32
		maxScore float32
	}{
		{
			name:     "EuclideanByte",
			simFunc:  codecs.VectorSimilarityFunctionEuclidean,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
		{
			name:     "DotProductByte",
			simFunc:  codecs.VectorSimilarityFunctionDotProduct,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
		{
			name:     "CosineByte",
			simFunc:  codecs.VectorSimilarityFunctionCosine,
			v1:       v1,
			v2:       v2,
			minScore: 0.0,
			maxScore: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := codecs.ComputeSimilarityByte(tc.simFunc, tc.v1, tc.v2)
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Score %f out of range [%f, %f]", score, tc.minScore, tc.maxScore)
			}
		})
	}
}

// TestQuantizationRoundTrip tests quantization and dequantization
func TestQuantizationRoundTrip(t *testing.T) {
	original := []float32{0.5, -0.5, 0.0, 1.0, -1.0}

	encodings := []codecs.ScalarEncoding{
		codecs.ScalarEncodingUnsignedByte,
		codecs.ScalarEncodingPackedNibble,
	}

	for _, enc := range encodings {
		t.Run(enc.String(), func(t *testing.T) {
			quantized := codecs.QuantizeVector(original, enc)
			if len(quantized) == 0 {
				t.Error("Quantized vector should not be empty")
			}

			dequantized := codecs.DequantizeVector(quantized, enc, len(original))
			if len(dequantized) != len(original) {
				t.Errorf("Expected %d dimensions, got %d", len(original), len(dequantized))
			}
		})
	}
}

// TestHNSWFormatConfiguration tests HNSW format configuration
func TestHNSWFormatConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		maxConn   int
		beamWidth int
		wantErr   bool
	}{
		{
			name:      "ValidConfiguration",
			maxConn:   16,
			beamWidth: 100,
			wantErr:   false,
		},
		{
			name:      "MaxConnAtLimit",
			maxConn:   512,
			beamWidth: 100,
			wantErr:   false,
		},
		{
			name:      "BeamWidthAtLimit",
			maxConn:   16,
			beamWidth: 3200,
			wantErr:   false,
		},
		{
			name:      "MaxConnExceedsLimit",
			maxConn:   513,
			beamWidth: 100,
			wantErr:   true,
		},
		{
			name:      "BeamWidthExceedsLimit",
			maxConn:   16,
			beamWidth: 3201,
			wantErr:   true,
		},
		{
			name:      "ZeroMaxConn",
			maxConn:   0,
			beamWidth: 100,
			wantErr:   true,
		},
		{
			name:      "ZeroBeamWidth",
			maxConn:   16,
			beamWidth: 0,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			format, err := codecs.NewLucene99HnswVectorsFormatWithParams(tc.maxConn, tc.beamWidth, 100)
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if format == nil {
					t.Error("Format should not be nil")
				}
			}
		})
	}
}

// TestVectorEncodingBits tests encoding bit calculations
func TestVectorEncodingBits(t *testing.T) {
	tests := []struct {
		encoding     codecs.ScalarEncoding
		expectedBits int
	}{
		{codecs.ScalarEncodingUnsignedByte, 8},
		{codecs.ScalarEncodingSevenBit, 7},
		{codecs.ScalarEncodingPackedNibble, 4},
		{codecs.ScalarEncodingSingleBitQueryNibble, 4},
		{codecs.ScalarEncodingDibitQueryNibble, 4},
	}

	for _, tc := range tests {
		t.Run(tc.encoding.String(), func(t *testing.T) {
			bits := tc.encoding.GetBits()
			if bits != tc.expectedBits {
				t.Errorf("Expected %d bits, got %d", tc.expectedBits, bits)
			}
		})
	}
}

// TestVectorEncodingPackedLength tests document packed length calculations
func TestVectorEncodingPackedLength(t *testing.T) {
	tests := []struct {
		name         string
		encoding     codecs.ScalarEncoding
		discreteDims int
		expected     int
	}{
		{"UnsignedByte_64", codecs.ScalarEncodingUnsignedByte, 64, 64},
		{"PackedNibble_64", codecs.ScalarEncodingPackedNibble, 64, 32},
		{"PackedNibble_65", codecs.ScalarEncodingPackedNibble, 65, 33},
		{"SingleBit_64", codecs.ScalarEncodingSingleBitQueryNibble, 64, 8},
		{"SingleBit_65", codecs.ScalarEncodingSingleBitQueryNibble, 65, 9},
		{"Dibit_64", codecs.ScalarEncodingDibitQueryNibble, 64, 16},
		{"Dibit_65", codecs.ScalarEncodingDibitQueryNibble, 65, 17},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			length := tc.encoding.GetDocPackedLength(tc.discreteDims)
			if length != tc.expected {
				t.Errorf("Expected packed length %d, got %d", tc.expected, length)
			}
		})
	}
}

// TestCentroidCalculation tests centroid calculation
func TestCentroidCalculation(t *testing.T) {
	vectors := [][]float32{
		{1.0, 2.0, 3.0},
		{3.0, 4.0, 5.0},
		{5.0, 6.0, 7.0},
	}

	expected := []float32{3.0, 4.0, 5.0}
	centroid := codecs.CalculateCentroid(vectors)

	if len(centroid) != len(expected) {
		t.Fatalf("Expected centroid length %d, got %d", len(expected), len(centroid))
	}

	for i := range expected {
		if centroid[i] != expected[i] {
			t.Errorf("Expected centroid[%d] = %f, got %f", i, expected[i], centroid[i])
		}
	}
}

// TestEmptyCentroid tests centroid calculation with empty input
func TestEmptyCentroid(t *testing.T) {
	centroid := codecs.CalculateCentroid([][]float32{})
	if centroid != nil {
		t.Error("Centroid of empty vectors should be nil")
	}
}

// TestVectorSimilarityFunctionStrings tests similarity function string representations
func TestVectorSimilarityFunctionStrings(t *testing.T) {
	tests := []struct {
		function codecs.VectorSimilarityFunction
		expected string
	}{
		{codecs.VectorSimilarityFunctionEuclidean, "EUCLIDEAN"},
		{codecs.VectorSimilarityFunctionDotProduct, "DOT_PRODUCT"},
		{codecs.VectorSimilarityFunctionCosine, "COSINE"},
		{codecs.VectorSimilarityFunctionMaximumInnerProduct, "MAXIMUM_INNER_PRODUCT"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if tc.function.String() != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, tc.function.String())
			}
		})
	}
}

// TestScalarQuantizedFormatSupportsFloatFallback tests float vector fallback support
func TestScalarQuantizedFormatSupportsFloatFallback(t *testing.T) {
	format := codecs.NewLucene104ScalarQuantizedVectorsFormat()
	if !format.SupportsFloatVectorFallback() {
		t.Error("ScalarQuantizedVectorsFormat should support float vector fallback")
	}
}

// TestHNSWFormatDoesNotSupportFloatFallback tests HNSW format fallback behavior
func TestHNSWFormatDoesNotSupportFloatFallback(t *testing.T) {
	format, err := codecs.NewLucene99HnswVectorsFormat()
	if err != nil {
		t.Fatalf("Failed to create HNSW format: %v", err)
	}
	if format.SupportsFloatVectorFallback() {
		t.Error("HNSW format should not support float vector fallback")
	}
}

// BenchmarkSimilarityCalculation benchmarks similarity calculations
func BenchmarkSimilarityCalculation(b *testing.B) {
	v1 := make([]float32, 128)
	v2 := make([]float32, 128)
	for i := range v1 {
		v1[i] = float32(i) * 0.01
		v2[i] = float32(127-i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codecs.ComputeSimilarity(codecs.VectorSimilarityFunctionCosine, v1, v2)
	}
}

// BenchmarkQuantizationVectorSearch benchmarks vector quantization
func BenchmarkQuantizationVectorSearch(b *testing.B) {
	vector := make([]float32, 128)
	for i := range vector {
		vector[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codecs.QuantizeVector(vector, codecs.ScalarEncodingUnsignedByte)
	}
}
