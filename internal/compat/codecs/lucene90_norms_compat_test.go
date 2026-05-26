// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_norms_compat_test.go covers Lucene90NormsFormat: the .nvd
// payload + .nvm metadata pair.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90NormsFormat (.nvd/.nvm)" — gap_notes:
//	  "No golden norms file from Lucene; compatibility test is
//	   Gocene-only."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene90Norms_DataAndMetaEnvelopes validates the codec headers on
// the .nvd / .nvm pair. Norms files DO NOT carry a per-field segment
// suffix (the format is not registered through PerFieldNormsFormat), so
// the expected suffix is the empty string.
func TestLucene90Norms_DataAndMetaEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "norms-format", seed)
			const suffix = ""
			nvd := findUniqueByExt(t, dir, ".nvd")
			nvm := findUniqueByExt(t, dir, ".nvm")
			expectIndexCodecName(t, dir, nvd, codecs.Lucene90NormsDataCodec,
				0, 32, suffix)
			expectIndexCodecName(t, dir, nvm, codecs.Lucene90NormsMetadataCodec,
				0, 32, suffix)
		})
	}
}

// TestLucene90Norms_FooterCRC validates the trailing CRC32 in both files.
func TestLucene90Norms_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "norms-format", seed)
			for _, ext := range []string{".nvd", ".nvm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}
