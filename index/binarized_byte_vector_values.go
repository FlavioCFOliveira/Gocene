//go:build ignore

package index

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// BinarizedByteVectorValues provides access to binarized byte vector values.
type BinarizedByteVectorValues interface {
	ByteVectorValues

	// GetCorrectiveTerms retrieve the corrective terms for the given vector ordinal.
	GetCorrectiveTerms(vectorOrd int) (quantization.QuantizationResult, error)

	// GetQuantizer returns the quantizer used to quantize the vectors.
	GetQuantizer() *quantization.OptimizedScalarQuantizer

	// GetCentroid returns the centroid of the vectors.
	GetCentroid() ([]float32, error)

	// Scorer returns a VectorScorer for the given query vector.
	ScorerBinarized(query []float32) (util.VectorScorer, error)
}

// DiscretizedDimensions returns the discretized dimensions for the given dimension and bucket.
func DiscretizedDimensions(dimension, bucket int) int {
	return quantization.Discretize(dimension, bucket)
}

// GetCentroidDP returns the dot product of the centroid with itself.
func GetCentroidDP(bvv BinarizedByteVectorValues) (float32, error) {
	centroid, err := bvv.GetCentroid()
	if err != nil {
		return 0, err
	}
	// return VectorUtil.dotProduct(centroid, centroid)
	var sum float32
	for _, v := range centroid {
		sum += v * v
	}
	return sum, nil
}
