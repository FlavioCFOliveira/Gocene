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
//   lucene/core/src/java/org/apache/lucene/search/TermInSetQuery.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermInSetQuery is a query that matches documents containing any of the
// specified terms in a given field.  It uses an automaton for efficient
// matching when the term set is large.
//
// Mirrors org.apache.lucene.search.TermInSetQuery (Lucene 10.4.0).
//
// Deviations from Java:
//   - PrefixCodedTerms / FilteredTermsEnum / ByteRunAutomaton are not yet
//     ported; terms are stored as a deduplicated []*util.BytesRef slice.
//   - Rewrite returns self (no blended / doc-values rewrite paths yet).
//   - SetEnum inner class is deferred (FilteredTermsEnum not ported).
//   - ramBytesUsed / getChildResources are not implemented.
//   - newIndexOrDocValuesQuery factory is deferred.
//   - getTermsCount returns len(terms).
//   - visit() on QueryVisitor is deferred pending ByteRunAutomaton port.
type TermInSetQuery struct {
	*BaseQuery
	field string
	terms []*util.BytesRef
}

// NewTermInSetQuery creates a new TermInSetQuery for the given field and terms.
// Terms are deduplicated and nil entries are ignored.
//
// Mirrors TermInSetQuery(String, Collection<BytesRef>).
func NewTermInSetQuery(field string, terms []*util.BytesRef) *TermInSetQuery {
	termMap := make(map[string]*util.BytesRef, len(terms))
	for _, term := range terms {
		if term != nil {
			key := string(term.ValidBytes())
			termMap[key] = term
		}
	}
	uniqueTerms := make([]*util.BytesRef, 0, len(termMap))
	for _, term := range termMap {
		uniqueTerms = append(uniqueTerms, term)
	}
	return &TermInSetQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     uniqueTerms,
	}
}

// Field returns the field name for this query.
func (q *TermInSetQuery) Field() string { return q.field }

// Terms returns the deduplicated terms in this query.
func (q *TermInSetQuery) Terms() []*util.BytesRef { return q.terms }

// GetBytesRefIterator returns an iterator over the terms in this query.
//
// Mirrors TermInSetQuery.getBytesRefIterator().
func (q *TermInSetQuery) GetBytesRefIterator() *TermInSetBytesRefIterator {
	return &TermInSetBytesRefIterator{terms: q.terms}
}

// TermInSetBytesRefIterator iterates over the BytesRef terms of a TermInSetQuery.
//
// Mirrors the anonymous BytesRefIterator returned by getBytesRefIterator().
type TermInSetBytesRefIterator struct {
	terms []*util.BytesRef
	index int
}

// Next returns the next BytesRef, or nil when exhausted.
func (it *TermInSetBytesRefIterator) Next() *util.BytesRef {
	if it.index >= len(it.terms) {
		return nil
	}
	t := it.terms[it.index]
	it.index++
	return t
}

// Clone creates a deep copy of this query.
//
// Mirrors Query.clone().
func (q *TermInSetQuery) Clone() Query {
	cloned := make([]*util.BytesRef, len(q.terms))
	for i, t := range q.terms {
		cloned[i] = t.Clone()
	}
	return NewTermInSetQuery(q.field, cloned)
}

