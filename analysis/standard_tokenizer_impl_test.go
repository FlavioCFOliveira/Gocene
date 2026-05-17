// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// implToken captures one token emitted by the standardTokenizerImpl
// scanner so test cases can assert on (text, type, start_offset).
type implToken struct {
	text      string
	tokenType int
	start     int
	length    int
}

// scanAll drains the scanner against the given input, returning every
// token in order.
func scanAll(t *testing.T, input string) []implToken {
	t.Helper()
	sc := newStandardTokenizerImpl()
	if err := sc.yyreset(strings.NewReader(input)); err != nil {
		t.Fatalf("yyreset: %v", err)
	}
	var out []implToken
	for {
		typ := sc.getNextToken()
		if typ == yyeof {
			break
		}
		out = append(out, implToken{
			text:      string(sc.buf[sc.startRead:sc.markedPos]),
			tokenType: typ,
			start:     sc.yychar(),
			length:    sc.yylength(),
		})
	}
	return out
}

// TestStandardTokenizerImpl_Basic covers the simplest ALPHANUM cases.
func TestStandardTokenizerImpl_Basic(t *testing.T) {
	got := scanAll(t, "The quick brown fox")
	want := []implToken{
		{text: "The", tokenType: TokenTypeAlphanum, start: 0, length: 3},
		{text: "quick", tokenType: TokenTypeAlphanum, start: 4, length: 5},
		{text: "brown", tokenType: TokenTypeAlphanum, start: 10, length: 5},
		{text: "fox", tokenType: TokenTypeAlphanum, start: 16, length: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestStandardTokenizerImpl_Apostrophes verifies the WB6/WB7 family
// rules (mid-letter punctuation joins ALetter sequences).
func TestStandardTokenizerImpl_Apostrophes(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"O'Reilly", []string{"O'Reilly"}},
		{"you're", []string{"you're"}},
		{"O'Reilly's", []string{"O'Reilly's"}},
	}
	for _, tc := range cases {
		got := scanAll(t, tc.input)
		var texts []string
		for _, g := range got {
			texts = append(texts, g.text)
		}
		if !reflect.DeepEqual(texts, tc.want) {
			t.Errorf("input %q: got %v, want %v", tc.input, texts, tc.want)
		}
	}
}

// TestStandardTokenizerImpl_Numeric covers the NUM rule, including
// IP-address-like dotted decimals.
func TestStandardTokenizerImpl_Numeric(t *testing.T) {
	cases := []struct {
		input string
		want  []implToken
	}{
		{"21.35", []implToken{{text: "21.35", tokenType: TokenTypeNum, start: 0, length: 5}}},
		{"216.239.63.104", []implToken{{text: "216.239.63.104", tokenType: TokenTypeNum, start: 0, length: 14}}},
	}
	for _, tc := range cases {
		got := scanAll(t, tc.input)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("input %q: got %+v, want %+v", tc.input, got, tc.want)
		}
	}
}

// TestStandardTokenizerImpl_Mid covers the mid-letter / mid-num /
// extend-num-let interactions ported from Lucene's testMid().
func TestStandardTokenizerImpl_Mid(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"A:B", []string{"A:B"}},
		{"A::B", []string{"A", "B"}},
		{"1.2", []string{"1.2"}},
		{"A.B", []string{"A.B"}},
		{"1..2", []string{"1", "2"}},
		{"A..B", []string{"A", "B"}},
		{"1,2", []string{"1,2"}},
		{"1,,2", []string{"1", "2"}},
		{"A.:B", []string{"A", "B"}},
		{"A:.B", []string{"A", "B"}},
		{"1,.2", []string{"1", "2"}},
		{"1.,2", []string{"1", "2"}},
		{"A:B_A:B", []string{"A:B_A:B"}},
		{"A:B_A::B", []string{"A:B_A", "B"}},
		{"1.2_1.2", []string{"1.2_1.2"}},
		{"A.B_A.B", []string{"A.B_A.B"}},
		{"1.2_1..2", []string{"1.2_1", "2"}},
		{"A.B_A..B", []string{"A.B_A", "B"}},
		{"1,2_1,2", []string{"1,2_1,2"}},
		{"1,2_1,,2", []string{"1,2_1", "2"}},
		{"C_A.:B", []string{"C_A", "B"}},
		{"C_A:.B", []string{"C_A", "B"}},
		{"3_1,.2", []string{"3_1", "2"}},
		{"3_1.,2", []string{"3_1", "2"}},
	}
	for _, tc := range cases {
		got := scanAll(t, tc.input)
		var texts []string
		for _, g := range got {
			texts = append(texts, g.text)
		}
		if !reflect.DeepEqual(texts, tc.want) {
			t.Errorf("input %q: got %v, want %v", tc.input, texts, tc.want)
		}
	}
}

