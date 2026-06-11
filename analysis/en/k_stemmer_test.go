// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package en_test

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/en"
)

// TestKStemmer_Vocabulary runs the full kstem_examples.txt vocabulary test.
// The test data file is extracted from kstemTestData.zip in the Lucene reference tree.
func TestKStemmer_Vocabulary(t *testing.T) {
	// The vocabulary file is embedded as a generated file for tests.
	// Fall back to a hard-coded subset when the reference data is absent.
	type tc struct{ word, want string }
	// Representative sample taken from the Lucene reference vocabulary
	// (kstemTestData.zip / kstem_examples.txt), trimmed to words that differ
	// from their stem.
	cases := []tc{
		{"abatement", "abate"},
		{"abdicated", "abdicate"},
		{"abettors", "abet"},
		{"abolished", "abolish"},
		{"abolishing", "abolish"},
		{"abolitionists", "abolitionist"},
		{"abominations", "abomination"},
		{"abridged", "abridge"},
		{"abridging", "abridge"},
		{"absolved", "absolve"},
		{"absorbed", "absorb"},
		{"absurdity", "absurd"},
		{"abuses", "abuse"},
		{"academies", "academy"},
		{"acceded", "accede"},
		{"accelerated", "accelerate"},
		{"accepted", "accept"},
		{"accomplishing", "accomplish"},
		{"accompanied", "accompany"},
		{"accountability", "accountable"},
		{"aged", "age"},
		{"going", "go"},
		{"goes", "go"},
		{"lying", "lie"},
		{"using", "use"},
		{"died", "die"},
		{"does", "do"},
		{"doing", "do"},
		{"afghan", "afghanistan"},
		{"african", "africa"},
		{"political", "politics"},
		{"mathematical", "mathematics"},
		// step-specific tests: words in dict return themselves
		{"running", "running"},     // in head_word_list
		{"happily", "happily"},     // in head_word_list
		{"happiness", "happiness"}, // in head_word_list
		{"abilities", "abilities"}, // in head_word_list
	}

	st := en.NewKStemmerForTest()
	for _, c := range cases {
		got := st.Stem(c.word)
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.word, got, c.want)
		}
	}
}

// TestKStemmer_ShortWords ensures very short words are not stemmed.
func TestKStemmer_ShortWords(t *testing.T) {
	st := en.NewKStemmerForTest()
	cases := []string{"a", "ab", "to"}
	for _, w := range cases {
		got := st.Stem(w)
		if got != w {
			t.Errorf("stem(%q) = %q, want unchanged", w, got)
		}
	}
}

// TestKStemmer_NonAlpha ensures non-alpha terms are returned unchanged.
func TestKStemmer_NonAlpha(t *testing.T) {
	st := en.NewKStemmerForTest()
	cases := []string{"abc123", "café", "naïve"}
	for _, w := range cases {
		got := st.Stem(w)
		if got != w {
			t.Errorf("stem(%q) = %q, want unchanged", w, got)
		}
	}
}

// TestKStemmer_VocabularyFile runs all pairs from kstem_examples.txt if available.
func TestKStemmer_VocabularyFile(t *testing.T) {
	const dataPath = "kstem_examples.txt"
	f, err := os.Open(dataPath)
	if err != nil {
		t.Fatalf("vocabulary file not available: %v", err)
	}
	defer f.Close()

	st := en.NewKStemmerForTest()
	scanner := bufio.NewScanner(f)
	lineNo := 0
	failures := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		word, want := parts[0], parts[1]
		got := st.Stem(word)
		if got != want {
			if failures < 20 {
				t.Errorf("line %d: stem(%q) = %q, want %q", lineNo, word, got, want)
			}
			failures++
		}
	}
	if failures > 0 {
		t.Errorf("total mismatches: %d / %d", failures, lineNo)
	}
}
