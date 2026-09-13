// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BayesianScoreQuery is a query wrapper that transforms the inner query's score into a calibrated probability via sigmoid
// calibration: P = sigmoid(alpha * (score - beta)).
//
// This implements the query-level Bayesian transform from "Bayesian BM25": the inner query
// (typically a multi-term BooleanQuery with BM25Similarity) produces a raw score, and this wrapper
// maps it to a probability in (0, 1) suitable for combination with other probability signals via
// LogOddsFusionQuery.
//
// The alpha parameter controls the sigmoid steepness (score sensitivity), and beta controls the
// midpoint (decision boundary). These can be set manually or estimated from the score distribution
// via BayesianScoreEstimator.
//
// An optional base rate encodes the corpus-level prior probability that a random document is
// relevant to a random query. When set, the posterior is computed in log-odds space:
// sigmoid(alpha * (score - beta) + logit(baseRate)). This shifts scores down for rare-relevance
// corpora, improving calibration.
type BayesianScoreQuery struct {
	query         Query
	alpha         float32
	beta          float32
	baseRate      float32
	logitBaseRate float32
}

// NewBayesianScoreQuery creates a BayesianScoreQuery with base rate.
// alpha: sigmoid steepness (must be positive and finite).
// beta: sigmoid midpoint (must be finite).
// baseRate: corpus-level relevance prior in [0, 1), or 0 to disable.
func NewBayesianScoreQuery(query Query, alpha, beta, baseRate float32) *BayesianScoreQuery {
	if query == nil {
		panic("query must not be null")
	}
	if math.IsInf(float64(alpha), 0) || math.IsNaN(float64(alpha)) || alpha <= 0 {
		panic(fmt.Sprintf("alpha must be a positive finite value, got %f", alpha))
	}
	if math.IsInf(float64(beta), 0) || math.IsNaN(float64(beta)) {
		panic(fmt.Sprintf("beta must be a finite value, got %f", beta))
	}
	if baseRate < 0 || baseRate >= 1 {
		panic(fmt.Sprintf("baseRate must be in [0, 1), got %f", baseRate))
	}

	logitBaseRate := float32(0)
	if baseRate > 0 {
		logitBaseRate = float32(math.Log(float64(baseRate / (1.0 - baseRate))))
	}

	return &BayesianScoreQuery{
		query:         query,
		alpha:         alpha,
		beta:          beta,
		baseRate:      baseRate,
		logitBaseRate: logitBaseRate,
	}
}

// NewBayesianScoreQuerySimple creates a BayesianScoreQuery without base rate.
func NewBayesianScoreQuerySimple(query Query, alpha, beta float32) *BayesianScoreQuery {
	return NewBayesianScoreQuery(query, alpha, beta, 0)
}

// GetQuery returns the wrapped query.
func (q *BayesianScoreQuery) GetQuery() Query {
	return q.query
}

// GetAlpha returns the sigmoid steepness parameter.
func (q *BayesianScoreQuery) GetAlpha() float32 {
	return q.alpha
}

// GetBeta returns the sigmoid midpoint parameter.
func (q *BayesianScoreQuery) GetBeta() float32 {
	return q.beta
}

// GetBaseRate returns the base rate, or 0 if not set.
func (q *BayesianScoreQuery) GetBaseRate() float32 {
	return q.baseRate
}

func bayesianScoreQuerySigmoid(x float32) float32 {
	if x >= 0 {
		return float32(1.0 / (1.0 + math.Exp(float64(-x))))
	}
	expX := math.Exp(float64(x))
	return float32(expX / (1.0 + expX))
}

// CreateWeight creates a Weight for the query.
func (q *BayesianScoreQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	innerWeight, err := q.query.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if !scoreMode.NeedsScores() {
		return innerWeight, nil
	}
	return &bayesianScoreWeight{
		query:       q,
		innerWeight: innerWeight,
	}, nil
}

