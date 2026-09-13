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

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/MultiTermQuery.java

import (
	"errors"
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── RewriteMethod ──────────────────────────────────────────────────────────

// RewriteMethod defines how a MultiTermQuery is rewritten.
//
// Mirrors the abstract nested class
// org.apache.lucene.search.MultiTermQuery.RewriteMethod, whose sole abstract
// member is
//
//	public abstract Query rewrite(IndexSearcher indexSearcher, MultiTermQuery query)
//
// The class also carries one concrete member,
//
//	protected TermsEnum getTermsEnum(MultiTermQuery query, Terms terms, AttributeSource atts)
//
// whose body is {@code query.getTermsEnum(terms, atts)}. Go interfaces cannot
// carry an implementation, so that member is rendered as the package function
// [rewriteMethodGetTermsEnum], which honours an override exactly as Java's
// dynamic dispatch does. No subclass in Apache Lucene 10.5.0 overrides it.
type RewriteMethod interface {
	// Rewrite rewrites query into a primitive query.
	Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error)
}

// rewriteMethodTermsEnumProvider is the optional interface a RewriteMethod
// satisfies when it overrides
// MultiTermQuery.RewriteMethod#getTermsEnum(MultiTermQuery, Terms,
// AttributeSource).
type rewriteMethodTermsEnumProvider interface {
	GetTermsEnum(query *MultiTermQuery, terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error)
}

// rewriteMethodGetTermsEnum reproduces the body of
// MultiTermQuery.RewriteMethod#getTermsEnum(MultiTermQuery, Terms,
// AttributeSource) in Apache Lucene 10.5.0:
//
//	return query.getTermsEnum(terms, atts);
//
// It first honours a RewriteMethod that overrides the member, mirroring Java's
// virtual dispatch, and otherwise runs the base body. The comment Lucene
// leaves on that call — "allow RewriteMethod subclasses to pull a TermsEnum
// from the MTQ" — is exactly what this indirection preserves.
func rewriteMethodGetTermsEnum(method RewriteMethod, query *MultiTermQuery, terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	if provider, ok := method.(rewriteMethodTermsEnumProvider); ok {
		return provider.GetTermsEnum(query, terms, atts)
	}
	return query.GetTermsEnumWithAttributes(terms, atts)
}

// rewriteMethodHashCode renders java.lang.Object#hashCode() as inherited by
// MultiTermQuery.RewriteMethod. TopTermsRewrite and DocValuesRewriteMethod
// override it; the anonymous RewriteMethod constants do not, and Java then
// uses identity, which Go reproduces through the interface value's dynamic
// type and pointer.
func rewriteMethodHashCode(method RewriteMethod) int {
	if method == nil {
		return 0
	}
	switch h := method.(type) {
	case interface{ HashCode() int }:
		return h.HashCode()
	case interface{ HashCode() uint64 }:
		return int(h.HashCode())
	}
	v := reflect.ValueOf(method)
	if v.Kind() == reflect.Pointer {
		return int(v.Pointer())
	}
	return javaStringHashCode(reflect.TypeOf(method).String())
}

// rewriteMethodEquals renders java.lang.Object#equals(Object) as inherited by
// MultiTermQuery.RewriteMethod: identity for the anonymous constants, and the
// declared equality for the subclasses that override it (TopTermsRewrite,
// DocValuesRewriteMethod).
func rewriteMethodEquals(a, b RewriteMethod) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a == b {
		return true
	}
	if eq, ok := a.(interface{ Equals(RewriteMethod) bool }); ok {
		return eq.Equals(b)
	}
	if eq, ok := a.(interface{ Equals(any) bool }); ok {
		return eq.Equals(b)
	}
	return false
}

// ─── MultiTermQuery.RewriteMethod constants ─────────────────────────────────

// constantScoreBlendedRewriteMethod is the anonymous RewriteMethod assigned to
// MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE, whose rewrite body is
// {@code new MultiTermQueryConstantScoreBlendedWrapper<>(query)}.
type constantScoreBlendedRewriteMethod struct{}

// Rewrite reproduces CONSTANT_SCORE_BLENDED_REWRITE.rewrite.
func (m *constantScoreBlendedRewriteMethod) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	return newMultiTermQueryConstantScoreBlendedWrapper(query), nil
}

