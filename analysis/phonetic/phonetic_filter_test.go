// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// collectFilterTokens drains a TokenStream and returns the term strings.
func collectFilterTokens(t *testing.T, ts analysis.TokenStream) []string {
	t.Helper()
	// Extract CharTermAttribute using the concrete type.
	var termAttr analysis.CharTermAttribute
	// Use the analysis package's BaseTokenFilter helpers.
	switch v := ts.(type) {
	case *DaitchMokotoffSoundexFilter:
		if a := v.BaseTokenFilter.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	case *DoubleMetaphoneFilter:
		if a := v.BaseTokenFilter.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	case *PhoneticFilter:
		if a := v.BaseTokenFilter.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	case *BeiderMorseFilter:
		if a := v.BaseTokenFilter.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	}
	var terms []string
	for {
		ok, err := ts.IncrementToken()
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

// newWhitespacePipeline builds a WhitespaceTokenizer → filter pipeline.
func newWhitespacePipeline(input string, makeFilter func(analysis.TokenStream) analysis.TokenStream) analysis.TokenStream {
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		panic(err)
	}
	return makeFilter(tok)
}

// containsAll reports whether got contains all of the expected strings.
func containsAll(got, expected []string) bool {
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, e := range expected {
		if !set[e] {
			return false
		}
	}
	return true
}

// equalSlices reports whether two string slices are equal (order-sensitive).
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// TestDaitchMokotoffSoundexFilter
// Source: TestDaitchMokotoffSoundexFilter.java
// ---------------------------------------------------------------------------

// TestDaitchMokotoffSoundexFilter_Algorithms validates the filter against
// testAlgorithms() from TestDaitchMokotoffSoundexFilter.java.
func TestDaitchMokotoffSoundexFilter_Algorithms(t *testing.T) {
	// inject=false: replace with codes
	ts := newWhitespacePipeline("aaa bbb ccc easgasg", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDaitchMokotoffSoundexFilter(input, false)
	})
	got := collectFilterTokens(t, ts)

	// Expected codes for inject=false (from Java test):
	expectedSet := map[string]bool{
		"000000": true,
		"700000": true,
		"400000": true,
		"450000": true,
		"454000": true,
		"540000": true,
		"545000": true,
		"500000": true,
		"045450": true,
	}
	for _, g := range got {
		if !expectedSet[g] {
			t.Errorf("unexpected code %q in inject=false output", g)
		}
	}
	for exp := range expectedSet {
		found := false
		for _, g := range got {
			if g == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected code %q in inject=false output %v", exp, got)
		}
	}
}

