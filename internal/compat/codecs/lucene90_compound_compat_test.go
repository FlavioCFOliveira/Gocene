// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_compound_compat_test.go covers Lucene90CompoundFormat: the
// .cfs payload (concatenated segment files) + .cfe entries directory.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90CompoundFormat (.cfs/.cfe)" — gap_notes:
//	  "No isolated golden test parses _0.cfs entry-by-entry; .cfe
//	   parsing is only implicit."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene90Compound_DataAndEntriesEnvelopes asserts the IndexHeader
// codec strings on both .cfs and .cfe. Compound files do NOT carry a
// per-field segment suffix.
func TestLucene90Compound_DataAndEntriesEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "compound-format", seed)
			cfs := findUniqueByExt(t, dir, ".cfs")
			cfe := findUniqueByExt(t, dir, ".cfe")
			const suffix = ""
			expectIndexCodecName(t, dir, cfs, codecs.Lucene90CompoundDataCodec,
				0, 32, suffix)
			expectIndexCodecName(t, dir, cfe, codecs.Lucene90CompoundEntriesCodec,
				0, 32, suffix)
		})
	}
}

// TestLucene90Compound_FooterCRC validates the trailing CRC32 in both
// files.
func TestLucene90Compound_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "compound-format", seed)
			for _, ext := range []string{".cfs", ".cfe"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}
