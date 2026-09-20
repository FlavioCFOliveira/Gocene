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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FlatFieldVectorsWriter is the Go port of
// org.apache.lucene.codecs.hnsw.FlatFieldVectorsWriter<T> (Apache Lucene
// 10.5.0): the vectors writer for a field.
//
// The Java type parameter T is the vector array type (float[] or byte[]);
// the Go type parameter is its element type (float32 or byte), so Java's
// List<T> is rendered as [][]T. The inherited addValue(int, T) is
// [spi.KnnFieldVectorsWriter.AddValue], which receives the vector boxed in
// an any, because the indexing chain drives every per-field writer through
// that non-generic surface.
//
// asKnnVectorValues is a concrete method of the Java class; implementers
// that do not override it forward to [DefaultAsKnnVectorValues].
type FlatFieldVectorsWriter[T any] interface {
	spi.KnnFieldVectorsWriter

	// GetVectors returns the list of vectors to be written.
	GetVectors() [][]T

	// AsKnnVectorValues returns a KnnVectorValues view over the vectors to be
	// written, for the given vector encoding and declared dimension.
	AsKnnVectorValues(encoding index.VectorEncoding, dim int) (index.KnnVectorValues, error)

	// GetDocsWithFieldSet returns the docsWithFieldSet for the field writer.
	GetDocsWithFieldSet() *index.DocsWithFieldSet

	// IsFinished reports whether the writer is done and no new vectors are
	// allowed to be added.
	IsFinished() bool
}

// DefaultAsKnnVectorValues carries the body of
// FlatFieldVectorsWriter.asKnnVectorValues(VectorEncoding, int): the vectors
// returned by GetVectors are wrapped with FloatVectorValues.fromFloats for
// FLOAT32 and ByteVectorValues.fromBytes for BYTE. The unchecked Java cast of
// the vector list, which fails with ClassCastException when T does not match
// the encoding, is returned as an error.
func DefaultAsKnnVectorValues[T any](w FlatFieldVectorsWriter[T], encoding index.VectorEncoding, dim int) (index.KnnVectorValues, error) {
	switch encoding {
	case index.VectorEncodingFloat32:
		vectors, ok := any(w.GetVectors()).([][]float32)
		if !ok {
			return nil, fmt.Errorf("ClassCastException: %T cannot be cast to List<float[]>", w.GetVectors())
		}
		return index.FromFloats(vectors, dim), nil
	case index.VectorEncodingByte:
		vectors, ok := any(w.GetVectors()).([][]byte)
		if !ok {
			return nil, fmt.Errorf("ClassCastException: %T cannot be cast to List<byte[]>", w.GetVectors())
		}
		return index.FromBytes(vectors, dim), nil
	}
	return nil, fmt.Errorf("unknown vector encoding: %v", encoding)
}
