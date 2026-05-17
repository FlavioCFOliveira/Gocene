// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// KnnFieldVectorsWriter is the per-field writer that a KnnVectorsWriter
// hands out one-per-vector-field to accumulate per-document vectors before
// the field is finalized. It is the Go port of
// org.apache.lucene.codecs.KnnFieldVectorsWriter from Apache Lucene 10.4.0.
//
// The Java reference is a generic abstract class parameterized over the
// vector type (Float32 or Byte). Go has no class polymorphism, so the
// generic surface is encoded with a type parameter: callers instantiate a
// KnnFieldVectorsWriter[float32] or KnnFieldVectorsWriter[byte] depending
// on the field's vector encoding. Concrete codec writers implement this
// interface with their own buffering strategy.
//
// Lifecycle (per field):
//  1. The owning KnnVectorsWriter creates a KnnFieldVectorsWriter instance
//     for the field via its own factory.
//  2. AddValue is invoked once per document that has a vector value, in
//     increasing docID order.
//  3. Finish is called once at the end of the segment, before the parent
//     KnnVectorsWriter writes the field to disk.
type KnnFieldVectorsWriter[T any] interface {
	// AddValue records the vector value for the given document. Callers must
	// invoke AddValue in strictly increasing docID order, matching the Java
	// contract that throws on out-of-order calls.
	AddValue(docID int, vectorValue []T) error

	// RAMBytesUsed returns an estimate of the in-memory footprint of all
	// values accumulated so far. The parent KnnVectorsWriter sums this
	// across fields when deciding whether to flush.
	RAMBytesUsed() int64

	// Finish marks the field as complete. Implementations may release
	// per-document scratch buffers or perform a final sort here.
	Finish() error
}
