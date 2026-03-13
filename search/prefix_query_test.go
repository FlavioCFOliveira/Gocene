// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestPrefixQuery_Basics(t *testing.T) {
	term := index.NewTerm("field", "abc")
	q := NewPrefixQuery(term)

	if q.GetField() != "field" {
		t.Errorf("Expected field 'field', got %q", q.GetField())
	}

	if !q.Prefix().Equals(term) {
		t.Error("Prefix should equal the original term")
	}

	// Test Clone
	cloned := q.Clone().(*PrefixQuery)
	if !cloned.Prefix().Equals(term) {
		t.Error("Cloned prefix should equal original")
	}

	// Test Equals
	q2 := NewPrefixQueryWithStrings("field", "abc")
	if !q.Equals(q2) {
		t.Error("Two PrefixQuery with same term should be equal")
	}

	// Test not equal
	q3 := NewPrefixQueryWithStrings("field", "abcd")
	if q.Equals(q3) {
		t.Error("Different prefixes should not be equal")
	}

	// Test HashCode
	if q.HashCode() != term.HashCode() {
		t.Errorf("Expected HashCode %d, got %d", term.HashCode(), q.HashCode())
	}

	// Test String
	if q.String() != "field:abc*" {
		t.Errorf("Expected field:abc*, got %q", q.String())
	}
}

func TestPrefixQuery_EmptyPrefix(t *testing.T) {
	q := NewPrefixQueryWithStrings("field", "")
	if q.String() != "field:*" {
		t.Errorf("Expected field:*, got %q", q.String())
	}
}

func TestPrefixQuery_Rewrite(t *testing.T) {
	q := NewPrefixQueryWithStrings("field", "abc")
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten != q {
		t.Error("PrefixQuery should currently rewrite to itself")
	}
}

func TestPrefixQuery_WithNilPrefix(t *testing.T) {
	q := NewPrefixQuery(nil)
	if q.GetField() != "" {
		t.Error("GetField should return empty string for nil prefix")
	}
	if q.String() != "<nil>:*" {
		t.Errorf("Expected <nil>:*, got %q", q.String())
	}
}
