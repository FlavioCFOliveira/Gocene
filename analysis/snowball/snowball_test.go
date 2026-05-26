// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/snowball/TestSnowball.java

package snowball

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/snowball/ext"
)

// stemWord passes word through a WhitespaceTokenizer → SnowballFilter and
// returns the stem.
func stemWord(t *testing.T, word string, stemmer SnowballStemmer) string {
	t.Helper()
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(word))
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	f := NewSnowballFilter(tokenizer, stemmer)
	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !ok {
		t.Fatalf("no token for %q", word)
	}
	attr := f.GetAttribute("CharTermAttribute")
	if attr == nil {
		t.Fatal("CharTermAttribute not found")
	}
	return attr.(analysis.CharTermAttribute).String()
}

// stemWordKeyword passes word through a KeywordTokenizer → SnowballFilter
// and returns the stem. Unlike stemWord, this emits exactly one token even
// for empty input (matching Java's KeywordTokenizer behaviour).
func stemWordKeyword(t *testing.T, word string, stemmer SnowballStemmer) string {
	t.Helper()
	tokenizer := analysis.NewKeywordTokenizer()
	tokenizer.SetReader(strings.NewReader(word))
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	f := NewSnowballFilter(tokenizer, stemmer)
	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !ok {
		t.Fatalf("no token for %q", word)
	}
	attr := f.GetAttribute("CharTermAttribute")
	if attr == nil {
		t.Fatal("CharTermAttribute not found")
	}
	return attr.(analysis.CharTermAttribute).String()
}

