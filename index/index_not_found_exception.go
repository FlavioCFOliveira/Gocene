// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// IndexNotFoundException signals that no index was found in the Directory.
// May indicate that the directory is empty, or that an index corruption has
// hidden the segments file. Mirrors
// org.apache.lucene.index.IndexNotFoundException from Apache Lucene 10.4.0,
// which extends FileNotFoundException in Java.
//
// Gocene preserves the (message, cause) constructor used internally by the
// existing code (segment_infos.go), while also exposing the Lucene-canonical
// single-argument constructor IndexNotFoundExceptionFromMessage.
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

// NewIndexNotFoundException creates a new IndexNotFoundException carrying an
// optional cause. The cause is appended via standard error wrapping.
func NewIndexNotFoundException(message string, cause error) *IndexNotFoundException {
	return &IndexNotFoundException{
		Message: message,
		Cause:   cause,
	}
}

// IndexNotFoundExceptionFromMessage matches the Java
// IndexNotFoundException(String msg) constructor for parity with Lucene code
// that uses only the descriptive message.
func IndexNotFoundExceptionFromMessage(msg string) *IndexNotFoundException {
	return &IndexNotFoundException{Message: msg}
}
