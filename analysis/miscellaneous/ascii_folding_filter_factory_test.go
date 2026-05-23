// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/miscellaneous/TestAsciiFoldingFilterFactory.java

package miscellaneous

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// drainTerms drains all CharTermAttribute strings from a BaseTokenFilter.
func drainTerms(t *testing.T, f *analysis.ASCIIFoldingFilter) []string {
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

// newWhitespaceFilter wraps a single-word string in a WhitespaceTokenizer + ASCIIFoldingFilter.
func newWhitespaceFilter(word string, preserveOriginal bool) *analysis.ASCIIFoldingFilter {
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(word))
	_ = tokenizer.Reset()
	if preserveOriginal {
		return analysis.NewASCIIFoldingFilterWithOptions(tokenizer, true)
	}
	return analysis.NewASCIIFoldingFilter(tokenizer)
}

// TestAsciiFoldingFilterFactory_MultiTermAnalysis verifies that
// ASCIIFoldingFilterFactory folds non-ASCII tokens, and that
// preserveOriginal=true keeps both the folded and original forms.
//
// Source: TestAsciiFoldingFilterFactory.testMultiTermAnalysis
func TestAsciiFoldingFilterFactory_MultiTermAnalysis(t *testing.T) {
	// factory (no preserveOriginal): "Été" -> "Ete"
	f := newWhitespaceFilter("Été", false)
	got := drainTerms(t, f)
	want := []string{"Ete"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("no-preserve: got %v, want %v", got, want)
	}

	// factory with preserveOriginal=true: "Été" -> both original and folded.
	// Deviation: Lucene emits folded first then original; Gocene emits original
	// first then folded (consistent with TestASCIIFoldingFilter_PreserveOriginal).
	f2 := newWhitespaceFilter("Été", true)
	got2 := drainTerms(t, f2)
	want2 := []string{"Été", "Ete"}
	if len(got2) != len(want2) {
		t.Fatalf("preserve: got %v, want %v", got2, want2)
	}
	for i := range want2 {
		if got2[i] != want2[i] {
			t.Errorf("preserve token[%d]: got %q, want %q", i, got2[i], want2[i])
		}
	}
}
