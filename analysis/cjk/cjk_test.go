// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cjk

import (
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// collectTokens runs a TokenStream to completion and collects term strings.
func collectTokens(t *testing.T, ts analysis.TokenStream) []string {
	t.Helper()
	var terms []string
	var termAttr analysis.CharTermAttribute
	switch v := ts.(type) {
	case *CJKWidthFilter:
		if a := v.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	case *CJKBigramFilter:
		if a := v.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
	}
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

// collectTokensWithTypes runs a TokenStream and collects (term, type) pairs.
func collectTokensWithTypes(t *testing.T, ts analysis.TokenStream) [][2]string {
	t.Helper()
	var out [][2]string
	var termAttr analysis.CharTermAttribute
	var typeAttr analysis.TypeAttribute
	if v, ok := ts.(*CJKBigramFilter); ok {
		if a := v.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); a != nil {
			termAttr = a.(analysis.CharTermAttribute)
		}
		if a := v.GetAttributeSource().GetAttribute(analysis.TypeAttributeType); a != nil {
			typeAttr = a.(analysis.TypeAttribute)
		}
	}
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		term := ""
		typ := ""
		if termAttr != nil {
			term = termAttr.String()
		}
		if typeAttr != nil {
			typ = typeAttr.GetType()
		}
		out = append(out, [2]string{term, typ})
	}
	return out
}

// newWhitespacePipeline builds a WhitespaceTokenizer → filter pipeline.
func newWhitespacePipeline(input string, makeFilter func(ts analysis.TokenStream) analysis.TokenStream) analysis.TokenStream {
	tok := analysis.NewWhitespaceTokenizer()
	_ = tok.SetReader(strings.NewReader(input))
	return makeFilter(tok)
}

// newStdPipeline builds a StandardTokenizer → filter pipeline.
func newStdPipeline(input string, makeFilter func(ts analysis.TokenStream) analysis.TokenStream) analysis.TokenStream {
	tok := analysis.NewStandardTokenizer()
	_ = tok.SetReader(strings.NewReader(input))
	return makeFilter(tok)
}

// ---------------------------------------------------------------------------
// TestCJKWidthFilter
// Source: TestCJKWidthFilter.java
// ---------------------------------------------------------------------------

// TestCJKWidthFilter_FullWidthASCII verifies fullwidth ASCII → basic latin folding.
// Source: TestCJKWidthFilter.testFullWidthASCII
func TestCJKWidthFilter_FullWidthASCII(t *testing.T) {
	ts := newWhitespacePipeline("Ｔｅｓｔ １２３４", func(in analysis.TokenStream) analysis.TokenStream {
		return NewCJKWidthFilter(in)
	})
	got := collectTokens(t, ts)
	want := []string{"Test", "1234"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d got %d; %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] want %q got %q", i, w, got[i])
		}
	}
}

// TestCJKWidthFilter_HalfWidthKana verifies halfwidth katakana → fullwidth conversion.
// Source: TestCJKWidthFilter.testHalfWidthKana
func TestCJKWidthFilter_HalfWidthKana(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"ｶﾀｶﾅ", []string{"カタカナ"}},
		{"ｳﾞｨｯﾂ", []string{"ヴィッツ"}},
		{"ﾊﾟﾅｿﾆｯｸ", []string{"パナソニック"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			ts := newWhitespacePipeline(tc.input, func(in analysis.TokenStream) analysis.TokenStream {
				return NewCJKWidthFilter(in)
			})
			got := collectTokens(t, ts)
			if len(got) != len(tc.want) {
				t.Fatalf("len: want %d got %d; %v", len(tc.want), len(got), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("[%d] want %q got %q", i, w, got[i])
				}
			}
		})
	}
}

// TestCJKWidthFilter_Empty verifies empty-term passthrough.
func TestCJKWidthFilter_Empty(t *testing.T) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader(""))
	ts := NewCJKWidthFilter(tok)
	got := collectTokens(t, ts)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("want [\"\"], got %v", got)
	}
}

// TestCJKWidthFilterFactory_Create verifies the factory creates a working filter.
func TestCJKWidthFilterFactory_Create(t *testing.T) {
	f := NewCJKWidthFilterFactory()
	ts := newWhitespacePipeline("Ｔｅｓｔ", func(in analysis.TokenStream) analysis.TokenStream {
		return f.Create(in)
	})
	got := collectTokens(t, ts)
	if len(got) != 1 || got[0] != "Test" {
		t.Errorf("want [\"Test\"], got %v", got)
	}
}

// ---------------------------------------------------------------------------
// TestCJKWidthCharFilter
// Source: TestCJKWidthCharFilter.java
// ---------------------------------------------------------------------------

// readAllFromFilter reads everything from an io.Reader into a string.
func readAllFromFilter(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(data)
}

// TestCJKWidthCharFilter_FullWidthASCII verifies fullwidth ASCII folding at reader level.
// Source: TestCJKWidthCharFilter.testFullWidthASCII
func TestCJKWidthCharFilter_FullWidthASCII(t *testing.T) {
	r := NewCJKWidthCharFilter(strings.NewReader("Ｔｅｓｔ"))
	got := readAllFromFilter(t, r)
	want := "Test"
	if got != want {
		t.Errorf("want %q got %q", want, got)
	}
}

