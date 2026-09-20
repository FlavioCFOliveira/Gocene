// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BlendedTermQuery blends index statistics across multiple terms.
// This is particularly useful when several terms should produce identical scores,
// regardless of their index statistics.
type BlendedTermQuery struct {
	BaseQuery
	terms         []*index.Term
	boosts        []float32
	contexts      []*index.TermStates
	rewriteMethod BlendedTermQueryRewriteMethod
}

// BlendedTermQueryBuilder is a builder for BlendedTermQuery.
type BlendedTermQueryBuilder struct {
	numTerms      int
	terms         []*index.Term
	boosts        []float32
	contexts      []*index.TermStates
	rewriteMethod BlendedTermQueryRewriteMethod
}

// NewBlendedTermQueryBuilder creates a new builder for BlendedTermQuery.
func NewBlendedTermQueryBuilder() *BlendedTermQueryBuilder {
	return &BlendedTermQueryBuilder{
		rewriteMethod: DISJUNCTION_MAX_REWRITE,
	}
}

// SetRewriteMethod sets the rewrite method. Default is DISJUNCTION_MAX_REWRITE.
func (b *BlendedTermQueryBuilder) SetRewriteMethod(m BlendedTermQueryRewriteMethod) *BlendedTermQueryBuilder {
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

// BlendedTermQueryRewriteMethod defines how queries for individual terms should
// be merged.
//
// Mirrors the nested class BlendedTermQuery.RewriteMethod (Lucene 10.5.0). It is
// a different contract from MultiTermQuery.RewriteMethod — which Go's flat
// package namespace would otherwise collide with — so it carries its enclosing
// class in the name.
type BlendedTermQueryRewriteMethod interface {
	Rewrite(subQueries []Query) Query
}

// BooleanRewrite merges sub queries into a BooleanQuery.
type BooleanRewrite struct{}

func (r *BooleanRewrite) Rewrite(subQueries []Query) Query {
	bq := NewBooleanQueryBuilder()
	for _, q := range subQueries {
		bq.Add(q, SHOULD)
	}
	return bq.Build()
}

var BOOLEAN_REWRITE = &BooleanRewrite{}

// DisjunctionMaxRewrite merges sub queries into a DisjunctionMaxQuery.
type DisjunctionMaxRewrite struct {
	tieBreakerMultiplier float32
}

func (r *DisjunctionMaxRewrite) Rewrite(subQueries []Query) Query {
	return NewDisjunctionMaxQuery(subQueries, r.tieBreakerMultiplier)
}

// Equals mirrors DisjunctionMaxRewrite.equals(Object), which takes a bare
// Object rather than a Query.
func (r *DisjunctionMaxRewrite) Equals(other any) bool {
	if o, ok := other.(*DisjunctionMaxRewrite); ok {
		return r.tieBreakerMultiplier == o.tieBreakerMultiplier
	}
	return false
}

var DISJUNCTION_MAX_REWRITE = &DisjunctionMaxRewrite{tieBreakerMultiplier: 0.01}

// NewBlendedTermQuery constructs a new BlendedTermQuery.
func NewBlendedTermQuery(terms []*index.Term, boosts []float32, contexts []*index.TermStates, rewriteMethod BlendedTermQueryRewriteMethod) *BlendedTermQuery {
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
			ctx, err := index.BuildTermStates(searcher, q.terms[i], true)
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
		adjusted, err := adjustFrequencies(searcher.GetTopReaderContext(), contexts[i], df, ttf)
		if err != nil {
			return nil, err
		}
		contexts[i] = adjusted
	}

	termQueries := make([]Query, len(q.terms))
	for i := 0; i < len(q.terms); i++ {
		termQueries[i] = NewTermQueryWithStates(q.terms[i], contexts[i])
		if q.boosts[i] != 1.0 {
			termQueries[i] = NewBoostQuery(termQueries[i], q.boosts[i])
		}
	}

	return q.rewriteMethod.Rewrite(termQueries), nil
}

// adjustFrequencies rebuilds ctx over the same leaves with artificial
// statistics, leaving the original TermStates untouched.
//
// Mirrors the private static
// BlendedTermQuery.adjustFrequencies(IndexReaderContext, TermStates, int, long).
func adjustFrequencies(readerContext index.IndexReaderContext, ctx *index.TermStates, artificialDf int, artificialTtf int64) (*index.TermStates, error) {
	leaves, err := readerContext.Leaves()
	if err != nil {
		return nil, err
	}
	newCtx, err := index.NewTermStatesForContext(readerContext)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(leaves); i++ {
		supplier, err := ctx.Get(leaves[i])
		if err != nil {
			return nil, err
		}
		if supplier == nil {
			continue
		}
		termState, err := supplier()
		if err != nil {
			return nil, err
		}
		if termState == nil {
			continue
		}
		newCtx.RegisterState(i, termState)
	}
	newCtx.AccumulateStatistics(artificialDf, artificialTtf)
	return newCtx, nil
}

// Equals checks if this query equals another.
func (q *BlendedTermQuery) Equals(other spi.Query) bool {
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

// ToString mirrors BlendedTermQuery.toString(String).
func (q *BlendedTermQuery) ToString(field string) string {
	var sb strings.Builder
	sb.WriteString("Blended(")
	for i := range q.terms {
		if i != 0 {
			sb.WriteString(" ")
		}
		var termQuery Query = NewTermQuery(q.terms[i])
		if q.boosts[i] != 1.0 {
			termQuery = NewBoostQuery(termQuery, q.boosts[i])
		}
		sb.WriteString(queryToString(termQuery, field))
	}
	sb.WriteString(")")
	return sb.String()
}

// String renders Query.toString(), the no-argument form that delegates to
// toString(String) with the empty default field.
func (q *BlendedTermQuery) String() string {
	return q.ToString("")
}

// Visit visits the terms in this query.
func (q *BlendedTermQuery) Visit(visitor QueryVisitor) {
	var termsToVisit []*index.Term
	for _, t := range q.terms {
		if visitor.AcceptField(t.Field) {
			termsToVisit = append(termsToVisit, t)
		}
	}
	if len(termsToVisit) > 0 {
		v := visitor.GetSubVisitor(SHOULD, q)
		v.ConsumeTerms(q, termsToVisit...)
	}
}

var _ Query = (*BlendedTermQuery)(nil)
