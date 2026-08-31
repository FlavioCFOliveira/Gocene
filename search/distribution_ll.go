// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// DistributionLL implements the Log-logistic distribution model.
//
// Unlike for DFR, the natural logarithm is used, as it is faster to compute and the original
// paper does not express any preference to a specific base.
type DistributionLL struct{}

// NewDistributionLL creates a new Log-logistic distribution.
func NewDistributionLL() *DistributionLL {
	return &DistributionLL{}
}

// Score computes the Log-logistic information score.
// Score = -log(lambda / (tf + lambda))
func (d *DistributionLL) Score(tf float64, lambda float64) float64 {
	if lambda <= 0 {
		return 0
	}
	return -math.Log(lambda / (tf + lambda))
}

// Name returns the name of this distribution.
func (d *DistributionLL) Name() string {
	return "LL"
}
