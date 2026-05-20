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
//   lucene/core/src/java/org/apache/lucene/search/DocValuesRewriteMethod.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesRewriteMethod rewrites MultiTermQueries into a
// ConstantScoreQuery backed by DocValues for term enumeration.
// It can perform these queries against an unindexed docvalues field.
//
// Mirrors org.apache.lucene.search.DocValuesRewriteMethod (Lucene 10.4.0).
//
// Deviations from the Java reference:
//   - The original is a singleton-like final class with no state; the
//     Go port is a zero-value struct. DefaultDocValuesRewriteMethod
//     satisfies the Java singleton pattern.
//   - DocValuesSkipper integration uses the existing DocValuesSkipper
//     interface already defined in this package; access is via the
//     optional DocValuesSkipperProvider interface on the leaf reader
//     (type-asserted from IndexReaderInterface).
//   - MultiTermQuery.getTermsEnum(Terms) requires the optional
//     MultiTermQueryTermsEnumProvider interface; when absent,
//     ScorerSupplier returns empty (no matches).
//   - TermsEnum ordinal access requires the optional TermsEnumWithOrd
//     interface; when absent the termSet is not populated and scoring
//     falls back to empty.
//   - The wrapper stores the MultiTermQuery as a Query interface so that
//     optional-interface type assertions remain valid at runtime.
type DocValuesRewriteMethod struct{}

// DefaultDocValuesRewriteMethod is the canonical instance.
var DefaultDocValuesRewriteMethod = &DocValuesRewriteMethod{}

// NewDocValuesRewriteMethod returns a DocValuesRewriteMethod.
// All instances are equal (no state). Mirrors the public constructor.
func NewDocValuesRewriteMethod() *DocValuesRewriteMethod {
	return &DocValuesRewriteMethod{}
}

// Rewrite wraps query in a ConstantScoreQuery backed by DocValues.
//
// Mirrors DocValuesRewriteMethod.rewrite(IndexSearcher, MultiTermQuery).
func (m *DocValuesRewriteMethod) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	return NewConstantScoreQuery(newMultiTermQueryDocValuesWrapper(query)), nil
}

// Equals reports whether other is also a *DocValuesRewriteMethod.
func (m *DocValuesRewriteMethod) Equals(other any) bool {
	if m == other {
		return true
	}
	_, ok := other.(*DocValuesRewriteMethod)
	return ok
}

// HashCode returns a stable constant (matching Java's classHash 641).
func (m *DocValuesRewriteMethod) HashCode() uint64 { return 641 }

// ─── Optional capability interfaces ─────────────────────────────────────────

// MultiTermQueryTermsEnumProvider is an optional interface that
// MultiTermQuery implementations may satisfy to produce a TermsEnum
// over a Terms instance. Mirrors MultiTermQuery.getTermsEnum(Terms).
type MultiTermQueryTermsEnumProvider interface {
	GetTermsEnum(terms index.Terms) (index.TermsEnum, error)
}

// SortedSetDocValuesWithTermsEnum is an optional interface for
// SortedSetDocValues that expose a TermsEnum for the field's terms.
// Required for full DocValues rewrite scoring.
type SortedSetDocValuesWithTermsEnum interface {
	index.SortedSetDocValues
	// TermsEnum returns a TermsEnum positioned at the start of the values.
	TermsEnum() (index.TermsEnum, error)
}

// SortedSetDocValuesOrdIterable is an optional interface for
// SortedSetDocValues that support NextOrd / DocValueCount access.
type SortedSetDocValuesOrdIterable interface {
	// NextOrd returns the next ordinal for the current document.
	// Returns -1 when no more ords remain for the current doc.
	NextOrd() (int64, error)
	// DocValueCount returns the number of ordinals for the current doc.
	DocValueCount() int
}

// TermsEnumWithOrd is an optional interface for TermsEnum that can
// report the current ordinal within a SortedSetDocValues.
type TermsEnumWithOrd interface {
	// Ord returns the ordinal of the current term.
	Ord() (int64, error)
}

// SortedDocValuesWithOrd is an optional interface for SortedDocValues
// that can return the current doc's ordinal without seeking.
type SortedDocValuesWithOrd interface {
	// OrdValue returns the ordinal for the current doc, or -1 if absent.
	OrdValue() (int64, error)
}