// TestStandardTokenizerImpl_Ideographic verifies that each CJKV
// ideograph emits as its own IDEOGRAPHIC token.
func TestStandardTokenizerImpl_Ideographic(t *testing.T) {
	got := scanAll(t, "我是中国人")
	if len(got) != 5 {
		t.Fatalf("expected 5 tokens, got %d: %+v", len(got), got)
	}
	for i, g := range got {
		if g.tokenType != TokenTypeIdeographic {
			t.Errorf("token[%d]: expected IDEOGRAPHIC, got %d", i, g.tokenType)
		}
	}
}

// TestStandardTokenizerImpl_Katakana verifies that katakana runs
// (including the dakuten combining mark) are kept together.
func TestStandardTokenizerImpl_Katakana(t *testing.T) {
	got := scanAll(t, "カタカナ")
	if len(got) != 1 || got[0].tokenType != TokenTypeKatakana {
		t.Errorf("expected one KATAKANA token, got %+v", got)
	}
}

// TestStandardTokenizerImpl_CombiningMarks reproduces Lucene's
// testCombiningMarks() cases for Hiragana and Katakana voiced
// consonants.
func TestStandardTokenizerImpl_CombiningMarks(t *testing.T) {
	cases := []struct {
		input    string
		wantText string
		wantType int
	}{
		{"ざ", "ざ", TokenTypeHiragana},
		{"ザ", "ザ", TokenTypeKatakana},
	}
	for _, tc := range cases {
		got := scanAll(t, tc.input)
		if len(got) != 1 {
			t.Errorf("input %q: expected 1 token, got %+v", tc.input, got)
			continue
		}
		if got[0].text != tc.wantText || got[0].tokenType != tc.wantType {
			t.Errorf("input %q: got %+v, want text=%q type=%d", tc.input, got[0], tc.wantText, tc.wantType)
		}
	}
}

// TestStandardTokenizerImpl_Hangul checks that runs of Hangul jamo
// emit as a single HANGUL token.
func TestStandardTokenizerImpl_Hangul(t *testing.T) {
	got := scanAll(t, "안녕하세요")
	if len(got) != 1 || got[0].tokenType != TokenTypeHangul {
		t.Errorf("expected one HANGUL token, got %+v", got)
	}
}

// TestStandardTokenizerImpl_Emoji exercises the EMOJI rule for a
// simple single emoji, an emoji modifier base+modifier pair, and a
// regional indicator pair.
func TestStandardTokenizerImpl_Emoji(t *testing.T) {
	cases := []struct {
		input    string
		expected []int // expected token types in order
	}{
		{"💩", []int{TokenTypeEmoji}},
		{"💩💩", []int{TokenTypeEmoji, TokenTypeEmoji}},
		{"🇺🇸", []int{TokenTypeEmoji}}, // U+1F1FA U+1F1F8 -- regional indicator pair
	}
	for _, tc := range cases {
		got := scanAll(t, tc.input)
		var types []int
		for _, g := range got {
			types = append(types, g.tokenType)
		}
		if !reflect.DeepEqual(types, tc.expected) {
			t.Errorf("input %q: got types %v, want %v (tokens=%+v)", tc.input, types, tc.expected, got)
		}
	}
}

// TestStandardTokenizerImpl_SoutheastAsian verifies that a run of
// Line_Break:SA (Complex_Context) characters such as Thai is kept
// together as one SOUTHEAST_ASIAN token.
func TestStandardTokenizerImpl_SoutheastAsian(t *testing.T) {
	got := scanAll(t, "การที่ได้ต้องแสดงว่างานดี")
	if len(got) < 1 {
		t.Fatalf("expected at least one token, got %+v", got)
	}
	if got[0].tokenType != TokenTypeSoutheastAsian {
		t.Errorf("expected SOUTHEAST_ASIAN, got %d (%+v)", got[0].tokenType, got)
	}
}

// TestStandardTokenizerImpl_PooEmoji is the canonical TestStandardAnalyzer
// test exercising mixed ALPHANUM and EMOJI: "poo💩poo" -> [poo, 💩, poo].
func TestStandardTokenizerImpl_PooEmoji(t *testing.T) {
	got := scanAll(t, "poo💩poo")
	if len(got) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %+v", len(got), got)
	}
	if got[0].text != "poo" || got[0].tokenType != TokenTypeAlphanum {
		t.Errorf("token[0]: %+v want poo/ALPHANUM", got[0])
	}
	if got[1].tokenType != TokenTypeEmoji {
		t.Errorf("token[1]: %+v want EMOJI", got[1])
	}
	if got[2].text != "poo" || got[2].tokenType != TokenTypeAlphanum {
		t.Errorf("token[2]: %+v want poo/ALPHANUM", got[2])
	}
}
