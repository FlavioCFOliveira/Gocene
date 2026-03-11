// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMMapDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("create with valid directory", func(t *testing.T) {
		dir, err := NewMMapDirectory(tempDir)
		if err != nil {
			t.Errorf("NewMMapDirectory() error = %v", err)
			return
		}
		if dir == nil {
			t.Error("NewMMapDirectory() returned nil")
			return
		}
		if dir.GetPath() != tempDir {
			t.Errorf("GetPath() = %v, want %v", dir.GetPath(), tempDir)
		}
		if err := dir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("create with non-existent directory", func(t *testing.T) {
		_, err := NewMMapDirectory("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("NewMMapDirectory() expected error for non-existent directory")
		}
	})

	t.Run("create with file instead of directory", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "gocene_mmap_test_file_*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		_, err = NewMMapDirectory(tempFile.Name())
		if err == nil {
			t.Error("NewMMapDirectory() expected error for file path")
		}
	})
}

func TestMMapDirectory_Settings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("default settings", func(t *testing.T) {
		if dir.GetPreload() {
			t.Error("GetPreload() = true, want false")
		}
		if dir.GetMaxChunkSize() != 30 {
			t.Errorf("GetMaxChunkSize() = %d, want 30", dir.GetMaxChunkSize())
		}
	})

	t.Run("set preload", func(t *testing.T) {
		dir.SetPreload(true)
		if !dir.GetPreload() {
			t.Error("GetPreload() = false after SetPreload(true)")
		}

		dir.SetPreload(false)
		if dir.GetPreload() {
			t.Error("GetPreload() = true after SetPreload(false)")
		}
	})

	t.Run("set chunk size", func(t *testing.T) {
		dir.SetMaxChunkSize(20) // 1MB chunks
		if dir.GetMaxChunkSize() != 20 {
			t.Errorf("GetMaxChunkSize() = %d, want 20", dir.GetMaxChunkSize())
		}

		// Test bounds
		dir.SetMaxChunkSize(0)
		if dir.GetMaxChunkSize() != 1 {
			t.Errorf("GetMaxChunkSize() = %d, want 1 (min)", dir.GetMaxChunkSize())
		}

		dir.SetMaxChunkSize(100)
		if dir.GetMaxChunkSize() != 62 {
			t.Errorf("GetMaxChunkSize() = %d, want 62 (max)", dir.GetMaxChunkSize())
		}
	})
}

func TestMMapDirectory_OpenInput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	ctx := IOContextRead

	t.Run("open existing file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tempDir, "testfile")
		content := []byte("Hello, World! This is a test file for memory mapping.")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		in, err := dir.OpenInput("testfile", ctx)
		if err != nil {
			t.Errorf("OpenInput() error = %v", err)
			return
		}
		defer in.Close()

		if in.Length() != int64(len(content)) {
			t.Errorf("Length() = %d, want %d", in.Length(), len(content))
		}

		// Read all content
		buf := make([]byte, len(content))
		if err := in.ReadBytes(buf); err != nil {
			t.Errorf("ReadBytes() error = %v", err)
			return
		}

		if string(buf) != string(content) {
			t.Errorf("Read content = %s, want %s", string(buf), string(content))
		}
	})

	t.Run("open non-existent file", func(t *testing.T) {
		_, err := dir.OpenInput("nonexistent", ctx)
		if err == nil {
			t.Error("OpenInput() expected error for non-existent file")
		}
	})

	t.Run("open empty file", func(t *testing.T) {
		// Create an empty file
		testFile := filepath.Join(tempDir, "emptyfile")
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		in, err := dir.OpenInput("emptyfile", ctx)
		if err != nil {
			t.Errorf("OpenInput() error = %v", err)
			return
		}
		defer in.Close()

		if in.Length() != 0 {
			t.Errorf("Length() = %d, want 0", in.Length())
		}
	})

	t.Run("read byte by byte", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tempDir, "bytefile")
		content := []byte("ABC")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		in, err := dir.OpenInput("bytefile", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		b1, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
		}
		if b1 != 'A' {
			t.Errorf("ReadByte() = %c, want A", b1)
		}

		b2, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
		}
		if b2 != 'B' {
			t.Errorf("ReadByte() = %c, want B", b2)
		}

		b3, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
		}
		if b3 != 'C' {
			t.Errorf("ReadByte() = %c, want C", b3)
		}

		// Should return EOF at end of file
		_, err = in.ReadByte()
		if err == nil {
			t.Error("ReadByte() expected EOF at end of file")
		}
	})

	t.Run("set position", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "seekfile")
		content := []byte("0123456789")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		in, err := dir.OpenInput("seekfile", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		// Seek to position 5
		if err := in.SetPosition(5); err != nil {
			t.Errorf("SetPosition(5) error = %v", err)
		}

		b, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
		}
		if b != '5' {
			t.Errorf("ReadByte() = %c, want 5", b)
		}
	})

	t.Run("clone", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "clonefile")
		content := []byte("Clone test content")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		in, err := dir.OpenInput("clonefile", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		// Read first byte
		in.ReadByte()

		// Clone
		clone := in.Clone()
		defer clone.Close()

		// Clone should be at position 0
		if clone.GetFilePointer() != 0 {
			t.Errorf("Clone position = %d, want 0", clone.GetFilePointer())
		}

		// Original should still be at position 1
		if in.GetFilePointer() != 1 {
			t.Errorf("Original position = %d, want 1", in.GetFilePointer())
		}
	})

	t.Run("slice", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "slicefile")
		content := []byte("Hello, World!")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		in, err := dir.OpenInput("slicefile", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		slice, err := in.Slice("slice", 7, 5) // "World"
		if err != nil {
			t.Errorf("Slice() error = %v", err)
			return
		}
		defer slice.Close()

		if slice.Length() != 5 {
			t.Errorf("Slice Length() = %d, want 5", slice.Length())
		}

		buf := make([]byte, 5)
		if err := slice.ReadBytes(buf); err != nil {
			t.Errorf("ReadBytes() error = %v", err)
			return
		}

		if string(buf) != "World" {
			t.Errorf("Slice content = %s, want World", string(buf))
		}
	})
}

