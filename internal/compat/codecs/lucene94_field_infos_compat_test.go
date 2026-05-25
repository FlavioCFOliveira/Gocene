// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene94_field_infos_compat_test.go covers Lucene94FieldInfosFormat:
// the .fnm field-info table. The same scenario also tests the legacy
// Lucene90FieldInfosFormat presence (it is read on older segments;
// Lucene94 is the current writer).
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene94FieldInfosFormat (.fnm)" — gap_notes:
//	  "No isolated test reads Lucene-emitted .fnm bytes."
//	"Lucene90FieldInfosFormat (legacy .fnm)" — gap_notes:
//	  "Legacy field infos lacks cross-version coverage."
//
// The legacy gap is acknowledged here: Lucene 10.4 does NOT emit the
// legacy .fnm format from a fresh IndexWriter run, so we can only
// exercise the current Lucene94 writer in a cross-engine fixture. The
// legacy reader path is covered by the Gocene-only unit tests in
// codecs/lucene90_field_infos_format_test.go.
package codecs

import (
	"testing"
)

// TestLucene94FieldInfos_HeaderAndCRC validates the .fnm IndexHeader on
// the Lucene-emitted field-infos-format corpus.
func TestLucene94FieldInfos_HeaderAndCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "field-infos-format", seed)
			fnm := findUniqueByExt(t, dir, ".fnm")
			const suffix = ""
			// "Lucene94FieldInfos" is the package-private constant
			// codecs/lucene94_field_infos_format.go::lucene94FICodecName.
			expectIndexCodecName(t, dir, fnm, "Lucene94FieldInfos",
				0, 32, suffix)
			if err := validateOneEnvelope(t, dir, fnm); err != nil {
				t.Fatalf("%s: CRC validation failed: %v", fnm, err)
			}
		})
	}
}