// Equals reports whether this query is equal to other.
//
// Mirrors TermInSetQuery.equals(Object).
func (q *TermInSetQuery) Equals(other Query) bool {
	o, ok := other.(*TermInSetQuery)
	if !ok {
		return false
	}
	if q.field != o.field || len(q.terms) != len(o.terms) {
		return false
	}
	set := make(map[string]struct{}, len(q.terms))
	for _, t := range q.terms {
		set[string(t.ValidBytes())] = struct{}{}
	}
	for _, t := range o.terms {
		if _, found := set[string(t.ValidBytes())]; !found {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
//
// Mirrors TermInSetQuery.hashCode().
func (q *TermInSetQuery) HashCode() int {
	h := 0
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	termHash := 0
	for _, t := range q.terms {
		termHash ^= t.HashCode()
	}
	return 31*h + termHash
}

// Rewrite returns the query unchanged.
//
// Mirrors TermInSetQuery.getTermsEnum() — full rewrite deferred pending
// PrefixCodedTerms and FilteredTermsEnum ports.
func (q *TermInSetQuery) Rewrite(_ IndexReader) (Query, error) {
	return q, nil
}

// String returns a human-readable representation of this query.
//
// Mirrors TermInSetQuery.toString(String).
func (q *TermInSetQuery) String() string {
	if len(q.terms) == 0 {
		return q.field + ":()"
	}
	result := q.field + ":("
	for i, t := range q.terms {
		if i > 0 {
			result += " "
		}
		bytes := t.ValidBytes()
		needsBinary := false
		for _, b := range bytes {
			if b < 0x20 || b > 0x7e {
				needsBinary = true
				break
			}
		}
		if needsBinary {
			result += "["
			for j, b := range bytes {
				if j > 0 {
					result += " "
				}
				result += fmt.Sprintf("%02x", b)
			}
			result += "]"
		} else {
			result += t.String()
		}
	}
	return result + ")"
}

// CreateWeight creates a Weight for this query.
//
// CreateWeight mirrors TermInSetQuery.createWeight in Java.
func (q *TermInSetQuery) CreateWeight(searcher *IndexSearcher, _ bool, boost float32) (Weight, error) {
	return &termInSetWeight{query: q, boost: boost}, nil
}

// termInSetWeight is the Weight implementation for TermInSetQuery.
// Mirrors the inner TermInSetWeight class from Java.
type termInSetWeight struct {
	query *TermInSetQuery
	boost float32
}

func (w *termInSetWeight) ScoreMode() ScoreMode                        { return COMPLETE_NO_SCORES }
func (w *termInSetWeight) GetValue() float32                           { return w.boost }
func (w *termInSetWeight) Query() Query                                { return w.query }
func (w *termInSetWeight) GetQuery() Query                             { return w.query }
func (w *termInSetWeight) GetValueForNormalization() float32           { return w.boost }
func (w *termInSetWeight) Normalize(_ float32)                         {}
func (w *termInSetWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }
func (w *termInSetWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return NewExplanation(false, 0, "no match"), nil
	}
	target, err := scorer.Advance(doc)
	if err != nil || target != doc {
		return NewExplanation(false, 0, "no match"), nil
	}
	return NewExplanation(true, w.boost, fmt.Sprintf("TermInSetQuery, boost=%v", w.boost)), nil
}

// ScorerSupplier wraps Scorer in a lazy ScorerSupplier.
func (w *termInSetWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return &termInSetScorerSupplier{scorer: scorer}, nil
}

// Scorer builds a scoring iterator for documents matching any term in the set.
// Mirrors TermInSetWeight.scorerSupplier#get in Java: gets the TermsEnum for the
// field and collects matching documents by seeking to each term in the set.
func (w *termInSetWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	leafReader := ctx.LeafReader()
	if leafReader == nil {
		return nil, nil
	}

	// The LeafReader must expose Terms(field) to iterate term postings.
	type termsProvider interface {
		Terms(field string) (index.Terms, error)
		MaxDoc() int
	}
	tp, ok := leafReader.(termsProvider)
	if !ok {
		return nil, nil
	}

	terms, err := tp.Terms(w.query.field)
	if err != nil || terms == nil {
		return nil, err
	}

	maxDoc := tp.MaxDoc()
	result, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}

	// For each term in the query, seek in the TermsEnum and collect matching docs.
	te, err := terms.GetIterator()
	if err != nil || te == nil {
		return nil, err
	}

	for _, termBytes := range w.query.terms {
		t := index.NewTerm(w.query.field, string(termBytes.ValidBytes()))
		found, err := te.SeekExact(t)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		pe, err := te.Postings(0) // DOCS_ONLY - just need doc IDs
		if err != nil || pe == nil {
			continue
		}
		for {
			docID, err := pe.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID == index.NO_MORE_DOCS {
				break
			}
			result.Set(docID)
		}
	}

	cardinality := result.Cardinality()
	if cardinality == 0 {
		return nil, nil
	}

	disi := util.NewBitSetIterator(result, int64(cardinality))
	return NewConstantScoreScorer(w.boost, COMPLETE_NO_SCORES, disi), nil
}

// BulkScorer delegates to DefaultBulkScorer via Scorer.
func (w *termInSetWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return NewDefaultBulkScorer(scorer), nil
}

func (w *termInSetWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }
func (w *termInSetWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// termInSetScorerSupplier adapts a Scorer into a ScorerSupplier.
type termInSetScorerSupplier struct {
	scorer Scorer
}

func (s *termInSetScorerSupplier) Get(_ int64) (Scorer, error) { return s.scorer, nil }
func (s *termInSetScorerSupplier) Cost() int64                 { return s.scorer.Cost() }
func (s *termInSetScorerSupplier) SetTopLevelScoringClause()   {}