func TestMMapDirectory_CreateOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	ctx := IOContextWrite

	t.Run("create new file", func(t *testing.T) {
		out, err := dir.CreateOutput("outputfile", ctx)
		if err != nil {
			t.Errorf("CreateOutput() error = %v", err)
			return
		}
		defer out.Close()

		// Write some data
		if err := out.WriteBytes([]byte("Test output")); err != nil {
			t.Errorf("WriteBytes() error = %v", err)
			return
		}

		if out.GetFilePointer() != 11 {
			t.Errorf("GetFilePointer() = %d, want 11", out.GetFilePointer())
		}

		if out.GetName() != "outputfile" {
			t.Errorf("GetName() = %s, want outputfile", out.GetName())
		}
	})

	t.Run("create duplicate file", func(t *testing.T) {
		_, err := dir.CreateOutput("duplicate", ctx)
		if err != nil {
			t.Fatalf("CreateOutput() error = %v", err)
		}

		// Try to create again
		_, err = dir.CreateOutput("duplicate", ctx)
		if err == nil {
			t.Error("CreateOutput() expected error for duplicate file")
		}
	})
}

func TestMMapDirectory_ReadWriteRoundTrip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Write a file using CreateOutput
	writeCtx := IOContextWrite
	out, err := dir.CreateOutput("roundtrip", writeCtx)
	if err != nil {
		t.Fatalf("CreateOutput() error = %v", err)
	}

	content := []byte("This is test content for round-trip verification.")
	if err := out.WriteBytes(content); err != nil {
		t.Fatalf("WriteBytes() error = %v", err)
	}
	out.Close()

	// Read the file using OpenInput (memory-mapped)
	readCtx := IOContextRead
	in, err := dir.OpenInput("roundtrip", readCtx)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Verify length
	if in.Length() != int64(len(content)) {
		t.Errorf("Length() = %d, want %d", in.Length(), len(content))
	}

	// Read and verify content
	buf := make([]byte, len(content))
	if err := in.ReadBytes(buf); err != nil {
		t.Errorf("ReadBytes() error = %v", err)
		return
	}

	if string(buf) != string(content) {
		t.Errorf("Read content = %s, want %s", string(buf), string(content))
	}
}

func TestMMapDirectory_MultiChunkFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create a file larger than a single chunk
	// Set chunk size to something small for testing (64KB)
	dir.SetMaxChunkSize(16) // 2^16 = 64KB

	// Create a test file larger than 64KB
	content := make([]byte, 100*1024) // 100KB
	for i := range content {
		content[i] = byte(i % 256)
	}

	testFile := filepath.Join(tempDir, "largefile")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create large test file: %v", err)
	}

	ctx := IOContextRead
	in, err := dir.OpenInput("largefile", ctx)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	if in.Length() != int64(len(content)) {
		t.Errorf("Length() = %d, want %d", in.Length(), len(content))
	}

	// Read the entire file
	buf := make([]byte, len(content))
	if err := in.ReadBytes(buf); err != nil {
		t.Errorf("ReadBytes() error = %v", err)
		return
	}

	// Verify content
	for i := range content {
		if buf[i] != content[i] {
			t.Errorf("Content mismatch at byte %d: got %d, want %d", i, buf[i], content[i])
			break
		}
	}
}

func TestMMapIndexInput_Close(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create a test file
	testFile := filepath.Join(tempDir, "closefile")
	content := []byte("Close test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ctx := IOContextRead
	in, err := dir.OpenInput("closefile", ctx)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}

	// Close should succeed
	if err := in.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Operations after close should fail
	// (The file should not be tracked anymore)
	if dir.IsFileOpen("closefile") {
		t.Error("File should not be tracked as open after Close()")
	}
}

func TestMMapDirectory_Closed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}

	// Close the directory
	dir.Close()

	ctx := IOContextRead
	_, err = dir.OpenInput("anyfile", ctx)
	if err == nil {
		t.Error("OpenInput() expected error on closed directory")
	}
}
