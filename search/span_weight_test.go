// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewSpanWeight(t *testing.T) {
	query := NewSpanTermQuery(index.NewTerm("field", "term"))

	sw := NewSpanWeight(query, nil)

	if sw.SpanQuery != query {
		t.Error("Expected SpanQuery to be set")
	}
	if sw.Similarity != nil {
		t.Error("Expected Similarity to be nil")
	}
}

func TestSpanWeightGetSpanQuery(t *testing.T) {
	query := NewSpanTermQuery(index.NewTerm("field", "term"))
	sw := NewSpanWeight(query, nil)

	if sw.GetSpanQuery() != query {
		t.Error("GetSpanQuery should return the query")
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

func TestNewSpanTermQuery(t *testing.T) {
	sq := NewSpanTermQuery(index.NewTerm("content", "hello"))

	if sq.GetField() != "content" {
		t.Errorf("Expected field 'content', got '%s'", sq.GetField())
	}
	if sq.Term().Text() != "hello" {
		t.Errorf("Expected term 'hello', got '%s'", sq.Term().Text())
	}
}

func TestSpanTermQueryGetField(t *testing.T) {
	sq := NewSpanTermQuery(index.NewTerm("field", "term"))
	if sq.GetField() != "field" {
		t.Error("GetField should return the field")
	}
}

func TestSpanTermQueryTerm(t *testing.T) {
	sq := NewSpanTermQuery(index.NewTerm("field", "term"))
	if sq.Term().Text() != "term" {
		t.Error("Term should return the term")
	}
}
