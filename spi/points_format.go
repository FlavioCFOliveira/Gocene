// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// PointsFormat is the canonical service-provider interface for encoding
// and decoding per-segment point (BKD) values. Mirrors
// org.apache.lucene.codecs.PointsFormat from Apache Lucene 10.4.0.
//
// The format constructs a [PointsWriter] for the write path and a
// [PointsReader] for the read path; both sides operate on the shared
// [SegmentWriteState] / [SegmentReadState] structs.
//
// Lifted into the SPI by rmp #4769 so spi.Codec can expose PointsFormat()
// as part of its canonical method set, mirroring the KnnVectorsFormat lift
// (rmp #4707) and the DocValuesFormat lift (rmp #4708). Before the lift the
// codecs/ package carried its own standalone PointsFormat interface; that
// interface is now an alias of this type.
type PointsFormat interface {
	// Name returns the format name used to look up the format on the read
	// path. Names match the Lucene wire identifiers (e.g.
	// "Lucene90PointsFormat").
	Name() string

	// FieldsWriter constructs the per-segment PointsWriter for the supplied
	// write state. Ownership transfers to the caller, which is responsible
	// for invoking Finish/Close.
	FieldsWriter(state *SegmentWriteState) (PointsWriter, error)

	// FieldsReader constructs the per-segment PointsReader for the supplied
	// read state. Ownership transfers to the caller, which is responsible
	// for invoking Close.
	FieldsReader(state *SegmentReadState) (PointsReader, error)
}

// PointsWriter is the canonical write-side contract a codec exposes for
// point values. Mirrors org.apache.lucene.codecs.PointsWriter from Apache
// Lucene 10.4.0.
//
// Lifecycle (per segment): the indexing chain invokes WriteField once per
// point field (the values are pulled from the supplied PointsReader), then
// Finish stamps the trailing metadata, then Close releases the outputs.
type PointsWriter interface {
	// WriteField writes the point values for fieldInfo, pulling them from
	// reader. The reader exposes the field's PointValues via its concrete
	// GetValues accessor (the wide read surface lives on the codecs side;
	// the SPI keeps only the integrity/close hooks here for the same reason
	// KnnVectorsReader does).
	WriteField(fieldInfo *schema.FieldInfo, reader PointsReader) error

	// Finish finalises the writing process (sentinel, lengths, footer).
	Finish() error

	// Close releases the underlying outputs. Always invoked after Finish,
	// including on the abort path.
	Close() error
}

// PointsReader is the canonical read-side contract a codec exposes for
// point values. Mirrors org.apache.lucene.codecs.PointsReader from Apache
// Lucene 10.4.0.
//
// Only the integrity-check and close hooks are part of the SPI surface; the
// per-field getValues method lives on the codecs-side wider interface
// because it references the PointValues / PointTree iterator types that
// have not yet been lifted into the SPI (the same divergence carried by
// KnnVectorsReader).
type PointsReader interface {
	// CheckIntegrity verifies the integrity of the on-disk point data.
	CheckIntegrity() error

	// Close releases the underlying inputs. Idempotent.
	Close() error
}
