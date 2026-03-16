// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFSDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("create with valid directory", func(t *testing.T) {
		dir, err := NewFSDirectory(tempDir)
		if err != nil {
			t.Errorf("NewFSDirectory() error = %v", err)
			return
		}
		if dir == nil {
			t.Error("NewFSDirectory() returned nil")
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
		_, err := NewFSDirectory("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("NewFSDirectory() expected error for non-existent directory")
		}
	})

	t.Run("create with file instead of directory", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "gocene_test_file_*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		_, err = NewFSDirectory(tempFile.Name())
		if err == nil {
			t.Error("NewFSDirectory() expected error for file path")
		}
	})

	t.Run("create with read-only directory", func(t *testing.T) {
		readOnlyDir, err := os.MkdirTemp("", "gocene_readonly_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(readOnlyDir)

		// Make directory read-only
		if err := os.Chmod(readOnlyDir, 0555); err != nil {
			t.Fatalf("failed to chmod: %v", err)
		}
		defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

		_, err = NewFSDirectory(readOnlyDir)
		if err == nil {
			t.Error("NewFSDirectory() expected error for read-only directory")
		}
	})
}

func TestFSDirectory_ListAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("empty directory", func(t *testing.T) {
		files, err := dir.ListAll()
		if err != nil {
			t.Errorf("ListAll() error = %v", err)
			return
		}
		if len(files) != 0 {
			t.Errorf("ListAll() = %v, want empty slice", files)
		}
	})

	t.Run("with files", func(t *testing.T) {
		// Create test files
		for _, name := range []string{"file1", "file2", "file3"} {
			path := filepath.Join(tempDir, name)
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		// Create a subdirectory (should be skipped)
		subDir := filepath.Join(tempDir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}

		// Create a hidden file (should be skipped)
		hiddenFile := filepath.Join(tempDir, ".hidden")
		if err := os.WriteFile(hiddenFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create hidden file: %v", err)
		}

		// Create a lock file (should be skipped)
		lockFile := filepath.Join(tempDir, "test.lock")
		if err := os.WriteFile(lockFile, []byte("lock"), 0644); err != nil {
			t.Fatalf("failed to create lock file: %v", err)
		}

		files, err := dir.ListAll()
		if err != nil {
			t.Errorf("ListAll() error = %v", err)
			return
		}
		if len(files) != 3 {
			t.Errorf("ListAll() returned %d files, want 3", len(files))
		}
		for i, name := range files {
			want := []string{"file1", "file2", "file3"}[i]
			if name != want {
				t.Errorf("ListAll()[%d] = %s, want %s", i, name, want)
			}
		}
	})

	t.Run("closed directory", func(t *testing.T) {
		tempDir2, _ := os.MkdirTemp("", "gocene_test_closed_*")
		defer os.RemoveAll(tempDir2)

		dir2, _ := NewFSDirectory(tempDir2)
		dir2.Close()

		_, err := dir2.ListAll()
		if err == nil {
			t.Error("ListAll() expected error on closed directory")
		}
	})
}

func TestFSDirectory_FileExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("existing file", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "testfile")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		if !dir.FileExists("testfile") {
			t.Error("FileExists() = false for existing file")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		if dir.FileExists("nonexistent") {
			t.Error("FileExists() = true for non-existent file")
		}
	})

	t.Run("closed directory", func(t *testing.T) {
		tempDir2, _ := os.MkdirTemp("", "gocene_test_closed_*")
		defer os.RemoveAll(tempDir2)

		dir2, _ := NewFSDirectory(tempDir2)
		dir2.Close()

		if dir2.FileExists("any") {
			t.Error("FileExists() should return false on closed directory")
		}
	})
}

func TestFSDirectory_FileLength(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("existing file", func(t *testing.T) {
		content := []byte("Hello, World!")
		testFile := filepath.Join(tempDir, "testfile")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		length, err := dir.FileLength("testfile")
		if err != nil {
			t.Errorf("FileLength() error = %v", err)
			return
		}
		if length != int64(len(content)) {
			t.Errorf("FileLength() = %d, want %d", length, len(content))
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := dir.FileLength("nonexistent")
		if err == nil {
			t.Error("FileLength() expected error for non-existent file")
		}
	})
}

func TestFSDirectory_DeleteFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("delete existing file", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "deleteme")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		if err := dir.DeleteFile("deleteme"); err != nil {
			t.Errorf("DeleteFile() error = %v", err)
			return
		}

		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("DeleteFile() did not delete the file")
		}
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		err := dir.DeleteFile("nonexistent")
		if err == nil {
			t.Error("DeleteFile() expected error for non-existent file")
		}
	})
}

