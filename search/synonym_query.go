// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SynonymQuery is a query that matches documents containing any of the specified terms.
// This is the Go port of Lucene's org.apache.lucene.search.SynonymQuery.
type SynonymQuery struct {
	*BaseQuery
	field string
	terms []*index.Term
	boosts []float32
}

// SynonymQueryBuilder builds a SynonymQuery.
type SynonymQueryBuilder struct {
	field  string
	terms  []*index.Term
	boosts []float32
}

// NewSynonymQueryBuilder creates a new SynonymQueryBuilder.
func NewSynonymQueryBuilder(field string) *SynonymQueryBuilder {
	return &SynonymQueryBuilder{
		field:  field,
		terms:  make([]*index.Term, 0),
		boosts: make([]float32, 0),
	}
}

// AddTerm adds a term with default boost of 1.0.
func (b *SynonymQueryBuilder) AddTerm(term *index.Term) *SynonymQueryBuilder {
	b.terms = append(b.terms, term)
	b.boosts = append(b.boosts, 1.0)
	return b
}

// AddTermWithBoost adds a term with a custom boost.
func (b *SynonymQueryBuilder) AddTermWithBoost(term *index.Term, boost float32) *SynonymQueryBuilder {
	b.terms = append(b.terms, term)
	b.boosts = append(b.boosts, boost)
	return b
}

// Build creates the SynonymQuery.
func (b *SynonymQueryBuilder) Build() *SynonymQuery {
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     b.field,
		terms:     b.terms,
		boosts:    b.boosts,
	}
}

// Clone creates a copy of this query.
func (q *SynonymQuery) Clone() Query {
	terms := make([]*index.Term, len(q.terms))
	copy(terms, q.terms)
	boosts := make([]float32, len(q.boosts))
	copy(boosts, q.boosts)
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		terms:     terms,
		boosts:    boosts,
	}
}

// Equals checks if this query equals another.
func (q *SynonymQuery) Equals(other Query) bool {
	if o, ok := other.(*SynonymQuery); ok {
		if q.field != o.field {
			return false
		}
		if len(q.terms) != len(o.terms) {
			return false
		}
		// Check if terms match (order doesn't matter for synonyms)
		termSet := make(map[string]bool)
		for _, t := range q.terms {
			termSet[t.String()] = true
		}
		for _, t := range o.terms {
			if !termSet[t.String()] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SynonymQuery) HashCode() int {
	hash := 0
	for _, t := range q.terms {
		hash = hash*31 + t.HashCode()
	}
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *SynonymQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SynonymQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return &SynonymWeight{BaseWeight: NewBaseWeight(q)}, nil
}

// GetField returns the field for this query.
func (q *SynonymQuery) GetField() string {
	return q.field
}

// GetTerms returns the terms for this query.
func (q *SynonymQuery) GetTerms() []*index.Term {
	return q.terms
}

// GetBoosts returns the boosts for this query.
func (q *SynonymQuery) GetBoosts() []float32 {
	return q.boosts
}

// String returns a string representation of this query.
func (q *SynonymQuery) String() string {
	return "SynonymQuery"
}

// SynonymWeight is the Weight implementation for SynonymQuery.
type SynonymWeight struct {
	*BaseWeight
}

// Scorer creates a scorer for this weight.
func (w *SynonymWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	return nil, nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *SynonymWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}

// Explain returns an explanation of the score for the given document.
func (w *SynonymWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "SynonymWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *SynonymWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	return nil, nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *SynonymWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *SynonymWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *SynonymWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure SynonymWeight implements Weight
var _ Weight = (*SynonymWeight)(nil)

// Ensure SynonymQuery implements Query
var _ Query = (*SynonymQuery)(nil)
