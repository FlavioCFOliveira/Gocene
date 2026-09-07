// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlendedTermQuery blends index statistics across multiple terms.
// This is particularly useful when several terms should produce identical scores,
// regardless of their index statistics.
type BlendedTermQuery struct {
	BaseQuery
	terms          []*index.Term
	boosts         []float32
	contexts       []*index.TermStates
	rewriteMethod  RewriteMethod
}

// BlendedTermQueryBuilder is a builder for BlendedTermQuery.
type BlendedTermQueryBuilder struct {
	numTerms      int
	terms         []*index.Term
	boosts        []float32
	contexts      []*index.TermStates
	rewriteMethod RewriteMethod
}

// NewBlendedTermQueryBuilder creates a new builder for BlendedTermQuery.
func NewBlendedTermQueryBuilder() *BlendedTermQueryBuilder {
	return &BlendedTermQueryBuilder{
		rewriteMethod: DISJUNCTION_MAX_REWRITE,
	}
}

// SetRewriteMethod sets the rewrite method. Default is DISJUNCTION_MAX_REWRITE.
func (b *BlendedTermQueryBuilder) SetRewriteMethod(m RewriteMethod) *BlendedTermQueryBuilder {
	b.rewriteMethod = m
	return b
}

// Add adds a new term to the builder with a default boost of 1.0.
func (b *BlendedTermQueryBuilder) Add(term *index.Term) *BlendedTermQueryBuilder {
	return b.AddWithBoost(term, 1.0)
}

// AddWithBoost adds a term with the provided boost.
func (b *BlendedTermQueryBuilder) AddWithBoost(term *index.Term, boost float32) *BlendedTermQueryBuilder {
	return b.AddWithContext(term, boost, nil)
}

// AddWithContext adds a term with the provided boost and context.
func (b *BlendedTermQueryBuilder) AddWithContext(term *index.Term, boost float32, context *index.TermStates) *BlendedTermQueryBuilder {
	if b.numTerms >= GetMaxClauseCount() {
		panic("too many clauses")
	}
	b.terms = append(b.terms, term)
	b.boosts = append(b.boosts, boost)
	b.contexts = append(b.contexts, context)
	b.numTerms++
	return b
}

// Build constructs the BlendedTermQuery.
func (b *BlendedTermQueryBuilder) Build() *BlendedTermQuery {
	return NewBlendedTermQuery(b.terms, b.boosts, b.contexts, b.rewriteMethod)
}

// RewriteMethod defines how queries for individual terms should be merged.
type RewriteMethod interface {
	Rewrite(subQueries []Query) Query
}

// BooleanRewrite merges sub queries into a BooleanQuery.
type BooleanRewrite struct{}

func (r *BooleanRewrite) Rewrite(subQueries []Query) Query {
	bq := NewBooleanQuery()
	for _, q := range subQueries {
		bq.Add(q, SHOULD)
	}
	return bq
}

var BOOLEAN_REWRITE = &BooleanRewrite{}

// DisjunctionMaxRewrite merges sub queries into a DisjunctionMaxQuery.
type DisjunctionMaxRewrite struct {
	tieBreakerMultiplier float32
}

func (r *DisjunctionMaxRewrite) Rewrite(subQueries []Query) Query {
	return NewDisjunctionMaxQuery(subQueries, r.tieBreakerMultiplier)
}

func (r *DisjunctionMaxRewrite) Equal(other Query) bool {
	if o, ok := other.(*DisjunctionMaxRewrite); ok {
		return r.tieBreakerMultiplier == o.tieBreakerMultiplier
	}
	return false
}

var DISJUNCTION_MAX_REWRITE = &DisjunctionMaxRewrite{tieBreakerMultiplier: 0.01}

// NewBlendedTermQuery constructs a new BlendedTermQuery.
func NewBlendedTermQuery(terms []*index.Term, boosts []float32, contexts []*index.TermStates, rewriteMethod RewriteMethod) *BlendedTermQuery {
	// Sort terms to ensure Equals/HashCode consistency.
	type termEntry struct {
		term    *index.Term
		boost   float32
		context *index.TermStates
	}
	entries := make([]termEntry, len(terms))
	for i := range terms {
		entries[i] = termEntry{terms[i], boosts[i], contexts[i]}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].term.CompareTo(entries[j].term) < 0
	})

	sortedTerms := make([]*index.Term, len(terms))
	sortedBoosts := make([]float32, len(boosts))
	sortedContexts := make([]*index.TermStates, len(contexts))
	for i, e := range entries {
		sortedTerms[i] = e.term
		sortedBoosts[i] = e.boost
		sortedContexts[i] = e.context
	}

	return &BlendedTermQuery{
		terms:         sortedTerms,
		boosts:        sortedBoosts,
		contexts:      sortedContexts,
		rewriteMethod: rewriteMethod,
	}
}

