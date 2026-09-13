// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/LogOddsFusionScorer.java

// LogOddsFusionScorer combines sub-scorer outputs (assumed to be probabilities
// in (0, 1)) via log-odds fusion with multiplicative confidence scaling.
//
// Mirrors org.apache.lucene.search.LogOddsFusionScorer (Lucene 10.5.0).
type LogOddsFusionScorer struct {
	*DisjunctionScorer
	subScorers     []Scorer
	totalClauses   int
	scalingFactor  float32
	signalWeights  []float32
	logitMin       []float32
	logitMax       []float32
	scorerIndexMap map[Scorer]int
}

const (
	clampMin = 1e-7
	clampMax = 1.0 - 1e-7
)

// NewLogOddsFusionScorer constructs a LogOddsFusionScorer.
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
	ds := newDisjunctionScorer(subScorers, scoreMode, leadCost)

	scalingFactor := float32(math.Pow(float64(totalClauses), float64(alpha)))

	var indexMap map[Scorer]int
	if signalWeights != nil {
		indexMap = make(map[Scorer]int, len(subScorers))
		for i, s := range subScorers {
			indexMap[s] = i
		}
	}

	return &LogOddsFusionScorer{
		DisjunctionScorer: ds,
		subScorers:        subScorers,
		totalClauses:      totalClauses,
		scalingFactor:     scalingFactor,
		signalWeights:     signalWeights,
		logitMin:          logitMin,
		logitMax:          logitMax,
		scorerIndexMap:    indexMap,
	}
}

func clampProbability(p float32) float32 {
	if p < clampMin {
		return clampMin
	}
	if p > clampMax {
		return clampMax
	}
	return p
}

func logit(p float32) float32 {
	clamped := clampProbability(p)
	return float32(math.Log(float64(clamped / (1.0 - clamped))))
}

func logOddsFusionScorerSigmoid(x float32) float32 {
	if x >= 0 {
		return float32(1.0 / (1.0 + math.Exp(float64(-x))))
	}
	expX := math.Exp(float64(x))
	return float32(expX / (1.0 + expX))
}

func softplus(x float32) float32 {
	if x > 20.0 {
		return x
	}
	return float32(math.Log1p(math.Exp(float64(x))))
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

// Score overrides DisjunctionScorer.Score by calling the specialized scoreTopList.
func (s *LogOddsFusionScorer) Score() (float32, error) {
	topList, err := s.getSubMatches()
	if err != nil {
		return 0, err
	}
	return s.scoreTopList(topList)
}

// scoreTopList implements the Log-Odds Fusion formula.
func (s *LogOddsFusionScorer) scoreTopList(topList *DisiWrapper) (float32, error) {
	var logitSum float64
	for w := topList; w != nil; w = w.next {
		subScore, err := w.scorable.Score()
		if err != nil {
			return 0, err
		}
		idx := -1
		if s.scorerIndexMap != nil {
			idx = s.scorerIndexMap[w.scorer]
		}

		signalIdx := 0
		if idx >= 0 {
			signalIdx = idx
		}
		gated := s.gateLogit(logit(subScore), signalIdx)

		if s.scorerIndexMap != nil && idx >= 0 {
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
	return logOddsFusionScorerSigmoid(scaledLogit), nil
}

// GetMaxScore returns an upper bound on the score for any document ≤ upTo.
func (s *LogOddsFusionScorer) GetMaxScore(upTo int) (float32, error) {
	var maxLogitSum float64
	for i, scorer := range s.subScorers {
		if scorer.DocID() <= upTo {
			maxSubScore, err := scorer.GetMaxScore(upTo)
			if err != nil {
				return 0, err
			}
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
	return logOddsFusionScorerSigmoid(scaledLogit), nil
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which LogOddsFusionScorer inherits unchanged through
// DisjunctionScorer.
func (s *LogOddsFusionScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ Scorer = (*LogOddsFusionScorer)(nil)
