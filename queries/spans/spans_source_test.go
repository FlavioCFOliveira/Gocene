// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestSpanTermQuery_FieldAndTerm(t *testing.T) {
	tq := NewSpanTermQuery(index.NewTerm("body", "test"))
	if tq.GetField() != "body" {
		t.Fatalf("GetField=%q", tq.GetField())
	}
	if tq.GetTerm().Text() != "test" {
		t.Fatalf("GetTerm=%q", tq.GetTerm().Text())
	}
}

func TestSpanTermQuery_StringNotEmpty(t *testing.T) {
	tq := NewSpanTermQuery(index.NewTerm("f", "hello"))
	s := tq.String()
	if s == "" {
		t.Fatal("String() returned empty")
	}
}

func TestSpanNearQuery_Ordered(t *testing.T) {
	a := NewSpanTermQuery(index.NewTerm("f", "a"))
	b := NewSpanTermQuery(index.NewTerm("f", "b"))
	nq := NewSpanNearQueryFromTerms([]*SpanTermQuery{a, b}, 5, true)
	if nq == nil {
		t.Fatal("NewSpanNearQueryFromTerms returned nil")
	}
	if nq.GetSlop() != 5 {
		t.Fatalf("GetSlop=%d", nq.GetSlop())
	}
	if !nq.IsInOrder() {
		t.Error("inOrder should be true")
	}
	if nq.GetField() != "f" {
		t.Fatalf("GetField=%q", nq.GetField())
	}
}

func TestSpanNearQuery_Unordered(t *testing.T) {
	sq := NewSpanNearQueryFromTerms(
		[]*SpanTermQuery{NewSpanTermQuery(index.NewTerm("f", "a"))}, 3, false)
	if sq == nil {
		t.Fatal("returned nil")
	}
	if sq.IsInOrder() {
		t.Error("inOrder should be false for unordered")
	}
}

func TestSpanNearQuery_MultipleClauses(t *testing.T) {
	clauses := []*SpanTermQuery{
		NewSpanTermQuery(index.NewTerm("field", "one")),
		NewSpanTermQuery(index.NewTerm("field", "two")),
		NewSpanTermQuery(index.NewTerm("field", "three")),
	}
	nq := NewSpanNearQueryFromTerms(clauses, 10, true)
	if nq.GetSlop() != 10 {
		t.Fatalf("GetSlop=%d", nq.GetSlop())
	}
}

func TestSpanNearBuilder_Ordered(t *testing.T) {
	builder := NewOrderedNearQuery("myfield")
	if builder == nil {
		t.Fatal("NewOrderedNearQuery returned nil")
	}
}

func TestSpanNearBuilder_Unordered(t *testing.T) {
	builder := NewUnorderedNearQuery("myfield")
	if builder == nil {
		t.Fatal("NewUnorderedNearQuery returned nil")
	}
}

func TestSpanTermQuery_RewriteSucceeds(t *testing.T) {
	tq := NewSpanTermQuery(index.NewTerm("f", "val"))
	_, err := tq.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	// Rewrite may return a clone; just verify no error
}

func TestSpanTermQuery_HashCodeStable(t *testing.T) {
	a := NewSpanTermQuery(index.NewTerm("f", "x"))
	b := NewSpanTermQuery(index.NewTerm("f", "x"))
	if a.HashCode() != b.HashCode() {
		t.Error("Equal SpanTermQueries should have equal hash codes")
	}
}
