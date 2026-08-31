// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// NormalizationZ implements Pareto-Zipf Normalization.
type NormalizationZ struct {
	z float64
}

// NewNormalizationZ creates a new NormalizationZ with the default parameter z = 0.3.
func NewNormalizationZ() *NormalizationZ {
	return NewNormalizationZWithParam(0.3)
}

// NewNormalizationZWithParam creates a NormalizationZ with the supplied parameter z.
// z represents A/(A+1) where A measures the specificity of the language.
// It must be in the range (0 .. 0.5).
func NewNormalizationZWithParam(z float64) *NormalizationZ {
	if math.IsNaN(z) || z <= 0 || z >= 0.5 {
		panic(fmt.Sprintf("illegal z value: %f, must be in the range (0 .. 0.5)", z))
	}
	return &NormalizationZ{z: z}
}

// Tfn computes the Pareto-Zipf normalized term frequency.
// Formula: tfn = tf * Math.pow(avgfl / fl, z)
func (n *NormalizationZ) Tfn(stats *LuceneBasicStats, freq float64, docLen float64) float64 {
	avgDocLen := stats.AvgFieldLength()
	if avgDocLen == 0 {
		avgDocLen = 1.0
	}
	if docLen == 0 {
		docLen = avgDocLen
	}
	return freq * math.Pow(avgDocLen/docLen, n.z)
}

// Name returns the name of this normalization.
func (n *NormalizationZ) Name() string {
	return "Z"
}

// String returns a string representation of the normalization.
func (n *NormalizationZ) String() string {
	return fmt.Sprintf("Z(%f)", n.z)
}

// Z returns the parameter z.
func (n *NormalizationZ) Z() float64 {
	return n.z
}
