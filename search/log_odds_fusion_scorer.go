// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
)

const (
	clampMin = 1e-7
	clampMax = 1.0 - 1e-7
)

// clampProbability restricts a probability to the range [clampMin, clampMax].
func clampProbability(p float32) float32 {
	if p < clampMin {
		return clampMin
	}
	if p > clampMax {
		return clampMax
	}
	return p
}

// logit computes the log-odds of a probability p: ln(p / (1 - p)).
func logit(p float32) float32 {
	clamped := clampProbability(p)
	return float32(math.Log(float64(clamped / (1.0 - clamped))))
}

// sigmoid computes the sigmoid of x: 1 / (1 + exp(-x)).
func sigmoid(x float32) float32 {
	if x >= 0 {
		return float32(1.0 / (1.0 + math.Exp(float64(-x))))
	}
	expX := math.Exp(float64(x))
	return float32(expX / (1.0 + expX))
}

// softplus computes log(1 + exp(x)).
// For x > 20, softplus(x) ≈ x.
func softplus(x float32) float32 {
	if x > 20.0 {
		return x
	}
	return float32(math.Log1p(math.Exp(float64(x))))
}

// LogOddsFusionScorer combines sub-scorer outputs via log-odds fusion.
// It embeds DisjunctionScorer to leverage the union iteration logic.
type LogOddsFusionScorer struct {
	*DisjunctionScorer
	subScorers    []Scorer
	totalClauses  int
	scalingFactor float32
	signalWeights []float32
	logitMin      []float32
	logitMax      []float32
	scorerIndexMap map[*Scorer]int
}

// NewLogOddsFusionScorer creates a new LogOddsFusionScorer.
func NewLogOddsFusionScorer(
	subScorers []Scorer,
	totalClauses int,
	alpha float32,
	signalWeights []float32,
	logitMin []float32,
	logitMax []float32,
	scoreMode ScoreMode,
	leadCost int64,
) *LogOddsFusionScorer {
	base := newDisjunctionScorer(subScorers, scoreMode, leadCost)

	scalingFactor := float32(math.Pow(float64(totalClauses), float64(alpha)))

	var indexMap map[*Scorer]int
	if signalWeights != nil {
		indexMap = make(map[*Scorer]int, len(subScorers))
		for i, s := range subScorers {
			indexMap[s] = i
		}
	}

	return &LogOddsFusionScorer{
		DisjunctionScorer: base,
		subScorers:        subScorers,
		totalClauses:      totalClauses,
		scalingFactor:     scalingFactor,
		signalWeights:     signalWeights,
		logitMin:          logitMin,
		logitMax:          logitMax,
		scorerIndexMap:     indexMap,
	}
}

func (s *LogOddsFusionScorer) gateLogit(rawLogit float32, signalIndex int) float32 {
	if s.logitMin != nil {
		rangeVal := s.logitMax[signalIndex] - s.logitMin[signalIndex]
		if rangeVal > 0 {
			val := (rawLogit - s.logitMin[signalIndex]) / rangeVal
			if val < 0 {
				return 0
			}
			if val > 1 {
				return 1
			}
			return val
		}
		return 0.5
	}
	return softplus(rawLogit)
}

// Score returns the fused score for the current document.
func (s *LogOddsFusionScorer) Score() float32 {
	topList, err := s.DisjunctionScorer.getSubMatches()
	if err != nil {
		return 0
	}
	return s.scoreTopList(topList)
}

func (s *LogOddsFusionScorer) scoreTopList(topList *DisiWrapper) float32 {
	var logitSum float64
	for w := topList; w != nil; w = w.next {
		subScore := w.scorable.Score()

		idx := 0
		if s.scorerIndexMap != nil {
			if foundIdx, ok := s.scorerIndexMap[w.scorer]; ok {
				idx = foundIdx
			}
		}

		gated := s.gateLogit(logit(subScore), idx)

		if s.signalWeights != nil {
			logitSum += float64(s.signalWeights[idx] * gated)
		} else {
			logitSum += float64(gated)
		}
	}

	var scaledLogit float32
	if s.signalWeights != nil {
		scaledLogit = float32(logitSum * float64(s.scalingFactor))
	} else {
		scaledLogit = float32((logitSum / float64(s.totalClauses)) * float64(s.scalingFactor))
	}

	return sigmoid(scaledLogit)
}

// GetMaxScore returns an upper bound on the score for any document up to upTo.
func (s *LogOddsFusionScorer) GetMaxScore(upTo int) float32 {
	var maxLogitSum float64
	for i, scorer := range s.subScorers {
		if scorer.DocID() <= upTo {
			maxSubScore := scorer.GetMaxScore(upTo)
			gated := s.gateLogit(logit(maxSubScore), i)

			if s.signalWeights != nil {
				maxLogitSum += float64(s.signalWeights[i] * gated)
			} else {
				maxLogitSum += float64(gated)
			}
		}
	}

	var scaledLogit float32
	if s.signalWeights != nil {
		scaledLogit = float32(maxLogitSum * float64(s.scalingFactor))
	} else {
		scaledLogit = float32((maxLogitSum / float64(s.totalClauses)) * float64(s.scalingFactor))
	}

	return sigmoid(scaledLogit)
}
