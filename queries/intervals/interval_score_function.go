// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalScoreFunction.java

package intervals

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalScoreFunction is an abstract scoring function over interval frequency.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalScoreFunction (abstract).
//
// Deviations from Java:
//   - scorer(float weight) returns a simScorer closure rather than a Similarity.SimScorer.
type IntervalScoreFunction interface {
	// Scorer returns a SimScorer for the given weight.
	Scorer(weight float32) search.SimScorer
	// Explain returns an Explanation for the given interval, weight and sloppy frequency.
	Explain(interval string, weight, sloppyFreq float32) search.Explanation
}

// SaturationFunction scores as weight * S / (S + k).
type SaturationFunction struct {
	pivot float32
}

// NewSaturationFunction creates a SaturationFunction.
func NewSaturationFunction(pivot float32) (*SaturationFunction, error) {
	if pivot <= 0 || math.IsInf(float64(pivot), 0) || math.IsNaN(float64(pivot)) {
		return nil, fmt.Errorf("pivot must be > 0, got: %v", pivot)
	}
	return &SaturationFunction{pivot: pivot}, nil
}

// Scorer returns a SimScorer.
func (f *SaturationFunction) Scorer(weight float32) search.SimScorer {
	pivot := f.pivot
	return &saturationSimScorer{weight: weight, pivot: pivot}
}

type saturationSimScorer struct {
	weight float32
	pivot  float32
}

func (s *saturationSimScorer) Score(doc int, freq float32) float32 {
	return s.weight * (1.0 - s.pivot/(s.pivot+freq))
}

// Explain returns an Explanation.
func (f *SaturationFunction) Explain(interval string, weight, sloppyFreq float32) search.Explanation {
	score := f.Scorer(weight).Score(0, sloppyFreq)
	exp := search.MatchExplanation(score, "Saturation function on interval frequency, computed as w * S / (S + k) from:")
	exp.AddDetail(search.MatchExplanation(weight, "w, weight of this function"))
	exp.AddDetail(search.MatchExplanation(f.pivot, "k, pivot feature value that would give a score contribution equal to w/2"))
	exp.AddDetail(search.MatchExplanation(sloppyFreq, "S, the sloppy frequency of the interval query "+interval))
	return exp
}

// SigmoidFunction scores as weight * S^a / (S^a + k^a).
type SigmoidFunction struct {
	pivot    float32
	exp      float32
	pivotPow float64 // k^a precomputed
}

// NewSigmoidFunction creates a SigmoidFunction.
func NewSigmoidFunction(pivot, exp float32) (*SigmoidFunction, error) {
	if pivot <= 0 || math.IsInf(float64(pivot), 0) || math.IsNaN(float64(pivot)) {
		return nil, fmt.Errorf("pivot must be > 0, got: %v", pivot)
	}
	if exp <= 0 || math.IsInf(float64(exp), 0) || math.IsNaN(float64(exp)) {
		return nil, fmt.Errorf("exp must be > 0, got: %v", exp)
	}
	return &SigmoidFunction{
		pivot:    pivot,
		exp:      exp,
		pivotPow: math.Pow(float64(pivot), float64(exp)),
	}, nil
}

// Scorer returns a SimScorer.
func (f *SigmoidFunction) Scorer(weight float32) search.SimScorer {
	return &sigmoidSimScorer{weight: weight, exp: float64(f.exp), pivotPow: f.pivotPow}
}

type sigmoidSimScorer struct {
	weight   float32
	exp      float64
	pivotPow float64
}

func (s *sigmoidSimScorer) Score(doc int, freq float32) float32 {
	return float32(float64(s.weight) * (1.0 - s.pivotPow/(math.Pow(float64(freq), s.exp)+s.pivotPow)))
}

// Explain returns an Explanation.
func (f *SigmoidFunction) Explain(interval string, weight, sloppyFreq float32) search.Explanation {
	score := f.Scorer(weight).Score(0, sloppyFreq)
	exp := search.MatchExplanation(score, "Sigmoid function on interval frequency, computed as w * S^a / (S^a + k^a) from:")
	exp.AddDetail(search.MatchExplanation(weight, "w, weight of this function"))
	exp.AddDetail(search.MatchExplanation(f.pivot, "k, pivot feature value that would give a score contribution equal to w/2"))
	exp.AddDetail(search.MatchExplanation(f.exp, "a, exponent, higher values make the function grow slower before k and faster after k"))
	exp.AddDetail(search.MatchExplanation(sloppyFreq, "S, the sloppy frequency of the interval query "+interval))
	return exp
}
