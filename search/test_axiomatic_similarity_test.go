// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/TestAxiomaticSimilarity.java
//
// Tests that constructors panic on invalid s/k/ql parameters, matching the
// Java IllegalArgumentException contract.

package search

import (
	"math"
	"testing"
)

// assertPanicF2EXP verifies that constructing NewLuceneAxiomaticF2EXP with the
// given s, k panics; t.SkipIf is false then the call is expected to succeed.
func assertPanicF2EXP(t *testing.T, s, k float32, expectPanic bool) {
	t.Helper()
	defer func() {
		r := recover()
		if expectPanic && r == nil {
			t.Errorf("NewLuceneAxiomaticF2EXP(%v,%v) should have panicked", s, k)
		}
		if !expectPanic && r != nil {
			t.Errorf("NewLuceneAxiomaticF2EXP(%v,%v) unexpectedly panicked: %v", s, k, r)
		}
	}()
	_ = NewLuceneAxiomaticF2EXP(s, k)
}

// assertPanicF3EXP verifies that constructing NewLuceneAxiomaticF3EXP with the
// given s, queryLen, k panics if expectPanic is true.
func assertPanicF3EXP(t *testing.T, s float32, queryLen int, k float32, expectPanic bool) {
	t.Helper()
	defer func() {
		r := recover()
		if expectPanic && r == nil {
			t.Errorf("NewLuceneAxiomaticF3EXP(%v,%v,%v) should have panicked", s, queryLen, k)
		}
		if !expectPanic && r != nil {
			t.Errorf("NewLuceneAxiomaticF3EXP(%v,%v,%v) unexpectedly panicked: %v", s, queryLen, k, r)
		}
	}()
	_ = NewLuceneAxiomaticF3EXP(s, queryLen, k)
}

// TestAxiomaticSimilarity_IllegalS mirrors testIllegalS.
// Java: AxiomaticF2EXP(±Inf, 0.1), AxiomaticF2EXP(-1, 0.1) → IllegalArgumentException.
func TestAxiomaticSimilarity_IllegalS(t *testing.T) {
	assertPanicF2EXP(t, float32(math.Inf(-1)), 0.1, true) // -Inf
	assertPanicF2EXP(t, float32(math.Inf(1)), 0.1, true)  // +Inf
	assertPanicF2EXP(t, -1, 0.1, true)
	assertPanicF2EXP(t, 2.0, 0.1, true)      // > 1
	assertPanicF2EXP(t, float32(math.NaN()), 0.1, true)

	// Valid s values (0 <= s <= 1) must succeed.
	assertPanicF2EXP(t, 0, 0.1, false)
	assertPanicF2EXP(t, 0.25, 0.1, false)
	assertPanicF2EXP(t, 1.0, 0.1, false)
}

// TestAxiomaticSimilarity_IllegalK mirrors testIllegalK.
func TestAxiomaticSimilarity_IllegalK(t *testing.T) {
	assertPanicF2EXP(t, 0.25, float32(math.Inf(-1)), true) // -Inf
	assertPanicF2EXP(t, 0.25, float32(math.Inf(1)), true)  // +Inf
	assertPanicF2EXP(t, 0.25, -1, true)
	assertPanicF2EXP(t, 0.25, 2.0, true)      // > 1
	assertPanicF2EXP(t, 0.25, float32(math.NaN()), true)

	// Valid k values must succeed.
	assertPanicF2EXP(t, 0.25, 0, false)
	assertPanicF2EXP(t, 0.25, 0.35, false)
	assertPanicF2EXP(t, 0.25, 1.0, false)
}

// TestAxiomaticSimilarity_IllegalQL mirrors testIllegalQL.
func TestAxiomaticSimilarity_IllegalQL(t *testing.T) {
	assertPanicF3EXP(t, 0.25, -1, 0.35, true)  // negative
	assertPanicF3EXP(t, 0.25, -100, 0.35, true)

	// Valid queryLen values must succeed.
	assertPanicF3EXP(t, 0.25, 0, 0.35, false)
	assertPanicF3EXP(t, 0.25, 5, 0.35, false)
}
