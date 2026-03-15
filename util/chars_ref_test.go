// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

// TestCharsRef_UTF16InUTF8Order tests UTF-16 in UTF-8 order sorting
// Source: TestCharsRef.testUTF16InUTF8Order()
// Purpose: Tests that UTF-16 strings can be sorted in UTF-8 order
func TestCharsRef_UTF16InUTF8Order(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	numStrings := AtLeast(r, 100)

	utf8 := make([]*BytesRef, numStrings)
	utf16 := make([]*CharsRef, numStrings)

	for i := 0; i < numStrings; i++ {
		s := RandomUnicodeString(r, 100)
		utf8[i] = NewBytesRef([]byte(s))
		utf16[i] = NewCharsRefFromString(s)
	}

	// Sort UTF-8 bytes
	sort.Slice(utf8, func(i, j int) bool {
		return BytesRefCompare(utf8[i], utf8[j]) < 0
	})

	// Sort UTF-16 using UTF-8 comparator
	cmp := UTF16SortedAsUTF8Comparator()
	sort.Slice(utf16, func(i, j int) bool {
		return cmp(utf16[i], utf16[j]) < 0
	})

	for i := 0; i < numStrings; i++ {
		expected := utf8[i].String()
		actual := utf16[i].String()
		if expected != actual {
			t.Errorf("Mismatch at index %d: expected %q, got %q", i, expected, actual)
		}
	}
}

// TestCharsRef_Append tests append operations
// Source: TestCharsRef.testAppend()
// Purpose: Tests appending char arrays to CharsRefBuilder
func TestCharsRef_Append(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	builder := NewCharsRefBuilder()
	var expected string

	numStrings := AtLeast(r, 10)
	for i := 0; i < numStrings; i++ {
		s := RandomRealisticUnicodeString(r, 1, 100)
		charArray := []rune(s)
		offset := r.Intn(len(charArray))
		length := len(charArray) - offset

		expected += string(charArray[offset : offset+length])
		builder.AppendRunes(charArray, offset, length)
	}

	if builder.String() != expected {
		t.Errorf("Expected %q, got %q", expected, builder.String())
	}
}

// TestCharsRef_Copy tests copy operations
// Source: TestCharsRef.testCopy()
// Purpose: Tests copying char arrays to CharsRefBuilder
func TestCharsRef_Copy(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	numIters := AtLeast(r, 10)

	for i := 0; i < numIters; i++ {
		builder := NewCharsRefBuilder()
		s := RandomRealisticUnicodeString(r, 1, 100)
		charArray := []rune(s)
		offset := r.Intn(len(charArray))
		length := len(charArray) - offset

		expected := string(charArray[offset : offset+length])
		builder.CopyRunes(charArray, offset, length)

		if builder.String() != expected {
			t.Errorf("Iteration %d: Expected %q, got %q", i, expected, builder.String())
		}
	}
}

// TestCharsRef_CharAt tests charAt operations
// Source: TestCharsRef.testCharSequenceCharAt()
// Purpose: Tests that charAt fully obeys CharSequence interface
func TestCharsRef_CharAt(t *testing.T) {
	c := NewCharsRefFromString("abc")

	// Test valid access
	if c.CharAt(1) != 'b' {
		t.Errorf("Expected 'b' at index 1, got %c", c.CharAt(1))
	}

	// Test negative index - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative index")
			}
		}()
		c.CharAt(-1)
	}()

	// Test index out of bounds - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for index out of bounds")
			}
		}()
		c.CharAt(3)
	}()
}

// TestCharsRef_SubSequence tests subSequence operations
// Source: TestCharsRef.testCharSequenceSubSequence()
// Purpose: Tests that subSequence fully obeys CharSequence interface
func TestCharsRef_SubSequence(t *testing.T) {
	sequences := []*CharsRef{
		NewCharsRefFromString("abc"),
		NewCharsRefFromRunes([]rune("0abc"), 1, 3),
		NewCharsRefFromRunes([]rune("abc0"), 0, 3),
		NewCharsRefFromRunes([]rune("0abc0"), 1, 3),
	}

	for _, c := range sequences {
		testSequence(t, c)
	}
}

