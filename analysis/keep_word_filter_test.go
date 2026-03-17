// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewKeepWordFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	keepWords := map[string]bool{"hello": true, "world": true}
	filter := NewKeepWordFilter(tokenizer, keepWords)

	if len(filter.GetKeepWords()) != 2 {
		t.Errorf("expected 2 keep words, got %d", len(filter.GetKeepWords()))
	}
}

func TestKeepWordFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		keepWords map[string]bool
		expected  []string
	}{
		{
			name:      "keep_specific_words",
			input:     "hello world foo bar",
			keepWords: map[string]bool{"hello": true, "world": true},
			expected:  []string{"hello", "world"},
		},
		{
			name:      "keep_all_words",
			input:     "hello world",
			keepWords: map[string]bool{"hello": true, "world": true},
			expected:  []string{"hello", "world"},
		},
		{
			name:      "keep_no_words",
			input:     "hello world",
			keepWords: map[string]bool{},
			expected:  []string{},
		},
		{
			name:      "case_sensitive",
			input:     "Hello hello HELLO",
			keepWords: map[string]bool{"hello": true},
			expected:  []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewKeepWordFilter(tokenizer, tt.keepWords)

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					tokens = append(tokens, attr.(CharTermAttribute).String())
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i, exp := range tt.expected {
				if tokens[i] != exp {
					t.Errorf("expected token[%d]=%q, got %q", i, exp, tokens[i])
				}
			}
		})
	}
}

func TestKeepWordFilterFactory(t *testing.T) {
	keepWords := map[string]bool{"test": true}
	factory := NewKeepWordFilterFactory(keepWords)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	kwf, ok := filter.(*KeepWordFilter)
	if !ok {
		t.Fatal("expected KeepWordFilter from factory")
	}

	if len(kwf.GetKeepWords()) != 1 {
		t.Errorf("expected 1 keep word from factory, got %d", len(kwf.GetKeepWords()))
	}
}
