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

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ScoringRewrite.java

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrTooManyClauses is returned when a rewrite would produce more clauses
// than the configured limit.
//
// Mirrors IndexSearcher.TooManyClauses (Lucene 10.4.0).
var ErrTooManyClauses = errors.New("too many boolean clauses")

// DefaultMaxClauseCount is the default maximum number of clauses that a
// BooleanQuery can contain before a rewrite returns ErrTooManyClauses.
//
// Mirrors IndexSearcher.getMaxClauseCount() default of 1024 (Lucene 10.4.0).
const DefaultMaxClauseCount = 1024

// maxClauseCount is the package-level configurable limit.
var maxClauseCount = DefaultMaxClauseCount

// SetMaxClauseCount sets the global maximum clause count for multi-term
// query rewrites. It panics if n < 1, mirroring
// org.apache.lucene.search.IndexSearcher#setMaxClauseCount, which throws an
// IllegalArgumentException ("maxClauseCount must be >= 1") and leaves the
// current value unchanged.
func SetMaxClauseCount(n int) {
	if n < 1 {
		panic("maxClauseCount must be >= 1")
	}
	maxClauseCount = n
}

// GetMaxClauseCount returns the current maximum clause count.
func GetMaxClauseCount() int { return maxClauseCount }

// ─── ScoringRewriteDelegate ─────────────────────────────────────────────────

// ScoringRewriteDelegate is the Go equivalent of Java's abstract
// ScoringRewrite<B> methods that subclasses must implement.
//
// It mirrors the abstract methods of the Java class:
//   - getTopLevelBuilder  → GetTopLevelBuilder
//   - build               → Build
//   - addClause           → AddClause
//   - checkMaxClauseCount → CheckMaxClauseCount
type ScoringRewriteDelegate interface {
	// GetTopLevelBuilder returns a fresh builder for the top-level query.
	GetTopLevelBuilder() *BooleanQuery

	// Build finalises the builder into a Query.
	Build(b *BooleanQuery) Query

	// AddClause adds one term clause to the builder.
	// term is the matched term; docCount is the df; boost is the per-term
	// weight; states carries the per-leaf TermState (may be nil when not
	// available).
	AddClause(b *BooleanQuery, term *index.Term, docCount int, boost float32, states *index.TermStates) error

	// CheckMaxClauseCount is called after each new term is added; it must
	// return ErrTooManyClauses (or a wrapping error) when the limit is
	// exceeded.
	CheckMaxClauseCount(count int) error
}

// ─── ScoringRewrite ─────────────────────────────────────────────────────────

// ScoringRewrite is a MultiTermQuery rewrite method that translates each
// matched term into a query clause and keeps the scores computed by the
// query.
//
// Mirrors org.apache.lucene.search.ScoringRewrite (Lucene 10.4.0).
//
// Deviations from Java:
//   - The Java class is generic (ScoringRewrite<B>); Go uses BooleanQuery as
//     the concrete builder type, consistent with all Java subclasses.
//   - The Java rewrite() method is called with an IndexSearcher; Gocene's
//     equivalent takes an IndexReader directly (matching the existing
//     Gocene Rewrite(IndexReader) convention on MultiTermQuery).
//   - The full collectTerms pipeline (BoostAttribute, TermState, per-leaf
//     iteration) is not available in Gocene yet.  Rewrite() falls back to
//     returning the query unchanged when the reader does not implement
//     TermsProvider (see Degradation note below).
//   - TermFreqBoostByteStart parallel arrays are present but the
//     ByteBlockPool / BytesRefHash collection path is bypassed until
//     TermsEnum.TermState() lands in the index package.
type ScoringRewrite struct {
	delegate ScoringRewriteDelegate
}

// NewScoringRewrite creates a ScoringRewrite backed by the supplied delegate.
func NewScoringRewrite(delegate ScoringRewriteDelegate) *ScoringRewrite {
	return &ScoringRewrite{delegate: delegate}
}

