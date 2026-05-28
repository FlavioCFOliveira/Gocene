// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// FieldInfosFormat encodes and decodes the .fnm field-metadata file of
// a segment.
//
// The signatures carry a segmentSuffix parameter so that PerField
// formats can disambiguate sub-formats that share the same segment
// directory; this matches the Apache Lucene 10.4.0
// org.apache.lucene.codecs.FieldInfosFormat surface.
type FieldInfosFormat interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// Read deserialises the .fnm file for segmentInfo, optionally
	// qualified by segmentSuffix.
	Read(dir store.Directory, segmentInfo *schema.SegmentInfo, segmentSuffix string, context store.IOContext) (*schema.FieldInfos, error)

	// Write serialises the .fnm file for segmentInfo, optionally
	// qualified by segmentSuffix.
	Write(dir store.Directory, segmentInfo *schema.SegmentInfo, segmentSuffix string, infos *schema.FieldInfos, context store.IOContext) error
}
