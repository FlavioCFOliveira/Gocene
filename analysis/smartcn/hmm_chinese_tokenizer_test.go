// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"strings"
	"testing"
)

// TestHMMChineseTokenizerBasic verifies that the tokenizer produces tokens
// from a mixed Chinese-English sentence.
func TestHMMChineseTokenizerBasic(t *testing.T) {
	tok, err := NewHMMChineseTokenizer()
	if err != nil {
		t.Fatalf("NewHMMChineseTokenizer: %v", err)
	}

	input := "我们是朋友 hello"
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	var tokenCount int
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		tokenCount++
	}

	if tokenCount == 0 {
		t.Error("expected at least one token, got none")
	}
	t.Logf("token count: %d", tokenCount)

	if err := tok.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestHMMChineseTokenizerReset verifies that Reset clears the tokenizer state.
func TestHMMChineseTokenizerReset(t *testing.T) {
	tok, err := NewHMMChineseTokenizer()
	if err != nil {
		t.Fatalf("NewHMMChineseTokenizer: %v", err)
	}

	for _, sentence := range []string{"中文测试", "hello world"} {
		if err := tok.SetReader(strings.NewReader(sentence)); err != nil {
			t.Fatalf("SetReader(%q): %v", sentence, err)
		}
		if err := tok.Reset(); err != nil {
			t.Fatalf("Reset(%q): %v", sentence, err)
		}
		count := 0
		for {
			ok, err := tok.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken(%q): %v", sentence, err)
			}
			if !ok {
				break
			}
			count++
		}
		if count == 0 {
			t.Errorf("sentence %q produced no tokens", sentence)
		}
	}
}
