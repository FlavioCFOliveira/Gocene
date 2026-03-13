// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestTermQuery_Basics(t *testing.T) {
	term := index.NewTerm("field", "value")
	q1 := NewTermQuery(term)

	// Test GetTerm
	if !q1.Term().Equals(term) {
		t.Error("Term() should return original term")
	}

	// Test Clone
	cloned := q1.Clone().(*TermQuery)
	if !cloned.Term().Equals(term) {
		t.Error("Cloned term should equal original")
	}

	// Test Equals
	q2 := NewTermQuery(index.NewTerm("field", "value"))
	if !q1.Equals(q2) {
		t.Error("Queries with same term should be equal")
	}

	q3 := NewTermQuery(index.NewTerm("field", "other"))
	if q1.Equals(q3) {
		t.Error("Queries with different term should not be equal")
	}

	// Test HashCode
	if q1.HashCode() != q2.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

	// Test String
	if q1.String() != "field:value" {
		t.Errorf("Expected 'field:value', got %q", q1.String())
	}
}

func TestTermQuery_Rewrite(t *testing.T) {
	term := index.NewTerm("f", "v")
	q := NewTermQuery(term)
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if rewritten != q {
		t.Error("TermQuery should rewrite to itself")
	}
}
