// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestCachingTokenFilter_Basic tests basic caching functionality.
// Source: TestCachingTokenFilter.java
// Purpose: Tests that tokens are cached and can be re-iterated.
func TestCachingTokenFilter_Basic(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))

	cacheFilter := NewCachingTokenFilter(tokenizer)
	defer cacheFilter.Close()

	// First pass - cache tokens
	var firstPass []string
	for {
		hasToken, err := cacheFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := cacheFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				firstPass = append(firstPass, termAttr.String())
			}
		}
	}

	expected := []string{"one", "two", "three"}
	if len(firstPass) != len(expected) {
		t.Errorf("First pass: expected %v, got %v", expected, firstPass)
	}

	// Reset and iterate again
	err := cacheFilter.Reset()
	if err != nil {
		t.Fatalf("Reset() error: %v", err)
	}

	var secondPass []string
	for {
		hasToken, err := cacheFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := cacheFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				secondPass = append(secondPass, termAttr.String())
			}
		}
	}

	if len(secondPass) != len(expected) {
		t.Errorf("Second pass: expected %v, got %v", expected, secondPass)
	}

	for i, token := range secondPass {
		if token != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], token)
		}
	}
}

// TestCachingTokenFilter_IsCached tests the IsCached method.
// Source: TestCachingTokenFilter.java
// Purpose: Tests that caching status is tracked correctly.
func TestCachingTokenFilter_IsCached(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two"))

	cacheFilter := NewCachingTokenFilter(tokenizer)
	defer cacheFilter.Close()

	if cacheFilter.IsCached() {
		t.Error("IsCached() should be false initially")
	}

	// Consume all tokens
	for {
		hasToken, err := cacheFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	if !cacheFilter.IsCached() {
		t.Error("IsCached() should be true after consuming all tokens")
	}
}

// TestCachingTokenFilter_GetCacheSize tests the GetCacheSize method.
// Source: TestCachingTokenFilter.java
// Purpose: Tests that cache size is tracked correctly.
func TestCachingTokenFilter_GetCacheSize(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))

	cacheFilter := NewCachingTokenFilter(tokenizer)
	defer cacheFilter.Close()

	if cacheFilter.GetCacheSize() != 0 {
		t.Errorf("Initial cache size should be 0, got %d", cacheFilter.GetCacheSize())
	}

	// Consume all tokens
	for {
		hasToken, err := cacheFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	if cacheFilter.GetCacheSize() != 3 {
		t.Errorf("Cache size should be 3, got %d", cacheFilter.GetCacheSize())
	}
}

// TestCachingTokenFilter_MultipleIterations tests multiple iterations over cached tokens.
// Source: TestCachingTokenFilter.java
// Purpose: Tests that the cache can be iterated multiple times.
func TestCachingTokenFilter_MultipleIterations(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	cacheFilter := NewCachingTokenFilter(tokenizer)
	defer cacheFilter.Close()

	// First iteration
	for {
		hasToken, err := cacheFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Multiple subsequent iterations
	for i := 0; i < 3; i++ {
		err := cacheFilter.Reset()
		if err != nil {
			t.Fatalf("Reset() error on iteration %d: %v", i, err)
		}

		count := 0
		for {
			hasToken, err := cacheFilter.IncrementToken()
			if err != nil {
				t.Fatalf("Error on iteration %d: %v", i, err)
			}
			if !hasToken {
				break
			}
			count++
		}

		if count != 3 {
			t.Errorf("Iteration %d: expected 3 tokens, got %d", i, count)
		}
	}
}
