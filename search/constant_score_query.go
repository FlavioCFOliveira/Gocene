// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// ConstantScoreQuery wraps another query and assigns a constant score to all matching documents.
// This is useful when you want to filter documents without affecting the ranking based on
// the original query's scoring.
type ConstantScoreQuery struct {
	*BaseQuery
	query Query
	score float32
}

// NewConstantScoreQuery creates a new ConstantScoreQuery.
// The default score is 1.0.
func NewConstantScoreQuery(query Query) *ConstantScoreQuery {
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
		score:     1.0,
	}
}

// NewConstantScoreQueryWithScore creates a ConstantScoreQuery with a custom score.
func NewConstantScoreQueryWithScore(query Query, score float32) *ConstantScoreQuery {
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
		score:     score,
	}
}

// Query returns the wrapped query.
func (q *ConstantScoreQuery) Query() Query {
	return q.query
}

// Score returns the constant score.
func (q *ConstantScoreQuery) Score() float32 {
	return q.score
}

// SetScore sets the constant score.
func (q *ConstantScoreQuery) SetScore(score float32) {
	q.score = score
}

// Clone creates a copy of this query.
func (q *ConstantScoreQuery) Clone() Query {
	if q.query == nil {
		return &ConstantScoreQuery{
			BaseQuery: &BaseQuery{},
			query:     nil,
			score:     q.score,
		}
	}
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     q.query.Clone(),
		score:     q.score,
	}
}

// Equals checks if this query equals another.
func (q *ConstantScoreQuery) Equals(other Query) bool {
	if o, ok := other.(*ConstantScoreQuery); ok {
		if q.score != o.score {
			return false
		}
		if q.query == nil || o.query == nil {
			return q.query == nil && o.query == nil
		}
		return q.query.Equals(o.query)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *ConstantScoreQuery) HashCode() int {
	hash := 0
	if q.query != nil {
		hash = q.query.HashCode()
	}
	return hash*31 + int(q.score*1000)
}

// Rewrite rewrites the query to a simpler form until convergence.
// Mirrors ConstantScoreQuery.rewrite(IndexSearcher) from Lucene 10.4.0. The outer
// convergence loop mirrors IndexSearcher.rewrite(), which is not modelled separately
// in Gocene; instead each Rewrite() that may produce intermediate results runs the
// loop internally.
func (q *ConstantScoreQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.query == nil {
		return q, nil
	}

	// Fully rewrite the inner query (mirrors IndexSearcher.rewrite on inner).
	inner, err := fullyRewrite(q.query, reader)
	if err != nil {
		return nil, err
	}

	// Extra simplifications legal because scores are not needed on the wrapped query.
	rewritten := inner
	if bq, ok := rewritten.(*BoostQuery); ok {
		rewritten = bq.Query()
	} else if csq, ok := rewritten.(*ConstantScoreQuery); ok {
		rewritten = csq.Query()
	} else if bq2, ok := rewritten.(*BooleanQuery); ok {
		rewritten = bq2.rewriteNoScoring()
	}

	// Bubble up MatchNoDocsQuery.
	if isMatchNoDocsQuery(rewritten) {
		return rewritten, nil
	}

	if rewritten != q.query {
		// The inner changed; recurse to converge any further simplifications.
		return NewConstantScoreQuery(rewritten).Rewrite(reader)
	}

	return q, nil
}

// CreateWeight builds the Weight for this ConstantScoreQuery, mirroring
// org.apache.lucene.search.ConstantScoreQuery.createWeight (Lucene 10.4.0).
//
// The wrapped query's Weight is always created without scores and with a
// boost of 1.0 (Lucene calls searcher.createWeight(query, innerScoreMode, 1f)).
// When the caller does not need scores, the inner Weight is returned directly
// — its scorers already iterate the correct doc set and produce no scores,
// which is exactly the filtering behaviour a ConstantScoreQuery wants.
//
// When scores are needed, the inner Weight is wrapped in a
// ConstantScoreWeight whose per-leaf ScorerSupplier delegates iteration to the
// inner ScorerSupplier and re-scores every matching document at the constant
// score q.Score()*boost (q.Score() defaults to 1.0, so the constant equals the
// boost — matching Lucene, where score() == this.boost). A nil inner
// ScorerSupplier means the inner query matches nothing on the leaf, so the
// outer supplier is nil too (no matches). Cacheability and Matches are
// delegated to the inner Weight, again mirroring the Java anonymous subclass.
//
// Deviations from Java:
//   - TwoPhaseIterator is not yet wired in Gocene's Scorer surface, so the
//     inner Scorer is consumed directly as a DocIdSetIterator (Scorer embeds
//     DocIdSetIterator) instead of preferring twoPhaseIterator() when present.
//   - The Java ConstantBulkScorer specialisation is not reproduced; the
//     ConstantScoreWeight's default BulkScorer (DefaultBulkScorer over the
//     constant-score Scorer) is used instead.
//
// This bool-based entry point exists for the stable Query.CreateWeight
// signature; it delegates to the ScoreMode-aware CreateWeightScoreMode with the
// coarsest mode that still satisfies the caller (COMPLETE when scores are
// needed, COMPLETE_NO_SCORES otherwise). IndexSearcher always reaches
// ConstantScoreQuery through CreateWeightScoreMode (via createWeight dispatch),
// so the full ScoreMode — including TOP_SCORES / TOP_DOCS — is preserved on the
// real search path.
func (q *ConstantScoreQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}
	return q.CreateWeightScoreMode(searcher, mode, boost)
}

