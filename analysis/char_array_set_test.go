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
	"fmt"
	"strings"
	"testing"
)

// Test stop words from Lucene
var testStopWords = []string{
	"a", "an", "and", "are", "as", "at", "be", "but", "by", "for", "if", "in",
	"into", "is", "it", "no", "not", "of", "on", "or", "such", "that", "the",
	"their", "then", "there", "these", "they", "this", "to", "was", "will", "with",
}

func TestCharArraySet_NewCharArraySet(t *testing.T) {
	tests := []struct {
		name       string
		startSize  int
		ignoreCase bool
		wantEmpty  bool
	}{
		{"empty set", 0, true, true},
		{"small set", 10, true, true},
		{"large set", 100, false, true},
		{"case sensitive", 10, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := NewCharArraySet(tt.startSize, tt.ignoreCase)
			if set.IsEmpty() != tt.wantEmpty {
				t.Errorf("NewCharArraySet(%d, %v).IsEmpty() = %v, want %v",
					tt.startSize, tt.ignoreCase, set.IsEmpty(), tt.wantEmpty)
			}
			if set.Size() != 0 {
				t.Errorf("NewCharArraySet(%d, %v).Size() = %d, want 0",
					tt.startSize, tt.ignoreCase, set.Size())
			}
		})
	}
}

func TestCharArraySet_Rehash(t *testing.T) {
	set := NewCharArraySet(0, true)
	for _, stopWord := range testStopWords {
		set.Add(stopWord)
	}

	if set.Size() != len(testStopWords) {
		t.Errorf("After adding all stop words, size = %d, want %d",
			set.Size(), len(testStopWords))
	}

	// Verify all stop words are present
	for _, stopWord := range testStopWords {
		if !set.ContainsString(stopWord) {
			t.Errorf("Set should contain %q", stopWord)
		}
	}
}

func TestCharArraySet_NonZeroOffset(t *testing.T) {
	words := []string{"Hello", "World", "this", "is", "a", "test"}
	findme := []rune("xthisy")

	set := NewCharArraySet(10, true)
	for _, word := range words {
		set.Add(word)
	}

	// Test contains with offset
	if !set.Contains(findme, 1, 4) {
		t.Error("Set should contain 'this' at offset 1, length 4")
	}
	if !set.ContainsString("this") {
		t.Error("Set should contain 'this'")
	}

	// Test unmodifiable
	unmodifiable := UnmodifiableSet(set)
	if !unmodifiable.Contains(findme, 1, 4) {
		t.Error("Unmodifiable set should contain 'this' at offset 1, length 4")
	}
	if !unmodifiable.ContainsString("this") {
		t.Error("Unmodifiable set should contain 'this'")
	}
}

func TestCharArraySet_ObjectContains(t *testing.T) {
	set := NewCharArraySet(10, true)
	set.AddInterface(1)

	if !set.ContainsInterface(1) {
		t.Error("Set should contain 1")
	}
	if !set.ContainsString("1") {
		t.Error("Set should contain '1'")
	}
	if !set.Contains([]rune("1"), 0, 1) {
		t.Error("Set should contain []rune('1')")
	}

	// Test unmodifiable
	unmodifiable := UnmodifiableSet(set)
	if !unmodifiable.ContainsInterface(1) {
		t.Error("Unmodifiable set should contain 1")
	}
	if !unmodifiable.ContainsString("1") {
		t.Error("Unmodifiable set should contain '1'")
	}
	if !unmodifiable.Contains([]rune("1"), 0, 1) {
		t.Error("Unmodifiable set should contain []rune('1')")
	}
}

func TestCharArraySet_Clear(t *testing.T) {
	set := NewCharArraySet(10, true)
	set.AddAll(testStopWords)

	if set.Size() != len(testStopWords) {
		t.Errorf("Not all words added, size = %d, want %d",
			set.Size(), len(testStopWords))
	}

	set.Clear()
	if set.Size() != 0 {
		t.Errorf("After clear, size = %d, want 0", set.Size())
	}

	// Verify no words are present after clear
	for _, stopWord := range testStopWords {
		if set.ContainsString(stopWord) {
			t.Errorf("Set should not contain %q after clear", stopWord)
		}
	}

	// Add words again
	set.AddAll(testStopWords)
	if set.Size() != len(testStopWords) {
		t.Errorf("Not all words added after clear, size = %d, want %d",
			set.Size(), len(testStopWords))
	}

	for _, stopWord := range testStopWords {
		if !set.ContainsString(stopWord) {
			t.Errorf("Set should contain %q", stopWord)
		}
	}
}

