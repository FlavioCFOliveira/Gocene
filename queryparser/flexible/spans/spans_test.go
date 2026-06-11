// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans_test

import (
	"testing"

	spanparser "github.com/FlavioCFOliveira/Gocene/queryparser/flexible/spans"
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
