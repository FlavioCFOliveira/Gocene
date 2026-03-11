// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
	"os"
)

// IOUtils provides utility methods for I/O operations.
type IOUtils struct{}

// Close closes the closer and ignores errors.
func Close(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

// CloseWhileHandlingException closes the closer, handling any exception.
func CloseWhileHandlingException(c io.Closer) error {
	if c != nil {
		return c.Close()
	}
	return nil
}

// DeleteFilesIgnoringExceptions deletes files, ignoring any exceptions.
func DeleteFilesIgnoringExceptions(files ...string) {
	for _, file := range files {
		_ = os.Remove(file)
	}
}

// FSync syncs the file to disk.
func FSync(file *os.File) error {
	if file != nil {
		return file.Sync()
	}
	return nil
}

// EnsureClose ensures the closer is closed, even if panics occur.
func EnsureClose(c io.Closer, err *error) {
	if c != nil {
		if closeErr := c.Close(); closeErr != nil && *err == nil {
			*err = closeErr
		}
	}
}
