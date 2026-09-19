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
//	lucene/core/src/java/org/apache/lucene/search/ScoringRewrite.java

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrTooManyClauses is returned when a rewrite would produce more clauses
// than the configured limit.
//
// Mirrors IndexSearcher.TooManyClauses (Lucene 10.5.0).
var ErrTooManyClauses = errors.New("too many boolean clauses")

// DefaultMaxClauseCount is the default maximum number of clauses that a
// BooleanQuery can contain before a rewrite returns ErrTooManyClauses.
//
// Mirrors IndexSearcher.getMaxClauseCount() default of 1024 (Lucene 10.5.0).
const DefaultMaxClauseCount = 1024

// SetMaxClauseCount and GetMaxClauseCount are declared by
// org.apache.lucene.search.IndexSearcher, not by ScoringRewrite, and therefore
// live in index_searcher.go over the MaxClauseCount variable. ScoringRewrite
// reads the limit the way Java does, through IndexSearcher.getMaxClauseCount().

// ─── ScoringRewriteDelegate ─────────────────────────────────────────────────

// ScoringRewriteClauseCounter carries the single abstract member ScoringRewrite
// declares for itself:
//
//	protected abstract void checkMaxClauseCount(int count) throws IOException;
//
// It is separated from [ScoringRewriteDelegate] because Java's inner class
// ScoringRewrite.ParallelArraysTermCollector reaches exactly this member
// through {@code ScoringRewrite.this}, and nothing else on the outer instance.
// Holding the narrow contract lets the collector stay non-generic, as the Java
// inner class effectively is at that call site.
type ScoringRewriteClauseCounter interface {
	// CheckMaxClauseCount is called after every new term to check that the
	// number of max clauses (e.g. in BooleanQuery) is not exceeded.
	CheckMaxClauseCount(count int) error
}

// ScoringRewriteDelegate carries the abstract members a ScoringRewrite<B>
// subclass must supply: checkMaxClauseCount(int) declared by ScoringRewrite
// itself, and getTopLevelBuilder() / build(B) / addClause(B, Term, int, float,
// TermStates) inherited from TermCollectingRewrite<B>.
//
// Go has no abstract methods, so the concrete rewrite installs itself as the
// delegate in its constructor — the same indirection [TopTermsRewriteDelegate]
// uses for TopTermsRewrite.
type ScoringRewriteDelegate[B any] interface {
	ScoringRewriteClauseCounter

	// GetTopLevelBuilder returns a suitable builder for the top-level Query
	// holding all expanded terms.
	//
	// Mirrors {@code protected abstract B getTopLevelBuilder() throws IOException}.
	GetTopLevelBuilder() (B, error)

	// Build finalizes the creation of the query from the builder.
	//
	// Mirrors {@code protected abstract Query build(B builder)}. Java's
	// signature declares no checked exception; the Go error return carries
	// the unchecked IllegalArgumentException a builder may raise (for
	// example SpanOrQuery's "Clauses must have same field").
	Build(builder B) (Query, error)

	// AddClause adds a MultiTermQuery term to the top-level query builder.
	//
	// Mirrors {@code protected abstract void addClause(B topLevel, Term term,
	// int docCount, float boost, TermStates states) throws IOException}.
	AddClause(topLevel B, term *index.Term, docCount int, boost float32, states *index.TermStates) error
}

// ─── ScoringRewrite ─────────────────────────────────────────────────────────

// ScoringRewrite is the base rewrite method that translates each term into a
// query, and keeps the scores as computed by the query.
//
// Mirrors org.apache.lucene.search.ScoringRewrite<B>, declared as
// {@code public abstract class ScoringRewrite<B> extends
// TermCollectingRewrite<B>} (Lucene 10.5.0). Java's type parameter B is
// carried over directly; Lucene marks the class @lucene.internal and notes it
// is "Only public to be accessible by spans package", which is exactly what
// SpanMultiTermQueryWrapper.SCORING_SPAN_QUERY_REWRITE uses it for.
type ScoringRewrite[B any] struct {
	delegate ScoringRewriteDelegate[B]
}

// NewScoringRewrite creates a ScoringRewrite backed by the supplied delegate.
//
// The delegate parameter is the Go stand-in for the subclass body that Java
// reaches through {@code this}.
func NewScoringRewrite[B any](delegate ScoringRewriteDelegate[B]) *ScoringRewrite[B] {
	return &ScoringRewrite[B]{delegate: delegate}
}

