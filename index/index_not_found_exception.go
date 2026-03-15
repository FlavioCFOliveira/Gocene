// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// IndexNotFoundException is thrown when an index cannot be found in a directory.
// This is the Go port of Lucene's org.apache.lucene.index.IndexNotFoundException.
type IndexNotFoundException struct {
	Message string
	Cause   error
}

// Error returns the error message.
func (e *IndexNotFoundException) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying cause.
func (e *IndexNotFoundException) Unwrap() error {
	return e.Cause
}

// NewIndexNotFoundException creates a new IndexNotFoundException.
func NewIndexNotFoundException(message string, cause error) *IndexNotFoundException {
	return &IndexNotFoundException{
		Message: message,
		Cause:   cause,
	}
}
