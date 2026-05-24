// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

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
