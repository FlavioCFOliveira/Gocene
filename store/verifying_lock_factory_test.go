// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestNewVerifyingLockFactory(t *testing.T) {
	delegate := NewNativeFSLockFactory()
	factory := NewVerifyingLockFactory(delegate)

	if factory == nil {
		t.Error("expected factory to be created")
	}

	// Initially no locks should be held
	if len(factory.GetHeldLocks()) != 0 {
		t.Error("expected no held locks initially")
	}
}

func TestVerifyingLockFactory_ObtainAndRelease(t *testing.T) {
	dir := NewByteBuffersDirectory()
	delegate := NewNativeFSLockFactory()
	factory := NewVerifyingLockFactory(delegate)

	// Obtain a lock
	lock, err := factory.ObtainLock(dir, "test.lock")
	if err != nil {
		t.Fatalf("failed to obtain lock: %v", err)
	}

	// Check that lock is held
	if !factory.IsLockHeld("test.lock") {
		t.Error("expected lock to be held")
	}

	heldLocks := factory.GetHeldLocks()
	if len(heldLocks) != 1 || heldLocks[0] != "test.lock" {
		t.Errorf("expected ['test.lock'], got %v", heldLocks)
	}

	// Release the lock
	err = lock.Close()
	if err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Check that lock is no longer held
	if factory.IsLockHeld("test.lock") {
		t.Error("expected lock to be released")
	}

	if len(factory.GetHeldLocks()) != 0 {
		t.Error("expected no held locks after release")
	}
}

func TestVerifyingLockFactory_DoubleObtain(t *testing.T) {
	dir := NewByteBuffersDirectory()
	delegate := NewNativeFSLockFactory()
	factory := NewVerifyingLockFactory(delegate)

	// Obtain a lock
	_, err := factory.ObtainLock(dir, "test.lock")
	if err != nil {
		t.Fatalf("failed to obtain lock: %v", err)
	}

	// Try to obtain the same lock again (should fail)
	_, err = factory.ObtainLock(dir, "test.lock")
	if err == nil {
		t.Error("expected error when obtaining already-held lock")
	}
}

func TestVerifyingLockFactory_DoubleRelease(t *testing.T) {
	dir := NewByteBuffersDirectory()
	delegate := NewNativeFSLockFactory()
	factory := NewVerifyingLockFactory(delegate)

	// Obtain a lock
	lock, err := factory.ObtainLock(dir, "test.lock")
	if err != nil {
		t.Fatalf("failed to obtain lock: %v", err)
	}

	// Release the lock
	err = lock.Close()
	if err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Try to release again (should fail)
	err = lock.Close()
	if err == nil {
		t.Error("expected error when releasing already-released lock")
	}
}