// Rewrite rewrites the query.
func (q *BayesianScoreQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewritten, err := q.query.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); ok {
		return rewritten, nil
	}
	if rewritten != q.query {
		return NewBayesianScoreQuery(rewritten, q.alpha, q.beta, q.baseRate), nil
	}
	return q, nil
}

// Visit visits the query.
func (q *BayesianScoreQuery) Visit(visitor QueryVisitor) {
	// Java: `query.visit(visitor.getSubVisitor(BooleanClause.Occur.MUST, this))`
	// (BayesianScoreQuery.java:145-147). The sub-visitor is the ARGUMENT to the
	// child query's visit, not the receiver.
	visitQuery(q.query, visitor.GetSubVisitor(MUST, q))
}

// String returns a string representation of the query.
func (q *BayesianScoreQuery) String(field string) string {
	base := fmt.Sprintf("BayesianScore(%s, alpha=%f, beta=%f", queryToString(q.query, field), q.alpha, q.beta)
	if q.baseRate > 0 {
		base += fmt.Sprintf(", baseRate=%f", q.baseRate)
	}
	return base + ")"
}

// Equals reports whether q and other model the same query.
func (q *BayesianScoreQuery) Equals(other spi.Query) bool {
	if q == other {
		return true
	}
	if other == nil {
		return false
	}
	that, ok := other.(*BayesianScoreQuery)
	if !ok {
		return false
	}
	return q.query.Equals(that.query) &&
		math.Float32bits(q.alpha) == math.Float32bits(that.alpha) &&
		math.Float32bits(q.beta) == math.Float32bits(that.beta) &&
		math.Float32bits(q.baseRate) == math.Float32bits(that.baseRate)
}

// HashCode returns a hash consistent with Equals.
func (q *BayesianScoreQuery) HashCode() int {
	h := 1
	h = 31*h + q.query.HashCode()
	h = 31*h + int(math.Float32bits(q.alpha))
	h = 31*h + int(math.Float32bits(q.beta))
	h = 31*h + int(math.Float32bits(q.baseRate))
	return h
}

type bayesianScoreWeight struct {
	BaseWeight
	query       *BayesianScoreQuery
	innerWeight Weight
}

// Matches returns the matches for the document.
func (w *bayesianScoreWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return w.innerWeight.Matches(context, doc)
}

// Explain returns an explanation for the score.
func (w *bayesianScoreWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	innerExpl, err := w.innerWeight.Explain(context, doc)
	if err != nil {
		return nil, err
	}
	if !innerExpl.IsMatch() {
		return innerExpl, nil
	}

	innerScore := innerExpl.GetValue()
	logOdds := w.query.alpha*(innerScore-w.query.beta) + w.query.logitBaseRate
	transformed := bayesianScoreQuerySigmoid(logOdds)

	if w.query.baseRate > 0 {
		return MatchExplanationWithDetails(
			transformed,
			"sigmoid calibration with base rate, computed as sigmoid(alpha * (score - beta) + logit(baseRate)) from:",
			innerExpl,
			MatchExplanation(w.query.alpha, "alpha, sigmoid steepness"),
			MatchExplanation(w.query.beta, "beta, sigmoid midpoint"),
			MatchExplanation(w.query.baseRate, "baseRate, corpus-level relevance prior"),
		), nil
	}

	return MatchExplanationWithDetails(
		transformed,
		"sigmoid calibration, computed as sigmoid(alpha * (score - beta)) from:",
		innerExpl,
		MatchExplanation(w.query.alpha, "alpha, sigmoid steepness"),
		MatchExplanation(w.query.beta, "beta, sigmoid midpoint"),
	), nil
}

// ScorerSupplier returns a scorer supplier.
func (w *bayesianScoreWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	innerSupplier, err := w.innerWeight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if innerSupplier == nil {
		return nil, nil
	}
	return &bayesianScoreScorerSupplier{
		innerSupplier: innerSupplier,
		query:         w.query,
	}, nil
}

