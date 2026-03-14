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
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCharacterUtils_ToLowerUpperCase(t *testing.T) {
	// Test ToLowerCase
	runes := []rune{'A', 'B', 'C'}
	ToLowerCase(runes, 0, 3)
	if runes[0] != 'a' || runes[1] != 'b' || runes[2] != 'c' {
		t.Errorf("ToLowerCase: got %v, want [a b c]", runes)
	}

	// Test ToUpperCase
	runes = []rune{'a', 'b', 'c'}
	ToUpperCase(runes, 0, 3)
	if runes[0] != 'A' || runes[1] != 'B' || runes[2] != 'C' {
		t.Errorf("ToUpperCase: got %v, want [A B C]", runes)
	}
}

func TestCharacterUtils_ToLowerCaseOffset(t *testing.T) {
	// Test with offset
	runes := []rune{'X', 'A', 'B', 'C', 'X'}
	ToLowerCase(runes, 1, 4)
	if runes[0] != 'X' || runes[1] != 'a' || runes[2] != 'b' || runes[3] != 'c' || runes[4] != 'X' {
		t.Errorf("ToLowerCase with offset: got %v, want [X a b c X]", runes)
	}
}

func TestCharacterUtils_ToUpperCaseOffset(t *testing.T) {
	// Test with offset
	runes := []rune{'X', 'a', 'b', 'c', 'X'}
	ToUpperCase(runes, 1, 4)
	if runes[0] != 'X' || runes[1] != 'A' || runes[2] != 'B' || runes[3] != 'C' || runes[4] != 'X' {
		t.Errorf("ToUpperCase with offset: got %v, want [X A B C X]", runes)
	}
}

func TestCharacterUtils_Conversions(t *testing.T) {
	// Test ToCodePoints and ToChars round-trip
	// In Go, each rune is a single code point (unlike Java's UTF-16)
	orig := []rune("Hello 世界")

	// Convert to code points
	codePoints := make([]int, len(orig))
	count, err := ToCodePoints(orig, 0, len(orig), codePoints, 0)
	if err != nil {
		t.Fatalf("ToCodePoints failed: %v", err)
	}

	// Convert back to runes
	restored := make([]rune, len(orig))
	charCount, err := ToChars(codePoints, 0, count, restored, 0)
	if err != nil {
		t.Fatalf("ToChars failed: %v", err)
	}

	if charCount != len(orig) {
		t.Errorf("ToChars returned %d chars, want %d", charCount, len(orig))
	}

	// Verify restored matches original
	for i := 0; i < len(orig); i++ {
		if restored[i] != orig[i] {
			t.Errorf("restored[%d] = %c, want %c", i, restored[i], orig[i])
		}
	}
}

func TestCharacterUtils_ConversionsWithOffset(t *testing.T) {
	// Test with offsets
	orig := []rune("XXHelloXX")
	subset := orig[2:7] // "Hello"

	codePoints := make([]int, 5)
	count, err := ToCodePoints(orig, 2, 5, codePoints, 0)
	if err != nil {
		t.Fatalf("ToCodePoints with offset failed: %v", err)
	}

	restored := make([]rune, 5)
	charCount, err := ToChars(codePoints, 0, count, restored, 0)
	if err != nil {
		t.Fatalf("ToChars failed: %v", err)
	}

	if charCount != 5 {
		t.Errorf("ToChars returned %d chars, want 5", charCount)
	}

	for i := 0; i < 5; i++ {
		if restored[i] != subset[i] {
			t.Errorf("restored[%d] = %c, want %c", i, restored[i], subset[i])
		}
	}
}

