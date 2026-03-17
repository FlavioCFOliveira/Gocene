// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

func TestNewSpanWeight(t *testing.T) {
	query := NewSpanQuery("field", "term")

	sw := NewSpanWeight(query, nil)

	if sw.Query != query {
		t.Error("Expected Query to be set")
	}
	if sw.Similarity != nil {
		t.Error("Expected Similarity to be nil")
	}
}

func TestSpanWeightGetQuery(t *testing.T) {
	query := NewSpanQuery("field", "term")
	sw := NewSpanWeight(query, nil)

	if sw.GetQuery() != query {
		t.Error("GetQuery should return the query")
	}
}

func TestSpanWeightGetValue(t *testing.T) {
	sw := NewSpanWeight(nil, nil)

	if sw.GetValue() != 1.0 {
		t.Errorf("Expected value 1.0, got %f", sw.GetValue())
	}
}

func TestSpanWeightIsCacheable(t *testing.T) {
	sw := NewSpanWeight(nil, nil)

	// SpanWeight should be cacheable
	if !sw.IsCacheable(nil) {
		t.Error("Expected SpanWeight to be cacheable")
	}
}

func TestNewSpanQuery(t *testing.T) {
	sq := NewSpanQuery("content", "hello")

	if sq.Field != "content" {
		t.Errorf("Expected field 'content', got '%s'", sq.Field)
	}
	if sq.Term != "hello" {
		t.Errorf("Expected term 'hello', got '%s'", sq.Term)
	}
}

func TestSpanQueryGetField(t *testing.T) {
	sq := NewSpanQuery("field", "term")
	if sq.GetField() != "field" {
		t.Error("GetField should return the field")
	}
}

func TestSpanQueryGetTerm(t *testing.T) {
	sq := NewSpanQuery("field", "term")
	if sq.GetTerm() != "term" {
		t.Error("GetTerm should return the term")
	}
}
