// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexFormatTooOldException is returned when Lucene detects an index that
// is too old for this Lucene version. It mirrors
// org.apache.lucene.index.IndexFormatTooOldException from Apache Lucene 10.4.0.
//
// The exception has two flavours, matching the Java original:
//   - "reason" form: only a textual reason is known (use NewIndexFormatTooOldExceptionReason);
//   - "version" form: numeric version + supported [min, max] window is known.
//
// version, minVersion, maxVersion are nullable in Java; we model them with
// pointers so callers can distinguish "not set" from "zero".
type IndexFormatTooOldException struct {
	resourceDescription string
	reason              string
	version             *int
	minVersion          *int
	maxVersion          *int
}

// NewIndexFormatTooOldExceptionReason builds the reason-form variant of the
// exception (no numeric version is available).
func NewIndexFormatTooOldExceptionReason(resourceDescription, reason string) *IndexFormatTooOldException {
	return &IndexFormatTooOldException{
		resourceDescription: resourceDescription,
		reason:              reason,
	}
}

// NewIndexFormatTooOldException builds the version-form variant of the
// exception, carrying the offending version and the supported window.
func NewIndexFormatTooOldException(resourceDescription string, version, minVersion, maxVersion int) *IndexFormatTooOldException {
	v, mn, mx := version, minVersion, maxVersion
	return &IndexFormatTooOldException{
		resourceDescription: resourceDescription,
		version:             &v,
		minVersion:          &mn,
		maxVersion:          &mx,
	}
}

// Error returns the formatted Lucene-compatible message. The text matches
// Java's IndexFormatTooOldException byte-for-byte for both variants.
func (e *IndexFormatTooOldException) Error() string {
	if e.version != nil {
		return fmt.Sprintf(
			"Format version is not supported (resource %s): %d (needs to be between %d and %d). This version of Lucene only supports indexes created with release %d.0 and later.",
			e.resourceDescription, *e.version, *e.minVersion, *e.maxVersion, util.MinSupportedMajor,
		)
	}
	return fmt.Sprintf(
		"Format version is not supported (resource %s): %s. This version of Lucene only supports indexes created with release %d.0 and later by default.",
		e.resourceDescription, e.reason, util.MinSupportedMajor,
	)
}

// GetResourceDescription returns the corrupted resource description.
func (e *IndexFormatTooOldException) GetResourceDescription() string { return e.resourceDescription }

// GetReason returns the textual reason, or "" if the version-form variant was
// constructed.
func (e *IndexFormatTooOldException) GetReason() string { return e.reason }

// GetVersion returns the on-disk version, or nil if the reason-form variant
// was constructed.
func (e *IndexFormatTooOldException) GetVersion() *int { return e.version }

// GetMinVersion returns the minimum supported version, or nil if the
// reason-form variant was constructed.
func (e *IndexFormatTooOldException) GetMinVersion() *int { return e.minVersion }

// GetMaxVersion returns the maximum supported version, or nil if the
// reason-form variant was constructed.
func (e *IndexFormatTooOldException) GetMaxVersion() *int { return e.maxVersion }
