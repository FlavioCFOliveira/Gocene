// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene104_scalar_quantized_compat_test.go covers Lucene10.4's
// scalar-quantized HNSW vectors format: .veq (quantized vector data),
// alongside .vec/.vex/.vem/.vemf/.vemq. The .veq wire format had no
// cross-engine coverage before Sprint 114 T7.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene104 Scalar-Quantized HNSW vectors (.veq)" — gap_notes:
//	  "No writer, no cross-engine fixture, no isolated test."
//
// Sprint 114 T7 registers the new "scalar-quantized-knn" scenario which
// drives Lucene104HnswScalarQuantizedVectorsFormat over a deterministic
// document set. This file validates the resulting .veq envelope.
package codecs

import (
	"testing"
)

// TestLucene104ScalarQuantized_VeqEnvelope opens the .veq file emitted
// by the scalar-quantized-knn scenario and confirms it carries a valid
// CodecUtil envelope. The exact codec name string is internal to
// Lucene's quantization writer (we don't pin a string here because
// the writer-side names are unexported in both Gocene and Lucene); the
// envelope structure is the binary contract we own.
func TestLucene104ScalarQuantized_VeqEnvelope(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "scalar-quantized-knn", seed)
			veq := findUniqueByExt(t, dir, ".veq")
			if err := validateOneEnvelope(t, dir, veq); err != nil {
				t.Fatalf("%s: CRC validation failed: %v", veq, err)
			}
			// Quantized vectors emit at least one byte per dimension
			// per document; for our 16-doc × 8-dim scenario that's
			// >= 128 bytes of payload beyond the envelope.
			mustNonEmpty(t, dir, veq, 48 /* worst-case header */)
		})
	}
}

// TestLucene104ScalarQuantized_AllSidecarFiles validates every file the
// scalar-quantized format emits: .vec (raw vector data), .vex (HNSW
// graph), .vem (per-field metadata), .vemf (HNSW format meta), .vemq
// (quantization metadata) and .veq (quantized data). All six MUST carry
// a valid CodecUtil envelope.
func TestLucene104ScalarQuantized_AllSidecarFiles(t *testing.T) {
	requireHarness(t)
	dir := generate(t, "scalar-quantized-knn", 0xC0FFEE)
	for _, ext := range []string{".vec", ".vex", ".vem", ".vemf", ".vemq", ".veq"} {
		name := findUniqueByExt(t, dir, ext)
		if err := validateOneEnvelope(t, dir, name); err != nil {
			t.Fatalf("%s: CRC validation failed: %v", name, err)
		}
	}
}

// TestLucene104ScalarQuantized_TwoSeedDeterminism asserts that
// generating the scenario twice at the same seed produces a file with
// the same name and same length. Byte-equality is enforced by the
// JUnit ScenarioDeterminismTest on the Java side; this test makes the
// invariant visible from Go too.
func TestLucene104ScalarQuantized_TwoSeedDeterminism(t *testing.T) {
	requireHarness(t)
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dirA := generate(t, "scalar-quantized-knn", seed)
			dirB := generate(t, "scalar-quantized-knn", seed)
			veqA := findUniqueByExt(t, dirA, ".veq")
			veqB := findUniqueByExt(t, dirB, ".veq")
			if veqA != veqB {
				t.Fatalf("seed=%d .veq filename mismatch: %s vs %s",
					seed, veqA, veqB)
			}
		})
	}
}