// CreateWeightScoreMode builds the Weight for this ConstantScoreQuery under the
// given full ScoreMode, mirroring
// org.apache.lucene.search.ConstantScoreQuery.createWeight (Lucene 10.4.0).
//
// The wrapped query is always built at boost 1.0 and never observes the outer
// score mode directly. Instead, following Lucene, an exhaustive outer mode
// (COMPLETE / COMPLETE_NO_SCORES) forwards COMPLETE_NO_SCORES to the inner
// query, while a non-exhaustive outer mode (TOP_SCORES / TOP_DOCS) forwards
// TOP_DOCS — so dynamic-pruning optimisations are not disabled for queries
// sorted by field or by top scores. The inner Weight is created through the
// searcher's createWeight dispatch so a composite inner query (e.g. another
// BooleanQuery or ConstantScoreQuery) sees the forwarded mode.
func (q *ConstantScoreQuery) CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	if q.query == nil {
		// A ConstantScoreQuery with no inner query matches nothing; a nil
		// Weight is interpreted as a no-match by IndexSearcher.
		return nil, nil
	}

	// If the outer mode is exhaustive then pass COMPLETE_NO_SCORES, otherwise
	// pass TOP_DOCS to preserve dynamic pruning for top-score / top-doc queries.
	innerScoreMode := TOP_DOCS
	if scoreMode.isExhaustive() {
		innerScoreMode = COMPLETE_NO_SCORES
	}

	// The wrapped query is always created at boost 1.0
	// (Lucene: searcher.createWeight(query, innerScoreMode, 1f)).
	inner, err := searcher.CreateWeight(q.query, innerScoreMode, 1.0)
	if err != nil {
		return nil, err
	}

	// When the caller does not need scores, the inner Weight already produces
	// the correct doc set with no scoring; return it directly.
	if !scoreMode.needsScores() {
		return inner, nil
	}

	// A nil inner Weight means the inner query matches nothing; propagate that
	// as a no-match Weight rather than building a wrapper that would NPE on the
	// nil inner Weight inside the supplier closure.
	if inner == nil {
		return nil, nil
	}

	// Constant score handed to every matching document. q.Score() defaults to
	// 1.0, so this equals boost in the common case — matching Lucene's
	// score() == this.boost.
	score := q.score * boost

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		innerSupplier, err := inner.ScorerSupplier(ctx)
		if err != nil {
			return nil, err
		}
		if innerSupplier == nil {
			// Inner query matches nothing on this leaf.
			return nil, nil
		}
		// Wrap the inner supplier: every Get rebuilds the inner Scorer and
		// re-scores its doc set at the constant score. The cost is taken from
		// the inner supplier so cost-based optimisations stay accurate.
		return NewConstantScoreScorerSupplier(
			score,
			COMPLETE,
			innerSupplier.Cost(),
			func(leadCost int64) (DocIdSetIterator, error) {
				innerScorer, err := innerSupplier.Get(leadCost)
				if err != nil {
					return nil, err
				}
				if innerScorer == nil {
					return NewEmptyDocIdSetIterator(), nil
				}
				// Scorer embeds DocIdSetIterator, so the inner scorer is its
				// own iterator (TwoPhaseIterator is not yet modelled).
				return innerScorer, nil
			},
		), nil
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return inner.IsCacheable(ctx)
	}

	return NewConstantScoreWeight(q, score, supplier, cacheable), nil
}

// fullyRewrite rewrites a query to a fixed point, mirroring IndexSearcher.rewrite().
// It is used by wrapper queries (CSQ, BoostQuery) that need to converge their inner
// query before applying their own simplification rules.
func fullyRewrite(query Query, reader IndexReader) (Query, error) {
	for {
		var next Query
		var err error
		if bq, ok := query.(*BooleanQuery); ok {
			next, err = bq.rewriteStep(reader)
		} else {
			next, err = query.Rewrite(reader)
		}
		if err != nil {
			return nil, err
		}
		if next == query {
			return query, nil
		}
		query = next
	}
}
