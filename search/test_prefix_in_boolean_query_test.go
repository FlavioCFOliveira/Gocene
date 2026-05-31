// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPrefixInBooleanQuery.java
//
// A corpus of "meaninglessnames" documents with exactly two "tangfulin"
// documents validates that PrefixQuery and TermQuery — standalone and nested
// inside a BooleanQuery SHOULD clause — match the same two documents through
// the end-to-end IndexWriter -> IndexSearcher path (rmp #18 / #123).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const prefixBoolField = "name"

// buildPrefixInBooleanIndex mirrors TestPrefixInBooleanQuery.beforeClass:
// 5137 "meaninglessnames", one "tangfulin", 6239 more "meaninglessnames", then
// a second "tangfulin" — exactly two documents carry "tangfulin".
func buildPrefixInBooleanIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < 5137; i++ {
		ix.addString(prefixBoolField, "meaninglessnames")
	}
	ix.addString(prefixBoolField, "tangfulin")
	for i := 5138; i < 11377; i++ {
		ix.addString(prefixBoolField, "meaninglessnames")
	}
	ix.addString(prefixBoolField, "tangfulin")
	return ix.searcher()
}

func TestPrefixInBooleanQuery_PrefixQuery(t *testing.T) {
	s, done := buildPrefixInBooleanIndex(t)
	defer done()
	q := search.NewPrefixQuery(index.NewTerm(prefixBoolField, "tang"))
	assertHitCount(t, s, q, 2)
}

func TestPrefixInBooleanQuery_TermQuery(t *testing.T) {
	s, done := buildPrefixInBooleanIndex(t)
	defer done()
	q := search.NewTermQuery(index.NewTerm(prefixBoolField, "tangfulin"))
	assertHitCount(t, s, q, 2)
}

func TestPrefixInBooleanQuery_TermBooleanQuery(t *testing.T) {
	s, done := buildPrefixInBooleanIndex(t)
	defer done()
	q := search.NewBooleanQuery()
	q.Add(search.NewTermQuery(index.NewTerm(prefixBoolField, "tangfulin")), search.SHOULD)
	q.Add(search.NewTermQuery(index.NewTerm(prefixBoolField, "notexistnames")), search.SHOULD)
	assertHitCount(t, s, q, 2)
}

func TestPrefixInBooleanQuery_PrefixBooleanQuery(t *testing.T) {
	s, done := buildPrefixInBooleanIndex(t)
	defer done()
	q := search.NewBooleanQuery()
	q.Add(search.NewPrefixQuery(index.NewTerm(prefixBoolField, "tang")), search.SHOULD)
	q.Add(search.NewTermQuery(index.NewTerm(prefixBoolField, "notexistnames")), search.SHOULD)
	assertHitCount(t, s, q, 2)
}
