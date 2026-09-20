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
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FlatVectorsWriter is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsWriter (Apache Lucene 10.5.0):
// a vectors writer for a field that allows additional indexing logic to be
// implemented by the caller.
//
// The Java reference is an abstract class extending KnnVectorsWriter and
// holding the protected final FlatVectorsScorer handed to its constructor.
// Its addField narrows the return type to FlatFieldVectorsWriter<?>; the Go
// rendering inherits [spi.KnnVectorsWriter.AddField] and callers assert the
// result to FlatFieldVectorsWriter[float32] or FlatFieldVectorsWriter[byte]
// by the field's vector encoding, as the Java callers cast it.
//
// The final mergeOneField of the Java class is rendered by the package
// function [MergeOneField].
type FlatVectorsWriter interface {
	spi.KnnVectorsWriter

	// GetFlatVectorScorer returns the [FlatVectorsScorer] for this writer.
	GetFlatVectorScorer() FlatVectorsScorer

	// MergeOneFlatVectorField merges the flat vectors of one field across the
	// segments tracked by mergeState. Mirrors the abstract
	// mergeOneFlatVectorField(FieldInfo, MergeState).
	MergeOneFlatVectorField(fieldInfo *index.FieldInfo, mergeState *index.MergeState) error
}

// BaseFlatVectorsWriter owns the [FlatVectorsScorer] a concrete
// FlatVectorsWriter is constructed with and supplies GetFlatVectorScorer.
type BaseFlatVectorsWriter struct {
	vectorsScorer FlatVectorsScorer
}

// NewBaseFlatVectorsWriter constructs a base writer bound to the supplied
// scorer. Mirrors the protected constructor
// FlatVectorsWriter(FlatVectorsScorer).
func NewBaseFlatVectorsWriter(scorer FlatVectorsScorer) *BaseFlatVectorsWriter {
	return &BaseFlatVectorsWriter{vectorsScorer: scorer}
}

// GetFlatVectorScorer returns the scorer this writer was constructed with.
func (w *BaseFlatVectorsWriter) GetFlatVectorScorer() FlatVectorsScorer {
	return w.vectorsScorer
}

// MergeOneField renders the final FlatVectorsWriter.mergeOneField(FieldInfo,
// MergeState): it merges the flat vectors of the field through
// MergeOneFlatVectorField and returns no deferred work.
func MergeOneField(w FlatVectorsWriter, fieldInfo *index.FieldInfo, mergeState *index.MergeState) (func() error, error) {
	if err := w.MergeOneFlatVectorField(fieldInfo, mergeState); err != nil {
		return nil, err
	}
	return nil, nil
}
