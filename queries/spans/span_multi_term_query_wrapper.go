// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/queries/src/java/org/apache/lucene/queries/spans/SpanMultiTermQueryWrapper.java

package spans

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashSpanMultiTermQueryWrapper seeds
// SpanMultiTermQueryWrapper.hashCode() in place of Java's classHash().
const classHashSpanMultiTermQueryWrapper = 0x536d_5477 // "SmTw"

// ErrSpanMultiTermRewriteFirst is returned by
// SpanMultiTermQueryWrapper.CreateWeight / CreateSpanWeight, reproducing
//
//	throw new IllegalArgumentException("Rewrite first!");
var ErrSpanMultiTermRewriteFirst = errors.New("Rewrite first!")

// ─── SpanRewriteMethod ──────────────────────────────────────────────────────

// SpanRewriteMethod defines how the wrapped MultiTermQuery is rewritten.
//
// Mirrors the nested class
// SpanMultiTermQueryWrapper.SpanRewriteMethod, declared as
//
//	public abstract static class SpanRewriteMethod extends MultiTermQuery.RewriteMethod {
//	  @Override
//	  public abstract SpanQuery rewrite(IndexSearcher indexSearcher, MultiTermQuery query);
//	}
//
// Deviation from Java: the single Java member is a covariant override of
// MultiTermQuery.RewriteMethod#rewrite, narrowing its Query return to
// SpanQuery. Go has no covariant returns, so the member is split in two, the
// way [SpanQuery] already splits createWeight: Rewrite (inherited from
// [search.RewriteMethod]) carries the search.Query return, and SpanRewrite
// carries the covariant SpanQuery one. Both take the same arguments as Java's
// rewrite, and every implementation in this package answers both from one body.
type SpanRewriteMethod interface {
	search.RewriteMethod

	// SpanRewrite carries the covariant SpanQuery return of
	// SpanRewriteMethod#rewrite(IndexSearcher, MultiTermQuery).
	SpanRewrite(searcher *search.IndexSearcher, query *search.MultiTermQuery) (SpanQuery, error)
}

// ─── SpanMultiTermQueryWrapper ──────────────────────────────────────────────

// SpanMultiTermQueryWrapper wraps any MultiTermQuery as a SpanQuery, so it can
// be nested within other SpanQuery classes.
//
// The query is rewritten by default to a SpanOrQuery containing the expanded
// terms, but this can be customized through SetRewriteMethod.
//
// Mirrors org.apache.lucene.queries.spans.SpanMultiTermQueryWrapper<Q extends
// MultiTermQuery> (Lucene 10.5.0).
//
// Deviation from Java: the Java class is generic in the concrete MultiTermQuery
// subclass Q, which it uses only to type getWrappedQuery()'s return and the
// protected `query` field. Gocene's MultiTermQuery subclasses embed
// [search.MultiTermQuery] and install themselves as its owner, so the embedded
// base already dispatches getField / toString / visit / hashCode / equals to
// the concrete query; the wrapper therefore holds that base, exactly as
// [search.NewMultiTermQueryConstantScoreWrapper] does.
type SpanMultiTermQueryWrapper struct {
	search.BaseQuery

	// query mirrors {@code protected final Q query}.
	query *search.MultiTermQuery
	// rewriteMethod mirrors {@code private SpanRewriteMethod rewriteMethod}.
	rewriteMethod SpanRewriteMethod
}

// NewSpanMultiTermQueryWrapper creates a new SpanMultiTermQueryWrapper around
// the query to wrap.
//
// Mirrors {@code public SpanMultiTermQueryWrapper(Q query)}:
//
//	this.query = Objects.requireNonNull(query);
//	this.rewriteMethod = selectRewriteMethod(query);
func NewSpanMultiTermQueryWrapper(query *search.MultiTermQuery) *SpanMultiTermQueryWrapper {
	if query == nil {
		panic("query must not be null")
	}
	return &SpanMultiTermQueryWrapper{
		query:         query,
		rewriteMethod: selectRewriteMethod(query),
	}
}

// selectRewriteMethod reproduces
//
//	MultiTermQuery.RewriteMethod method = query.getRewriteMethod();
//	if (method instanceof TopTermsRewrite) {
//	  final int pqsize = ((TopTermsRewrite<?>) method).getSize();
//	  return new TopTermsSpanBooleanQueryRewrite(pqsize);
//	} else {
//	  return SCORING_SPAN_QUERY_REWRITE;
//	}
//
// The instanceof test goes through [search.AsTopTermsRewrite], which is how a
// generic Java class is recognised from Go.
func selectRewriteMethod(query *search.MultiTermQuery) SpanRewriteMethod {
	method := query.GetRewriteMethod()
	if pqsize, ok := search.AsTopTermsRewrite(method); ok {
		return NewTopTermsSpanBooleanQueryRewrite(pqsize)
	}
	return ScoringSpanQueryRewrite
}

