// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanExplanationsOfNonMatches.java
//
// Tests explanation output for non-matching span queries. Full integration
// against a real index is deferred until RandomIndexWriter and
// IndexSearcher span-wiring are complete (backlog #2709); here we verify
// that non-match explanations are structured, non-nil, and mention no
// match when appropriate.
//
// We use composite span queries (SpanOrQuery, SpanNotQuery) whose weights
// use the base SpanWeight whose Explain method is safe with nil context.

package spans

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSpanExplanationsOfNonMatches_Explain verifies that explanation
// methods return properly structured results for non-matching documents.
func TestSpanExplanationsOfNonMatches_Explain(t *testing.T) {
	t.Parallel()

	t.Run("explain_non_matching_doc_or_query", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "absent")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 999)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp == nil {
			t.Fatal("expected non-nil Explanation")
		}
	})

	t.Run("non_match_description_contains_no_match", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "ghost")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 5)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		desc := exp.GetDescription()
		if !strings.Contains(desc, "no") && !strings.Contains(desc, "No") {
			t.Errorf("non-match description %q should contain negation", desc)
		}
	})

	t.Run("search_SpanNotQuery_explain_non_match", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanNotQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "x")),
			search.NewSpanTermQuery(index.NewTerm("f", "y")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 100)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if exp == nil {
			t.Fatal("expected non-nil Explanation")
		}
	})

	t.Run("explain_no_panic_for_nil_context", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "term")),
		)
		w, err := q.CreateWeight(nil, true, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		exp, err := w.Explain(nil, 0)
		if err != nil {
			// No panic is the goal; any error is fine as long as we don't crash.
			t.Logf("Explain returned error (expected with nil context): %v", err)
		}
		if exp == nil && err == nil {
			t.Fatal("expected either explanation or error")
		}
		_ = exp // exp may be nil when there's an error
	})

	t.Run("explain_multiple_docs", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "notfound")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		// Call explain for a few different doc IDs — should not panic.
		for _, doc := range []int{0, 1, 2, 100, 1000} {
			exp, err := w.Explain(nil, doc)
			if err != nil {
				t.Logf("Explain doc %d: %v", doc, err)
				continue
			}
			if exp == nil {
				t.Errorf("nil explanation for doc %d", doc)
			}
		}
	})
}