// CheckMaxClauseCount forwards to the installed delegate, reproducing the
// dispatch Java performs on {@code ScoringRewrite.this.checkMaxClauseCount}.
func (r *ScoringRewrite[B]) CheckMaxClauseCount(count int) error {
	return r.delegate.CheckMaxClauseCount(count)
}

// Rewrite collects every term of query across the searcher's leaves and
// assembles them, in sorted term order, into the builder supplied by the
// delegate.
//
// Mirrors {@code public final Query rewrite(IndexSearcher indexSearcher,
// final MultiTermQuery query)} of Apache Lucene 10.5.0, statement for
// statement. The Java body's two asserts (state != null, and
// reader.docFreq(term) == termStates[pos].docFreq()) are omitted: Java asserts
// are disabled unless the JVM runs with -ea, so they are not observable
// behaviour.
func (r *ScoringRewrite[B]) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	// IndexReader reader = indexSearcher.getIndexReader();
	reader := searcher.GetIndexReader()

	// final B builder = getTopLevelBuilder();
	builder, err := r.delegate.GetTopLevelBuilder()
	if err != nil {
		return nil, err
	}

	// final ParallelArraysTermCollector col = new ParallelArraysTermCollector();
	// collectTerms(reader, query, col);
	col := NewParallelArraysTermCollector(r)
	if err := CollectTerms(r, reader, query, col); err != nil {
		return nil, err
	}

	// final int size = col.terms.size();
	size := col.Terms.Size()
	if size > 0 {
		// final int[] sort = col.terms.sort();
		sort := col.Terms.Sort()
		boost := col.Array.Boost
		termStates := col.Array.TermState
		for i := 0; i < size; i++ {
			pos := sort[i]
			// final Term term = new Term(query.getField(), col.terms.get(pos, new BytesRef()));
			term := index.NewTermFromBytesRef(query.GetField(), col.Terms.Get(pos, util.NewBytesRefEmpty()))
			if err := r.delegate.AddClause(builder, term, termStates[pos].DocFreq(), boost[pos], termStates[pos]); err != nil {
				return nil, err
			}
		}
	}
	// return build(builder);
	return r.delegate.Build(builder)
}

// ─── TermFreqBoostByteStart ─────────────────────────────────────────────────

// TermFreqBoostByteStart is the special BytesStartArray that keeps parallel
// arrays for boost and docFreq.
//
// Mirrors the nested class ScoringRewrite.TermFreqBoostByteStart, declared as
// {@code static final class TermFreqBoostByteStart extends
// DirectBytesStartArray} (Lucene 10.5.0).
type TermFreqBoostByteStart struct {
	util.DirectBytesStartArray
	// Boost holds the per-term boost (1.0 when no BoostAttribute is present).
	Boost []float32
	// TermState holds the per-term TermStates aggregated across leaves.
	TermState []*index.TermStates
}

// NewTermFreqBoostByteStart allocates a TermFreqBoostByteStart sized for
// initSize terms.
//
// Mirrors {@code public TermFreqBoostByteStart(int initSize)}.
func NewTermFreqBoostByteStart(initSize int) *TermFreqBoostByteStart {
	return &TermFreqBoostByteStart{
		DirectBytesStartArray: *util.NewDirectBytesStartArray(initSize),
	}
}

// Init reproduces
//
//	final int[] ord = super.init();
//	boost = new float[ArrayUtil.oversize(ord.length, Float.BYTES)];
//	termState = new TermStates[ArrayUtil.oversize(ord.length, NUM_BYTES_OBJECT_REF)];
//	return ord;
func (a *TermFreqBoostByteStart) Init() []int {
	ord := a.DirectBytesStartArray.Init()
	a.Boost = make([]float32, util.Oversize(len(ord), 4))
	a.TermState = make([]*index.TermStates, util.Oversize(len(ord), util.NumBytesObjectRef))
	return ord
}