// GetRewriteMethod is expert: it returns the rewriteMethod.
//
// Mirrors {@code public final SpanRewriteMethod getRewriteMethod()}.
func (w *SpanMultiTermQueryWrapper) GetRewriteMethod() SpanRewriteMethod {
	return w.rewriteMethod
}

// SetRewriteMethod is expert: it sets the rewrite method. This only makes
// sense to be a span rewrite method.
//
// Mirrors {@code public final void setRewriteMethod(SpanRewriteMethod rewriteMethod)}.
func (w *SpanMultiTermQueryWrapper) SetRewriteMethod(rewriteMethod SpanRewriteMethod) {
	w.rewriteMethod = rewriteMethod
}

// GetField returns the name of the field matched by this query.
//
// Mirrors {@code public String getField() { return query.getField(); }}.
func (w *SpanMultiTermQueryWrapper) GetField() string {
	return w.query.GetField()
}

// CreateWeight reproduces
//
//	throw new IllegalArgumentException("Rewrite first!");
func (w *SpanMultiTermQueryWrapper) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return nil, ErrSpanMultiTermRewriteFirst
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanMultiTermQueryWrapper.createWeight, whose whole body is
//
//	throw new IllegalArgumentException("Rewrite first!");
func (w *SpanMultiTermQueryWrapper) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	return nil, ErrSpanMultiTermRewriteFirst
}

// GetWrappedQuery returns the wrapped query.
//
// Mirrors {@code public Query getWrappedQuery() { return query; }}.
func (w *SpanMultiTermQueryWrapper) GetWrappedQuery() search.Query {
	return w.query
}

// ToString reproduces
//
//	StringBuilder builder = new StringBuilder();
//	builder.append("SpanMultiTermQueryWrapper(");
//	String queryStr = query.toString(field);
//	builder.append(queryStr);
//	builder.append(")");
//	return builder.toString();
func (w *SpanMultiTermQueryWrapper) ToString(field string) string {
	return "SpanMultiTermQueryWrapper(" + w.query.String(field) + ")"
}

// String renders Query.toString(), whose Java body is toString("").
func (w *SpanMultiTermQueryWrapper) String() string { return w.ToString("") }

// Rewrite reproduces
//
//	return rewriteMethod.rewrite(indexSearcher, query);
func (w *SpanMultiTermQueryWrapper) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	return w.rewriteMethod.Rewrite(indexSearcher, w.query)
}

// Visit reproduces
//
//	if (visitor.acceptField(query.getField())) {
//	  query.visit(visitor.getSubVisitor(Occur.MUST, this));
//	}
func (w *SpanMultiTermQueryWrapper) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(w.query.GetField()) {
		w.query.Visit(visitor.GetSubVisitor(search.MUST, w))
	}
}

// HashCode reproduces {@code return classHash() * 31 + query.hashCode();}.
func (w *SpanMultiTermQueryWrapper) HashCode() int {
	return classHashSpanMultiTermQueryWrapper*31 + w.query.HashCode()
}

// Equals reproduces
//
//	return sameClassAs(other)
//	    && query.equals(((SpanMultiTermQueryWrapper<?>) other).query);
func (w *SpanMultiTermQueryWrapper) Equals(other spi.Query) bool {
	o, ok := other.(*SpanMultiTermQueryWrapper)
	if !ok {
		return false
	}
	return w.query.Equals(o.query)
}

var _ SpanQuery = (*SpanMultiTermQueryWrapper)(nil)

// ─── SCORING_SPAN_QUERY_REWRITE ─────────────────────────────────────────────

// scoringSpanQueryRewriteDelegate is the anonymous
// {@code new ScoringRewrite<List<SpanQuery>>() { ... }} that
// SCORING_SPAN_QUERY_REWRITE holds as its delegate.
//
// The Java builder type is List<SpanQuery>, a mutable reference that addClause
// appends to and build() drains; the Go rendering of that reference is
// *[]SpanQuery.
type scoringSpanQueryRewriteDelegate struct{}

// GetTopLevelBuilder reproduces {@code return new ArrayList<SpanQuery>();}.
func (d *scoringSpanQueryRewriteDelegate) GetTopLevelBuilder() (*[]SpanQuery, error) {
	builder := make([]SpanQuery, 0)
	return &builder, nil
}