// ConstantScoreBlendedRewrite is a rewrite method where documents are assigned
// a constant score equal to the query's boost. It maintains a boolean
// query-like implementation over the most costly terms while pre-processing
// the less costly terms into a filter bitset, and enforces an upper limit on
// the number of terms allowed in the boolean query-like implementation.
//
// Mirrors MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE (Lucene 10.5.0).
var ConstantScoreBlendedRewrite RewriteMethod = &constantScoreBlendedRewriteMethod{}

// constantScoreRewriteMethod is the anonymous RewriteMethod assigned to
// MultiTermQuery.CONSTANT_SCORE_REWRITE, whose rewrite body is
// {@code new MultiTermQueryConstantScoreWrapper<>(query)}.
type constantScoreRewriteMethod struct{}

// Rewrite reproduces CONSTANT_SCORE_REWRITE.rewrite.
func (m *constantScoreRewriteMethod) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	return NewMultiTermQueryConstantScoreWrapper(query), nil
}

// ConstantScoreRewrite is a rewrite method that first creates a private
// Filter, by visiting each term in sequence and marking all docs for that
// term. Matching documents are assigned a constant score equal to the query's
// boost.
//
// Mirrors MultiTermQuery.CONSTANT_SCORE_REWRITE (Lucene 10.5.0).
var ConstantScoreRewrite RewriteMethod = &constantScoreRewriteMethod{}

// DocValuesRewrite is a rewrite method that uses SORTED / SORTED_SET doc
// values to find matching docs through a post-filtering type approach. All
// matching docs are assigned a constant score equal to the query's boost.
//
// Mirrors MultiTermQuery.DOC_VALUES_REWRITE, declared as
// {@code new DocValuesRewriteMethod()} (Lucene 10.5.0).
var DocValuesRewrite RewriteMethod = NewDocValuesRewriteMethod()

// ScoringBooleanRewrite is a rewrite method that first translates each term
// into a SHOULD clause in a BooleanQuery, and keeps the scores as computed by
// the query.
//
// Mirrors MultiTermQuery.SCORING_BOOLEAN_REWRITE, declared as
// {@code ScoringRewrite.SCORING_BOOLEAN_REWRITE} (Lucene 10.5.0).
var ScoringBooleanRewrite RewriteMethod = ScoringBooleanRewriteMethod

// ConstantScoreBooleanRewrite is like ScoringBooleanRewrite except scores are
// not computed: each matching document receives a constant score equal to the
// query's boost.
//
// Mirrors MultiTermQuery.CONSTANT_SCORE_BOOLEAN_REWRITE, declared as
// {@code ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE} (Lucene 10.5.0).
var ConstantScoreBooleanRewrite RewriteMethod = ConstantScoreBooleanRewriteMethod

// ─── MultiTermQuery ─────────────────────────────────────────────────────────

// MultiTermQueryOwner is the Go rendering of the dynamic dispatch that Java
// obtains for free from {@code this} inside the abstract MultiTermQuery.
//
// Java declares
//
//	protected abstract TermsEnum getTermsEnum(Terms terms, AttributeSource atts);
//
// and every call made through a MultiTermQuery reference lands in the concrete
// subclass body. Go embedding is not virtual: from the embedded
// [MultiTermQuery] value there is no way back to the struct that embeds it, so
// the concrete query installs itself with [MultiTermQuery.SetOwner] and the
// base forwards to it. This adds no behaviour Lucene does not have; it only
// restores the dispatch Java performs implicitly. The same indirection already
// carries BaseTermsEnum's defaults in index/base_terms_enum.go.
type MultiTermQueryOwner interface {
	// GetTermsEnumWithAttributes constructs the enumeration to be used,
	// expanding the pattern term. Mirrors the abstract
	// MultiTermQuery#getTermsEnum(Terms, AttributeSource).
	GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error)
}

// errMultiTermQueryNoOwner reports a MultiTermQuery whose concrete subclass
// never installed itself with SetOwner. It is the Go equivalent of trying to
// instantiate Java's abstract MultiTermQuery: the abstract
// getTermsEnum(Terms, AttributeSource) has no body to run.
var errMultiTermQueryNoOwner = errors.New(
	"MultiTermQuery.getTermsEnum(Terms, AttributeSource) is abstract: " +
		"the concrete subclass must install itself with SetOwner")

