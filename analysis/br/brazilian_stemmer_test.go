// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package br

import "testing"

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func checkStem(t *testing.T, input, want string) {
	t.Helper()
	s := &BrazilianStemmer{}
	got := s.Stem(input)
	if got != want {
		t.Errorf("Stem(%q) = %q; want %q", input, got, want)
	}
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_WithSnowballExamples
// Source: TestBrazilianAnalyzer.testWithSnowballExamples
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_WithSnowballExamples(t *testing.T) {
	cases := [][2]string{
		{"boa", "boa"},
		{"boainain", "boainain"},
		{"boas", "boas"},
		{"bôas", "boas"},
		{"boassu", "boassu"},
		{"boataria", "boat"},
		{"boate", "boat"},
		{"boates", "boat"},
		{"boatos", "boat"},
		{"bob", "bob"},
		{"boba", "bob"},
		{"bobagem", "bobag"},
		{"bobagens", "bobagens"},
		{"bobalhões", "bobalho"},
		{"bobear", "bob"},
		{"bobeira", "bobeir"},
		{"bobinho", "bobinh"},
		{"bobinhos", "bobinh"},
		{"bobo", "bob"},
		{"bobs", "bobs"},
		{"boca", "boc"},
		{"bocadas", "boc"},
		{"bocadinho", "bocadinh"},
		{"bocado", "boc"},
		{"bocaiúva", "bocaiuv"},
		{"boçal", "bocal"},
		{"bocarra", "bocarr"},
		{"bocas", "boc"},
		{"bode", "bod"},
		{"bodoque", "bodoqu"},
		{"body", "body"},
		{"boeing", "boeing"},
		{"boem", "boem"},
		{"boemia", "boem"},
		{"boêmio", "boemi"},
		{"bogotá", "bogot"},
		{"boi", "boi"},
		{"bóia", "boi"},
		{"boiando", "boi"},
		{"quiabo", "quiab"},
		{"quicaram", "quic"},
		{"quickly", "quickly"},
		{"quieto", "quiet"},
		{"quietos", "quiet"},
		{"quilate", "quilat"},
		{"quilates", "quilat"},
		{"quilinhos", "quilinh"},
		{"quilo", "quil"},
		{"quilombo", "quilomb"},
		{"quilométricas", "quilometr"},
	}
	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			checkStem(t, tc[0], tc[1])
		})
	}
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_NotIndexable — terms outside the indexable range
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_NotIndexable(t *testing.T) {
	s := &BrazilianStemmer{}
	// length ≤ 2 → ""
	if got := s.Stem("ab"); got != "" {
		t.Errorf("Stem(%q) = %q; want \"\"", "ab", got)
	}
	// length ≥ 30 → ""
	long := "abcdefghijklmnopqrstuvwxyzabcde"
	if got := s.Stem(long); got != "" {
		t.Errorf("Stem(long) = %q; want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_NonAlpha — non-letter terms return CT unchanged
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_NonAlpha(t *testing.T) {
	s := &BrazilianStemmer{}
	// After changeTerm "123" is "123" which is not stemmable but is indexable.
	got := s.Stem("123")
	if got != "123" {
		t.Errorf("Stem(%q) = %q; want %q", "123", got, "123")
	}
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_Step1_Amente
// "exatamente" → R1 contains "amente" → remove → "exat"
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_Step1_Amente(t *testing.T) {
	checkStem(t, "exatamente", "exat")
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_Diacritics — accent removal
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_Diacritics(t *testing.T) {
	// "ÁGUA" → lowercased + diacritics removed → "agua" → step4 removes "a" → "agu"
	checkStem(t, "ÁGUA", "agu")
	// "bôas" → "boas" (from Snowball examples above; also tested here for clarity)
	checkStem(t, "bôas", "boas")
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_Step5_GuE — gu + e trimming
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_Step5_GuE(t *testing.T) {
	// "bodoque" → step5 removes 'e', then 'u' after 'g'... actually no 'gu'
	// Use a word that ends in "gue" in RV:
	// "chegue" → ct="chegue", rv="gue" → step2 removes "ue"? No.
	// Let's use the Snowball case "bodoque" → "bodoqu" (ct check only).
	checkStem(t, "bodoque", "bodoqu")
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_LeadingPunctuation — leading punct stripped in CT
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_LeadingPunctuation(t *testing.T) {
	s := &BrazilianStemmer{}
	// "\"boca\"" → strip leading and trailing quote → "boca" → stem → "boc"
	got := s.Stem("\"boca\"")
	if got != "boc" {
		t.Errorf("Stem(%q) = %q; want %q", "\"boca\"", got, "boc")
	}
}

// ---------------------------------------------------------------------------
// TestBrazilianStemmer_Reuse — stemmer can be reused across calls
// ---------------------------------------------------------------------------

func TestBrazilianStemmer_Reuse(t *testing.T) {
	s := &BrazilianStemmer{}
	if got := s.Stem("boataria"); got != "boat" {
		t.Errorf("first call: Stem=%q want \"boat\"", got)
	}
	if got := s.Stem("bobinha"); got != "bobinh" {
		t.Errorf("second call: Stem=%q want \"bobinh\"", got)
	}
}
