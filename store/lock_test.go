// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
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
