// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
)

// DistributionSPL implements the smoothed power-law (SPL) distribution for the
// information-based framework.
//
// Unlike for DFR, the natural logarithm is used, as it is faster to compute and
// the original paper does not express any preference to a specific base.
//
// WARNING: this model currently returns infinite scores for very small tf values
// and negative scores for very large tf values.
type DistributionSPL struct{}

// NewDistributionSPL creates a new SPL distribution.
func NewDistributionSPL() *DistributionSPL {
	return &DistributionSPL{}
}

// Score computes the SPL information score.
//
// Logic ported from org.apache.lucene.search.similarities.DistributionSPL.score.
func (d *DistributionSPL) Score(tf float64, lambda float64) float64 {
	// Note: Java asserts lambda != 1.

	// tfn/(tfn+1) -> 1 - 1/(tfn+1), guaranteed to be non decreasing when tfn increases
	q := 1.0 - 1.0/(tf+1.0)
	if q == 1.0 {
		q = math.Nextafter(1.0, math.Inf(-1))
	}

	pow := math.Pow(lambda, q)
	if pow == lambda {
		// this can happen because of floating-point rounding
		// but then we return infinity when taking the log, so we enforce
		// that pow is different from lambda
		if lambda < 1 {
			// x^y > x when x < 1 and y < 1
			pow = math.Nextafter(lambda, math.Inf(1))
		} else {
			// x^y < x when x > 1 and y < 1
			pow = math.Nextafter(lambda, math.Inf(-1))
		}
	}

	return -math.Log((pow - lambda) / (1.0 - lambda))
}

// Name returns the name of this distribution.
func (d *DistributionSPL) Name() string {
	return "SPL"
}
