// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_points_compat_test.go covers Lucene90PointsFormat: the
// .kdd/.kdi/.kdm triple (BKD data + index + metadata).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90PointsFormat (.kdd/.kdi/.kdm)" — gap_notes:
//	  "No fixture with BKD points; no Lucene-write cross test."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene90Points_AllThreeEnvelopes validates the IndexHeader codec
// names on .kdd (data), .kdi (index) and .kdm (metadata). Points files
// do not carry a per-field segment suffix.
func TestLucene90Points_AllThreeEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "points-format", seed)
			const suffix = ""
			kdd := findUniqueByExt(t, dir, ".kdd")
			kdi := findUniqueByExt(t, dir, ".kdi")
			kdm := findUniqueByExt(t, dir, ".kdm")
			expectIndexCodecName(t, dir, kdd, codecs.Lucene90PointsDataCodec,
				0, 32, suffix)
			expectIndexCodecName(t, dir, kdi, codecs.Lucene90PointsIndexCodec,
				0, 32, suffix)
			expectIndexCodecName(t, dir, kdm, codecs.Lucene90PointsMetaCodec,
				0, 32, suffix)
		})
	}
}

// TestLucene90Points_FooterCRC validates the trailing CRC32 on all three
// files.
func TestLucene90Points_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "points-format", seed)
			for _, ext := range []string{".kdd", ".kdi", ".kdm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}
