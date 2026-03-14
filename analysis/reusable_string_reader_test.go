// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"testing"
)

// TestReusableStringReader_Basic tests basic reading functionality.
// Source: TestReusableStringReader.java
// Purpose: Tests that the reader can read from a string.
func TestReusableStringReader_Basic(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("hello world")

	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, got %d", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Expected 'hello', got %q", string(buf))
	}
}

// TestReusableStringReader_EOF tests EOF handling.
// Source: TestReusableStringReader.java
// Purpose: Tests that EOF is returned at the end of the string.
func TestReusableStringReader_EOF(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("hi")

	// Read all content
	buf := make([]byte, 10)
	n, err := reader.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 bytes, got %d", n)
	}
	if string(buf[:n]) != "hi" {
		t.Errorf("Expected 'hi', got %q", string(buf[:n]))
	}

	// Subsequent read should return EOF with 0 bytes
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF on second read, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes on EOF, got %d", n)
	}
}

// TestReusableStringReader_Reset tests resetting the reader.
// Source: TestReusableStringReader.java
// Purpose: Tests that the reader can be reset to the beginning.
func TestReusableStringReader_Reset(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("test")

	// Read some content
	buf := make([]byte, 2)
	reader.Read(buf)

	// Reset and read again
	reader.Reset()

	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read error after reset: %v", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 bytes after reset, got %d", n)
	}
	if string(buf) != "te" {
		t.Errorf("Expected 'te' after reset, got %q", string(buf))
	}
}

// TestReusableStringReader_Reuse tests reusing the reader with different strings.
// Source: TestReusableStringReader.java
// Purpose: Tests that the reader can be reused with different inputs.
func TestReusableStringReader_Reuse(t *testing.T) {
	reader := NewReusableStringReader()

	// First string
	reader.SetValue("first")
	buf := make([]byte, 10)
	n, _ := reader.Read(buf)
	if string(buf[:n]) != "first" {
		t.Errorf("Expected 'first', got %q", string(buf[:n]))
	}

	// Second string
	reader.SetValue("second")
	n, _ = reader.Read(buf)
	if string(buf[:n]) != "second" {
		t.Errorf("Expected 'second', got %q", string(buf[:n]))
	}
}

// TestReusableStringReader_Length tests the Length method.
// Source: TestReusableStringReader.java
// Purpose: Tests that the length is tracked correctly.
func TestReusableStringReader_Length(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("hello")

	if reader.Length() != 5 {
		t.Errorf("Expected length 5, got %d", reader.Length())
	}

	reader.SetValue("world!")
	if reader.Length() != 6 {
		t.Errorf("Expected length 6, got %d", reader.Length())
	}
}

// TestReusableStringReader_Position tests the Position method.
// Source: TestReusableStringReader.java
// Purpose: Tests that the position is tracked correctly.
func TestReusableStringReader_Position(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("hello")

	if reader.Position() != 0 {
		t.Errorf("Initial position should be 0, got %d", reader.Position())
	}

	buf := make([]byte, 3)
	reader.Read(buf)

	if reader.Position() != 3 {
		t.Errorf("Position should be 3 after reading 3 bytes, got %d", reader.Position())
	}
}

// TestReusableStringReader_ReadRune tests reading runes.
// Source: TestReusableStringReader.java
// Purpose: Tests that runes can be read correctly.
func TestReusableStringReader_ReadRune(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("hello")

	ch, size, err := reader.ReadRune()
	if err != nil {
		t.Fatalf("ReadRune error: %v", err)
	}
	if ch != 'h' {
		t.Errorf("Expected 'h', got %c", ch)
	}
	if size != 1 {
		t.Errorf("Expected size 1 for ASCII, got %d", size)
	}
}

// TestReusableStringReader_ReadRune_UTF8 tests reading UTF-8 runes.
// Source: TestReusableStringReader.java
// Purpose: Tests that UTF-8 runes can be read correctly.
func TestReusableStringReader_ReadRune_UTF8(t *testing.T) {
	reader := NewReusableStringReader()
	reader.SetValue("日本語") // Japanese characters

	ch, size, err := reader.ReadRune()
	if err != nil {
		t.Fatalf("ReadRune error: %v", err)
	}
	if ch != '日' {
		t.Errorf("Expected '日', got %c", ch)
	}
	if size != 3 {
		t.Errorf("Expected size 3 for Japanese character, got %d", size)
	}
}
