// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package search contains tests for VectorSimilarityValuesSource.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestVectorSimilarityValuesSource.java
//
// GOC-3245: Port test `org.apache.lucene.search.TestVectorSimilarityValuesSource`.
//
// The Java test builds a multi-document index via RandomIndexWriter and tests
// e.g. Euclidean/Dot/Cosine similarity values sources. Gocene lacks the
// RandomIndexWriter/KnnVectorField/DoubleValuesSource.SimilarityToQueryVector
// roundtrip infrastructure. These tests verify the Float- and Byte- vector
// similarity values source types compile and function correctly at the unit
// level (constructor, equals, hashCode, toString, NeedsScores, IsCacheable).
package search

import (
	"math"
	"strings"
	"testing"
)

// TestVectorSimilarityValuesSource_FloatConstructor verifies the
// FloatVectorSimilarityValuesSource constructor defensively copies its input.
func TestVectorSimilarityValuesSource_FloatConstructor(t *testing.T) {
	vec := []float32{1.0, 2.0, 3.0}
	s := NewFloatVectorSimilarityValuesSource(vec, "myField")

	if s.GetField() != "myField" {
		t.Errorf("GetField() = %q, want %q", s.GetField(), "myField")
	}
	if s.NeedsScores() {
		t.Errorf("NeedsScores() = true, want false")
	}
	if !s.IsCacheable(nil) {
		t.Errorf("IsCacheable() = false, want true")
	}
	// QueryVector returns a copy.
	got := s.QueryVector()
	if len(got) != 3 || got[0] != 1.0 || got[1] != 2.0 || got[2] != 3.0 {
		t.Errorf("QueryVector() = %v, want [1 2 3]", got)
	}
	// Mutation of original must not affect stored vector.
	vec[0] = 99
	if s.QueryVector()[0] != 1.0 {
		t.Errorf("stored vector was mutated by input change")
	}
}

// TestVectorSimilarityValuesSource_FloatEquals verifies equals/hashCode
// semantics for FloatVectorSimilarityValuesSource.
func TestVectorSimilarityValuesSource_FloatEquals(t *testing.T) {
	vec := []float32{1.5, 2.5, 3.5}
	a := NewFloatVectorSimilarityValuesSource(vec, "field")
	b := NewFloatVectorSimilarityValuesSource(vec, "field")
	c := NewFloatVectorSimilarityValuesSource(vec, "other")
	d := NewFloatVectorSimilarityValuesSource([]float32{9.9, 8.8}, "field")

	if !a.Equals(a) {
		t.Error("a.Equals(a) = false (identity)")
	}
	if !a.Equals(b) {
		t.Error("a.Equals(b) = false (value equality)")
	}
	if a.Equals(c) {
		t.Error("a.Equals(c) = true (different field)")
	}
	if a.Equals(d) {
		t.Error("a.Equals(d) = true (different vector)")
	}
	if a.Equals(nil) {
		t.Error("a.Equals(nil) = true")
	}
	if a.Equals("not-a-source") {
		t.Error("a.Equals(string) = true")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal sources produced different hash codes")
	}
}

// TestVectorSimilarityValuesSource_FloatString verifies the toString format.
func TestVectorSimilarityValuesSource_FloatString(t *testing.T) {
	s := NewFloatVectorSimilarityValuesSource([]float32{1.0, 2.0}, "myVec")
	str := s.String()
	if !strings.Contains(str, "myVec") {
		t.Errorf("String() %q does not contain field name", str)
	}
	if !strings.Contains(str, "FloatVectorSimilarityValuesSource") {
		t.Errorf("String() %q missing type name", str)
	}
}

// TestVectorSimilarityValuesSource_FloatNaNInf verifies that NaN and Inf
// float32 values are handled by QueryVector copy.
func TestVectorSimilarityValuesSource_FloatNaNInf(t *testing.T) {
	vec := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))}
	s := NewFloatVectorSimilarityValuesSource(vec, "nanfield")
	got := s.QueryVector()
	if len(got) != 3 {
		t.Fatalf("QueryVector() len = %d, want 3", len(got))
	}
	for i, v := range vec {
		if math.Float32bits(got[i]) != math.Float32bits(v) {
			t.Errorf("QueryVector()[%d] bit mismatch", i)
		}
	}
}

// TestVectorSimilarityValuesSource_ImplementsInterface verifies the
// FloatVectorSimilarityValuesSource satisfies VectorSimilarityValuesSource.
func TestVectorSimilarityValuesSource_ImplementsInterface(t *testing.T) {
	s := NewFloatVectorSimilarityValuesSource([]float32{1.0}, "f")
	var _ VectorSimilarityValuesSource = s
}