func testSequence(t *testing.T, c *CharsRef) {
	// slice
	if c.SubSequence(0, 1).String() != "a" {
		t.Errorf("Expected 'a' for subSequence(0,1), got %q", c.SubSequence(0, 1).String())
	}

	// mid subsequence
	if c.SubSequence(1, 2).String() != "b" {
		t.Errorf("Expected 'b' for subSequence(1,2), got %q", c.SubSequence(1, 2).String())
	}

	// end subsequence
	if c.SubSequence(1, 3).String() != "bc" {
		t.Errorf("Expected 'bc' for subSequence(1,3), got %q", c.SubSequence(1, 3).String())
	}

	// empty subsequence
	if c.SubSequence(0, 0).String() != "" {
		t.Errorf("Expected empty string for subSequence(0,0), got %q", c.SubSequence(0, 0).String())
	}

	// Test negative start - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative start")
			}
		}()
		c.SubSequence(-1, 1)
	}()

	// Test negative end - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative end")
			}
		}()
		c.SubSequence(0, -1)
	}()

	// Test end > length - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for end > length")
			}
		}()
		c.SubSequence(0, 4)
	}()

	// Test start > end - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for start > end")
			}
		}()
		c.SubSequence(2, 1)
	}()
}

// TestCharsRef_InvalidDeepCopy tests invalid deep copy
// Source: TestCharsRef.testInvalidDeepCopy()
// Purpose: Tests that deep copy of invalid CharsRef throws exception
func TestCharsRef_InvalidDeepCopy(t *testing.T) {
	from := NewCharsRefFromRunes([]rune{'a', 'b'}, 0, 2)
	from.Offset += 1 // now invalid (offset + length > len(chars))

	// This should panic because the offset is now invalid
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid deep copy")
			}
		}()
		DeepCopyOf(from)
	}()
}

// TestCharsRef_Constructor tests various constructors
func TestCharsRef_Constructor(t *testing.T) {
	// Empty constructor
	c1 := NewCharsRef()
	if c1.Length != 0 {
		t.Errorf("Expected length 0, got %d", c1.Length)
	}
	if c1.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", c1.Offset)
	}

	// Capacity constructor
	c2 := NewCharsRefWithCapacity(100)
	if cap(c2.Chars) < 100 {
		t.Errorf("Expected capacity at least 100, got %d", cap(c2.Chars))
	}

	// String constructor
	c3 := NewCharsRefFromString("hello")
	if c3.String() != "hello" {
		t.Errorf("Expected 'hello', got %q", c3.String())
	}

	// Runes constructor
	c4 := NewCharsRefFromRunes([]rune("world"), 0, 5)
	if c4.String() != "world" {
		t.Errorf("Expected 'world', got %q", c4.String())
	}
}

// TestCharsRef_Clone tests the Clone method
func TestCharsRef_Clone(t *testing.T) {
	original := NewCharsRefFromString("original")
	cloned := original.Clone()

	if cloned.String() != "original" {
		t.Errorf("Expected 'original', got %q", cloned.String())
	}

	// Modify original - clone should see the change (shallow copy)
	original.Chars[original.Offset] = 'X'
	if cloned.Chars[cloned.Offset] != 'X' {
		t.Error("Clone should share underlying array")
	}
}

// TestCharsRef_DeepCopyOf tests the DeepCopyOf function
func TestCharsRef_DeepCopyOf(t *testing.T) {
	original := NewCharsRefFromString("original")
	copied := DeepCopyOf(original)

	if copied.String() != "original" {
		t.Errorf("Expected 'original', got %q", copied.String())
	}

	// Modify original - copy should NOT see the change
	original.Chars[original.Offset] = 'X'
	if copied.Chars[copied.Offset] != 'o' {
		t.Error("Deep copy should be independent")
	}

	// Test with nil
	nilCopy := DeepCopyOf(nil)
	if nilCopy != nil {
		t.Error("Expected nil for nil input")
	}

	// Test with empty
	empty := NewCharsRef()
	emptyCopy := DeepCopyOf(empty)
	if emptyCopy.Length != 0 {
		t.Error("Expected empty copy for empty input")
	}
}

// TestCharsRef_CharsEquals tests the CharsEquals method
func TestCharsRef_CharsEquals(t *testing.T) {
	c1 := NewCharsRefFromString("test")
	c2 := NewCharsRefFromString("test")
	c3 := NewCharsRefFromString("other")

	if !c1.CharsEquals(c2) {
		t.Error("Expected equal CharsRef")
	}
	if c1.CharsEquals(c3) {
		t.Error("Expected unequal CharsRef")
	}
	if !c1.CharsEquals(c1) {
		t.Error("Expected equal with itself")
	}

	// Test with nil
	if c1.CharsEquals(nil) {
		t.Error("Expected unequal with nil")
	}
}