// DocValuesSkipperProvider is an optional interface for a leaf reader
// (type-asserted from IndexReaderInterface) that provides a
// DocValuesSkipper for a named field.
type DocValuesSkipperProvider interface {
	// GetDocValuesSkipper returns the DocValuesSkipper, or nil if none.
	GetDocValuesSkipper(field string) (DocValuesSkipper, error)
}

// ─── multiTermQueryDocValuesWrapper ─────────────────────────────────────────

// multiTermQueryDocValuesWrapper wraps a MultiTermQuery as a Query
// that evaluates using DocValues.
//
// The wrapped query is stored as a Query interface so optional-interface
// type assertions (e.g. MultiTermQueryTermsEnumProvider) work at runtime.
//
// Mirrors DocValuesRewriteMethod.MultiTermQueryDocValuesWrapper.
type multiTermQueryDocValuesWrapper struct {
	BaseQuery
	// mtq is the concrete MultiTermQuery for field/clone access.
	mtq *MultiTermQuery
	// asQuery is the same value stored as Query for type assertions.
	asQuery Query
}

func newMultiTermQueryDocValuesWrapper(query *MultiTermQuery) *multiTermQueryDocValuesWrapper {
	return &multiTermQueryDocValuesWrapper{mtq: query, asQuery: query}
}

// GetField returns the field name.
func (w *multiTermQueryDocValuesWrapper) GetField() string { return w.mtq.GetField() }

// String mirrors MultiTermQueryDocValuesWrapper.toString.
func (w *multiTermQueryDocValuesWrapper) String(field string) string {
	return w.mtq.String(field)
}

// Equals mirrors MultiTermQueryDocValuesWrapper.equals.
func (w *multiTermQueryDocValuesWrapper) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*multiTermQueryDocValuesWrapper)
	if !ok || o == nil {
		return false
	}
	return w.mtq.Equals(o.mtq)
}

// HashCode mirrors MultiTermQueryDocValuesWrapper.hashCode:
// 31 * classHash + query.hashCode().
func (w *multiTermQueryDocValuesWrapper) HashCode() int {
	const classSeed = int(0x4d56_7a3c)
	return 31*classSeed + w.mtq.HashCode()
}

// Visit implements Query.
func (w *multiTermQueryDocValuesWrapper) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(w.mtq.GetField()) {
		visitor.GetSubVisitor(FILTER, w.asQuery)
	}
}

// Rewrite returns self.
func (w *multiTermQueryDocValuesWrapper) Rewrite(reader IndexReader) (Query, error) {
	return w, nil
}

// CreateWeight builds the DocValues-backed constant-score Weight.
func (w *multiTermQueryDocValuesWrapper) CreateWeight(
	searcher *IndexSearcher,
	needsScores bool,
	boost float32,
) (Weight, error) {
	return NewConstantScoreWeight(
		w, boost,
		func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
			return dvwScorerSupplier(w, ctx, boost)
		},
		func(_ *index.LeafReaderContext) bool { return true },
	), nil
}

// Compile-time check: multiTermQueryDocValuesWrapper satisfies Query.
var _ Query = (*multiTermQueryDocValuesWrapper)(nil)

// ─── scorer supplier ─────────────────────────────────────────────────────────

// dvwScorerSupplier builds the per-leaf ScorerSupplier.
func dvwScorerSupplier(
	w *multiTermQueryDocValuesWrapper,
	ctx *index.LeafReaderContext,
	score float32,
) (ScorerSupplier, error) {
	// ctx.Reader() is IndexReaderInterface; type-assert to *index.LeafReader
	// for concrete DocValues access.
	lr, ok := ctx.Reader().(*index.LeafReader)
	if !ok {
		return nil, nil
	}
	values, err := lr.GetSortedSetDocValues(w.mtq.GetField())
	if err != nil {
		return nil, err
	}
	if values == nil || values.GetValueCount() == 0 {
		return nil, nil
	}

	ext, hasExt := values.(SortedSetDocValuesWithTermsEnum)
	if !hasExt {
		return newEmptyDVWScorerSupplier(score), nil
	}
	// w.asQuery is a Query interface — type assertions for optional
	// interfaces work on interface values.
	provider, hasProvider := w.asQuery.(MultiTermQueryTermsEnumProvider)
	if !hasProvider {
		return newEmptyDVWScorerSupplier(score), nil
	}
	te, err := provider.GetTermsEnum(newSortedSetDocValuesTerms(ext))
	if err != nil {
		return nil, err
	}
	if te == nil {
		return newEmptyDVWScorerSupplier(score), nil
	}
	first, err := te.Next()
	if err != nil {
		return nil, err
	}
	if first == nil {
		// No matching terms.
		return newEmptyDVWScorerSupplier(score), nil
	}
	termSet, minOrd, maxOrd, err := dvwBuildTermSet(te, values.GetValueCount())
	if err != nil {
		return nil, err
	}
	if maxOrd < 0 {
		return newEmptyDVWScorerSupplier(score), nil
	}

	// Optional skipper range pruning. ctx.Reader() is IndexReaderInterface
	// so type assertions for optional interfaces work.
	if sp, ok2 := ctx.Reader().(DocValuesSkipperProvider); ok2 {
		skipper, serr := sp.GetDocValuesSkipper(w.mtq.GetField())
		if serr != nil {
			return nil, serr
		}
		if skipper != nil &&
			(minOrd > skipper.MaxValue(0) || maxOrd < skipper.MinValue(0)) {
			return newEmptyDVWScorerSupplier(score), nil
		}
	}

	twoPhase, err := dvwBuildTwoPhase(values, termSet, maxOrd)
	if err != nil {
		return nil, err
	}
	disi := NewTwoPhaseIteratorAsDocIdSetIterator(twoPhase)
	// Use GetValueCount as a cost upper-bound; actual cost is the
	// number of matched docs, which is unknown before iteration.
	cost := int64(values.GetValueCount())
	return &dvwScorerSupplierImpl{score: score, disi: disi, cost: cost}, nil
}

