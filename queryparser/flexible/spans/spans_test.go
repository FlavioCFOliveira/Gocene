// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSpanTermQuery verifies that search.SpanTermQuery construction, clone,
// equality, and string representation work correctly.
func TestSpanTermQuery(t *testing.T) {
	term := index.NewTerm("field1", "hello")
	q := search.NewSpanTermQuery(term)

	if q.Term() != term {
		t.Error("Term() should return the same term")
	}

	out := q.String("field1")
	if out == "" {
		t.Error("String() should not be empty")
	}

	clone := q.Clone()
	cloneSpanQ, ok := clone.(search.SpanQuery)
	if !ok {
		t.Fatal("Clone() should produce a SpanQuery")
	}
	if !cloneSpanQ.Equals(q) {
		t.Error("Clone() should be equal to original")
	}

	q2 := search.NewSpanTermQuery(index.NewTerm("field1", "hello"))
	if !q.Equals(q2) {
		t.Error("Same term queries should be equal")
	}

	q3 := search.NewSpanTermQuery(index.NewTerm("field1", "world"))
	if q.Equals(q3) {
		t.Error("Different term queries should not be equal")
	}

	if q.HashCode() != q2.HashCode() {
		t.Error("Same terms should have same hash code")
	}

	if q.Equals(nil) {
		t.Error("Equals(nil) should be false")
	}

	if q.GetField() != "field1" {
		t.Errorf("GetField() = %q, want %q", q.GetField(), "field1")
	}

	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatal(err)
	}
	if rewritten != q {
		t.Error("Rewrite should return self")
	}
}

// TestSpanOrQuery verifies search.SpanOrQuery construction and clause handling.
func TestSpanOrQuery(t *testing.T) {
	stq1 := search.NewSpanTermQuery(index.NewTerm("f", "a"))
	stq2 := search.NewSpanTermQuery(index.NewTerm("f", "b"))
	stq3 := search.NewSpanTermQuery(index.NewTerm("f", "c"))

	q1 := search.NewSpanOrQuery(stq1)
	if q1 == nil {
		t.Fatal("NewSpanOrQuery with single clause should return non-nil")
	}

	q2 := search.NewSpanOrQuery(stq1, stq2, stq3)
	if q2 == nil {
		t.Fatal("NewSpanOrQuery with multiple clauses should return non-nil")
	}
	if len(q2.Clauses()) != 3 {
		t.Errorf("Clauses() length = %d, want 3", len(q2.Clauses()))
	}

	qEmpty := search.NewSpanOrQuery()
	if qEmpty != nil {
		t.Error("NewSpanOrQuery with no clauses should return nil")
	}

	stqOther := search.NewSpanTermQuery(index.NewTerm("g", "x"))
	qMismatch := search.NewSpanOrQuery(stq1, stqOther)
	if qMismatch != nil {
		t.Error("NewSpanOrQuery with mismatched fields should return nil")
	}

	q3 := search.NewSpanOrQuery(stq1, stq2)
	q3.AddClause(stq3)
	if len(q3.Clauses()) != 3 {
		t.Errorf("After AddClause, Clauses() length = %d, want 3", len(q3.Clauses()))
	}

	q3.AddClause(search.NewSpanTermQuery(index.NewTerm("wrong", "x")))
	if len(q3.Clauses()) != 3 {
		t.Errorf("AddClause with wrong field should be rejected, length = %d", len(q3.Clauses()))
	}

	clone := q2.Clone()
	if clone == nil {
		t.Fatal("Clone() should return non-nil")
	}
	if !clone.Equals(q2) {
		t.Error("Clone should equal original")
	}

	q4 := search.NewSpanOrQuery(stq1, stq2, stq3)
	if !q2.Equals(q4) {
		t.Error("Same clauses should be equal")
	}
	if q2.Equals(nil) {
		t.Error("Equals(nil) should be false")
	}

	if q2.HashCode() != q4.HashCode() {
		t.Error("Same structure should have same hash code")
	}

	str := q2.String("f")
	if str == "" {
		t.Error("String() should not be empty")
	}

	if q2.GetField() != "f" {
		t.Errorf("GetField() = %q, want %q", q2.GetField(), "f")
	}

	// Rewrite: zero clauses → MatchNoDocs
	emptyOr := search.NewSpanOrQuery(stq1)
	emptyOrClauses := &search.SpanOrQuery{}
	singleClause, err := emptyOr.Rewrite(nil)
	if err != nil {
		t.Fatal(err)
	}
	if singleClause == nil {
		t.Fatal("Rewrite of single clause or empty should not return nil")
	}
	_, _ = emptyOrClauses, singleClause

	// Rewrite: single clause returns the clause directly via search.NewSpanOrQuery
	single, err := search.NewSpanOrQuery(stq1).Rewrite(nil)
	if err != nil {
		t.Fatal(err)
	}
	if single != stq1 {
		t.Error("Single-clause rewrite should return the clause directly")
	}

	// Rewrite: multiple clauses returns self
	multi, err := search.NewSpanOrQuery(stq1, stq2).Rewrite(nil)
	if err != nil {
		t.Fatal(err)
	}
	if multi == nil {
		t.Fatal("Multiple-clause rewrite should not return nil")
	}
}

// TestSpanQueryInterface checks that SpanTermQuery and SpanOrQuery implement
// SpanQuery.
func TestSpanQueryInterface(t *testing.T) {
	stq := search.NewSpanTermQuery(index.NewTerm("f", "x"))
	orq := search.NewSpanOrQuery(stq)

	var _ search.SpanQuery = stq
	var _ search.SpanQuery = orq
	_ = stq
	_ = orq
}

// TestSpansConfigHandler verifies the spans config handler construction.
func TestSpansConfigHandler(t *testing.T) {
	h := spans.NewSpansQueryConfigHandler()
	if h == nil {
		t.Fatal("NewSpansQueryConfigHandler should not return nil")
	}
}

// TestSpansValidators verifies the spans processor and builder types exist.
func TestSpansValidators(t *testing.T) {
	v := &spans.SpansValidatorQueryNodeProcessor{}
	if v == nil {
		t.Fatal("SpansValidatorQueryNodeProcessor should not be nil")
	}

	u := &spans.UniqueFieldQueryNodeProcessor{}
	if u == nil {
		t.Fatal("UniqueFieldQueryNodeProcessor should not be nil")
	}
}

// TestSpansBuilders verifies the spans builder types exist.
func TestSpansBuilders(t *testing.T) {
	b1 := &spans.SpanTermQueryNodeBuilder{}
	if b1 == nil {
		t.Fatal("SpanTermQueryNodeBuilder should not be nil")
	}

	b2 := &spans.SpanOrQueryNodeBuilder{}
	if b2 == nil {
		t.Fatal("SpanOrQueryNodeBuilder should not be nil")
	}

	tree := spans.NewSpansQueryTreeBuilder()
	if tree == nil {
		t.Fatal("NewSpansQueryTreeBuilder should not return nil")
	}
}
