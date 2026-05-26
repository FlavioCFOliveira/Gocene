// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene99_hnsw_flat_compat_test.go covers Lucene99HnswVectorsFormat and
// its underlying Lucene99FlatVectorsFormat: the .vec (flat vector data),
// .vex (HNSW graph), .vem (per-field metadata), .vemf (HNSW format
// metadata).
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene99HnswVectorsFormat (.vec/.vex/.vem)" — gap_notes:
//	  "HNSW graph bytes inside .cfs are not exercised by Gocene tests."
//	"Lucene99FlatVectorsFormat (flat .vec)" — gap_notes:
//	  "No round-trip or golden test against Lucene-produced flat
//	   vectors."
package codecs

import (
	"testing"
)

// TestLucene99Hnsw_FlatAndGraphFiles validates the codec envelopes on
// .vec, .vex, .vem and .vemf. The non-quantized HNSW scenario
// (knn-vectors-format) produces both float and byte vector fields, so
// .vec contains both flavours.
func TestLucene99Hnsw_FlatAndGraphFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "knn-vectors-format", seed)
			for _, ext := range []string{".vec", ".vex", ".vem", ".vemf"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene99Hnsw_GraphPayloadNonEmpty checks the .vex file is larger
// than the bare envelope. A non-empty HNSW graph always emits at least
// a level-0 layer + one node connection.
func TestLucene99Hnsw_GraphPayloadNonEmpty(t *testing.T) {
	requireHarness(t)
	dir := generate(t, "knn-vectors-format", 0xC0FFEE)
	vex := findUniqueByExt(t, dir, ".vex")
	// IndexHeader varies in length with the codec-name length; pass a
	// conservative lower bound of 32 (max codec name) + 16 footer.
	mustNonEmpty(t, dir, vex, 48)
}
