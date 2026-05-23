// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ne

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// analyzeToTokens builds the NepaliAnalyzer pipeline directly and collects terms.
func analyzeToTokens(t *testing.T, a *NepaliAnalyzer, text string) []string {
	t.Helper()
	tok := analysis.NewStandardTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	lc := analysis.NewLowerCaseFilter(tok)
	dd := analysis.NewDecimalDigitFilter(lc)
	var stopSlice []string
	if sw := a.GetStopWords(); sw != nil {
		stopSlice = sw.Items()
	}
	sf := analysis.NewStopFilter(dd, stopSlice)
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
// Tests
// Source: TestNepaliAnalyzer.java
// ---------------------------------------------------------------------------

// TestNepaliAnalyzer_ResourcesAvailable verifies analyzer can be constructed.
// Source: TestNepaliAnalyzer.testResourcesAvailable
func TestNepaliAnalyzer_ResourcesAvailable(t *testing.T) {
	a := NewNepaliAnalyzer()
	if a == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

// TestNepaliAnalyzer_LowerCase verifies Latin characters are lowercased.
// Source: TestNepaliAnalyzer.testLowerCase
func TestNepaliAnalyzer_LowerCase(t *testing.T) {
	a := NewNepaliAnalyzer()
	got := analyzeToTokens(t, a, "FIFA")
	if len(got) != 1 || got[0] != "fifa" {
		t.Errorf("expected [\"fifa\"], got %v", got)
	}
}

// TestNepaliAnalyzer_Digits verifies Nepali digits are folded to ASCII.
// Source: TestNepaliAnalyzer.testDigits
func TestNepaliAnalyzer_Digits(t *testing.T) {
	a := NewNepaliAnalyzer()
	got := analyzeToTokens(t, a, "१२३४")
	if len(got) != 1 || got[0] != "1234" {
		t.Errorf("expected [\"1234\"], got %v", got)
	}
}

// TestNepaliAnalyzer_StopWords verifies Nepali stop words are removed.
func TestNepaliAnalyzer_StopWords(t *testing.T) {
	a := NewNepaliAnalyzer()
	// "र" is a stop word; "नेपाल" is not.
	got := analyzeToTokens(t, a, "नेपाल र भारत")
	for _, tok := range got {
		if tok == "र" {
			t.Errorf("stop word 'र' should be removed; got %v", got)
		}
	}
}

// TestNepaliAnalyzer_CustomStopWords verifies custom stop words.
func TestNepaliAnalyzer_CustomStopWords(t *testing.T) {
	stopWords := analysis.GetWordSetFromStrings([]string{"foo", "bar"}, false)
	a := NewNepaliAnalyzerWithStopWords(stopWords)
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
		t.Errorf("'baz' should be in output; got %v", got)
	}
}

// TestNepaliAnalyzer_GetDefaultStopSet verifies stop set is non-empty.
func TestNepaliAnalyzer_GetDefaultStopSet(t *testing.T) {
	ss := GetDefaultStopSet()
	if ss == nil || ss.Size() == 0 {
		t.Fatal("default stop set should be non-empty")
	}
}
