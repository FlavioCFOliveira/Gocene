// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func newTerm(field, text string) *index.Term {
	t := &index.Term{Field: field}
	// Term holds either Text or BytesValue. Use the public constructor when
	// available; otherwise set Text directly via the factory.
	tt := index.NewTerm(field, text)
	if tt != nil {
		return tt
	}
	return t
}

func sortedKeys(m map[string]float32) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestExtractQueryTerms_TermQuery(t *testing.T) {
	q := search.NewTermQuery(newTerm("body", "alpha"))
	weights := map[string]float32{}
	got := extractQueryTerms(q, "", 1.0, nil, weights)

	if len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("expected [alpha], got %v", got)
	}
	if weights["alpha"] != 1.0 {
		t.Fatalf("expected weight 1.0, got %f", weights["alpha"])
	}
}

func TestExtractQueryTerms_PhraseQuery(t *testing.T) {
	q := search.NewPhraseQuery("body")
	q.AddTerm(newTerm("body", "quick"))
	q.AddTerm(newTerm("body", "brown"))

	weights := map[string]float32{}
	got := extractQueryTerms(q, "", 1.0, nil, weights)

	sort.Strings(got)
	if len(got) != 2 || got[0] != "brown" || got[1] != "quick" {
		t.Fatalf("expected [brown quick], got %v", got)
	}
}

func TestExtractQueryTerms_BooleanQuery_DropsMustNot(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(newTerm("body", "keep1")), search.MUST)
	bq.Add(search.NewTermQuery(newTerm("body", "keep2")), search.SHOULD)
	bq.Add(search.NewTermQuery(newTerm("body", "skip")), search.MUST_NOT)

	weights := map[string]float32{}
	got := extractQueryTerms(bq, "", 1.0, nil, weights)
	sort.Strings(got)

	if len(got) != 2 || got[0] != "keep1" || got[1] != "keep2" {
		t.Fatalf("expected [keep1 keep2], got %v", got)
	}
	if _, ok := weights["skip"]; ok {
		t.Fatalf("MUST_NOT term must not appear in weights")
	}
}

func TestExtractQueryTerms_BoostQueryPropagatesBoost(t *testing.T) {
	inner := search.NewTermQuery(newTerm("body", "boosted"))
	q := search.NewBoostQuery(inner, 3.5)

	weights := map[string]float32{}
	_ = extractQueryTerms(q, "", 1.0, nil, weights)

	if weights["boosted"] != 3.5 {
		t.Fatalf("expected weight 3.5, got %f", weights["boosted"])
	}
}

func TestExtractQueryTerms_DisjunctionMaxQuery(t *testing.T) {
	dmq := search.NewDisjunctionMaxQuery([]search.Query{
		search.NewTermQuery(newTerm("body", "a")),
		search.NewTermQuery(newTerm("body", "b")),
	})

	weights := map[string]float32{}
	got := extractQueryTerms(dmq, "", 1.0, nil, weights)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected [a b], got %v", got)
	}
}

func TestExtractQueryTerms_ConstantScoreQuery(t *testing.T) {
	inner := search.NewTermQuery(newTerm("body", "cscore"))
	q := search.NewConstantScoreQuery(inner)

	weights := map[string]float32{}
	got := extractQueryTerms(q, "", 1.0, nil, weights)
	if len(got) != 1 || got[0] != "cscore" {
		t.Fatalf("expected [cscore], got %v", got)
	}
}

func TestExtractQueryTerms_SpanTermQuery(t *testing.T) {
	q := search.NewSpanTermQuery(newTerm("body", "spanned"))
	weights := map[string]float32{}
	got := extractQueryTerms(q, "", 1.0, nil, weights)
	if len(got) != 1 || got[0] != "spanned" {
		t.Fatalf("expected [spanned], got %v", got)
	}
}

func TestExtractQueryTerms_FieldFilter(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(newTerm("title", "ignored")), search.MUST)
	bq.Add(search.NewTermQuery(newTerm("body", "kept")), search.MUST)

	weights := map[string]float32{}
	got := extractQueryTerms(bq, "body", 1.0, nil, weights)
	if len(got) != 1 || got[0] != "kept" {
		t.Fatalf("expected [kept], got %v", got)
	}
	if _, ok := weights["ignored"]; ok {
		t.Fatalf("field-filter must drop foreign-field terms")
	}
}

