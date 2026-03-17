// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewMinHashFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewMinHashFilter(tokenizer, 2, 3)

	if filter.GetHashCount() != 2 {
		t.Errorf("expected hashCount=2, got %d", filter.GetHashCount())
	}

	if filter.GetBucketCount() != 3 {
		t.Errorf("expected bucketCount=3, got %d", filter.GetBucketCount())
	}
}

func TestMinHashFilter_IncrementToken(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))
	filter := NewMinHashFilter(tokenizer, 2, 3)

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

	// Should have 6 tokens (2 hash functions * 3 buckets)
	if len(tokens) != 6 {
		t.Errorf("expected 6 tokens, got %d: %v", len(tokens), tokens)
		return
	}

	// All tokens should be non-empty (hash strings)
	for i, token := range tokens {
		if token == "" {
			t.Errorf("token[%d] is empty", i)
		}
	}
}

func TestMinHashFilter_MultipleTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))
	filter := NewMinHashFilter(tokenizer, 1, 2)

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

	// Should have 4 tokens (2 input tokens * 1 hash * 2 buckets)
	if len(tokens) != 4 {
		t.Errorf("expected 4 tokens, got %d: %v", len(tokens), tokens)
		return
	}
}

func TestMinHashFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	filter := NewMinHashFilter(tokenizer, 2, 3)

	hasToken, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken {
		t.Error("expected no tokens for empty input")
	}
}

func TestMinHashFilterFactory(t *testing.T) {
	factory := NewMinHashFilterFactory(3, 5)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	mhf, ok := filter.(*MinHashFilter)
	if !ok {
		t.Fatal("expected MinHashFilter from factory")
	}

	if mhf.GetHashCount() != 3 {
		t.Errorf("expected hashCount=3 from factory, got %d", mhf.GetHashCount())
	}

	if mhf.GetBucketCount() != 5 {
		t.Errorf("expected bucketCount=5 from factory, got %d", mhf.GetBucketCount())
	}
}

func TestHashToString(t *testing.T) {
	tests := []struct {
		hash     uint64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{61, "Z"},
		{62, "10"},
		{12345, "3D7"},
	}

	for _, tt := range tests {
		result := hashToString(tt.hash)
		if result != tt.expected {
			t.Errorf("hashToString(%d) = %q, expected %q", tt.hash, result, tt.expected)
		}
	}
}
