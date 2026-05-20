// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// BinarizedByteVectorValues extends ByteVectorValues with the extra
// corrective-term and quantizer methods required by the Lucene 10.2
// binary quantization format.
//
// Port of org.apache.lucene.backward_codecs.lucene102.BinarizedByteVectorValues
// (Lucene 10.4.0, package-private).
//
// Deviation from Java: Java uses an abstract class; Go uses an interface
// that embeds index.ByteVectorValues plus the additional methods.
type BinarizedByteVectorValues interface {
	index.ByteVectorValues

	// GetCorrectiveTerms returns the corrective terms for the given vector
	// ordinal.  For dot-product distances the terms are, in order:
	//   lower optimised interval, upper optimised interval,
	//   dot-product of the non-centred vector with the centroid,
	//   sum of quantised components.
	// For Euclidean:
	//   lower optimised interval, upper optimised interval,
	//   L2-norm of the centred vector,
	//   sum of quantised components.
	GetCorrectiveTerms(vectorOrd int) (quantization.QuantizationResult, error)

	// GetQuantizer returns the quantiser used to quantise the vectors.
	GetQuantizer() *quantization.OptimizedScalarQuantizer

	// GetCentroid returns the centroid used during quantisation.
	GetCentroid() ([]float32, error)

	// Scorer returns a VectorScorer for the given float query vector, or
	// nil if scoring is not supported for this field.
	Scorer(query []float32) (search.VectorScorer, error)

	// Copy returns an independent copy of this iterator.
	Copy() (BinarizedByteVectorValues, error)
}

// DiscretizedDimensions returns the number of dimensions after binarisation.
// Mirrors BinarizedByteVectorValues.discretizedDimensions() which calls
// OptimizedScalarQuantizer.discretize(dimension, 64).
func DiscretizedDimensions(bvv BinarizedByteVectorValues) int {
	return quantization.Discretize(bvv.Dimension(), 64)
}

// CentroidDP computes the dot product of the centroid with itself.
// Mirrors BinarizedByteVectorValues.getCentroidDP() — used during merge.
func CentroidDP(bvv BinarizedByteVectorValues) (float32, error) {
	centroid, err := bvv.GetCentroid()
	if err != nil {
		return 0, err
	}
	return dotProduct(centroid, centroid), nil
}

// dotProduct computes the float32 dot product of two vectors.
func dotProduct(a, b []float32) float32 {
	var sum float32
	for i, v := range a {
		sum += v * b[i]
	}
	return sum
}
