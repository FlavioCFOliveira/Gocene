// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Port of lucene/core/src/test/org/apache/lucene/analysis/TestWordlistLoader.java
// (Apache Lucene 10.5.0).
//
// Ported methods: testWordlistLoading, testComments, testSnowballListLoading.
// testGetLines calls WordlistLoader.getLines(InputStream, Charset), which is
// not ported (no Gocene rendering of java.nio.charset.Charset exists yet), so
// it is not reproduced here.
//
// Java overload → Go name: getWordSet(Reader) → GetWordSet,
// getWordSet(Reader, String) → GetWordSetWithComment,
// getSnowballWordSet(Reader) → GetSnowballWordSetFromReader.

import (
	"bufio"
	"strings"
	"testing"
)

func TestWordlistLoader_WordlistLoading(t *testing.T) {
	s := "ONE\n  two \nthree\n\n"
	wordSet1, err := GetWordSet(strings.NewReader(s))
	if err != nil {
		t.Fatalf("getWordSet: %v", err)
	}
	checkWordlistSet(t, wordSet1)
	wordSet2, err := GetWordSet(bufio.NewReader(strings.NewReader(s)))
	if err != nil {
		t.Fatalf("getWordSet: %v", err)
	}
	checkWordlistSet(t, wordSet2)
}

func TestWordlistLoader_Comments(t *testing.T) {
	s := "ONE\n  two \nthree\n#comment"
	wordSet1, err := GetWordSetWithComment(strings.NewReader(s), "#")
	if err != nil {
		t.Fatalf("getWordSet: %v", err)
	}
	checkWordlistSet(t, wordSet1)
	if wordSet1.ContainsString("#comment") {
		t.Error("set must not contain #comment")
	}
	if wordSet1.ContainsString("comment") {
		t.Error("set must not contain comment")
	}
}

func checkWordlistSet(t *testing.T, wordset *CharArraySet) {
	t.Helper()
	if wordset.Size() != 3 {
		t.Errorf("size: got %d, want 3", wordset.Size())
	}
	if !wordset.ContainsString("ONE") { // case is not modified
		t.Error("set must contain ONE")
	}
	if !wordset.ContainsString("two") { // surrounding whitespace is removed
		t.Error("set must contain two")
	}
	if !wordset.ContainsString("three") {
		t.Error("set must contain three")
	}
	if wordset.ContainsString("four") {
		t.Error("set must not contain four")
	}
}

// testSnowballListLoading: test stopwords in snowball format.
func TestWordlistLoader_SnowballListLoading(t *testing.T) {
	s := "|comment\n" + // commented line
		" |comment\n" + // commented line with leading whitespace
		"\n" + // blank line
		"  \t\n" + // line with only whitespace
		" |comment | comment\n" + // commented line with comment
		"ONE\n" + // stopword, in uppercase
		"   two   \n" + // stopword with leading/trailing space
		" three   four five \n" + // multiple stopwords
		"six seven | comment\n" // multiple stopwords + comment
	wordset, err := GetSnowballWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("getSnowballWordSet: %v", err)
	}
	if wordset.Size() != 7 {
		t.Errorf("size: got %d, want 7", wordset.Size())
	}
	for _, w := range []string{"ONE", "two", "three", "four", "five", "six", "seven"} {
		if !wordset.ContainsString(w) {
			t.Errorf("set must contain %s", w)
		}
	}
}
