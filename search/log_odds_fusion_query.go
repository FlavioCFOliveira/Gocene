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
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/LogOddsFusionQuery.java

// LogOddsFusionQuery combines sub-query probability scores via log-odds fusion.
// Sub-queries are expected to produce scores in (0, 1) representing probabilities.
//
// Mirrors org.apache.lucene.search.LogOddsFusionQuery (Lucene 10.5.0).
type LogOddsFusionQuery struct {
	clauses       []Query
	alpha         float32
	signalWeights []float32
	logitMin      []float32
	logitMax      []float32
}

// NewLogOddsFusionQuery creates a new LogOddsFusionQuery with per-signal weights and optional logit normalization.
func NewLogOddsFusionQuery(
	clauses []Query,
	alpha float32,
	weights []float32,
	logitMin []float32,
	logitMax []float32,
) (*LogOddsFusionQuery, error) {
	if clauses == nil {
		return nil, fmt.Errorf("collection of Queries must not be null")
	}
	if math.IsNaN(float64(alpha)) || alpha < 0 || alpha > 1 {
		return nil, fmt.Errorf("alpha must be in [0, 1], got %f", alpha)
	}

	var signalWeights []float32
	if weights != nil {
		if len(weights) != len(clauses) {
			return nil, fmt.Errorf("weights length %d must equal clauses size %d", len(weights), len(clauses))
		}
		var sum float32
		for _, w := range weights {
			if math.IsInf(float64(w), 0) || w < 0 {
				return nil, fmt.Errorf("weights must be non-negative and finite, got %f", w)
			}
			sum += w
		}
		if math.Abs(float64(sum-1.0)) > 1e-3 {
			return nil, fmt.Errorf("weights must sum to 1.0, got %f", sum)
		}
		signalWeights = make([]float32, len(weights))
		copy(signalWeights, weights)
	}

	var lMin, lMax []float32
	if logitMin != nil && logitMax != nil {
		if len(logitMin) != len(clauses) {
			return nil, fmt.Errorf("logitMin length %d must equal clauses size %d", len(logitMin), len(clauses))
		}
		if len(logitMax) != len(clauses) {
			return nil, fmt.Errorf("logitMax length %d must equal clauses size %d", len(logitMax), len(clauses))
		}
		lMin = make([]float32, len(logitMin))
		copy(lMin, logitMin)
		lMax = make([]float32, len(logitMax))
		copy(lMax, logitMax)
	}

	return &LogOddsFusionQuery{
		clauses:       clauses,
		alpha:         alpha,
		signalWeights: signalWeights,
		logitMin:      lMin,
		logitMax:      lMax,
	}, nil
}

// NewLogOddsFusionQuerySimple creates a new LogOddsFusionQuery with uniform weighting and softplus gating.
func NewLogOddsFusionQuerySimple(clauses []Query, alpha float32) (*LogOddsFusionQuery, error) {
	return NewLogOddsFusionQuery(clauses, alpha, nil, nil, nil)
}

// NewLogOddsFusionQueryDefault creates a new LogOddsFusionQuery with default alpha=0.5.
func NewLogOddsFusionQueryDefault(clauses []Query) (*LogOddsFusionQuery, error) {
	return NewLogOddsFusionQuery(clauses, 0.5, nil, nil, nil)
}

func (q *LogOddsFusionQuery) GetClauses() []Query {
	return q.clauses
}

func (q *LogOddsFusionQuery) GetAlpha() float32 {
	return q.alpha
}

func (q *LogOddsFusionQuery) GetWeights() []float32 {
	if q.signalWeights == nil {
		return nil
	}
	w := make([]float32, len(q.signalWeights))
	copy(w, q.signalWeights)
	return w
}

type logOddsFusionWeight struct {
	BaseWeight
	query   *LogOddsFusionQuery
	weights []Weight
	mode    ScoreMode
}

func (q *LogOddsFusionQuery) CreateWeight(searcher *IndexSearcher, mode ScoreMode, boost float32) (Weight, error) {
	weights := make([]Weight, len(q.clauses))
	for i, clause := range q.clauses {
		w, err := searcher.CreateWeight(clause, mode, boost)
		if err != nil {
			return nil, err
		}
		weights[i] = w
	}
	return &logOddsFusionWeight{
		query:   q,
		weights: weights,
		mode:    mode,
	}, nil
}

func (w *logOddsFusionWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	var mis []Matches
	for _, weight := range w.weights {
		mi, err := weight.Matches(ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			mis = append(mis, mi)
		}
	}
	return MatchesUtils.FromSubMatches(mis), nil
}

