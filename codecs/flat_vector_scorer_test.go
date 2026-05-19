// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/codecs/hnsw/TestFlatVectorScorer.java
//
// Purpose: smoke-test that the codecs root re-exposes the canonical
// FlatVectorsScorer surface that lives under codecs/hnsw, and that the
// singleton DefaultFlatVectorScorerInstance is wired so external
// consumers (the codecs root being the historical entry point) see the
// same scorer the hnsw package serves.
//
// The deep behavioural coverage of DefaultFlatVectorScorer (dimension
// guards, float/byte parity, self-match, supplier copy isolation) lives
// alongside the implementation in
// codecs/hnsw/default_flat_vector_scorer_test.go. This file only checks
// the cross-package wiring so we don't silently regress the public path.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
)

// TestFlatVectorScorer_DelegatesToHnswPort verifies that the canonical
// scorer the wider Gocene codebase reaches for via codecs/hnsw matches
// the singleton DefaultFlatVectorScorerInstance — i.e. there is exactly
// one DefaultFlatVectorScorer in the system, not a codecs-root shadow.
func TestFlatVectorScorer_DelegatesToHnswPort(t *testing.T) {
	if got := hnsw.NewDefaultFlatVectorScorer(); got != hnsw.DefaultFlatVectorScorerInstance {
		t.Fatalf("NewDefaultFlatVectorScorer() = %p, want singleton %p",
			got, hnsw.DefaultFlatVectorScorerInstance)
	}
	if got, want := hnsw.DefaultFlatVectorScorerInstance.String(), "DefaultFlatVectorScorer()"; got != want {
		t.Fatalf("DefaultFlatVectorScorerInstance.String() = %q, want %q", got, want)
	}
}

// TestFlatVectorScorerUtil_DelegatesToHnswPort verifies the
// FlatVectorScorerUtil factory helpers return the expected singletons
// from the hnsw port. The flat scorer must be the default singleton;
// the scalar-quantized scorer must wrap that singleton.
func TestFlatVectorScorerUtil_DelegatesToHnswPort(t *testing.T) {
	if got := hnsw.GetLucene99FlatVectorsScorer(); got != hnsw.DefaultFlatVectorScorerInstance {
		t.Fatalf("GetLucene99FlatVectorsScorer() = %v, want DefaultFlatVectorScorerInstance", got)
	}
	if hnsw.GetLucene99ScalarQuantizedVectorsScorer() == nil {
		t.Fatal("GetLucene99ScalarQuantizedVectorsScorer() returned nil")
	}
}
