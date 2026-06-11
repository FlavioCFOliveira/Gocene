// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/document/SortedDocValuesField.java
//   (newSlowSetQuery factory)
//
// SortedSetDocValuesSetQuery matches documents whose SortedSetDocValues
// field carries at least one value from a fixed set of BytesRef terms.
//
// In Lucene 10.4.0 there is no standalone SortedSetDocValuesSetQuery class.
// SortedDocValuesField.newSlowSetQuery and
// SortedSetDocValuesField.newSlowSetQuery both delegate to TermInSetQuery
// with MultiTermQuery.DOC_VALUES_REWRITE.  Gocene mirrors that delegation
// through a named factory function and keeps the concrete type as
// *TermInSetQuery, matching the Java reference's internal representation.

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NewSortedSetDocValuesSetQuery creates a query that matches documents
// whose SortedDocValues or SortedSetDocValues field (field) contains at
// least one of the provided terms.
//
// Mirrors SortedDocValuesField.newSlowSetQuery(String, Collection<BytesRef>)
// and SortedSetDocValuesField.newSlowSetQuery(String, Collection<BytesRef>)
// from Lucene 10.4.0.
//
// NOTE: The returned Query is a *TermInSetQuery.  The Lucene reference
// passes MultiTermQuery.DOC_VALUES_REWRITE to the TermInSetQuery
// constructor; Gocene's TermInSetQuery does not yet support the
// doc-values rewrite path (Rewrite returns self).  A future port of
// the doc-values rewrite path will enable index-based matching without
// postings.
func NewSortedSetDocValuesSetQuery(field string, terms []*util.BytesRef) *TermInSetQuery {
	return NewTermInSetQuery(field, terms)
}