func TestCharacterUtils_NewCharacterBuffer(t *testing.T) {
	// Test normal creation
	buffer, err := NewCharacterBuffer(1024)
	if err != nil {
		t.Fatalf("NewCharacterBuffer(1024) failed: %v", err)
	}
	if len(buffer.GetBuffer()) != 1024 {
		t.Errorf("Buffer size = %d, want 1024", len(buffer.GetBuffer()))
	}
	if buffer.GetOffset() != 0 {
		t.Errorf("Buffer offset = %d, want 0", buffer.GetOffset())
	}
	if buffer.GetLength() != 0 {
		t.Errorf("Buffer length = %d, want 0", buffer.GetLength())
	}

	// Test minimum size
	buffer, err = NewCharacterBuffer(2)
	if err != nil {
		t.Fatalf("NewCharacterBuffer(2) failed: %v", err)
	}
	if len(buffer.GetBuffer()) != 2 {
		t.Errorf("Buffer size = %d, want 2", len(buffer.GetBuffer()))
	}

	// Test error on size < 2
	_, err = NewCharacterBuffer(1)
	if err == nil {
		t.Error("NewCharacterBuffer(1) should fail with size < 2")
	}
}

func TestCharacterUtils_FillNoHighSurrogate(t *testing.T) {
	reader := strings.NewReader("helloworld")
	buffer, err := NewCharacterBuffer(6)
	if err != nil {
		t.Fatalf("NewCharacterBuffer failed: %v", err)
	}

	// First fill
	result, err := Fill(buffer, reader, 6)
	if err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if !result {
		t.Error("Fill should return true when buffer filled")
	}
	if buffer.GetOffset() != 0 {
		t.Errorf("Buffer offset = %d, want 0", buffer.GetOffset())
	}
	if buffer.GetLength() != 6 {
		t.Errorf("Buffer length = %d, want 6", buffer.GetLength())
	}

	// Second fill
	result, err = Fill(buffer, reader, 6)
	if err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if result {
		t.Error("Fill should return false when not fully filled")
	}
	if buffer.GetLength() != 4 {
		t.Errorf("Buffer length = %d, want 4", buffer.GetLength())
	}

	// Third fill - should return false (EOF)
	result, err = Fill(buffer, reader, 6)
	if err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if result {
		t.Error("Fill should return false at EOF")
	}
	if buffer.GetLength() != 0 {
		t.Errorf("Buffer length at EOF = %d, want 0", buffer.GetLength())
	}
}

func TestCharacterUtils_FillWithSurrogates(t *testing.T) {
	// Test with surrogate pairs - using valid Unicode supplementary characters
	// U+1041C = \uD801\uDC1C (Deseret Small Letter)
	// Note: Go handles surrogates differently from Java, so we test with valid code points
	input := "1234\U0001041C789123\U0001041C\U0001041C"
	reader := strings.NewReader(input)

	buffer, err := NewCharacterBuffer(5)
	if err != nil {
		t.Fatalf("NewCharacterBuffer failed: %v", err)
	}

	// First fill
	result, err := Fill(buffer, reader, 5)
	if err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if !result {
		t.Error("Fill should return true when buffer filled")
	}
	if buffer.GetLength() < 1 {
		t.Errorf("Buffer length = %d, want >= 1", buffer.GetLength())
	}

	// Continue reading
	for {
		result, err = Fill(buffer, reader, 5)
		if err != nil {
			t.Fatalf("Fill failed: %v", err)
		}
		if !result && buffer.GetLength() == 0 {
			break
		}
	}
}

func TestCharacterUtils_FillBuffer(t *testing.T) {
	reader := strings.NewReader("hello")
	buffer, err := NewCharacterBuffer(10)
	if err != nil {
		t.Fatalf("NewCharacterBuffer failed: %v", err)
	}

	result, err := FillBuffer(buffer, reader)
	if err != nil {
		t.Fatalf("FillBuffer failed: %v", err)
	}
	if result {
		t.Error("FillBuffer should return false for partial read")
	}
	if buffer.GetLength() != 5 {
		t.Errorf("Buffer length = %d, want 5", buffer.GetLength())
	}
}

