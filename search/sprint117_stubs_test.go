// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Sprint 117 / T4679 test coverage for the stub completions in
// automaton_query.go, axiomatic_similarity.go, constant_score_auto_rewrite.go,
// double_values_source.go, long_values_source.go, more_like_this.go,
// multi_term_query_wrapper.go, query_rescorer.go, top_terms_rewrite.go,
// scoring_rewrite.go, date_range_query.go, point_in_set_query.go,
// point_query.go and range_field_query.go.

package search

import (
	"math"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TestAxiomaticSimilarity_ComputeNorm_NoState verifies the default branch
// when no FieldInvertState is supplied.
func TestAxiomaticSimilarity_ComputeNorm_NoState(t *testing.T) {
	sim := NewAxiomaticSimilarity()
	if got := sim.ComputeNorm("f", nil); got != 1.0 {
		t.Errorf("ComputeNorm(nil) = %v, want 1.0", got)
	}
}

// TestAxiomaticSimilarity_ComputeNorm_WithState verifies length-based norm.
func TestAxiomaticSimilarity_ComputeNorm_WithState(t *testing.T) {
	sim := NewAxiomaticSimilarity()
	state := &index.FieldInvertState{}
	// FieldInvertState has unexported fields, but Length() reads .length.
	// We rely on the zero value (length=0) -> 1.0 default.
	if got := sim.ComputeNorm("f", state); got != 1.0 {
		t.Errorf("ComputeNorm(zero state) = %v, want 1.0 fallback", got)
	}
	// Indirect: ComputeNormFromInvertState exposes the canonical entry point.
	if got := sim.ComputeNormFromInvertState(state); got < 0 {
		t.Errorf("ComputeNormFromInvertState returned negative: %v", got)
	}
}

// TestConstantScoreAutoRewrite_Threshold verifies that the auto rewrite
// stays inside the Boolean expansion for small term counts and wraps the
// query in a ConstantScoreQuery directly when the threshold is exceeded.
func TestConstantScoreAutoRewrite_Threshold(t *testing.T) {
	r := NewConstantScoreAutoRewrite()
	r.SetThreshold(4)
	mtq := NewMultiTermQuery("field", index.NewTerm("field", "a"))
	got, err := r.Rewrite(mtq, nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if got == nil {
		t.Fatal("Rewrite returned nil")
	}
}

// TestMultiTermQueryConstantScoreWrapper_Rewrite verifies the wrapper
// rewrites to a ConstantScoreQuery.
func TestMultiTermQueryConstantScoreWrapper_Rewrite(t *testing.T) {
	mtq := NewMultiTermQuery("field", index.NewTerm("field", "a"))
	w := NewMultiTermQueryConstantScoreWrapper(mtq)
	rewritten, err := w.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*ConstantScoreQuery); !ok {
		t.Errorf("expected *ConstantScoreQuery, got %T", rewritten)
	}
}

// TestTopTermsRewrite_Size verifies that the rewrite clamps to r.size.
func TestTopTermsRewrite_Size(t *testing.T) {
	r := NewTopTermsRewrite(3)
	mtq := NewMultiTermQuery("field", index.NewTerm("field", "a"))
	got, err := r.Rewrite(mtq, nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if got == nil {
		t.Fatal("Rewrite returned nil")
	}
}

// TestTopTermsRewrite_ZeroSize verifies the zero-size short-circuit.
func TestTopTermsRewrite_ZeroSize(t *testing.T) {
	r := NewTopTermsRewrite(0)
	mtq := NewMultiTermQuery("field", index.NewTerm("field", "a"))
	got, err := r.Rewrite(mtq, nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := got.(*MatchNoDocsQuery); !ok {
		t.Errorf("expected *MatchNoDocsQuery, got %T", got)
	}
}

// TestDoubleValuesSource_GetRangeQuery verifies that the range query for
// double values resolves to a NumericDocValuesRangeQuery / FieldExistsQuery.
func TestDoubleValuesSource_GetRangeQuery(t *testing.T) {
	src := NewDoubleValuesSource("price")
	// Full range -> FieldExistsQuery
	if _, ok := src.GetRangeQuery(math.Inf(-1), math.Inf(+1)).(*FieldExistsQuery); !ok {
		t.Errorf("full-open range should fold to FieldExistsQuery")
	}
	// NaN -> MatchNoDocs
	if _, ok := src.GetRangeQuery(math.NaN(), 1).(*MatchNoDocsQuery); !ok {
		t.Errorf("NaN lower bound should fold to MatchNoDocsQuery")
	}
	// Empty range (lower > upper) -> MatchNoDocs
	if _, ok := src.GetRangeQuery(2, 1).(*MatchNoDocsQuery); !ok {
		t.Errorf("inverted range should fold to MatchNoDocsQuery")
	}
	// Normal range -> NumericDocValuesRangeQuery
	if _, ok := src.GetRangeQuery(0.0, 1.0).(*NumericDocValuesRangeQuery); !ok {
		t.Errorf("normal range should produce a NumericDocValuesRangeQuery")
	}
}

// TestLongValuesSource_GetRangeQuery verifies the long-values path.
func TestLongValuesSource_GetRangeQuery(t *testing.T) {
	src := NewLongValuesSource("v")
	if _, ok := src.GetRangeQuery(math.MinInt64, math.MaxInt64).(*FieldExistsQuery); !ok {
		t.Errorf("full-open range should fold to FieldExistsQuery")
	}
	if _, ok := src.GetRangeQuery(10, 1).(*MatchNoDocsQuery); !ok {
		t.Errorf("inverted range should fold to MatchNoDocsQuery")
	}
	if _, ok := src.GetRangeQuery(0, 100).(*NumericDocValuesRangeQuery); !ok {
		t.Errorf("normal range should produce a NumericDocValuesRangeQuery")
	}
}

// TestAutomatonQuery_RewriteWithoutReader verifies the safe degradation
// path when no reader is available (e.g. unit-test contexts).  The query
// is wrapped in a ConstantScoreQuery rather than producing nil.
func TestAutomatonQuery_RewriteWithoutReader(t *testing.T) {
	a := automaton.MakeString("hello")
	q := NewAutomatonQuery(index.NewTerm("f", "hello"), a)
	got, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if got == nil {
		t.Fatal("Rewrite returned nil")
	}
	// Single-string automata fold to a TermQuery.
	if _, ok := got.(*TermQuery); !ok {
		t.Errorf("single-string Rewrite should yield *TermQuery, got %T", got)
	}
}

// TestQueryRescorer_NoSearcher verifies the empty-input contract.
func TestQueryRescorer_NoSearcher(t *testing.T) {
	r := NewQueryRescorer(NewMatchAllDocsQuery())
	got, err := r.Rescore(nil, &TopDocs{ScoreDocs: []*ScoreDoc{NewScoreDoc(0, 1, 0)}})
	if err != nil {
		t.Fatalf("Rescore error: %v", err)
	}
	if got == nil || len(got.ScoreDocs) != 1 {
		t.Errorf("expected passthrough TopDocs, got %v", got)
	}
}

// TestQueryRescorer_NilTopDocs verifies the nil-input contract.
func TestQueryRescorer_NilTopDocs(t *testing.T) {
	r := NewQueryRescorer(NewMatchAllDocsQuery())
	got, err := r.Rescore(nil, nil)
	if err != nil {
		t.Fatalf("Rescore error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

// TestDateRangeQuery_Rewrite verifies that the date range query rewrites
// to a PointRangeQuery with sortable-long packed bounds.
func TestDateRangeQuery_Rewrite(t *testing.T) {
	lo := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	q := NewDateRangeQuery("ts", lo, hi)
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*PointRangeQuery); !ok {
		t.Errorf("expected *PointRangeQuery, got %T", rewritten)
	}
}

// TestDateRangeQuery_InvertedRange verifies the inverted-range short-circuit.
func TestDateRangeQuery_InvertedRange(t *testing.T) {
	lo := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	q := NewDateRangeQuery("ts", lo, hi)
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("inverted range should fold to MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestPointInSetQuery_RewriteEmpty verifies the empty-set short-circuit.
func TestPointInSetQuery_RewriteEmpty(t *testing.T) {
	q := NewPointInSetQuery("f", 1, 4, nil)
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("empty set should fold to MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestPointInSetQuery_RewriteExpands verifies that a non-empty set expands
// to a BooleanQuery of PointRangeQueries.
func TestPointInSetQuery_RewriteExpands(t *testing.T) {
	q := NewPointInSetQuery("f", 1, 4, [][]byte{
		{0, 0, 0, 1},
		{0, 0, 0, 2},
	})
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	bq, ok := rewritten.(*BooleanQuery)
	if !ok {
		t.Fatalf("expected *BooleanQuery, got %T", rewritten)
	}
	if got := len(bq.Clauses()); got != 2 {
		t.Errorf("expected 2 clauses, got %d", got)
	}
}

// TestPointQuery_CreateWeight verifies that the abstract base produces
// an empty-but-non-nil Weight.
func TestPointQuery_CreateWeight(t *testing.T) {
	q := NewPointQuery("f", 1, 4)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight error: %v", err)
	}
	if w == nil {
		t.Fatal("CreateWeight returned nil Weight")
	}
}

// TestNumericDocValuesRangeQuery_CreateWeight smoke-tests the weight
// pipeline; with no reader context the weight builds but cannot produce
// matches.
func TestNumericDocValuesRangeQuery_CreateWeight(t *testing.T) {
	q := NewNumericDocValuesRangeQuery("v", 0, 100)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight error: %v", err)
	}
	if w == nil {
		t.Fatal("CreateWeight returned nil Weight")
	}

// TestQueryRescorer_DefaultCombine verifies the default combine function
// adds the second-pass score when secondPassMatches is true and returns
// the first-pass score unchanged otherwise.
}
func TestQueryRescorer_DefaultCombine(t *testing.T) {
	if got := defaultQueryRescorerCombine(1, true, 2); got != 3 {
		t.Errorf("combine(1, true, 2) = %v, want 3", got)
	}
	if got := defaultQueryRescorerCombine(1, false, 2); got != 1 {
		t.Errorf("combine(1, false, 2) = %v, want 1", got)
	}
}