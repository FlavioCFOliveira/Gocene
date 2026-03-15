// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCloser is a test implementation of io.Closer that can be configured to return errors
type testCloser struct {
	closeErr error
	panicVal interface{}
	closed   bool
}

func (c *testCloser) Close() error {
	c.closed = true
	if c.panicVal != nil {
		panic(c.panicVal)
	}
	return c.closeErr
}

// testError is a custom error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestDeleteFilesIgnoringExceptions tests deleting files while ignoring exceptions
// Source: TestIOUtils.testDeleteFileIgnoringExceptions()
func TestDeleteFilesIgnoringExceptions(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1")

	// Create the file
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete the file - should succeed without error
	DeleteFilesIgnoringExceptions(file1)

	// Verify file was deleted
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}
}

// TestDeleteFilesIgnoringExceptions_NonExistent tests deleting non-existent files
// Source: TestIOUtils.testDontDeleteFileIgnoringExceptions()
func TestDeleteFilesIgnoringExceptions_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "nonexistent_file")

	// Should not throw exception for non-existent file
	DeleteFilesIgnoringExceptions(file1)
	// No assertion needed - test passes if no panic/error
}

// TestDeleteFilesIgnoringExceptions_Mixed tests deleting multiple files where some exist
// Source: TestIOUtils.testDeleteTwoFilesIgnoringExceptions()
func TestDeleteFilesIgnoringExceptions_Mixed(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1")
	file2 := filepath.Join(tempDir, "file2")

	// Only create file2
	if err := os.WriteFile(file2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete both files - should succeed even though file1 doesn't exist
	DeleteFilesIgnoringExceptions(file1, file2)

	// Verify file2 was deleted
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("Expected file2 to be deleted")
	}
}

// TestCloseWhileSuppressingExceptions tests the close while suppressing exceptions behavior
// Source: TestIOUtils.testCloseExposesErrors()
func TestCloseWhileSuppressingExceptions(t *testing.T) {
	// Test 1: Exception close adds suppressed exception to primary error
	t.Run("ExceptionAddsSuppressed", func(t *testing.T) {
		exceptionClose := &testCloser{closeErr: errors.New("IO error")}
		primaryErr := errors.New("primary error")

		result := CloseWhileSuppressingExceptions(primaryErr, exceptionClose)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// The result should contain the primary error with suppressed close error
		if !strings.Contains(result.Error(), "primary error") {
			t.Errorf("Expected result to contain primary error, got: %v", result)
		}
		if !strings.Contains(result.Error(), "IO error") {
			t.Errorf("Expected result to contain suppressed IO error, got: %v", result)
		}
	})

	// Test 2: Error takes precedence over Exception
	t.Run("ErrorTakesPrecedence", func(t *testing.T) {
		ioError := &testCloser{closeErr: errors.New("IO error")}
		panicError := &testCloser{panicVal: "panic!"}
		primaryErr := errors.New("primary error")

		result := CloseWhileSuppressingExceptions(primaryErr, ioError, panicError)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// Panic error should take precedence
		if !strings.Contains(result.Error(), "panic") {
			t.Errorf("Expected panic error to take precedence, got: %v", result)
		}
	})

	// Test 3: No primary error but close errors exist
	t.Run("NoPrimaryError", func(t *testing.T) {
		exceptionClose := &testCloser{closeErr: errors.New("close error")}

		result := CloseWhileSuppressingExceptions(nil, exceptionClose)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if !strings.Contains(result.Error(), "close error") {
			t.Errorf("Expected result to contain close error, got: %v", result)
		}
	})

	// Test 4: All close successfully with no primary error
	t.Run("AllSuccessNoPrimary", func(t *testing.T) {
		successClose := &testCloser{}

		result := CloseWhileSuppressingExceptions(nil, successClose)

		if result != nil {
			t.Errorf("Expected nil result, got: %v", result)
		}
	})

	// Test 5: Nil closeables are ignored
	t.Run("NilCloseablesIgnored", func(t *testing.T) {
		primaryErr := errors.New("primary error")

		result := CloseWhileSuppressingExceptions(primaryErr, nil, nil)

		// Should return the primary error unchanged
		if result == nil || result.Error() != "primary error" {
			t.Errorf("Expected primary error unchanged, got: %v", result)
		}
	})
}

