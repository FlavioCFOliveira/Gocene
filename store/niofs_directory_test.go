// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewNIOFSDirectory(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test creating NIOFSDirectory
	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	if dir.GetPath() != tempDir {
		t.Errorf("Expected path %s, got %s", tempDir, dir.GetPath())
	}
}

func TestNIOFSDirectory_CreateOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create output
	out, err := dir.CreateOutput("test_file", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write data
	testData := []byte("Hello, NIOFS!")
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	// Close output
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close output: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tempDir, "test_file")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created")
	}

	// Verify file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected %q, got %q", testData, content)
	}
}

func TestNIOFSDirectory_OpenInput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create file with content
	testData := []byte("Test data for reading")
	testFile := filepath.Join(tempDir, "read_test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open input
	in, err := dir.OpenInput("read_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Read single byte
	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read byte: %v", err)
	}
	if b != testData[0] {
		t.Errorf("Expected byte %d, got %d", testData[0], b)
	}

	// Read remaining bytes
	remaining := make([]byte, len(testData)-1)
	if err := in.ReadBytes(remaining); err != nil {
		t.Fatalf("Failed to read bytes: %v", err)
	}
	if string(remaining) != string(testData[1:]) {
		t.Errorf("Expected %q, got %q", testData[1:], remaining)
	}
}

func TestNIOFSDirectory_Seek(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create file with content
	testData := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	testFile := filepath.Join(tempDir, "seek_test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open input
	in, err := dir.OpenInput("seek_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Seek to position 10
	if err := in.SetPosition(10); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	// Read byte at position 10
	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read byte: %v", err)
	}
	if b != testData[10] {
		t.Errorf("Expected byte %c at position 10, got %c", testData[10], b)
	}
}

func TestNIOFSDirectory_Clone(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create file with content
	testData := []byte("Clone test data")
	testFile := filepath.Join(tempDir, "clone_test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open input
	in, err := dir.OpenInput("clone_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Read first 5 bytes
	first5 := make([]byte, 5)
	if err := in.ReadBytes(first5); err != nil {
		t.Fatalf("Failed to read bytes: %v", err)
	}

	// Clone the input
	cloned := in.Clone()
	defer cloned.Close()

	// Original should be at position 5
	if in.GetFilePointer() != 5 {
		t.Errorf("Original should be at position 5, got %d", in.GetFilePointer())
	}

	// Clone should also be at position 5
	if cloned.GetFilePointer() != 5 {
		t.Errorf("Clone should be at position 5, got %d", cloned.GetFilePointer())
	}

	// Read remaining from clone
	remaining := make([]byte, len(testData)-5)
	if err := cloned.ReadBytes(remaining); err != nil {
		t.Fatalf("Failed to read from clone: %v", err)
	}
	if string(remaining) != string(testData[5:]) {
		t.Errorf("Expected %q, got %q", testData[5:], remaining)
	}
}

func TestNIOFSDirectory_Slice(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create file with content
	testData := []byte("Slice test data here")
	testFile := filepath.Join(tempDir, "slice_test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open input
	in, err := dir.OpenInput("slice_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Create slice from position 6 with length 4
	slice, err := in.Slice("slice desc", 6, 4)
	if err != nil {
		t.Fatalf("Failed to create slice: %v", err)
	}
	defer slice.Close()

	// Read slice content
	sliceData := make([]byte, 4)
	if err := slice.ReadBytes(sliceData); err != nil {
		t.Fatalf("Failed to read from slice: %v", err)
	}
	if string(sliceData) != "test" {
		t.Errorf("Expected 'test', got %q", sliceData)
	}
}

func TestNIOFSDirectory_BufferedWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create output
	out, err := dir.CreateOutput("buffered_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write individual bytes (tests buffering)
	testData := []byte("Buffered write test")
	for _, b := range testData {
		if err := out.WriteByte(b); err != nil {
			t.Fatalf("Failed to write byte: %v", err)
		}
	}

	// Close to flush buffer
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close output: %v", err)
	}

	// Verify content
	filePath := filepath.Join(tempDir, "buffered_test")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected %q, got %q", testData, content)
	}
}

func TestNIOFSDirectory_NonExistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Try to open non-existent file
	_, err = dir.OpenInput("non_existent", IOContext{})
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestNIOFSDirectory_DuplicateCreate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_niofs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewNIOFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to create NIOFSDirectory: %v", err)
	}
	defer dir.Close()

	// Create first output
	out1, err := dir.CreateOutput("duplicate", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create first output: %v", err)
	}
	out1.Close()

	// Try to create second output with same name
	_, err = dir.CreateOutput("duplicate", IOContext{})
	if err == nil {
		t.Error("Expected error for duplicate file, got nil")
	}
}
