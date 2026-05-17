// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package hnsw

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// FlatFieldVectorsWriter is the Go port of
// org.apache.lucene.codecs.hnsw.FlatFieldVectorsWriter<T> (Lucene 10.4.0).
// It is the per-field writer surface that a concrete
// [FlatVectorsWriter] returns from [FlatVectorsWriter.AddField]; the
// caller accumulates per-document vectors through the inherited
// AddValue (declared on [codecs.KnnFieldVectorsWriter]) and the
// FlatField-specific accessors defined here.
//
// The Java reference is a generic abstract class parameterized over
// the vector element type (Float or Byte). Gocene preserves the
// generic surface via the Go type parameter T (float32 | byte).
// Concrete subclasses live in codec-specific packages (e.g. the
// Lucene99FlatVectorsWriter port to land in a later sprint) and embed
// [codecs.KnnFieldVectorsWriter[T]] plus the additional surface
// declared here.
//
// Lifecycle (per field, single-threaded):
//  1. The owning FlatVectorsWriter creates an instance via its AddField
//     factory.
//  2. AddValue (inherited from KnnFieldVectorsWriter[T]) is invoked
//     once per document with the field's vector, in strictly
//     increasing docID order.
//  3. Finish is called once at the end of the segment, marking the
//     writer as immutable; subsequent AddValue calls must return an
//     error (the Java reference throws).
//  4. GetVectors and GetDocsWithFieldSet are consulted by the parent
//     writer to flush the field to disk.
//
// The Sprint 19 placeholder [DocsWithFieldSet] used by
// GetDocsWithFieldSet is documented in forward_deps.go; the API shape
// matches the Java reference but the underlying bitset is not yet
// implemented.
type FlatFieldVectorsWriter[T any] interface {
	codecs.KnnFieldVectorsWriter[T]

	// GetVectors returns the per-document vectors accumulated so far,
	// in docID order. The returned slice is the writer's own storage:
	// callers must not retain it past the next AddValue / Finish call.
	GetVectors() [][]T

	// GetDocsWithFieldSet returns the [DocsWithFieldSet] describing
	// which docIDs have a value for this field. The parent
	// FlatVectorsWriter consults the set to skip empty docs when
	// flushing.
	GetDocsWithFieldSet() *DocsWithFieldSet

	// IsFinished reports whether [codecs.KnnFieldVectorsWriter.Finish]
	// has been invoked. Once true, AddValue must return an error.
	IsFinished() bool
}

// BaseFlatFieldVectorsWriter captures the trivial state shared by
// every concrete FlatFieldVectorsWriter: the in-memory vector buffer,
// the docs-with-field bitset, and the finished flag. Concrete
// subclasses embed *BaseFlatFieldVectorsWriter[T] to inherit the
// non-controversial defaults and supply only the codec-specific
// AddValue + RAMBytesUsed (which depend on the element-size invariant
// the subclass enforces).
//
// The struct holds no synchronization: per-field writers are single
// threaded by contract in both Lucene and Gocene.
type BaseFlatFieldVectorsWriter[T any] struct {
	vectors  [][]T
	docs     *DocsWithFieldSet
	finished bool
}

// NewBaseFlatFieldVectorsWriter constructs an empty base writer.
// Mirrors the implicit zero-state of the Java abstract class.
func NewBaseFlatFieldVectorsWriter[T any]() *BaseFlatFieldVectorsWriter[T] {
	return &BaseFlatFieldVectorsWriter[T]{
		docs: NewDocsWithFieldSet(),
	}
}

// GetVectors returns the accumulated per-document vectors.
func (b *BaseFlatFieldVectorsWriter[T]) GetVectors() [][]T {
	return b.vectors
}

// GetDocsWithFieldSet returns the docs-with-field bitset.
func (b *BaseFlatFieldVectorsWriter[T]) GetDocsWithFieldSet() *DocsWithFieldSet {
	return b.docs
}

// IsFinished reports whether Finish has been called.
func (b *BaseFlatFieldVectorsWriter[T]) IsFinished() bool {
	return b.finished
}

// MarkFinished flips the finished flag. Concrete subclasses call this
// from their [codecs.KnnFieldVectorsWriter.Finish] implementation
// after performing any per-format finalization (e.g. flushing trailing
// quantization corrections). The Java reference exposes the same
// invariant via the protected `finished = true` assignment inside
// concrete subclasses; the Go counterpart promotes it to a tiny
// method so the field stays unexported.
func (b *BaseFlatFieldVectorsWriter[T]) MarkFinished() {
	b.finished = true
}

// AppendVector records a per-document vector by docID. The base does
// no validation beyond ordering — concrete subclasses are expected to
// enforce element-count invariants (matching the Java reference, which
// trusts the caller for the same reason).
//
// Callers are responsible for invoking AppendVector in strictly
// increasing docID order; the Java reference assumes the same
// invariant and consumers blow up at flush time if it is violated.
func (b *BaseFlatFieldVectorsWriter[T]) AppendVector(docID int, value []T) {
	cp := make([]T, len(value))
	copy(cp, value)
	b.vectors = append(b.vectors, cp)
	b.docs.Add(docID)
}
