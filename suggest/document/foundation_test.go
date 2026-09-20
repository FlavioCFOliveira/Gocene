// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

func TestCompletionAnalyzer(t *testing.T) {
	analyzer := NewCompletionAnalyzer(analysis.NewStandardAnalyzer())

	// Test that it wraps the analyzer and provides a CompletionTokenStream
	ts := analyzer.TokenStream("field", nil) // Using nil reader for simplicity in foundation test
	if _, ok := ts.(*CompletionTokenStream); !ok {
		t.Errorf("Expected CompletionTokenStream, got %T", ts)
	}
}

func TestSuggestField(t *testing.T) {
	field := NewSuggestField("name", "suggestion", 4)
	if field.Type() != SuggestFieldTYPE {
		t.Errorf("Expected type %v, got %v", SuggestFieldTYPE, field.Type())
	}

	payload := field.buildSuggestPayload()
	if len(payload) == 0 {
		t.Error("Payload should not be empty")
	}
}

func TestContextSuggestField(t *testing.T) {
	field := NewContextSuggestField("name", "suggestion", 4, "ctx1", "ctx2")
	if field.Type() != CONTEXT_TYPE {
		t.Errorf("Expected type %v, got %v", CONTEXT_TYPE, field.Type())
	}
}

func TestReservedCharacters(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for reserved characters")
		}
	}()
	NewSuggestField("name", "suggestion\x1f", 4)
}
