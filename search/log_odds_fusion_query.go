// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LogOddsFusionQuery combines sub-query probability scores via log-odds fusion.
// Sub-queries are expected to produce scores in (0, 1).
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
		return nil, fmt.Errorf("clauses must not be nil")
	}
	if alpha < 0 || alpha > 1 {
		return nil, fmt.Errorf("alpha must be in [0, 1], got %g", alpha)
	}

	if weights != nil {
		if len(weights) != len(clauses) {
			return nil, fmt.Errorf("weights length %d must equal clauses size %d", len(weights), len(clauses))
		}
		var sum float32
		for _, w := range weights {
			if math.IsNaN(float64(w)) || math.IsInf(float64(w), 0) || w < 0 {
				return nil, fmt.Errorf("weights must be non-negative and finite, got %g", w)
			}
			sum += w
		}
		if math.Abs(float64(sum-1.0)) > 1e-3 {
			return nil, fmt.Errorf("weights must sum to 1.0, got %g", sum)
		}
	}

	if logitMin != nil && logitMax != nil {
		if len(logitMin) != len(clauses) {
			return nil, fmt.Errorf("logitMin length %d must equal clauses size %d", len(logitMin), len(clauses))
		}
		if len(logitMax) != len(clauses) {
			return nil, fmt.Errorf("logitMax length %d must equal clauses size %d", len(logitMax), len(clauses))
		}
	} else {
		// Ensure both are nil if one is nil
		logitMin = nil
		logitMax = nil
	}

	return &LogOddsFusionQuery{
		clauses:       clauses,
		alpha:         alpha,
		signalWeights: weights,
		logitMin:      logitMin,
		logitMax:      logitMax,
	}, nil
}

// NewLogOddsFusionQueryWithWeights creates a new LogOddsFusionQuery with per-signal weights (softplus gating).
func NewLogOddsFusionQueryWithWeights(clauses []Query, alpha float32, weights []float32) (*LogOddsFusionQuery, error) {
	return NewLogOddsFusionQuery(clauses, alpha, weights, nil, nil)
}

// NewLogOddsFusionQueryUniform creates a new LogOddsFusionQuery with uniform weighting and softplus gating.
func NewLogOddsFusionQueryUniform(clauses []Query, alpha float32) (*LogOddsFusionQuery, error) {
	return NewLogOddsFusionQuery(clauses, alpha, nil, nil, nil)
}

// NewLogOddsFusionQueryDefault creates a new LogOddsFusionQuery with default alpha=0.5 and uniform weighting.
func NewLogOddsFusionQueryDefault(clauses []Query) (*LogOddsFusionQuery, error) {
	return NewLogOddsFusionQuery(clauses, 0.5, nil, nil, nil)
}

// GetClauses returns the clauses of the query.
func (q *LogOddsFusionQuery) GetClauses() []Query {
	return q.clauses
}

// CreateWeight creates a weight for the query.
func (q *LogOddsFusionQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewLogOddsFusionWeight(q, searcher, scoreMode, boost), nil
}

// Rewrite rewrites the query.
func (q *LogOddsFusionQuery) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQuery("empty LogOddsFusionQuery"), nil
	}

	if len(q.clauses) == 1 {
		return q.clauses[0].Rewrite(indexSearcher)
	}

	actuallyRewritten := false
	var rewrittenClauses []Query
	var newWeights []float32
	var newLogitMin []float32
	var newLogitMax []float32

	for i, sub := range q.clauses {
		rewrittenSub, err := sub.Rewrite(indexSearcher)
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

// Visit visits the sub-queries.
func (q *LogOddsFusionQuery) Visit(visitor QueryVisitor) {
	v := visitor.GetSubVisitor(BooleanClauseOccurShould, q)
	for _, sub := range q.clauses {
		sub.Visit(v)
	}
}

// ToString returns a string representation of the query.
func (q *LogOddsFusionQuery) ToString(field string) string {
	var base string
	for i, sub := range q.clauses {
		s := sub.ToString(field)
		if _, ok := sub.(*BooleanQuery); ok {
			s = "(" + s + ")"
		}
		if i > 0 {
			base += " & "
		}
		base += s
	}
	res := "LogOdds(" + base + ")^" + fmt.Sprintf("%g", q.alpha)
	if q.signalWeights != nil {
		res += fmt.Sprintf(" w=%v", q.signalWeights)
	}
	return res
}

type logOddsFusionWeight struct {
	query     *LogOddsFusionQuery
	weights   []Weight
	scoreMode ScoreMode
}

// NewLogOddsFusionWeight creates a weight for a LogOddsFusionQuery.
func NewLogOddsFusionWeight(query *LogOddsFusionQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) *logOddsFusionWeight {
	weights := make([]Weight, len(query.clauses))
	for i, clause := range query.clauses {
		w, _ := searcher.CreateWeight(clause, scoreMode, boost)
		weights[i] = w
	}
	return &logOddsFusionWeight{
		query:     query,
		weights:   weights,
		scoreMode: scoreMode,
	}
}

func (w *logOddsFusionWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	var mis []Matches
	for _, weight := range w.weights {
		mi, err := weight.Matches(context, doc)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			mis = append(mis, mi)
		}
	}
	return MatchesFromSubMatches(mis), nil
}