// Build reproduces
//
//	return new SpanOrQuery(builder.toArray(SpanQuery[]::new));
func (d *scoringSpanQueryRewriteDelegate) Build(builder *[]SpanQuery) (search.Query, error) {
	return NewSpanOrQuery(*builder...)
}

// CheckMaxClauseCount reproduces the empty body
//
//	// we accept all terms as SpanOrQuery has no limits
func (d *scoringSpanQueryRewriteDelegate) CheckMaxClauseCount(count int) error {
	// we accept all terms as SpanOrQuery has no limits
	return nil
}

// AddClause reproduces
//
//	final SpanTermQuery q = new SpanTermQuery(term, states);
//	topLevel.add(q);
func (d *scoringSpanQueryRewriteDelegate) AddClause(
	topLevel *[]SpanQuery,
	term *index.Term,
	docCount int,
	boost float32,
	states *index.TermStates,
) error {
	q := NewSpanTermQueryWithTermStates(term, states)
	*topLevel = append(*topLevel, q)
	return nil
}

// scoringSpanQueryRewrite is the anonymous
// {@code new SpanRewriteMethod() { ... }} assigned to
// SCORING_SPAN_QUERY_REWRITE.
type scoringSpanQueryRewrite struct {
	// delegate mirrors
	// {@code private final ScoringRewrite<List<SpanQuery>> delegate}.
	delegate *search.ScoringRewrite[*[]SpanQuery]
}

// Rewrite carries the search.Query return of
// MultiTermQuery.RewriteMethod#rewrite; Java has the one covariant member and
// reaches this spelling through the supertype.
func (m *scoringSpanQueryRewrite) Rewrite(searcher *search.IndexSearcher, query *search.MultiTermQuery) (search.Query, error) {
	return m.SpanRewrite(searcher, query)
}

// SpanRewrite reproduces
//
//	return (SpanQuery) delegate.rewrite(indexSearcher, query);
func (m *scoringSpanQueryRewrite) SpanRewrite(searcher *search.IndexSearcher, query *search.MultiTermQuery) (SpanQuery, error) {
	q, err := m.delegate.Rewrite(searcher, query)
	if err != nil {
		return nil, err
	}
	sq, ok := q.(SpanQuery)
	if !ok {
		// Java's cast, which the delegate's build() makes unreachable: it
		// always returns a SpanOrQuery.
		return nil, fmt.Errorf("SCORING_SPAN_QUERY_REWRITE: delegate returned %T, not a SpanQuery", q)
	}
	return sq, nil
}

// ScoringSpanQueryRewrite is a rewrite method that first translates each term
// into a SpanTermQuery in a BooleanClause.Occur#SHOULD clause in a
// BooleanQuery, and keeps the scores as computed by the query.
//
// Mirrors
// {@code public static final SpanRewriteMethod SCORING_SPAN_QUERY_REWRITE}
// (Lucene 10.5.0).
var ScoringSpanQueryRewrite SpanRewriteMethod = &scoringSpanQueryRewrite{
	delegate: search.NewScoringRewrite[*[]SpanQuery](&scoringSpanQueryRewriteDelegate{}),
}

// ─── TopTermsSpanBooleanQueryRewrite ────────────────────────────────────────

// topTermsSpanBooleanQueryRewriteDelegate is the anonymous
// {@code new TopTermsRewrite<List<SpanQuery>>(size) { ... }} that
// TopTermsSpanBooleanQueryRewrite's constructor instantiates. Its four members
// belong to that anonymous subclass, not to the enclosing rewrite method, so
// they live on their own unexported type rather than on
// [TopTermsSpanBooleanQueryRewrite].
type topTermsSpanBooleanQueryRewriteDelegate struct{}

// GetMaxSize reproduces {@code return Integer.MAX_VALUE;}.
func (d *topTermsSpanBooleanQueryRewriteDelegate) GetMaxSize() int {
	return math.MaxInt32
}

// GetTopLevelBuilder reproduces {@code return new ArrayList<SpanQuery>();}.
func (d *topTermsSpanBooleanQueryRewriteDelegate) GetTopLevelBuilder() (*[]SpanQuery, error) {
	builder := make([]SpanQuery, 0)
	return &builder, nil
}

// Build reproduces
//
//	return new SpanOrQuery(builder.toArray(SpanQuery[]::new));
func (d *topTermsSpanBooleanQueryRewriteDelegate) Build(builder *[]SpanQuery) (search.Query, error) {
	return NewSpanOrQuery(*builder...)
}