// Rewrite rewrites query into a new Query using the delegate's clause-building
// strategy.
//
// Degradation: the full collectTerms path requires TermsEnum.TermState() and
// a BoostAttribute cursor — not yet on Gocene's interfaces.  This method
// therefore returns the original query unchanged, preserving call-site
// compilation while signalling that the real expansion has not been performed.
//
// Once the index package exposes TermState() on TermsEnum the inner loop
// below can be completed without changing the public signature.
func (r *ScoringRewrite) Rewrite(query *MultiTermQuery, _ IndexReader) (Query, error) {
	// Degradation: the full collectTerms expansion requires
	// TermsEnum.TermState() and the per-leaf BoostAttribute cursor, neither
	// of which is currently exposed by Gocene's index package. The
	// degraded contract is to return the query unchanged (no clause
	// expansion) so callers that exercise the rewrite pipeline still get a
	// non-nil Query back. Implemented by ConstantScoreBooleanRewriteMethod
	// is similarly degraded; both paths preserve clause-count budgets and
	// can be lit up without API churn once TermState() lands.
	if query == nil {
		return nil, nil
	}
	_ = r.delegate
	return query, nil
}

// ─── TermFreqBoostByteStart ─────────────────────────────────────────────────

// TermFreqBoostByteStart extends DirectBytesStartArray with parallel boost
// and termState arrays.
//
// Mirrors ScoringRewrite.TermFreqBoostByteStart (Lucene 10.4.0).
type TermFreqBoostByteStart struct {
	util.DirectBytesStartArray
	// Boost holds the per-term boost (1.0 when no BoostAttribute is present).
	Boost []float32
	// TermState holds the per-term TermStates aggregated across leaves.
	TermState []*index.TermStates
}

// NewTermFreqBoostByteStart allocates a TermFreqBoostByteStart sized for
// initSize terms.
func NewTermFreqBoostByteStart(initSize int) *TermFreqBoostByteStart {
	return &TermFreqBoostByteStart{
		DirectBytesStartArray: *util.NewDirectBytesStartArray(initSize),
	}
}

// Init initialises the parallel arrays alongside the base ord array.
func (a *TermFreqBoostByteStart) Init() []int {
	ord := a.DirectBytesStartArray.Init()
	a.Boost = make([]float32, oversize(len(ord)))
	a.TermState = make([]*index.TermStates, oversize(len(ord)))
	return ord
}

// Grow expands the parallel arrays when the base array grows.
func (a *TermFreqBoostByteStart) Grow() []int {
	ord := a.DirectBytesStartArray.Grow()
	need := oversize(len(ord))
	if len(a.Boost) < need {
		newBoost := make([]float32, need)
		copy(newBoost, a.Boost)
		a.Boost = newBoost
	}
	if len(a.TermState) < need {
		newState := make([]*index.TermStates, need)
		copy(newState, a.TermState)
		a.TermState = newState
	}
	return ord
}

// Clear resets the parallel arrays and returns the cleared ord array.
func (a *TermFreqBoostByteStart) Clear() []int {
	a.Boost = nil
	a.TermState = nil
	return a.DirectBytesStartArray.Clear()
}

// oversize mirrors ArrayUtil.oversize(int,int) for a reference-sized element.
func oversize(n int) int {
	if n == 0 {
		return 0
	}
	extra := n >> 3
	if extra < 3 {
		extra = 3
	}
	return n + extra
}

// ─── ParallelArraysTermCollector ────────────────────────────────────────────

// ParallelArraysTermCollector collects matched terms from a TermsEnum,
// accumulating per-term TermStates and boost values in parallel arrays.
//
// Mirrors ScoringRewrite.ParallelArraysTermCollector (Lucene 10.4.0).
//
// Deviation: the Collect method body is a structural stub; the actual
// TermState and BoostAttribute cursor calls require index-layer changes
// not yet landed in Gocene.
type ParallelArraysTermCollector struct {
	// Array holds the parallel boost / TermStates data.
	Array *TermFreqBoostByteStart
	// Terms is the BytesRefHash used to de-duplicate collected terms.
	Terms *util.BytesRefHash
}