// MultiTermQuery is an abstract Query that matches documents containing a
// subset of terms provided by a FilteredTermsEnum enumeration.
//
// This query cannot be used directly; you must embed it and define
// GetTermsEnumWithAttributes to provide a FilteredTermsEnum that iterates
// through the terms to be matched, then install the concrete query with
// [MultiTermQuery.SetOwner].
//
// NOTE: if RewriteMethod is either ConstantScoreBooleanRewrite or
// ScoringBooleanRewrite, you may encounter ErrTooManyClauses during searching,
// which happens when the number of terms to be searched exceeds
// GetMaxClauseCount(). Setting RewriteMethod to ConstantScoreBlendedRewrite or
// ConstantScoreRewrite prevents this.
//
// The recommended rewrite method is ConstantScoreBlendedRewrite: it doesn't
// spend CPU computing unhelpful scores, and is the most performant rewrite
// method given the query. If you need scoring (like FuzzyQuery), use
// TopTermsScoringBooleanQueryRewrite which uses a priority queue to only
// collect competitive terms and not hit this limitation.
//
// Mirrors org.apache.lucene.search.MultiTermQuery (Lucene 10.5.0).
type MultiTermQuery struct {
	field         string
	rewriteMethod RewriteMethod
	// owner carries the concrete subclass; see [MultiTermQueryOwner].
	owner MultiTermQueryOwner
}

// NewMultiTermQuery constructs a query matching terms that cannot be
// represented with a single Term.
//
// Mirrors the protected constructor
// {@code MultiTermQuery(final String field, RewriteMethod rewriteMethod)},
// including both Objects.requireNonNull checks and their messages. Java's
// constructor is only reachable through {@code super(...)}; the Go equivalent
// is likewise only meaningful from a subclass constructor, which must follow
// it with [MultiTermQuery.SetOwner].
func NewMultiTermQuery(field string, rewriteMethod RewriteMethod) *MultiTermQuery {
	if field == "" && rewriteMethod == nil {
		// Fall through to the individual checks so the panic message names
		// the offending argument exactly as Java's does.
		_ = field
	}
	if rewriteMethod == nil {
		panic("rewriteMethod must not be null")
	}
	return &MultiTermQuery{
		field:         field,
		rewriteMethod: rewriteMethod,
	}
}

// SetOwner installs the concrete subclass instance as the receiver of the
// abstract MultiTermQuery#getTermsEnum(Terms, AttributeSource). See
// [MultiTermQueryOwner] for why this is needed.
//
// A subclass that embeds MultiTermQuery by value must call SetOwner on the
// final object, after every copy, otherwise the owner would point at a
// temporary.
func (q *MultiTermQuery) SetOwner(owner MultiTermQueryOwner) {
	q.owner = owner
}

// GetField returns the field name for this query.
//
// Mirrors {@code public final String getField()}.
func (q *MultiTermQuery) GetField() string {
	return q.field
}

// GetTermsEnumWithAttributes constructs the enumeration to be used, expanding
// the pattern term. This method should only be called if the field exists (ie,
// implementations can assume the field does exist). It never returns nil
// (implementations return index.EmptyTermsEnum if no terms match). The
// TermsEnum must already be positioned to the first matching term. The given
// AttributeSource is passed by the RewriteMethod to share information between
// segments, for example TopTermsRewrite uses it to share maximum competitive
// boosts.
//
// Mirrors the abstract
// {@code protected abstract TermsEnum getTermsEnum(Terms terms, AttributeSource atts)}.
// Java's two getTermsEnum overloads are distinguished by arity; Go has no
// overloading, so the AttributeSource form carries the parameter in its name.
func (q *MultiTermQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	if q.owner == nil {
		return nil, errMultiTermQueryNoOwner
	}
	return q.owner.GetTermsEnumWithAttributes(terms, atts)
}

// GetTermsEnum constructs an enumeration that expands the pattern term. This
// method should only be called if the field exists (ie, implementations can
// assume the field does exist). It never returns nil. The returned TermsEnum
// is positioned to the first matching term.
//
// Mirrors {@code public final TermsEnum getTermsEnum(Terms terms)}, whose body
// is {@code return getTermsEnum(terms, new AttributeSource());}.
func (q *MultiTermQuery) GetTermsEnum(terms index.Terms) (index.TermsEnum, error) {
	return q.GetTermsEnumWithAttributes(terms, util.NewAttributeSource())
}

