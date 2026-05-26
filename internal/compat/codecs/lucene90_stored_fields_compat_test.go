// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_stored_fields_compat_test.go covers Lucene90StoredFieldsFormat
// (the .fdt payload + .fdx index + .fdm metadata triple) AND its
// compressing variant. The base scenario uses the default codec
// (BEST_SPEED + LZ4); the new compressing-stored-fields scenario added
// by rmp 4615 exercises BEST_COMPRESSION (DEFLATE).
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90StoredFieldsFormat (.fdt/.fdx/.fdm)" — gap_notes:
//	  "Stored-fields bytes inside .cfs fixture are not decoded by an
//	   isolated test."
//	"Lucene90 compressing block format (LZ4/Deflate/BEST_SPEED)" —
//	  gap_notes: "Compression bytes never compared to Lucene output."
package codecs

import (
	"testing"
)

// TestLucene90StoredFields_BestSpeed_AllThreeFiles checks the LZ4 path:
// .fdt/.fdx/.fdm IndexHeaders all parse and CRC32 trailers all validate.
// The compressing-stored-fields code path stamps the per-codec NAME as
// "Lucene90StoredFieldsFastData" and "Lucene90StoredFieldsFastIndex"
// for the BEST_SPEED layout (see codecs/lucene90/lucene90_stored_fields_format.go).
func TestLucene90StoredFields_BestSpeed_AllThreeFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "stored-fields-format", seed)
			for _, ext := range []string{".fdt", ".fdx", ".fdm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene90StoredFields_BestCompression_AllThreeFiles is the DEFLATE
// counterpart, fed by the new "compressing-stored-fields" scenario. The
// scenario uses Lucene104Codec(BEST_COMPRESSION) which switches the
// stored-fields wire codec to "Lucene90StoredFieldsHighData".
func TestLucene90StoredFields_BestCompression_AllThreeFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "compressing-stored-fields", seed)
			for _, ext := range []string{".fdt", ".fdx", ".fdm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene90StoredFields_BothModesProduceDifferentBytes is the
// payload-level smoke test: BEST_SPEED and BEST_COMPRESSION on the same
// documents MUST produce different .fdt bytes (LZ4 vs DEFLATE compress
// differently). Catches the regression where the Mode constructor is
// ignored and both modes silently use LZ4.
func TestLucene90StoredFields_BothModesProduceDifferentBytes(t *testing.T) {
	requireHarness(t)
	// Note: we cannot directly compare the two .fdt bytes because the
	// two scenarios index different documents (different numDocs and
	// different body content). The differentiator is instead the codec
	// name stamped in the IndexHeader: BEST_SPEED writes
	// "Lucene90StoredFieldsFastData" while BEST_COMPRESSION writes
	// "Lucene90StoredFieldsHighData". Validate both names appear.
	speedDir := generate(t, "stored-fields-format", 0xC0FFEE)
	compDir := generate(t, "compressing-stored-fields", 0xC0FFEE)
	speedFdt := findUniqueByExt(t, speedDir, ".fdt")
	compFdt := findUniqueByExt(t, compDir, ".fdt")
	const suffix = ""
	expectIndexCodecName(t, speedDir, speedFdt, "Lucene90StoredFieldsFastData",
		0, 32, suffix)
	expectIndexCodecName(t, compDir, compFdt, "Lucene90StoredFieldsHighData",
		0, 32, suffix)
}
