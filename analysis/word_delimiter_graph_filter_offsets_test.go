// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestWordDelimiterGraphFilter_Offsets_CamelCase verifies that the filter
// reports precise per-subword character offsets, mirroring Lucene 10.4.0's
// WordDelimiterGraphFilter offset bookkeeping (rebase against the input
// token's start offset, advance by subword positions).
func TestWordDelimiterGraphFilter_Offsets_CamelCase(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))
	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	// "Power" occupies char 0..5 of "PowerShot".
	if tokens[0].text != "Power" || tokens[0].startOffset != 0 || tokens[0].endOffset != 5 {
		t.Errorf("token[0]=%+v want text=Power start=0 end=5", tokens[0])
	}
	// "Shot" occupies char 5..9.
	if tokens[1].text != "Shot" || tokens[1].startOffset != 5 || tokens[1].endOffset != 9 {
		t.Errorf("token[1]=%+v want text=Shot start=5 end=9", tokens[1])
	}

	_ = filter.End()
	_ = filter.Close()
}

// TestWordDelimiterGraphFilter_Offsets_Numerics verifies offsets across
// alpha/digit transitions where rune positions still align with UTF-16
// indices (BMP-only ASCII input).
func TestWordDelimiterGraphFilter_Offsets_Numerics(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("j2se"))
	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d (%+v)", len(tokens), tokens)
	}
	want := []struct {
		text       string
		start, end int
	}{
		{"j", 0, 1},
		{"2", 1, 2},
		{"se", 2, 4},
	}
	for i, w := range want {
		if tokens[i].text != w.text || tokens[i].startOffset != w.start || tokens[i].endOffset != w.end {
			t.Errorf("token[%d]=%+v want %+v", i, tokens[i], w)
		}
	}
}

// TestWordDelimiterGraphFilter_Offsets_Hyphen verifies that the delimiter
// character itself is not part of any emitted subword's offset span.
func TestWordDelimiterGraphFilter_Offsets_Hyphen(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power-Shot"))
	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d (%+v)", len(tokens), tokens)
	}
	// "Power" 0..5, "Shot" 6..10 (the '-' at index 5 is skipped).
	if tokens[0].startOffset != 0 || tokens[0].endOffset != 5 {
		t.Errorf("token[0] offsets=%d..%d want 0..5", tokens[0].startOffset, tokens[0].endOffset)
	}
	if tokens[1].startOffset != 6 || tokens[1].endOffset != 10 {
		t.Errorf("token[1] offsets=%d..%d want 6..10", tokens[1].startOffset, tokens[1].endOffset)
	}
}
