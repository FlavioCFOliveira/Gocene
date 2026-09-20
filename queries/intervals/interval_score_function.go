// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalScoreFunction.java

package intervals

import (
	"fmt"
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalScoreFunction is an abstract scoring function over interval frequency.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalScoreFunction (abstract).
//
// Deviations from Java:
//   - scorer(float weight) returns a named search.SimScorer implementation rather
//     than a Java anonymous Similarity.SimScorer subclass.
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

// Score104 mirrors the anonymous Similarity.SimScorer.score(float, long)
// returned by SaturationFunction.scorer(float).
//
// Java writes f / (f + k) as 1 - k / (f + k) so that the result cannot
// decrease with f in spite of rounding.
func (s *saturationSimScorer) Score104(freq float32, norm int64) float32 {
	return s.weight * (1.0 - s.pivot/(s.pivot+freq))
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (s *saturationSimScorer) AsBulkSimScorer() search.BulkSimScorer {
	return search.NewDefaultBulkSimScorer(s)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (s *saturationSimScorer) Explain104(freq search.Explanation, norm int64) search.Explanation {
	e := search.NewExplanation(true, s.Score104(freq.GetValue(), norm),
		"score(freq="+formatFloatGeneric(freq.GetValue())+"), with freq of:")
	e.AddDetail(freq)
	return e
}

// Explain returns an Explanation.
func (f *SaturationFunction) Explain(interval string, weight, sloppyFreq float32) search.Explanation {
	score := f.Scorer(weight).Score104(sloppyFreq, 1)
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

// Score104 mirrors the anonymous Similarity.SimScorer.score(float, long)
// returned by SigmoidFunction.scorer(float).
//
// Java writes f^a / (f^a + k^a) as 1 - k^a / (f^a + k^a) so that the result
// cannot decrease with f in spite of rounding.
func (s *sigmoidSimScorer) Score104(freq float32, norm int64) float32 {
	return float32(float64(s.weight) * (1.0 - s.pivotPow/(math.Pow(float64(freq), s.exp)+s.pivotPow)))
}

// AsBulkSimScorer mirrors the concrete body of Similarity.SimScorer.asBulkSimScorer()
// in Apache Lucene 10.5.0: new DefaultBulkSimScorer(this).
func (s *sigmoidSimScorer) AsBulkSimScorer() search.BulkSimScorer {
	return search.NewDefaultBulkSimScorer(s)
}

// Explain104 mirrors the concrete body of Similarity.SimScorer.explain(Explanation, long)
// in Apache Lucene 10.5.0: Explanation.match(score(freq.getValue().floatValue(), norm),
// "score(freq=" + freq.getValue() + "), with freq of:", freq).
func (s *sigmoidSimScorer) Explain104(freq search.Explanation, norm int64) search.Explanation {
	e := search.NewExplanation(true, s.Score104(freq.GetValue(), norm),
		"score(freq="+formatFloatGeneric(freq.GetValue())+"), with freq of:")
	e.AddDetail(freq)
	return e
}

// Explain returns an Explanation.
func (f *SigmoidFunction) Explain(interval string, weight, sloppyFreq float32) search.Explanation {
	score := f.Scorer(weight).Score104(sloppyFreq, 1)
	exp := search.MatchExplanation(score, "Sigmoid function on interval frequency, computed as w * S^a / (S^a + k^a) from:")
	exp.AddDetail(search.MatchExplanation(weight, "w, weight of this function"))
	exp.AddDetail(search.MatchExplanation(f.pivot, "k, pivot feature value that would give a score contribution equal to w/2"))
	exp.AddDetail(search.MatchExplanation(f.exp, "a, exponent, higher values make the function grow slower before k and faster after k"))
	exp.AddDetail(search.MatchExplanation(sloppyFreq, "S, the sloppy frequency of the interval query "+interval))
	return exp
}

// formatFloatGeneric renders a float32 the way Java's Float.toString does for
// Explanation strings. Java's Float.toString uses the shortest decimal
// representation that round-trips; Go's 'g' verb is the closest equivalent
// without writing a full shortest-decimal implementation.
func formatFloatGeneric(f float32) string {
	return strconv.FormatFloat(float64(f), 'g', -1, 32)
}

// Compile-time guarantees.
var (
	_ search.SimScorer = (*saturationSimScorer)(nil)
	_ search.SimScorer = (*sigmoidSimScorer)(nil)
)