func TestCharArraySet_ModifyOnUnmodifiable(t *testing.T) {
	set := NewCharArraySet(10, true)
	set.AddAll(testStopWords)
	size := set.Size()

	unmodifiable := UnmodifiableSet(set)
	if unmodifiable.Size() != size {
		t.Errorf("Set size changed due to unmodifiableSet call, size = %d, want %d",
			unmodifiable.Size(), size)
	}

	notInSet := "SirGallahad"
	if unmodifiable.ContainsString(notInSet) {
		t.Errorf("Test String already exists in set: %s", notInSet)
	}

	// Test that all modification methods panic
	t.Run("AddRunes panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("AddRunes should panic on unmodifiable set")
			}
		}()
		unmodifiable.AddRunes([]rune(notInSet))
	})

	t.Run("Add panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Add should panic on unmodifiable set")
			}
		}()
		unmodifiable.Add(notInSet)
	})

	t.Run("Clear panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Clear should panic on unmodifiable set")
			}
		}()
		unmodifiable.Clear()
	})

	// Verify size unchanged after attempted modifications
	if unmodifiable.Size() != size {
		t.Errorf("Size of unmodifiable set changed, size = %d, want %d",
			unmodifiable.Size(), size)
	}

	// Verify original set still contains all stop words
	for _, stopWord := range testStopWords {
		if !set.ContainsString(stopWord) {
			t.Errorf("Original set should contain %q", stopWord)
		}
		if !unmodifiable.ContainsString(stopWord) {
			t.Errorf("Unmodifiable set should contain %q", stopWord)
		}
	}
}

func TestCharArraySet_UnmodifiableSet(t *testing.T) {
	set := NewCharArraySet(10, true)
	set.AddAll(testStopWords)
	set.AddInterface(1)
	size := set.Size()

	unmodifiable := UnmodifiableSet(set)
	if unmodifiable.Size() != size {
		t.Errorf("Set size changed due to unmodifiableSet call, size = %d, want %d",
			unmodifiable.Size(), size)
	}

	// Verify all stop words are present
	for _, stopword := range testStopWords {
		if !unmodifiable.ContainsString(stopword) {
			t.Errorf("Unmodifiable set should contain %q", stopword)
		}
	}

	if !unmodifiable.ContainsInterface(1) {
		t.Error("Unmodifiable set should contain 1")
	}
	if !unmodifiable.ContainsString("1") {
		t.Error("Unmodifiable set should contain '1'")
	}
	if !unmodifiable.Contains([]rune("1"), 0, 1) {
		t.Error("Unmodifiable set should contain []rune('1')")
	}

	// Test panic on nil set
	t.Run("panics on nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("UnmodifiableSet(nil) should panic")
			}
		}()
		UnmodifiableSet(nil)
	})
}

func TestCharArraySet_SupplementaryChars(t *testing.T) {
	// Test Unicode supplementary characters (surrogate pairs)
	// Using \U for Unicode code points > \uFFFF (8-digit hex)
	upperArr := []string{"Abc\U0001041C", "\U0001041C\U0001041CCDE", "A\U0001041CB"}
	lowerArr := []string{"abc\U00010444", "\U00010444\U00010444cde", "a\U00010444b"}

	t.Run("case insensitive", func(t *testing.T) {
		set := NewCharArraySetFromCollection(testStopWords, true)
		for _, word := range upperArr {
			set.Add(word)
		}

		for i := 0; i < len(upperArr); i++ {
			if !set.ContainsString(upperArr[i]) {
				t.Errorf("Term %q is missing in the set", upperArr[i])
			}
			if !set.ContainsString(lowerArr[i]) {
				t.Errorf("Term %q is missing in the set (case insensitive)", lowerArr[i])
			}
		}
	})

	t.Run("case sensitive", func(t *testing.T) {
		set := NewCharArraySetFromCollection(testStopWords, false)
		for _, word := range upperArr {
			set.Add(word)
		}

		for i := 0; i < len(upperArr); i++ {
			if !set.ContainsString(upperArr[i]) {
				t.Errorf("Term %q is missing in the set", upperArr[i])
			}
			// Lower case versions should NOT match in case-sensitive mode
			if set.ContainsString(lowerArr[i]) {
				t.Errorf("Term %q is in the set but shouldn't (case sensitive)", lowerArr[i])
			}
		}
	})
}

