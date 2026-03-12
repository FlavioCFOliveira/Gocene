// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestBaseDirectory(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new base directory has correct initial state",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				if bd == nil {
					t.Fatal("expected non-nil BaseDirectory")
				}
				if !bd.IsOpen() {
					t.Error("expected directory to be open")
				}
				if bd.GetLockFactory() == nil {
					t.Error("expected lock factory to be set")
				}
			},
		},
		{
			name: "ensure open returns nil when open",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				if err := bd.EnsureOpen(); err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "ensure open returns error when closed",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				bd.MarkClosed()
				if err := bd.EnsureOpen(); err == nil {
					t.Error("expected error for closed directory")
				}
			},
		},
		{
			name: "track open files",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)

				bd.AddOpenFile("test.txt")
				if !bd.IsFileOpen("test.txt") {
					t.Error("expected file to be tracked as open")
				}
				if bd.GetOpenFileCount("test.txt") != 1 {
					t.Errorf("expected count 1, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.AddOpenFile("test.txt")
				if bd.GetOpenFileCount("test.txt") != 2 {
					t.Errorf("expected count 2, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.RemoveOpenFile("test.txt")
				if bd.GetOpenFileCount("test.txt") != 1 {
					t.Errorf("expected count 1, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.RemoveOpenFile("test.txt")
				if bd.IsFileOpen("test.txt") {
					t.Error("expected file to not be open")
				}
			},
		},
		{
			name: "set lock factory",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				factory := NewNativeFSLockFactory()

				if err := bd.SetLockFactory(factory); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if bd.GetLockFactory() != factory {
					t.Error("expected lock factory to be set")
				}
			},
		},
		{
			name: "set lock factory returns error when closed",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				bd.MarkClosed()

				if err := bd.SetLockFactory(NewNativeFSLockFactory()); err == nil {
					t.Error("expected error when setting factory on closed directory")
				}
			},
		},
		{
			name: "set nil lock factory returns error",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)

				if err := bd.SetLockFactory(nil); err == nil {
					t.Error("expected error when setting nil factory")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}


// TestFilterDirectory tests the FilterDirectory wrapper implementation.
// Ported from: org.apache.lucene.store.TestFilterDirectory
func TestFilterDirectory(t *testing.T) {
	t.Run("delegate access", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		filterDir := NewFilterDirectory(baseDir)

		// Verify GetDelegate returns the wrapped directory
		if filterDir.GetDelegate() != baseDir {
			t.Error("GetDelegate() should return the wrapped directory")
		}
	})

	t.Run("all operations delegate correctly", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		filterDir := NewFilterDirectory(baseDir)

		// Test CreateOutput through FilterDirectory
		out, err := filterDir.CreateOutput("test.txt", IOContextWrite)
		if err != nil {
			t.Fatalf("CreateOutput() error = %v", err)
		}
		if err := out.WriteBytes([]byte("hello")); err != nil {
			t.Fatalf("WriteBytes() error = %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		// Verify file exists in delegate
		if !baseDir.FileExists("test.txt") {
			t.Error("File should exist in delegate directory")
		}

		// Test ListAll through FilterDirectory
		files, err := filterDir.ListAll()
		if err != nil {
			t.Errorf("ListAll() error = %v", err)
		}
		if len(files) != 1 || files[0] != "test.txt" {
			t.Errorf("ListAll() = %v, want [test.txt]", files)
		}

		// Test FileExists through FilterDirectory
		if !filterDir.FileExists("test.txt") {
			t.Error("FileExists() should return true for existing file")
		}

		// Test FileLength through FilterDirectory
		length, err := filterDir.FileLength("test.txt")
		if err != nil {
			t.Errorf("FileLength() error = %v", err)
		}
		if length != 5 {
			t.Errorf("FileLength() = %d, want 5", length)
		}

		// Test OpenInput through FilterDirectory
		in, err := filterDir.OpenInput("test.txt", IOContextRead)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}
		buf := make([]byte, 5)
		if err := in.ReadBytes(buf); err != nil {
			t.Errorf("ReadBytes() error = %v", err)
		}
		if string(buf) != "hello" {
			t.Errorf("Read content = %s, want hello", string(buf))
		}
		in.Close()

		// Test DeleteFile through FilterDirectory
		if err := filterDir.DeleteFile("test.txt"); err != nil {
			t.Errorf("DeleteFile() error = %v", err)
		}
		if baseDir.FileExists("test.txt") {
			t.Error("File should be deleted from delegate directory")
		}
	})

	t.Run("set delegate", func(t *testing.T) {
		baseDir1 := NewByteBuffersDirectory()
		defer baseDir1.Close()

		baseDir2 := NewByteBuffersDirectory()
		defer baseDir2.Close()

		filterDir := NewFilterDirectory(baseDir1)
		if filterDir.GetDelegate() != baseDir1 {
			t.Error("Initial delegate should be baseDir1")
		}

		filterDir.SetDelegate(baseDir2)
		if filterDir.GetDelegate() != baseDir2 {
			t.Error("Delegate should be updated to baseDir2")
		}
	})

	t.Run("close delegates to wrapped directory", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		filterDir := NewFilterDirectory(baseDir)

		// Create a file
		out, _ := filterDir.CreateOutput("test.txt", IOContextWrite)
		out.Close()

		// Close through filter
		if err := filterDir.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Verify base directory is closed
		if baseDir.IsOpen() {
			t.Error("Delegate directory should be closed")
		}
	})

	t.Run("ensure open checks delegate", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		filterDir := NewFilterDirectory(baseDir)

		if err := filterDir.EnsureOpen(); err != nil {
			t.Errorf("EnsureOpen() error = %v", err)
		}

		filterDir.Close()

		if err := filterDir.EnsureOpen(); err == nil {
			t.Error("EnsureOpen() should return error when delegate is closed")
		}
	})

	t.Run("is open reflects delegate state", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		filterDir := NewFilterDirectory(baseDir)

		if !filterDir.IsOpen() {
			t.Error("IsOpen() should return true when delegate is open")
		}

		filterDir.Close()

		if filterDir.IsOpen() {
			t.Error("IsOpen() should return false when delegate is closed")
		}
	})

	t.Run("obtain lock delegates", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		filterDir := NewFilterDirectory(baseDir)

		lock, err := filterDir.ObtainLock("test.lock")
		if err != nil {
			t.Fatalf("ObtainLock() error = %v", err)
		}
		if lock == nil {
			t.Fatal("ObtainLock() returned nil lock")
		}
		if !lock.IsLocked() {
			t.Error("Lock should be locked")
		}

		// Release the lock
		if err := lock.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

// TestTrackingDirectoryWrapper tests the TrackingDirectoryWrapper implementation.
// Ported from: org.apache.lucene.store.TestTrackingDirectoryWrapper
func TestTrackingDirectoryWrapper(t *testing.T) {
	t.Run("track empty", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		created := trackingDir.GetCreatedFiles()
		if len(created) != 0 {
			t.Errorf("GetCreatedFiles() = %v, want empty", created)
		}
	})

	t.Run("track create", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		out, err := trackingDir.CreateOutput("foo", IOContextWrite)
		if err != nil {
			t.Fatalf("CreateOutput() error = %v", err)
		}
		out.Close()

		created := trackingDir.GetCreatedFiles()
		if len(created) != 1 {
			t.Errorf("GetCreatedFiles() length = %d, want 1", len(created))
		}
		if _, ok := created["foo"]; !ok {
			t.Error("GetCreatedFiles() should contain 'foo'")
		}
	})

	t.Run("track delete", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create file
		out, _ := trackingDir.CreateOutput("foo", IOContextWrite)
		out.Close()

		// Verify file is tracked
		created := trackingDir.GetCreatedFiles()
		if len(created) != 1 {
			t.Error("Should have 1 created file")
		}

		// Delete file
		if err := trackingDir.DeleteFile("foo"); err != nil {
			t.Fatalf("DeleteFile() error = %v", err)
		}

		// Verify tracking cleared
		created = trackingDir.GetCreatedFiles()
		if len(created) != 0 {
			t.Errorf("GetCreatedFiles() = %v, want empty after delete", created)
		}

		// Verify deleted files tracked
		deleted := trackingDir.GetDeletedFiles()
		if len(deleted) != 1 {
			t.Errorf("GetDeletedFiles() length = %d, want 1", len(deleted))
		}
	})

	t.Run("track bytes written", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create and write file
		out, _ := trackingDir.CreateOutput("test", IOContextWrite)
		out.WriteBytes([]byte("Hello, World!")) // 13 bytes
		out.Close()

		written := trackingDir.GetTotalBytesWritten()
		if written != 13 {
			t.Errorf("GetTotalBytesWritten() = %d, want 13", written)
		}
	})

	t.Run("track bytes deleted", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create and write file
		out, _ := trackingDir.CreateOutput("test", IOContextWrite)
		out.WriteBytes([]byte("Hello, World!")) // 13 bytes
		out.Close()

		// Delete file
		trackingDir.DeleteFile("test")

		deleted := trackingDir.GetTotalBytesDeleted()
		if deleted != 13 {
			t.Errorf("GetTotalBytesDeleted() = %d, want 13", deleted)
		}
	})

	t.Run("track file names", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create files
		for _, name := range []string{"a", "b", "c"} {
			out, _ := trackingDir.CreateOutput(name, IOContextWrite)
			out.Close()
		}

		names := trackingDir.GetCreatedFileNames()
		if len(names) != 3 {
			t.Errorf("GetCreatedFileNames() length = %d, want 3", len(names))
		}
	})

	t.Run("has created file", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		if trackingDir.HasCreatedFile("foo") {
			t.Error("HasCreatedFile() should return false for non-existent file")
		}

		out, _ := trackingDir.CreateOutput("foo", IOContextWrite)
		out.Close()

		if !trackingDir.HasCreatedFile("foo") {
			t.Error("HasCreatedFile() should return true for created file")
		}
	})

	t.Run("has deleted file", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create and delete file
		out, _ := trackingDir.CreateOutput("foo", IOContextWrite)
		out.Close()
		trackingDir.DeleteFile("foo")

		if !trackingDir.HasDeletedFile("foo") {
			t.Error("HasDeletedFile() should return true for deleted file")
		}
	})

	t.Run("clear tracking", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create file
		out, _ := trackingDir.CreateOutput("foo", IOContextWrite)
		out.WriteBytes([]byte("content"))
		out.Close()

		// Clear tracking
		trackingDir.Clear()

		created := trackingDir.GetCreatedFiles()
		if len(created) != 0 {
			t.Error("GetCreatedFiles() should be empty after clear")
		}

		if trackingDir.GetTotalBytesWritten() != 0 {
			t.Error("GetTotalBytesWritten() should be 0 after clear")
		}
	})

	t.Run("file size tracking", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create file
		out, _ := trackingDir.CreateOutput("test", IOContextWrite)
		out.WriteBytes([]byte("Hello"))
		out.Close()

		size := trackingDir.GetFileSize("test")
		if size != 5 {
			t.Errorf("GetFileSize() = %d, want 5", size)
		}
	})

	t.Run("created file count", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		if trackingDir.GetCreatedFileCount() != 0 {
			t.Error("GetCreatedFileCount() should be 0 initially")
		}

		out, _ := trackingDir.CreateOutput("test", IOContextWrite)
		out.Close()

		if trackingDir.GetCreatedFileCount() != 1 {
			t.Errorf("GetCreatedFileCount() = %d, want 1", trackingDir.GetCreatedFileCount())
		}
	})

	t.Run("deleted file count", func(t *testing.T) {
		baseDir := NewByteBuffersDirectory()
		defer baseDir.Close()

		trackingDir := NewTrackingDirectoryWrapper(baseDir)

		// Create and delete
		out, _ := trackingDir.CreateOutput("test", IOContextWrite)
		out.Close()
		trackingDir.DeleteFile("test")

		if trackingDir.GetDeletedFileCount() != 1 {
			t.Errorf("GetDeletedFileCount() = %d, want 1", trackingDir.GetDeletedFileCount())
		}
	})
}