func TestCharacterUtils_ReadFully(t *testing.T) {
	reader := strings.NewReader("hello world")
	dest := make([]rune, 20)

	count, err := ReadFully(reader, dest, 0, 11)
	if err != nil {
		t.Fatalf("ReadFully failed: %v", err)
	}
	if count != 11 {
		t.Errorf("ReadFully returned %d, want 11", count)
	}
	if string(dest[:count]) != "hello world" {
		t.Errorf("ReadFully read %q, want 'hello world'", string(dest[:count]))
	}
}

func TestCharacterUtils_ReadFullyPartial(t *testing.T) {
	reader := strings.NewReader("hello")
	dest := make([]rune, 20)

	// Try to read more than available
	count, err := ReadFully(reader, dest, 0, 20)
	if err != nil {
		t.Fatalf("ReadFully failed: %v", err)
	}
	if count != 5 {
		t.Errorf("ReadFully returned %d, want 5 (partial)", count)
	}
	if string(dest[:count]) != "hello" {
		t.Errorf("ReadFully read %q, want 'hello'", string(dest[:count]))
	}
}

func TestCharacterUtils_CodePointCount(t *testing.T) {
	tests := []struct {
		name     string
		runes    []rune
		expected int
	}{
		{"ASCII", []rune{'a', 'b', 'c'}, 3},
		{"Unicode", []rune{'世', '界'}, 2},
		{"Mixed", []rune{'H', 'i', ' ', '世', '界'}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := CodePointCount(tt.runes)
			if count != tt.expected {
				t.Errorf("CodePointCount(%v) = %d, want %d", tt.runes, count, tt.expected)
			}
		})
	}
}

func TestCharacterUtils_CodePointAt(t *testing.T) {
	tests := []struct {
		name     string
		runes    []rune
		index    int
		expected rune
	}{
		{"ASCII first", []rune{'a', 'b', 'c'}, 0, 'a'},
		{"ASCII middle", []rune{'a', 'b', 'c'}, 1, 'b'},
		{"ASCII last", []rune{'a', 'b', 'c'}, 2, 'c'},
		{"Unicode", []rune{'世', '界'}, 0, '世'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CodePointAt(tt.runes, tt.index)
			if result != tt.expected {
				t.Errorf("CodePointAt(%v, %d) = %c, want %c", tt.runes, tt.index, result, tt.expected)
			}
		})
	}
}

func TestCharacterUtils_CodePointAtInvalid(t *testing.T) {
	runes := []rune{'a', 'b', 'c'}

	// Test negative index
	result := CodePointAt(runes, -1)
	if result != utf8.RuneError {
		t.Errorf("CodePointAt with negative index should return RuneError, got %c", result)
	}

	// Test index out of bounds
	result = CodePointAt(runes, 10)
	if result != utf8.RuneError {
		t.Errorf("CodePointAt with out of bounds index should return RuneError, got %c", result)
	}
}

func TestCharacterUtils_CharCount(t *testing.T) {
	tests := []struct {
		name     string
		codePoint rune
		expected int
	}{
		{"ASCII", 'a', 1},
		{"BMP", '世', 1},
		{"Supplementary", '\U0001F600', 2}, // Emoji
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := CharCount(tt.codePoint)
			if count != tt.expected {
				t.Errorf("CharCount(%c) = %d, want %d", tt.codePoint, count, tt.expected)
			}
		})
	}
}