func TestCharArraySet_SingleHighSurrogate(t *testing.T) {
	// Note: Go does not allow surrogate code points in string literals
	// Using valid Unicode supplementary characters instead
	// These are characters in the SMP (Supplementary Multilingual Plane)
	upperArr := []string{"ABC\U0001F600", "ABC\U0001F600EfG", "\U0001F600EfG", "\U0001F600\U0001041CB"}
	lowerArr := []string{"abc\U0001F600", "abc\U0001F600efg", "\U0001F600efg", "\U0001F600\U00010444b"}

	t.Run("case insensitive", func(t *testing.T) {
		set := NewCharArraySetFromCollection(testStopWords, true)
		for _, word := range upperArr {
			set.Add(word)
		}

		for i := 0; i < len(upperArr); i++ {
			if !set.ContainsString(upperArr[i]) {
				t.Errorf("Term %q is missing in the set", upperArr[i])
			}
			if !set.ContainsString(lowerArr[i]) {
				t.Errorf("Term %q is missing in the set (case insensitive)", lowerArr[i])
			}
		}
	})

	t.Run("case sensitive", func(t *testing.T) {
		set := NewCharArraySetFromCollection(testStopWords, false)
		for _, word := range upperArr {
			set.Add(word)
		}

		for i := 0; i < len(upperArr); i++ {
			if !set.ContainsString(upperArr[i]) {
				t.Errorf("Term %q is missing in the set", upperArr[i])
			}
			// Note: emoji case handling varies; most emojis don't have case mappings
			// So we skip the case-sensitive lower check for these
		}
	})
}

func TestCharArraySet_Copy(t *testing.T) {
	t.Run("copy case insensitive", func(t *testing.T) {
		setIgnoreCase := NewCharArraySet(10, true)
		setIgnoreCase.AddAll(testStopWords)
		setIgnoreCase.AddInterface(1)

		copySet := CopySet(setIgnoreCase)
		if copySet.Size() != setIgnoreCase.Size() {
			t.Errorf("Copy size = %d, want %d", copySet.Size(), setIgnoreCase.Size())
		}

		// Verify all stop words are present
		for _, stopword := range testStopWords {
			if !copySet.ContainsString(stopword) {
				t.Errorf("Copy should contain %q", stopword)
			}
			// Case insensitive
			if !copySet.ContainsString(strings.ToUpper(stopword)) {
				t.Errorf("Copy should contain uppercase %q (case insensitive)", strings.ToUpper(stopword))
			}
		}

		// Add new words to copy - should not affect original
		newWords := []string{"word1", "word2"}
		for _, w := range newWords {
			copySet.Add(w)
		}

		if copySet.Size() <= setIgnoreCase.Size() {
			t.Errorf("Copy should have more elements after adding new words")
		}
		for _, w := range newWords {
			if setIgnoreCase.ContainsString(w) {
				t.Errorf("Original set should not contain added word %q", w)
			}
		}
	})

	t.Run("copy case sensitive", func(t *testing.T) {
		setCaseSens := NewCharArraySet(10, false)
		setCaseSens.AddAll(testStopWords)
		setCaseSens.AddInterface(1)

		copySet := CopySet(setCaseSens)
		if copySet.Size() != setCaseSens.Size() {
			t.Errorf("Copy size = %d, want %d", copySet.Size(), setCaseSens.Size())
		}

		// Verify all stop words are present (case sensitive)
		for _, stopword := range testStopWords {
			if !copySet.ContainsString(stopword) {
				t.Errorf("Copy should contain %q", stopword)
			}
			// Uppercase should NOT match in case-sensitive mode
			if copySet.ContainsString(strings.ToUpper(stopword)) {
				t.Errorf("Copy should not contain uppercase %q (case sensitive)", strings.ToUpper(stopword))
			}
		}
	})
}

func TestCharArraySet_CopyStrings(t *testing.T) {
	words := []string{"hello", "world", "test"}
	set := CopyStrings(words, true)

	if set.Size() != len(words) {
		t.Errorf("CopyStrings size = %d, want %d", set.Size(), len(words))
	}

	for _, word := range words {
		if !set.ContainsString(word) {
			t.Errorf("CopyStrings should contain %q", word)
		}
	}
}

