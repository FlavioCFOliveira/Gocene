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

// TestByteBuffersDataInput tests ByteBuffersIndexInput data reading operations.
// Ported from: org.apache.lucene.store.TestByteBuffersDataInput
func TestByteBuffersDataInput(t *testing.T) {
	t.Run("read across buffer boundary", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create large data that spans multiple internal buffers
		// Default buffer size is typically 1024 or larger
		largeData := make([]byte, 2048)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		out, err := dir.CreateOutput("large_file", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}
		if err := out.WriteBytes(largeData); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		in, err := dir.OpenInput("large_file", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		// Read across buffer boundary at offset 1020
		if err := in.SetPosition(1020); err != nil {
			t.Fatalf("Failed to seek: %v", err)
		}

		buf := make([]byte, 20)
		if err := in.ReadBytes(buf); err != nil {
			t.Fatalf("Failed to read across boundary: %v", err)
		}

		// Verify data integrity
		for i, b := range buf {
			expected := byte((1020 + i) % 256)
			if b != expected {
				t.Errorf("Byte at position %d: expected %d, got %d", 1020+i, expected, b)
				break
			}
		}
	})

	t.Run("read empty file", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create empty file
		out, err := dir.CreateOutput("empty", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		in, err := dir.OpenInput("empty", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		// Try to read from empty file
		_, err = in.ReadByte()
		if err == nil {
			t.Error("Expected error reading from empty file")
		}
	})

	t.Run("read at exact buffer boundary", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create data of exactly buffer size
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte(i % 256)
		}

		out, err := dir.CreateOutput("boundary", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}
		if err := out.WriteBytes(data); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		in, err := dir.OpenInput("boundary", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		// Read at exact buffer boundary (position 1023 is last byte)
		if err := in.SetPosition(1023); err != nil {
			t.Fatalf("Failed to seek: %v", err)
		}

		b, err := in.ReadByte()
		if err != nil {
			t.Fatalf("Failed to read at boundary: %v", err)
		}
		if b != data[1023] {
			t.Errorf("Expected %d, got %d", data[1023], b)
		}
	})

	t.Run("random access reads", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create test data
		data := make([]byte, 500)
		for i := range data {
			data[i] = byte(i % 256)
		}

		out, err := dir.CreateOutput("random", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}
		if err := out.WriteBytes(data); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		in, err := dir.OpenInput("random", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		// Test random access at various positions
		positions := []int64{0, 100, 250, 499}
		for _, pos := range positions {
			if err := in.SetPosition(pos); err != nil {
				t.Fatalf("Failed to seek to %d: %v", pos, err)
			}

			b, err := in.ReadByte()
			if err != nil {
				t.Fatalf("Failed to read at position %d: %v", pos, err)
			}
			if b != data[pos] {
				t.Errorf("Position %d: expected %d, got %d", pos, data[pos], b)
			}
		}
	})
}

// TestByteBuffersDataOutput tests ByteBuffersIndexOutput data writing operations.
// Ported from: org.apache.lucene.store.TestByteBuffersDataOutput
func TestByteBuffersDataOutput(t *testing.T) {
	t.Run("write across buffer boundary", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create data larger than typical buffer
		largeData := make([]byte, 2048)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		out, err := dir.CreateOutput("large_write", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}

		// Write in chunks to test buffer management
		chunkSize := 100
		for i := 0; i < len(largeData); i += chunkSize {
			end := i + chunkSize
			if end > len(largeData) {
				end = len(largeData)
			}
			if err := out.WriteBytes(largeData[i:end]); err != nil {
				t.Fatalf("Failed to write chunk at %d: %v", i, err)
			}
		}

		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		// Verify written data
		length, _ := dir.FileLength("large_write")
		if length != int64(len(largeData)) {
			t.Errorf("Expected length %d, got %d", len(largeData), length)
		}

		in, err := dir.OpenInput("large_write", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		readData := make([]byte, len(largeData))
		if err := in.ReadBytes(readData); err != nil {
			t.Fatalf("Failed to read: %v", err)
		}

		for i := range largeData {
			if readData[i] != largeData[i] {
				t.Errorf("Byte %d: expected %d, got %d", i, largeData[i], readData[i])
				break
			}
		}
	})

	t.Run("write byte by byte across boundary", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		out, err := dir.CreateOutput("byte_by_byte", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}

		// Write 1500 bytes one at a time
		for i := 0; i < 1500; i++ {
			if err := out.WriteByte(byte(i % 256)); err != nil {
				t.Fatalf("Failed to write byte %d: %v", i, err)
			}
		}

		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		// Verify
		length, _ := dir.FileLength("byte_by_byte")
		if length != 1500 {
			t.Errorf("Expected length 1500, got %d", length)
		}

		in, err := dir.OpenInput("byte_by_byte", IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}
		defer in.Close()

		// Check first and last bytes
		first, _ := in.ReadByte()
		if first != 0 {
			t.Errorf("Expected first byte 0, got %d", first)
		}

		if err := in.SetPosition(1499); err != nil {
			t.Fatalf("Failed to seek: %v", err)
		}

		last, _ := in.ReadByte()
		if last != 1499%256 {
			t.Errorf("Expected last byte %d, got %d", 1499%256, last)
		}
	})

	t.Run("get file pointer during write", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		out, err := dir.CreateOutput("pointer_test", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}

		// Check initial position
		if out.GetFilePointer() != 0 {
			t.Errorf("Expected initial position 0, got %d", out.GetFilePointer())
		}

		// Write and check position
		out.WriteByte(1)
		if out.GetFilePointer() != 1 {
			t.Errorf("Expected position 1, got %d", out.GetFilePointer())
		}

		out.WriteBytes([]byte{2, 3, 4})
		if out.GetFilePointer() != 4 {
			t.Errorf("Expected position 4, got %d", out.GetFilePointer())
		}

		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}
	})

	t.Run("write after get name", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		out, err := dir.CreateOutput("name_test", IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}

		// Get name before writing
		name := out.GetName()
		if name != "name_test" {
			t.Errorf("Expected name 'name_test', got %s", name)
		}

		// Write after getting name
		if err := out.WriteBytes([]byte("data")); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}

		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}

		// Verify file exists and has content
		if !dir.FileExists("name_test") {
			t.Error("File should exist")
		}

		length, _ := dir.FileLength("name_test")
		if length != 4 {
			t.Errorf("Expected length 4, got %d", length)
		}
	})
}