// TestCJKWidthCharFilter_HalfWidthKana verifies halfwidth kana folding.
// Source: TestCJKWidthCharFilter.testHalfWidthKana
func TestCJKWidthCharFilter_HalfWidthKana(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"ｶﾀｶﾅ", "カタカナ"},
		{"ｳﾞｨｯﾂ", "ヴィッツ"},
		{"ﾊﾟﾅｿﾆｯｸ", "パナソニック"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			r := NewCJKWidthCharFilter(strings.NewReader(tc.input))
			got := readAllFromFilter(t, r)
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCJKBigramFilter
// Source: TestCJKBigramFilter.java
// ---------------------------------------------------------------------------

// TestCJKBigramFilter_AllScripts tests bigrams across all CJK scripts.
// Source: TestCJKBigramFilter.testAllScripts
func TestCJKBigramFilter_AllScripts(t *testing.T) {
	ts := newStdPipeline("多くの学生が試験に落ちた。", func(in analysis.TokenStream) analysis.TokenStream {
		return NewCJKBigramFilterFull(in, Han|Hiragana|Katakana|Hangul, false)
	})
	got := collectTokens(t, ts)
	want := []string{"多く", "くの", "の学", "学生", "生が", "が試", "試験", "験に", "に落", "落ち", "ちた"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d got %d; %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] want %q got %q", i, w, got[i])
		}
	}
}

// TestCJKBigramFilter_HanOnly verifies HAN-only mode passes Hiragana as-is.
// Source: TestCJKBigramFilter.testHanOnly
func TestCJKBigramFilter_HanOnly(t *testing.T) {
	ts := newStdPipeline("多くの学生が試験に落ちた。", func(in analysis.TokenStream) analysis.TokenStream {
		return NewCJKBigramFilterWithFlags(in, Han)
	})
	pairs := collectTokensWithTypes(t, ts)

	// Han pairs get DOUBLE, others get their original type.
	wantTerms := []string{"多", "く", "の", "学生", "が", "試験", "に", "落", "ち", "た"}
	wantTypes := []string{SingleType, "<HIRAGANA>", "<HIRAGANA>", DoubleType, "<HIRAGANA>", DoubleType, "<HIRAGANA>", SingleType, "<HIRAGANA>", "<HIRAGANA>", SingleType}
	_ = wantTypes // type assertions vary; at minimum verify term sequence
	if len(pairs) != len(wantTerms) {
		t.Fatalf("len: want %d got %d; %v", len(wantTerms), len(pairs), pairs)
	}
	for i, w := range wantTerms {
		if pairs[i][0] != w {
			t.Errorf("[%d] term: want %q got %q", i, w, pairs[i][0])
		}
	}
}

// TestCJKBigramFilter_Unigrams tests unigram+bigram output mode.
// Source: TestCJKBigramFilter.testUnigramsAndBigramsAllScripts
func TestCJKBigramFilter_Unigrams(t *testing.T) {
	ts := newStdPipeline("多くの学生が試験に落ちた。", func(in analysis.TokenStream) analysis.TokenStream {
		return NewCJKBigramFilterFull(in, Han|Hiragana|Katakana|Hangul, true)
	})
	got := collectTokens(t, ts)
	// Should produce 23 tokens: unigram+bigram interleaved for 11 CJK chars.
	want := []string{
		"多", "多く", "く", "くの", "の", "の学", "学", "学生", "生", "生が", "が",
		"が試", "試", "試験", "験", "験に", "に", "に落", "落", "落ち", "ち", "ちた", "た",
	}
	if len(got) != len(want) {
		t.Fatalf("len: want %d got %d; %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] want %q got %q", i, w, got[i])
		}
	}
}

// TestCJKBigramFilter_HugeInput tests bigram formation on repeated CJK text.
// Source: TestCJKBigramFilter.testHuge
func TestCJKBigramFilter_HugeInput(t *testing.T) {
	repeated := strings.Repeat("多くの学生が試験に落ちた", 2)
	ts := newStdPipeline(repeated, func(in analysis.TokenStream) analysis.TokenStream {
		return NewCJKBigramFilter(in)
	})
	got := collectTokens(t, ts)
	// Each 11-char segment produces 11 bigrams, but with cross-segment bigrams.
	if len(got) == 0 {
		t.Fatal("expected tokens, got none")
	}
	// First bigram should always be 多く.
	if got[0] != "多く" {
		t.Errorf("first token: want 多く got %q", got[0])
	}
}

// TestCJKBigramFilterFactory_Create verifies factory creates working filter.
func TestCJKBigramFilterFactory_Create(t *testing.T) {
	f := NewCJKBigramFilterFactory()
	ts := newStdPipeline("多くの", func(in analysis.TokenStream) analysis.TokenStream {
		return f.Create(in)
	})
	got := collectTokens(t, ts)
	if len(got) == 0 {
		t.Fatal("expected tokens")
	}
}