// TestCharsRef_CompareTo tests the CompareTo method
func TestCharsRef_CompareTo(t *testing.T) {
	c1 := NewCharsRefFromString("aaa")
	c2 := NewCharsRefFromString("bbb")
	c3 := NewCharsRefFromString("aaa")

	if c1.CompareTo(c2) >= 0 {
		t.Error("Expected aaa < bbb")
	}
	if c2.CompareTo(c1) <= 0 {
		t.Error("Expected bbb > aaa")
	}
	if c1.CompareTo(c3) != 0 {
		t.Error("Expected equal comparison")
	}
	if c1.CompareTo(nil) <= 0 {
		t.Error("Expected anything > nil")
	}
}

// TestCharsRef_HashCode tests the HashCode method
func TestCharsRef_HashCode(t *testing.T) {
	c1 := NewCharsRefFromString("test")
	h1 := c1.HashCode()

	c2 := NewCharsRefFromString("test")
	h2 := c2.HashCode()

	if h1 != h2 {
		t.Error("Expected same hash code for equal content")
	}

	c3 := NewCharsRefFromString("different")
	h3 := c3.HashCode()
	if h1 == h3 {
		t.Error("Expected different hash codes for different content")
	}

	c4 := NewCharsRef()
	if c4.HashCode() != 0 {
		t.Error("Expected 0 hash for empty")
	}
}

// TestCharsRef_IsValid tests the IsValid method
func TestCharsRef_IsValid(t *testing.T) {
	// Valid CharsRef
	c1 := NewCharsRefFromString("test")
	if !c1.IsValid() {
		t.Error("Expected valid CharsRef")
	}

	// Empty CharsRef
	c2 := NewCharsRef()
	if !c2.IsValid() {
		t.Error("Expected empty CharsRef to be valid")
	}

	// Invalid: negative length
	c3 := NewCharsRefFromString("test")
	c3.Length = -1
	if c3.IsValid() {
		t.Error("Expected invalid for negative length")
	}

	// Invalid: length > len(chars)
	c4 := NewCharsRefFromString("test")
	c4.Length = 100
	if c4.IsValid() {
		t.Error("Expected invalid for length > len(chars)")
	}

	// Invalid: negative offset
	c5 := NewCharsRefFromString("test")
	c5.Offset = -1
	if c5.IsValid() {
		t.Error("Expected invalid for negative offset")
	}

	// Invalid: offset + length > len(chars)
	c6 := NewCharsRefFromRunes([]rune("ab"), 0, 2)
	c6.Offset = 1
	if c6.IsValid() {
		t.Error("Expected invalid for offset + length > len(chars)")
	}
}

// TestCharsRef_ValidChars tests the ValidChars method
func TestCharsRef_ValidChars(t *testing.T) {
	c := NewCharsRefFromString("Hello, World!")

	valid := c.ValidChars()
	if string(valid) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %q", string(valid))
	}

	c.Offset = 7
	c.Length = 5
	valid = c.ValidChars()
	if string(valid) != "World" {
		t.Errorf("Expected 'World', got %q", string(valid))
	}

	c2 := NewCharsRef()
	if c2.ValidChars() != nil {
		t.Error("Expected nil for empty CharsRef")
	}
}

// TestCharsRefBuilder_Constructor tests the CharsRefBuilder constructor
func TestCharsRefBuilder_Constructor(t *testing.T) {
	builder := NewCharsRefBuilder()
	if builder == nil {
		t.Fatal("Expected non-nil CharsRefBuilder")
	}
	if builder.Length() != 0 {
		t.Errorf("Expected length 0, got %d", builder.Length())
	}
}

// TestCharsRefBuilder_AppendChar tests appending single chars
func TestCharsRefBuilder_AppendChar(t *testing.T) {
	builder := NewCharsRefBuilder()
	builder.AppendChar('a').AppendChar('b').AppendChar('c')

	if builder.String() != "abc" {
		t.Errorf("Expected 'abc', got %q", builder.String())
	}
}

// TestCharsRefBuilder_Clear tests the Clear method
func TestCharsRefBuilder_Clear(t *testing.T) {
	builder := NewCharsRefBuilder()
	builder.AppendChar('a')
	builder.Clear()

	if builder.Length() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", builder.Length())
	}
	if builder.String() != "" {
		t.Errorf("Expected empty string after clear, got %q", builder.String())
	}
}

// TestCharsRefBuilder_SetCharAt tests the SetCharAt method
func TestCharsRefBuilder_SetCharAt(t *testing.T) {
	builder := NewCharsRefBuilder()
	builder.AppendChar('a')
	builder.AppendChar('b')
	builder.SetCharAt(0, 'x')

	if builder.CharAt(0) != 'x' {
		t.Errorf("Expected 'x' at index 0, got %c", builder.CharAt(0))
	}
}

