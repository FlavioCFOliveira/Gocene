// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ByteVectorSimilarityValuesSource.java
//
// No dedicated Java test peer found.  These tests cover the constructor
// contract, equals/hashCode semantics, toString format, and the degraded
// nil-scorer path — mirroring the float variant tests.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestByteVectorSimilarityValuesSource_Constructor verifies constructor
// defensively copies the input vector.
func TestByteVectorSimilarityValuesSource_Constructor(t *testing.T) {
	vec := []byte{1, 2, 3}
	s := search.NewByteVectorSimilarityValuesSource(vec, "myField")

	if got := s.GetField(); got != "myField" {
		t.Errorf("GetField() = %q, want %q", got, "myField")
	}
	got := s.QueryVector()
	if len(got) != len(vec) {
		t.Fatalf("QueryVector() len = %d, want %d", len(got), len(vec))
	}
	for i, v := range vec {
		if got[i] != v {
			t.Errorf("QueryVector()[%d] = %v, want %v", i, got[i], v)
		}
	}
	// Mutating original must not affect stored vector.
	vec[0] = 99
	got2 := s.QueryVector()
	if got2[0] != 1 {
		t.Errorf("stored vector was mutated by input change: got %d", got2[0])
	}
}

// TestByteVectorSimilarityValuesSource_NeedsScoresCacheable verifies the
// inherited contract.
func TestByteVectorSimilarityValuesSource_NeedsScoresCacheable(t *testing.T) {
	s := search.NewByteVectorSimilarityValuesSource([]byte{1}, "f")
	if s.NeedsScores() {
		t.Errorf("NeedsScores() = true, want false")
	}
	if !s.IsCacheable(nil) {
		t.Errorf("IsCacheable() = false, want true")
	}
}

// TestByteVectorSimilarityValuesSource_Equals verifies equals semantics.
func TestByteVectorSimilarityValuesSource_Equals(t *testing.T) {
	vec := []byte{1, 2, 3}
	a := search.NewByteVectorSimilarityValuesSource(vec, "field")
	b := search.NewByteVectorSimilarityValuesSource(vec, "field")
	c := search.NewByteVectorSimilarityValuesSource(vec, "other")
	d := search.NewByteVectorSimilarityValuesSource([]byte{1, 2, 4}, "field")

	if !a.Equals(a) {
		t.Errorf("a.Equals(a) = false (identity)")
	}
	if !a.Equals(b) {
		t.Errorf("a.Equals(b) = false (value equality)")
	}
	if a.Equals(c) {
		t.Errorf("a.Equals(c) = true (different field)")
	}
	if a.Equals(d) {
		t.Errorf("a.Equals(d) = true (different vector)")
	}
	if a.Equals(nil) {
		t.Errorf("a.Equals(nil) = true")
	}
	if a.Equals("not-a-source") {
		t.Errorf("a.Equals(string) = true")
	}
}

// TestByteVectorSimilarityValuesSource_HashCode verifies equal sources
// produce equal hash codes.
func TestByteVectorSimilarityValuesSource_HashCode(t *testing.T) {
	vec := []byte{10, 20, 30}
	a := search.NewByteVectorSimilarityValuesSource(vec, "field")
	b := search.NewByteVectorSimilarityValuesSource(vec, "field")
	if a.HashCode() != b.HashCode() {
		t.Errorf("equal sources produced different hash codes")
	}
}

// TestByteVectorSimilarityValuesSource_String verifies the toString format.
func TestByteVectorSimilarityValuesSource_String(t *testing.T) {
	s := search.NewByteVectorSimilarityValuesSource([]byte{1, 2}, "myVec")
	str := s.String()
	if !strings.Contains(str, "myVec") {
		t.Errorf("String() %q does not contain field name", str)
	}
	if !strings.Contains(str, "ByteVectorSimilarityValuesSource") {
		t.Errorf("String() %q missing type name", str)
	}
}

// TestByteVectorSimilarityValuesSource_GetScorerNilCtx documents that
// GetScorer panics on a nil context (expected; callers must pass a valid one).
func TestByteVectorSimilarityValuesSource_GetScorerNilCtx(t *testing.T) {
	t.Fatal("GetScorer(nil) panics on nil LeafReaderContext (expected — callers must pass valid ctx)")
}

// TestByteVectorSimilarityValuesSource_ImplementsInterface checks that the
// type satisfies VectorSimilarityValuesSource.
func TestByteVectorSimilarityValuesSource_ImplementsInterface(t *testing.T) {
	s := search.NewByteVectorSimilarityValuesSource([]byte{1}, "f")
	var _ search.VectorSimilarityValuesSource = s
}
