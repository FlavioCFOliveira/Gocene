// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "fmt"

// LockReleaseFailedException is thrown when a write.lock could not be
// released (for example because the lock file could not be safely deleted).
//
// This is the Go port of org.apache.lucene.store.LockReleaseFailedException
// from Apache Lucene 10.4.0. In Java it extends IOException; the Go form
// satisfies the error interface and supports errors.Is / errors.As.
type LockReleaseFailedException struct {
	Message string
	Cause   error
}

// NewLockReleaseFailedException constructs a LockReleaseFailedException with
// the given message and optional cause.
func NewLockReleaseFailedException(message string, cause error) *LockReleaseFailedException {
	return &LockReleaseFailedException{Message: message, Cause: cause}
}

// Error implements the error interface.
func (e *LockReleaseFailedException) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the wrapped cause, enabling errors.Is and errors.As.
func (e *LockReleaseFailedException) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