func TestFSDirectory_Sync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("sync existing files", func(t *testing.T) {
		// Create test files
		for _, name := range []string{"file1", "file2"} {
			path := filepath.Join(tempDir, name)
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		if err := dir.Sync([]string{"file1", "file2"}); err != nil {
			t.Errorf("Sync() error = %v", err)
		}
	})

	t.Run("sync non-existent file", func(t *testing.T) {
		err := dir.Sync([]string{"nonexistent"})
		if err == nil {
			t.Error("Sync() expected error for non-existent file")
		}
	})
}

func TestFSDirectory_Rename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("rename existing file", func(t *testing.T) {
		source := filepath.Join(tempDir, "source")
		dest := filepath.Join(tempDir, "dest")

		if err := os.WriteFile(source, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		if err := dir.Rename("source", "dest"); err != nil {
			t.Errorf("Rename() error = %v", err)
			return
		}

		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Error("Rename() source still exists")
		}
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			t.Error("Rename() destination does not exist")
		}
	})

	t.Run("rename non-existent source", func(t *testing.T) {
		err := dir.Rename("nonexistent", "dest")
		if err == nil {
			t.Error("Rename() expected error for non-existent source")
		}
	})

	t.Run("rename to existing destination", func(t *testing.T) {
		source := filepath.Join(tempDir, "source2")
		dest := filepath.Join(tempDir, "dest2")

		os.WriteFile(source, []byte("test"), 0644)
		os.WriteFile(dest, []byte("test"), 0644)

		err := dir.Rename("source2", "dest2")
		if err == nil {
			t.Error("Rename() expected error when destination exists")
		}
	})
}

func TestSimpleFSDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("create and close", func(t *testing.T) {
		dir, err := NewSimpleFSDirectory(tempDir)
		if err != nil {
			t.Errorf("NewSimpleFSDirectory() error = %v", err)
			return
		}
		if dir == nil {
			t.Error("NewSimpleFSDirectory() returned nil")
			return
		}

		if err := dir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestSimpleFSDirectory_CreateOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewSimpleFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create SimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	ctx := IOContextWrite

	t.Run("create new file", func(t *testing.T) {
		out, err := dir.CreateOutput("testfile", ctx)
		if err != nil {
			t.Errorf("CreateOutput() error = %v", err)
			return
		}
		defer out.Close()

		// Write some data
		if err := out.WriteBytes([]byte("Hello, World!")); err != nil {
			t.Errorf("WriteBytes() error = %v", err)
			return
		}

		if out.GetFilePointer() != 13 {
			t.Errorf("GetFilePointer() = %d, want 13", out.GetFilePointer())
		}

		if out.GetName() != "testfile" {
			t.Errorf("GetName() = %s, want testfile", out.GetName())
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

func TestSimpleFSDirectory_OpenInput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewSimpleFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create SimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	ctx := IOContextRead

	// Create a test file
	testFile := filepath.Join(tempDir, "testinput")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("open existing file", func(t *testing.T) {
		in, err := dir.OpenInput("testinput", ctx)
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

	t.Run("read byte", func(t *testing.T) {
		in, err := dir.OpenInput("testinput", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		b, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
			return
		}
		if b != 'H' {
			t.Errorf("ReadByte() = %c, want H", b)
		}
	})

	t.Run("clone", func(t *testing.T) {
		in, err := dir.OpenInput("testinput", ctx)
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
		in, err := dir.OpenInput("testinput", ctx)
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

	t.Run("set position", func(t *testing.T) {
		in, err := dir.OpenInput("testinput", ctx)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		defer in.Close()

		if err := in.SetPosition(7); err != nil {
			t.Errorf("SetPosition() error = %v", err)
			return
		}

		if in.GetFilePointer() != 7 {
			t.Errorf("GetFilePointer() = %d, want 7", in.GetFilePointer())
		}

		b, _ := in.ReadByte()
		if b != 'W' {
			t.Errorf("ReadByte() = %c, want W", b)
		}
	})
}

func TestFSLock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create FSDirectory: %v", err)
	}
	defer dir.Close()

	t.Run("obtain and release lock", func(t *testing.T) {
		lock, err := dir.ObtainLock("test")
		if err != nil {
			t.Errorf("ObtainLock() error = %v", err)
			return
		}

		if !lock.IsLocked() {
			t.Error("IsLocked() = false after obtaining lock")
		}

		if err := lock.EnsureValid(); err != nil {
			t.Errorf("EnsureValid() error = %v", err)
		}

		if err := lock.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		if lock.IsLocked() {
			t.Error("IsLocked() = true after releasing lock")
		}
	})

	t.Run("obtain lock twice should fail", func(t *testing.T) {
		lock, err := dir.ObtainLock("double")
		if err != nil {
			t.Fatalf("First ObtainLock() error = %v", err)
		}
		defer lock.Close()

		_, err = dir.ObtainLock("double")
		if err == nil {
			t.Error("Second ObtainLock() should fail")
		}
	})
}

func TestOpen(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("open existing directory", func(t *testing.T) {
		dir, err := Open(tempDir)
		if err != nil {
			t.Errorf("Open() error = %v", err)
			return
		}
		if dir == nil {
			t.Error("Open() returned nil")
			return
		}
		if err := dir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("open non-existent directory", func(t *testing.T) {
		_, err := Open("/nonexistent/path")
		if err == nil {
			t.Error("Open() expected error for non-existent directory")
		}
	})
}

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "test.txt", false},
		{"valid with underscore", "test_file.txt", false},
		{"valid with hyphen", "test-file.txt", false},
		{"valid with numbers", "test123.txt", false},
		{"valid with dot", "test.file.txt", false},
		{"empty name", "", true},
		{"path traversal double dot", "../etc/passwd", true},
		{"path traversal single", "..", true},
		{"path traversal nested", "foo/../../../etc/passwd", true},
		{"absolute path unix", "/etc/passwd", true},
		{"null byte", "test\x00.txt", true},
		{"forward slash", "test/file.txt", true},
		{"backslash", "test\\file.txt", true},
		{"invalid chars space", "test file.txt", true},
		{"invalid chars colon", "test:file.txt", true},
		{"invalid chars star", "test*file.txt", true},
		{"invalid chars question", "test?file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestFSDirectory_PathTraversalProtection(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gocene_security_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file outside the temp directory to test protection
	outsideFile := filepath.Join(tempDir, "..", "outside_test.txt")

	dir, err := NewSimpleFSDirectory(tempDir)
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory() error = %v", err)
	}
	defer dir.Close()

	// Test CreateOutput with path traversal
	t.Run("create output blocks path traversal", func(t *testing.T) {
		_, err := dir.CreateOutput("../outside_test.txt", IOContext{})
		if err == nil {
			t.Error("CreateOutput() should reject path traversal")
		}
	})

	// Test OpenInput with path traversal
	t.Run("open input blocks path traversal", func(t *testing.T) {
		_, err := dir.OpenInput("../outside_test.txt", IOContext{})
		if err == nil {
			t.Error("OpenInput() should reject path traversal")
		}
	})

	// Test DeleteFile with path traversal
	t.Run("delete file blocks path traversal", func(t *testing.T) {
		err := dir.DeleteFile("../outside_test.txt")
		if err == nil {
			t.Error("DeleteFile() should reject path traversal")
		}
	})

	// Test FileExists with path traversal
	t.Run("file exists blocks path traversal", func(t *testing.T) {
		exists := dir.FileExists("../outside_test.txt")
		if exists {
			t.Error("FileExists() should reject path traversal and return false")
		}
	})

	// Test FileLength with path traversal
	t.Run("file length blocks path traversal", func(t *testing.T) {
		_, err := dir.FileLength("../outside_test.txt")
		if err == nil {
			t.Error("FileLength() should reject path traversal")
		}
	})

	// Test Rename with path traversal in source
	t.Run("rename blocks path traversal in source", func(t *testing.T) {
		err := dir.Rename("../outside_test.txt", "dest.txt")
		if err == nil {
			t.Error("Rename() should reject path traversal in source")
		}
	})

	// Test Rename with path traversal in destination
	t.Run("rename blocks path traversal in destination", func(t *testing.T) {
		err := dir.Rename("source.txt", "../outside_test.txt")
		if err == nil {
			t.Error("Rename() should reject path traversal in destination")
		}
	})

	// Test Sync with path traversal
	t.Run("sync blocks path traversal", func(t *testing.T) {
		err := dir.Sync([]string{"../outside_test.txt"})
		if err == nil {
			t.Error("Sync() should reject path traversal")
		}
	})

	// Test ObtainLock with path traversal
	t.Run("obtain lock blocks path traversal", func(t *testing.T) {
		_, err := dir.ObtainLock("../outside_lock")
		if err == nil {
			t.Error("ObtainLock() should reject path traversal")
		}
	})

	// Verify the outside file was not created
	if _, err := os.Stat(outsideFile); err == nil {
		t.Error("Path traversal protection failed - outside file was created")
	}
}
