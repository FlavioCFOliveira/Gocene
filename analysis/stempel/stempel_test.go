// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package stempel

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// attrSourcer is the minimal interface for accessing the attribute source from
// a BaseTokenStream or BaseTokenFilter.
type attrSourcer interface {
	GetAttributeSource() *util.AttributeSource
}

// collectTokens drives a TokenStream to completion and returns the term strings.
func collectTokens(stream analysis.TokenStream) ([]string, error) {
	defer stream.Close()
	src, ok := stream.(attrSourcer)
	if !ok {
		return nil, nil
	}
	attrSrc := src.GetAttributeSource()

	var tokens []string
	for {
		hasToken, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}
		a := attrSrc.GetAttribute(analysis.CharTermAttributeType)
		if a == nil {
			continue
		}
		attr := a.(analysis.CharTermAttribute)
		tokens = append(tokens, string(attr.Buffer()[:attr.Length()]))
	}
	return tokens, nil
}

// analyzeText runs the analyzer on text and returns the tokens.
func analyzeText(a analysis.Analyzer, text string) ([]string, error) {
	stream, err := a.TokenStream("field", strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	return collectTokens(stream)
}

// ---- StempelPolishStemFilterFactory tests ------------------------------------

// TestStempelPolishStemFilterFactory_Basics mirrors
// TestStempelPolishStemFilterFactory.testBasics.
func TestStempelPolishStemFilterFactory_Basics(t *testing.T) {
	factory, err := NewStempelPolishStemFilterFactory(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build a simple whitespace tokenizer + factory pipeline.
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("studenta studenci"))
	stream := factory.Create(tokenizer)

	tokens, err := collectTokens(stream)
	if err != nil {
		t.Fatalf("token collection failed: %v", err)
	}
	want := []string{"student", "student"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens %v, want %v", len(tokens), tokens, want)
	}
	for i, tok := range tokens {
		if tok != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, want[i])
		}
	}
}

// TestStempelPolishStemFilterFactory_BogusArguments mirrors
// TestStempelPolishStemFilterFactory.testBogusArguments.
func TestStempelPolishStemFilterFactory_BogusArguments(t *testing.T) {
	_, err := NewStempelPolishStemFilterFactory(map[string]string{"bogusArg": "bogusValue"})
	if err == nil {
		t.Fatal("expected error for unknown parameter, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error message %q should contain 'unknown'", err.Error())
	}
}

// ---- PolishAnalyzer tests ----------------------------------------------------

// TestPolishAnalyzer_ResourcesAvailable mirrors
// TestPolishAnalyzer.testResourcesAvailable.
func TestPolishAnalyzer_ResourcesAvailable(t *testing.T) {
	a := NewPolishAnalyzer()
	a.Close()
}

// TestPolishAnalyzer_Basics mirrors TestPolishAnalyzer.testBasics.
func TestPolishAnalyzer_Basics(t *testing.T) {
	a := NewPolishAnalyzer()
	defer a.Close()

	// stemming
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"studenta", "student"},
		{"studenci", "student"},
	} {
		tokens, err := analyzeText(a, tc.input)
		if err != nil {
			t.Fatalf("analyzeText(%q): %v", tc.input, err)
		}
		if len(tokens) != 1 || tokens[0] != tc.want {
			t.Errorf("stem(%q) = %v, want [%q]", tc.input, tokens, tc.want)
		}
	}

	// stopword removed ("był")
	tokens, err := analyzeText(a, "był")
	if err != nil {
		t.Fatalf("analyzeText(był): %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("stopword 'był' should produce no tokens, got %v", tokens)
	}
}

// TestPolishAnalyzer_Exclude mirrors TestPolishAnalyzer.testExclude.
func TestPolishAnalyzer_Exclude(t *testing.T) {
	exclusionSet := analysis.NewCharArraySet(4, false)
	exclusionSet.Add("studenta")
	a := NewPolishAnalyzerFull(GetDefaultStopSet(), exclusionSet)
	defer a.Close()

	// excluded token should not be stemmed
	tokens, err := analyzeText(a, "studenta")
	if err != nil {
		t.Fatalf("analyzeText(studenta): %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "studenta" {
		t.Errorf("excluded word should stay as 'studenta', got %v", tokens)
	}

	// non-excluded token should be stemmed
	tokens, err = analyzeText(a, "studenci")
	if err != nil {
		t.Fatalf("analyzeText(studenci): %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "student" {
		t.Errorf("stem(studenci) = %v, want [student]", tokens)
	}
}
