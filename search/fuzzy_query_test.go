// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestFuzzyQuery_Basics(t *testing.T) {
	term := index.NewTerm("field", "abc")
	q := NewFuzzyQuery(term)

	if !q.Term().Equals(term) {
		t.Error("Term should equal original")
	}

	if q.MaxEdits() != 2 {
		t.Errorf("Expected maxEdits 2, got %d", q.MaxEdits())
	}

	if q.PrefixLength() != 0 {
		t.Errorf("Expected prefixLength 0, got %d", q.PrefixLength())
	}

	// Test Clone
	cloned := q.Clone().(*FuzzyQuery)
	if !cloned.Term().Equals(term) || cloned.MaxEdits() != q.MaxEdits() {
		t.Error("Clone failed")
	}

	// Test Equals
	q2 := NewFuzzyQueryWithStrings("field", "abc")
	if !q.Equals(q2) {
		t.Error("Equal queries should be equal")
	}

	// Test HashCode
	if q.HashCode() != q2.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

	// Test String
	if q.String() != "field:abc~2" {
		t.Errorf("Expected field:abc~2, got %q", q.String())
	}
}

func TestFuzzyQuery_Params(t *testing.T) {
	term := index.NewTerm("field", "abc")
	q := NewFuzzyQueryWithParams(term, 1, 2, 10)

	if q.MaxEdits() != 1 {
		t.Errorf("Expected maxEdits 1, got %d", q.MaxEdits())
	}

	if q.PrefixLength() != 2 {
		t.Errorf("Expected prefixLength 2, got %d", q.PrefixLength())
	}

	if q.MaxExpansions() != 10 {
		t.Errorf("Expected maxExpansions 10, got %d", q.MaxExpansions())
	}

	if !q.TranspositionsAllowed() {
		t.Error("Expected transpositions to be allowed by default")
	}
}

func TestFuzzyQuery_Rewrite(t *testing.T) {
	q := NewFuzzyQueryWithStrings("field", "abc")
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten != q {
		t.Error("FuzzyQuery should currently rewrite to itself")
	}
}

func TestFuzzyQuery_EqualsComplexity(t *testing.T) {
	q := NewFuzzyQueryWithStrings("field", "abc")

	tests := []struct {
		name     string
		other    Query
		expected bool
	}{
		{"Same", NewFuzzyQueryWithStrings("field", "abc"), true},
		{"DiffTerm", NewFuzzyQueryWithStrings("field", "abcd"), false},
		{"DiffMaxEdits", NewFuzzyQueryWithStringsMaxEdits("field", "abc", 1), false},
		{"DiffPrefixLen", NewFuzzyQueryWithParams(index.NewTerm("field", "abc"), 2, 1, 50), false},
		{"DiffExpansions", NewFuzzyQueryWithParams(index.NewTerm("field", "abc"), 2, 0, 10), false},
		{"DiffType", NewMatchAllDocsQuery(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if q.Equals(tt.other) != tt.expected {
				t.Errorf("Equals(%s) expected %v, got %v", tt.name, tt.expected, !tt.expected)
			}
		})
	}
}