func (w *logOddsFusionWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	var suppliers []ScorerSupplier
	var activeWeights []float32
	var activeMin []float32
	var activeMax []float32

	for i, weight := range w.weights {
		ss, err := weight.ScorerSupplier(ctx)
		if err != nil {
			return nil, err
		}
		if ss != nil {
			suppliers = append(suppliers, ss)
			if w.query.signalWeights != nil {
				activeWeights = append(activeWeights, w.query.signalWeights[i])
			}
			if w.query.logitMin != nil {
				activeMin = append(activeMin, w.query.logitMin[i])
				activeMax = append(activeMax, w.query.logitMax[i])
			}
		}
	}

	if len(suppliers) == 0 {
		return nil, nil
	}
	if len(suppliers) == 1 {
		return suppliers[0], nil
	}

	return &logOddsFusionSupplier{
		suppliers:     suppliers,
		totalClauses:  len(w.query.clauses),
		alpha:         w.query.alpha,
		activeWeights: activeWeights,
		activeMin:     activeMin,
		activeMax:     activeMax,
		scoreMode:     w.mode,
	}, nil
}

type logOddsFusionSupplier struct {
	suppliers     []ScorerSupplier
	totalClauses  int
	alpha         float32
	activeWeights []float32
	activeMin     []float32
	activeMax     []float32
	scoreMode     ScoreMode
}

func (s *logOddsFusionSupplier) Get(leadCost int64) (Scorer, error) {
	scorers := make([]Scorer, len(s.suppliers))
	for i, ss := range s.suppliers {
		sc, err := ss.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers[i] = sc
	}
	return NewLogOddsFusionScorer(
		scorers,
		s.totalClauses,
		s.alpha,
		s.activeWeights,
		s.activeMin,
		s.activeMax,
		s.scoreMode,
		leadCost,
	), nil
}

func (s *logOddsFusionSupplier) Cost() int64 {
	var total int64
	for _, ss := range s.suppliers {
		total += ss.Cost()
	}
	return total
}

func (s *logOddsFusionSupplier) SetTopLevelScoringClause() error {
	for _, ss := range s.suppliers {
		ss.SetTopLevelScoringClause()
	}
	return nil
}

