// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// CombinedFieldQuery searches across multiple fields as if they were a single field.
// This is useful for searching across multiple fields that represent the same content
// (e.g., title, body, and keywords) and treating them as one combined field.
//
// The query blends document frequencies and collection statistics across fields
// to provide more accurate scoring.
type CombinedFieldQuery struct {
	BaseQuery
	fieldTerms map[string][]*index.Term
	boosts     map[string]float32
}

// NewCombinedFieldQuery creates a new CombinedFieldQuery.
// The fieldTerms map contains field names mapped to terms to search for in each field.
func NewCombinedFieldQuery(fieldTerms map[string][]*index.Term) *CombinedFieldQuery {
	boosts := make(map[string]float32)
	for field := range fieldTerms {
		boosts[field] = 1.0
	}
	return &CombinedFieldQuery{
		fieldTerms: fieldTerms,
		boosts:     boosts,
	}
}

// NewCombinedFieldQueryWithTerms creates a CombinedFieldQuery from a list of terms
// that will be searched across all specified fields.
func NewCombinedFieldQueryWithTerms(fields []string, terms []*index.Term) *CombinedFieldQuery {
	fieldTerms := make(map[string][]*index.Term)
	for _, field := range fields {
		fieldTerms[field] = terms
	}
	return NewCombinedFieldQuery(fieldTerms)
}

// GetFieldTerms returns the field to terms mapping.
func (q *CombinedFieldQuery) GetFieldTerms() map[string][]*index.Term {
	return q.fieldTerms
}

// GetFields returns all fields in this query.
func (q *CombinedFieldQuery) GetFields() []string {
	fields := make([]string, 0, len(q.fieldTerms))
	for field := range q.fieldTerms {
		fields = append(fields, field)
	}
	return fields
}

// GetTermsForField returns the terms for a specific field.
func (q *CombinedFieldQuery) GetTermsForField(field string) []*index.Term {
	return q.fieldTerms[field]
}

// SetBoost sets the boost for a field.
func (q *CombinedFieldQuery) SetBoost(field string, boost float32) {
	q.boosts[field] = boost
}

// GetBoost returns the boost for a field.
func (q *CombinedFieldQuery) GetBoost(field string) float32 {
	if boost, ok := q.boosts[field]; ok {
		return boost
	}
	return 1.0
}

// Rewrite rewrites this query to a simpler form.
// Combines all field queries into a single BooleanQuery with SHOULD clauses.
func (q *CombinedFieldQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.fieldTerms) == 0 {
		return NewMatchNoDocsQuery(), nil
	}

	// Create a boolean query to combine all field queries
	bq := NewBooleanQuery()

	for field, terms := range q.fieldTerms {
		if len(terms) == 0 {
			continue
		}

		// Create a blended term query for this field's terms
		blended := NewBlendedTermQuery(terms...)
		boost := q.boosts[field]

		// Apply boost if different from 1.0
		if boost != 1.0 {
			bq.Add(NewBoostQuery(blended, boost), SHOULD)
		} else {
			bq.Add(blended, SHOULD)
		}
	}

	// If only one clause, return it directly
	clauses := bq.Clauses()
	if len(clauses) == 1 {
		return clauses[0].Query, nil
	}

	return bq, nil
}

// CreateWeight creates a Weight for this query.
func (q *CombinedFieldQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Rewrite and create weight
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// Clone creates a copy of this query.
func (q *CombinedFieldQuery) Clone() Query {
	fieldTermsCopy := make(map[string][]*index.Term)
	for field, terms := range q.fieldTerms {
		termsCopy := make([]*index.Term, len(terms))
		for i, term := range terms {
			termsCopy[i] = index.NewTerm(term.Field, term.Text())
		}
		fieldTermsCopy[field] = termsCopy
	}

	cq := NewCombinedFieldQuery(fieldTermsCopy)
	for field, boost := range q.boosts {
		cq.boosts[field] = boost
	}
	return cq
}

// Equals checks if this query equals another.
func (q *CombinedFieldQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*CombinedFieldQuery); ok {
		if len(q.fieldTerms) != len(o.fieldTerms) {
			return false
		}
		for field, terms := range q.fieldTerms {
			otherTerms, ok := o.fieldTerms[field]
			if !ok || len(terms) != len(otherTerms) {
				return false
			}
			for i, term := range terms {
				if !term.Equals(otherTerms[i]) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *CombinedFieldQuery) HashCode() int {
	h := 17
	for field, terms := range q.fieldTerms {
		h = 31*h + index.NewTerm(field, "").HashCode()
		for _, term := range terms {
			h = 31*h + term.HashCode()
		}
	}
	return h
}

// String returns a string representation of the query.
func (q *CombinedFieldQuery) String() string {
	return "CombinedFieldQuery(fields=" + string(rune(len(q.fieldTerms)+'0')) + ")"
}

// Ensure CombinedFieldQuery implements Query
var _ Query = (*CombinedFieldQuery)(nil)