// Grow reproduces
//
//	final int[] ord = super.grow();
//	boost = ArrayUtil.grow(boost, ord.length);
//	if (termState.length < ord.length) { ...grow termState... }
//	return ord;
func (a *TermFreqBoostByteStart) Grow() []int {
	ord := a.DirectBytesStartArray.Grow()
	a.Boost = util.GrowFloat32(a.Boost, len(ord))
	if len(a.TermState) < len(ord) {
		tmpTermState := make([]*index.TermStates, util.Oversize(len(ord), util.NumBytesObjectRef))
		copy(tmpTermState, a.TermState)
		a.TermState = tmpTermState
	}
	return ord
}

// Clear reproduces
//
//	boost = null;
//	termState = null;
//	return super.clear();
func (a *TermFreqBoostByteStart) Clear() []int {
	a.Boost = nil
	a.TermState = nil
	return a.DirectBytesStartArray.Clear()
}

// ─── ParallelArraysTermCollector ────────────────────────────────────────────

// ParallelArraysTermCollector collects matched terms from a TermsEnum,
// accumulating per-term TermStates and boost values in parallel arrays.
//
// Mirrors the inner class ScoringRewrite.ParallelArraysTermCollector, declared
// as {@code final class ParallelArraysTermCollector extends TermCollector}
// (Lucene 10.5.0). Java's implicit outer-instance reference — used solely for
// {@code ScoringRewrite.this.checkMaxClauseCount(terms.size())} — is carried
// explicitly by the outer field.
type ParallelArraysTermCollector struct {
	BaseTermCollector

	// Array holds the parallel boost / TermStates data.
	Array *TermFreqBoostByteStart
	// Terms is the BytesRefHash used to de-duplicate collected terms.
	Terms *util.BytesRefHash

	outer     ScoringRewriteClauseCounter
	termsEnum index.TermsEnum
	boostAtt  BoostAttribute
}

// NewParallelArraysTermCollector builds an empty collector backed by a fresh
// ByteBlockPool, bound to the ScoringRewrite that constructed it.
//
// Mirrors the field initialisers
//
//	final TermFreqBoostByteStart array = new TermFreqBoostByteStart(16);
//	final BytesRefHash terms =
//	    new BytesRefHash(new ByteBlockPool(new ByteBlockPool.DirectAllocator()), 16, array);
//
// together with the implicit inner-class constructor argument.
func NewParallelArraysTermCollector(outer ScoringRewriteClauseCounter) *ParallelArraysTermCollector {
	arr := NewTermFreqBoostByteStart(16)
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	terms := util.NewBytesRefHashWithCapacity(pool, 16, arr)
	return &ParallelArraysTermCollector{
		Array: arr,
		Terms: terms,
		outer: outer,
	}
}

// SetNextEnum reproduces
//
//	this.termsEnum = termsEnum;
//	this.boostAtt = termsEnum.attributes().addAttribute(BoostAttribute.class);
func (c *ParallelArraysTermCollector) SetNextEnum(termsEnum index.TermsEnum) error {
	c.termsEnum = termsEnum
	boostAtt, ok := termsEnum.Attributes().AddAttribute(BoostAttributeType).(BoostAttribute)
	if !ok {
		return errBoostAttribute
	}
	c.boostAtt = boostAtt
	return nil
}

// Collect reproduces {@code public boolean collect(BytesRef bytes)} of
// ScoringRewrite.ParallelArraysTermCollector.
//
// Gocene's TermCollector hands over the whole Term rather than the raw
// BytesRef (see search/term_collecting_rewrite.go); bytes is term.Bytes.
func (c *ParallelArraysTermCollector) Collect(term *index.Term) (bool, error) {
	// final int e = terms.add(bytes);
	e, err := c.Terms.Add(term.Bytes)
	if err != nil {
		return false, err
	}
	// final TermState state = termsEnum.termState();
	state, err := index.TermStateDelegated(c.termsEnum)
	if err != nil {
		return false, err
	}
	docFreq, err := c.termsEnum.DocFreq()
	if err != nil {
		return false, err
	}
	totalTermFreq, err := c.termsEnum.TotalTermFreq()
	if err != nil {
		return false, err
	}
	if e < 0 {
		// duplicate term: update docFreq
		pos := (-e) - 1
		c.Array.TermState[pos].Register(c.ReaderContext.Ord, state, docFreq, totalTermFreq)
	} else {
		// new entry: we populate the entry initially
		c.Array.Boost[e] = c.boostAtt.GetBoost()
		// new TermStates(topReaderContext, state, readerContext.ord,
		//                termsEnum.docFreq(), termsEnum.totalTermFreq())
		//
		// is `this(null, context); register(state, ord, docFreq, totalTermFreq);`
		// in TermStates — spelled out here because index.TermStates exposes the
		// one-argument constructor and register(), not the five-argument form.
		states, err := index.NewTermStatesForContext(c.TopReaderContext)
		if err != nil {
			return false, err
		}
		states.Register(c.ReaderContext.Ord, state, docFreq, totalTermFreq)
		c.Array.TermState[e] = states
		if err := c.outer.CheckMaxClauseCount(c.Terms.Size()); err != nil {
			return false, err
		}
	}
	return true, nil
}

