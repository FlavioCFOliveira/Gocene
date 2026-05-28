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
//     SegmentInfoFormat (singular), TermVectorsFormat, CompoundFormat.
//
// # Intentionally NOT yet on this surface
//
//   - SegmentInfosFormat (plural / segments_N): deferred to rmp #4706.
//     The codecs-side signature elides IOContext on Read/Write while the
//     index-side signature carries it; unifying requires changing every
//     IndexWriter / DirectoryReader call site and is out of scope for
//     rmp #4693.
//   - KnnVectorsFormat: deferred to rmp #4707. The codecs-side
//     interface and the index-side KnnVectorsFormatFactory abstraction
//     need reconciliation first.
//   - DocValuesFormat: deferred to rmp #4708. The codecs-side family
//     drags in DocValuesProducer/Consumer plus a large web of
//     value-type and iterator interfaces that live only in index/
//     today.
//
// Each deferred member stays declared on the legacy index.Codec /
// codecs.Codec surfaces with TODO(T46XX) markers until its companion
// task lands.
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
	// metadata file (.si). The plural SegmentInfosFormat (segments_N)
	// is not yet part of this surface — see the deferral note above.
	SegmentInfoFormat() SegmentInfoFormat

	// TermVectorsFormat returns the format used for per-document term
	// vectors.
	TermVectorsFormat() TermVectorsFormat

	// CompoundFormat returns the format used to pack per-segment files
	// into a .cfs / .cfe compound pair, or nil when the codec does not
	// support compound files.
	CompoundFormat() CompoundFormat
}
