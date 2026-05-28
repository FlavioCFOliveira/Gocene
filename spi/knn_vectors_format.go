// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// KnnVectorsFormat is the canonical wide service-provider interface for
// encoding and decoding per-segment KNN (K-Nearest Neighbors) vector
// values. Mirrors org.apache.lucene.codecs.KnnVectorsFormat from Apache
// Lucene 10.4.0.
//
// The format constructs a [KnnVectorsWriter] for the write path and a
// [KnnVectorsReader] for the read path; both sides operate on the shared
// [SegmentWriteState] / [SegmentReadState] structs.
//
// Lifted into the SPI by rmp #4707 so spi.Codec can expose
// KnnVectorsFormat() as part of its canonical method set. Before the lift,
// the index/ package carried a narrow KnnVectorsFormatFactory wrapper that
// returned a reduced "consumer" writer; that narrow surface is gone and
// callers consume the wide writer directly.
type KnnVectorsFormat interface {
	// Name returns the format name used to look up the format on the read
	// path. Names match the Lucene wire identifiers (e.g.
	// "Lucene99HnswVectorsFormat").
	Name() string

	// FieldsWriter constructs the per-segment KnnVectorsWriter for the
	// supplied write state. Ownership transfers to the caller, which is
	// responsible for invoking Finish/Close.
	FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error)

	// FieldsReader constructs the per-segment KnnVectorsReader for the
	// supplied read state. Ownership transfers to the caller, which is
	// responsible for invoking Close.
	FieldsReader(state *SegmentReadState) (KnnVectorsReader, error)
}

// KnnVectorsWriter is the wide canonical write-side contract a codec
// exposes for KNN vectors. Mirrors org.apache.lucene.codecs.KnnVectorsWriter
// from Apache Lucene 10.4.0.
//
// Lifecycle (per segment):
//
//  1. The owning KnnVectorsFormat constructs a writer via FieldsWriter.
//  2. AddField is invoked once per vector field; the returned
//     [KnnFieldVectorsWriter] accumulates per-document vector values.
//  3. Flush serialises every per-field buffer in one shot once all
//     documents have been observed.
//  4. Finish writes trailing metadata (sentinel, footer).
//  5. Close releases the underlying outputs.
//
// WriteField is the merge-side counterpart of AddField: it consumes
// vectors from a pre-existing reader rather than from a buffer populated
// document-by-document. Implementations that do not support merge through
// a single reader may return an error from WriteField.
//
// The Accountable surface (RamBytesUsed) lets the indexing chain track the
// writer's in-memory footprint as it accumulates vectors.
type KnnVectorsWriter interface {
	// RamBytesUsed reports the in-memory footprint of every per-field
	// buffer held by the writer.
	RamBytesUsed() int64

	// AddField registers a new vector field for indexing and returns the
	// per-field writer the indexing chain feeds vector values into.
	//
	// The returned value is the wide [KnnFieldVectorsWriter] — codec
	// implementations typically back it with a strongly-typed (FLOAT32 or
	// BYTE) sub-writer; the non-generic interface mirrors Java's
	// KnnFieldVectorsWriter<?> wildcard.
	AddField(fieldInfo *schema.FieldInfo) (KnnFieldVectorsWriter, error)

	// Flush serialises every buffered field for maxDoc documents,
	// optionally remapping doc IDs through sortMap when the segment is
	// index-sorted. sortMap is nil when no sort is active.
	Flush(maxDoc int, sortMap SorterDocMap) error

	// WriteField is the single-reader merge entrypoint: vectors for
	// fieldInfo are streamed in from reader instead of from per-document
	// AddValue calls. Implementations that buffer vectors in memory
	// typically return an error here and rely on a separate merge path.
	WriteField(fieldInfo *schema.FieldInfo, reader KnnVectorsReader) error

	// Finish is invoked once after Flush (or the last WriteField) to
	// stamp any trailing metadata (sentinel, footer).
	Finish() error

	// Close releases the underlying outputs. Always invoked after Flush
	// or Finish, including on the abort path.
	Close() error
}

// KnnVectorsReader is the wide canonical read-side contract a codec
// exposes for KNN vectors. Mirrors
// org.apache.lucene.codecs.KnnVectorsReader from Apache Lucene 10.4.0.
//
// Only the integrity-check and close hooks are part of the SPI surface;
// the per-encoding read methods (getFloatVectorValues, getByteVectorValues,
// search, …) live on the codecs-side wider interface because they
// reference iterator types that have not yet been lifted into the SPI.
type KnnVectorsReader interface {
	// CheckIntegrity verifies the integrity of the on-disk vector data.
	CheckIntegrity() error

	// Close releases the underlying inputs. Idempotent.
	Close() error
}

// KnnFieldVectorsWriter is the wide, non-generic per-field write-side
// contract returned by [KnnVectorsWriter.AddField]. Mirrors the
// wildcard-erased view of org.apache.lucene.codecs.KnnFieldVectorsWriter<?>
// from Apache Lucene 10.4.0.
//
// Concrete codec implementations typically back this interface with a
// strongly-typed (FLOAT32 or BYTE) sub-writer; this non-generic surface
// is what the indexing chain consumes, where the vector value flows
// through as a typed []float32 or []byte boxed in an any. Implementations
// dispatch on the field's declared VectorEncoding to coerce the value to
// the right element type.
//
// Lifecycle (per field):
//
//  1. The owning KnnVectorsWriter creates a per-field writer via AddField.
//  2. AddValue is invoked once per document that has a vector value, in
//     increasing docID order. The Lucene reference rejects non-monotonic
//     docID input; implementations match that contract.
//  3. Finish is called once at the end of the segment, before the parent
//     KnnVectorsWriter writes the field to disk.
//
// The Accountable surface (RamBytesUsed) lets the parent writer roll up
// per-field memory pressure for the indexing chain.
type KnnFieldVectorsWriter interface {
	// AddValue records the vector value for the given document. The
	// value's concrete type matches the field's VectorEncoding:
	// []float32 for FLOAT32, []byte for BYTE. Implementations return an
	// error when the encoding does not match.
	AddValue(docID int, vectorValue any) error

	// RamBytesUsed reports the per-field in-memory footprint.
	RamBytesUsed() int64

	// Finish marks the field as complete. Implementations may release
	// per-document scratch buffers or perform a final sort here.
	Finish() error
}