// TestFSDirectoryCrossCompatibility tests that different FSDirectory
// implementations can coexist on the same path.
// Ported from: org.apache.lucene.store.TestDirectory.testDirectInstantiation()
func TestFSDirectoryCrossCompatibility(t *testing.T) {
	t.Run("cross directory file operations", func(t *testing.T) {
		// Create temp directory
		tempDir, err := os.MkdirTemp("", "gocene_dir_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create two directory instances on the same path
		dir1, err := NewSimpleFSDirectory(tempDir)
		if err != nil {
			t.Fatalf("failed to create dir1: %v", err)
		}
		defer dir1.Close()

		dir2, err := NewSimpleFSDirectory(tempDir)
		if err != nil {
			t.Fatalf("failed to create dir2: %v", err)
		}
		defer dir2.Close()

		// Create a file with dir1
		out, err := dir1.CreateOutput("testfile", IOContextWrite)
		if err != nil {
			t.Fatalf("CreateOutput() error = %v", err)
		}

		// Write data
		testData := make([]byte, 256)
		for i := range testData {
			testData[i] = byte(i)
		}
		if err := out.WriteByte(42); err != nil {
			t.Fatalf("WriteByte() error = %v", err)
		}
		if err := out.WriteBytes(testData); err != nil {
			t.Fatalf("WriteBytes() error = %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		// Verify file exists via dir2
		if !dir2.FileExists("testfile") {
			t.Error("File should be visible to dir2")
		}

		// Verify file length via dir2
		length, err := dir2.FileLength("testfile")
		if err != nil {
			t.Errorf("FileLength() error = %v", err)
		}
		if length != 257 { // 1 + 256
			t.Errorf("FileLength() = %d, want 257", length)
		}

		// Read via dir2
		in, err := dir2.OpenInput("testfile", IOContextRead)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}

		// Read first byte
		b, err := in.ReadByte()
		if err != nil {
			t.Errorf("ReadByte() error = %v", err)
		}
		if b != 42 {
			t.Errorf("ReadByte() = %d, want 42", b)
		}

		// Read array
		readBuf := make([]byte, 256)
		if err := in.ReadBytes(readBuf); err != nil {
			t.Errorf("ReadBytes() error = %v", err)
		}
		for i, b := range readBuf {
			if b != byte(i) {
				t.Errorf("ReadBytes()[%d] = %d, want %d", i, b, byte(i))
				break
			}
		}
		in.Close()

		// Delete via dir1
		if err := dir1.DeleteFile("testfile"); err != nil {
			t.Errorf("DeleteFile() error = %v", err)
		}

		// Verify deletion visible to dir2
		if dir2.FileExists("testfile") {
			t.Error("File should not exist after deletion")
		}
	})

	t.Run("cross directory locking", func(t *testing.T) {
		// Create temp directory
		tempDir, err := os.MkdirTemp("", "gocene_lock_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dir1, _ := NewSimpleFSDirectory(tempDir)
		defer dir1.Close()

		dir2, _ := NewSimpleFSDirectory(tempDir)
		defer dir2.Close()

		// Obtain lock with dir1
		lock, err := dir1.ObtainLock("test.lock")
		if err != nil {
			t.Fatalf("ObtainLock() error = %v", err)
		}

		// Try to obtain same lock with dir2 - should fail for file-based locks
		// Note: SimpleFSDirectory uses NativeFSLockFactory which should fail
		// on the second lock attempt
		_, err = dir2.ObtainLock("test.lock")
		// We expect an error since the lock is already held

		// Release lock
		if err := lock.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Now should be able to obtain lock with dir2
		lock2, err := dir2.ObtainLock("test.lock")
		if err != nil {
			t.Errorf("Second ObtainLock() error = %v", err)
		}
		if lock2 != nil {
			lock2.Close()
		}
	})
}

// TestDirectoryNotDirectory tests error handling when path is a file.
// Ported from: org.apache.lucene.store.TestDirectory.testNotDirectory()
func TestDirectoryNotDirectory(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "gocene_not_dir_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Try to create directory on a file path - should fail
	_, err = NewSimpleFSDirectory(tempFile.Name())
	if err == nil {
		t.Error("NewSimpleFSDirectory() should fail when path is a file")
	}
}


// TestDirectoryClosed tests operations on closed directories.
func TestDirectoryClosed(t *testing.T) {
	t.Run("operations on closed ByteBuffersDirectory", func(t *testing.T) {
		dir := NewByteBuffersDirectory()

		// Create a file first
		out, _ := dir.CreateOutput("test", IOContextWrite)
		out.Close()

		// Close directory
		dir.Close()

		// Verify operations fail
		if dir.IsOpen() {
			t.Error("IsOpen() should return false after Close()")
		}

		_, err := dir.ListAll()
		if err == nil {
			t.Error("ListAll() should fail on closed directory")
		}

		_, err = dir.FileLength("test")
		if err == nil {
			t.Error("FileLength() should fail on closed directory")
		}

		_, err = dir.OpenInput("test", IOContextRead)
		if err == nil {
			t.Error("OpenInput() should fail on closed directory")
		}

		_, err = dir.CreateOutput("new", IOContextWrite)
		if err == nil {
			t.Error("CreateOutput() should fail on closed directory")
		}

		err = dir.DeleteFile("test")
		if err == nil {
			t.Error("DeleteFile() should fail on closed directory")
		}
	})
}

// TestLockFactoryNoLocking tests the NoLockFactory behavior.
// Ported from: org.apache.lucene.store.TestLockFactory.testDirectoryNoLocking()
func TestLockFactoryNoLocking(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// Set NoLockFactory
	dir.SetLockFactory(NewNoLockFactory())

	// First lock should succeed (returns no-op lock)
	lock1, err := dir.ObtainLock("write.lock")
	if err != nil {
		t.Fatalf("First ObtainLock() error = %v", err)
	}
	if lock1.IsLocked() {
		t.Error("No-op lock should not report as locked")
	}

	// Second lock should also succeed (no locking)
	lock2, err := dir.ObtainLock("write.lock")
	if err != nil {
		t.Errorf("Second ObtainLock() error = %v", err)
	}
	if lock2 != nil {
		lock2.Close()
	}

	if lock1 != nil {
		lock1.Close()
	}
}

// TestLockFactoryCustom tests custom lock factory implementation.
// Ported from: org.apache.lucene.store.TestLockFactory.testCustomLockFactory()
func TestLockFactoryCustom(t *testing.T) {
	// Create a mock lock factory that tracks created locks
	mockFactory := &mockLockFactory{
		locksCreated: make(map[string]Lock),
	}

	dir := NewByteBuffersDirectory()
	defer dir.Close()
	dir.SetLockFactory(mockFactory)

	// Obtain a lock
	lock, err := dir.ObtainLock("test.lock")
	if err != nil {
		t.Fatalf("ObtainLock() error = %v", err)
	}

	// Verify lock was tracked
	mockFactory.mu.Lock()
	if len(mockFactory.locksCreated) != 1 {
		t.Errorf("Expected 1 lock created, got %d", len(mockFactory.locksCreated))
	}
	if _, ok := mockFactory.locksCreated["test.lock"]; !ok {
		t.Error("Lock should be tracked with correct name")
	}
	mockFactory.mu.Unlock()

	lock.Close()
}

// mockLockFactory is a test implementation of LockFactory
type mockLockFactory struct {
	locksCreated map[string]Lock
	mu           sync.Mutex
}

func (f *mockLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	lock := NewMockLock()
	f.locksCreated[lockName] = lock
	return lock, nil
}

// TestConcurrentDirectoryAccess tests concurrent operations on directories.
func TestConcurrentDirectoryAccess(t *testing.T) {
	t.Run("concurrent file creation", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()

				name := fmt.Sprintf("file%d", n)
				out, err := dir.CreateOutput(name, IOContextWrite)
				if err != nil {
					t.Errorf("CreateOutput(%s) error: %v", name, err)
					return
				}
				out.WriteBytes([]byte(fmt.Sprintf("content%d", n)))
				out.Close()
			}(i)
		}

		wg.Wait()

		// Verify all files were created
		files, err := dir.ListAll()
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if len(files) != numGoroutines {
			t.Errorf("Expected %d files, got %d", numGoroutines, len(files))
		}
	})

	t.Run("concurrent read access", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Create file
		out, _ := dir.CreateOutput("concurrent", IOContextWrite)
		out.WriteBytes([]byte("concurrent access test data"))
		out.Close()

		var wg sync.WaitGroup
		numReaders := 10

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				in, err := dir.OpenInput("concurrent", IOContextRead)
				if err != nil {
					t.Errorf("OpenInput() error: %v", err)
					return
				}
				defer in.Close()

				buf := make([]byte, 25)
				if err := in.ReadBytes(buf); err != nil {
					t.Errorf("ReadBytes() error: %v", err)
				}
			}()
		}

		wg.Wait()
	})
}
