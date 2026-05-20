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
//   lucene/core/src/java/org/apache/lucene/search/FloatVectorSimilarityValuesSource.java

import (
	"fmt"
	"hash/fnv"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FloatVectorSimilarityScorer is an optional interface that concrete
// index.FloatVectorValues implementations may satisfy to expose a
// VectorScorer for a given query vector. It mirrors the Java method
// FloatVectorValues.scorer(float[]).
//
// When the wrapped FloatVectorValues does not implement this interface,
// GetScorer returns nil.
type FloatVectorSimilarityScorer interface {
	// Scorer returns a VectorScorer for the given query vector, or nil
	// when scoring is not supported.
	Scorer(queryVector []float32) (VectorScorer, error)
}

// FloatVectorSimilarityValuesSource is a VectorSimilarityValuesSource
// that computes vector similarity scores between a float32 query vector
// and the KnnFloatVectorField values stored for each document.
//
// Ported from org.apache.lucene.search.FloatVectorSimilarityValuesSource
// (Lucene 10.4.0). In Java this is a package-private final class;
// the Go port exports it so external callers can construct it directly
// via NewFloatVectorSimilarityValuesSource.
//
// Deviations from the Java reference:
//   - FloatVectorValues.checkField is not called when vectorValues is nil;
//     the check relies on the index layer to surface type mismatches at
//     read time, matching the existing Gocene convention.
//   - When index.FloatVectorValues does not implement
//     FloatVectorSimilarityScorer, GetScorer returns nil (no scorer
//     available), rather than panicking. This matches the intended
//     degraded-until-codec-wired behaviour of the search layer.
type FloatVectorSimilarityValuesSource struct {
	baseVectorSimilarityValuesSource
	queryVector []float32
}

// NewFloatVectorSimilarityValuesSource constructs a
// FloatVectorSimilarityValuesSource for the given float32 query vector
// and field name.
//
// Mirrors FloatVectorSimilarityValuesSource(float[], String).
func NewFloatVectorSimilarityValuesSource(vector []float32, fieldName string) *FloatVectorSimilarityValuesSource {
	dst := make([]float32, len(vector))
	copy(dst, vector)
	return &FloatVectorSimilarityValuesSource{
		baseVectorSimilarityValuesSource: baseVectorSimilarityValuesSource{fieldName: fieldName},
		queryVector:                      dst,
	}
}

// QueryVector returns a copy of the stored query vector.
func (s *FloatVectorSimilarityValuesSource) QueryVector() []float32 {
	out := make([]float32, len(s.queryVector))
	copy(out, s.queryVector)
	return out
}

// GetScorer returns a VectorScorer for the given leaf context, or nil
// when the field is absent from the segment or the underlying
// FloatVectorValues does not support scoring.
//
// Mirrors FloatVectorSimilarityValuesSource.getScorer.
func (s *FloatVectorSimilarityValuesSource) GetScorer(ctx *index.LeafReaderContext) (VectorScorer, error) {
	lr, ok := ctx.Reader().(*index.LeafReader)
	if !ok {
		return nil, nil
	}
	vectorValues, err := lr.GetFloatVectorValues(s.fieldName)
	if err != nil {
		return nil, err
	}
	if vectorValues == nil {
		return nil, nil
	}
	ss, ok := vectorValues.(FloatVectorSimilarityScorer)
	if !ok {
		// The underlying FloatVectorValues does not support scoring yet.
		return nil, nil
	}
	return ss.Scorer(s.queryVector)
}

// Equals reports whether other is a *FloatVectorSimilarityValuesSource
// with the same field name and query vector.
//
// Mirrors FloatVectorSimilarityValuesSource.equals.
func (s *FloatVectorSimilarityValuesSource) Equals(other any) bool {
	if s == other {
		return true
	}
	o, ok := other.(*FloatVectorSimilarityValuesSource)
	if !ok || o == nil {
		return false
	}
	if s.fieldName != o.fieldName {
		return false
	}
	if len(s.queryVector) != len(o.queryVector) {
		return false
	}
	for i, v := range s.queryVector {
		// Use bit-exact comparison (matching Java Arrays.equals on float[]).
		if math.Float32bits(v) != math.Float32bits(o.queryVector[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash combining fieldName and queryVector.
//
// Mirrors FloatVectorSimilarityValuesSource.hashCode, which uses
// Objects.hash(fieldName, Arrays.hashCode(queryVector)).
func (s *FloatVectorSimilarityValuesSource) HashCode() uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprint(h, s.fieldName)
	for _, v := range s.queryVector {
		b := math.Float32bits(v)
		_, _ = h.Write([]byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)})
	}
	return h.Sum64()
}

// String returns a human-readable description.
//
// Mirrors FloatVectorSimilarityValuesSource.toString.
func (s *FloatVectorSimilarityValuesSource) String() string {
	return fmt.Sprintf("FloatVectorSimilarityValuesSource(fieldName=%s queryVector=%v)",
		s.fieldName, s.queryVector)
}

// Compile-time check: FloatVectorSimilarityValuesSource satisfies
// VectorSimilarityValuesSource.
var _ VectorSimilarityValuesSource = (*FloatVectorSimilarityValuesSource)(nil)