func (w *logOddsFusionWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	var suppliers []ScorerSupplier
	var activeWeights []float32
	var activeLogitMin []float32
	var activeLogitMax []float32

	for i, weight := range w.weights {
		ss, err := weight.ScorerSupplier(context)
		if err != nil {
			return nil, err
		}
		if ss != nil {
			suppliers = append(suppliers, ss)
			if w.query.signalWeights != nil {
				activeWeights = append(activeWeights, w.query.signalWeights[i])
			}
			if w.query.logitMin != nil {
				activeLogitMin = append(activeLogitMin, w.query.logitMin[i])
				activeLogitMax = append(activeLogitMax, w.query.logitMax[i])
			}
		}
	}

	if len(suppliers) == 0 {
		return nil, nil
	} else if len(suppliers) == 1 {
		return suppliers[0], nil
	} else {
		totalClauses := len(w.query.clauses)
		alpha := w.query.alpha

		return &logOddsFusionScorerSupplier{
			suppliers:     suppliers,
			totalClauses:  totalClauses,
			alpha:         alpha,
			activeWeights: activeWeights,
			activeMin:     activeLogitMin,
			activeMax:     activeLogitMax,
			scoreMode:     w.scoreMode,
		}, nil
	}
}

func (w *logOddsFusionWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, weight := range w.weights {
		if !weight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (w *logOddsFusionWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	match := false
	var subsOnMatch []Explanation
	var subsOnNoMatch []Explanation
	var logitSum float64

	for i, weight := range w.weights {
		e, err := weight.Explain(context, doc)
		if err != nil {
			return nil, err
		}
		if e.IsMatch() {
			match = true
			subsOnMatch = append(subsOnMatch, e)
			subScore := float32(e.GetValue())
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
		scalingFactor := math.Pow(float64(len(w.weights)), float64(w.query.alpha))
		var scaledLogit float32
		var description string
		if w.query.signalWeights != nil {
			scaledLogit = float32(logitSum * scalingFactor)
			description = "weighted log-odds fusion, computed as sigmoid(weightedLogit * n^alpha) from:"
		} else {
			scaledLogit = float32((logitSum / float64(len(w.weights))) * scalingFactor)
			description = "log-odds fusion, computed as sigmoid(meanLogit * n^alpha) from:"
		}
		score := sigmoid(scaledLogit)
		return NewExplanationMatch(score, description, subsOnMatch), nil
	}
	return NewExplanationNoMatch("No matching clause", subsOnNoMatch), nil
}

type logOddsFusionScorerSupplier struct {
	suppliers     []ScorerSupplier
	totalClauses  int
	alpha         float32
	activeWeights []float32
	activeMin     []float32
	activeMax     []float32
	scoreMode     ScoreMode
}

func (s *logOddsFusionScorerSupplier) Get(leadCost int64) (Scorer, error) {
	var scorers []Scorer
	for _, ss := range s.suppliers {
		scorer, err := ss.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
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

func (s *logOddsFusionScorerSupplier) Cost() int64 {
	var cost int64
	for _, ss := range s.suppliers {
		cost += ss.Cost()
	}
	return cost
}

func (s *logOddsFusionScorerSupplier) SetTopLevelScoringClause() {
	for _, ss := range s.suppliers {
		ss.SetTopLevelScoringClause()
	}
}

func (w *logOddsFusionWeight) GetWeight() Weight {
	return w
}

var _ Weight = (*logOddsFusionWeight)(nil)
var _ Query = (*LogOddsFusionQuery)(nil)
