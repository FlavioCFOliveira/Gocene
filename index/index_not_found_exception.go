// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// IndexNotFoundException is an alias of spi.IndexNotFoundException.
// The canonical declaration moved to package spi in rmp #4706 so that the
// lifted *spi.SegmentInfos can raise it without an index-side dependency.
type IndexNotFoundException = spi.IndexNotFoundException

// NewIndexNotFoundException is a thin re-export of spi.NewIndexNotFoundException.
func NewIndexNotFoundException(message string, cause error) *IndexNotFoundException {
	return spi.NewIndexNotFoundException(message, cause)
}

// IndexNotFoundExceptionFromMessage is a thin re-export of
// spi.IndexNotFoundExceptionFromMessage.
func IndexNotFoundExceptionFromMessage(msg string) *IndexNotFoundException {
	return spi.IndexNotFoundExceptionFromMessage(msg)
}

// IsIndexNotFound is a thin re-export of spi.IsIndexNotFound.
func IsIndexNotFound(err error) bool {
	return spi.IsIndexNotFound(err)
}