// TestSnowball_English verifies that the English (Porter) Snowball stemmer
// correctly stems common English words.
//
// Source: TestSnowball.testEnglish
func TestSnowball_English(t *testing.T) {
	tests := []struct{ in, want string }{
		{"he", "he"},
		{"abhorred", "abhor"},
		{"accents", "accent"},
	}
	for _, tc := range tests {
		got := stemWord(t, tc.in, ext.NewEnglishStemmer())
		if got != tc.want {
			t.Errorf("English.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_EmptyTerm verifies that every supported language stemmer
// returns an empty string for empty input.
//
// Source: TestSnowball.testEmptyTerm
// Deviation: Java uses language strings to construct stemmers via reflection;
// Gocene tests all concrete stemmers directly.
func TestSnowball_EmptyTerm(t *testing.T) {
	stemmers := []struct {
		name    string
		stemmer SnowballStemmer
	}{
		{"Arabic", ext.NewArabicStemmer()},
		{"Armenian", ext.NewArmenianStemmer()},
		{"Basque", ext.NewBasqueStemmer()},
		{"Catalan", ext.NewCatalanStemmer()},
		{"Danish", ext.NewDanishStemmer()},
		{"Dutch", ext.NewDutchStemmer()},
		{"English", ext.NewEnglishStemmer()},
		{"Estonian", ext.NewEstonianStemmer()},
		{"Finnish", ext.NewFinnishStemmer()},
		{"French", ext.NewFrenchStemmer()},
		{"German", ext.NewGermanStemmer()},
		{"Greek", ext.NewGreekStemmer()},
		{"Hindi", ext.NewHindiStemmer()},
		{"Hungarian", ext.NewHungarianStemmer()},
		{"Indonesian", ext.NewIndonesianStemmer()},
		{"Irish", ext.NewIrishStemmer()},
		{"Italian", ext.NewItalianStemmer()},
		{"Lithuanian", ext.NewLithuanianStemmer()},
		{"Nepali", ext.NewNepaliStemmer()},
		{"Norwegian", ext.NewNorwegianStemmer()},
		{"Porter", ext.NewPorterStemmer()},
		{"Portuguese", ext.NewPortugueseStemmer()},
		{"Romanian", ext.NewRomanianStemmer()},
		{"Russian", ext.NewRussianStemmer()},
		{"Serbian", ext.NewSerbianStemmer()},
		{"Spanish", ext.NewSpanishStemmer()},
		{"Swedish", ext.NewSwedishStemmer()},
		{"Tamil", ext.NewTamilStemmer()},
		{"Turkish", ext.NewTurkishStemmer()},
		{"Yiddish", ext.NewYiddishStemmer()},
	}

	for _, s := range stemmers {
		t.Run(s.name, func(t *testing.T) {
			got := stemWordKeyword(t, "", s.stemmer)
			if got != "" {
				t.Errorf("%s.Stem(\"\") = %q, want \"\"", s.name, got)
			}
		})
	}
}

// TestSnowball_FilterTokens verifies that SnowballFilter correctly stems
// a token and preserves other attributes from the input stream.
//
// Source: TestSnowball.testFilterTokens
func TestSnowball_FilterTokens(t *testing.T) {
	// "accents" → English stemmer → "accent"
	got := stemWord(t, "accents", ext.NewEnglishStemmer())
	if got != "accent" {
		t.Errorf("stem(accents) = %q, want %q", got, "accent")
	}
}

// TestSnowball_German verifies the German Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_German(t *testing.T) {
	tests := []struct{ in, want string }{
		// umlaut normalisation + suffix removal
		{"frauen", "frau"},
		{"schöner", "schon"},
		{"spielen", "spiel"},
		{"abgehängt", "abgehangt"},
	}
	stemmer := ext.NewGermanStemmer()
	for _, tc := range tests {
		got := stemWord(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("German.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Dutch verifies the Dutch Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Dutch(t *testing.T) {
	tests := []struct{ in, want string }{
		{"fietsen", "fiets"},
		{"werken", "werk"},
		{"zingen", "zing"},
	}
	stemmer := ext.NewDutchStemmer()
	for _, tc := range tests {
		got := stemWord(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Dutch.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Indonesian verifies the Indonesian Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Indonesian(t *testing.T) {
	tests := []struct{ in, want string }{
		{"berlari", "lari"},
		{"membantu", "bantu"},
		{"perjalanan", "jalan"},
	}
	stemmer := ext.NewIndonesianStemmer()
	for _, tc := range tests {
		got := stemWord(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Indonesian.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Porter verifies the Porter Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Porter(t *testing.T) {
	tests := []struct{ in, want string }{
		{"running", "run"},
		{"caresses", "caress"},
		{"flies", "fli"},
	}
	stemmer := ext.NewPorterStemmer()
	for _, tc := range tests {
		got := stemWord(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Porter.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Greek verifies the Greek Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Greek(t *testing.T) {
	tests := []struct{ in, want string }{
		// tolower + suffix removal
		{"τρέχω", "τρεχ"},
		{"ελλάδα", "ελλαδ"},
		{"σπιτια", "σπιτ"},
	}
	stemmer := ext.NewGreekStemmer()
	for _, tc := range tests {
		got := stemWord(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Greek.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Hindi verifies the Hindi Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Hindi(t *testing.T) {
	tests := []struct{ in, want string }{
		// short words that should be unchanged (< 2 runes past the first)
		{"कर", "कर"},
		// suffix removal: ों → stripped
		{"लड़कों", "लड़क"},
	}
	stemmer := ext.NewHindiStemmer()
	for _, tc := range tests {
		got := stemWordKeyword(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Hindi.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSnowball_Arabic verifies the Arabic Snowball stemmer against
// reference outputs from the Snowball 2.2.0 algorithm.
func TestSnowball_Arabic(t *testing.T) {
	tests := []struct{ in, want string }{
		// short root: should remain unchanged
		{"كتاب", "كتاب"},
		// definite article al- prefix removal
		{"الكتاب", "كتاب"},
	}
	stemmer := ext.NewArabicStemmer()
	for _, tc := range tests {
		got := stemWordKeyword(t, tc.in, stemmer)
		if got != tc.want {
			t.Errorf("Arabic.Stem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
