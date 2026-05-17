// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// LuceneDFIIndependence mirrors org.apache.lucene.search.similarities.
// Independence — the divergence-from-independence measure plugged into
// DFISimilarity.
type LuceneDFIIndependence interface {
	// Score computes the distance from independence between the actual
	// term frequency and the expected one.
	Score(freq, expected float64) float64

	// String returns the measure name (used in DFISimilarity.toString).
	String() string
}

// LuceneIndependenceChiSquared mirrors IndependenceChiSquared.
// Score = (freq - expected)^2 / expected.
type LuceneIndependenceChiSquared struct{}

// NewLuceneIndependenceChiSquared constructs the parameter-free measure.
func NewLuceneIndependenceChiSquared() *LuceneIndependenceChiSquared {
	return &LuceneIndependenceChiSquared{}
}

// Score implements LuceneDFIIndependence.
func (LuceneIndependenceChiSquared) Score(freq, expected float64) float64 {
	if expected == 0 {
		return 0
	}
	d := freq - expected
	return d * d / expected
}

// String returns "ChiSquared".
func (LuceneIndependenceChiSquared) String() string { return "ChiSquared" }

// LuceneIndependenceSaturated mirrors IndependenceSaturated.
// Score = (freq - expected) / expected.
type LuceneIndependenceSaturated struct{}

// NewLuceneIndependenceSaturated constructs the parameter-free measure.
func NewLuceneIndependenceSaturated() *LuceneIndependenceSaturated {
	return &LuceneIndependenceSaturated{}
}

// Score implements LuceneDFIIndependence.
func (LuceneIndependenceSaturated) Score(freq, expected float64) float64 {
	if expected == 0 {
		return 0
	}
	return (freq - expected) / expected
}

// String returns "Saturated".
func (LuceneIndependenceSaturated) String() string { return "Saturated" }

// LuceneIndependenceStandardized mirrors IndependenceStandardized.
// Score = (freq - expected) / sqrt(expected).
type LuceneIndependenceStandardized struct{}

// NewLuceneIndependenceStandardized constructs the parameter-free measure.
func NewLuceneIndependenceStandardized() *LuceneIndependenceStandardized {
	return &LuceneIndependenceStandardized{}
}

// Score implements LuceneDFIIndependence.
func (LuceneIndependenceStandardized) Score(freq, expected float64) float64 {
	if expected <= 0 {
		return 0
	}
	return (freq - expected) / math.Sqrt(expected)
}

// String returns "Standardized".
func (LuceneIndependenceStandardized) String() string { return "Standardized" }

// Compile-time guarantees.
var (
	_ LuceneDFIIndependence = LuceneIndependenceChiSquared{}
	_ LuceneDFIIndependence = LuceneIndependenceSaturated{}
	_ LuceneDFIIndependence = LuceneIndependenceStandardized{}
)
