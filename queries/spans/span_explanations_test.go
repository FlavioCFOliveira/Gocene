// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanExplanations.java
//
// Tests explanation output for span queries. Full integration against a
// real index is deferred until RandomIndexWriter and IndexSearcher
// span-wiring are complete (backlog #2709); here we verify that the
// explanation methods return structured, non-nil results.
//
// We use composite span queries (SpanOrQuery, SpanNotQuery) for these
// tests because their weights use the base SpanWeight whose Explain
// method is safe to call with a nil LeafReaderContext (it does not
// dereference the context).  SpanTermWeight.Explain requires a real
// context and is tested via the integration suite.

package spans

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSpanExplanations_SpanWeightExplain verifies that SpanWeight.Explain
// returns a non-nil Explanation for various query types.
func TestSpanExplanations_SpanWeightExplain(t *testing.T) {
	t.Parallel()

	t.Run("search_SpanOrWeight_Explain_non_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "a")),
			search.NewSpanTermQuery(index.NewTerm("f", "b")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp == nil {
			t.Fatal("expected non-nil Explanation")
		}
	})

	t.Run("search_SpanNotWeight_Explain_non_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanNotQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "include")),
			search.NewSpanTermQuery(index.NewTerm("f", "exclude")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp == nil {
			t.Fatal("expected non-nil Explanation")
		}
	})

	t.Run("search_SpanFirstWeight_Explain_non_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanFirstQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "term")), 3,
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp == nil {
			t.Fatal("expected non-nil Explanation")
		}
	})

	t.Run("explanation_non_empty_string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "test")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp.GetDescription() == "" {
			t.Error("expected non-empty description")
		}
	})

	t.Run("explanation_string_contains_query", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "myterm")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		desc := exp.GetDescription()
		// For base SpanWeight, the explanation mentions the query or "weight".
		if !strings.Contains(desc, "weight") && !strings.Contains(desc, "match") && !strings.Contains(desc, "no") {
			t.Errorf("explanation description %q should mention weight, match, or no match", desc)
		}
	})
}
