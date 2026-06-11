// Copyright 2026 Gocene. All rights reserved.

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
}

func TestSpanNearQuery_Unordered(t *testing.T) {
	sq := NewSpanNearQueryFromTerms(
		[]*SpanTermQuery{NewSpanTermQuery(index.NewTerm("f", "a"))}, 3, false)
	if sq == nil {
		t.Fatal("NewSpanNearQueryFromTerms returned nil")
	}
	if sq.IsInOrder() {
		t.Error("inOrder should be false")
	}
}

func TestSpanContainQuery_New(t *testing.T) {
	big := NewSpanTermQuery(index.NewTerm("f", "big"))
	little := NewSpanTermQuery(index.NewTerm("f", "little"))
	_, err := NewSpanContainQuery(big, little)
	if err != nil {
		t.Fatalf("NewSpanContainQuery: %v", err)
	}
}
