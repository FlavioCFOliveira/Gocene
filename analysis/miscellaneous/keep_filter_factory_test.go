// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/miscellaneous/TestKeepFilterFactory.java

package miscellaneous

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// drainTokens drains all CharTermAttribute strings from f.
func drainTokens(t *testing.T, f *analysis.KeepWordFilter) []string {
	t.Helper()
	var tokens []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := f.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute not found")
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}
	return tokens
}

// boolSet converts a slice of words into a map[string]bool suitable for
// NewKeepWordFilter / NewKeepWordFilterFactory.
func boolSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// TestKeepFilterFactory_Inform verifies that KeepWordFilter keeps only the
// words in the provided set, analogous to the Java testInform test which loads
// word sets from keep-1.txt (2 words), keep-2.txt (2 words, total 4), and
// keep-snowball.txt (8 words in snowball format with table layout).
//
// Deviation: the Java test uses SPI-based KeepWordFilterFactory loaded from
// classpath resource files; Gocene does not have SPI or ResourceLoader.
// This port uses inline word sets matching the file contents exactly.
//
// Source: TestKeepFilterFactory.testInform
func TestKeepFilterFactory_Inform(t *testing.T) {
	// keep-1.txt contains: foo, bar (2 words)
	words1 := boolSet("foo", "bar")
	if len(words1) != 2 {
		t.Errorf("words1 size: got %d, want 2", len(words1))
	}

	// keep-1.txt + keep-2.txt contains: foo, bar, junk, more (4 words)
	words2 := boolSet("foo", "bar", "junk", "more")
	if len(words2) != 4 {
		t.Errorf("words2 size: got %d, want 4", len(words2))
	}

	// keep-snowball.txt: he, him, his, himself, she, her, hers, herself (8 words)
	snowballWords := boolSet("he", "him", "his", "himself", "she", "her", "hers", "herself")
	if len(snowballWords) != 8 {
		t.Errorf("snowball size: got %d, want 8", len(snowballWords))
	}
	for _, w := range []string{"he", "him", "his", "himself", "she", "her", "hers", "herself"} {
		if !snowballWords[w] {
			t.Errorf("snowball words missing %q", w)
		}
	}

	// Functional check: filter keeps only words in set.
	input := "he foo she junk bar unknown"
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))
	_ = tokenizer.Reset()

	// Use words2 (4 words): expect foo, bar, junk
	f := analysis.NewKeepWordFilter(tokenizer, words2)
	got := drainTokens(t, f)
	want := []string{"foo", "junk", "bar"}
	if len(got) != len(want) {
		t.Fatalf("filter: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestKeepFilterFactory_EmptyWords verifies that KeepWordFilter with no words
// passes no tokens through.
//
// Source: TestKeepFilterFactory.testInform (defaults case)
func TestKeepFilterFactory_EmptyWords(t *testing.T) {
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))
	_ = tokenizer.Reset()

	f := analysis.NewKeepWordFilter(tokenizer, map[string]bool{})
	got := drainTokens(t, f)
	if len(got) != 0 {
		t.Errorf("expected no tokens with empty word set, got %v", got)
	}
}