// ─── Sentinel rewrite instances ─────────────────────────────────────────────

// scoringBooleanDelegate is the anonymous ScoringRewrite<BooleanQuery.Builder>
// subclass assigned to ScoringRewrite.SCORING_BOOLEAN_REWRITE.
type scoringBooleanDelegate struct{}

// GetTopLevelBuilder returns {@code new BooleanQuery.Builder()}.
func (d *scoringBooleanDelegate) GetTopLevelBuilder() (*BooleanQueryBuilder, error) {
	return NewBooleanQueryBuilder(), nil
}

// Build returns {@code builder.build()}.
func (d *scoringBooleanDelegate) Build(builder *BooleanQueryBuilder) (Query, error) {
	return builder.Build(), nil
}

// AddClause reproduces
//
//	final TermQuery tq = new TermQuery(term, states);
//	topLevel.add(new BoostQuery(tq, boost), BooleanClause.Occur.SHOULD);
func (d *scoringBooleanDelegate) AddClause(
	topLevel *BooleanQueryBuilder,
	term *index.Term,
	_ int,
	boost float32,
	states *index.TermStates,
) error {
	tq := NewTermQueryWithStates(term, states)
	topLevel.Add(NewBoostQuery(tq, boost), SHOULD)
	return nil
}

// CheckMaxClauseCount reproduces
//
//	if (count > IndexSearcher.getMaxClauseCount()) throw new IndexSearcher.TooManyClauses();
func (d *scoringBooleanDelegate) CheckMaxClauseCount(count int) error {
	if count > GetMaxClauseCount() {
		return ErrTooManyClauses
	}
	return nil
}

// ScoringBooleanRewriteMethod is a rewrite method that first translates each
// term into a BooleanClause.Occur#SHOULD clause in a BooleanQuery, and keeps
// the scores as computed by the query. Note that typically such scores are
// meaningless to the user, and require non-trivial CPU to compute, so it is
// almost always better to use MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE or
// MultiTermQuery.CONSTANT_SCORE_REWRITE instead.
//
// NOTE: this rewrite method returns ErrTooManyClauses if the number of terms
// exceeds IndexSearcher.getMaxClauseCount().
//
// Mirrors ScoringRewrite.SCORING_BOOLEAN_REWRITE (Lucene 10.5.0).
var ScoringBooleanRewriteMethod = NewScoringRewrite[*BooleanQueryBuilder](&scoringBooleanDelegate{})

// constantScoreBooleanRewriteMethod is the anonymous RewriteMethod assigned to
// ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE.
type constantScoreBooleanRewriteMethod struct{}

// Rewrite reproduces
//
//	final Query bq = SCORING_BOOLEAN_REWRITE.rewrite(indexSearcher, query);
//	// strip the scores off
//	return new ConstantScoreQuery(bq);
func (m *constantScoreBooleanRewriteMethod) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	bq, err := ScoringBooleanRewriteMethod.Rewrite(searcher, query)
	if err != nil {
		return nil, err
	}
	return NewConstantScoreQuery(bq), nil
}

// ConstantScoreBooleanRewriteMethod is like ScoringBooleanRewriteMethod except
// scores are not computed: each matching document receives a constant score
// equal to the query's boost.
//
// NOTE: this rewrite method returns ErrTooManyClauses if the number of terms
// exceeds IndexSearcher.getMaxClauseCount().
//
// Mirrors ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE (Lucene 10.5.0).
var ConstantScoreBooleanRewriteMethod RewriteMethod = &constantScoreBooleanRewriteMethod{}