func TestCharacterUtils_SurrogateChecks(t *testing.T) {
	// Test IsHighSurrogate
	if !IsHighSurrogate(0xD800) {
		t.Error("IsHighSurrogate(0xD800) = false, want true")
	}
	if !IsHighSurrogate(0xDBFF) {
		t.Error("IsHighSurrogate(0xDBFF) = false, want true")
	}
	if IsHighSurrogate(0xD7FF) {
		t.Error("IsHighSurrogate(0xD7FF) = true, want false")
	}
	if IsHighSurrogate(0xDC00) {
		t.Error("IsHighSurrogate(0xDC00) = true, want false")
	}

	// Test IsLowSurrogate
	if !IsLowSurrogate(0xDC00) {
		t.Error("IsLowSurrogate(0xDC00) = false, want true")
	}
	if !IsLowSurrogate(0xDFFF) {
		t.Error("IsLowSurrogate(0xDFFF) = false, want true")
	}
	if IsLowSurrogate(0xDBFF) {
		t.Error("IsLowSurrogate(0xDBFF) = true, want false")
	}
	if IsLowSurrogate(0xE000) {
		t.Error("IsLowSurrogate(0xE000) = true, want false")
	}

	// Test IsSurrogate
	if !IsSurrogate(0xD800) {
		t.Error("IsSurrogate(0xD800) = false, want true")
	}
	if !IsSurrogate(0xDFFF) {
		t.Error("IsSurrogate(0xDFFF) = false, want true")
	}
	if IsSurrogate(0xD7FF) {
		t.Error("IsSurrogate(0xD7FF) = true, want false")
	}
	if IsSurrogate(0xE000) {
		t.Error("IsSurrogate(0xE000) = true, want false")
	}
}

func TestCharacterUtils_CharacterBufferReset(t *testing.T) {
	buffer, err := NewCharacterBuffer(10)
	if err != nil {
		t.Fatalf("NewCharacterBuffer failed: %v", err)
	}

	// Set some values
	buffer.lastTrailingHighSurrogate = 'X'

	// Reset
	buffer.Reset()

	if buffer.GetOffset() != 0 {
		t.Errorf("After Reset(), offset = %d, want 0", buffer.GetOffset())
	}
	if buffer.GetLength() != 0 {
		t.Errorf("After Reset(), length = %d, want 0", buffer.GetLength())
	}
	if buffer.lastTrailingHighSurrogate != 0 {
		t.Errorf("After Reset(), lastTrailingHighSurrogate = %c, want 0", buffer.lastTrailingHighSurrogate)
	}
}

// Mock reader for testing Fill with errors
type errorReader struct {
	err error
}

func (r *errorReader) ReadRune() (rune, int, error) {
	return 0, 0, r.err
}

func TestCharacterUtils_FillWithError(t *testing.T) {
	expectedErr := errors.New("read error")
	reader := &errorReader{err: expectedErr}

	buffer, err := NewCharacterBuffer(10)
	if err != nil {
		t.Fatalf("NewCharacterBuffer failed: %v", err)
	}

	_, err = Fill(buffer, reader, 10)
	if err == nil {
		t.Error("Fill should return error when reader returns error")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Fill returned wrong error: %v, want %v", err, expectedErr)
	}
}

// Benchmark tests
func BenchmarkCharacterUtils_ToLowerCase(b *testing.B) {
	runes := make([]rune, 1000)
	for i := range runes {
		runes[i] = 'A'
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToLowerCase(runes, 0, len(runes))
	}
}

func BenchmarkCharacterUtils_ToUpperCase(b *testing.B) {
	runes := make([]rune, 1000)
	for i := range runes {
		runes[i] = 'a'
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToUpperCase(runes, 0, len(runes))
	}
}

func BenchmarkCharacterUtils_ToCodePoints(b *testing.B) {
	orig := []rune("Hello 世界 🌍 This is a test string with various characters")
	codePoints := make([]int, len(orig))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToCodePoints(orig, 0, len(orig), codePoints, 0)
	}
}

func BenchmarkCharacterUtils_ToChars(b *testing.B) {
	orig := []rune("Hello 世界 🌍 This is a test string with various characters")
	codePoints := make([]int, len(orig))
	ToCodePoints(orig, 0, len(orig), codePoints, 0)
	restored := make([]rune, len(orig))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToChars(codePoints, 0, len(codePoints), restored, 0)
	}
}

func BenchmarkCharacterUtils_Fill(b *testing.B) {
	data := strings.NewReader(strings.Repeat("Hello World ", 100))
	buffer, _ := NewCharacterBuffer(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Seek(0, io.SeekStart)
		Fill(buffer, data, 1024)
	}
}