// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// BlendedTermQuery blends scores from multiple terms.
// This is the Go port of Lucene's org.apache.lucene.search.BlendedTermQuery.
type BlendedTermQuery struct {
	BaseQuery
	terms  []*index.Term
	boosts []float32
}

// NewBlendedTermQuery creates a new BlendedTermQuery.
func NewBlendedTermQuery(terms ...*index.Term) *BlendedTermQuery {
	boosts := make([]float32, len(terms))
	for i := range boosts {
		boosts[i] = 1.0
	}
	return &BlendedTermQuery{
		terms:  terms,
		boosts: boosts,
	}
}

// GetField returns the field of the first term (all terms should have the same field).
func (q *BlendedTermQuery) GetField() string {
	if len(q.terms) > 0 {
		return q.terms[0].Field
	}
	return ""
}

// GetTerms returns the terms.
func (q *BlendedTermQuery) GetTerms() []*index.Term {
	return q.terms
}

// GetBoosts returns the boosts for each term.
func (q *BlendedTermQuery) GetBoosts() []float32 {
	return q.boosts
}

// SetBoost sets the boost for a term at the given index.
func (q *BlendedTermQuery) SetBoost(index int, boost float32) {
	if index >= 0 && index < len(q.boosts) {
		q.boosts[index] = boost
	}
}

// Rewrite rewrites this query to a simpler form.
func (q *BlendedTermQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.terms) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	if len(q.terms) == 1 {
		return NewTermQuery(q.terms[0]), nil
	}
	// For multiple terms, create a boolean query with SHOULD clauses
	bq := NewBooleanQuery()
	for i, term := range q.terms {
		tq := NewTermQuery(term)
		// Apply boost if different from 1.0
		if q.boosts[i] != 1.0 {
			bq.Add(NewBoostQuery(tq, q.boosts[i]), SHOULD)
		} else {
			bq.Add(tq, SHOULD)
		}
	}
	return bq, nil
}

// CreateWeight creates a Weight for this query.
func (q *BlendedTermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Rewrite to boolean query and create weight
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// Clone creates a copy of this query.
func (q *BlendedTermQuery) Clone() Query {
	termsCopy := make([]*index.Term, len(q.terms))
	for i, term := range q.terms {
		termsCopy[i] = index.NewTerm(term.Field, term.Text())
	}
	bq := NewBlendedTermQuery(termsCopy...)
	bq.boosts = make([]float32, len(q.boosts))
	copy(bq.boosts, q.boosts)
	return bq
}

// Equals checks if this query equals another.
func (q *BlendedTermQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*BlendedTermQuery); ok {
		if len(q.terms) != len(o.terms) {
			return false
		}
		for i := range q.terms {
			if !q.terms[i].Equals(o.terms[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *BlendedTermQuery) HashCode() int {
	h := 17
	for _, term := range q.terms {
		h = 31*h + term.HashCode()
	}
	return h
}

// String returns a string representation of the query.
func (q *BlendedTermQuery) String() string {
	return "BlendedTermQuery(terms=" + string(rune(len(q.terms)+'0')) + ")"
}

// Ensure BlendedTermQuery implements Query
var _ Query = (*BlendedTermQuery)(nil)
