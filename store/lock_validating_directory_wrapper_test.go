// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/* (no dedicated peer
// in Lucene 10.4.0; behaviour is exercised indirectly via IndexWriter tests.
// Tests below cover the documented contract: EnsureValid runs before every
// destructive op and propagates errors).

package store

import (
	"errors"
	"testing"
)

// invalidatingLock is a Lock that fails EnsureValid on demand for tests.
type invalidatingLock struct {
	*BaseLock
	invalid bool
}

func (l *invalidatingLock) Close() error { l.MarkReleased(); return nil }

func (l *invalidatingLock) EnsureValid() error {
	if l.invalid {
		return errors.New("lock invalidated")
	}
	return nil
}

func TestLockValidatingDirectoryWrapper_BlocksDeleteWhenInvalid(t *testing.T) {
	inner := NewByteBuffersDirectory()
	out, err := inner.CreateOutput("foo", IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	_ = out.Close()
	lock := &invalidatingLock{BaseLock: NewBaseLock()}
	w := NewLockValidatingDirectoryWrapper(inner, lock)
	if err := w.DeleteFile("foo"); err != nil {
		t.Fatalf("DeleteFile with valid lock should succeed: %v", err)
	}
	if _, err := inner.OpenInput("foo", IOContextDefault); err == nil {
		t.Fatalf("file should have been deleted")
	}

	// Now invalidate and ensure a second destructive op is rejected.
	lock.invalid = true
	if err := w.DeleteFile("bar"); err == nil {
		t.Fatalf("DeleteFile with invalid lock should fail")
	}
}

func TestLockValidatingDirectoryWrapper_BlocksCreateOutputWhenInvalid(t *testing.T) {
	inner := NewByteBuffersDirectory()
	lock := &invalidatingLock{BaseLock: NewBaseLock(), invalid: true}
	w := NewLockValidatingDirectoryWrapper(inner, lock)
	if _, err := w.CreateOutput("foo", IOContextDefault); err == nil {
		t.Fatalf("CreateOutput with invalid lock should fail")
	}
}
