// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CompoundFormat packs the per-segment files of a segment into a
// single .cfs / .cfe compound-file pair, reducing file-handle pressure
// when many segments are open.
//
// Mirrors org.apache.lucene.codecs.CompoundFormat in Apache Lucene
// 10.4.0.
type CompoundFormat interface {
	// Write packs the files listed in si.Files() into a compound file
	// pair in dir.
	Write(dir store.Directory, si *schema.SegmentInfo, ctx store.IOContext) error

	// GetCompoundReader returns a read-only Directory view of the .cfs
	// compound file for the given segment.
	GetCompoundReader(dir store.Directory, si *schema.SegmentInfo) (CompoundDirectory, error)
}

// CompoundDirectory is a read-only Directory view of a compound file.
// It extends store.Directory with a checksum-validation hook.
//
// Mirrors org.apache.lucene.codecs.CompoundDirectory.
type CompoundDirectory interface {
	store.Directory

	// CheckIntegrity validates the checksums of every file in the
	// compound directory.
	CheckIntegrity() error
}
