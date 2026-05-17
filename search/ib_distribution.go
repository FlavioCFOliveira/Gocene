// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// LuceneIBDistribution mirrors org.apache.lucene.search.similarities.
// Distribution — the probabilistic model used by IBSimilarity.
type LuceneIBDistribution interface {
	// Score returns -log(Prob(X >= tfn | lambda)).
	Score(stats *LuceneBasicStats, tfn, lambda float64) float64

	// Explain returns an Explanation node tagged with the model name.
	Explain(stats *LuceneBasicStats, tfn, lambda float64) Explanation

	// String returns the distribution code ("LL", "SPL").
	String() string
}

// LuceneDistributionLL mirrors DistributionLL — log-logistic.
type LuceneDistributionLL struct{}

// NewLuceneDistributionLL constructs the parameter-free LL distribution.
func NewLuceneDistributionLL() *LuceneDistributionLL { return &LuceneDistributionLL{} }

// Score implements LuceneIBDistribution: -log(lambda / (tfn + lambda)).
func (LuceneDistributionLL) Score(_ *LuceneBasicStats, tfn, lambda float64) float64 {
	if lambda <= 0 {
		return 0
	}
	return -math.Log(lambda / (tfn + lambda))
}

// Explain returns an Explanation node tagged "DistributionLL".
func (d LuceneDistributionLL) Explain(stats *LuceneBasicStats, tfn, lambda float64) Explanation {
	return NewExplanation(true, float32(d.Score(stats, tfn, lambda)), "DistributionLL")
}

// String returns "LL".
func (LuceneDistributionLL) String() string { return "LL" }

// LuceneDistributionSPL mirrors DistributionSPL — smoothed power-law.
// WARNING (from Lucene): this model returns +infinity for very small tf
// and negative scores for very large tf.
type LuceneDistributionSPL struct{}

// NewLuceneDistributionSPL constructs the parameter-free SPL distribution.
func NewLuceneDistributionSPL() *LuceneDistributionSPL { return &LuceneDistributionSPL{} }

// Score implements LuceneIBDistribution. The implementation mirrors Java's
// boundary handling with math.Nextafter.
func (LuceneDistributionSPL) Score(_ *LuceneBasicStats, tfn, lambda float64) float64 {
	if lambda == 1 {
		// Java asserts lambda != 1. Defensively short-circuit to avoid
		// log(0/0). LambdaDF/TTF already perturb the value to avoid this.
		return 0
	}
	q := 1 - 1/(tfn+1)
	if q == 1 {
		q = math.Nextafter(1.0, math.Inf(-1)) // Math.nextDown(1.0)
	}
	pow := math.Pow(lambda, q)
	if pow == lambda {
		if lambda < 1 {
			pow = math.Nextafter(lambda, math.Inf(+1))
		} else {
			pow = math.Nextafter(lambda, math.Inf(-1))
		}
	}
	return -math.Log((pow - lambda) / (1 - lambda))
}

// Explain returns an Explanation node tagged "DistributionSPL".
func (d LuceneDistributionSPL) Explain(stats *LuceneBasicStats, tfn, lambda float64) Explanation {
	return NewExplanation(true, float32(d.Score(stats, tfn, lambda)), "DistributionSPL")
}

// String returns "SPL".
func (LuceneDistributionSPL) String() string { return "SPL" }

// Compile-time guarantees.
var (
	_ LuceneIBDistribution = LuceneDistributionLL{}
	_ LuceneIBDistribution = LuceneDistributionSPL{}
)
