// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// IndexFormatTooNewException signals that the on-disk index version is
// higher than what this Lucene release supports. It mirrors
// org.apache.lucene.index.IndexFormatTooNewException from Apache Lucene 10.4.0.
type IndexFormatTooNewException struct {
	resourceDescription string
	version             int
	minVersion          int
	maxVersion          int
}

// NewIndexFormatTooNewException constructs an IndexFormatTooNewException with
// the corrupted resource description, the encountered version, and the
// supported [minVersion, maxVersion] range.
func NewIndexFormatTooNewException(resourceDescription string, version, minVersion, maxVersion int) *IndexFormatTooNewException {
	return &IndexFormatTooNewException{
		resourceDescription: resourceDescription,
		version:             version,
		minVersion:          minVersion,
		maxVersion:          maxVersion,
	}
}

// Error returns the formatted Lucene-compatible error message.
func (e *IndexFormatTooNewException) Error() string {
	return fmt.Sprintf(
		"Format version is not supported (resource %s): %d (needs to be between %d and %d)",
		e.resourceDescription, e.version, e.minVersion, e.maxVersion,
	)
}

// GetResourceDescription returns the corrupted resource description.
func (e *IndexFormatTooNewException) GetResourceDescription() string { return e.resourceDescription }

// GetVersion returns the version that was found on disk.
func (e *IndexFormatTooNewException) GetVersion() int { return e.version }

// GetMinVersion returns the minimum supported version.
func (e *IndexFormatTooNewException) GetMinVersion() int { return e.minVersion }

// GetMaxVersion returns the maximum supported version.
func (e *IndexFormatTooNewException) GetMaxVersion() int { return e.maxVersion }