func (w *bayesianScoreWeight) Count(context *index.LeafReaderContext) (int, error) {
	return w.innerWeight.Count(context)
}

func (w *bayesianScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.innerWeight.IsCacheable(ctx)
}

type bayesianScoreScorerSupplier struct {
	innerSupplier ScorerSupplier
	query         *BayesianScoreQuery
}

func (s *bayesianScoreScorerSupplier) Get(leadCost int64) (Scorer, error) {
	innerScorer, err := s.innerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	return &bayesianScoreScorer{
		inner: innerScorer,
		query: s.query,
	}, nil
}

func (s *bayesianScoreScorerSupplier) Cost() int64 {
	return s.innerSupplier.Cost()
}

func (s *bayesianScoreScorerSupplier) SetTopLevelScoringClause() error {
	return s.innerSupplier.SetTopLevelScoringClause()
}

type bayesianScoreScorer struct {
	inner Scorer
	query *BayesianScoreQuery
}

func (s *bayesianScoreScorer) Score() (float32, error) {
	innerScore, err := s.inner.Score()
	if err != nil {
		return 0, err
	}
	return bayesianScoreQuerySigmoid(s.query.alpha*(innerScore-s.query.beta) + s.query.logitBaseRate), nil
}

func (s *bayesianScoreScorer) DocID() int {
	return s.inner.DocID()
}

// Iterator mirrors `public final DocIdSetIterator iterator()` of FilterScorer
// in Apache Lucene 10.5.0 (FilterScorer.java:58-61), which BayesianScoreScorer
// inherits: `return in.iterator()`. The return type is
// org.apache.lucene.search.DocIdSetIterator, so the Go rendering is the
// search package's DocIdSetIterator, not util's.
func (s *bayesianScoreScorer) Iterator() DocIdSetIterator {
	return s.inner.Iterator()
}

func (s *bayesianScoreScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.inner.TwoPhaseIterator()
}

func (s *bayesianScoreScorer) AdvanceShallow(target int) (int, error) {
	return s.inner.AdvanceShallow(target)
}

func (s *bayesianScoreScorer) GetMaxScore(upTo int) (float32, error) {
	innerMax, err := s.inner.GetMaxScore(upTo)
	if err != nil {
		return 0, err
	}
	return bayesianScoreQuerySigmoid(s.query.alpha*(innerMax-s.query.beta) + s.query.logitBaseRate), nil
}

func (s *bayesianScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	// For a wrapper, we let the inner scorer fill the buffer and then we transform the scores.
	err := s.inner.NextDocsAndScores(upTo, liveDocs, buffer)
	if err != nil {
		return err
	}
	for i := 0; i < buffer.Size; i++ {
		score := buffer.Features[i]
		buffer.Features[i] = bayesianScoreQuerySigmoid(s.query.alpha*(score-s.query.beta) + s.query.logitBaseRate)
	}
	return nil
}

func (s *bayesianScoreScorer) SmoothingScore(docID int) (float32, error) {
	return s.inner.SmoothingScore(docID)
}

func (s *bayesianScoreScorer) SetMinCompetitiveScore(minScore float32) error {
	if minScore > 0 && minScore < 1 {
		clamped := minScore
		if clamped < 1e-7 {
			clamped = 1e-7
		} else if clamped > 1-1e-7 {
			clamped = 1 - 1e-7
		}
		logitMin := float32(math.Log(float64(clamped / (1.0 - clamped))))
		innerMin := (logitMin-s.query.logitBaseRate)/s.query.alpha + s.query.beta
		if innerMin < 0 {
			innerMin = 0
		}
		return s.inner.SetMinCompetitiveScore(innerMin)
	}
	return nil
}

func (s *bayesianScoreScorer) GetChildren() ([]ChildScorable, error) {
	return s.inner.GetChildren()
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (b *bayesianScoreScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(b)
}