// GetTermsCount returns the number of unique terms contained in this query, if
// known up-front. If not known, -1 is returned.
//
// Mirrors {@code public long getTermsCount() { return -1; }}.
//
// Java subclasses that know the count (TermInSetQuery) override this member,
// and a call through a MultiTermQuery reference reaches the override. A Go
// subclass shadows it instead, so the override is seen through the concrete
// type but not through the embedded base — unlike
// GetTermsEnumWithAttributes, which is routed through [MultiTermQueryOwner]
// because Lucene requires the abstract dispatch there.
func (q *MultiTermQuery) GetTermsCount() int64 {
	return -1
}

// Rewrite rewrites this query through its RewriteMethod.
//
// To rewrite to a simpler form, instead return a simpler enum from
// GetTermsEnumWithAttributes.
//
// Mirrors {@code public final Query rewrite(IndexSearcher indexSearcher)},
// whose body is {@code return rewriteMethod.rewrite(indexSearcher, this);}.
func (q *MultiTermQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q.rewriteMethod.Rewrite(searcher, q)
}

// GetRewriteMethod returns the rewrite method used to build the final query.
//
// Mirrors {@code public RewriteMethod getRewriteMethod()}.
func (q *MultiTermQuery) GetRewriteMethod() RewriteMethod {
	return q.rewriteMethod
}

// CreateWeight reproduces the inherited Query#createWeight, which
// MultiTermQuery does not override:
//
//	throw new UnsupportedOperationException("Query " + this + " does not implement createWeight");
//
// A MultiTermQuery is never a primitive query: it always rewrites first.
func (q *MultiTermQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return nil, fmt.Errorf("Query %s does not implement createWeight", q.String(""))
}

// String renders Java's abstract {@code Query.toString(String field)} as
// reached through a MultiTermQuery reference: the rendering belongs to the
// concrete subclass, so the call is routed through the installed owner. Java
// gets the same dispatch from {@code this}.
func (q *MultiTermQuery) String(field string) string {
	if q.owner == nil {
		return ""
	}
	if owner, ok := q.owner.(Query); ok {
		return queryToString(owner, field)
	}
	return ""
}

// Visit renders Java's abstract {@code Query.visit(QueryVisitor visitor)} as
// reached through a MultiTermQuery reference: the traversal belongs to the
// concrete subclass, so the call is routed through the installed owner.
func (q *MultiTermQuery) Visit(visitor QueryVisitor) {
	if q.owner == nil {
		return
	}
	if owner, ok := q.owner.(Query); ok {
		owner.Visit(visitor)
	}
}

// HashCode reproduces MultiTermQuery#hashCode():
//
//	final int prime = 31;
//	int result = classHash();
//	result = prime * result + rewriteMethod.hashCode();
//	result = prime * result + field.hashCode();
//	return result;
func (q *MultiTermQuery) HashCode() int {
	const prime = 31
	result := q.classHash()
	result = prime*result + rewriteMethodHashCode(q.rewriteMethod)
	result = prime*result + javaStringHashCode(q.field)
	return result
}

// Equals reproduces MultiTermQuery#equals(Object):
//
//	return sameClassAs(other) && equalsTo(getClass().cast(other));
//
// with equalsTo comparing the rewrite method and the field.
func (q *MultiTermQuery) Equals(other spi.Query) bool {
	if !q.sameClassAs(other) {
		return false
	}
	o, ok := other.(multiTermQueryFields)
	if !ok {
		return false
	}
	return rewriteMethodEquals(q.rewriteMethod, o.GetRewriteMethod()) && q.field == o.GetField()
}

// multiTermQueryFields is the Go stand-in for the private
// {@code equalsTo(MultiTermQuery other)} helper's access to the other
// instance's two fields. Every MultiTermQuery subclass carries both accessors
// by embedding.
type multiTermQueryFields interface {
	GetField() string
	GetRewriteMethod() RewriteMethod
}

// classType returns the reflect.Type standing in for Java's
// {@code getClass()} — the concrete subclass when one is installed.
func (q *MultiTermQuery) classType() reflect.Type {
	if q.owner != nil {
		return reflect.TypeOf(q.owner)
	}
	return reflect.TypeOf(q)
}

// sameClassAs reproduces {@code Query#sameClassAs(Object)}:
//
//	return other != null && getClass() == other.getClass();
//
// Java's parameter is Object; the Go rendering takes the shared Query contract
// [spi.Query], which is what Equals now receives.
func (q *MultiTermQuery) sameClassAs(other spi.Query) bool {
	if other == nil {
		return false
	}
	return q.classType() == reflect.TypeOf(other)
}

