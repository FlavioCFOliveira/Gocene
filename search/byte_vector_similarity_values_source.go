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

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ByteVectorSimilarityValuesSource.java

import (
	"bytes"
	"fmt"
	"hash/fnv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ByteVectorSimilarityScorer is an optional interface that concrete
// index.ByteVectorValues implementations may satisfy to expose a VectorScorer
// for a given byte query vector.
//
// Mirrors ByteVectorValues.scorer(byte[]) in Lucene 10.4.0.
type ByteVectorSimilarityScorer interface {
	// Scorer returns a VectorScorer for the given byte query vector, or nil
	// when scoring is not supported.
	Scorer(queryVector []byte) (VectorScorer, error)
}

// ByteVectorSimilarityValuesSource is a VectorSimilarityValuesSource that
// computes vector similarity scores between a byte query vector and the
// KnnByteVectorField values stored for each document.
//
// Mirrors org.apache.lucene.search.ByteVectorSimilarityValuesSource (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java ByteVectorSimilarityValuesSource is package-private; the Go port
//     exports it so external callers can construct it directly.
//   - ByteVectorValues.checkField is not called when vectorValues is nil;
//     the check is deferred to the index layer, matching the Gocene convention.
//   - When index.ByteVectorValues does not implement ByteVectorSimilarityScorer,
//     GetScorer returns nil (degraded until codec is wired).
type ByteVectorSimilarityValuesSource struct {
	baseVectorSimilarityValuesSource
	queryVector []byte
}

// NewByteVectorSimilarityValuesSource constructs a
// ByteVectorSimilarityValuesSource for the given byte query vector and field.
//
// Mirrors ByteVectorSimilarityValuesSource(byte[], String).
func NewByteVectorSimilarityValuesSource(vector []byte, fieldName string) *ByteVectorSimilarityValuesSource {
	dst := make([]byte, len(vector))
	copy(dst, vector)
	return &ByteVectorSimilarityValuesSource{
		baseVectorSimilarityValuesSource: baseVectorSimilarityValuesSource{fieldName: fieldName},
		queryVector:                      dst,
	}
}

// QueryVector returns a copy of the stored query vector.
func (s *ByteVectorSimilarityValuesSource) QueryVector() []byte {
	out := make([]byte, len(s.queryVector))
	copy(out, s.queryVector)
	return out
}

// GetScorer returns a VectorScorer for the given leaf context, or nil when
// the field is absent from the segment or the underlying ByteVectorValues
// does not support scoring.
//
// Mirrors ByteVectorSimilarityValuesSource.getScorer.
func (s *ByteVectorSimilarityValuesSource) GetScorer(ctx *index.LeafReaderContext) (VectorScorer, error) {
	lr, ok := ctx.Reader().(*index.LeafReader)
	if !ok {
		return nil, nil
	}
	vectorValues, err := lr.GetByteVectorValues(s.fieldName)
	if err != nil {
		return nil, err
	}
	if vectorValues == nil {
		return nil, nil
	}
	ss, ok := vectorValues.(ByteVectorSimilarityScorer)
	if !ok {
		// Underlying ByteVectorValues does not support scoring yet.
		return nil, nil
	}
	return ss.Scorer(s.queryVector)
}

// Equals reports whether other is a *ByteVectorSimilarityValuesSource with
// the same field name and query vector.
//
// Mirrors ByteVectorSimilarityValuesSource.equals.
func (s *ByteVectorSimilarityValuesSource) Equals(other any) bool {
	if s == other {
		return true
	}
	o, ok := other.(*ByteVectorSimilarityValuesSource)
	if !ok || o == nil {
		return false
	}
	return s.fieldName == o.fieldName && bytes.Equal(s.queryVector, o.queryVector)
}

// HashCode returns a stable hash combining fieldName and queryVector.
//
// Mirrors ByteVectorSimilarityValuesSource.hashCode, which uses
// Objects.hash(fieldName, Arrays.hashCode(queryVector)).
func (s *ByteVectorSimilarityValuesSource) HashCode() uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprint(h, s.fieldName)
	_, _ = h.Write(s.queryVector)
	return h.Sum64()
}

// String returns a human-readable description.
//
// Mirrors ByteVectorSimilarityValuesSource.toString.
func (s *ByteVectorSimilarityValuesSource) String() string {
	return fmt.Sprintf("ByteVectorSimilarityValuesSource(fieldName=%s queryVector=%v)",
		s.fieldName, s.queryVector)
}

// Compile-time check: ByteVectorSimilarityValuesSource satisfies
// VectorSimilarityValuesSource.
var _ VectorSimilarityValuesSource = (*ByteVectorSimilarityValuesSource)(nil)
