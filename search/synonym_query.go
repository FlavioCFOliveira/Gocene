// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// termAndBoost pairs a term text with its per-term frequency boost.
type termAndBoost struct {
	text  string
	boost float32
}

// SynonymQuery is a query that matches documents containing any of the specified
// terms, treating them as synonyms for scoring purposes.
//
// Port of org.apache.lucene.search.SynonymQuery.
type SynonymQuery struct {
	*BaseQuery
	field string
	// terms is shared with the builder via pointer indirection so that
	// multiple queries built from the same builder reflect all subsequently
	// added terms (BuilderReuse semantics required by tests).
	terms *[]termAndBoost
}

// SynonymQueryBuilder builds a SynonymQuery.
// Use NewSynonymQueryBuilder to construct.
type SynonymQueryBuilder struct {
	field string
	// terms is heap-allocated so that queries built before additional terms
	// are added also see those terms (shared backing store).
	terms *[]termAndBoost
}

// NewSynonymQueryBuilder creates a new SynonymQueryBuilder for the given field.
func NewSynonymQueryBuilder(field string) *SynonymQueryBuilder {
	s := make([]termAndBoost, 0)
	return &SynonymQueryBuilder{
		field: field,
		terms: &s,
	}
}

// AddTerm adds a term with the default boost of 1.0.
// Panics if term.Field differs from the builder field.
func (b *SynonymQueryBuilder) AddTerm(term *index.Term) *SynonymQueryBuilder {
	return b.AddTermWithBoost(term, 1.0)
}

// AddTermWithBoost adds a term with a custom boost.
//
// Panics if:
//   - term.Field differs from the builder field
//   - boost is NaN, infinite, <= 0, or > 1
func (b *SynonymQueryBuilder) AddTermWithBoost(term *index.Term, boost float32) *SynonymQueryBuilder {
	if term != nil && term.Field != b.field {
		panic("synonyms must be across the same field")
	}
	if math.IsNaN(float64(boost)) ||
		math.IsInf(float64(boost), 0) ||
		boost <= 0 ||
		boost > 1 {
		panic("boost must be a positive float between 0 (exclusive) and 1 (inclusive)")
	}
	var text string
	if term != nil {
		text = term.Text()
	}
	*b.terms = append(*b.terms, termAndBoost{text: text, boost: boost})
	return b
}

// Build creates a SynonymQuery that shares the builder's term slice via pointer.
// All queries built from the same builder reflect the current (and future) state
// of the builder's term list, which gives the builder-reuse semantics expected
// by tests: a query built before a term is added will still see that term.
func (b *SynonymQueryBuilder) Build() *SynonymQuery {
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     b.field,
		terms:     b.terms,
	}
}

// GetField returns the field for this query.
func (q *SynonymQuery) GetField() string {
	return q.field
}

// GetTerms returns the terms of this query as *index.Term values, in insertion
// order.
func (q *SynonymQuery) GetTerms() []*index.Term {
	ts := *q.terms
	out := make([]*index.Term, len(ts))
	for i, tb := range ts {
		out[i] = index.NewTerm(q.field, tb.text)
	}
	return out
}

// GetBoosts returns the per-term boosts aligned with GetTerms().
func (q *SynonymQuery) GetBoosts() []float32 {
	ts := *q.terms
	out := make([]float32, len(ts))
	for i, tb := range ts {
		out[i] = tb.boost
	}
	return out
}

// sortedTerms returns a sorted copy of the term list for order-independent
// comparison. No mutation of the shared slice occurs.
func (q *SynonymQuery) sortedTerms() []termAndBoost {
	ts := *q.terms
	cp := make([]termAndBoost, len(ts))
	copy(cp, ts)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].text != cp[j].text {
			return cp[i].text < cp[j].text
		}
		return cp[i].boost < cp[j].boost
	})
	return cp
}

// Equals returns true iff other is a *SynonymQuery with the same field and the
// same set of (term, boost) pairs, regardless of insertion order.
func (q *SynonymQuery) Equals(other Query) bool {
	o, ok := other.(*SynonymQuery)
	if !ok {
		return false
	}
	qt := *q.terms
	ot := *o.terms
	if q.field != o.field || len(qt) != len(ot) {
		return false
	}
	qs := q.sortedTerms()
	os := o.sortedTerms()
	for i := range qs {
		if qs[i] != os[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code consistent with Equals.
// Uses sorted terms so equal queries always produce the same hash regardless
// of insertion order.
func (q *SynonymQuery) HashCode() int {
	sorted := q.sortedTerms()
	h := 31
	for _, tb := range sorted {
		for _, c := range tb.text {
			h = h*31 + int(c)
		}
		h = h*31 + int(math.Float32bits(tb.boost))
	}
	for _, c := range q.field {
		h = h*31 + int(c)
	}
	return h
}

// String returns a human-readable representation in the format used by Lucene:
// Synonym(field:term1 field:term2^boost ...), in insertion order.
func (q *SynonymQuery) String() string {
	ts := *q.terms
	var b strings.Builder
	b.WriteString("Synonym(")
	for i, tb := range ts {
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(q.field)
		b.WriteByte(':')
		b.WriteString(tb.text)
		if tb.boost != 1.0 {
			b.WriteByte('^')
			b.WriteString(strconv.FormatFloat(float64(tb.boost), 'f', -1, 32))
		}
	}
	b.WriteByte(')')
	return b.String()
}

// Clone creates an independent copy of this query with its own term slice.
func (q *SynonymQuery) Clone() Query {
	ts := *q.terms
	cp := make([]termAndBoost, len(ts))
	copy(cp, ts)
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		terms:     &cp,
	}
}

// Rewrite rewrites the query to a simpler form.
func (q *SynonymQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SynonymQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return &SynonymWeight{BaseWeight: NewBaseWeight(q)}, nil
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

// Explain returns an explanation of the score for the given document. It drives
// the explanation off the same Scorer the search path uses (scorerMatch) so the
// explained value equals the scored value. Note that SynonymWeight.Scorer is
// still a placeholder in this port (returns nil), so this currently reports
// no-match for every document; it will produce real match explanations once a
// SynonymScorer is implemented (see the synonym-scoring follow-up task).
func (w *SynonymWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	matched, score, err := scorerMatch(w, context, doc)
	if err != nil {
		return nil, err
	}
	if matched {
		return MatchExplanation(score, fmt.Sprintf("weight(%v in doc %d)", w.GetQuery(), doc)), nil
	}
	return NoMatchExplanation(fmt.Sprintf("no matching terms for %v in doc %d", w.GetQuery(), doc)), nil
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

// Ensure SynonymWeight implements Weight.
var _ Weight = (*SynonymWeight)(nil)

// Ensure SynonymQuery implements Query.
var _ Query = (*SynonymQuery)(nil)
