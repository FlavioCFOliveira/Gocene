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
//     segments_N), TermVectorsFormat, CompoundFormat, KnnVectorsFormat,
//     DocValuesFormat.
//
// DocValuesFormat joined the SPI in rmp #4708, completing the lift of
// every per-component format accessor onto a single SPI surface. The
// index-side value-type projection (random-access Get(docID) shape)
// stays out of the SPI and is tracked separately as rmp #4709; that
// divergence does not affect this Codec interface because the format
// accessor returns the canonical iterator-shaped surface used by the
// codecs side.
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

	// DocValuesFormat returns the format used for per-document column-
	// stride values (.dvd / .dvm). Lifted onto the SPI by rmp #4708
	// once the DocValuesFormat / DocValuesProducer / DocValuesConsumer
	// family plus their value-type and iterator contracts moved into
	// package spi. Codec implementations that do not support doc values
	// may return nil.
	DocValuesFormat() DocValuesFormat
}
