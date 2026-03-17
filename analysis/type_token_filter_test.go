// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewTypeTokenFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	types := map[string]bool{"word": true}
	filter := NewTypeTokenFilter(tokenizer, types, true)

	if !filter.IsUseWhitelist() {
		t.Error("expected useWhitelist to be true")
	}

	if len(filter.GetTypes()) != 1 {
		t.Errorf("expected 1 type, got %d", len(filter.GetTypes()))
	}
}

func TestTypeTokenFilter_Whitelist(t *testing.T) {
	tests := []struct {
		name     string
		types    map[string]bool
		expected []string
	}{
		{
			name:     "keep_only_word_type",
			types:    map[string]bool{"word": true},
			expected: []string{"hello", "world"},
		},
		{
			name:     "keep_multiple_types",
			types:    map[string]bool{"word": true, "number": true},
			expected: []string{"hello", "world"},
		},
		{
			name:     "empty_types_keeps_nothing",
			types:    map[string]bool{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader("hello world"))
			filter := NewTypeTokenFilter(tokenizer, tt.types, true)

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

func TestTypeTokenFilter_Blacklist(t *testing.T) {
	tests := []struct {
		name     string
		types    map[string]bool
		expected []string
	}{
		{
			name:     "remove_word_type",
			types:    map[string]bool{"word": true},
			expected: []string{},
		},
		{
			name:     "empty_types_keeps_all",
			types:    map[string]bool{},
			expected: []string{"hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader("hello world"))
			filter := NewTypeTokenFilter(tokenizer, tt.types, false)

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

func TestTypeTokenFilterFactory(t *testing.T) {
	types := map[string]bool{"word": true}
	factory := NewTypeTokenFilterFactory(types, true)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	ttf, ok := filter.(*TypeTokenFilter)
	if !ok {
		t.Fatal("expected TypeTokenFilter from factory")
	}

	if !ttf.IsUseWhitelist() {
		t.Error("expected useWhitelist=true from factory")
	}
}
