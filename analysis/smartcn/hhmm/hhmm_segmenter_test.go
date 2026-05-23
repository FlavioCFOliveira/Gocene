// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"testing"

)

// TestHHMMSegmenterProcess verifies that the segmenter can process a simple
// Chinese sentence and returns reasonable tokens.
func TestHHMMSegmenterProcess(t *testing.T) {
	seg, err := NewHHMMSegmenter()
	if err != nil {
		t.Fatalf("NewHHMMSegmenter: %v", err)
	}

	// "我们是朋友" = "We are friends"
	sentence := "我们是朋友"
	tokens, err := seg.Process(sentence)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Expect SENTENCE_BEGIN, one or more Chinese words, SENTENCE_END.
	if len(tokens) < 3 {
		t.Errorf("Process(%q): too few tokens: %d", sentence, len(tokens))
	}

	// First token should be SENTENCE_BEGIN.
	if tokens[0].WordType != WordTypeSentenceBegin {
		t.Errorf("tokens[0].WordType = %d, want WordTypeSentenceBegin (%d)", tokens[0].WordType, WordTypeSentenceBegin)
	}

	// Last token should be SENTENCE_END.
	last := tokens[len(tokens)-1]
	if last.WordType != WordTypeSentenceEnd {
		t.Errorf("tokens[last].WordType = %d, want WordTypeSentenceEnd (%d)", last.WordType, WordTypeSentenceEnd)
	}

	// Log all tokens for inspection.
	for i, tk := range tokens {
		t.Logf("  token[%d]: %q start=%d end=%d type=%d weight=%d",
			i, string(tk.CharArray), tk.StartOffset, tk.EndOffset, tk.WordType, tk.Weight)
	}
}

// TestHHMMSegmenterEnglish verifies that the segmenter handles ASCII input.
func TestHHMMSegmenterEnglish(t *testing.T) {
	seg, err := NewHHMMSegmenter()
	if err != nil {
		t.Fatalf("NewHHMMSegmenter: %v", err)
	}

	sentence := "hello world"
	tokens, err := seg.Process(sentence)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Expect BEGIN + at least 1 string/number token + END.
	if len(tokens) < 3 {
		t.Errorf("Process(%q): too few tokens: %d", sentence, len(tokens))
	}
	for i, tk := range tokens {
		t.Logf("  token[%d]: %q start=%d end=%d type=%d", i, string(tk.CharArray), tk.StartOffset, tk.EndOffset, tk.WordType)
	}
}