// dvwBuildTermSet populates a LongBitSet from the TermsEnum ordinals.
// Returns the populated set plus the min and max ord encountered. The
// TermsEnum must already have been advanced past the first term.
func dvwBuildTermSet(
	te index.TermsEnum,
	valueCount int,
) (termSet *util.LongBitSet, minOrd, maxOrd int64, err error) {
	termSet, err = util.NewLongBitSet(int64(valueCount))
	if err != nil {
		return
	}
	withOrd, canOrd := te.(TermsEnumWithOrd)
	if !canOrd {
		return termSet, 0, -1, nil
	}
	minOrd = int64(valueCount)
	maxOrd = -1
	for {
		var ord int64
		ord, err = withOrd.Ord()
		if err != nil {
			return
		}
		if ord >= 0 {
			if ord < minOrd {
				minOrd = ord
			}
			if ord > maxOrd {
				maxOrd = ord
			}
			termSet.Set(ord)
		}
		var next *index.Term
		next, err = te.Next()
		if err != nil || next == nil {
			err = nil
			break
		}
	}
	return
}

// dvwBuildTwoPhase constructs a TwoPhaseIterator that matches documents
// where at least one ordinal is in termSet and ≤ maxOrd.
func dvwBuildTwoPhase(
	values index.SortedSetDocValues,
	termSet *util.LongBitSet,
	maxOrd int64,
) (*TwoPhaseIterator, error) {
	// Try singleton path (SortedDocValues underlying).
	singleton := index.UnwrapSingletonSortedSet(values)
	if singleton != nil {
		withOrd, ok := singleton.(SortedDocValuesWithOrd)
		if ok {
			approx := castToDISI(singleton)
			return NewTwoPhaseIteratorWithMatchCost(approx, func() (bool, error) {
				ord, err := withOrd.OrdValue()
				if err != nil {
					return false, err
				}
				return ord >= 0 && termSet.Get(ord), nil
			}, 3), nil
		}
	}

	// Multi-valued path.
	ordIter, hasOrdIter := values.(SortedSetDocValuesOrdIterable)
	if hasOrdIter {
		approx := castToDISI(values)
		max := maxOrd
		return NewTwoPhaseIteratorWithMatchCost(approx, func() (bool, error) {
			count := ordIter.DocValueCount()
			for i := 0; i < count; i++ {
				ord, err := ordIter.NextOrd()
				if err != nil {
					return false, err
				}
				if ord > max {
					return false, nil // values are sorted — terminate early
				}
				if termSet.Get(ord) {
					return true, nil
				}
			}
			return false, nil
		}, 3), nil
	}

	// Fallback: no ord iteration — never matches.
	return NewTwoPhaseIteratorWithMatchCost(
		castToDISI(values),
		func() (bool, error) { return false, nil },
		3,
	), nil
}

// ─── sortedSetDocValuesTerms ─────────────────────────────────────────────────

// sortedSetDocValuesTerms wraps a SortedSetDocValuesWithTermsEnum as an
// index.Terms, allowing MultiTermQueryTermsEnumProvider to enumerate it.
type sortedSetDocValuesTerms struct {
	values SortedSetDocValuesWithTermsEnum
}

