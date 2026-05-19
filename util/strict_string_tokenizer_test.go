// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestStrictStringTokenizer_VersionLike mirrors how Lucene's Version parser
// drives the tokenizer: split a canonical "major.minor.bugfix" string and
// expect exactly three tokens in order.
func TestStrictStringTokenizer_VersionLike(t *testing.T) {
	t.Parallel()

	tok := NewStrictStringTokenizer("10.4.0", '.')
	want := []string{"10", "4", "0"}

	for i, w := range want {
		if !tok.HasMoreTokens() {
			t.Fatalf("token %d: HasMoreTokens=false, want true", i)
		}
		if got := tok.NextToken(); got != w {
			t.Fatalf("token %d: got %q, want %q", i, got, w)
		}
	}
	if tok.HasMoreTokens() {
		t.Fatalf("HasMoreTokens=true after consuming all tokens")
	}
}

// TestStrictStringTokenizer_PreservesEmpty validates the contract that
// distinguishes this tokenizer from java.util.StringTokenizer: empty tokens
// between consecutive delimiters and at the boundaries are returned.
func TestStrictStringTokenizer_PreservesEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		delim byte
		want  []string
	}{
		{"leading_empty", ".a", '.', []string{"", "a"}},
		{"trailing_empty", "a.", '.', []string{"a", ""}},
		{"both_empty", ".a.", '.', []string{"", "a", ""}},
		{"only_delimiters", "...", '.', []string{"", "", "", ""}},
		{"empty_input", "", '.', []string{""}},
		{"no_delimiters", "abc", '.', []string{"abc"}},
		{"consecutive_delims", "a..b", '.', []string{"a", "", "b"}},
		{"non_dot_delim", "a-b-c", '-', []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok := NewStrictStringTokenizer(tc.input, tc.delim)
			var got []string
			for tok.HasMoreTokens() {
				got = append(got, tok.NextToken())
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len=%d, want %d (got=%q want=%q)", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("token %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestStrictStringTokenizer_NextAfterExhaustedPanics enforces the
// "fail-fast on programmer error" contract: calling NextToken after exhaustion
// must panic, matching Java's IllegalStateException("no more tokens").
func TestStrictStringTokenizer_NextAfterExhaustedPanics(t *testing.T) {
	t.Parallel()

	tok := NewStrictStringTokenizer("a", '.')
	if got := tok.NextToken(); got != "a" {
		t.Fatalf("first NextToken: got %q, want %q", got, "a")
	}
	if tok.HasMoreTokens() {
		t.Fatalf("HasMoreTokens=true after single token consumed")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on NextToken after exhaustion, got none")
		}
	}()
	_ = tok.NextToken()
}

// TestStrictStringTokenizer_NonASCIIDelimiterPanics documents the
// ASCII-delimiter restriction at construction time.
func TestStrictStringTokenizer_NonASCIIDelimiterPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for non-ASCII delimiter, got none")
		}
	}()
	_ = NewStrictStringTokenizer("ignored", 0x80)
}

// TestStrictStringTokenizer_NonASCIIPayloadIsTransparent verifies that the
// byte-indexed scan transparently carries multi-byte UTF-8 sequences within
// tokens. The delimiter is ASCII; the payload contains "é" (0xC3 0xA9).
func TestStrictStringTokenizer_NonASCIIPayloadIsTransparent(t *testing.T) {
	t.Parallel()

	tok := NewStrictStringTokenizer("café.naïve", '.')
	if got := tok.NextToken(); got != "café" {
		t.Fatalf("first token: got %q, want %q", got, "café")
	}
	if got := tok.NextToken(); got != "naïve" {
		t.Fatalf("second token: got %q, want %q", got, "naïve")
	}
	if tok.HasMoreTokens() {
		t.Fatalf("HasMoreTokens=true after consuming all tokens")
	}
}