// AddClause reproduces
//
//	final SpanTermQuery q = new SpanTermQuery(term, states);
//	topLevel.add(q);
func (d *topTermsSpanBooleanQueryRewriteDelegate) AddClause(
	topLevel *[]SpanQuery,
	term *index.Term,
	docFreq int,
	boost float32,
	states *index.TermStates,
) error {
	q := NewSpanTermQueryWithTermStates(term, states)
	*topLevel = append(*topLevel, q)
	return nil
}

// TopTermsSpanBooleanQueryRewrite is a rewrite method that first translates
// each term into a SpanTermQuery in a BooleanClause.Occur#SHOULD clause in a
// BooleanQuery, and keeps the scores as computed by the query.
//
// This rewrite method only uses the top scoring terms so it will not overflow
// the boolean max clause count.
//
// Mirrors the nested class
// {@code public static final class TopTermsSpanBooleanQueryRewrite extends
// SpanRewriteMethod} (Lucene 10.5.0).
type TopTermsSpanBooleanQueryRewrite struct {
	// delegate mirrors
	// {@code private final TopTermsRewrite<List<SpanQuery>> delegate}.
	delegate *search.TopTermsRewrite[*[]SpanQuery]
}

// NewTopTermsSpanBooleanQueryRewrite creates a
// TopTermsSpanBooleanQueryRewrite for at most size terms.
//
// Mirrors {@code public TopTermsSpanBooleanQueryRewrite(int size)}.
func NewTopTermsSpanBooleanQueryRewrite(size int) *TopTermsSpanBooleanQueryRewrite {
	return &TopTermsSpanBooleanQueryRewrite{
		delegate: search.NewTopTermsRewrite[*[]SpanQuery](
			size, &topTermsSpanBooleanQueryRewriteDelegate{}),
	}
}

// GetSize returns the maximum priority queue size.
//
// Mirrors {@code public int getSize() { return delegate.getSize(); }}.
func (r *TopTermsSpanBooleanQueryRewrite) GetSize() int {
	return r.delegate.GetSize()
}

// Rewrite carries the search.Query return of
// MultiTermQuery.RewriteMethod#rewrite.
func (r *TopTermsSpanBooleanQueryRewrite) Rewrite(searcher *search.IndexSearcher, query *search.MultiTermQuery) (search.Query, error) {
	return r.SpanRewrite(searcher, query)
}

// SpanRewrite reproduces
//
//	return (SpanQuery) delegate.rewrite(indexSearcher, query);
func (r *TopTermsSpanBooleanQueryRewrite) SpanRewrite(searcher *search.IndexSearcher, query *search.MultiTermQuery) (SpanQuery, error) {
	q, err := r.delegate.Rewrite(searcher, query)
	if err != nil {
		return nil, err
	}
	sq, ok := q.(SpanQuery)
	if !ok {
		// Java's cast, which the delegate's build() makes unreachable: it
		// always returns a SpanOrQuery.
		return nil, fmt.Errorf("TopTermsSpanBooleanQueryRewrite: delegate returned %T, not a SpanQuery", q)
	}
	return sq, nil
}

// HashCode reproduces {@code return 31 * delegate.hashCode();}.
func (r *TopTermsSpanBooleanQueryRewrite) HashCode() int {
	return 31 * r.delegate.HashCode()
}

// Equals reproduces
//
//	if (this == obj) return true;
//	if (obj == null) return false;
//	if (getClass() != obj.getClass()) return false;
//	final TopTermsSpanBooleanQueryRewrite other = (TopTermsSpanBooleanQueryRewrite) obj;
//	return delegate.equals(other.delegate);
//
// Both delegates are instances of the same anonymous TopTermsRewrite subclass
// created by the constructor above, so TopTermsRewrite#equals — getClass()
// identity plus size equality — reduces to comparing the two sizes.
func (r *TopTermsSpanBooleanQueryRewrite) Equals(obj search.RewriteMethod) bool {
	if obj == nil {
		return false
	}
	if search.RewriteMethod(r) == obj {
		return true
	}
	other, ok := obj.(*TopTermsSpanBooleanQueryRewrite)
	if !ok {
		return false
	}
	return r.delegate.GetSize() == other.delegate.GetSize()
}

var (
	_ SpanRewriteMethod = (*scoringSpanQueryRewrite)(nil)
	_ SpanRewriteMethod = (*TopTermsSpanBooleanQueryRewrite)(nil)
)
