// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// explainTestIndex builds a single-segment index from the supplied per-document
// field values (one text field named "field" per document) and returns an
// open searcher together with the sole leaf context. The caller is responsible
// for closing the returned reader via the cleanup registered on t.
func explainTestIndex(t *testing.T, values []string) (*search.IndexSearcher, *index.LeafReaderContext) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, value := range values {
		doc := document.NewDocument()
		field, err := document.NewTextField("field", value, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected a single leaf, got %d", len(leaves))
	}

	return search.NewIndexSearcher(reader), leaves[0]
}

// scoreOfDoc runs query through searcher and returns the score recorded for the
// given leaf-local doc id. It fails the test if the doc is absent from the
// result set.
func scoreOfDoc(t *testing.T, searcher *search.IndexSearcher, query search.Query, doc int) float32 {
	t.Helper()
	topDocs, err := searcher.Search(query, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, sd := range topDocs.ScoreDocs {
		if sd.Doc == doc {
			return sd.Score
		}
	}
	t.Fatalf("doc %d not present in search results", doc)
	return 0
}

// assertExplainMatchesScore asserts that the Explanation for a matching doc is a
// match whose value equals the scored value to within a small tolerance, and
// that it carries a non-empty description.
func assertExplainMatchesScore(t *testing.T, exp search.Explanation, err error, want float32) {
	t.Helper()
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if exp == nil {
		t.Fatal("Explain returned nil explanation")
	}
	if !exp.IsMatch() {
		t.Fatalf("expected IsMatch()==true, got false (desc=%q)", exp.GetDescription())
	}
	if exp.GetDescription() == "" {
		t.Error("expected a non-empty explanation description")
	}
	if !floatsClose(exp.GetValue(), want) {
		t.Errorf("explanation value = %v, want score %v", exp.GetValue(), want)
	}
}

// assertExplainNoMatch asserts that the Explanation reports a non-match.
func assertExplainNoMatch(t *testing.T, exp search.Explanation, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if exp == nil {
		t.Fatal("Explain returned nil explanation")
	}
	if exp.IsMatch() {
		t.Errorf("expected IsMatch()==false, got true (value=%v, desc=%q)",
			exp.GetValue(), exp.GetDescription())
	}
}

// floatsClose reports whether a and b are within a small absolute tolerance.
func floatsClose(a, b float32) bool {
	const eps = 1e-4
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

// TestTermWeight_Explain verifies TermWeight.Explain produces a match whose
// value equals the scored value for a matching document and a non-match for a
// document the term does not occur in.
func TestTermWeight_Explain(t *testing.T) {
	// Docs: 0:"all" 1:"dogs" 2:"like" 3:"all" 4:"fetch"
	searcher, leaf := explainTestIndex(t, []string{"all", "dogs", "like", "all", "fetch"})

	term := index.NewTerm("field", "all")
	query := search.NewTermQuery(term)
	weight, err := query.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	// Matching doc 0.
	want := scoreOfDoc(t, searcher, query, 0)
	exp, err := weight.Explain(leaf, 0)
	assertExplainMatchesScore(t, exp, err, want)

	// Non-matching doc 1 ("dogs" only).
	exp, err = weight.Explain(leaf, 1)
	assertExplainNoMatch(t, exp, err)
}

// TestBooleanWeight_Explain verifies BooleanWeight.Explain combines clause
// explanations and reports a match whose value equals the scored value, and a
// non-match for a document satisfying none of the optional clauses.
func TestBooleanWeight_Explain(t *testing.T) {
	// Docs: 0:"all dogs" 1:"all" 2:"like" 3:"cat"
	searcher, leaf := explainTestIndex(t, []string{"all dogs", "all", "like", "cat"})

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "all")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "dogs")), search.SHOULD)

	weight, err := bq.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	// Doc 0 matches both SHOULD clauses.
	want := scoreOfDoc(t, searcher, bq, 0)
	exp, err := weight.Explain(leaf, 0)
	assertExplainMatchesScore(t, exp, err, want)
	if len(exp.GetDetails()) == 0 {
		t.Error("expected per-clause sub-explanations for a boolean match")
	}

	// Doc 1 matches a single SHOULD clause ("all").
	want = scoreOfDoc(t, searcher, bq, 1)
	exp, err = weight.Explain(leaf, 1)
	assertExplainMatchesScore(t, exp, err, want)

	// Doc 3 ("cat") matches no clause.
	exp, err = weight.Explain(leaf, 3)
	assertExplainNoMatch(t, exp, err)
}

// TestBooleanWeight_Explain_MustNot verifies that a MUST_NOT clause matching a
// document forces a non-match explanation.
func TestBooleanWeight_Explain_MustNot(t *testing.T) {
	// Docs: 0:"all dogs" 1:"all cat"
	searcher, leaf := explainTestIndex(t, []string{"all dogs", "all cat"})

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "dogs")), search.MUST_NOT)

	weight, err := bq.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	// Doc 0 contains the prohibited term "dogs" -> non-match.
	exp, err := weight.Explain(leaf, 0)
	assertExplainNoMatch(t, exp, err)

	// Doc 1 contains "all" and not "dogs" -> match.
	want := scoreOfDoc(t, searcher, bq, 1)
	exp, err = weight.Explain(leaf, 1)
	assertExplainMatchesScore(t, exp, err, want)
}
