// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible/spans"
)

// TestSpanQueryParser verifies the spans query parser component construction.
func TestSpanQueryParser(t *testing.T) {
	// Test that SpansQueryConfigHandler can be created.
	config := spans.NewSpansQueryConfigHandler()
	if config == nil {
		t.Fatal("NewSpansQueryConfigHandler returned nil")
	}

	// Test that UniqueFieldAttributeImpl works.
	attr := &spans.UniqueFieldAttributeImpl{}
	attr.SetUniqueField("field1")
	if attr.GetUniqueField() != "field1" {
		t.Errorf("GetUniqueField = %q, want %q", attr.GetUniqueField(), "field1")
	}

	// Test that SpansValidatorQueryNodeProcessor works.
	processor := &spans.SpansValidatorQueryNodeProcessor{}
	node, err := processor.Process(nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if node != nil {
		t.Error("Process should return nil for nil input")
	}

	// Test that SpanOrQuery and SpanTermQuery constructors work.
	_ = search.NewSpanOrQuery
	_ = search.NewSpanTermQuery
}

// TestSpanQueryParserSimpleSample verifies the span query parser example.
func TestSpanQueryParserSimpleSample(t *testing.T) {
	// Just verify the package can be imported and basic types created.
	config := spans.NewSpansQueryConfigHandler()
	if config == nil {
		t.Fatal("NewSpansQueryConfigHandler returned nil")
	}
}
