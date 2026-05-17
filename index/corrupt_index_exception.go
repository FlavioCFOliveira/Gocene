// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// CorruptIndexException is returned when Lucene detects an inconsistency in
// the index. It mirrors org.apache.lucene.index.CorruptIndexException from
// Apache Lucene 10.4.0.
//
// The Error() string follows the Lucene format:
//
//	"<message> (resource=<resourceDescription>)"
//
// optionally suffixed with the wrapped cause via standard error wrapping.
type CorruptIndexException struct {
	message             string
	resourceDescription string
	cause               error
}

// NewCorruptIndexException constructs a CorruptIndexException with a message
// and a textual description of the corrupted resource (file, directory, etc.).
func NewCorruptIndexException(message, resourceDescription string) *CorruptIndexException {
	return &CorruptIndexException{
		message:             message,
		resourceDescription: resourceDescription,
	}
}

// NewCorruptIndexExceptionWithCause constructs a CorruptIndexException with a
// message, resource description, and a root cause.
func NewCorruptIndexExceptionWithCause(message, resourceDescription string, cause error) *CorruptIndexException {
	return &CorruptIndexException{
		message:             message,
		resourceDescription: resourceDescription,
		cause:               cause,
	}
}

// Error returns the formatted error message:
// "<message> (resource=<resourceDescription>)". When a cause is set, the
// cause is appended via ": <cause>" to match Java's Throwable.toString
// behavior closely while remaining idiomatic Go.
func (e *CorruptIndexException) Error() string {
	base := fmt.Sprintf("%s (resource=%s)", e.message, e.resourceDescription)
	if e.cause != nil {
		return base + ": " + e.cause.Error()
	}
	return base
}

// Unwrap returns the wrapped cause, enabling errors.Is/As traversal.
func (e *CorruptIndexException) Unwrap() error {
	return e.cause
}

// GetResourceDescription returns the description of the file that was
// corrupted. Matches Lucene's CorruptIndexException#getResourceDescription.
func (e *CorruptIndexException) GetResourceDescription() string {
	return e.resourceDescription
}

// GetOriginalMessage returns the original exception message without the
// corrupted file description. Matches Lucene's getOriginalMessage.
func (e *CorruptIndexException) GetOriginalMessage() string {
	return e.message
}