// classHash reproduces {@code Query#classHash()}, which returns
// {@code getClass().getName().hashCode()} — "a constant integer for a given
// class, derived from the name of the class". The Go rendering hashes the
// concrete type's name with java.lang.String#hashCode so the value is stable
// per type and distinct between types, which is the whole contract Lucene
// states for it.
func (q *MultiTermQuery) classHash() int {
	return javaStringHashCode(q.classType().String())
}

// javaStringHashCode reproduces java.lang.String#hashCode():
//
//	s[0]*31^(n-1) + s[1]*31^(n-2) + ... + s[n-1]
//
// evaluated over UTF-16 code units with 32-bit wraparound. Gocene strings are
// UTF-8; for the ASCII identifiers this is applied to, the two unit sequences
// coincide.
func javaStringHashCode(s string) int {
	var h int32
	for _, r := range s {
		if r <= 0xFFFF {
			h = 31*h + int32(r)
			continue
		}
		// Supplementary code point: Java iterates the two UTF-16 surrogates.
		r -= 0x10000
		hi := int32(0xD800 + (r >> 10))
		lo := int32(0xDC00 + (r & 0x3FF))
		h = 31*h + hi
		h = 31*h + lo
	}
	return int(h)
}

// ─── Nested rewrite methods ─────────────────────────────────────────────────

// TopTermsScoringBooleanQueryRewrite is a rewrite method that first translates
// each term into a SHOULD clause in a BooleanQuery, and keeps the scores as
// computed by the query.
//
// This rewrite method only uses the top scoring terms so it will not overflow
// the boolean max clause count.
//
// Mirrors the nested class
// MultiTermQuery.TopTermsScoringBooleanQueryRewrite, declared as
// {@code extends TopTermsRewrite<BooleanQuery.Builder>} (Lucene 10.5.0).
type TopTermsScoringBooleanQueryRewrite struct {
	*TopTermsRewrite[*BooleanQueryBuilder]
}

// NewTopTermsScoringBooleanQueryRewrite creates a
// TopTermsScoringBooleanQueryRewrite for at most size terms.
//
// NOTE: if GetMaxClauseCount() is smaller than size, then it will be used
// instead.
func NewTopTermsScoringBooleanQueryRewrite(size int) *TopTermsScoringBooleanQueryRewrite {
	r := &TopTermsScoringBooleanQueryRewrite{}
	r.TopTermsRewrite = newTopTermsRewrite[*BooleanQueryBuilder](size, r)
	return r
}

// GetMaxSize returns IndexSearcher.getMaxClauseCount().
func (r *TopTermsScoringBooleanQueryRewrite) GetMaxSize() int { return GetMaxClauseCount() }

// GetTopLevelBuilder returns {@code new BooleanQuery.Builder()}.
func (r *TopTermsScoringBooleanQueryRewrite) GetTopLevelBuilder() (*BooleanQueryBuilder, error) {
	return NewBooleanQueryBuilder(), nil
}

// Build returns {@code builder.build()}.
func (r *TopTermsScoringBooleanQueryRewrite) Build(builder *BooleanQueryBuilder) Query {
	return builder.Build()
}

// AddClause reproduces
//
//	final TermQuery tq = new TermQuery(term, states);
//	topLevel.add(new BoostQuery(tq, boost), BooleanClause.Occur.SHOULD);
func (r *TopTermsScoringBooleanQueryRewrite) AddClause(topLevel *BooleanQueryBuilder, term *index.Term, docCount int, boost float32, states *index.TermStates) error {
	tq := NewTermQueryWithStates(term, states)
	topLevel.Add(NewBoostQuery(tq, boost), SHOULD)
	return nil
}

// TopTermsBlendedFreqScoringRewrite is a rewrite method that first translates
// each term into a SHOULD clause in a BooleanQuery, but adjusts the
// frequencies used for scoring to be blended across the terms, otherwise the
// rarest term typically ranks highest (often not useful eg in the set of
// expanded terms in a FuzzyQuery).
//
// This rewrite method only uses the top scoring terms so it will not overflow
// the boolean max clause count.
//
// Mirrors the nested class MultiTermQuery.TopTermsBlendedFreqScoringRewrite,
// declared as {@code extends TopTermsRewrite<BlendedTermQuery.Builder>}
// (Lucene 10.5.0).
type TopTermsBlendedFreqScoringRewrite struct {
	*TopTermsRewrite[*BlendedTermQueryBuilder]
}

