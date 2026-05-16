// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "fmt"

// AlreadyClosedException is thrown when there is an attempt to access something
// that has already been closed.
//
// This is the Go port of org.apache.lucene.store.AlreadyClosedException from
// Apache Lucene 10.4.0. In Java this extends IllegalStateException; Go has no
// equivalent base type, so AlreadyClosedException is modelled as a value type
// that satisfies the error interface. Callers that need to distinguish it can
// use errors.As or compare with errors.Is against a sentinel value.
type AlreadyClosedException struct {
	Message string
	Cause   error
}

// NewAlreadyClosedException constructs an AlreadyClosedException with the given
// message. The cause argument may be nil.
func NewAlreadyClosedException(message string, cause error) *AlreadyClosedException {
	return &AlreadyClosedException{Message: message, Cause: cause}
}

// Error implements the error interface.
func (e *AlreadyClosedException) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the wrapped cause, enabling use with errors.Is and errors.As.
func (e *AlreadyClosedException) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