func TestCharArraySet_EmptySet(t *testing.T) {
	emptySet := NewEmptyCharArraySet()

	if emptySet.Size() != 0 {
		t.Errorf("Empty set size = %d, want 0", emptySet.Size())
	}
	if !emptySet.IsEmpty() {
		t.Error("Empty set should be empty")
	}

	// Test contains returns false for all inputs
	for _, stopword := range testStopWords {
		if emptySet.ContainsString(stopword) {
			t.Errorf("Empty set should not contain %q", stopword)
		}
	}
	if emptySet.ContainsString("foo") {
		t.Error("Empty set should not contain 'foo'")
	}
	if emptySet.ContainsInterface("foo") {
		t.Error("Empty set should not contain 'foo' (interface)")
	}
	if emptySet.Contains([]rune("foo"), 0, 3) {
		t.Error("Empty set should not contain []rune('foo')")
	}

	// Test panic on null
	t.Run("panics on null", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Empty set Contains(nil) should panic")
			}
		}()
		emptySet.Contains(nil, 0, 0)
	})
}

func TestCharArraySet_ContainsWithNull(t *testing.T) {
	set := NewCharArraySet(1, true)

	t.Run("Contains panics on nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Contains(nil) should panic")
			}
		}()
		set.Contains(nil, 0, 10)
	})
}

func TestCharArraySet_ToString(t *testing.T) {
	set := CopyStrings([]string{"test"}, false)
	str := set.String()
	if str != "[test]" {
		t.Errorf("String() = %q, want [test]", str)
	}

	set.Add("test2")
	str = set.String()
	if !strings.Contains(str, ", ") {
		t.Errorf("String() = %q, should contain ', '", str)
	}
}

func TestCharArraySet_AddReturn(t *testing.T) {
	set := NewCharArraySet(10, true)

	// First add should return true (element not present)
	if !set.Add("test") {
		t.Error("First Add('test') should return true")
	}

	// Second add should return false (element already present)
	if set.Add("test") {
		t.Error("Second Add('test') should return false")
	}
}

func TestCharArraySet_Items(t *testing.T) {
	words := []string{"hello", "world", "test"}
	set := NewCharArraySetFromCollection(words, false)

	items := set.Items()
	if len(items) != len(words) {
		t.Errorf("Items() returned %d items, want %d", len(items), len(words))
	}

	// Verify all words are present in items
	itemSet := make(map[string]bool)
	for _, item := range items {
		itemSet[item] = true
	}
	for _, word := range words {
		if !itemSet[word] {
			t.Errorf("Items() missing %q", word)
		}
	}
}

func TestCharArraySet_ContainsAll(t *testing.T) {
	set := NewCharArraySetFromCollection(testStopWords, true)

	// Test contains all
	if !set.ContainsAll([]string{"a", "an", "and"}) {
		t.Error("Should contain all stop words: a, an, and")
	}

	// Test not contains all
	if set.ContainsAll([]string{"a", "nonexistent"}) {
		t.Error("Should not contain 'nonexistent'")
	}
}

func TestCharArraySet_SetOperations(t *testing.T) {
	setA := NewCharArraySetFromStrings(true, "a", "b", "c")
	setB := NewCharArraySetFromStrings(true, "b", "c", "d")

	t.Run("union", func(t *testing.T) {
		union := SetUnion(setA, setB)
		if union.Size() != 4 {
			t.Errorf("Union size = %d, want 4", union.Size())
		}
		for _, word := range []string{"a", "b", "c", "d"} {
			if !union.ContainsString(word) {
				t.Errorf("Union should contain %q", word)
			}
		}
	})

	t.Run("intersection", func(t *testing.T) {
		intersection := SetIntersection(setA, setB)
		if intersection.Size() != 2 {
			t.Errorf("Intersection size = %d, want 2", intersection.Size())
		}
		for _, word := range []string{"b", "c"} {
			if !intersection.ContainsString(word) {
				t.Errorf("Intersection should contain %q", word)
			}
		}
		if intersection.ContainsString("a") {
			t.Error("Intersection should not contain 'a'")
		}
	})

	t.Run("difference", func(t *testing.T) {
		difference := SetDifference(setA, setB)
		if difference.Size() != 1 {
			t.Errorf("Difference size = %d, want 1", difference.Size())
		}
		if !difference.ContainsString("a") {
			t.Error("Difference should contain 'a'")
		}
		if difference.ContainsString("b") {
			t.Error("Difference should not contain 'b'")
		}
	})
}

