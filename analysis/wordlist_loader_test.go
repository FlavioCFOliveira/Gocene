/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"strings"
	"testing"
)

func TestWordlistLoader_WordlistLoading(t *testing.T) {
	s := "ONE\n  two \nthree\n\n"

	wordSet1, err := GetWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetWordSetFromReader failed: %v", err)
	}
	checkSet(t, wordSet1.CharArraySet)

	// Test with BufferedReader equivalent (strings.NewReader is already buffered)
	wordSet2, err := GetWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetWordSetFromReader failed: %v", err)
	}
	checkSet(t, wordSet2.CharArraySet)
}

func TestWordlistLoader_Comments(t *testing.T) {
	s := "ONE\n  two \nthree\n#comment"

	wordSet1, err := GetWordSetFromReaderWithComment(strings.NewReader(s), "#")
	if err != nil {
		t.Fatalf("GetWordSetFromReaderWithComment failed: %v", err)
	}
	checkSet(t, wordSet1.CharArraySet)

	if wordSet1.ContainsString("#comment") {
		t.Error("WordSet should not contain '#comment'")
	}
	if wordSet1.ContainsString("comment") {
		t.Error("WordSet should not contain 'comment'")
	}
}

func checkSet(t *testing.T, wordset *CharArraySet) {
	if wordset.Size() != 3 {
		t.Errorf("WordSet size = %d, want 3", wordset.Size())
	}
	if !wordset.ContainsString("ONE") {
		t.Error("WordSet should contain 'ONE' (case not modified)")
	}
	if !wordset.ContainsString("two") {
		t.Error("WordSet should contain 'two' (surrounding whitespace removed)")
	}
	if !wordset.ContainsString("three") {
		t.Error("WordSet should contain 'three'")
	}
	if wordset.ContainsString("four") {
		t.Error("WordSet should not contain 'four'")
	}
}

func TestWordlistLoader_SnowballListLoading(t *testing.T) {
	s := `|comment
 |comment


 |comment | comment
ONE
   two
 three   four five
six seven | comment`

	wordset, err := GetSnowballWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetSnowballWordSetFromReader failed: %v", err)
	}

	if wordset.Size() != 7 {
		t.Errorf("WordSet size = %d, want 7", wordset.Size())
	}
	if !wordset.ContainsString("ONE") {
		t.Error("WordSet should contain 'ONE'")
	}
	if !wordset.ContainsString("two") {
		t.Error("WordSet should contain 'two'")
	}
	if !wordset.ContainsString("three") {
		t.Error("WordSet should contain 'three'")
	}
	if !wordset.ContainsString("four") {
		t.Error("WordSet should contain 'four'")
	}
	if !wordset.ContainsString("five") {
		t.Error("WordSet should contain 'five'")
	}
	if !wordset.ContainsString("six") {
		t.Error("WordSet should contain 'six'")
	}
	if !wordset.ContainsString("seven") {
		t.Error("WordSet should contain 'seven'")
	}
}

func TestWordlistLoader_GetLines(t *testing.T) {
	s := "One \n#Comment \n \n Two \n  Three  \n"

	lines, err := GetLines(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("GetLines returned %d lines, want 3", len(lines))
	}
	if lines[0] != "One" {
		t.Errorf("lines[0] = %q, want 'One'", lines[0])
	}
	if lines[1] != "Two" {
		t.Errorf("lines[1] = %q, want 'Two'", lines[1])
	}
	if lines[2] != "Three" {
		t.Errorf("lines[2] = %q, want 'Three'", lines[2])
	}
}

func TestWordlistLoader_GetLinesWithBOM(t *testing.T) {
	// Test with BOM marker
	s := "\uFEFFOne\nTwo\nThree"

	lines, err := GetLines(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("GetLines returned %d lines, want 3", len(lines))
	}
	if lines[0] != "One" {
		t.Errorf("lines[0] = %q, want 'One' (BOM should be stripped)", lines[0])
	}
}

