// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Port of lucene/core/src/test/org/apache/lucene/analysis/TestWordlistLoader.java
// (Apache Lucene 10.5.0): testWordlistLoading, testComments,
// testSnowballListLoading and testGetLines.
//
// Java overload -> Go name: getWordSet(Reader) -> GetWordSet,
// getWordSet(Reader, String) -> GetWordSetWithComment,
// getSnowballWordSet(Reader) -> GetSnowballWordSet,
// getLines(InputStream, Charset) -> GetLines.

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/unicode"
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

func checkWordlistSet(t *testing.T, wordset *UnmodifiableCharArraySet) {
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
	wordset, err := GetSnowballWordSet(strings.NewReader(s))
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

func TestWordlistLoader_GetLines(t *testing.T) {
	s := "One \n#Comment \n \n Two \n  Three  \n"
	charset := unicode.UTF8
	sByteArr, err := charset.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	sInputStream := bytes.NewReader(sByteArr)
	lines, err := GetLines(sInputStream, charset)
	if err != nil {
		t.Fatalf("getLines: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("size: got %d, want 3", len(lines))
	}
	if lines[0] != "One" {
		t.Errorf("lines[0]: got %q, want %q", lines[0], "One")
	}
	if lines[1] != "Two" {
		t.Errorf("lines[1]: got %q, want %q", lines[1], "Two")
	}
	if lines[2] != "Three" {
		t.Errorf("lines[2]: got %q, want %q", lines[2], "Three")
	}
}
