// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewKeywordRepeatFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewKeywordRepeatFilter(tokenizer)

	if filter == nil {
		t.Error("expected filter to be created")
	}
}

func TestKeywordRepeatFilter_IncrementToken(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))
	filter := NewKeywordRepeatFilter(tokenizer)

	var tokens []string
	var keywords []bool

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

		if attr := filter.GetAttributeSource().GetAttribute("KeywordAttribute"); attr != nil {
			keywords = append(keywords, attr.(*KeywordAttribute).IsKeywordToken())
		}
	}

	// Should have 4 tokens: hello(keyword), hello(non-keyword), world(keyword), world(non-keyword)
	if len(tokens) != 4 {
		t.Errorf("expected 4 tokens, got %d: %v", len(tokens), tokens)
		return
	}

	expectedTokens := []string{"hello", "hello", "world", "world"}
	for i, exp := range expectedTokens {
		if tokens[i] != exp {
			t.Errorf("expected token[%d]=%q, got %q", i, exp, tokens[i])
		}
	}

	// Check keyword pattern: true, false, true, false
	expectedKeywords := []bool{true, false, true, false}
	for i, exp := range expectedKeywords {
		if keywords[i] != exp {
			t.Errorf("expected keyword[%d]=%v, got %v", i, exp, keywords[i])
		}
	}
}

func TestKeywordRepeatFilter_SingleToken(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))
	filter := NewKeywordRepeatFilter(tokenizer)

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

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens for single input, got %d: %v", len(tokens), tokens)
	}

	if tokens[0] != "test" || tokens[1] != "test" {
		t.Errorf("expected both tokens to be 'test', got %v", tokens)
	}
}

func TestKeywordRepeatFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	filter := NewKeywordRepeatFilter(tokenizer)

	hasToken, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken {
		t.Error("expected no tokens for empty input")
	}
}

func TestKeywordRepeatFilterFactory(t *testing.T) {
	factory := NewKeywordRepeatFilterFactory()
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	_, ok := filter.(*KeywordRepeatFilter)
	if !ok {
		t.Fatal("expected KeywordRepeatFilter from factory")
	}
}
