// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestPrefixInBooleanQuery.java
// (Apache Lucene 10.5.0).
//
// https://issues.apache.org/jira/browse/LUCENE-1974
//
// represent the bug of
//
// BooleanScorer.score(Collector collector, int max, int firstDocID)
//
// Line 273, end=8192, subScorerDocID=11378, then more got false?
//
// The @BeforeClass index is rebuilt for every test (pibqBeforeClass); every
// test reads it only, so the data each test sees is the same.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const pibqField = "name"

// pibqBeforeClass renders beforeClass(); afterClass is registered with
// t.Cleanup.
func pibqBeforeClass(t *testing.T) *search.IndexSearcher {
	t.Helper()
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)

	doc := document.NewDocument()
	field := newStringField(t, pibqField, "meaninglessnames", false)
	doc.Add(field)

	for i := 0; i < 5137; i++ {
		mustAddDocument(t, writer, doc)
	}

	field.SetStringValue("tangfulin")
	mustAddDocument(t, writer, doc)

	field.SetStringValue("meaninglessnames")
	for i := 5138; i < 11377; i++ {
		mustAddDocument(t, writer, doc)
	}

	field.SetStringValue("tangfulin")
	mustAddDocument(t, writer, doc)

	reader := mustGetReader(t, writer)
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		if err := directory.Close(); err != nil {
			t.Errorf("close directory: %v", err)
		}
	})
	searcher := newSearcher(t, reader)
	mustClose(t, writer)
	return searcher
}

func TestPrefixInBooleanQueryPrefixQuery(t *testing.T) {
	searcher := pibqBeforeClass(t)
	query := search.NewPrefixQuery(index.NewTerm(pibqField, "tang"))
	if got := mustSearch(t, searcher, query, 1000).TotalHits.Value; got != 2 {
		t.Fatalf("Number of matched documents: expected 2, got %d", got)
	}
}

func TestPrefixInBooleanQueryTermQuery(t *testing.T) {
	searcher := pibqBeforeClass(t)
	query := search.NewTermQuery(index.NewTerm(pibqField, "tangfulin"))
	if got := mustSearch(t, searcher, query, 1000).TotalHits.Value; got != 2 {
		t.Fatalf("Number of matched documents: expected 2, got %d", got)
	}
}

func TestPrefixInBooleanQueryTermBooleanQuery(t *testing.T) {
	searcher := pibqBeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(search.NewTermQuery(index.NewTerm(pibqField, "tangfulin")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm(pibqField, "notexistnames")), search.SHOULD)
	if got := mustSearch(t, searcher, query.Build(), 1000).TotalHits.Value; got != 2 {
		t.Fatalf("Number of matched documents: expected 2, got %d", got)
	}
}

func TestPrefixInBooleanQueryPrefixBooleanQuery(t *testing.T) {
	searcher := pibqBeforeClass(t)
	query := search.NewBooleanQueryBuilder()
	query.Add(search.NewPrefixQuery(index.NewTerm(pibqField, "tang")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm(pibqField, "notexistnames")), search.SHOULD)
	if got := mustSearch(t, searcher, query.Build(), 1000).TotalHits.Value; got != 2 {
		t.Fatalf("Number of matched documents: expected 2, got %d", got)
	}
}
