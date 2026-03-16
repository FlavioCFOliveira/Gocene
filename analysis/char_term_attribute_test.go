// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestCharTermAttribute_Empty tests that a new attribute is empty.
func TestCharTermAttribute_Empty(t *testing.T) {
	attr := NewCharTermAttribute()
	if attr.Length() != 0 {
		t.Errorf("New attribute length = %d, want 0", attr.Length())
	}
	if attr.String() != "" {
		t.Errorf("New attribute string = %q, want empty", attr.String())
	}
	if len(attr.Buffer()) != 0 {
		t.Errorf("New attribute buffer length = %d, want 0", len(attr.Buffer()))
	}
}

// TestCharTermAttribute_SetValue tests setting the term value.
func TestCharTermAttribute_SetValue(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")

	if attr.String() != "hello" {
		t.Errorf("SetValue('hello') = %q, want 'hello'", attr.String())
	}
	if attr.Length() != 5 {
		t.Errorf("Length after SetValue = %d, want 5", attr.Length())
	}
}

// TestCharTermAttribute_Append tests appending to the term.
func TestCharTermAttribute_Append(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")
	attr.Append([]byte(" world"))

	if attr.String() != "hello world" {
		t.Errorf("After Append = %q, want 'hello world'", attr.String())
	}
}

// TestCharTermAttribute_AppendString tests appending a string.
func TestCharTermAttribute_AppendString(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.AppendString("hello")
	attr.AppendString(" ")
	attr.AppendString("world")

	if attr.String() != "hello world" {
		t.Errorf("After AppendString = %q, want 'hello world'", attr.String())
	}
}

// TestCharTermAttribute_AppendChar tests appending a single character.
func TestCharTermAttribute_AppendChar(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.AppendChar('a')
	attr.AppendChar('b')
	attr.AppendChar('c')

	if attr.String() != "abc" {
		t.Errorf("After AppendChar = %q, want 'abc'", attr.String())
	}
}

// TestCharTermAttribute_SetEmpty tests clearing the term.
func TestCharTermAttribute_SetEmpty(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")
	attr.SetEmpty()

	if attr.Length() != 0 {
		t.Errorf("After SetEmpty, length = %d, want 0", attr.Length())
	}
	if attr.String() != "" {
		t.Errorf("After SetEmpty, string = %q, want empty", attr.String())
	}
}

// TestCharTermAttribute_ResizeBuffer tests resizing the buffer.
func TestCharTermAttribute_ResizeBuffer(t *testing.T) {
	attr := NewCharTermAttribute()

	// Small resize should not change buffer if within capacity
	buf := attr.ResizeBuffer(10)
	if len(buf) < 10 {
		t.Errorf("ResizeBuffer(10) = %d, want at least 10", len(buf))
	}

	// Large resize should grow buffer
	buf = attr.ResizeBuffer(1000)
	if len(buf) < 1000 {
		t.Errorf("ResizeBuffer(1000) = %d, want at least 1000", len(buf))
	}
}

// TestCharTermAttribute_Grow tests growing the buffer.
func TestCharTermAttribute_Grow(t *testing.T) {
	attr := NewCharTermAttribute()

	// Grow should increase capacity
	buf := attr.Grow(50)
	if len(buf) < 50 {
		t.Errorf("Grow(50) = %d, want at least 50", len(buf))
	}
}

// TestCharTermAttribute_SetLength tests setting the length.
func TestCharTermAttribute_SetLength(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello world")

	// Set length to truncate
	attr.SetLength(5)
	if attr.String() != "hello" {
		t.Errorf("After SetLength(5) = %q, want 'hello'", attr.String())
	}
}

// TestCharTermAttribute_Bytes tests getting a copy of the term bytes.
func TestCharTermAttribute_Bytes(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")

	bytes := attr.Bytes()
	if string(bytes) != "hello" {
		t.Errorf("Bytes() = %q, want 'hello'", string(bytes))
	}

	// Modifying returned bytes should not affect the attribute
	bytes[0] = 'x'
	if attr.String() != "hello" {
		t.Errorf("After modifying returned bytes, string = %q, want 'hello'", attr.String())
	}
}

// TestCharTermAttribute_Buffer tests getting the internal buffer.
func TestCharTermAttribute_Buffer(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")

	buf := attr.Buffer()
	// Buffer returns the internal slice, which may be larger than the term
	if len(buf) < 5 {
		t.Errorf("Buffer() length = %d, want at least 5", len(buf))
	}
}

// TestCharTermAttribute_Copy tests copying the attribute.
func TestCharTermAttribute_Copy(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")

	copy := attr.Copy().(CharTermAttribute)
	if copy.String() != "hello" {
		t.Errorf("Copy().String() = %q, want 'hello'", copy.String())
	}

	// Modifying copy should not affect original
	copy.SetValue("world")
	if attr.String() != "hello" {
		t.Errorf("After modifying copy, original = %q, want 'hello'", attr.String())
	}
}

