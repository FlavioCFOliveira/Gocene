// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/TestSearch.java
//
// TestSearch performs a battery of Boolean / Phrase / Term searches over a small
// fixed corpus and checks that the results obtained from a multi-segment index
// match those obtained from a single-segment (force-merged) index. As the
// upstream comment notes, the test does not assert that the absolute ordering is
// "correct" — only that the multi-file and single-file layouts agree, which is
// exactly the cross-segment-vs-single-segment search invariant Gocene must hold.
package search_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testSearchDocs is the seven-document corpus from the upstream test. The
// whitespace analyzer used by the integration harness yields one token per
// space-separated word, matching the MockAnalyzer tokenization the upstream
// relies on for these single-letter terms.
var testSearchDocs = []string{
	"a b c d e",
	"a b c d e a b c d e",
	"a b c d e f g h i j",
	"a c e",
	"e c a",
	"a c e a c e",
	"a c e a b c",
}

// TestSearch_Search mirrors TestSearch.testSearch: it runs doTestSearch over a
// multi-segment index and a single-segment index and asserts the rendered output
// is identical.
func TestSearch_Search(t *testing.T) {
	multiFileOutput := doTestSearch(t, false)
	singleFileOutput := doTestSearch(t, true)
	if multiFileOutput != singleFileOutput {
		t.Errorf("multi-segment and single-segment search output differ:\n--- multi ---\n%s\n--- single ---\n%s",
			multiFileOutput, singleFileOutput)
	}
}

// doTestSearch indexes the corpus (committing after each document so the
// non-merged case is genuinely multi-segment), optionally force-merges to a
// single segment, then renders the hits for every query under a SCORE+INT(id)
// sort. The rendered string mirrors the upstream PrintWriter output closely
// enough to detect any divergence between the two layouts.
func doTestSearch(t *testing.T, forceMerge bool) string {
	t.Helper()
	ix := newIntegrationIndex(t)
	for j, text := range testSearchDocs {
		doc := document.NewDocument()
		f, err := document.NewTextField("contents", text, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		dv, err := document.NewNumericDocValuesField("id", int64(j))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(dv)
		ix.addDoc(doc)
		// Commit after every document so the non-force-merged index has one
		// segment per document, exercising the cross-segment search path.
		ix.commit()
	}
	if forceMerge {
		ix.forceMerge(1)
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	sort := search.NewSort(
		&search.SortField{Type: search.SortFieldTypeScore, Reverse: true},
		search.NewSortField("id", search.SortFieldTypeInt),
	)

	var out strings.Builder
	for _, q := range buildTestSearchQueries() {
		top, err := searcher.SearchWithSort(q, 1000, sort)
		if err != nil {
			t.Fatalf("SearchWithSort(%v): %v", q, err)
		}
		fmt.Fprintf(&out, "%d total results\n", len(top.ScoreDocs))
		for i := 0; i < len(top.ScoreDocs) && i < 10; i++ {
			sd := top.ScoreDocs[i]
			doc, err := searcher.Doc(sd.Doc)
			if err != nil {
				t.Fatalf("Doc(%d): %v", sd.Doc, err)
			}
			contents := ""
			if f := doc.Get("contents"); f != nil {
				contents = f.StringValue()
			}
			fmt.Fprintf(&out, "%d %s\n", i, contents)
		}
	}
	return out.String()
}

// buildTestSearchQueries mirrors TestSearch.buildQueries: SHOULD(a,b),
// phrase(a b), phrase(a b c), SHOULD(a,c), phrase(a c), phrase(a c e).
func buildTestSearchQueries() []search.Query {
	term := func(t string) *index.Term { return index.NewTerm("contents", t) }

	booleanAB := search.NewBooleanQuery()
	booleanAB.Add(search.NewTermQuery(term("a")), search.SHOULD)
	booleanAB.Add(search.NewTermQuery(term("b")), search.SHOULD)

	phraseAB := search.NewPhraseQuery("contents", term("a"), term("b"))
	phraseABC := search.NewPhraseQuery("contents", term("a"), term("b"), term("c"))

	booleanAC := search.NewBooleanQuery()
	booleanAC.Add(search.NewTermQuery(term("a")), search.SHOULD)
	booleanAC.Add(search.NewTermQuery(term("c")), search.SHOULD)

	phraseAC := search.NewPhraseQuery("contents", term("a"), term("c"))
	phraseACE := search.NewPhraseQuery("contents", term("a"), term("c"), term("e"))

	return []search.Query{
		booleanAB, phraseAB, phraseABC, booleanAC, phraseAC, phraseACE,
	}
}
