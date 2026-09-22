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