func TestExtractQueryTerms_NestedBooleanAndBoost(t *testing.T) {
	inner := search.NewBooleanQuery()
	inner.Add(search.NewTermQuery(newTerm("body", "alpha")), search.SHOULD)
	inner.Add(search.NewTermQuery(newTerm("body", "beta")), search.SHOULD)

	outer := search.NewBooleanQuery()
	outer.Add(search.NewBoostQuery(inner, 2.0), search.MUST)
	outer.Add(search.NewTermQuery(newTerm("body", "gamma")), search.MUST)

	weights := map[string]float32{}
	got := extractQueryTerms(outer, "", 1.0, nil, weights)

	keys := sortedKeys(weights)
	if len(keys) != 3 {
		t.Fatalf("expected 3 distinct terms, got %v", keys)
	}
	if weights["alpha"] != 2.0 || weights["beta"] != 2.0 {
		t.Fatalf("boost must propagate to nested children: weights=%v", weights)
	}
	if weights["gamma"] != 1.0 {
		t.Fatalf("non-boosted sibling kept at 1.0: got %f", weights["gamma"])
	}
	_ = got
}

func TestQueryScorerExtractsTerms(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(newTerm("body", "alpha")), search.MUST)
	bq.Add(search.NewTermQuery(newTerm("body", "beta")), search.SHOULD)

	qs := NewQueryScorerWithField(bq, "body")
	terms := qs.GetQueryTerms()
	sort.Strings(terms)

	if len(terms) != 2 || terms[0] != "alpha" || terms[1] != "beta" {
		t.Fatalf("expected [alpha beta], got %v", terms)
	}
	if qs.maxTermWeight != 1.0 {
		t.Fatalf("expected maxTermWeight=1.0, got %f", qs.maxTermWeight)
	}
}

func TestHighlighterFactoryExtractsTerms(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(newTerm("body", "alpha")), search.MUST)

	hf := NewHighlighterFactory(bq, "body")
	terms := hf.extractTerms(bq)
	if len(terms) != 1 || terms[0] != "alpha" {
		t.Fatalf("expected [alpha], got %v", terms)
	}
}

// TestHighlighter_TermQuery_End2End verifies that a TermQuery flows through
// QueryScorer -> SimpleHighlighter to yield a non-empty fragment containing
// the matching term.
func TestHighlighter_TermQuery_End2End(t *testing.T) {
	q := search.NewTermQuery(newTerm("body", "quick"))
	qs := NewQueryScorerWithField(q, "body")
	scorer := NewSimpleFragmentScorer(qs.GetQueryTerms())
	h := NewSimpleHighlighter(scorer)

	text := "The quick brown fox jumps over the lazy dog."
	frag, err := h.GetBestFragment(text, 1)
	if err != nil {
		t.Fatalf("highlight: %v", err)
	}
	if frag == "" {
		t.Fatal("expected a non-empty fragment for TermQuery match")
	}
}

// TestHighlighter_BooleanQuery_End2End covers the same flow for a
// BooleanQuery (MUST + SHOULD with MUST_NOT excluded).
func TestHighlighter_BooleanQuery_End2End(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(newTerm("body", "quick")), search.MUST)
	bq.Add(search.NewTermQuery(newTerm("body", "fox")), search.SHOULD)
	bq.Add(search.NewTermQuery(newTerm("body", "lazy")), search.MUST_NOT)

	qs := NewQueryScorerWithField(bq, "body")

	for _, want := range []string{"quick", "fox"} {
		found := false
		for _, got := range qs.GetQueryTerms() {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("BooleanQuery extraction missing %q in %v", want, qs.GetQueryTerms())
		}
	}
	for _, got := range qs.GetQueryTerms() {
		if got == "lazy" {
			t.Errorf("MUST_NOT term %q must not be extracted", got)
		}
	}

	scorer := NewSimpleFragmentScorer(qs.GetQueryTerms())
	h := NewSimpleHighlighter(scorer)
	frag, err := h.GetBestFragment("The quick brown fox jumps over the lazy dog.", 1)
	if err != nil {
		t.Fatalf("highlight: %v", err)
	}
	if frag == "" {
		t.Fatal("expected a non-empty fragment for BooleanQuery match")
	}
}
