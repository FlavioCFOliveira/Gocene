// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

// This file bridges unexported symbols to the external quantization_test
// package. Tests that import index (which depends on quantization) must live
// in quantization_test to avoid an import cycle, mirroring Lucene, whose tests
// sit outside the production dependency graph.

// ScratchSize exposes scratchSize (ScalarQuantizer.SCRATCH_SIZE).
const ScratchSize = scratchSize

// FromVectorsWithSampleSize exposes fromVectorsWithSampleSize
// (ScalarQuantizer.fromVectors with an explicit sample size).
var FromVectorsWithSampleSize = fromVectorsWithSampleSize

// GetUpperAndLowerQuantile exposes getUpperAndLowerQuantile
// (ScalarQuantizer.getUpperAndLowerQuantile).
var GetUpperAndLowerQuantile = getUpperAndLowerQuantile

// NewEuclidean builds the record
// ScalarQuantizedVectorSimilarity.Euclidean(float constMultiplier).
func NewEuclidean(constMultiplier float32) *Euclidean {
	return &Euclidean{constMultiplier: constMultiplier}
}

// NewDotProduct builds the record ScalarQuantizedVectorSimilarity.DotProduct(
// float constMultiplier, ByteVectorComparator comparator).
func NewDotProduct(constMultiplier float32, comparator ByteVectorComparator) *DotProduct {
	return &DotProduct{constMultiplier: constMultiplier, comparator: comparator}
}

// NewMaximumInnerProduct builds the record
// ScalarQuantizedVectorSimilarity.MaximumInnerProduct(float constMultiplier,
// ByteVectorComparator comparator).
func NewMaximumInnerProduct(constMultiplier float32, comparator ByteVectorComparator) *MaximumInnerProduct {
	return &MaximumInnerProduct{constMultiplier: constMultiplier, comparator: comparator}
}

// MinimumMSEGrid exposes minimumMSEGrid (OptimizedScalarQuantizer.MINIMUM_MSE_GRID,
// package-private in Lucene and read by TestOptimizedScalarQuantizer).
var MinimumMSEGrid = minimumMSEGrid