// TestCharTermAttribute_CopyTo tests copying to another attribute.
func TestCharTermAttribute_CopyTo(t *testing.T) {
	attr1 := NewCharTermAttribute()
	attr1.SetValue("hello")

	attr2 := NewCharTermAttribute()
	attr1.CopyTo(attr2)

	if attr2.String() != "hello" {
		t.Errorf("CopyTo: destination = %q, want 'hello'", attr2.String())
	}
}

// TestCharTermAttribute_Clear tests clearing the attribute.
func TestCharTermAttribute_Clear(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")
	attr.Clear()

	if attr.Length() != 0 {
		t.Errorf("After Clear, length = %d, want 0", attr.Length())
	}
}

// TestCharTermAttribute_SetEmptyAndGet tests getting buffer after clear.
func TestCharTermAttribute_SetEmptyAndGet(t *testing.T) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello")

	buf := attr.SetEmptyAndGet()
	if attr.Length() != 0 {
		t.Errorf("After SetEmptyAndGet, length = %d, want 0", attr.Length())
	}
	// Buffer should still have capacity
	if cap(buf) < 5 {
		t.Errorf("SetEmptyAndGet buffer capacity = %d, want at least 5", cap(buf))
	}
}

// TestCharTermAttribute_Chaining tests method chaining.
func TestCharTermAttribute_Chaining(t *testing.T) {
	attr := NewCharTermAttribute()

	// All append methods should return the attribute for chaining
	result := attr.SetValue("hello").
		AppendString(" ").
		Append([]byte("world"))

	if result != attr {
		t.Error("Chaining should return the same attribute")
	}

	if attr.String() != "hello world" {
		t.Errorf("After chaining = %q, want 'hello world'", attr.String())
	}
}

// TestCharTermAttribute_Unicode tests Unicode handling.
func TestCharTermAttribute_Unicode(t *testing.T) {
	attr := NewCharTermAttribute()

	// Test various Unicode strings
	tests := []string{
		"hello",
		"你好世界",
		"こんにちは",
		"مرحبا",
		"Привет",
		"🎉🚀💻",
	}

	for _, test := range tests {
		attr.SetValue(test)
		if attr.String() != test {
			t.Errorf("Unicode test: got %q, want %q", attr.String(), test)
		}
	}
}

// TestCharTermAttribute_LargeTerm tests handling large terms.
func TestCharTermAttribute_LargeTerm(t *testing.T) {
	attr := NewCharTermAttribute()

	// Create a large string
	large := make([]byte, 10000)
	for i := range large {
		large[i] = 'a' + byte(i%26)
	}

	attr.SetValue(string(large))
	if attr.Length() != len(large) {
		t.Errorf("Large term length = %d, want %d", attr.Length(), len(large))
	}

	// Append to grow further
	attr.Append(large)
	if attr.Length() != len(large)*2 {
		t.Errorf("After append, length = %d, want %d", attr.Length(), len(large)*2)
	}
}

// TestCharTermAttribute_SetLengthExceedsBuffer tests SetLength with invalid length.
func TestCharTermAttribute_SetLengthExceedsBuffer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetLength with length > buffer size should panic")
		}
	}()

	attr := NewCharTermAttribute()
	attr.SetValue("hello")
	attr.SetLength(1000) // Should panic
}

// TestCharTermAttribute_MultipleOperations tests multiple operations in sequence.
func TestCharTermAttribute_MultipleOperations(t *testing.T) {
	attr := NewCharTermAttribute()

	// Series of operations
	attr.AppendString("test")
	attr.AppendChar('i')
	attr.AppendString("ng")
	if attr.String() != "testing" {
		t.Errorf("After appends = %q, want 'testing'", attr.String())
	}

	attr.SetEmpty()
	if attr.String() != "" {
		t.Errorf("After clear = %q, want empty", attr.String())
	}

	attr.SetValue("new value")
	if attr.String() != "new value" {
		t.Errorf("After SetValue = %q, want 'new value'", attr.String())
	}

	attr.SetLength(3)
	if attr.String() != "new" {
		t.Errorf("After SetLength(3) = %q, want 'new'", attr.String())
	}
}

// Benchmark tests
func BenchmarkCharTermAttribute_SetValue(b *testing.B) {
	attr := NewCharTermAttribute()
	for i := 0; i < b.N; i++ {
		attr.SetValue("hello world")
	}
}

func BenchmarkCharTermAttribute_Append(b *testing.B) {
	attr := NewCharTermAttribute()
	data := []byte("hello world")
	for i := 0; i < b.N; i++ {
		attr.SetEmpty()
		attr.Append(data)
	}
}

func BenchmarkCharTermAttribute_Copy(b *testing.B) {
	attr := NewCharTermAttribute()
	attr.SetValue("hello world")
	for i := 0; i < b.N; i++ {
		attr.Copy()
	}
}
