// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "fmt"

// LockObtainFailedException is thrown when the write.lock could not be
// acquired. This typically happens when a writer tries to open an index that
// another writer already has open.
//
// This is the Go port of org.apache.lucene.store.LockObtainFailedException
// from Apache Lucene 10.4.0. In Java it extends IOException; the Go form
// satisfies the error interface and supports errors.Is / errors.As.
type LockObtainFailedException struct {
	Message string
	Cause   error
}

// NewLockObtainFailedException constructs a LockObtainFailedException with
// the given message and optional cause.
func NewLockObtainFailedException(message string, cause error) *LockObtainFailedException {
	return &LockObtainFailedException{Message: message, Cause: cause}
}

// Error implements the error interface.
func (e *LockObtainFailedException) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the wrapped cause, enabling errors.Is and errors.As.
func (e *LockObtainFailedException) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