func TestWordlistLoader_GetWordSetWithResult(t *testing.T) {
	s := "word1\nword2\nword3\n"

	result := NewCharArraySet(10, false)
	_, err := GetWordSet(strings.NewReader(s), result)
	if err != nil {
		t.Fatalf("GetWordSet failed: %v", err)
	}

	if result.Size() != 3 {
		t.Errorf("Result size = %d, want 3", result.Size())
	}
	if !result.ContainsString("word1") {
		t.Error("Result should contain 'word1'")
	}
	if !result.ContainsString("word2") {
		t.Error("Result should contain 'word2'")
	}
	if !result.ContainsString("word3") {
		t.Error("Result should contain 'word3'")
	}
}

func TestWordlistLoader_GetWordSetCaseInsensitive(t *testing.T) {
	s := "Word1\nWORD2\nword3\n"

	result := NewCharArraySet(10, true) // case insensitive
	_, err := GetWordSet(strings.NewReader(s), result)
	if err != nil {
		t.Fatalf("GetWordSet failed: %v", err)
	}

	// In case-insensitive mode, all variations should match
	if !result.ContainsString("word1") {
		t.Error("Result should contain 'word1' (case insensitive)")
	}
	if !result.ContainsString("WORD1") {
		t.Error("Result should contain 'WORD1' (case insensitive)")
	}
	if !result.ContainsString("word2") {
		t.Error("Result should contain 'word2' (case insensitive)")
	}
	if !result.ContainsString("WORD2") {
		t.Error("Result should contain 'WORD2' (case insensitive)")
	}
}

func TestWordlistLoader_GetStemDict(t *testing.T) {
	s := "running\trun\njumping\tjump\nswimming\tswim"

	result := NewCharArrayMap[string](10, false)
	_, err := GetStemDict(strings.NewReader(s), result)
	if err != nil {
		t.Fatalf("GetStemDict failed: %v", err)
	}

	if result.Size() != 3 {
		t.Errorf("Result size = %d, want 3", result.Size())
	}
	if v := result.GetString("running"); v != "run" {
		t.Errorf("stemDict['running'] = %q, want 'run'", v)
	}
	if v := result.GetString("jumping"); v != "jump" {
		t.Errorf("stemDict['jumping'] = %q, want 'jump'", v)
	}
	if v := result.GetString("swimming"); v != "swim" {
		t.Errorf("stemDict['swimming'] = %q, want 'swim'", v)
	}
}

func TestWordlistLoader_GetStemDictCaseInsensitive(t *testing.T) {
	s := "Running\trun\nJumping\tjump"

	result := NewCharArrayMap[string](10, true) // case insensitive
	_, err := GetStemDict(strings.NewReader(s), result)
	if err != nil {
		t.Fatalf("GetStemDict failed: %v", err)
	}

	// In case-insensitive mode
	if v := result.GetString("running"); v != "run" {
		t.Errorf("stemDict['running'] = %q, want 'run' (case insensitive)", v)
	}
	if v := result.GetString("RUNNING"); v != "run" {
		t.Errorf("stemDict['RUNNING'] = %q, want 'run' (case insensitive)", v)
	}
}

func TestWordlistLoader_EmptyInput(t *testing.T) {
	s := ""

	wordSet, err := GetWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetWordSetFromReader failed: %v", err)
	}

	if wordSet.Size() != 0 {
		t.Errorf("WordSet size = %d, want 0 for empty input", wordSet.Size())
	}
}

func TestWordlistLoader_WhitespaceOnly(t *testing.T) {
	s := "   \n\t\n   \n"

	wordSet, err := GetWordSetFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("GetWordSetFromReader failed: %v", err)
	}

	if wordSet.Size() != 0 {
		t.Errorf("WordSet size = %d, want 0 for whitespace-only input", wordSet.Size())
	}
}

