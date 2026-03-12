// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestMatchAllDocsQuery(t *testing.T) {
	q1 := NewMatchAllDocsQuery()
	q2 := NewMatchAllDocsQuery()

	// Test Clone
	cloned := q1.Clone()
	if _, ok := cloned.(*MatchAllDocsQuery); !ok {
		t.Error("Clone should return *MatchAllDocsQuery")
	}

	// Test Equals
	if !q1.Equals(q2) {
		t.Error("Two MatchAllDocsQuery should be equal")
	}

	// Test HashCode
	if q1.HashCode() != 0 {
		t.Error("MatchAllDocsQuery HashCode should be 0")
	}
}

func TestPrefixQuery(t *testing.T) {
	term := index.NewTerm("field", "prefix")
	q1 := NewPrefixQuery(term)

	// Test GetField
	if q1.GetField() != "field" {
		t.Errorf("Expected field 'field', got '%s'", q1.GetField())
	}

	// Test Prefix
	if !q1.Prefix().Equals(term) {
		t.Error("Prefix should equal the original term")
	}

	// Test Clone
	cloned := q1.Clone().(*PrefixQuery)
	if !cloned.Prefix().Equals(term) {
		t.Error("Cloned prefix should equal original")
	}

	// Test Equals
	term2 := index.NewTerm("field", "prefix")
	q2 := NewPrefixQuery(term2)
	if !q1.Equals(q2) {
		t.Error("Two PrefixQuery with same term should be equal")
	}

	// Test not equal with different term
	term3 := index.NewTerm("field", "different")
	q3 := NewPrefixQuery(term3)
	if q1.Equals(q3) {
		t.Error("PrefixQuery with different prefix should not be equal")
	}

	// Test HashCode
	if q1.HashCode() != term.HashCode() {
		t.Error("PrefixQuery HashCode should equal term HashCode")
	}
}

func TestPrefixQueryWithNilPrefix(t *testing.T) {
	q := NewPrefixQuery(nil)
	if q.GetField() != "" {
		t.Error("GetField should return empty string for nil prefix")
	}
}

func TestFieldExistsQuery(t *testing.T) {
	q1 := NewFieldExistsQuery("title")

	// Test GetField
	if q1.GetField() != "title" {
		t.Errorf("Expected field 'title', got '%s'", q1.GetField())
	}

	// Test Clone
	cloned := q1.Clone().(*FieldExistsQuery)
	if cloned.GetField() != "title" {
		t.Error("Cloned field should equal original")
	}

	// Test Equals
	q2 := NewFieldExistsQuery("title")
	if !q1.Equals(q2) {
		t.Error("Two FieldExistsQuery with same field should be equal")
	}

	// Test not equal with different field
	q3 := NewFieldExistsQuery("body")
	if q1.Equals(q3) {
		t.Error("FieldExistsQuery with different field should not be equal")
	}

	// Test Equals with different type
	q4 := NewMatchAllDocsQuery()
	if q1.Equals(q4) {
		t.Error("FieldExistsQuery should not equal MatchAllDocsQuery")
	}

	// Test HashCode consistency - same field should have same hash
	h1 := q1.HashCode()
	h2 := q2.HashCode()
	if h1 != h2 {
		t.Error("FieldExistsQuery with same field should have same HashCode")
	}

	// Different field should have different hash (most likely)
	q3hash := NewFieldExistsQuery("body")
	h3 := q3hash.HashCode()
	if h1 == h3 {
		t.Error("FieldExistsQuery with different fields should likely have different HashCodes")
	}
}
