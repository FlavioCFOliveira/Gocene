// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestTokenFilter_Chaining tests token filter chaining.
// Source: TestTokenFilter.java
// Purpose: Tests that filters can be chained together.
func TestTokenFilter_Chaining(t *testing.T) {
	// Create a simple pipeline: WhitespaceTokenizer -> LowerCaseFilter -> StopFilter
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("The Quick Brown Fox"))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	stopFilter := NewStopFilter(lowerFilter, []string{"the", "a", "an"})

	defer stopFilter.Close()

	var tokens []string
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"quick", "brown", "fox"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], token)
		}
	}
}

// TestTokenFilter_InputPropagation tests that input is properly propagated.
// Source: TestTokenFilter.java
// Purpose: Tests that the input TokenStream is properly wrapped.
func TestTokenFilter_InputPropagation(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLowerCaseFilter(tokenizer)

	if filter.GetInput() != tokenizer {
		t.Error("GetInput() should return the wrapped tokenizer")
	}
}

// TestTokenFilter_EndDelegation tests that End() is delegated to input.
// Source: TestTokenFilter.java
// Purpose: Tests end-of-stream operations are delegated.
func TestTokenFilter_EndDelegation(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := NewLowerCaseFilter(tokenizer)
	defer filter.Close()

	// Consume all tokens
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Call End() - should be delegated to input without error
	err := filter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestTokenFilter_CloseDelegation tests that Close() is delegated to input.
// Source: TestTokenFilter.java
// Purpose: Tests that resources are properly released.
func TestTokenFilter_CloseDelegation(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLowerCaseFilter(tokenizer)

	// Close should not panic and should delegate to input
	err := filter.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestTokenFilter_AttributeSharing tests that attributes are shared with input.
// Source: TestTokenFilter.java
// Purpose: Tests that filters share AttributeSource with their input.
func TestTokenFilter_AttributeSharing(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("TEST"))

	filter := NewLowerCaseFilter(tokenizer)
	defer filter.Close()

	// Both should share the same AttributeSource
	tokenizerAttrSrc := tokenizer.GetAttributeSource()
	filterAttrSrc := filter.GetAttributeSource()

	if tokenizerAttrSrc != filterAttrSrc {
		t.Error("Filter should share AttributeSource with input")
	}
}

// TestTokenFilterFactory tests TokenFilterFactory implementations.
func TestTokenFilterFactory(t *testing.T) {
	factory := NewLowerCaseFilterFactory()
	tokenizer := NewWhitespaceTokenizer()

	filter := factory.Create(tokenizer)
	if filter == nil {
		t.Error("Factory.Create() should return a non-nil filter")
	}

	// Verify it's the correct type
	if _, ok := filter.(*LowerCaseFilter); !ok {
		t.Error("Factory should create LowerCaseFilter instances")
	}
}

// TestBaseTokenFilter_Implementation tests BaseTokenFilter methods.
func TestBaseTokenFilter_Implementation(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	baseFilter := NewBaseTokenFilter(tokenizer)

	if baseFilter.GetInput() != tokenizer {
		t.Error("GetInput() should return the wrapped tokenizer")
	}

	// Test Close
	err := baseFilter.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
