// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestQueryRescorerWithSpans.java
//
// Tests the QueryRescorer integration with span queries.  Full index-based
// rescoring is deferred until RandomIndexWriter and IndexSearcher
// span-wiring are complete (backlog #2709); here we verify that the
// rescorer constructs correctly and handles basic edge cases without
// panicking.

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestQueryRescorerWithSpans_Basic(t *testing.T) {
	t.Parallel()

	t.Run("new_rescorer_with_span_term", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "hello"))
		rescorer := search.NewQueryRescorer(spanQ)
		if rescorer == nil {
			t.Fatal("expected non-nil Rescorer")
		}
		if rescorer.GetQuery() == nil {
			t.Fatal("rescorer query should not be nil")
		}
	})

	t.Run("new_rescorer_with_span_near", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanNearQuery(
			[]search.SpanQuery{
				search.NewSpanTermQuery(index.NewTerm("f", "a")),
				search.NewSpanTermQuery(index.NewTerm("f", "b")),
			},
			1,
			true,
		)
		rescorer := search.NewQueryRescorer(spanQ)
		if rescorer == nil {
			t.Fatal("expected non-nil Rescorer")
		}
	})

	t.Run("rescore_nil_topdocs_returns_nil", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "x"))
		rescorer := search.NewQueryRescorer(spanQ)
		result, err := rescorer.Rescore(nil, nil)
		if err != nil {
			t.Fatalf("Rescore: %v", err)
		}
		if result != nil {
			t.Error("expected nil result for nil input")
		}
	})

	t.Run("rescore_empty_topdocs", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "x"))
		rescorer := search.NewQueryRescorer(spanQ)
		topDocs := &search.TopDocs{
			TotalHits: search.NewTotalHits(0, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{},
		}
		result, err := rescorer.Rescore(nil, topDocs)
		if err != nil {
			t.Fatalf("Rescore: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result for empty input")
		}
		if result.TotalHits == nil || result.TotalHits.Value != 0 {
			t.Errorf("TotalHits = %+v; want Value=0", result.TotalHits)
		}
	})

	t.Run("rescore_with_custom_combine", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "x"))
		combine := func(first float32, matched bool, second float32) float32 {
			if matched {
				return first + second*2.0
			}
			return first
		}
		rescorer := search.NewQueryRescorerWithCombine(spanQ, combine)
		if rescorer == nil {
			t.Fatal("expected non-nil Rescorer")
		}
	})

	t.Run("rescore_preserves_single_hit", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "x"))
		rescorer := search.NewQueryRescorer(spanQ)
		hits := []*search.ScoreDoc{{Doc: 5, Score: 1.0}}
		topDocs := &search.TopDocs{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: hits,
		}
		// Without a real searcher, the hit passes through unchanged
		// (the combine fallback hits when scorer is nil).
		result, err := rescorer.Rescore(nil, topDocs)
		if err != nil {
			t.Fatalf("Rescore: %v", err)
		}
		if result == nil || len(result.ScoreDocs) != 1 {
			t.Fatalf("expected 1 hit; got %v", result)
		}
		if result.ScoreDocs[0].Doc != 5 {
			t.Errorf("doc = %d; want 5", result.ScoreDocs[0].Doc)
		}
	})
}
