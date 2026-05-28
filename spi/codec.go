// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// Codec aggregates the per-component formats that together describe a
// segment's on-disk encoding.
//
// Mirrors org.apache.lucene.codecs.Codec in Apache Lucene 10.4.0.
//
// # Currently unified members
//
//   - Name, PostingsFormat, StoredFieldsFormat, FieldInfosFormat,
//     SegmentInfoFormat (singular .si), SegmentInfosFormat (plural
//     segments_N), TermVectorsFormat, CompoundFormat, KnnVectorsFormat.
//
// # Intentionally NOT yet on this surface
//
//   - DocValuesFormat: deferred to rmp #4708. The codecs-side family
//     drags in DocValuesProducer/Consumer plus a large web of
//     value-type and iterator interfaces that live only in index/
//     today.
//
// DocValuesFormat stays declared on the codecs.Codec surface with a
// TODO(T4708) marker until its companion task lands.
type Codec interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// PostingsFormat returns the format used for term -> document
	// postings.
	PostingsFormat() PostingsFormat

	// StoredFieldsFormat returns the format used for the per-document
	// stored fields (.fdt / .fdx).
	StoredFieldsFormat() StoredFieldsFormat

	// FieldInfosFormat returns the format used for field metadata
	// (.fnm).
	FieldInfosFormat() FieldInfosFormat

	// SegmentInfoFormat returns the format used for the per-segment
	// metadata file (.si).
	SegmentInfoFormat() SegmentInfoFormat

	// SegmentInfosFormat returns the format used for the plural
	// segments_N file. Lifted onto the SPI by rmp #4706 once
	// *SegmentInfos and *SegmentCommitInfo moved into package spi.
	SegmentInfosFormat() SegmentInfosFormat

	// TermVectorsFormat returns the format used for per-document term
	// vectors.
	TermVectorsFormat() TermVectorsFormat

	// CompoundFormat returns the format used to pack per-segment files
	// into a .cfs / .cfe compound pair, or nil when the codec does not
	// support compound files.
	CompoundFormat() CompoundFormat

	// KnnVectorsFormat returns the format used for K-Nearest Neighbors
	// vector encoding (.vex / .vem and per-format sidecars). Lifted onto
	// the SPI by rmp #4707 once the wide KnnVectorsWriter contract moved
	// into package spi. Codec implementations that do not support KNN
	// vectors may return nil.
	KnnVectorsFormat() KnnVectorsFormat
}