// TestCharsRefBuilder_CopyChars tests copying from another CharsRef
func TestCharsRefBuilder_CopyChars(t *testing.T) {
	source := NewCharsRefFromString("source")
	builder := NewCharsRefBuilder()
	builder.CopyChars(source)

	if builder.String() != "source" {
		t.Errorf("Expected 'source', got %q", builder.String())
	}

	// Test with nil
	builder2 := NewCharsRefBuilder()
	builder2.AppendChar('x')
	builder2.CopyChars(nil)
	if builder2.Length() != 0 {
		t.Errorf("Expected length 0 after copying nil, got %d", builder2.Length())
	}
}

// TestCharsRefBuilder_ToCharsRef tests the ToCharsRef method
func TestCharsRefBuilder_ToCharsRef(t *testing.T) {
	builder := NewCharsRefBuilder()
	builder.AppendChar('t').AppendChar('e').AppendChar('s').AppendChar('t')

	ref := builder.ToCharsRef()
	if ref.String() != "test" {
		t.Errorf("Expected 'test', got %q", ref.String())
	}

	// Modify builder - ref should NOT change (deep copy)
	builder.Clear()
	builder.AppendChar('x')
	if ref.String() != "test" {
		t.Error("ToCharsRef should return independent copy")
	}
}

// TestCharsRefBuilder_Grow tests the Grow method
func TestCharsRefBuilder_Grow(t *testing.T) {
	builder := NewCharsRefBuilder()
	builder.AppendChar('a')

	builder.Grow(1000)
	if cap(builder.Chars()) < 1000 {
		t.Errorf("Expected capacity at least 1000, got %d", cap(builder.Chars()))
	}

	// Content should be preserved
	if builder.CharAt(0) != 'a' {
		t.Error("Content should be preserved after grow")
	}
}

// TestCharsRef_UTF16SortedAsUTF8Comparator tests the UTF-8 comparator
func TestCharsRef_UTF16SortedAsUTF8Comparator(t *testing.T) {
	cmp := UTF16SortedAsUTF8Comparator()

	// Test basic comparison
	c1 := NewCharsRefFromString("a")
	c2 := NewCharsRefFromString("b")
	if cmp(c1, c2) >= 0 {
		t.Error("Expected a < b")
	}

	// Test equal
	c3 := NewCharsRefFromString("test")
	c4 := NewCharsRefFromString("test")
	if cmp(c3, c4) != 0 {
		t.Error("Expected equal comparison")
	}

	// Test prefix
	c5 := NewCharsRefFromString("test")
	c6 := NewCharsRefFromString("testing")
	if cmp(c5, c6) >= 0 {
		t.Error("Expected shorter string < longer string when prefix")
	}
}

// TestCharsRef_CopyOfSubArray tests the CopyOfSubArray function
func TestCharsRef_CopyOfSubArray(t *testing.T) {
	chars := []rune{'a', 'b', 'c', 'd', 'e'}

	// Normal case
	result := CopyOfSubArray(chars, 1, 4)
	if string(result) != "bcd" {
		t.Errorf("Expected 'bcd', got %q", string(result))
	}

	// from < 0
	result2 := CopyOfSubArray(chars, -1, 2)
	if string(result2) != "ab" {
		t.Errorf("Expected 'ab', got %q", string(result2))
	}

	// to > len
	result3 := CopyOfSubArray(chars, 3, 10)
	if string(result3) != "de" {
		t.Errorf("Expected 'de', got %q", string(result3))
	}

	// from >= to
	result4 := CopyOfSubArray(chars, 3, 3)
	if len(result4) != 0 {
		t.Error("Expected empty for from >= to")
	}
}

// TestCharsRef_Grow tests the Grow function
func TestCharsRef_Grow(t *testing.T) {
	chars := []rune{'a', 'b', 'c'}

	// Grow to larger size
	grown := Grow(chars, 100)
	if cap(grown) < 100 {
		t.Errorf("Expected capacity at least 100, got %d", cap(grown))
	}
	if string(grown[:3]) != "abc" {
		t.Errorf("Expected content preserved, got %q", string(grown[:3]))
	}

	// Grow to smaller size (should not shrink)
	grown2 := Grow(grown, 10)
	if cap(grown2) < 100 {
		t.Error("Grow should not shrink capacity")
	}
}