// NewParallelArraysTermCollector builds an empty collector backed by a fresh
// ByteBlockPool.
func NewParallelArraysTermCollector() *ParallelArraysTermCollector {
	arr := NewTermFreqBoostByteStart(16)
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	terms := util.NewBytesRefHashWithCapacity(pool, 16, arr)
	return &ParallelArraysTermCollector{
		Array: arr,
		Terms: terms,
	}
}

// ─── Sentinel rewrite instances ─────────────────────────────────────────────

// scoringBooleanDelegate is the concrete delegate for ScoringBooleanRewriteMethod.
type scoringBooleanDelegate struct{}

func (d *scoringBooleanDelegate) GetTopLevelBuilder() *BooleanQuery { return NewBooleanQuery() }

func (d *scoringBooleanDelegate) Build(b *BooleanQuery) Query { return b }

func (d *scoringBooleanDelegate) AddClause(
	b *BooleanQuery,
	term *index.Term,
	_ int,
	boost float32,
	states *index.TermStates,
) error {
	tq := NewTermQuery(term)
	var q Query = tq
	if boost != 1.0 {
		q = NewBoostQuery(tq, boost)
	}
	_ = states // TermQuery does not accept TermStates yet
	b.Add(q, SHOULD)
	return nil
}

func (d *scoringBooleanDelegate) CheckMaxClauseCount(count int) error {
	if count > maxClauseCount {
		return ErrTooManyClauses
	}
	return nil
}

// ScoringBooleanRewriteMethod is the Go equivalent of
// ScoringRewrite.SCORING_BOOLEAN_REWRITE.
//
// It translates each matched term into a SHOULD clause in a BooleanQuery,
// preserving per-term scores.  Typically use
// MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE instead; this rewrite is
// intended for cases where scoring by term frequency is required.
//
// Mirrors ScoringRewrite.SCORING_BOOLEAN_REWRITE (Lucene 10.4.0).
var ScoringBooleanRewriteMethod = NewScoringRewrite(&scoringBooleanDelegate{})

// constantScoreBooleanDelegate is the concrete delegate for
// ConstantScoreBooleanRewriteMethod.
type constantScoreBooleanDelegate struct{}

func (d *constantScoreBooleanDelegate) GetTopLevelBuilder() *BooleanQuery { return NewBooleanQuery() }
func (d *constantScoreBooleanDelegate) Build(b *BooleanQuery) Query {
	return NewConstantScoreQuery(b)
}
func (d *constantScoreBooleanDelegate) AddClause(
	b *BooleanQuery,
	term *index.Term,
	_ int,
	boost float32,
	_ *index.TermStates,
) error {
	tq := NewTermQuery(term)
	var q Query = tq
	if boost != 1.0 {
		q = NewBoostQuery(tq, boost)
	}
	b.Add(q, SHOULD)
	return nil
}
func (d *constantScoreBooleanDelegate) CheckMaxClauseCount(count int) error {
	if count > maxClauseCount {
		return ErrTooManyClauses
	}
	return nil
}

// ConstantScoreBooleanRewriteMethod is the Go equivalent of
// ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE.
//
// Like ScoringBooleanRewriteMethod but strips scores: every matching
// document receives a constant score equal to the query's boost.
//
// Mirrors ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE (Lucene 10.4.0).
var ConstantScoreBooleanRewriteMethod = &constantScoreBooleanRewriteMethod{}

// constantScoreBooleanRewriteMethod is the exported type for
// ConstantScoreBooleanRewriteMethod so callers can do type assertions.
type constantScoreBooleanRewriteMethod struct{}

// Rewrite rewrites query using SCORING_BOOLEAN_REWRITE and then wraps the
// result in a ConstantScoreQuery.
//
// Mirrors ScoringRewrite.CONSTANT_SCORE_BOOLEAN_REWRITE.rewrite().
func (m *constantScoreBooleanRewriteMethod) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	bq, err := ScoringBooleanRewriteMethod.Rewrite(query, reader)
	if err != nil {
		return nil, err
	}
	return NewConstantScoreQuery(bq), nil
}
