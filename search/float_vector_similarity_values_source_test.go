// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer exists for FloatVectorSimilarityValuesSource in
// Lucene 10.4.0 (TestFloatVectorSimilarityValuesSource is absent).
// These tests cover the constructor contract, equals/hashCode semantics,
// toString format, and the degraded nil-scorer path.

package search_test

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestFloatVectorSimilarityValuesSource_Constructor(t *testing.T) {
	vec := []float32{1.0, 2.0, 3.0}
	s := search.NewFloatVectorSimilarityValuesSource(vec, "myField")

	if got := s.GetField(); got != "myField" {
		t.Errorf("GetField() = %q, want %q", got, "myField")
	}
	// QueryVector must be a defensive copy.
	got := s.QueryVector()
	if len(got) != len(vec) {
		t.Fatalf("QueryVector() len = %d, want %d", len(got), len(vec))
	}
	for i, v := range vec {
		if math.Float32bits(got[i]) != math.Float32bits(v) {
			t.Errorf("QueryVector()[%d] = %v, want %v", i, got[i], v)
		}
	}
	// Mutating original slice must not affect stored vector.
	vec[0] = 99
	got2 := s.QueryVector()
	if math.Float32bits(got2[0]) != math.Float32bits(1.0) {
		t.Errorf("stored vector was mutated by input change: got %v", got2[0])
	}
}

func TestFloatVectorSimilarityValuesSource_NeedsScoresCacheable(t *testing.T) {
	s := search.NewFloatVectorSimilarityValuesSource([]float32{1.0}, "f")
	if s.NeedsScores() {
		t.Errorf("NeedsScores() = true, want false")
	}
	if !s.IsCacheable(nil) {
		t.Errorf("IsCacheable() = false, want true")
	}
}

func TestFloatVectorSimilarityValuesSource_Equals(t *testing.T) {
	vec := []float32{1.0, 2.0}
	a := search.NewFloatVectorSimilarityValuesSource(vec, "field")
	b := search.NewFloatVectorSimilarityValuesSource(vec, "field")
	c := search.NewFloatVectorSimilarityValuesSource(vec, "other")
	d := search.NewFloatVectorSimilarityValuesSource([]float32{1.0, 3.0}, "field")

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

	// NaN equality: bit-exact comparison means NaN == NaN.
	nan32 := math.Float32frombits(0x7FC00000)
	na := search.NewFloatVectorSimilarityValuesSource([]float32{nan32}, "f")
	nb := search.NewFloatVectorSimilarityValuesSource([]float32{nan32}, "f")
	if !na.Equals(nb) {
		t.Errorf("NaN vectors should be bit-exact-equal")
	}
}

func TestFloatVectorSimilarityValuesSource_HashCode(t *testing.T) {
	vec := []float32{1.0, 2.0}
	a := search.NewFloatVectorSimilarityValuesSource(vec, "field")
	b := search.NewFloatVectorSimilarityValuesSource(vec, "field")
	if a.HashCode() != b.HashCode() {
		t.Errorf("equal sources produced different hash codes")
	}
	c := search.NewFloatVectorSimilarityValuesSource(vec, "other")
	if a.HashCode() == c.HashCode() {
		// Acceptable collision, but log for awareness.
		t.Logf("hash collision between different field names (acceptable)")
	}
}

func TestFloatVectorSimilarityValuesSource_String(t *testing.T) {
	s := search.NewFloatVectorSimilarityValuesSource([]float32{1.5, -2.0}, "myVec")
	str := s.String()
	if !strings.Contains(str, "myVec") {
		t.Errorf("String() %q does not contain field name", str)
	}
	if !strings.Contains(str, "FloatVectorSimilarityValuesSource") {
		t.Errorf("String() %q missing type name", str)
	}
}

// TestFloatVectorSimilarityValuesSource_GetScorerNilCtx verifies that
// GetScorer panics on a nil context (nil pointer dereference on
// ctx.Reader()), which is the expected behaviour since callers must
// always pass a valid LeafReaderContext.
func TestFloatVectorSimilarityValuesSource_GetScorerNilCtx(t *testing.T) {
	s := search.NewFloatVectorSimilarityValuesSource([]float32{1.0, 2.0}, "f")
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("GetScorer(nil) should panic, got nil")
			}
		}()
		_, _ = s.GetScorer(nil)
	}()
}
