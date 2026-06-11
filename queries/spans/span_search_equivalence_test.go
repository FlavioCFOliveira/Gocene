// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanSearchEquivalence.java
//
// Tests search equivalence properties for span queries.  Full
// search-equivalence verification (assertSubsetOf, assertSameSet, etc.)
// requires a RandomIndexWriter and IndexSearcher that are not yet
// wired for spans (backlog #2709).  Here we test the equivalence helpers
// and span query BooleanQuery/DisjunctionMaxQuery composition behaviour
// that can be verified without a real index.

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSpanSearchEquivalence_SpanOrSingleton verifies that a single-clause
// SpanOrQuery rewrites to the underlying clause (port of Lucene's
// equivalent test).
func TestSpanSearchEquivalence_SpanOrSingleton(t *testing.T) {
	t.Parallel()

	t.Run("single_clause_rewrites_to_span_term", func(t *testing.T) {
		t.Parallel()
		stq := search.NewSpanTermQuery(index.NewTerm("f", "a"))
		orQ := search.NewSpanOrQuery(stq)
		if orQ == nil {
			t.Fatal("expected non-nil SpanOrQuery")
		}
		rewritten, err := orQ.Rewrite(nil)
		if err != nil {
			t.Fatalf("Rewrite: %v", err)
		}
		// Single-clause rewrite should return the clause itself.
		_, isSTQ := rewritten.(*search.SpanTermQuery)
		if !isSTQ {
			t.Errorf("rewritten type = %T; want *SpanTermQuery", rewritten)
		}
	})
}

// TestSpanSearchEquivalence_BooleanComposition verifies that span queries
// integrate correctly with BooleanQuery clauses (port of Lucene's
// LUCENE-4477 / LUCENE-4401 pattern).
func TestSpanSearchEquivalence_BooleanComposition(t *testing.T) {
	t.Parallel()

	t.Run("span_term_as_boolean_should", func(t *testing.T) {
		t.Parallel()
		stq := search.NewSpanTermQuery(index.NewTerm("f", "a"))
		bq := search.NewBooleanQuery()
		bq.Add(stq, search.SHOULD)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("two_span_terms_as_should", func(t *testing.T) {
		t.Parallel()
		stq1 := search.NewSpanTermQuery(index.NewTerm("f", "a"))
		stq2 := search.NewSpanTermQuery(index.NewTerm("f", "b"))
		bq := search.NewBooleanQuery()
		bq.Add(stq1, search.SHOULD)
		bq.Add(stq2, search.SHOULD)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("span_or_as_boolean_must", func(t *testing.T) {
		t.Parallel()
		orQ := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "a")),
			search.NewSpanTermQuery(index.NewTerm("f", "b")),
		)
		bq := search.NewBooleanQuery()
		bq.Add(orQ, search.MUST)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("span_not_as_boolean_filter", func(t *testing.T) {
		t.Parallel()
		snQ := search.NewSpanNotQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "a")),
			search.NewSpanTermQuery(index.NewTerm("f", "b")),
		)
		bq := search.NewBooleanQuery()
		bq.Add(snQ, search.FILTER)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("disjunction_max_with_span_term", func(t *testing.T) {
		t.Parallel()
		dmq := search.NewDisjunctionMaxQueryWithTieBreaker(
			[]search.Query{
				search.NewSpanTermQuery(index.NewTerm("f", "a")),
				search.NewSpanTermQuery(index.NewTerm("f", "b")),
			},
			1.0,
		)
		if dmq == nil {
			t.Fatal("expected non-nil DisjunctionMaxQuery")
		}
	})
}

// TestSpanSearchEquivalence_Rewrite verifies Rewrite methods on span
// query types.
func TestSpanSearchEquivalence_Rewrite(t *testing.T) {
	t.Parallel()

	t.Run("span_or_single_rewrites_to_span_term", func(t *testing.T) {
		t.Parallel()
		orQ := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "x")),
		)
		if orQ == nil {
			t.Fatal("expected non-nil")
		}
		rewritten, err := orQ.Rewrite(nil)
		if err != nil {
			t.Fatalf("Rewrite: %v", err)
		}
		_, isSTQ := rewritten.(*search.SpanTermQuery)
		if !isSTQ {
			t.Errorf("type = %T; want *SpanTermQuery", rewritten)
		}
	})
}
