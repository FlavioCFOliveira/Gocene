// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestTeeSinkTokenFilter_Basic tests basic tee functionality.
// Source: TestTeeSinkTokenFilter.java
// Purpose: Tests that tokens are teed to multiple sinks.
func TestTeeSinkTokenFilter_Basic(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))

	teeFilter := NewTeeSinkTokenFilter(tokenizer)
	sink1 := teeFilter.NewSinkTokenStream()
	sink2 := teeFilter.NewSinkTokenStream()

	defer teeFilter.Close()

	// Consume all tokens
	for {
		hasToken, err := teeFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Verify both sinks received all tokens
	if sink1.Size() != 3 {
		t.Errorf("Sink1 should have 3 tokens, got %d", sink1.Size())
	}

	if sink2.Size() != 3 {
		t.Errorf("Sink2 should have 3 tokens, got %d", sink2.Size())
	}

	// Verify token content
	tokens1 := sink1.GetTokens()
	if tokens1[0].Term != "one" {
		t.Errorf("Expected 'one', got %q", tokens1[0].Term)
	}
	if tokens1[1].Term != "two" {
		t.Errorf("Expected 'two', got %q", tokens1[1].Term)
	}
	if tokens1[2].Term != "three" {
		t.Errorf("Expected 'three', got %q", tokens1[2].Term)
	}
}

// TestTeeSinkTokenFilter_MultipleSinks tests multiple sinks.
// Source: TestTeeSinkTokenFilter.java
// Purpose: Tests that multiple sinks can be created.
func TestTeeSinkTokenFilter_MultipleSinks(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b"))

	teeFilter := NewTeeSinkTokenFilter(tokenizer)

	// Create multiple sinks
	sinks := make([]*TokenSink, 5)
	for i := range sinks {
		sinks[i] = teeFilter.NewSinkTokenStream()
	}

	defer teeFilter.Close()

	// Consume all tokens
	for {
		hasToken, err := teeFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Verify all sinks have the tokens
	for i, sink := range sinks {
		if sink.Size() != 2 {
			t.Errorf("Sink %d should have 2 tokens, got %d", i, sink.Size())
		}
	}
}

// TestTeeSinkTokenFilter_SinkIteration tests sink iteration.
// Source: TestTeeSinkTokenFilter.java
// Purpose: Tests that sinks can iterate over tokens.
func TestTeeSinkTokenFilter_SinkIteration(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("x y z"))

	teeFilter := NewTeeSinkTokenFilter(tokenizer)
	sink := teeFilter.NewSinkTokenStream()

	defer teeFilter.Close()

	// Consume all tokens
	for {
		hasToken, err := teeFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Iterate over sink
	var terms []string
	for {
		hasToken, token := sink.IncrementToken()
		if !hasToken {
			break
		}
		terms = append(terms, token.Term)
	}

	if len(terms) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(terms))
	}

	if terms[0] != "x" || terms[1] != "y" || terms[2] != "z" {
		t.Errorf("Expected ['x', 'y', 'z'], got %v", terms)
	}
}

// TestTeeSinkTokenFilter_SinkReset tests sink reset.
// Source: TestTeeSinkTokenFilter.java
// Purpose: Tests that sinks can be reset for re-iteration.
func TestTeeSinkTokenFilter_SinkReset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two"))

	teeFilter := NewTeeSinkTokenFilter(tokenizer)
	sink := teeFilter.NewSinkTokenStream()

	defer teeFilter.Close()

	// Consume all tokens
	for {
		hasToken, err := teeFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// First iteration
	count1 := 0
	for {
		hasToken, _ := sink.IncrementToken()
		if !hasToken {
			break
		}
		count1++
	}

	// Reset and second iteration
	sink.Reset()

	count2 := 0
	for {
		hasToken, _ := sink.IncrementToken()
		if !hasToken {
			break
		}
		count2++
	}

	if count1 != 2 {
		t.Errorf("First iteration: expected 2 tokens, got %d", count1)
	}

	if count2 != 2 {
		t.Errorf("Second iteration: expected 2 tokens, got %d", count2)
	}
}

// TestTeeSinkTokenFilter_Attributes tests that attributes are captured.
// Source: TestTeeSinkTokenFilter.java
// Purpose: Tests that token attributes are properly captured.
func TestTeeSinkTokenFilter_Attributes(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	teeFilter := NewTeeSinkTokenFilter(tokenizer)
	sink := teeFilter.NewSinkTokenStream()

	defer teeFilter.Close()

	// Consume all tokens
	for {
		hasToken, err := teeFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	tokens := sink.GetTokens()
	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d", len(tokens))
	}

	if tokens[0].Term != "hello" {
		t.Errorf("Expected term 'hello', got %q", tokens[0].Term)
	}
}