func TestWordlistLoader_CommentOnlyLines(t *testing.T) {
	s := "#comment1\n#comment2\n#comment3\n"

	wordSet, err := GetWordSetFromReaderWithComment(strings.NewReader(s), "#")
	if err != nil {
		t.Fatalf("GetWordSetFromReaderWithComment failed: %v", err)
	}

	if wordSet.Size() != 0 {
		t.Errorf("WordSet size = %d, want 0 for comment-only input", wordSet.Size())
	}
}

func TestWordlistLoader_MixedContent(t *testing.T) {
	s := `# This is a comment
word1
   word2
# Another comment
word3
`

	wordSet, err := GetWordSetFromReaderWithComment(strings.NewReader(s), "#")
	if err != nil {
		t.Fatalf("GetWordSetFromReaderWithComment failed: %v", err)
	}

	if wordSet.Size() != 3 {
		t.Errorf("WordSet size = %d, want 3", wordSet.Size())
	}
	if !wordSet.ContainsString("word1") {
		t.Error("WordSet should contain 'word1'")
	}
	if !wordSet.ContainsString("word2") {
		t.Error("WordSet should contain 'word2'")
	}
	if !wordSet.ContainsString("word3") {
		t.Error("WordSet should contain 'word3'")
	}
}

func TestWordlistLoader_GetWordSetFromStrings(t *testing.T) {
	words := []string{"word1", "word2", "word3"}

	set := GetWordSetFromStrings(words, false)
	if set.Size() != 3 {
		t.Errorf("WordSet size = %d, want 3", set.Size())
	}
	for _, word := range words {
		if !set.ContainsString(word) {
			t.Errorf("WordSet should contain %q", word)
		}
	}
}

func TestWordlistLoader_GetWordSetFromStringsCaseInsensitive(t *testing.T) {
	words := []string{"Word1", "WORD2", "word3"}

	set := GetWordSetFromStrings(words, true)
	if set.Size() != 3 {
		t.Errorf("WordSet size = %d, want 3", set.Size())
	}

	// Test case insensitive matching
	if !set.ContainsString("word1") {
		t.Error("WordSet should contain 'word1' (case insensitive)")
	}
	if !set.ContainsString("WORD1") {
		t.Error("WordSet should contain 'WORD1' (case insensitive)")
	}
}

func TestWordlistLoader_SnowballEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only comments",
			input:    "|comment\n|another",
			expected: []string{},
		},
		{
			name:     "words with trailing comments",
			input:    "word1 | comment\nword2|another",
			expected: []string{"word1", "word2"},
		},
		{
			name:     "multiple words per line",
			input:    "word1 word2 word3\nword4 word5",
			expected: []string{"word1", "word2", "word3", "word4", "word5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wordset, err := GetSnowballWordSetFromReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("GetSnowballWordSetFromReader failed: %v", err)
			}

			if wordset.Size() != len(tt.expected) {
				t.Errorf("WordSet size = %d, want %d", wordset.Size(), len(tt.expected))
			}

			for _, word := range tt.expected {
				if !wordset.ContainsString(word) {
					t.Errorf("WordSet should contain %q", word)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkWordlistLoader_GetWordSet(b *testing.B) {
	s := strings.Repeat("word\n", 1000)
	reader := strings.NewReader(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		set := NewCharArraySet(1000, false)
		GetWordSet(reader, set)
	}
}

func BenchmarkWordlistLoader_GetSnowballWordSet(b *testing.B) {
	s := strings.Repeat("word1 word2 word3 | comment\n", 333)
	reader := strings.NewReader(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		set := NewCharArraySet(1000, false)
		GetSnowballWordSet(reader, set)
	}
}

func BenchmarkWordlistLoader_GetLines(b *testing.B) {
	s := strings.Repeat("word\n#comment\n", 500)
	reader := strings.NewReader(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		GetLines(reader)
	}
}