func newSortedSetDocValuesTerms(v SortedSetDocValuesWithTermsEnum) *sortedSetDocValuesTerms {
	return &sortedSetDocValuesTerms{values: v}
}

func (t *sortedSetDocValuesTerms) GetIterator() (index.TermsEnum, error) {
	return t.values.TermsEnum()
}

func (t *sortedSetDocValuesTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te, err := t.values.TermsEnum()
	if err != nil || te == nil {
		return te, err
	}
	_, err = te.SeekCeil(seekTerm)
	return te, err
}

func (t *sortedSetDocValuesTerms) GetDocCount() (int, error)           { return -1, nil }
func (t *sortedSetDocValuesTerms) GetSumDocFreq() (int64, error)       { return -1, nil }
func (t *sortedSetDocValuesTerms) GetSumTotalTermFreq() (int64, error) { return -1, nil }
func (t *sortedSetDocValuesTerms) Size() int64                         { return -1 }
func (t *sortedSetDocValuesTerms) HasFreqs() bool                      { return false }
func (t *sortedSetDocValuesTerms) HasOffsets() bool                    { return false }
func (t *sortedSetDocValuesTerms) HasPositions() bool                  { return false }
func (t *sortedSetDocValuesTerms) HasPayloads() bool                   { return false }
func (t *sortedSetDocValuesTerms) GetMin() (*index.Term, error)        { return nil, nil }
func (t *sortedSetDocValuesTerms) GetMax() (*index.Term, error)        { return nil, nil }

// GetPostingsReader returns nil — DocValues terms carry no postings.
func (t *sortedSetDocValuesTerms) GetPostingsReader(_ string, _ int) (index.PostingsEnum, error) {
	return nil, nil
}

// Compile-time check.
var _ index.Terms = (*sortedSetDocValuesTerms)(nil)

// ─── helpers ─────────────────────────────────────────────────────────────────

// castToDISI attempts to use v as a DocIdSetIterator; returns an
// always-exhausted DISI when v does not implement the interface.
func castToDISI(v any) DocIdSetIterator {
	if d, ok := v.(DocIdSetIterator); ok {
		return d
	}
	return newAlwaysExhaustedDISI()
}

// alwaysExhaustedDISI is a DocIdSetIterator that is immediately exhausted.
type alwaysExhaustedDISI struct{}

func (e *alwaysExhaustedDISI) DocID() int                      { return NO_MORE_DOCS }
func (e *alwaysExhaustedDISI) NextDoc() (int, error)           { return NO_MORE_DOCS, nil }
func (e *alwaysExhaustedDISI) Advance(target int) (int, error) { return NO_MORE_DOCS, nil }
func (e *alwaysExhaustedDISI) Cost() int64                     { return 0 }
func (e *alwaysExhaustedDISI) DocIDRunEnd() int                { return NO_MORE_DOCS }

func newAlwaysExhaustedDISI() DocIdSetIterator { return &alwaysExhaustedDISI{} }

// dvwScorerSupplierImpl is the ScorerSupplier returned by dvwScorerSupplier.
type dvwScorerSupplierImpl struct {
	score float32
	disi  DocIdSetIterator
	cost  int64
}

func (s *dvwScorerSupplierImpl) Get(leadCost int64) (Scorer, error) {
	return NewConstantScoreScorer(s.score, TOP_SCORES, s.disi), nil
}

func (s *dvwScorerSupplierImpl) Cost() int64 { return s.cost }

// SetTopLevelScoringClause is a no-op for DocValues-backed suppliers;
// the constant score does not benefit from top-level clause hints.
func (s *dvwScorerSupplierImpl) SetTopLevelScoringClause() {}

// newEmptyDVWScorerSupplier returns a ScorerSupplier that yields zero
// results (immediately exhausted).
func newEmptyDVWScorerSupplier(score float32) ScorerSupplier {
	return &dvwScorerSupplierImpl{score: score, disi: newAlwaysExhaustedDISI(), cost: 0}
}

// dvwFormat produces a debug string. Used by tests.
func dvwFormat(field, queryStr string) string {
	return fmt.Sprintf("DocValuesRewriteMethod(%s:%s)", field, queryStr)
}

// Compile-time checks.
var (
	_ DocIdSetIterator = (*alwaysExhaustedDISI)(nil)
	_ ScorerSupplier   = (*dvwScorerSupplierImpl)(nil)
)
