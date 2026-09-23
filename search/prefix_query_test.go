// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestPrefixQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestPrefixQueryPrefixQuery(t *testing.T) {
	directory := newDirectory()

	categories := []string{"/Computers", "/Computers/Mac", "/Computers/Windows"}
	writer := newRandomIndexWriter(t, directory)
	for i := 0; i < len(categories); i++ {
		doc := newTestDocument(newStringField(t, "category", categories[i], true))
		mustAddDocument(t, writer, doc)
	}
	reader := mustGetReader(t, writer)

	query := search.NewPrefixQuery(index.NewTerm("category", "/Computers"))
	searcher := newSearcher(t, reader)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 3 {
		t.Fatalf("All documents in /Computers category and below: expected 3, got %d", len(hits))
	}

	query = search.NewPrefixQuery(index.NewTerm("category", "/Computers/Mac"))
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 1 {
		t.Fatalf("One in /Computers/Mac: expected 1, got %d", len(hits))
	}

	query = search.NewPrefixQuery(index.NewTerm("category", ""))
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 3 {
		t.Fatalf("everything: expected 3, got %d", len(hits))
	}
	mustClose(t, writer, reader, directory)
}

func TestPrefixQueryMatchAll(t *testing.T) {
	directory := newDirectory()

	writer := newRandomIndexWriter(t, directory)
	doc := newTestDocument(newStringField(t, "field", "field", true))
	mustAddDocument(t, writer, doc)

	reader := mustGetReader(t, writer)

	query := search.NewPrefixQuery(index.NewTerm("field", ""))
	searcher := newSearcher(t, reader)

	if got := mustSearch(t, searcher, query, 1000).TotalHits.Value; got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	mustClose(t, writer, reader, directory)
}

func TestPrefixQueryRandomBinaryPrefix(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	numTerms := atLeast(1000)
	terms := map[string]struct{}{}
	for len(terms) < numTerms {
		bytes := make([]byte, nextInt(1, 10))
		random().Read(bytes)
		terms[string(bytes)] = struct{}{}
	}

	termsList := make([]*util.BytesRef, 0, len(terms))
	for term := range terms {
		termsList = append(termsList, util.NewBytesRef([]byte(term)))
	}
	r0 := random()
	r0.Shuffle(len(termsList), func(i, j int) { termsList[i], termsList[j] = termsList[j], termsList[i] })
	for _, term := range termsList {
		doc := newTestDocument(newStringFieldBytes(t, "field", term.ValidBytes(), false))
		mustAddDocument(t, w, doc)
	}

	r := mustGetReader(t, w)
	s := newSearcher(t, r)

	iters := atLeast(100)
	for iter := 0; iter < iters; iter++ {
		bytes := make([]byte, random().Intn(3))
		random().Read(bytes)
		prefix := util.NewBytesRef(bytes)
		q := search.NewPrefixQuery(index.NewTermFromBytesRef("field", prefix))
		count := 0
		for _, term := range termsList {
			if util.StartsWith(term, prefix) {
				count++
			}
		}
		if got := mustCount(t, s, q); got != count {
			t.Fatalf("expected %d, got %d", count, got)
		}
	}
	mustClose(t, r, w, dir)
}
