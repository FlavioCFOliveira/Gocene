// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryDecomposer splits a disjunction query into its constituent parts so
// that they can be indexed and run separately in the Monitor.
//
// Port of org.apache.lucene.monitor.QueryDecomposer.
//
// Decomposes BooleanQuery (OR/SHOULD clauses), DisjunctionMaxQuery, and
// nested BooleanQuery wrappers into their individual leaf queries. Non-
// decomposable queries (TermQuery, PhraseQuery, etc.) are returned as-is.
type QueryDecomposer struct{}

// NewQueryDecomposer returns a default QueryDecomposer.
func NewQueryDecomposer() *QueryDecomposer { return &QueryDecomposer{} }

// Decompose splits the query into indexable sub-queries.
// BooleanQuery (disjunctive) and DisjunctionMaxQuery are decomposed into
// their constituent leaf clauses. Nested BooleanQuery wrappers are
// recursively decomposed. Non-decomposable queries are returned as-is.
func (d *QueryDecomposer) Decompose(q search.Query) []search.Query {
	if q == nil {
		return nil
	}

	// BooleanQuery: decompose SHOULD / MUST clauses into individual queries.
	if bq, ok := q.(*search.BooleanQuery); ok {
		return d.decomposeBoolean(bq)
	}

	// DisjunctionMaxQuery: decompose sub-queries.
	if dmq, ok := q.(*search.DisjunctionMaxQuery); ok {
		return d.decomposeDisjunctionMax(dmq)
	}

	// BoostQuery: unwrap the wrapped query.
	if boostQ, ok := q.(*search.BoostQuery); ok {
		return d.Decompose(boostQ.Query())
	}

	// Non-decomposable leaf query — return as-is.
	return []search.Query{q}
}

// decomposeBoolean decomposes a BooleanQuery into its constituent leaf queries.
// SHOULD clauses are decomposed individually; MUST clauses that are single
// leaf queries are decomposed; nested BooleanQuery clauses are recursively
// decomposed.
func (d *QueryDecomposer) decomposeBoolean(bq *search.BooleanQuery) []search.Query {
	var result []search.Query

	for _, clause := range bq.Clauses() {
		switch clause.Occur {
		case search.SHOULD:
			// SHOULD clauses are direct candidates for decomposition.
			result = append(result, d.Decompose(clause.Query)...)

		case search.MUST:
			// MUST clauses with single leaf sub-queries are decomposable.
			subs := d.Decompose(clause.Query)
			result = append(result, subs...)

		case search.FILTER:
			// FILTER clauses decompose like MUST.
			subs := d.Decompose(clause.Query)
			result = append(result, subs...)

		case search.MUST_NOT:
			// MUST_NOT clauses are not individually indexable; skip.
		}
	}

	// Deduplicate: remove structurally identical queries.
	return deduplicateQueries(result)
}

// decomposeDisjunctionMax decomposes a DisjunctionMaxQuery.
func (d *QueryDecomposer) decomposeDisjunctionMax(dmq *search.DisjunctionMaxQuery) []search.Query {
	var result []search.Query
	for _, sub := range dmq.Disjuncts() {
		result = append(result, d.Decompose(sub)...)
	}
	return deduplicateQueries(result)
}

// deduplicateQueries removes structurally identical (Equals) queries.
func deduplicateQueries(queries []search.Query) []search.Query {
	if len(queries) <= 1 {
		return queries
	}
	seen := make(map[int]bool, len(queries))
	out := make([]search.Query, 0, len(queries))
	for _, q := range queries {
		h := q.HashCode()
		if seen[h] {
			// Check full equality for collision safety.
			dup := false
			for _, existing := range out {
				if q.Equals(existing) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
		}
		seen[h] = true
		out = append(out, q)
	}
	return out
}
