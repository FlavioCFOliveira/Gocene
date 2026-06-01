// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPhrasePrefixQuery.java
//
// Five tokenized "body" documents are indexed; a MultiPhraseQuery is then built
// by enumerating, through the committed reader's merged TermsEnum, the terms
// sharing the prefix "pi" ("piccadilly", "pie", "pizza") and adding them as the
// second position of the phrase. Exercising the full IndexWriter -> reader ->
// IndexSearcher path (rmp #18 / #123 / #124), the "blueberry" phrase must match
// exactly two documents and the "strawberry" phrase (no such term) must match
// none — identical to Lucene's assertions.
//
// Deviation from the reference, immaterial to the assertions: MockAnalyzer is
// replaced by the WhitespaceAnalyzer, which tokenizes "blueberry pie" into the
// position-aware terms "blueberry"@0 and "pie"@1 exactly as the test requires.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const phrasePrefixField = "body"

// termsProvider is the narrow accessor the merged DirectoryReader exposes for a
// field's terms; mirrors MultiTerms.getTerms(reader, field) in the reference.
type termsProvider interface {
	Terms(field string) (index.Terms, error)
}

// TestPhrasePrefixQuery_TestPhrasePrefixQuery ports testPhrasePrefix.
func TestPhrasePrefixQuery_TestPhrasePrefixQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addText(phrasePrefixField, "blueberry pie")         // doc 0
	ix.addText(phrasePrefixField, "blueberry strudel")     // doc 1
	ix.addText(phrasePrefixField, "blueberry pizza")       // doc 2
	ix.addText(phrasePrefixField, "blueberry chewing gum") // doc 3
	ix.addText(phrasePrefixField, "piccadilly circus")     // doc 4

	s, done := ix.searcher()
	defer done()

	// Enumerate the terms sharing the prefix "pi": this yields "piccadilly",
	// "pie" and "pizza" in byte order, mirroring the reference TermsEnum walk.
	termsWithPrefix := collectPrefixTerms(t, s, phrasePrefixField, "pi")
	if len(termsWithPrefix) != 3 {
		t.Fatalf("prefix %q: got %d terms %v, want 3 (piccadilly, pie, pizza)",
			"pi", len(termsWithPrefix), termTexts(termsWithPrefix))
	}

	// query1: "blueberry" followed by any of the pi* terms.
	query1 := search.NewMultiPhraseQueryBuilder()
	query1.Add(index.NewTerm(phrasePrefixField, "blueberry"))
	query1.AddTerms(termsWithPrefix)

	// query2: "strawberry" (no such term) followed by any of the pi* terms.
	query2 := search.NewMultiPhraseQueryBuilder()
	query2.Add(index.NewTerm(phrasePrefixField, "strawberry"))
	query2.AddTerms(termsWithPrefix)

	top1, err := s.Search(query1.Build(), 1000)
	if err != nil {
		t.Fatalf("search query1: %v", err)
	}
	if got := len(top1.ScoreDocs); got != 2 {
		t.Errorf("query1 (blueberry pi*): got %d hits, want 2", got)
	}

	top2, err := s.Search(query2.Build(), 1000)
	if err != nil {
		t.Fatalf("search query2: %v", err)
	}
	if got := len(top2.ScoreDocs); got != 0 {
		t.Errorf("query2 (strawberry pi*): got %d hits, want 0", got)
	}
}

// collectPrefixTerms walks the field's merged TermsEnum from the ceiling of
// prefix and collects every term that starts with prefix, stopping at the first
// term that does not. Mirrors the reference seekCeil/next loop over
// MultiTerms.getTerms(reader, field).iterator().
func collectPrefixTerms(t *testing.T, s *search.IndexSearcher, field, prefix string) []*index.Term {
	t.Helper()
	tp, ok := s.GetIndexReader().(termsProvider)
	if !ok {
		t.Fatalf("reader does not expose Terms(field)")
	}
	terms, err := tp.Terms(field)
	if err != nil {
		t.Fatalf("Terms(%q): %v", field, err)
	}
	if terms == nil {
		t.Fatalf("Terms(%q) returned nil", field)
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	cur, err := it.SeekCeil(index.NewTerm(field, prefix))
	if err != nil {
		t.Fatalf("SeekCeil(%q): %v", prefix, err)
	}
	var out []*index.Term
	for cur != nil {
		text := cur.Text()
		if !strings.HasPrefix(text, prefix) {
			break
		}
		out = append(out, index.NewTerm(field, text))
		cur, err = it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
	}
	return out
}

// termTexts is a diagnostic helper rendering a term slice's texts.
func termTexts(terms []*index.Term) []string {
	out := make([]string, len(terms))
	for i, term := range terms {
		out[i] = term.Text()
	}
	return out
}
