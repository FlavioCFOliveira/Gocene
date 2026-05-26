// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_doc_values_compat_test.go covers Lucene90DocValuesFormat: the
// .dvd payload + .dvm metadata pair, exercising the NUMERIC, BINARY,
// SORTED, SORTED_NUMERIC and SORTED_SET flavours (the doc-values-format
// scenario emits one document carrying all five).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90DocValuesFormat (.dvd/.dvm)" — gap_notes:
//	  "No test reads doc-values segments produced by Lucene; combined
//	   test uses Gocene-only round-trip."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene90DocValues_DataAndMetaEnvelopes opens the .dvd/.dvm pair
// emitted by Lucene and confirms the IndexHeader codec strings + version
// matches Gocene's constants (Lucene90DocValuesDataCodec /
// Lucene90DocValuesMetaCodec).
func TestLucene90DocValues_DataAndMetaEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "doc-values-format", seed)
			const suffix = "Lucene90_0"
			dvd := findUniqueByExt(t, dir, ".dvd")
			dvm := findUniqueByExt(t, dir, ".dvm")
			version := expectIndexCodecName(t, dir, dvd,
				codecs.Lucene90DocValuesDataCodec,
				codecs.Lucene90DocValuesVersionStart,
				codecs.Lucene90DocValuesVersionCurrent, suffix)
			if version != codecs.Lucene90DocValuesVersionCurrent {
				t.Errorf("%s: version=%d, want %d", dvd, version,
					codecs.Lucene90DocValuesVersionCurrent)
			}
			expectIndexCodecName(t, dir, dvm,
				codecs.Lucene90DocValuesMetaCodec,
				codecs.Lucene90DocValuesVersionStart,
				codecs.Lucene90DocValuesVersionCurrent, suffix)
		})
	}
}

// TestLucene90DocValues_FooterCRC validates the trailing CRC32 in both
// files (class (a) of the three-class gate).
func TestLucene90DocValues_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "doc-values-format", seed)
			for _, ext := range []string{".dvd", ".dvm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}
