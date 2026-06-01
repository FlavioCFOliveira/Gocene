// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DisjunctionMaxQuery is a query that generates the union of documents produced by its subqueries,
// and that scores each document with the maximum score for that document produced by any subquery,
// plus a tie breaking increment for any additional matching subqueries.
type DisjunctionMaxQuery struct {
	*BaseQuery
	disjuncts            []Query
	tieBreakerMultiplier float32
}

// NewDisjunctionMaxQuery creates a new DisjunctionMaxQuery.
// A nil disjuncts slice is normalised to an empty (non-nil) slice.
func NewDisjunctionMaxQuery(disjuncts []Query) *DisjunctionMaxQuery {
	if disjuncts == nil {
		disjuncts = []Query{}
	}
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            disjuncts,
		tieBreakerMultiplier: 0.0,
	}
}

// NewDisjunctionMaxQueryWithTieBreaker creates a DisjunctionMaxQuery with a tie breaker multiplier.
// The tieBreakerMultiplier allows documents with multiple matching subqueries to be scored
// higher than documents with only a single matching subquery.
func NewDisjunctionMaxQueryWithTieBreaker(disjuncts []Query, tieBreakerMultiplier float32) *DisjunctionMaxQuery {
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            disjuncts,
		tieBreakerMultiplier: tieBreakerMultiplier,
	}
}

// Disjuncts returns the disjuncts (subqueries).
func (q *DisjunctionMaxQuery) Disjuncts() []Query {
	return q.disjuncts
}

// Add adds a subquery to this disjunction.
func (q *DisjunctionMaxQuery) Add(query Query) {
	q.disjuncts = append(q.disjuncts, query)
}

// TieBreakerMultiplier returns the tie breaker multiplier.
func (q *DisjunctionMaxQuery) TieBreakerMultiplier() float32 {
	return q.tieBreakerMultiplier
}

// SetTieBreakerMultiplier sets the tie breaker multiplier.
func (q *DisjunctionMaxQuery) SetTieBreakerMultiplier(tieBreakerMultiplier float32) {
	q.tieBreakerMultiplier = tieBreakerMultiplier
}

// Clone creates a copy of this query.
func (q *DisjunctionMaxQuery) Clone() Query {
	clonedDisjuncts := make([]Query, len(q.disjuncts))
	for i, disjunct := range q.disjuncts {
		if disjunct != nil {
			clonedDisjuncts[i] = disjunct.Clone()
		}
	}
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            clonedDisjuncts,
		tieBreakerMultiplier: q.tieBreakerMultiplier,
	}
}

// Equals checks if this query equals another.
func (q *DisjunctionMaxQuery) Equals(other Query) bool {
	if o, ok := other.(*DisjunctionMaxQuery); ok {
		if q.tieBreakerMultiplier != o.tieBreakerMultiplier || len(q.disjuncts) != len(o.disjuncts) {
			return false
		}
		for i, disjunct := range q.disjuncts {
			if disjunct == nil || o.disjuncts[i] == nil {
				if disjunct != nil || o.disjuncts[i] != nil {
					return false
				}
				continue
			}
			if !disjunct.Equals(o.disjuncts[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *DisjunctionMaxQuery) HashCode() int {
	hash := 0
	for _, disjunct := range q.disjuncts {
		if disjunct != nil {
			hash = hash*31 + disjunct.HashCode()
		}
	}
	return hash*31 + int(q.tieBreakerMultiplier*1000)
}

// Rewrite optimizes this query and its sub-queries. An empty disjunction
// becomes a MatchNoDocsQuery; a single disjunct unwraps to that disjunct; a
// tie-breaker of 1.0 collapses to a SHOULD BooleanQuery (the sum of the
// disjuncts); otherwise each sub-query is rewritten and, if any changed, a new
// DisjunctionMaxQuery is returned. Mirrors DisjunctionMaxQuery.rewrite.
func (q *DisjunctionMaxQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.disjuncts) == 0 {
		return NewMatchNoDocsQueryWithReason("empty DisjunctionMaxQuery"), nil
	}
	if len(q.disjuncts) == 1 {
		return q.disjuncts[0], nil
	}
	if q.tieBreakerMultiplier == 1.0 {
		bq := NewBooleanQuery()
		for _, sub := range q.disjuncts {
			bq.Add(sub, SHOULD)
		}
		return bq, nil
	}

	actuallyRewritten := false
	rewrittenDisjuncts := make([]Query, 0, len(q.disjuncts))
	for _, sub := range q.disjuncts {
		rewrittenSub, err := sub.Rewrite(reader)
		if err != nil {
			return nil, err
		}
		if rewrittenSub != sub {
			actuallyRewritten = true
		}
		rewrittenDisjuncts = append(rewrittenDisjuncts, rewrittenSub)
	}
	if actuallyRewritten {
		return NewDisjunctionMaxQueryWithTieBreaker(rewrittenDisjuncts, q.tieBreakerMultiplier), nil
	}
	return q, nil
}

// CreateWeight builds the Weight for this query. The bool-based entry point maps
// needsScores to a ScoreMode and delegates to CreateWeightScoreMode, so the
// full ScoreMode flows to the sub-weights.
func (q *DisjunctionMaxQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE
	if !needsScores {
		mode = COMPLETE_NO_SCORES
	}
	return q.CreateWeightScoreMode(searcher, mode, boost)
}

// CreateWeightScoreMode builds the DisjunctionMaxWeight, threading the full
// ScoreMode down to each disjunct's weight. Implements scoreModeWeightCreator.
func (q *DisjunctionMaxQuery) CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewDisjunctionMaxWeight(searcher, q, scoreMode, boost)
}
