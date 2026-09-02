// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// AlreadyClosedException is thrown when an operation is attempted on a closed resource.
// This is the Go port of Lucene's org.apache.lucene.store.AlreadyClosedException.
type AlreadyClosedException struct {
	Message string
	Cause   error
}

// Error returns the error message.
func (e *AlreadyClosedException) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying cause.
func (e *AlreadyClosedException) Unwrap() error {
	return e.Cause
}

// NewAlreadyClosedException creates a new AlreadyClosedException.
func NewAlreadyClosedException(message string, cause error) *AlreadyClosedException {
	return &AlreadyClosedException{
		Message: message,
		Cause:   cause,
	}
}
