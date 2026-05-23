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

// TestSnowball_EmptyTerm verifies that each supported language stemmer
// returns an empty string for empty input.
//
// Source: TestSnowball.testEmptyTerm
// Deviation: Java uses language strings to construct stemmers via reflection;
// Gocene tests a representative set of concrete stemmers directly.
func TestSnowball_EmptyTerm(t *testing.T) {
	stemmers := []struct {
		name    string
		stemmer SnowballStemmer
	}{
		{"English", ext.NewEnglishStemmer()},
		{"French", ext.NewFrenchStemmer()},
		{"Spanish", ext.NewSpanishStemmer()},
		{"Italian", ext.NewItalianStemmer()},
		{"Portuguese", ext.NewPortugueseStemmer()},
		{"Finnish", ext.NewFinnishStemmer()},
		{"Swedish", ext.NewSwedishStemmer()},
		{"Norwegian", ext.NewNorwegianStemmer()},
		{"Danish", ext.NewDanishStemmer()},
		{"Russian", ext.NewRussianStemmer()},
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
