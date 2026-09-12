// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfo provides information about a segment such as its name,
// directory, and files related to the segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfo.
//
// The canonical declaration lives in package spi (rmp #4706 SPI
// unification); this file re-exports the type via a Go alias plus the
// NO/YES constants Lucene declares on the class, so existing callers
// continue to compile unchanged.
type SegmentInfo = spi.SegmentInfo

const (
	// No mirrors Lucene's SegmentInfo.NO: used by some member fields to
	// mean not present (e.g. no norms, no deletes).
	No = -1

	// Yes mirrors Lucene's SegmentInfo.YES: used by some member fields to
	// mean present (e.g. have norms, have deletes).
	Yes = 1
)

// NewSegmentInfo creates a new SegmentInfo with the given name, maxDoc and
// directory. See spi.NewSegmentInfo for the version and codec defaults.
func NewSegmentInfo(name string, maxDoc int, dir store.Directory) *SegmentInfo {
	return spi.NewSegmentInfo(name, maxDoc, dir)
}