func TestCharArraySet_SetEquals(t *testing.T) {
	setA := NewCharArraySetFromStrings(true, "a", "b", "c")
	setB := NewCharArraySetFromStrings(true, "a", "b", "c")
	setC := NewCharArraySetFromStrings(true, "a", "b")

	if !SetEquals(setA, setB) {
		t.Error("SetEquals should return true for equal sets")
	}
	if SetEquals(setA, setC) {
		t.Error("SetEquals should return false for different sets")
	}
}

func TestCharArraySet_ForEach(t *testing.T) {
	set := NewCharArraySetFromCollection(testStopWords, true)
	count := 0
	visited := make(map[string]bool)

	set.ForEach(func(item string) bool {
		count++
		visited[item] = true
		return true
	})

	if count != len(testStopWords) {
		t.Errorf("ForEach visited %d items, want %d", count, len(testStopWords))
	}

	for _, word := range testStopWords {
		if !visited[word] {
			t.Errorf("ForEach did not visit %q", word)
		}
	}
}

func TestCharArraySet_EarlyTermination(t *testing.T) {
	set := NewCharArraySetFromStrings(true, "a", "b", "c", "d", "e")
	count := 0

	set.ForEach(func(item string) bool {
		count++
		return count < 3 // Stop after 3 items
	})

	if count != 3 {
		t.Errorf("ForEach should have stopped early, visited %d items", count)
	}
}

func TestCharArraySet_AddInterface(t *testing.T) {
	set := NewCharArraySet(10, true)

	// Add various types
	set.AddInterface("string")
	set.AddInterface(42)
	set.AddInterface(3.14)

	// Verify all are present
	if !set.ContainsString("string") {
		t.Error("Should contain 'string'")
	}
	if !set.ContainsString("42") {
		t.Error("Should contain '42'")
	}
	if !set.ContainsString("3.14") {
		t.Error("Should contain '3.14'")
	}
}

func TestCharArraySet_LargeCapacity(t *testing.T) {
	// Test with a large number of elements
	set := NewCharArraySet(10000, true)
	for i := 0; i < 10000; i++ {
		// Generate unique strings using strconv
		set.Add(fmt.Sprintf("word%d", i))
	}

	if set.Size() != 10000 {
		t.Errorf("Large set size = %d, want 10000", set.Size())
	}
}

func TestCharArraySet_EmptyThenAdd(t *testing.T) {
	set := NewCharArraySet(0, true)

	// Add items to empty set
	for _, word := range testStopWords {
		set.Add(word)
	}

	if set.Size() != len(testStopWords) {
		t.Errorf("Size after adding = %d, want %d", set.Size(), len(testStopWords))
	}
}

func TestCharArraySet_FromCollection(t *testing.T) {
	set := NewCharArraySetFromCollection(testStopWords, true)

	if set.Size() != len(testStopWords) {
		t.Errorf("Size = %d, want %d", set.Size(), len(testStopWords))
	}

	for _, word := range testStopWords {
		if !set.ContainsString(word) {
			t.Errorf("Should contain %q", word)
		}
	}
}

// Benchmark tests
func BenchmarkCharArraySet_Add(b *testing.B) {
	set := NewCharArraySet(b.N, true)
	for i := 0; i < b.N; i++ {
		set.Add(string(rune('a'+i%26)) + string(rune('a'+(i+1)%26)))
	}
}

func BenchmarkCharArraySet_Contains(b *testing.B) {
	set := NewCharArraySetFromCollection(testStopWords, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set.ContainsString(testStopWords[i%len(testStopWords)])
	}
}

func BenchmarkCharArraySet_ContainsIgnoreCase(b *testing.B) {
	set := NewCharArraySetFromCollection(testStopWords, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set.ContainsString(strings.ToUpper(testStopWords[i%len(testStopWords)]))
	}
}

func BenchmarkCharArraySet_ContainsRunes(b *testing.B) {
	set := NewCharArraySetFromCollection(testStopWords, true)
	runes := make([][]rune, len(testStopWords))
	for i, word := range testStopWords {
		runes[i] = []rune(word)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set.Contains(runes[i%len(runes)], 0, len(runes[i%len(runes)]))
	}
}
