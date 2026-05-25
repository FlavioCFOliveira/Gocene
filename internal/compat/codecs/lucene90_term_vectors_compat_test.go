// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_term_vectors_compat_test.go covers Lucene90TermVectorsFormat:
// .tvd payload, .tvx index, .tvm metadata.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90TermVectorsFormat (.tvd/.tvx/.tvm)" — gap_notes:
//	  "No Lucene-emitted term-vectors file available as a fixture."
package codecs

import (
	"testing"
)

// TestLucene90TermVectors_AllThreeFiles validates the three term-vectors
// files emitted by Lucene 10.4. The IndexHeader codec name is
// "Lucene90TermVectorsData" (see codecs/lucene90_term_vectors_format.go,
// Lucene90TermVectorsFormatName constant) for all three; each file gets
// the same name but a different per-file suffix in practice.
func TestLucene90TermVectors_AllThreeFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "term-vectors-format", seed)
			for _, ext := range []string{".tvd", ".tvx", ".tvm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}
