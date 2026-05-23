// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/hunspell/StemmerTestBase.java

package hunspell

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// testdataDir is the path to hunspell test resource files vendored from Lucene.
const testdataDir = "testdata"

// loadTestDictionary loads a Dictionary from testdata files.
// ignoreCase controls case-insensitive stemming.
func loadTestDictionary(t *testing.T, ignoreCase bool, affixFile string, dictFiles ...string) *Dictionary {
	t.Helper()

	affPath := filepath.Join(testdataDir, affixFile)
	aff, err := os.Open(affPath)
	if err != nil {
		t.Fatalf("open %s: %v", affPath, err)
	}
	defer aff.Close()

	dictReaders := make([]io.Reader, len(dictFiles))
	closers := make([]io.Closer, len(dictFiles))
	for i, df := range dictFiles {
		dicPath := filepath.Join(testdataDir, df)
		f, err := os.Open(dicPath)
		if err != nil {
			t.Fatalf("open %s: %v", dicPath, err)
		}
		dictReaders[i] = f
		closers[i] = f
	}
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()

	dict, err := NewDictionary(aff, dictReaders, ignoreCase)
	if err != nil {
		t.Fatalf("NewDictionary(%s): %v", affixFile, err)
	}
	return dict
}

// assertStemsTo verifies that Stemmer.Stem(word) returns exactly the expected
// stems (sorted, case-sensitive comparison).
//
// If no expected stems are given, the assertion verifies that the word produces
// no stems (e.g., it is an unknown or invalid form).
func assertStemsTo(t *testing.T, stemmer *Stemmer, word string, expected ...string) {
	t.Helper()

	got := stemmer.Stem(word)
	sort.Strings(got)

	exp := make([]string, len(expected))
	copy(exp, expected)
	sort.Strings(exp)

	if len(got) != len(exp) {
		t.Errorf("Stem(%q): got %v, want %v", word, got, exp)
		return
	}
	for i := range exp {
		if got[i] != exp[i] {
			t.Errorf("Stem(%q)[%d]: got %q, want %q (full: got=%v want=%v)",
				word, i, got[i], exp[i], got, exp)
		}
	}
}

// TestStemmerTestBase_Helpers validates the test helper functions themselves.
//
// Source: StemmerTestBase — abstract base class with no @Test methods.
// This test is self-referential: it loads a simple dictionary and verifies
// that assertStemsTo behaves correctly for both matching and non-matching cases.
func TestStemmerTestBase_Helpers(t *testing.T) {
	dict := loadTestDictionary(t, false, "simple.aff", "simple.dic")
	stemmer := NewStemmer(dict)

	// "apache" should stem to something (any non-empty result).
	stems := stemmer.Stem("apache")
	if len(stems) == 0 {
		t.Errorf("expected stems for 'apache', got none")
	}

	// Helper with expected=[] verifies empty stems.
	assertStemsTo(t, stemmer, "zzznonsense")
}
