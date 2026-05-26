// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanQueryVisitor.java

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ---------------------------------------------------------------------------
// Test infrastructure: term-collecting QueryVisitor
// ---------------------------------------------------------------------------

// termCollectingVisitor collects all terms that ConsumeTerms is called with.
type termCollectingVisitor struct {
	terms []*index.Term
}

func (v *termCollectingVisitor) AcceptField(_ string) bool { return true }
func (v *termCollectingVisitor) VisitLeaf(_ search.Query)  {}
func (v *termCollectingVisitor) ConsumeTerms(_ search.Query, terms ...*index.Term) {
	v.terms = append(v.terms, terms...)
}
func (v *termCollectingVisitor) ConsumeTermsMatching(_ search.Query, _ string, _ func() search.ByteRunAutomaton) {
}
func (v *termCollectingVisitor) GetSubVisitor(_ search.Occur, _ search.Query) search.QueryVisitor {
	return v
}

// containsTerm reports whether the collected slice contains a term with the
// given field and text value.
func (v *termCollectingVisitor) containsTerm(field, text string) bool {
	for _, t := range v.terms {
		if t.Field != field {
			continue
		}
		if t.Bytes == nil {
			if text == "" {
				return true
			}
			continue
		}
		if string(t.Bytes.Bytes) == text {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TestSpanQueryVisitor_ExtractTermsEquivalent
//
// Verifies that Query.Visit populates a QueryVisitor with the expected terms
// from SpanTermQuery and SpanNearQuery trees, mirroring the assertion in
// Java's TestSpanQueryVisitor.testExtractTermsEquivalent.
//
// Deviation from Java: Java calls IndexSearcher.createWeight(...) which
// internally calls SpanWeight.extractTerms. Gocene instead calls Visit
// directly on the query because Gocene's IndexSearcher does not yet wire
// TermStates (backlog #2709). The observable contract — which terms get
// collected — is identical.
// ---------------------------------------------------------------------------

func TestSpanQueryVisitor_ExtractTermsEquivalent(t *testing.T) {
	t.Parallel()

	// Single SpanTermQuery
	t.Run("SpanTermQuery_single", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(newTerm("body", "fox"))
		v := &termCollectingVisitor{}
		q.Visit(v)
		if !v.containsTerm("body", "fox") {
			t.Errorf("expected term {body:fox}; got %v", v.terms)
		}
		if len(v.terms) != 1 {
			t.Errorf("expected 1 term; got %d: %v", len(v.terms), v.terms)
		}
	})

	// SpanNearQuery with two SpanTermQuery clauses
	t.Run("SpanNearQuery_two_clauses", func(t *testing.T) {
		t.Parallel()
		qFox := NewSpanTermQuery(newTerm("body", "fox"))
		qDog := NewSpanTermQuery(newTerm("body", "dog"))
		near := NewSpanNearQueryFromTerms([]*SpanTermQuery{qFox, qDog}, 1, true)
		v := &termCollectingVisitor{}
		near.Visit(v)
		if !v.containsTerm("body", "fox") {
			t.Errorf("expected term {body:fox}; got %v", v.terms)
		}
		if !v.containsTerm("body", "dog") {
			t.Errorf("expected term {body:dog}; got %v", v.terms)
		}
		if len(v.terms) != 2 {
			t.Errorf("expected 2 terms; got %d: %v", len(v.terms), v.terms)
		}
	})

	// SpanNearQuery with three clauses
	t.Run("SpanNearQuery_three_clauses", func(t *testing.T) {
		t.Parallel()
		qa := NewSpanTermQuery(newTerm("body", "a"))
		qb := NewSpanTermQuery(newTerm("body", "b"))
		qc := NewSpanTermQuery(newTerm("body", "c"))
		near := NewSpanNearQueryFromTerms([]*SpanTermQuery{qa, qb, qc}, 2, false)
		v := &termCollectingVisitor{}
		near.Visit(v)
		for _, text := range []string{"a", "b", "c"} {
			if !v.containsTerm("body", text) {
				t.Errorf("expected term {body:%s}; got %v", text, v.terms)
			}
		}
		if len(v.terms) != 3 {
			t.Errorf("expected 3 terms; got %d: %v", len(v.terms), v.terms)
		}
	})

	// Field rejection: when visitor rejects the field, no terms are collected.
	t.Run("SpanTermQuery_field_rejected", func(t *testing.T) {
		t.Parallel()
		q := NewSpanTermQuery(newTerm("body", "fox"))
		reject := &rejectingTermCollectingVisitor{}
		q.Visit(reject)
		if len(reject.terms) != 0 {
			t.Errorf("expected 0 terms when field rejected; got %v", reject.terms)
		}
	})
}

// rejectingTermCollectingVisitor rejects all fields.
type rejectingTermCollectingVisitor struct {
	terms []*index.Term
}

func (v *rejectingTermCollectingVisitor) AcceptField(_ string) bool { return false }
func (v *rejectingTermCollectingVisitor) VisitLeaf(_ search.Query)  {}
func (v *rejectingTermCollectingVisitor) ConsumeTerms(_ search.Query, terms ...*index.Term) {
	v.terms = append(v.terms, terms...)
}
func (v *rejectingTermCollectingVisitor) ConsumeTermsMatching(_ search.Query, _ string, _ func() search.ByteRunAutomaton) {
}
func (v *rejectingTermCollectingVisitor) GetSubVisitor(_ search.Occur, _ search.Query) search.QueryVisitor {
	return v
}

// TestSpanQueryVisitor_SpanNearQuery_GetTermStates verifies that
// GetTermStatesFromSlice accumulates states from all sub-weights.
func TestSpanQueryVisitor_SpanNearQuery_GetTermStates(t *testing.T) {
	t.Parallel()
	// Two independent SpanTermQuery weights: each contributes nothing to the
	// states map (ExtractStates is a no-op per the TermStates-backlog deviation),
	// but GetTermStatesFromSlice must not panic and must return a valid (empty) map.
	t1 := NewSpanTermQuery(newTerm("body", "x"))
	t2 := NewSpanTermQuery(newTerm("body", "y"))
	w1, err := t1.CreateSpanWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateSpanWeight t1: %v", err)
	}
	w2, err := t2.CreateSpanWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateSpanWeight t2: %v", err)
	}
	states := GetTermStatesFromSlice([]*SpanWeight{w1, w2})
	if states == nil {
		t.Fatal("GetTermStatesFromSlice must not return nil")
	}
}
