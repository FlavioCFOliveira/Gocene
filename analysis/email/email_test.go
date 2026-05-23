// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package email

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// analyzeToTokens builds a UAX29URLEmailTokenizer → LowerCaseFilter → StopFilter
// pipeline and collects term strings.
func analyzeToTokens(t *testing.T, a *UAX29URLEmailAnalyzer, text string) []string {
	t.Helper()
	tok := analysis.NewUAX29URLEmailTokenizer()
	tok.SetMaxTokenLength(a.GetMaxTokenLength())
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	lc := analysis.NewLowerCaseFilter(tok)

	var stopSlice []string
	if sw := a.GetStopWords(); sw != nil {
		stopSlice = sw.Items()
	}
	sf := analysis.NewStopFilter(lc, stopSlice)
	defer sf.Close()

	src := sf.GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	if attr := src.GetAttribute(analysis.CharTermAttributeType); attr != nil {
		termAttr = attr.(analysis.CharTermAttribute)
	}

	var terms []string
	for {
		ok, err := sf.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if termAttr != nil {
			terms = append(terms, termAttr.String())
		}
	}
	return terms
}

// ---------------------------------------------------------------------------
// TestUAX29URLEmailAnalyzer
// Source: TestUAX29URLEmailAnalyzer.java
// ---------------------------------------------------------------------------

// TestUAX29URLEmailAnalyzer_Basic verifies basic tokenization with stop words.
// Source: TestUAX29URLEmailAnalyzer.testHugeDoc (simplified)
func TestUAX29URLEmailAnalyzer_Basic(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	got := analyzeToTokens(t, a, "testing 1234")
	want := []string{"testing", "1234"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d got %d; %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] want %q got %q", i, w, got[i])
		}
	}
}

// TestUAX29URLEmailAnalyzer_EmailPreserved verifies email addresses are kept intact.
func TestUAX29URLEmailAnalyzer_EmailPreserved(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	got := analyzeToTokens(t, a, "contact user@example.com for help")
	found := false
	for _, tok := range got {
		if tok == "user@example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected user@example.com in tokens; got %v", got)
	}
}

// TestUAX29URLEmailAnalyzer_URLPreserved verifies URLs are kept intact.
func TestUAX29URLEmailAnalyzer_URLPreserved(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	got := analyzeToTokens(t, a, "visit https://example.com today")
	found := false
	for _, tok := range got {
		if tok == "https://example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected https://example.com in tokens; got %v", got)
	}
}

// TestUAX29URLEmailAnalyzer_StopWords verifies English stop words are removed.
func TestUAX29URLEmailAnalyzer_StopWords(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	got := analyzeToTokens(t, a, "the quick brown fox")
	for _, tok := range got {
		if tok == "the" {
			t.Errorf("stop word 'the' should be removed; got %v", got)
		}
	}
}

// TestUAX29URLEmailAnalyzer_Lowercase verifies tokens are lowercased.
func TestUAX29URLEmailAnalyzer_Lowercase(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	got := analyzeToTokens(t, a, "Hello World")
	for _, tok := range got {
		for _, r := range tok {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("token %q contains uppercase after lowercasing", tok)
			}
		}
	}
}

// TestUAX29URLEmailAnalyzer_MaxTokenLength verifies token length truncation.
func TestUAX29URLEmailAnalyzer_MaxTokenLength(t *testing.T) {
	a := NewUAX29URLEmailAnalyzer()
	a.SetMaxTokenLength(5)
	got := analyzeToTokens(t, a, "short toolong word")
	for _, tok := range got {
		if len([]rune(tok)) > 5 {
			t.Errorf("token %q exceeds maxTokenLength 5", tok)
		}
	}
}

// TestUAX29URLEmailAnalyzer_CustomStopWords verifies a custom stop-word set.
func TestUAX29URLEmailAnalyzer_CustomStopWords(t *testing.T) {
	stopWords := analysis.GetWordSetFromStrings([]string{"foo", "bar"}, false)
	a := NewUAX29URLEmailAnalyzerWithStopWords(stopWords)
	got := analyzeToTokens(t, a, "foo bar baz")
	for _, tok := range got {
		if tok == "foo" || tok == "bar" {
			t.Errorf("custom stop word %q should be removed; got %v", tok, got)
		}
	}
	found := false
	for _, tok := range got {
		if tok == "baz" {
			found = true
		}
	}
	if !found {
		t.Errorf("non-stop-word 'baz' should be present; got %v", got)
	}
}

// TestUAX29URLEmailTokenizerImpl_Construct verifies the impl stub can be constructed.
func TestUAX29URLEmailTokenizerImpl_Construct(t *testing.T) {
	impl := NewUAX29URLEmailTokenizerImpl()
	if impl == nil {
		t.Fatal("expected non-nil impl")
	}
}