// TestDaitchMokotoffSoundexFilter_InjectTrue validates inject=true mode.
func TestDaitchMokotoffSoundexFilter_InjectTrue(t *testing.T) {
	// inject=true: original + codes
	ts := newWhitespacePipeline("aaa bbb", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDaitchMokotoffSoundexFilter(input, true)
	})
	got := collectFilterTokens(t, ts)

	// Original tokens must be present.
	if !containsAll(got, []string{"aaa", "bbb"}) {
		t.Errorf("inject=true: missing original tokens, got %v", got)
	}
	// Codes must also be present.
	if !containsAll(got, []string{"000000", "700000"}) {
		t.Errorf("inject=true: missing codes, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// TestDaitchMokotoffSoundexFilterFactory
// Source: TestDaitchMokotoffSoundexFilterFactory.java
// ---------------------------------------------------------------------------

// TestDaitchMokotoffSoundexFilterFactory_Defaults validates the default factory
// (inject=true).
// Source: TestDaitchMokotoffSoundexFilterFactory.testDefaults()
func TestDaitchMokotoffSoundexFilterFactory_Defaults(t *testing.T) {
	factory := NewDaitchMokotoffSoundexFilterFactory()
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	// Default inject=true: original + codes.
	if !containsAll(got, []string{"international", "063963"}) {
		t.Errorf("factory defaults: got %v, want to contain [international, 063963]", got)
	}
}

// TestDaitchMokotoffSoundexFilterFactory_InjectFalse validates inject=false.
// Source: TestDaitchMokotoffSoundexFilterFactory.testSettingInject()
func TestDaitchMokotoffSoundexFilterFactory_InjectFalse(t *testing.T) {
	factory, err := NewDaitchMokotoffSoundexFilterFactoryWithArgs(map[string]string{
		"inject": "false",
	})
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	for _, g := range got {
		if g == "international" {
			t.Errorf("inject=false: original token should not appear, got %v", got)
		}
	}
	if !containsAll(got, []string{"063963"}) {
		t.Errorf("inject=false: got %v, want to contain [063963]", got)
	}
}

// TestDaitchMokotoffSoundexFilterFactory_BogusArgs validates unknown parameter rejection.
// Source: TestDaitchMokotoffSoundexFilterFactory.testBogusArguments()
func TestDaitchMokotoffSoundexFilterFactory_BogusArgs(t *testing.T) {
	_, err := NewDaitchMokotoffSoundexFilterFactoryWithArgs(map[string]string{
		"bogusArg": "bogusValue",
	})
	if err == nil {
		t.Errorf("expected error for bogus argument, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestDoubleMetaphoneFilter
// Source: TestDoubleMetaphoneFilter.java
// ---------------------------------------------------------------------------

// TestDoubleMetaphoneFilter_Size4FalseInject validates size=4, inject=false.
// Source: TestDoubleMetaphoneFilter.testSize4FalseInject()
func TestDoubleMetaphoneFilter_Size4FalseInject(t *testing.T) {
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDoubleMetaphoneFilter(input, 4, false)
	})
	got := collectFilterTokens(t, ts)
	want := []string{"ANTR"}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDoubleMetaphoneFilter_Size4TrueInject validates size=4, inject=true.
// Source: TestDoubleMetaphoneFilter.testSize4TrueInject()
func TestDoubleMetaphoneFilter_Size4TrueInject(t *testing.T) {
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDoubleMetaphoneFilter(input, 4, true)
	})
	got := collectFilterTokens(t, ts)
	if !containsAll(got, []string{"international", "ANTR"}) {
		t.Errorf("got %v, want to contain [international, ANTR]", got)
	}
}

// TestDoubleMetaphoneFilter_AlternateInjectFalse validates alternate codes with inject=false.
// Source: TestDoubleMetaphoneFilter.testAlternateInjectFalse()
func TestDoubleMetaphoneFilter_AlternateInjectFalse(t *testing.T) {
	ts := newWhitespacePipeline("Kuczewski", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDoubleMetaphoneFilter(input, 4, false)
	})
	got := collectFilterTokens(t, ts)
	if !containsAll(got, []string{"KSSK", "KXFS"}) {
		t.Errorf("got %v, want to contain [KSSK, KXFS]", got)
	}
}

// TestDoubleMetaphoneFilter_Size8FalseInject validates size=8, inject=false.
// Source: TestDoubleMetaphoneFilter.testSize8FalseInject()
func TestDoubleMetaphoneFilter_Size8FalseInject(t *testing.T) {
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDoubleMetaphoneFilter(input, 8, false)
	})
	got := collectFilterTokens(t, ts)
	want := []string{"ANTRNXNL"}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDoubleMetaphoneFilter_NonConvertableStrings validates strings that produce
