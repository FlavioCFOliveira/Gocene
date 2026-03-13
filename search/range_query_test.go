// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

func TestTermRangeQuery_Basics(t *testing.T) {
	q := NewTermRangeQueryWithStrings("field", "a", "c", true, true)

	if q.Field() != "field" {
		t.Errorf("Expected field 'field', got %q", q.Field())
	}

	if string(q.LowerTerm()) != "a" {
		t.Errorf("Expected lower term 'a', got %q", string(q.LowerTerm()))
	}

	if string(q.UpperTerm()) != "c" {
		t.Errorf("Expected upper term 'c', got %q", string(q.UpperTerm()))
	}

	if !q.IncludesLower() {
		t.Error("Expected includesLower to be true")
	}

	if !q.IncludesUpper() {
		t.Error("Expected includesUpper to be true")
	}

	// Test Clone
	cloned := q.Clone().(*TermRangeQuery)
	if cloned.Field() != q.Field() || !cloned.Equals(q) {
		t.Error("Clone failed")
	}

	// Test Equals
	q2 := NewTermRangeQueryWithStrings("field", "a", "c", true, true)
	if !q.Equals(q2) {
		t.Error("Equivalent queries should be equal")
	}

	// Test HashCode
	if q.HashCode() != q2.HashCode() {
		t.Error("Equivalent queries should have same HashCode")
	}

	// Test String
	if q.String() != "field:[a TO c]" {
		t.Errorf("Expected field:[a TO c], got %q", q.String())
	}

	qExclusive := NewTermRangeQueryWithStrings("field", "a", "c", false, false)
	if qExclusive.String() != "field:{a TO c}" {
		t.Errorf("Expected field:{a TO c}, got %q", qExclusive.String())
	}
}

func TestTermRangeQuery_NullTerms(t *testing.T) {
	q := NewTermRangeQuery("field", nil, nil, true, true)
	if q.String() != "field:[* TO *]" {
		t.Errorf("Expected field:[* TO *], got %q", q.String())
	}

	q2 := NewTermRangeQueryWithStrings("field", "", "", true, true)
	// NewTermRangeQueryWithStrings converts empty string to nil
	if q2.String() != "field:[* TO *]" {
		t.Errorf("Expected field:[* TO *], got %q", q2.String())
	}
}

func TestTermRangeQuery_Rewrite(t *testing.T) {
	q := NewTermRangeQueryWithStrings("field", "a", "c", true, true)
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten != q {
		t.Error("TermRangeQuery should currently rewrite to itself")
	}
}

func TestTermRangeQuery_EqualsComplexity(t *testing.T) {
	q := NewTermRangeQueryWithStrings("field", "a", "c", true, true)

	tests := []struct {
		name     string
		other    Query
		expected bool
	}{
		{"Same", NewTermRangeQueryWithStrings("field", "a", "c", true, true), true},
		{"DiffField", NewTermRangeQueryWithStrings("other", "a", "c", true, true), false},
		{"DiffLower", NewTermRangeQueryWithStrings("field", "b", "c", true, true), false},
		{"DiffUpper", NewTermRangeQueryWithStrings("field", "a", "d", true, true), false},
		{"DiffInclLower", NewTermRangeQueryWithStrings("field", "a", "c", false, true), false},
		{"DiffInclUpper", NewTermRangeQueryWithStrings("field", "a", "c", true, false), false},
		{"DiffType", NewMatchAllDocsQuery(), false},
		{"NullLowerSame", NewTermRangeQuery("field", nil, []byte("c"), true, true), true}, // q also needs to have null lower for this to be true in comparison
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// If testing null lower, we need a query with null lower
			base := q
			if tt.name == "NullLowerSame" {
				base = NewTermRangeQuery("field", nil, []byte("c"), true, true)
			}
			if base.Equals(tt.other) != tt.expected {
				t.Errorf("Equals(%s) expected %v, got %v", tt.name, tt.expected, !tt.expected)
			}
		})
	}
}
