// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanSimilarity.java
//
// Tests that span query weights can be created without panics and that
// basic scoring infrastructure (SpanWeight, SpanScorer) does not panic
// when invoked.  Full similarity integration (BM25, ClassicSimilarity,
// BooleanSimilarity) against a real index is deferred until
// RandomIndexWriter and IndexSearcher span-wiring are complete.

package spans

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestSpanSimilarity_WeightCreation verifies that CreateWeight and
// CreateSpanWeight return non-nil weights that do not panic.
func TestSpanSimilarity_WeightCreation(t *testing.T) {
	t.Parallel()

	t.Run("SpanTermQuery_CreateWeight", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "term"))
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		if w == nil {
			t.Fatal("expected non-nil Weight")
		}
	})

	t.Run("SpanTermQuery_CreateSpanWeight", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "term"))
		sw, err := q.CreateSpanWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateSpanWeight: %v", err)
		}
		if sw == nil {
			t.Fatal("expected non-nil SpanWeight")
		}
	})

	t.Run("SpanNearQuery_CreateWeight", func(t *testing.T) {
		t.Parallel()
		q := NewOrderedNearQuery("f").
			AddClause(NewSpanTermQuery(index.NewTerm("f", "a"))).
			AddClause(NewSpanTermQuery(index.NewTerm("f", "b"))).
			SetSlop(0).
			Build()
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		if w == nil {
			t.Fatal("expected non-nil Weight")
		}
	})

	t.Run("search_SpanOrQuery_Weight", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "a")),
			search.NewSpanTermQuery(index.NewTerm("f", "b")),
		)
		w, err := q.CreateWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateWeight: %v", err)
		}
		if w == nil {
			t.Fatal("expected non-nil Weight")
		}
	})

	t.Run("SpanWeight_IsCacheable", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "t"))
		sw, err := q.CreateSpanWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateSpanWeight: %v", err)
		}
		if !sw.IsCacheable(nil) {
			t.Error("expected IsCacheable=true")
		}
	})
}

// TestSpanSimilarity_NoNegativeInfNaN verifies that span scoring does not
// produce negative, infinite, or NaN scores for basic cases (port of
// Lucene's TestSpanSimilarity which tests that absent-term queries don't
// produce bad scores).
func TestSpanSimilarity_NoNegativeInfNaN(t *testing.T) {
	t.Parallel()

	// SpanScorer with zero positions should have freq=0 → score=0.
	t.Run("empty_spans_scorer_freq_zero", func(t *testing.T) {
		t.Parallel()
		emptySpans := search.NewSpans(nil, nil, nil)
		scorer := search.NewSpanScorer(emptySpans, 1.0)
		score := scorer.Score()
		if score < 0 || math.IsInf(float64(score), 0) || math.IsNaN(float64(score)) {
			t.Errorf("bad score: %v", score)
		}
	})

	// SpanWeight.GetValue should return 1.0.
	t.Run("span_weight_get_value", func(t *testing.T) {
		t.Parallel()
		sw := search.NewSpanWeight(nil, nil)
		v := sw.GetValue()
		if v != 1.0 {
			t.Errorf("GetValue = %f; want 1.0", v)
		}
	})

	// SpanWeight.Count returns -1 (placeholder).
	t.Run("span_weight_count", func(t *testing.T) {
		t.Parallel()
		sw := search.NewSpanWeight(nil, nil)
		c, err := sw.Count(nil)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if c != -1 {
			t.Errorf("Count = %d; want -1", c)
		}
	})

	// SpanScorer sloppy frequency: for TermSpans width=0 each occurrence
	// contributes 1/(1+0)=1.0 to the frequency. With 3 occurrences on one doc
	// each occurrence contributes 1.0 to freq, matching the Lucene formula
	// freq += 1/(1+width) = 1/(1+0) = 1.0.
	t.Run("scorer_sloppy_freq_term_spans", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {0, 2, 5}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("t"))}
		sp := NewTermSpans(pe, term, 128.0)
		// The SpanScorer requires a SimScorer to compute frequencies;
		// without one setFreqCurrentDoc short-circuits to freq=1.
		// Use a simple simScorer to verify the computation path.
		sim := search.NewClassicSimilarity()
		simScorer := sim.Scorer(
			search.NewCollectionStatistics("f", 1, 1, -1, -1),
			search.NewTermStatistics(&term, 1, 3),
		)
		sc := newSpanScorer(sp, simScorer, nil)
		doc, err := sc.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		freq, err := sc.sloppyFreq()
		if err != nil {
			t.Fatalf("sloppyFreq: %v", err)
		}
		// 3 occurrences × 1.0 (width=0) = 3.0
		if freq != 3.0 {
			t.Errorf("sloppyFreq = %f; want 3.0 (3 spans x 1.0 each)", freq)
		}
		// Score is computed as simScorer.Score(doc, freq); with IDF=0 for
		// a single-doc collection the score may be 0 — what matters is that
		// the sloppy frequency (3.0) matches the Lucene formula.
	})

	// Verify scoring with a single TermSpans position: width=0 → contribution 1.0,
	// and the final score is simScorer.Score(doc, freq) where freq=1.0.
	t.Run("scorer_single_position", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {7}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("t"))}
		sp := NewTermSpans(pe, term, 128.0)
		sim := search.NewClassicSimilarity()
		simScorer := sim.Scorer(
			search.NewCollectionStatistics("f", 1, 1, -1, -1),
			search.NewTermStatistics(&term, 1, 1),
		)
		sc := newSpanScorer(sp, simScorer, nil)
		doc, err := sc.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		// A single position with width=0 → freq = 1.0
		freq, err := sc.sloppyFreq()
		if err != nil {
			t.Fatalf("sloppyFreq: %v", err)
		}
		if freq != 1.0 {
			t.Errorf("sloppyFreq = %f; want 1.0", freq)
		}
	})

	// Verify GapSpans.Width() returns the gap width (used in formula 1/(1+width)).
	t.Run("gap_spans_width_returns_gap", func(t *testing.T) {
		t.Parallel()
		gs := NewGapSpans(3)
		if gs.Width() != 3 {
			t.Errorf("Width = %d; want 3", gs.Width())
		}
		gs2 := NewGapSpans(0)
		if gs2.Width() != 0 {
			t.Errorf("Width = %d; want 0", gs2.Width())
		}
	})
}