func (w *logOddsFusionWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if len(w.weights) > 1024 { // Boolean rewrite threshold
		return false
	}
	for _, weight := range w.weights {
		if !weight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (w *logOddsFusionWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	var match bool
	var subsOnMatch []Explanation
	var subsOnNoMatch []Explanation
	var logitSum float64
	totalClauses := len(w.weights)

	for i, weight := range w.weights {
		e, err := weight.Explain(ctx, doc)
		if err != nil {
			return nil, err
		}
		if e.IsMatch() {
			match = true
			subsOnMatch = append(subsOnMatch, e)
			subScore := e.GetValue()
			rawLogit := logit(subScore)
			var gated float32
			if w.query.logitMin != nil {
				rangeVal := w.query.logitMax[i] - w.query.logitMin[i]
				if rangeVal > 0 {
					gated = (rawLogit - w.query.logitMin[i]) / rangeVal
					if gated < 0 {
						gated = 0
					} else if gated > 1 {
						gated = 1
					}
				} else {
					gated = 0.5
				}
			} else {
				gated = softplus(rawLogit)
			}
			if w.query.signalWeights != nil {
				logitSum += float64(w.query.signalWeights[i] * gated)
			} else {
				logitSum += float64(gated)
			}
		} else if !match {
			subsOnNoMatch = append(subsOnNoMatch, e)
		}
	}

	if match {
		scalingFactor := math.Pow(float64(totalClauses), float64(w.query.alpha))
		var scaledLogit float32
		var description string
		if w.query.signalWeights != nil {
			scaledLogit = float32(logitSum * scalingFactor)
			description = "weighted log-odds fusion, computed as sigmoid(weightedLogit * n^alpha) from:"
		} else {
			scaledLogit = float32((logitSum / float64(totalClauses)) * scalingFactor)
			description = "log-odds fusion, computed as sigmoid(meanLogit * n^alpha) from:"
		}
		score := logOddsFusionScorerSigmoid(scaledLogit)
		return MatchExplanationWithDetails(score, description, subsOnMatch...), nil
	}
	return NoMatchExplanationWithDetails("No matching clause", subsOnNoMatch...), nil
}

func (q *LogOddsFusionQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQuery("empty LogOddsFusionQuery"), nil
	}
	if len(q.clauses) == 1 {
		return q.clauses[0].Rewrite(searcher)
	}

	actuallyRewritten := false
	var rewrittenClauses []Query
	var newWeights []float32
	var newLogitMin []float32
	var newLogitMax []float32

	for i, sub := range q.clauses {
		rewrittenSub, err := sub.Rewrite(searcher)
		if err != nil {
			return nil, err
		}
		if rewrittenSub != sub {
			actuallyRewritten = true
		}
		if _, ok := rewrittenSub.(*MatchNoDocsQuery); !ok {
			rewrittenClauses = append(rewrittenClauses, rewrittenSub)
			if q.signalWeights != nil {
				newWeights = append(newWeights, q.signalWeights[i])
			}
			if q.logitMin != nil {
				newLogitMin = append(newLogitMin, q.logitMin[i])
				newLogitMax = append(newLogitMax, q.logitMax[i])
			}
		} else {
			actuallyRewritten = true
		}
	}

	if !actuallyRewritten {
		return q, nil
	}
	if len(rewrittenClauses) == 0 {
		return NewMatchNoDocsQuery("empty LogOddsFusionQuery"), nil
	}
	if len(rewrittenClauses) == 1 {
		return rewrittenClauses[0], nil
	}

	if newWeights != nil {
		var sum float32
		for _, w := range newWeights {
			sum += w
		}
		if sum > 0 {
			for i := range newWeights {
				newWeights[i] /= sum
			}
		}
	}

	return NewLogOddsFusionQuery(rewrittenClauses, q.alpha, newWeights, newLogitMin, newLogitMax)
}

func (q *LogOddsFusionQuery) Visit(visitor QueryVisitor) {
	v := visitor.GetSubVisitor(SHOULD, q)
	for _, clause := range q.clauses {
		clause.Visit(v)
	}
}

func (q *LogOddsFusionQuery) ToString(field string) string {
	var parts []string
	for _, sub := range q.clauses {
		s := queryToString(sub, field)
		if _, ok := sub.(*BooleanQuery); ok {
			s = "(" + s + ")"
		}
		parts = append(parts, s)
	}
	base := fmt.Sprintf("LogOdds(%s)^%f", strings.Join(parts, " & "), q.alpha)
	if q.signalWeights != nil {
		return fmt.Sprintf("%s w=%v", base, q.signalWeights)
	}
	return base
}

func (q *LogOddsFusionQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*LogOddsFusionQuery)
	if !ok {
		return false
	}
	if q.alpha != o.alpha {
		return false
	}
	if len(q.clauses) != len(o.clauses) {
		return false
	}
	for i := range q.clauses {
		if q.clauses[i] != o.clauses[i] {
			return false
		}
	}
	if len(q.signalWeights) != len(o.signalWeights) {
		return false
	}
	for i := range q.signalWeights {
		if q.signalWeights[i] != o.signalWeights[i] {
			return false
		}
	}
	if len(q.logitMin) != len(o.logitMin) {
		return false
	}
	for i := range q.logitMin {
		if q.logitMin[i] != o.logitMin[i] {
			return false
		}
	}
	if len(q.logitMax) != len(o.logitMax) {
		return false
	}
	for i := range q.logitMax {
		if q.logitMax[i] != o.logitMax[i] {
			return false
		}
	}
	return true
}

func (q *LogOddsFusionQuery) HashCode() int {
	h := 17 // classHash approximation
	h = 31*h + int(math.Float32bits(q.alpha))

	// clauses hash: Objects.hashCode(clauses) on a java.util.List folds
	// 31*h + element.hashCode() from a seed of 1.
	cHash := 1
	for _, c := range q.clauses {
		e := 0
		if c != nil {
			e = c.HashCode()
		}
		cHash = 31*cHash + e
	}
	h = 31*h + cHash

	// weights hash
	wHash := 0
	for _, w := range q.signalWeights {
		wHash = 31*wHash + int(math.Float32bits(w))
	}
	h = 31*h + wHash

	// min hash
	minHash := 0
	for _, m := range q.logitMin {
		minHash = 31*minHash + int(math.Float32bits(m))
	}
	h = 31*h + minHash

	// max hash
	maxHash := 0
	for _, m := range q.logitMax {
		maxHash = 31*maxHash + int(math.Float32bits(m))
	}
	h = 31*h + maxHash

	return h
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (l *logOddsFusionSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(l)
}
