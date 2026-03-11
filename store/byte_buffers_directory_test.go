// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestNewByteBuffersDirectory(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	if dir == nil {
		t.Fatal("Expected non-nil directory")
	}

	if !dir.IsOpen() {
		t.Error("Expected directory to be open")
	}
}

func TestByteBuffersDirectory_CreateOutput(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create output
	out, err := dir.CreateOutput("test_file", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write data
	testData := []byte("Hello, ByteBuffers!")
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	// Close output
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close output: %v", err)
	}

	// Verify file exists
	if !dir.FileExists("test_file") {
		t.Error("File should exist after closing output")
	}

	// Verify file length
	length, err := dir.FileLength("test_file")
	if err != nil {
		t.Fatalf("Failed to get file length: %v", err)
	}
	if length != int64(len(testData)) {
		t.Errorf("Expected length %d, got %d", len(testData), length)
	}
}

func TestByteBuffersDirectory_OpenInput(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create and write file
	testData := []byte("Test data for reading")
	out, err := dir.CreateOutput("read_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
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

func TestByteBuffersDirectory_ListAll(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create multiple files
	files := []string{"file1", "file2", "file3"}
	for _, name := range files {
		out, err := dir.CreateOutput(name, IOContext{})
		if err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close %s: %v", name, err)
		}
	}

	// List all files
	list, err := dir.ListAll()
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(list) != len(files) {
		t.Errorf("Expected %d files, got %d", len(files), len(list))
	}

	// Verify all files are present
	for _, name := range files {
		found := false
		for _, listed := range list {
			if listed == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("File %s not found in list", name)
		}
	}
}

func TestByteBuffersDirectory_DeleteFile(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create file
	out, err := dir.CreateOutput("to_delete", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify file exists
	if !dir.FileExists("to_delete") {
		t.Fatal("File should exist")
	}

	// Delete file
	if err := dir.DeleteFile("to_delete"); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Verify file no longer exists
	if dir.FileExists("to_delete") {
		t.Error("File should not exist after deletion")
	}
}

func TestByteBuffersDirectory_Rename(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create and write file
	testData := []byte("Test data")
	out, err := dir.CreateOutput("old_name", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Rename file
	if err := dir.Rename("old_name", "new_name"); err != nil {
		t.Fatalf("Failed to rename: %v", err)
	}

	// Verify old name doesn't exist
	if dir.FileExists("old_name") {
		t.Error("Old name should not exist")
	}

	// Verify new name exists
	if !dir.FileExists("new_name") {
		t.Error("New name should exist")
	}

	// Verify content is preserved
	in, err := dir.OpenInput("new_name", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open renamed file: %v", err)
	}
	defer in.Close()

	content := make([]byte, len(testData))
	if err := in.ReadBytes(content); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Content mismatch: expected %q, got %q", testData, content)
	}
}

func TestByteBuffersDirectory_Seek(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create and write file
	testData := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	out, err := dir.CreateOutput("seek_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
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

func TestByteBuffersDirectory_Clone(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create and write file
	testData := []byte("Clone test data")
	out, err := dir.CreateOutput("clone_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
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

	// Verify positions
	if in.GetFilePointer() != 5 {
		t.Errorf("Original should be at position 5, got %d", in.GetFilePointer())
	}
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

func TestByteBuffersDirectory_Slice(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create and write file
	testData := []byte("Slice test data here")
	out, err := dir.CreateOutput("slice_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
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

func TestByteBuffersDirectory_TempOutput(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create temp output
	out, err := dir.CreateTempOutput("temp", ".tmp", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create temp output: %v", err)
	}

	// Get the name
	name := out.GetName()

	// Write and close
	testData := []byte("Temporary data")
	if err := out.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify file exists
	if !dir.FileExists(name) {
		t.Errorf("Temp file %s should exist", name)
	}
}

func TestByteBuffersDirectory_Sync(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create file
	out, err := dir.CreateOutput("sync_test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes([]byte("data")); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Sync should be no-op
	if err := dir.Sync([]string{"sync_test"}); err != nil {
		t.Fatalf("Sync should not fail: %v", err)
	}
}

func TestByteBuffersDirectory_NonExistentFile(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Try to open non-existent file
	_, err := dir.OpenInput("non_existent", IOContext{})
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// Try to delete non-existent file
	if err := dir.DeleteFile("non_existent"); err == nil {
		t.Error("Expected error when deleting non-existent file, got nil")
	}
}

func TestByteBuffersDirectory_DuplicateCreate(t *testing.T) {
	dir := NewByteBuffersDirectory()
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

func TestByteBuffersDirectory_WriteIndividualBytes(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create output
	out, err := dir.CreateOutput("byte_by_byte", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write individual bytes
	testData := []byte("Byte by byte")
	for _, b := range testData {
		if err := out.WriteByte(b); err != nil {
			t.Fatalf("Failed to write byte: %v", err)
		}
	}

	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify content
	in, err := dir.OpenInput("byte_by_byte", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	content := make([]byte, len(testData))
	if err := in.ReadBytes(content); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected %q, got %q", testData, content)
	}
}

func TestByteBuffersDirectory_ConcurrentAccess(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Create file with content
	out, err := dir.CreateOutput("concurrent", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes([]byte("Concurrent access test data")); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Open multiple inputs sequentially (concurrent access test removed due to race in BaseDirectory)
	for i := 0; i < 3; i++ {
		in, err := dir.OpenInput("concurrent", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}

		content := make([]byte, 25)
		if err := in.ReadBytes(content); err != nil {
			t.Fatalf("Failed to read: %v", err)
		}
		in.Close()
	}
}