// TestDeleteFilesIfExist tests deleting files if they exist
// Source: TestIOUtils.testDeleteFileIfExists()
func TestDeleteFilesIfExist(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1")

	// Create the file
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete the file
	if err := DeleteFilesIfExist(file1); err != nil {
		t.Fatalf("DeleteFilesIfExist failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}
}

// TestDeleteFilesIfExist_NonExistent tests deleting non-existent files
// Source: TestIOUtils.testDontDeleteDoesntExist()
func TestDeleteFilesIfExist_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "nonexistent_file")

	// Should not throw exception for non-existent file
	if err := DeleteFilesIfExist(file1); err != nil {
		t.Errorf("DeleteFilesIfExist should not error for non-existent file: %v", err)
	}
}

// TestDeleteFilesIfExist_Mixed tests deleting multiple files where some exist
// Source: TestIOUtils.testDeleteTwoFilesIfExist()
func TestDeleteFilesIfExist_Mixed(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1")
	file2 := filepath.Join(tempDir, "file2")

	// Only create file2
	if err := os.WriteFile(file2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete both files
	if err := DeleteFilesIfExist(file1, file2); err != nil {
		t.Fatalf("DeleteFilesIfExist failed: %v", err)
	}

	// Verify file2 was deleted
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("Expected file2 to be deleted")
	}
}

// TestFSyncDirectory tests fsync on a directory
// Source: TestIOUtils.testFsyncDirectory()
func TestFSyncDirectory(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")

	// Create subdirectory
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// FSync the directory - should not throw
	if err := FSyncDirectory(subDir); err != nil {
		t.Errorf("FSyncDirectory failed: %v", err)
	}
}

// TestFSyncDirectory_NonExistent tests fsync on a non-existent directory
// Source: TestIOUtils.testFsyncNonExistentDirectory()
func TestFSyncDirectory_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "nonexistent")

	// Should throw error for non-existent directory
	err := FSyncDirectory(nonExistentDir)
	if err == nil {
		t.Error("Expected error for non-existent directory")
	}
}

