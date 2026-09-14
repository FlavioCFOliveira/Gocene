// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FloatVectorValues provides access to per-document floating point vector
// values indexed as KnnFloatVectorField. It is the Go port of
// org.apache.lucene.index.FloatVectorValues from Apache Lucene 10.5.0; see
// [KnnVectorValues] for why the declaration lives in spi. The static members
// checkField and fromFloats need index types and are rendered in the index
// package as CheckFloatVectorField and FromFloats.
type FloatVectorValues interface {
	KnnVectorValues

	// VectorValue returns the vector value for the given vector ordinal,
	// which must be in [0, Size() - 1]. The returned slice may be shared
	// across calls.
	VectorValue(ord int) ([]float32, error)

	// CopyFloatVectorValues is the covariant FloatVectorValues.copy()
	// override: it returns the same copy as Copy, typed as
	// FloatVectorValues.
	CopyFloatVectorValues() (FloatVectorValues, error)

	// Scorer returns a VectorScorer for the given query vector and these
	// values. When the underlying format quantizes the vectors, the scorer
	// scores against the quantized vectors. The result may be nil. Java's
	// default body throws UnsupportedOperationException.
	Scorer(target []float32) (util.VectorScorer, error)

	// Rescorer returns a VectorScorer for rescoring an existing set of hits
	// with the highest fidelity scoring available. Java's default body
	// returns Scorer(target).
	Rescorer(target []float32) (util.VectorScorer, error)
}