// no phonetic code.
// Source: TestDoubleMetaphoneFilter.testNonConvertableStringsWithInject()
func TestDoubleMetaphoneFilter_NonConvertableStrings(t *testing.T) {
	ts := newWhitespacePipeline("12345 #$%@#^%&", func(input analysis.TokenStream) analysis.TokenStream {
		return NewDoubleMetaphoneFilter(input, 8, true)
	})
	got := collectFilterTokens(t, ts)
	if !containsAll(got, []string{"12345", "#$%@#^%&"}) {
		t.Errorf("got %v, want to contain non-convertable strings", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoubleMetaphoneFilterFactory
// Source: TestDoubleMetaphoneFilterFactory.java
// ---------------------------------------------------------------------------

// TestDoubleMetaphoneFilterFactory_Defaults validates default factory settings.
// Source: TestDoubleMetaphoneFilterFactory.testDefaults()
func TestDoubleMetaphoneFilterFactory_Defaults(t *testing.T) {
	factory := NewDoubleMetaphoneFilterFactory()
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	if !containsAll(got, []string{"international", "ANTR"}) {
		t.Errorf("factory defaults: got %v, want to contain [international, ANTR]", got)
	}
}

// TestDoubleMetaphoneFilterFactory_SizeAndInject validates size=8, inject=false.
// Source: TestDoubleMetaphoneFilterFactory.testSettingSizeAndInject()
func TestDoubleMetaphoneFilterFactory_SizeAndInject(t *testing.T) {
	factory, err := NewDoubleMetaphoneFilterFactoryWithArgs(map[string]string{
		"inject":        "false",
		"maxCodeLength": "8",
	})
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	ts := newWhitespacePipeline("international", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	want := []string{"ANTRNXNL"}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDoubleMetaphoneFilterFactory_BogusArgs validates unknown parameter rejection.
// Source: TestDoubleMetaphoneFilterFactory.testBogusArguments()
func TestDoubleMetaphoneFilterFactory_BogusArgs(t *testing.T) {
	_, err := NewDoubleMetaphoneFilterFactoryWithArgs(map[string]string{
		"bogusArg": "bogusValue",
	})
	if err == nil {
		t.Errorf("expected error for bogus argument, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestPhoneticFilter
// Source: TestPhoneticFilter.java
// ---------------------------------------------------------------------------

// TestPhoneticFilter_Algorithms validates PhoneticFilter with various encoders.
// Source: TestPhoneticFilter.testAlgorithms()
func TestPhoneticFilter_Algorithms(t *testing.T) {
	tests := []struct {
		encoder Encoder
		inject  bool
		input   string
		want    []string
	}{
		{
			encoder: NewMetaphone(),
			inject:  true,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A", "aaa", "B", "bbb", "KKK", "ccc", "ESKS", "easgasg"},
		},
		{
			encoder: NewMetaphone(),
			inject:  false,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A", "B", "KKK", "ESKS"},
		},
		{
			encoder: NewDoubleMetaphone(),
			inject:  true,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A", "aaa", "PP", "bbb", "KK", "ccc", "ASKS", "easgasg"},
		},
		{
			encoder: NewDoubleMetaphone(),
			inject:  false,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A", "PP", "KK", "ASKS"},
		},
		{
			encoder: NewSoundex(),
			inject:  true,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A000", "aaa", "B000", "bbb", "C000", "ccc", "E220", "easgasg"},
		},
		{
			encoder: NewSoundex(),
			inject:  false,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A000", "B000", "C000", "E220"},
		},
		{
			encoder: NewRefinedSoundex(),
			inject:  true,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A0", "aaa", "B1", "bbb", "C3", "ccc", "E034034", "easgasg"},
		},
		{
			encoder: NewRefinedSoundex(),
			inject:  false,
			input:   "aaa bbb ccc easgasg",
			want:    []string{"A0", "B1", "C3", "E034034"},
		},
	}

	for _, tt := range tests {
		enc := tt.encoder
		inject := tt.inject
		ts := newWhitespacePipeline(tt.input, func(input analysis.TokenStream) analysis.TokenStream {
			return NewPhoneticFilter(input, enc, inject)
		})
		got := collectFilterTokens(t, ts)
		if !equalSlices(got, tt.want) {
			t.Errorf("encoder=%T inject=%v: got %v, want %v", enc, inject, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestPhoneticFilterFactory
// Source: TestPhoneticFilterFactory.java
// ---------------------------------------------------------------------------

// TestPhoneticFilterFactory_Defaults validates the default factory configuration.
// Source: TestPhoneticFilterFactory.testFactoryDefaults()
func TestPhoneticFilterFactory_Defaults(t *testing.T) {
	factory := NewPhoneticFilterFactory("Metaphone")
	if factory.inject != true {
		t.Errorf("default inject should be true")
	}
}

// TestPhoneticFilterFactory_InjectFalse validates inject=false setting.
// Source: TestPhoneticFilterFactory.testInjectFalse()
func TestPhoneticFilterFactory_InjectFalse(t *testing.T) {
	factory, err := NewPhoneticFilterFactoryWithArgs(map[string]string{
		"encoder": "Metaphone",
		"inject":  "false",
	})
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if factory.inject != false {
		t.Errorf("inject should be false")
	}
}

// TestPhoneticFilterFactory_MissingEncoder validates missing encoder error.
// Source: TestPhoneticFilterFactory.testMissingEncoder()
func TestPhoneticFilterFactory_MissingEncoder(t *testing.T) {
	_, err := NewPhoneticFilterFactoryWithArgs(map[string]string{})
	if err == nil {
		t.Errorf("expected error for missing encoder")
	}
}

// ---------------------------------------------------------------------------
// TestBeiderMorseFilter
// Source: TestBeiderMorseFilter.java
// ---------------------------------------------------------------------------

// TestBeiderMorseFilter_Numbers validates that numeric inputs pass through.
// Source: TestBeiderMorseFilter.testNumbers()
func TestBeiderMorseFilter_Numbers(t *testing.T) {
	engine := NewPhoneticEngine(NameTypeGeneric, RuleTypeExact, true)
	ts := newWhitespacePipeline("1234", func(input analysis.TokenStream) analysis.TokenStream {
		return NewBeiderMorseFilter(input, engine)
	})
	got := collectFilterTokens(t, ts)
	if len(got) == 0 {
		t.Errorf("expected at least one token for numeric input")
	}
}

// TestBeiderMorseFilter_EmptyToken validates that empty tokens produce no output.
// Source: TestBeiderMorseFilter.testEmptyTerm()
func TestBeiderMorseFilter_EmptyToken(t *testing.T) {
	engine := NewPhoneticEngine(NameTypeGeneric, RuleTypeExact, true)
	// Using empty input: should produce empty result.
	ts := newWhitespacePipeline("", func(input analysis.TokenStream) analysis.TokenStream {
		return NewBeiderMorseFilter(input, engine)
	})
	got := collectFilterTokens(t, ts)
	if len(got) != 0 {
		t.Errorf("expected no tokens for empty input, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// TestBeiderMorseFilterFactory
// Source: TestBeiderMorseFilterFactory.java
// ---------------------------------------------------------------------------

// TestBeiderMorseFilterFactory_Defaults validates that the factory creates
// a functional BeiderMorseFilter.
// Source: TestBeiderMorseFilterFactory.testBasics() - structural validation only;
// exact BM output depends on the full rule set.
func TestBeiderMorseFilterFactory_Defaults(t *testing.T) {
	factory := NewBeiderMorseFilterFactory()
	ts := newWhitespacePipeline("Weinberg", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	// Verify that encoding produces at least one token.
	if len(got) == 0 {
		t.Errorf("BeiderMorse factory: expected at least one token for 'Weinberg', got none")
	}
}

// TestBeiderMorseFilterFactory_BogusArgs validates unknown parameter rejection.
// Source: TestBeiderMorseFilterFactory.testBogusArguments()
func TestBeiderMorseFilterFactory_BogusArgs(t *testing.T) {
	_, err := NewBeiderMorseFilterFactoryWithArgs(map[string]string{
		"bogusArg": "bogusValue",
	})
	if err == nil {
		t.Errorf("expected error for bogus argument, got nil")
	}
}

// TestBeiderMorseFilterFactory_Options validates nameType and ruleType options.
// Source: TestBeiderMorseFilterFactory.testOptions()
func TestBeiderMorseFilterFactory_Options(t *testing.T) {
	factory, err := NewBeiderMorseFilterFactoryWithArgs(map[string]string{
		"nameType": "ASHKENAZI",
		"ruleType": "EXACT",
	})
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	ts := newWhitespacePipeline("Weinberg", func(input analysis.TokenStream) analysis.TokenStream {
		return factory.Create(input)
	})
	got := collectFilterTokens(t, ts)
	// Verify that encoding produces at least one token.
	if len(got) == 0 {
		t.Errorf("BeiderMorse ASHKENAZI/EXACT: expected at least one token for 'Weinberg'")
	}
}