// TestFSync tests fsync on a file
// Source: TestIOUtils.testFsyncFile()
func TestFSync(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "testfile.txt")

	// Create and write to file
	if err := os.WriteFile(testFile, []byte("test data\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Open file for fsync
	f, err := os.OpenFile(testFile, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	// FSync the file - should not throw
	if err := FSync(f); err != nil {
		t.Errorf("FSync failed: %v", err)
	}
}

// TestFSync_NilFile tests fsync with nil file
func TestFSync_NilFile(t *testing.T) {
	// Should not panic with nil file
	if err := FSync(nil); err != nil {
		t.Errorf("FSync with nil file should not error: %v", err)
	}
}

// TestApplyToAll tests applying a function to all items
// Source: TestIOUtils.testApplyToAll()
func TestApplyToAll(t *testing.T) {
	t.Run("AllSuccess", func(t *testing.T) {
		items := []int{1, 2, 3}
		var processed []int

		err := ApplyToAll(items, func(i int) error {
			processed = append(processed, i)
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(processed) != 3 {
			t.Errorf("Expected 3 processed items, got %d", len(processed))
		}
	})

	t.Run("FirstError", func(t *testing.T) {
		items := []int{1, 2}
		var processed []int

		err := ApplyToAll(items, func(i int) error {
			processed = append(processed, i)
			return errors.New("error " + string(rune('0'+i)))
		})

		if err == nil {
			t.Fatal("Expected error")
		}
		// First error should be returned
		if err.Error() != "error 1" {
			t.Errorf("Expected first error, got: %v", err)
		}
		// Both items should have been processed
		if len(processed) != 2 {
			t.Errorf("Expected 2 processed items, got %d", len(processed))
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		items := []int{1, 2, 3}

		err := ApplyToAll(items, func(i int) error {
			return errors.New("error")
		})

		if err == nil {
			t.Fatal("Expected error")
		}
		// Error message should indicate multiple errors
		if !strings.Contains(err.Error(), "errors applying function") {
			t.Errorf("Expected error message about multiple errors, got: %v", err)
		}
	})
}

// TestClose tests the Close function
func TestClose(t *testing.T) {
	t.Run("CloseSuccess", func(t *testing.T) {
		c := &testCloser{}
		Close(c)
		if !c.closed {
			t.Error("Expected closer to be closed")
		}
	})

	t.Run("CloseNil", func(t *testing.T) {
		// Should not panic with nil
		Close(nil)
	})

	t.Run("CloseWithError", func(t *testing.T) {
		c := &testCloser{closeErr: errors.New("close error")}
		// Should not panic even with error
		Close(c)
		if !c.closed {
			t.Error("Expected closer to be closed")
		}
	})
}

// TestCloseWhileHandlingException tests CloseWhileHandlingException
func TestCloseWhileHandlingException(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		c := &testCloser{}
		err := CloseWhileHandlingException(c, "test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !c.closed {
			t.Error("Expected closer to be closed")
		}
	})

	t.Run("NilCloser", func(t *testing.T) {
		err := CloseWhileHandlingException(nil, "test")
		if err != nil {
			t.Errorf("Expected no error for nil closer, got: %v", err)
		}
	})

	t.Run("CloseError", func(t *testing.T) {
		c := &testCloser{closeErr: errors.New("close failed")}
		err := CloseWhileHandlingException(c, "test")
		if err == nil {
			t.Error("Expected error")
		}
		if !strings.Contains(err.Error(), "error closing test") {
			t.Errorf("Expected error message about closing, got: %v", err)
		}
	})
}

// TestCloseAll tests CloseAll function
func TestCloseAll(t *testing.T) {
	t.Run("AllSuccess", func(t *testing.T) {
		c1 := &testCloser{}
		c2 := &testCloser{}

		err := CloseAll(c1, c2)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !c1.closed || !c2.closed {
			t.Error("Expected both closers to be closed")
		}
	})

	t.Run("OneError", func(t *testing.T) {
		c1 := &testCloser{}
		c2 := &testCloser{closeErr: errors.New("close error")}

		err := CloseAll(c1, c2)

		if err == nil {
			t.Error("Expected error")
		}
		if !c1.closed || !c2.closed {
			t.Error("Expected both closers to be closed")
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		c1 := &testCloser{closeErr: errors.New("error1")}
		c2 := &testCloser{closeErr: errors.New("error2")}

		err := CloseAll(c1, c2)

		if err == nil {
			t.Error("Expected error")
		}
		if !strings.Contains(err.Error(), "errors closing resources") {
			t.Errorf("Expected error about closing resources, got: %v", err)
		}
	})

	t.Run("NilCloseables", func(t *testing.T) {
		c1 := &testCloser{}

		err := CloseAll(c1, nil, nil)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !c1.closed {
			t.Error("Expected c1 to be closed")
		}
	})
}

// TestCloseAllWhileHandlingException tests CloseAllWhileHandlingException
func TestCloseAllWhileHandlingException(t *testing.T) {
	c1 := &testCloser{}
	c2 := &testCloser{closeErr: errors.New("close error")}

	// Should not panic even with errors
	CloseAllWhileHandlingException(c1, c2, nil)

	if !c1.closed || !c2.closed {
		t.Error("Expected both closers to be closed")
	}
}

// TestDeleteFiles tests DeleteFiles function
func TestDeleteFiles(t *testing.T) {
	t.Run("DeleteExisting", func(t *testing.T) {
		tempDir := t.TempDir()
		file1 := filepath.Join(tempDir, "file1")

		if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		if err := DeleteFiles(file1); err != nil {
			t.Errorf("DeleteFiles failed: %v", err)
		}

		if _, err := os.Stat(file1); !os.IsNotExist(err) {
			t.Error("Expected file to be deleted")
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		tempDir := t.TempDir()
		file1 := filepath.Join(tempDir, "nonexistent")

		// Should not error for non-existent file
		if err := DeleteFiles(file1); err != nil {
			t.Errorf("DeleteFiles should not error for non-existent file: %v", err)
		}
	})

	t.Run("DeleteMultiple", func(t *testing.T) {
		tempDir := t.TempDir()
		file1 := filepath.Join(tempDir, "file1")
		file2 := filepath.Join(tempDir, "file2")

		if err := os.WriteFile(file1, []byte("test1"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := os.WriteFile(file2, []byte("test2"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		if err := DeleteFiles(file1, file2); err != nil {
			t.Errorf("DeleteFiles failed: %v", err)
		}

		if _, err := os.Stat(file1); !os.IsNotExist(err) {
			t.Error("Expected file1 to be deleted")
		}
		if _, err := os.Stat(file2); !os.IsNotExist(err) {
			t.Error("Expected file2 to be deleted")
		}
	})
}

// TestResourcePool tests the ResourcePool type
func TestResourcePool(t *testing.T) {
	t.Run("AddAndClose", func(t *testing.T) {
		pool := NewResourcePool()
		c1 := &testCloser{}
		c2 := &testCloser{}

		pool.Add(c1)
		pool.Add(c2)

		if pool.Len() != 2 {
			t.Errorf("Expected pool length 2, got %d", pool.Len())
		}

		err := pool.CloseAll()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !c1.closed || !c2.closed {
			t.Error("Expected both resources to be closed")
		}

		if pool.Len() != 0 {
			t.Errorf("Expected pool to be empty after close, got %d", pool.Len())
		}
	})

	t.Run("CloseWithError", func(t *testing.T) {
		pool := NewResourcePool()
		c1 := &testCloser{}
		c2 := &testCloser{closeErr: errors.New("close error")}

		pool.Add(c1)
		pool.Add(c2)

		err := pool.CloseAll()
		if err == nil {
			t.Error("Expected error")
		}

		if !c1.closed || !c2.closed {
			t.Error("Expected both resources to be closed")
		}
	})

	t.Run("CloseAllWhileHandlingException", func(t *testing.T) {
		pool := NewResourcePool()
		c1 := &testCloser{}
		c2 := &testCloser{closeErr: errors.New("close error")}

		pool.Add(c1)
		pool.Add(c2)

		// Should not panic
		pool.CloseAllWhileHandlingException()

		if !c1.closed || !c2.closed {
			t.Error("Expected both resources to be closed")
		}
	})
}

// TestSafeClose tests SafeClose function
func TestSafeClose(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		c := &testCloser{}
		err := SafeClose(c)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !c.closed {
			t.Error("Expected closer to be closed")
		}
	})

	t.Run("Nil", func(t *testing.T) {
		err := SafeClose(nil)
		if err != nil {
			t.Errorf("Expected no error for nil, got: %v", err)
		}
	})

	t.Run("Panic", func(t *testing.T) {
		c := &testCloser{panicVal: "panic!"}
		err := SafeClose(c)
		if err == nil {
			t.Error("Expected error from panic")
		}
		if !strings.Contains(err.Error(), "panic") {
			t.Errorf("Expected panic error message, got: %v", err)
		}
	})
}

// TestEnsureOpen tests EnsureOpen function
func TestEnsureOpen(t *testing.T) {
	t.Run("NotNil", func(t *testing.T) {
		c := &testCloser{}
		// Should not panic
		EnsureOpen(c, "test")
	})

	t.Run("Nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil closer")
			}
		}()
		EnsureOpen(nil, "test")
	})
}

// TestCheckClosed tests CheckClosed function
func TestCheckClosed(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Nil", nil, false},
		{"Closed", errors.New("file is closed"), true},
		{"EOF", errors.New("EOF"), true},
		{"Other", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckClosed(tt.err)
			if result != tt.expected {
				t.Errorf("CheckClosed(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestCloseChan tests CloseChan function
func TestCloseChan(t *testing.T) {
	t.Run("CloseOpenChannel", func(t *testing.T) {
		ch := make(chan int)
		CloseChan(ch)
		// Verify channel is closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Expected channel to be closed")
			}
		default:
			t.Error("Expected channel to be closed and readable")
		}
	})

	t.Run("CloseAlreadyClosed", func(t *testing.T) {
		ch := make(chan int)
		close(ch)
		// Should not panic when closing already closed channel
		CloseChan(ch)
	})
}

// BenchmarkClose benchmarks the Close function
func BenchmarkClose(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := &testCloser{}
		Close(c)
	}
}

// BenchmarkCloseAll benchmarks the CloseAll function
func BenchmarkCloseAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c1 := &testCloser{}
		c2 := &testCloser{}
		c3 := &testCloser{}
		CloseAll(c1, c2, c3)
	}
}

// Ensure testCloser implements io.Closer
var _ io.Closer = (*testCloser)(nil)
