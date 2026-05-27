// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// extractQueryTerms walks the query tree and collects all terms reachable
// from leaf TermQuery / PhraseQuery / SpanTermQuery nodes, along with their
// effective boost weights.
//
// This is the Go port of the recursive term extraction implemented in
// Lucene 10.4.0's org.apache.lucene.search.highlight.QueryTermExtractor and
// WeightedSpanTermExtractor. The supported query types mirror the contract
// referenced by T4675: BooleanQuery, PhraseQuery, TermQuery, MultiTermQuery
// (no per-term enumeration without an index reader — best-effort field-only
// recognition), SpanQuery (concrete span term query), BoostQuery,
// DisjunctionMaxQuery, and ConstantScoreQuery.
//
// Parameters:
//   - query: the query tree to extract from
//   - field: optional field filter; when non-empty, only terms in this field
//     are returned. When empty, terms from all fields are returned.
//   - boost: the boost accumulated from outer BoostQuery wrappers.
//   - terms: target slice for collected terms (mutated in-place).
//   - weights: target map of term -> max accumulated weight (mutated).
//
// Returns the (possibly extended) terms slice.
func extractQueryTerms(query search.Query, field string, boost float32, terms []string, weights map[string]float32) []string {
	if query == nil {
		return terms
	}

	add := func(termField, termText string, weight float32) {
		if termText == "" {
			return
		}
		if field != "" && termField != field {
			return
		}
		if existing, ok := weights[termText]; !ok || weight > existing {
			weights[termText] = weight
		}
		// Track first occurrence; do not duplicate.
		for _, t := range terms {
			if t == termText {
				return
			}
		}
		terms = append(terms, termText)
	}

	switch q := query.(type) {

	case *search.TermQuery:
		t := q.Term()
		if t != nil {
			add(t.Field, t.Text(), boost)
		}

	case *search.PhraseQuery:
		for _, t := range q.Terms() {
			if t != nil {
				add(t.Field, t.Text(), boost)
			}
		}

	case *search.SpanTermQuery:
		t := q.Term()
		if t != nil {
			add(t.Field, t.Text(), boost)
		}

	case *search.BooleanQuery:
		for _, clause := range q.Clauses() {
			if clause == nil || clause.Occur == search.MUST_NOT {
				continue
			}
			terms = extractQueryTerms(clause.Query, field, boost, terms, weights)
		}

	case *search.BoostQuery:
		terms = extractQueryTerms(q.Query(), field, boost*q.Boost(), terms, weights)

	case *search.DisjunctionMaxQuery:
		for _, d := range q.Disjuncts() {
			terms = extractQueryTerms(d, field, boost, terms, weights)
		}

	case *search.ConstantScoreQuery:
		terms = extractQueryTerms(q.Query(), field, boost, terms, weights)

	default:
		// Best-effort fallback: query types without a structural Terms()
		// accessor (e.g. MultiTermQuery prefixes/wildcards/ranges) cannot
		// be fully enumerated without an IndexReader. The Lucene
		// reference handles those via Query.visit; we follow the
		// conservative subset that is recognisable by concrete type.
	}

	return terms
}

// Common BooleanClause occur identifier — alias to the search package
// constants so this file does not need to reach into search internals.
var (
	_ = search.MUST
	_ = search.SHOULD
	_ = search.MUST_NOT
)