// NewTopTermsBlendedFreqScoringRewrite creates a
// TopTermsBlendedFreqScoringRewrite for at most size terms.
//
// NOTE: if GetMaxClauseCount() is smaller than size, then it will be used
// instead.
func NewTopTermsBlendedFreqScoringRewrite(size int) *TopTermsBlendedFreqScoringRewrite {
	r := &TopTermsBlendedFreqScoringRewrite{}
	r.TopTermsRewrite = newTopTermsRewrite[*BlendedTermQueryBuilder](size, r)
	return r
}

// GetMaxSize returns IndexSearcher.getMaxClauseCount().
func (r *TopTermsBlendedFreqScoringRewrite) GetMaxSize() int { return GetMaxClauseCount() }

// GetTopLevelBuilder reproduces
//
//	BlendedTermQuery.Builder builder = new BlendedTermQuery.Builder();
//	builder.setRewriteMethod(BlendedTermQuery.BOOLEAN_REWRITE);
//	return builder;
func (r *TopTermsBlendedFreqScoringRewrite) GetTopLevelBuilder() (*BlendedTermQueryBuilder, error) {
	builder := NewBlendedTermQueryBuilder()
	builder.SetRewriteMethod(BOOLEAN_REWRITE)
	return builder, nil
}

// Build returns {@code builder.build()}.
func (r *TopTermsBlendedFreqScoringRewrite) Build(builder *BlendedTermQueryBuilder) Query {
	return builder.Build()
}

// AddClause reproduces {@code topLevel.add(term, boost, states);}.
func (r *TopTermsBlendedFreqScoringRewrite) AddClause(topLevel *BlendedTermQueryBuilder, term *index.Term, docCount int, boost float32, states *index.TermStates) error {
	topLevel.AddWithContext(term, boost, states)
	return nil
}

// TopTermsBoostOnlyBooleanQueryRewrite is a rewrite method that first
// translates each term into a SHOULD clause in a BooleanQuery, but the scores
// are only computed as the boost.
//
// This rewrite method only uses the top scoring terms so it will not overflow
// the boolean max clause count.
//
// Mirrors the nested class
// MultiTermQuery.TopTermsBoostOnlyBooleanQueryRewrite, declared as
// {@code extends TopTermsRewrite<BooleanQuery.Builder>} (Lucene 10.5.0).
type TopTermsBoostOnlyBooleanQueryRewrite struct {
	*TopTermsRewrite[*BooleanQueryBuilder]
}

// NewTopTermsBoostOnlyBooleanQueryRewrite creates a
// TopTermsBoostOnlyBooleanQueryRewrite for at most size terms.
//
// NOTE: if GetMaxClauseCount() is smaller than size, then it will be used
// instead.
func NewTopTermsBoostOnlyBooleanQueryRewrite(size int) *TopTermsBoostOnlyBooleanQueryRewrite {
	r := &TopTermsBoostOnlyBooleanQueryRewrite{}
	r.TopTermsRewrite = newTopTermsRewrite[*BooleanQueryBuilder](size, r)
	return r
}

// GetMaxSize returns IndexSearcher.getMaxClauseCount().
func (r *TopTermsBoostOnlyBooleanQueryRewrite) GetMaxSize() int { return GetMaxClauseCount() }

// GetTopLevelBuilder returns {@code new BooleanQuery.Builder()}.
func (r *TopTermsBoostOnlyBooleanQueryRewrite) GetTopLevelBuilder() (*BooleanQueryBuilder, error) {
	return NewBooleanQueryBuilder(), nil
}

// Build returns {@code builder.build()}.
func (r *TopTermsBoostOnlyBooleanQueryRewrite) Build(builder *BooleanQueryBuilder) Query {
	return builder.Build()
}

// AddClause reproduces
//
//	final Query q = new ConstantScoreQuery(new TermQuery(term, states));
//	topLevel.add(new BoostQuery(q, boost), BooleanClause.Occur.SHOULD);
func (r *TopTermsBoostOnlyBooleanQueryRewrite) AddClause(topLevel *BooleanQueryBuilder, term *index.Term, docFreq int, boost float32, states *index.TermStates) error {
	var q Query = NewConstantScoreQuery(NewTermQueryWithStates(term, states))
	topLevel.Add(NewBoostQuery(q, boost), SHOULD)
	return nil
}
