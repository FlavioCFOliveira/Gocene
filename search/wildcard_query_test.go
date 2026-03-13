// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestWildcardQuery_Basics(t *testing.T) {
	term := index.NewTerm("field", "b*a")
	q := NewWildcardQuery(term)

	if q.GetField() != "field" {
		t.Errorf("Expected field 'field', got %q", q.GetField())
	}

	if !q.Term().Equals(term) {
		t.Error("Term should equal the original term")
	}

	// Test Clone
	cloned := q.Clone().(*WildcardQuery)
	if !cloned.Term().Equals(term) {
		t.Error("Cloned term should equal original")
	}

	// Test Equals
	q2 := NewWildcardQueryWithStrings("field", "b*a")
	if !q.Equals(q2) {
		t.Error("Two WildcardQuery with same term should be equal")
	}

	// Test not equal
	q3 := NewWildcardQueryWithStrings("field", "b?a")
	if q.Equals(q3) {
		t.Error("Different patterns should not be equal")
	}

	// Test HashCode
	if q.HashCode() != term.HashCode() {
		t.Errorf("Expected HashCode %d, got %d", term.HashCode(), q.HashCode())
	}

	// Test String
	if q.String() != "field:b*a" {
		t.Errorf("Expected field:b*a, got %q", q.String())
	}
}

func TestWildcardQuery_Rewrite(t *testing.T) {
	q := NewWildcardQueryWithStrings("field", "b*a")
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten != q {
		t.Error("WildcardQuery should currently rewrite to itself")
	}
}

func TestWildcardQuery_WithNilTerm(t *testing.T) {
	q := NewWildcardQuery(nil)
	if q.GetField() != "" {
		t.Error("GetField should return empty string for nil term")
	}
	if q.String() != "<nil>" {
		t.Errorf("Expected <nil>, got %q", q.String())
	}

	cloned := q.Clone().(*WildcardQuery)
	if cloned.Term() != nil {
		t.Error("Cloned nil term should be nil")
	}

	if !q.Equals(NewWildcardQuery(nil)) {
		t.Error("Two nil term WildcardQueries should be equal")
	}
}
