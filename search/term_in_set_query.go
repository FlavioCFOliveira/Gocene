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
// Full weight implementation deferred pending TermsEnum / SetEnum port.
func (q *TermInSetQuery) CreateWeight(searcher *IndexSearcher, _ bool, boost float32) (Weight, error) {
	return &termInSetWeight{query: q, boost: boost}, nil
}

// termInSetWeight is the Weight implementation for TermInSetQuery.
//
// Full scorer/bulkScorer are deferred until SetEnum / index integration land.
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
func (w *termInSetWeight) Explain(_ *index.LeafReaderContext, _ int) (Explanation, error) {
	return nil, nil
}
func (w *termInSetWeight) ScorerSupplier(_ *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}
func (w *termInSetWeight) Scorer(_ *index.LeafReaderContext) (Scorer, error) { return nil, nil }
func (w *termInSetWeight) BulkScorer(_ *index.LeafReaderContext) (BulkScorer, error) {
	return nil, nil
}
func (w *termInSetWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }
func (w *termInSetWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}
