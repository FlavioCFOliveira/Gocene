// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	spanparser "github.com/FlavioCFOliveira/Gocene/queryparser/flexible/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSpanQueryParser verifies the spans query parser component construction.
func TestSpanQueryParser(t *testing.T) {
	config := spanparser.NewSpansQueryConfigHandler()
	if config == nil {
		t.Fatal("NewSpansQueryConfigHandler returned nil")
	}

	attr := &spanparser.UniqueFieldAttributeImpl{}
	attr.SetUniqueField("field1")
	if attr.GetUniqueField() != "field1" {
		t.Errorf("GetUniqueField = %q, want %q", attr.GetUniqueField(), "field1")
	}

	processor := &spanparser.SpansValidatorQueryNodeProcessor{}
	node, err := processor.Process(nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if node != nil {
		t.Error("Process should return nil for nil input")
	}
}

// TestSpanQueryParserSimpleSample verifies the span query parser example.
func TestSpanQueryParserSimpleSample(t *testing.T) {
	config := spanparser.NewSpansQueryConfigHandler()
	if config == nil {
		t.Fatal("NewSpansQueryConfigHandler returned nil")
	}
}

// TestSpanTermQueryNodeBuilder verifies that SpanTermQueryNodeBuilder builds
// a SpanTermQuery from a FieldQueryNode.
func TestSpanTermQueryNodeBuilder(t *testing.T) {
	builder := &spanparser.SpanTermQueryNodeBuilder{}
	node := flexible.NewFieldQueryNode("body", "hello", 0, 5)
	q, err := builder.Build(node)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	stq, ok := q.(*search.SpanTermQuery)
	if !ok {
		t.Fatalf("expected SpanTermQuery, got %T", q)
	}
	if stq.Term().Field != "body" || stq.Term().Text() != "hello" {
		t.Errorf("unexpected term: field=%s text=%s", stq.Term().Field, stq.Term().Text())
	}
}

// TestSpanOrQueryNodeBuilder verifies that SpanOrQueryNodeBuilder builds
// a SpanOrQuery from an OrQueryNode containing FieldQueryNode children.
func TestSpanOrQueryNodeBuilder(t *testing.T) {
	builder := &spanparser.SpanOrQueryNodeBuilder{}

	child1 := flexible.NewFieldQueryNode("body", "hello", 0, 5)
	child2 := flexible.NewFieldQueryNode("body", "world", 0, 5)
	orNode := flexible.NewOrQueryNode([]flexible.QueryNode{child1, child2})

	q, err := builder.Build(orNode)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	soq, ok := q.(*search.SpanOrQuery)
	if !ok {
		t.Fatalf("expected SpanOrQuery, got %T", q)
	}
	clauses := soq.Clauses()
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
}

// TestSpansQueryTreeBuilder verifies the tree builder dispatches correctly.
func TestSpansQueryTreeBuilder(t *testing.T) {
	builder := spanparser.NewSpansQueryTreeBuilder()

	// FieldQueryNode → SpanTermQuery
	node := flexible.NewFieldQueryNode("body", "test", 0, 4)
	q, err := builder.Build(node)
	if err != nil {
		t.Fatalf("Build field node error: %v", err)
	}
	if _, ok := q.(*search.SpanTermQuery); !ok {
		t.Fatalf("expected SpanTermQuery, got %T", q)
	}

	// OrQueryNode → SpanOrQuery
	child1 := flexible.NewFieldQueryNode("body", "a", 0, 1)
	child2 := flexible.NewFieldQueryNode("body", "b", 0, 1)
	orNode := flexible.NewOrQueryNode([]flexible.QueryNode{child1, child2})
	q, err = builder.Build(orNode)
	if err != nil {
		t.Fatalf("Build or node error: %v", err)
	}
	if _, ok := q.(*search.SpanOrQuery); !ok {
		t.Fatalf("expected SpanOrQuery, got %T", q)
	}
}
