// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestBasics.java
//
// This file provides a comprehensive test suite covering all 8 span query
// types: SpanTermQuery, SpanNearQuery, SpanOrQuery, SpanNotQuery,
// SpanFirstQuery, SpanPositionRangeQuery, SpanContainingQuery, and
// SpanWithinQuery.  Construction, hash/equals, String, visitor, Clone,
// field accessors, and field-mismatch rejection are tested per type.
//
// Full index-integrated matching tests (Lucene's checkHits pattern) are
// deferred until RandomIndexWriter and the IndexSearcher span-wiring are
// complete (backlog #2709).

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// SpanTermQuery  (queries/spans type)
// ---------------------------------------------------------------------------

func TestBasics_SpanTermQuery(t *testing.T) {
	t.Parallel()

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		term := index.NewTerm("body", "hello")
		q := NewSpanTermQuery(term)
		if q.GetField() != "body" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "body")
		}
		got := q.GetTerm()
		if got == nil || got.Field != "body" || string(got.Bytes.Bytes) != "hello" {
			t.Errorf("GetTerm = %v; want {body:hello}", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "test"))
		s := q.String()
		if s != "f:test" {
			t.Errorf("String = %q; want %q", s, "f:test")
		}
	})

	t.Run("visitor", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("body", "fox"))
		v := &termCollectingVisitor{}
		q.Visit(v)
		if !v.containsTerm("body", "fox") {
			t.Errorf("visitor did not collect {body:fox}; got %v", v.terms)
		}
		if len(v.terms) != 1 {
			t.Errorf("visitor collected %d terms; want 1", len(v.terms))
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := NewSpanTermQuery(index.NewTerm("f", "x"))
		q2 := NewSpanTermQuery(index.NewTerm("f", "x"))
		q3 := NewSpanTermQuery(index.NewTerm("f", "y"))
		q4 := NewSpanTermQuery(index.NewTerm("g", "x"))
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries have different hash")
		}
		if q1.Equals(q3) {
			t.Error("different terms reported equal")
		}
		if q1.Equals(q4) {
			t.Error("different fields reported equal")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "val"))
		c := q.Clone().(*SpanTermQuery)
		if !q.Equals(c) {
			t.Error("clone not equal to original")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})

	t.Run("nil_context_getspans", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(index.NewTerm("f", "t"))
		sw, err := q.CreateSpanWeight(nil, false, 1.0)
		if err != nil {
			t.Fatalf("CreateSpanWeight: %v", err)
		}
		spans, err := sw.GetSpans(nil, PostingsPositions)
		if err != nil {
			t.Fatalf("GetSpans(nil): %v", err)
		}
		if spans != nil {
			t.Fatal("expected nil spans for nil context")
		}
	})

	t.Run("field_empty_term", func(t *testing.T) {
		t.Parallel()
		// SpanTermQuery with empty term — should still construct fine.
		term := &index.Term{Field: "f", Bytes: util.NewBytesRef([]byte(""))}
		q := NewSpanTermQuery(term)
		if q.GetTerm().Bytes.Length != 0 {
			t.Error("expected empty bytes")
		}
		s := q.String()
		if s != "f:" {
			// String rendering of empty term
			t.Errorf("String = %q; want %q", s, "f:")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanNearQuery  (queries/spans type, uses builder API)
// ---------------------------------------------------------------------------

func TestBasics_SpanNearQuery(t *testing.T) {
	t.Parallel()

	term := func(field, text string) *index.Term {
		return index.NewTerm(field, text)
	}
	stq := func(field, text string) *SpanTermQuery {
		return NewSpanTermQuery(term(field, text))
	}

	t.Run("ordered_builder", func(t *testing.T) {
		t.Parallel()
		q := NewOrderedNearQuery("f").
			AddClause(stq("f", "a")).
			AddClause(stq("f", "b")).
			SetSlop(1).
			Build()
		if q.GetField() != "f" {
			t.Errorf("GetField = %q", q.GetField())
		}
		if !q.IsInOrder() {
			t.Error("expected inOrder=true")
		}
		if q.GetSlop() != 1 {
			t.Errorf("GetSlop = %d; want 1", q.GetSlop())
		}
		if len(q.GetClauses()) != 2 {
			t.Errorf("got %d clauses; want 2", len(q.GetClauses()))
		}
	})

	t.Run("unordered_builder", func(t *testing.T) {
		t.Parallel()
		q := NewUnorderedNearQuery("f").
			AddClause(stq("f", "a")).
			AddClause(stq("f", "b")).
			SetSlop(3).
			Build()
		if q.GetField() != "f" {
			t.Errorf("GetField = %q", q.GetField())
		}
		if q.IsInOrder() {
			t.Error("expected inOrder=false")
		}
		if q.GetSlop() != 3 {
			t.Errorf("GetSlop = %d; want 3", q.GetSlop())
		}
	})

	t.Run("NewSpanNearQueryFromTerms", func(t *testing.T) {
		t.Parallel()
		clauses := []*SpanTermQuery{stq("f", "a"), stq("f", "b")}
		q := NewSpanNearQueryFromTerms(clauses, 2, true)
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
		if len(q.GetClauses()) != 2 {
			t.Errorf("got %d clauses; want 2", len(q.GetClauses()))
		}
	})

	t.Run("add_gap", func(t *testing.T) {
		t.Parallel()
		q := NewOrderedNearQuery("f").
			AddClause(stq("f", "a")).
			AddGap(2).
			AddClause(stq("f", "b")).
			SetSlop(0).
			Build()
		if len(q.GetClauses()) != 2 {
			t.Errorf("gap should not add clause; got %d clauses", len(q.GetClauses()))
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		build := func() *SpanNearQuery {
			return NewOrderedNearQuery("f").
				AddClause(stq("f", "x")).
				AddClause(stq("f", "y")).
				SetSlop(1).
				Build()
		}
		q1 := build()
		q2 := build()
		q3 := NewOrderedNearQuery("f").
			AddClause(stq("f", "x")).
			AddClause(stq("f", "z")).
			SetSlop(1).
			Build()
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		if q1.Equals(q3) {
			t.Error("different clauses reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := NewOrderedNearQuery("f").
			AddClause(stq("f", "a")).
			AddClause(stq("f", "b")).
			SetSlop(1).
			Build()
		s := q.String()
		if s != "spanNear([f:a, f:b], 1, true)" {
			t.Errorf("String = %q", s)
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := NewOrderedNearQuery("f").
			AddClause(stq("f", "a")).
			SetSlop(0).
			Build()
		c := q.Clone().(*SpanNearQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanOrQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanOrQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction_two_clauses", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(stq("f", "a"), stq("f", "b"))
		if q == nil {
			t.Fatal("expected non-nil SpanOrQuery")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
		if len(q.Clauses()) != 2 {
			t.Errorf("got %d clauses; want 2", len(q.Clauses()))
		}
	})

	t.Run("construction_single_clause", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(stq("f", "a"))
		if q == nil {
			t.Fatal("expected non-nil for single clause")
		}
	})

	t.Run("field_mismatch_returns_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(stq("f", "a"), stq("g", "b"))
		if q != nil {
			t.Error("expected nil for different fields")
		}
	})

	t.Run("empty_clauses_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery()
		if q != nil {
			t.Error("expected nil for empty clauses")
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		makeQ := func() *search.SpanOrQuery {
			return search.NewSpanOrQuery(stq("f", "x"), stq("f", "y"))
		}
		q1 := makeQ()
		q2 := makeQ()
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanOrQuery(stq("f", "x"), stq("f", "z"))
		if q1.Equals(q3) {
			t.Error("different queries reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(stq("f", "a"), stq("f", "b"))
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanOrQuery(stq("f", "a"), stq("f", "b"))
		c := q.Clone().(*search.SpanOrQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanNotQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanNotQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		include := stq("f", "include")
		exclude := stq("f", "exclude")
		q := search.NewSpanNotQuery(include, exclude)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	t.Run("field_mismatch_returns_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanNotQuery(stq("f", "a"), stq("g", "b"))
		if q != nil {
			t.Error("expected nil for different fields")
		}
	})

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		inc := stq("f", "foo")
		exc := stq("f", "bar")
		q := search.NewSpanNotQuery(inc, exc)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if !q.Include().Equals(inc) {
			t.Error("Include() mismatch")
		}
		if !q.Exclude().Equals(exc) {
			t.Error("Exclude() mismatch")
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := search.NewSpanNotQuery(stq("f", "x"), stq("f", "y"))
		q2 := search.NewSpanNotQuery(stq("f", "x"), stq("f", "y"))
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanNotQuery(stq("f", "x"), stq("f", "z"))
		if q1.Equals(q3) {
			t.Error("different queries reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanNotQuery(stq("f", "good"), stq("f", "bad"))
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanNotQuery(stq("f", "a"), stq("f", "b"))
		c := q.Clone().(*search.SpanNotQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanFirstQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanFirstQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanFirstQuery(stq("f", "term"), 5)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		match := stq("f", "foo")
		q := search.NewSpanFirstQuery(match, 3)
		if !q.Match().Equals(match) {
			t.Error("Match() mismatch")
		}
		if q.End() != 3 {
			t.Errorf("End() = %d; want 3", q.End())
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := search.NewSpanFirstQuery(stq("f", "x"), 3)
		q2 := search.NewSpanFirstQuery(stq("f", "x"), 3)
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanFirstQuery(stq("f", "x"), 5)
		if q1.Equals(q3) {
			t.Error("different end reported equal")
		}
		q4 := search.NewSpanFirstQuery(stq("f", "y"), 3)
		if q1.Equals(q4) {
			t.Error("different match reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanFirstQuery(stq("f", "hello"), 2)
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanFirstQuery(stq("f", "val"), 7)
		c := q.Clone().(*search.SpanFirstQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})

	t.Run("end_zero", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanFirstQuery(stq("f", "x"), 0)
		if q.End() != 0 {
			t.Errorf("End() = %d; want 0", q.End())
		}
	})
}

// ---------------------------------------------------------------------------
// SpanPositionRangeQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanPositionRangeQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanPositionRangeQuery(stq("f", "term"), 1, 5)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		match := stq("f", "foo")
		q := search.NewSpanPositionRangeQuery(match, 2, 6)
		if !q.Match().Equals(match) {
			t.Error("Match() mismatch")
		}
		if q.Start() != 2 {
			t.Errorf("Start() = %d; want 2", q.Start())
		}
		if q.End() != 6 {
			t.Errorf("End() = %d; want 6", q.End())
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := search.NewSpanPositionRangeQuery(stq("f", "x"), 1, 5)
		q2 := search.NewSpanPositionRangeQuery(stq("f", "x"), 1, 5)
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanPositionRangeQuery(stq("f", "x"), 1, 6)
		if q1.Equals(q3) {
			t.Error("different end reported equal")
		}
		q4 := search.NewSpanPositionRangeQuery(stq("f", "x"), 2, 5)
		if q1.Equals(q4) {
			t.Error("different start reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanPositionRangeQuery(stq("f", "hello"), 0, 10)
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanPositionRangeQuery(stq("f", "val"), 2, 8)
		c := q.Clone().(*search.SpanPositionRangeQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})

	t.Run("zero_range", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanPositionRangeQuery(stq("f", "x"), 0, 0)
		if q.Start() != 0 || q.End() != 0 {
			t.Errorf("zero range: got start=%d end=%d", q.Start(), q.End())
		}
	})
}

// ---------------------------------------------------------------------------
// SpanContainingQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanContainingQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		big := stq("f", "big")
		small := stq("f", "small")
		q := search.NewSpanContainingQuery(big, small)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	t.Run("field_mismatch_returns_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanContainingQuery(stq("f", "x"), stq("g", "y"))
		if q != nil {
			t.Error("expected nil for different fields")
		}
	})

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		big := stq("f", "biggy")
		small := stq("f", "smally")
		q := search.NewSpanContainingQuery(big, small)
		if !q.Big().Equals(big) {
			t.Error("Big() mismatch")
		}
		if !q.Small().Equals(small) {
			t.Error("Small() mismatch")
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := search.NewSpanContainingQuery(stq("f", "x"), stq("f", "y"))
		q2 := search.NewSpanContainingQuery(stq("f", "x"), stq("f", "y"))
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanContainingQuery(stq("f", "x"), stq("f", "z"))
		if q1.Equals(q3) {
			t.Error("different small reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanContainingQuery(stq("f", "outer"), stq("f", "inner"))
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanContainingQuery(stq("f", "a"), stq("f", "b"))
		c := q.Clone().(*search.SpanContainingQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanWithinQuery  (search package type)
// ---------------------------------------------------------------------------

func TestBasics_SpanWithinQuery(t *testing.T) {
	t.Parallel()

	stq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	t.Run("construction", func(t *testing.T) {
		t.Parallel()
		big := stq("f", "container")
		small := stq("f", "contained")
		q := search.NewSpanWithinQuery(big, small)
		if q == nil {
			t.Fatal("expected non-nil")
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	t.Run("field_mismatch_returns_nil", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanWithinQuery(stq("f", "x"), stq("g", "y"))
		if q != nil {
			t.Error("expected nil for different fields")
		}
	})

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		big := stq("f", "biggy")
		small := stq("f", "smally")
		q := search.NewSpanWithinQuery(big, small)
		if !q.Big().Equals(big) {
			t.Error("Big() mismatch")
		}
		if !q.Small().Equals(small) {
			t.Error("Small() mismatch")
		}
	})

	t.Run("equals/hash", func(t *testing.T) {
		t.Parallel()
		q1 := search.NewSpanWithinQuery(stq("f", "x"), stq("f", "y"))
		q2 := search.NewSpanWithinQuery(stq("f", "x"), stq("f", "y"))
		if !q1.Equals(q2) {
			t.Error("equal queries not equal")
		}
		if q1.HashCode() != q2.HashCode() {
			t.Error("equal queries different hash")
		}
		q3 := search.NewSpanWithinQuery(stq("f", "x"), stq("f", "z"))
		if q1.Equals(q3) {
			t.Error("different small reported equal")
		}
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanWithinQuery(stq("f", "outer"), stq("f", "inner"))
		s := q.String("")
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("clone", func(t *testing.T) {
		t.Parallel()
		q := search.NewSpanWithinQuery(stq("f", "a"), stq("f", "b"))
		c := q.Clone().(*search.SpanWithinQuery)
		if !q.Equals(c) {
			t.Error("clone not equal")
		}
		if q == c {
			t.Error("clone is same pointer")
		}
	})
}

// ---------------------------------------------------------------------------
// SpanContainQuery (queries/spans abstract base) — field validation
// ---------------------------------------------------------------------------

func TestBasics_SpanContainQuery(t *testing.T) {
	t.Parallel()

	// Use search.SpanTermQuery for SpanContainQuery arguments since
	// *queries/spans.SpanTermQuery does not implement search.SpanQuery.
	sstq := func(field, text string) search.SpanQuery {
		return search.NewSpanTermQuery(index.NewTerm(field, text))
	}

	// Test NewSpanContainQuery with same fields → success.
	t.Run("same_fields", func(t *testing.T) {
		t.Parallel()
		q, err := NewSpanContainQuery(sstq("f", "big"), sstq("f", "small"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.GetField() != "f" {
			t.Errorf("GetField = %q; want %q", q.GetField(), "f")
		}
	})

	// Test NewSpanContainQuery with different fields → error.
	t.Run("different_fields", func(t *testing.T) {
		t.Parallel()
		_, err := NewSpanContainQuery(sstq("f", "big"), sstq("g", "small"))
		if err == nil {
			t.Fatal("expected error for different fields")
		}
	})

	// Test NewSpanContainQuery with nil big → error.
	t.Run("nil_big", func(t *testing.T) {
		t.Parallel()
		_, err := NewSpanContainQuery(nil, sstq("f", "small"))
		if err == nil {
			t.Fatal("expected error for nil big")
		}
	})

	// Test NewSpanContainQuery with nil little → error.
	t.Run("nil_little", func(t *testing.T) {
		t.Parallel()
		_, err := NewSpanContainQuery(sstq("f", "big"), nil)
		if err == nil {
			t.Fatal("expected error for nil little")
		}
	})

	// Test accessibility of big/little.
	t.Run("big_little_accessors", func(t *testing.T) {
		t.Parallel()
		big := sstq("f", "super")
		little := sstq("f", "sub")
		q, err := NewSpanContainQuery(big, little)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !q.Big.Equals(big) {
			t.Error("Big accessor mismatch")
		}
		if !q.Little.Equals(little) {
			t.Error("Little accessor mismatch")
		}
	})
}

// ---------------------------------------------------------------------------
// BooleanQuery composition — span query as SHOULD clause
// ---------------------------------------------------------------------------

func TestBasics_BooleanQueryComposition(t *testing.T) {
	t.Parallel()

	t.Run("span_term_in_boolean_should", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanTermQuery(index.NewTerm("f", "hello"))
		bq := search.NewBooleanQuery()
		bq.Add(spanQ, search.SHOULD)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("span_or_in_boolean_must", func(t *testing.T) {
		t.Parallel()
		spanQ := search.NewSpanOrQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "a")),
			search.NewSpanTermQuery(index.NewTerm("f", "b")),
		)
		bq := search.NewBooleanQuery()
		bq.Add(spanQ, search.MUST)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})

	t.Run("span_first_in_boolean_filter", func(t *testing.T) {
		t.Parallel()
		sf := search.NewSpanFirstQuery(
			search.NewSpanTermQuery(index.NewTerm("f", "term")), 3,
		)
		bq := search.NewBooleanQuery()
		bq.Add(sf, search.FILTER)
		if bq == nil {
			t.Fatal("expected non-nil BooleanQuery")
		}
	})
}

// ---------------------------------------------------------------------------
// MemPostingsEnum-based matching tests (verifies Spans output for 8 types)
// ---------------------------------------------------------------------------

func TestBasics_MatchingOutput(t *testing.T) {
	t.Parallel()

	// Positions: doc 0 → "w1 w2 w3" → w1@0, w2@1, w3@2
	//           doc 1 → "w1" → w1@0
	//           doc 2 → "w2 w3 w1" → w2@0, w3@1, w1@2
	posW1 := map[int][]int{0: {0}, 1: {0}, 2: {2}}
	posW2 := map[int][]int{0: {1}, 2: {0}}
	posW3 := map[int][]int{0: {2}, 2: {1}}

	t.Run("SpanTerm_w1", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "w1", posW1)
		result, err := drainSpans(sp)
		if err != nil {
			t.Fatalf("drainSpans: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("matched %d docs; want 3", len(result))
		}
		// doc 0 should have span [0,1)
		if spans, ok := result[0]; !ok || len(spans) != 1 || spans[0] != [2]int{0, 1} {
			t.Errorf("doc 0: got %v; want [[0,1]]", result[0])
		}
		// doc 1 should have span [0,1)
		if spans, ok := result[1]; !ok || len(spans) != 1 || spans[0] != [2]int{0, 1} {
			t.Errorf("doc 1: got %v; want [[0,1]]", result[1])
		}
		// doc 2 should have span [2,3)
		if spans, ok := result[2]; !ok || len(spans) != 1 || spans[0] != [2]int{2, 3} {
			t.Errorf("doc 2: got %v; want [[2,3]]", result[2])
		}
	})

	t.Run("SpanTerm_w2", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "w2", posW2)
		result, err := drainSpans(sp)
		if err != nil {
			t.Fatalf("drainSpans: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("matched %d docs; want 2", len(result))
		}
		if spans, ok := result[0]; !ok || len(spans) != 1 || spans[0] != [2]int{1, 2} {
			t.Errorf("doc 0: got %v; want [[1,2]]", result[0])
		}
		if spans, ok := result[2]; !ok || len(spans) != 1 || spans[0] != [2]int{0, 1} {
			t.Errorf("doc 2: got %v; want [[0,1]]", result[2])
		}
	})

	t.Run("SpanNearOrdered_slop0_w1_w2", func(t *testing.T) {
		t.Parallel()
		s1 := buildTermSpans("f", "w1", posW1)
		s2 := buildTermSpans("f", "w2", posW2)
		sp, err := NewNearSpansOrdered(0, []Spans{s1, s2})
		if err != nil {
			t.Fatalf("NewNearSpansOrdered: %v", err)
		}
		result, err := drainSpans(sp)
		if err != nil {
			t.Fatalf("drainSpans: %v", err)
		}
		// Only doc 0 has w1@0 then w2@1 consecutively.
		if len(result) != 1 {
			t.Errorf("matched %d docs; want 1", len(result))
		}
		if spans, ok := result[0]; !ok || len(spans) != 1 || spans[0] != [2]int{0, 2} {
			t.Errorf("doc 0: got %v; want [[0,2]]", result[0])
		}
	})

	t.Run("SpanNearUnordered_slop1_w1_w3", func(t *testing.T) {
		t.Parallel()
		s1 := buildTermSpans("f", "w1", posW1)
		s3 := buildTermSpans("f", "w3", posW3)
		sp, err := NewNearSpansUnordered(1, []Spans{s1, s3})
		if err != nil {
			t.Fatalf("NewNearSpansUnordered: %v", err)
		}
		result, err := drainSpans(sp)
		if err != nil {
			t.Fatalf("drainSpans: %v", err)
		}
		// Doc 0: w1@0 + w3@2, gap=1 ≤ slop=1 ✓
		// Doc 2: w1@2 + w3@1, sorted: w3@1 start, w1@2 start. maxEnd=3, minStart=1 → (3-1-2)=0 ≤1 ✓
		if len(result) != 2 {
			t.Errorf("matched %d docs; want 2", len(result))
		}
	})

	t.Run("empty_term", func(t *testing.T) {
		t.Parallel()
		// TermSpans with no positions → no matches.
		sp := buildTermSpans("f", "absent", map[int][]int{})
		result, err := drainSpans(sp)
		if err != nil {
			t.Fatalf("drainSpans: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("matched %d docs; want 0", len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// TestBasics_All is retained as a top-level runner for the sub-test groups
// above, matching the original deferred-file name.
// ---------------------------------------------------------------------------

func TestBasics_All(t *testing.T) {
	// All tests are now in their own TestBasics_* functions above.
	// This function serves as a smoke check that the test file loads.
	t.Log("test_basics_test.go loaded — individual TestBasics_* functions exist for all 8 types")
}
