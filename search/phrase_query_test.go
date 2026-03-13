// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

func TestPhraseQuery_Basics(t *testing.T) {
	q1 := NewPhraseQueryWithStrings("field", "one", "two")

	// Test Field
	if q1.Field() != "field" {
		t.Errorf("Expected field 'field', got %q", q1.Field())
	}

	// Test Terms
	if len(q1.Terms()) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(q1.Terms()))
	}

	// Test Slop
	if q1.GetSlop() != 0 {
		t.Errorf("Expected slop 0, got %d", q1.GetSlop())
	}
	q1.SetSlop(2)
	if q1.GetSlop() != 2 {
		t.Errorf("Expected slop 2, got %d", q1.GetSlop())
	}

	// Test Clone
	cloned := q1.Clone().(*PhraseQuery)
	if cloned.Field() != "field" || cloned.GetSlop() != 2 || len(cloned.Terms()) != 2 {
		t.Error("Clone failed to preserve properties")
	}

	// Test Equals
	q2 := NewPhraseQueryWithStrings("field", "one", "two")
	q2.SetSlop(2)
	if !q1.Equals(q2) {
		t.Error("Queries with same properties should be equal")
	}

	q3 := NewPhraseQueryWithStrings("field", "one", "two")
	q3.SetSlop(1)
	if q1.Equals(q3) {
		t.Error("Queries with different slop should not be equal")
	}

	// Test HashCode
	if q1.HashCode() != q2.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

	// Test String
	q4 := NewPhraseQueryWithStrings("field", "hello", "world")
	if q4.String() != "field:\"hello world\"" {
		t.Errorf("Expected field:\"hello world\", got %q", q4.String())
	}
	q4.SetSlop(5)
	if q4.String() != "field:\"hello world\"~5" {
		t.Errorf("Expected field:\"hello world\"~5, got %q", q4.String())
	}
}

func TestPhraseQuery_Rewrite(t *testing.T) {
	// Single term phrase rewrites to TermQuery
	qSingle := NewPhraseQueryWithStrings("field", "one")
	rewritten, err := qSingle.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*TermQuery); !ok {
		t.Errorf("Single term phrase should rewrite to TermQuery, got %T", rewritten)
	}

	// Multiple term phrase rewrites to itself
	qMulti := NewPhraseQueryWithStrings("field", "one", "two")
	rewritten2, _ := qMulti.Rewrite(nil)
	if rewritten2 != qMulti {
		t.Error("Multiple term phrase should rewrite to itself")
	}
}
