// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestPhrasePrefixQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// This class tests PhrasePrefixQuery class.
func TestPhrasePrefixQueryPhrasePrefix(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	doc1 := document.NewDocument()
	doc2 := document.NewDocument()
	doc3 := document.NewDocument()
	doc4 := document.NewDocument()
	doc5 := document.NewDocument()
	doc1.Add(newTextField(t, "body", "blueberry pie", true))
	doc2.Add(newTextField(t, "body", "blueberry strudel", true))
	doc3.Add(newTextField(t, "body", "blueberry pizza", true))
	doc4.Add(newTextField(t, "body", "blueberry chewing gum", true))
	doc5.Add(newTextField(t, "body", "piccadilly circus", true))
	mustAddDocument(t, writer, doc1)
	mustAddDocument(t, writer, doc2)
	mustAddDocument(t, writer, doc3)
	mustAddDocument(t, writer, doc4)
	mustAddDocument(t, writer, doc5)
	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, reader)

	// PhrasePrefixQuery query1 = new PhrasePrefixQuery();
	query1builder := search.NewMultiPhraseQueryBuilder()
	// PhrasePrefixQuery query2 = new PhrasePrefixQuery();
	query2builder := search.NewMultiPhraseQueryBuilder()
	query1builder.Add(index.NewTerm("body", "blueberry"))
	query2builder.Add(index.NewTerm("body", "strawberry"))

	var termsWithPrefix []*index.Term

	// this TermEnum gives "piccadilly", "pie" and "pizza".
	prefix := "pi"
	terms, err := index.MultiTermsGetTerms(reader, "body")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	te, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	if _, err := te.SeekCeil(index.NewTerm("body", prefix)); err != nil {
		t.Fatalf("seekCeil: %v", err)
	}
	for {
		s := te.Term().Text()
		if strings.HasPrefix(s, prefix) {
			termsWithPrefix = append(termsWithPrefix, index.NewTerm("body", s))
		} else {
			break
		}
		next, err := te.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if next == nil {
			break
		}
	}

	query1builder.AddTerms(append([]*index.Term(nil), termsWithPrefix...))
	query2builder.AddTerms(append([]*index.Term(nil), termsWithPrefix...))

	result := mustSearch(t, searcher, query1builder.Build(), 1000).ScoreDocs
	assertIntEquals(t, 2, len(result))

	result = mustSearch(t, searcher, query2builder.Build(), 1000).ScoreDocs
	assertIntEquals(t, 0, len(result))
	mustClose(t, reader, indexStore)
}
