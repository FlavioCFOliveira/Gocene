// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"testing"
)

func TestNativeFSLockFactory(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new factory",
			fn: func(t *testing.T) {
				factory := NewNativeFSLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}
			},
		},
		{
			name: "obtain lock",
			fn: func(t *testing.T) {
				factory := NewNativeFSLockFactory()
				// Note: In a real implementation, this would need a real directory
				// For testing, we pass nil since the mock doesn't use it
				lock, err := factory.ObtainLock(nil, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if lock == nil {
					t.Fatal("expected non-nil lock")
				}
				if !lock.IsLocked() {
					t.Error("expected lock to be locked")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestNativeFSLock(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new lock is locked",
			fn: func(t *testing.T) {
				lock := &NativeFSLock{
					BaseLock: NewBaseLock(),
					name:     "test.lock",
				}

				if !lock.IsLocked() {
					t.Error("expected new lock to be locked")
				}
			},
		},
		{
			name: "close releases lock",
			fn: func(t *testing.T) {
				lock := &NativeFSLock{
					BaseLock: NewBaseLock(),
					name:     "test.lock",
				}

				if err := lock.Close(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if lock.IsLocked() {
					t.Error("expected lock to be released after close")
				}
			},
		},
		{
			name: "close on released lock returns nil",
			fn: func(t *testing.T) {
				lock := &NativeFSLock{
					BaseLock: NewBaseLock(),
					name:     "test.lock",
				}

				lock.Close()
				err := lock.Close()

				if err != nil {
					t.Errorf("expected nil error on second close, got %v", err)
				}
			},
		},
		{
			name: "ensure valid when locked",
			fn: func(t *testing.T) {
				lock := &NativeFSLock{
					BaseLock: NewBaseLock(),
					name:     "test.lock",
				}

				if err := lock.EnsureValid(); err != nil {
					t.Errorf("expected no error when locked, got %v", err)
				}
			},
		},
		{
			name: "ensure valid when released",
			fn: func(t *testing.T) {
				lock := &NativeFSLock{
					BaseLock: NewBaseLock(),
					name:     "test.lock",
				}

				lock.Close()

				if err := lock.EnsureValid(); err == nil {
					t.Error("expected error when released")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestSingleInstanceLockFactory(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new factory",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}
			},
		},
		{
			name: "obtain lock",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				lock, err := factory.ObtainLock(nil, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if lock == nil {
					t.Fatal("expected non-nil lock")
				}
			},
		},
		{
			name: "obtain same lock twice fails",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				_, err := factory.ObtainLock(nil, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				_, err = factory.ObtainLock(nil, "test.lock")
				if err == nil {
					t.Error("expected error when obtaining same lock twice")
				}
			},
		},
		{
			name: "different lock names succeed",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()

				_, err := factory.ObtainLock(nil, "test1.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				_, err = factory.ObtainLock(nil, "test2.lock")
				if err != nil {
					t.Errorf("unexpected error for different lock name: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestSingleInstanceLock(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "close releases lock",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				lock, _ := factory.ObtainLock(nil, "test.lock")

				if err := lock.Close(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if lock.IsLocked() {
					t.Error("expected lock to be released")
				}
			},
		},
		{
			name: "close twice succeeds",
			fn: func(t *testing.T) {
				factory := NewSingleInstanceLockFactory()
				lock, _ := factory.ObtainLock(nil, "test.lock")

				lock.Close()
				err := lock.Close()

				if err != nil {
					t.Errorf("expected nil error on second close, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestNoLockFactory(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new factory",
			fn: func(t *testing.T) {
				factory := NewNoLockFactory()
				if factory == nil {
					t.Fatal("expected non-nil factory")
				}
			},
		},
		{
			name: "obtain lock returns no-op lock",
			fn: func(t *testing.T) {
				factory := NewNoLockFactory()
				lock, err := factory.ObtainLock(nil, "test.lock")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if lock == nil {
					t.Fatal("expected non-nil lock")
				}
				if lock.IsLocked() {
					t.Error("expected no-op lock to not be locked")
				}
			},
		},
		{
			name: "close no-op lock succeeds",
			fn: func(t *testing.T) {
				factory := NewNoLockFactory()
				lock, _ := factory.ObtainLock(nil, "test.lock")

				if err := lock.Close(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "ensure valid no-op lock succeeds",
			fn: func(t *testing.T) {
				factory := NewNoLockFactory()
				lock, _ := factory.ObtainLock(nil, "test.lock")

				if err := lock.EnsureValid(); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestBaseLock(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new base lock is locked",
			fn: func(t *testing.T) {
				lock := NewBaseLock()
				if !lock.IsLocked() {
					t.Error("expected new lock to be locked")
				}
			},
		},
		{
			name: "mark released",
			fn: func(t *testing.T) {
				lock := NewBaseLock()
				lock.MarkReleased()

				if lock.IsLocked() {
					t.Error("expected lock to be released")
				}
			},
		},
		{
			name: "verify locked returns nil when locked",
			fn: func(t *testing.T) {
				lock := NewBaseLock()
				if err := lock.VerifyLocked(); err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "verify locked returns error when released",
			fn: func(t *testing.T) {
				lock := NewBaseLock()
				lock.MarkReleased()
				if err := lock.VerifyLocked(); err == nil {
					t.Error("expected error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

// TestLockFactoryStress tests lock factory behavior under stress conditions.
// Ported from: org.apache.lucene.store.TestLockFactory
func TestLockFactoryStress(t *testing.T) {
	t.Run("single instance lock factory stress", func(t *testing.T) {
		factory := NewSingleInstanceLockFactory()

		// Obtain and release many locks
		for i := 0; i < 100; i++ {
			lockName := fmt.Sprintf("lock_%d", i)
			lock, err := factory.ObtainLock(nil, lockName)
			if err != nil {
				t.Fatalf("Failed to obtain lock %s: %v", lockName, err)
			}
			if err := lock.Close(); err != nil {
				t.Errorf("Failed to release lock %s: %v", lockName, err)
			}
		}
	})

	t.Run("lock released can be reobtained", func(t *testing.T) {
		factory := NewSingleInstanceLockFactory()

		// Obtain lock
		lock1, err := factory.ObtainLock(nil, "reusable.lock")
		if err != nil {
			t.Fatalf("Failed to obtain lock: %v", err)
		}

		// Release lock
		if err := lock1.Close(); err != nil {
			t.Fatalf("Failed to release lock: %v", err)
		}

		// Should be able to obtain same lock again
		lock2, err := factory.ObtainLock(nil, "reusable.lock")
		if err != nil {
			t.Errorf("Should be able to reobtain released lock: %v", err)
		}
		if lock2 != nil {
			lock2.Close()
		}
	})

	t.Run("multiple different locks", func(t *testing.T) {
		factory := NewSingleInstanceLockFactory()

		// Obtain multiple different locks
		locks := make([]Lock, 10)
		for i := 0; i < 10; i++ {
			lockName := fmt.Sprintf("multi_%d.lock", i)
			lock, err := factory.ObtainLock(nil, lockName)
			if err != nil {
				t.Fatalf("Failed to obtain lock %s: %v", lockName, err)
			}
			locks[i] = lock
		}

		// Verify all are locked
		for i, lock := range locks {
			if !lock.IsLocked() {
				t.Errorf("Lock %d should be locked", i)
			}
		}

		// Release all
		for i, lock := range locks {
			if err := lock.Close(); err != nil {
				t.Errorf("Failed to release lock %d: %v", i, err)
			}
		}
	})
}

// TestLockFactoryWithDirectory tests lock factory integration with directories.
// Ported from: org.apache.lucene.store.TestLockFactory.testDirectoryLocking()
func TestLockFactoryWithDirectory(t *testing.T) {
	t.Run("native fs lock with byte buffers directory", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Set NativeFSLockFactory
		dir.SetLockFactory(NewNativeFSLockFactory())

		// Obtain lock through directory
		lock, err := dir.ObtainLock("test.lock")
		if err != nil {
			t.Fatalf("Failed to obtain lock: %v", err)
		}
		if lock == nil {
			t.Fatal("Expected non-nil lock")
		}
		if !lock.IsLocked() {
			t.Error("Expected lock to be locked")
		}

		// Release lock
		if err := lock.Close(); err != nil {
			t.Errorf("Failed to release lock: %v", err)
		}
	})

	t.Run("single instance lock with byte buffers directory", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()

		// Set SingleInstanceLockFactory
		dir.SetLockFactory(NewSingleInstanceLockFactory())

		// Obtain lock
		lock1, err := dir.ObtainLock("single.lock")
		if err != nil {
			t.Fatalf("Failed to obtain first lock: %v", err)
		}

		// Try to obtain same lock again - should fail
		_, err = dir.ObtainLock("single.lock")
		if err == nil {
			t.Error("Expected error obtaining same lock twice")
		}

		// Release first lock
		lock1.Close()
	})
}

// TestLockValidity tests lock validity checking.
// Ported from: org.apache.lucene.store.TestLockFactory
func TestLockValidity(t *testing.T) {
	t.Run("native fs lock ensure valid", func(t *testing.T) {
		lock := &NativeFSLock{
			BaseLock: NewBaseLock(),
			name:     "validity.lock",
			path:     "", // In-memory lock for testing
		}

		// Should be valid when locked
		if err := lock.EnsureValid(); err != nil {
			t.Errorf("Expected no error when locked: %v", err)
		}

		// Release lock
		lock.Close()

		// Should be invalid after release
		if err := lock.EnsureValid(); err == nil {
			t.Error("Expected error when lock is released")
		}
	})

	t.Run("single instance lock ensure valid", func(t *testing.T) {
		factory := NewSingleInstanceLockFactory()
		lock, _ := factory.ObtainLock(nil, "validity.lock")

		// Should be valid when locked
		if err := lock.EnsureValid(); err != nil {
			t.Errorf("Expected no error when locked: %v", err)
		}

		// Release lock
		lock.Close()

		// Should be invalid after release
		if err := lock.EnsureValid(); err == nil {
			t.Error("Expected error when lock is released")
		}
	})
}
