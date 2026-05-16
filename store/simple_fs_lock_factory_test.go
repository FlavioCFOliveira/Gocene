// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestSimpleFSLockFactory.java
// (Gocene port covers the contractually observable behaviour: create-exclusive,
// EnsureValid rejection after Close, removal on Close.)

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newFSDirForTest(t *testing.T) *FSDirectory {
	t.Helper()
	dir, err := NewFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	return dir
}

func TestSimpleFSLockFactory_ObtainAndClose(t *testing.T) {
	dir := newFSDirForTest(t)
	defer dir.Close()
	factory := NewSimpleFSLockFactory()
	lock, err := factory.ObtainLock(dir, "write.lock")
	if err != nil {
		t.Fatalf("ObtainLock: %v", err)
	}
	if !lock.IsLocked() {
		t.Fatalf("IsLocked should be true after obtain")
	}
	if err := lock.EnsureValid(); err != nil {
		t.Fatalf("EnsureValid: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if lock.IsLocked() {
		t.Fatalf("IsLocked should be false after Close")
	}
	// Lock file should be gone.
	if _, err := os.Stat(filepath.Join(dir.GetPath(), "write.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed, stat err = %v", err)
	}
}

func TestSimpleFSLockFactory_DoubleObtainFails(t *testing.T) {
	dir := newFSDirForTest(t)
	defer dir.Close()
	factory := NewSimpleFSLockFactory()
	first, err := factory.ObtainLock(dir, "write.lock")
	if err != nil {
		t.Fatalf("first ObtainLock: %v", err)
	}
	defer first.Close()
	_, err = factory.ObtainLock(dir, "write.lock")
	if err == nil {
		t.Fatalf("second ObtainLock should fail")
	}
	var lofe *LockObtainFailedException
	if !errors.As(err, &lofe) {
		t.Fatalf("expected LockObtainFailedException, got %T (%v)", err, err)
	}
}

func TestSimpleFSLockFactory_EnsureValidAfterClose(t *testing.T) {
	dir := newFSDirForTest(t)
	defer dir.Close()
	factory := NewSimpleFSLockFactory()
	lock, err := factory.ObtainLock(dir, "write.lock")
	if err != nil {
		t.Fatalf("ObtainLock: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := lock.EnsureValid(); err == nil {
		t.Fatalf("EnsureValid should fail after Close")
	}
}

func TestSimpleFSLockFactory_RejectsNonFSDirectory(t *testing.T) {
	factory := NewSimpleFSLockFactory()
	mem := NewByteBuffersDirectory()
	_, err := factory.ObtainLock(mem, "write.lock")
	if err == nil {
		t.Fatalf("expected error when passing non-FS directory")
	}
}