// Rewrite rewrites this query by blending index statistics.
func (q *BlendedTermQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	contexts := make([]*index.TermStates, len(q.contexts))
	copy(contexts, q.contexts)

	for i := 0; i < len(contexts); i++ {
		if contexts[i] == nil || !contexts[i].WasBuiltFor(searcher.GetTopReaderContext()) {
			ctx, err := buildTermStates(searcher, q.terms[i])
			if err != nil {
				return nil, err
			}
			contexts[i] = ctx
		}
	}

	// Compute aggregated doc freq and total term freq.
	df := 0
	var ttf int64
	for _, ctx := range contexts {
		df = max(df, ctx.DocFreq())
		ttf += ctx.TotalTermFreq()
	}

	for i := 0; i < len(contexts); i++ {
		contexts[i].AccumulateStatistics(df, ttf)
	}

	termQueries := make([]Query, len(q.terms))
	for i := 0; i < len(q.terms); i++ {
		tq := NewTermQuery(q.terms[i], contexts[i])
		if q.boosts[i] != 1.0 {
			tq = NewBoostQuery(tq, q.boosts[i])
		}
		termQueries[i] = tq
	}

	return q.rewriteMethod.Rewrite(termQueries), nil
}

func buildTermStates(searcher *IndexSearcher, term *index.Term) (*index.TermStates, error) {
	ctx := searcher.GetTopReaderContext()
	leaves, err := ctx.Leaves()
	if err != nil {
		return nil, err
	}

	ts := index.NewTermStates(ctx.ID().(*struct{}), len(leaves))
	for i, leaf := range leaves {
		terms := leaf.Reader().Terms(term.Field)
		if terms == nil {
			continue
		}
		// Seek to the term.
		if err := terms.Seek(term); err != nil {
			continue
		}
		if terms.Next() == nil || !terms.GetTerm().Equals(term) {
			continue
		}
		// Get TermState.
		state := terms.GetState()
		if state == nil {
			continue
		}
		ts.Register(i, state, terms.DocFreq(), terms.TotalTermFreq())
	}
	return ts, nil
}

// Equals checks if this query equals another.
func (q *BlendedTermQuery) Equals(other Query) bool {
	if o, ok := other.(*BlendedTermQuery); ok {
		if len(q.terms) != len(o.terms) {
			return false
		}
		for i := range q.terms {
			if !q.terms[i].Equals(o.terms[i]) || q.boosts[i] != o.boosts[i] {
				return false
			}
		}
		// Contexts are implementation details, not part of equality.
		return q.rewriteMethod == o.rewriteMethod
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *BlendedTermQuery) HashCode() int {
	h := 17
	for i := range q.terms {
		h = 31*h + q.terms[i].HashCode()
		h = 31*h + int(q.boosts[i])
	}
	h = 31*h + 0 // rewriteMethod hash
	return h
}

// String returns a string representation of the query.
func (q *BlendedTermQuery) String() string {
	var sb strings.Builder
	sb.WriteString("Blended(")
	for i := range q.terms {
		if i != 0 {
			sb.WriteString(" ")
		}
		tq := NewTermQuery(q.terms[i])
		if q.boosts[i] != 1.0 {
			tq = NewBoostQuery(tq, q.boosts[i])
		}
		sb.WriteString(tq.String())
	}
	sb.WriteString(")")
	return sb.String()
}

// Visit visits the terms in this query.
func (q *BlendedTermQuery) Visit(visitor QueryVisitor) {
	var termsToVisit []*index.Term
	for _, t := range q.terms {
		if visitor.AcceptField(t.Field()) {
			termsToVisit = append(termsToVisit, t)
		}
	}
	if len(termsToVisit) > 0 {
		v := visitor.GetSubVisitor(SHOULD, q)
		v.ConsumeTerms(q, termsToVisit...)
	}
}

var _ Query = (*BlendedTermQuery)(nil)
