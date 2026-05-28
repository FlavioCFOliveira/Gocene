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
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FlatVectorsWriter is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsWriter (Lucene 10.4.0). It
// extends the [codecs.KnnVectorsWriter] surface with one additional
// hook that allows callers (typically HNSW codec writers) to
// participate in the flat-vector merge pipeline:
//
//   - AddField is inherited from [codecs.KnnVectorsWriter]; it returns
//     the wide non-generic [codecs.KnnFieldVectorsWriter] (=
//     [spi.KnnFieldVectorsWriter]). Concrete callers type-assert the
//     returned value to FlatFieldVectorsWriter[float32] or
//     FlatFieldVectorsWriter[byte] based on the field's VectorEncoding,
//     mirroring how the Java reference recovers the parameterised
//     wildcard `FlatFieldVectorsWriter<?>` at the call site.
//   - MergeOneFieldToIndex performs the actual merge for a single
//     field across the segments tracked by mergeState and returns a
//     [hnsw.CloseableRandomVectorScorerSupplier] that scores against
//     the newly-merged vectors. The HNSW codec writer wires that
//     supplier into its graph builder to assemble the merged HNSW
//     index without re-reading the flat vectors from disk.
//
// The Java reference is an abstract class holding a protected final
// FlatVectorsScorer field; the Go port encodes that surface as an
// interface (this type) plus an embeddable [BaseFlatVectorsWriter]
// struct that owns the scorer reference and supplies the
// GetFlatVectorScorer accessor.
type FlatVectorsWriter interface {
	codecs.KnnVectorsWriter

	// GetFlatVectorScorer returns the scorer this writer was
	// constructed with.
	GetFlatVectorScorer() FlatVectorsScorer

	// MergeOneFieldToIndex merges the named field across all segments
	// tracked by mergeState and returns a
	// [hnsw.CloseableRandomVectorScorerSupplier] that scores against
	// the newly-merged vectors. The returned supplier owns a temporary
	// file handle and must be closed by the caller.
	//
	// mergeState is the placeholder [MergeState] documented in
	// forward_deps.go; the canonical type lives in the (not-yet-ported)
	// index/merge package and will be swapped in by a later sprint.
	MergeOneFieldToIndex(
		fieldInfo *index.FieldInfo,
		mergeState *MergeState,
	) (hnsw.CloseableRandomVectorScorerSupplier, error)
}

// BaseFlatVectorsWriter owns the [FlatVectorsScorer] handle a concrete
// FlatVectorsWriter is constructed with and supplies the
// GetFlatVectorScorer accessor. Concrete subclasses embed
// *BaseFlatVectorsWriter and implement:
//
//   - [codecs.KnnVectorsWriter] (WriteField, Finish, Close);
//   - [FlatVectorsWriter] (AddField, MergeOneFieldToIndex).
//
// The struct holds no synchronization: writers are single-threaded by
// contract in both Lucene and Gocene.
type BaseFlatVectorsWriter struct {
	vectorsScorer FlatVectorsScorer
}

// NewBaseFlatVectorsWriter constructs a base writer bound to the
// supplied scorer. Mirrors the protected constructor
// FlatVectorsWriter(FlatVectorsScorer).
func NewBaseFlatVectorsWriter(scorer FlatVectorsScorer) *BaseFlatVectorsWriter {
	return &BaseFlatVectorsWriter{vectorsScorer: scorer}
}

// GetFlatVectorScorer returns the scorer this writer was constructed
// with.
func (w *BaseFlatVectorsWriter) GetFlatVectorScorer() FlatVectorsScorer {
	return w.vectorsScorer
}
