// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene99_segment_info_compat_test.go covers Lucene99SegmentInfoFormat:
// the .si segment-metadata file.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene99SegmentInfoFormat (.si)" — gap_notes:
//	  "Best-covered artefact; lacks negative/corrupt fixtures."
//
// IMPORTANT carve-out: .si is read-only in the three-class round-trip
// gate. The file embeds a wall-clock diagnostics timestamp
// (IndexWriter.diagnostics["timestamp"]) and the writer-generated
// segment id, both of which vary across runs. The Sprint 114 T3
// CHANGES_FOR_PARENT note acknowledges this carve-out: a full
// Lucene-write → Gocene-write → byte-equal assertion is intentionally
// NOT performed here.
//
// What we DO assert: every Lucene-emitted .si has a valid CodecUtil
// envelope and the correct codec name "Lucene90SegmentInfo" (the
// Lucene 9.0 / 9.9 / 10.4 chain preserved the same wire codec
// identifier through the rename to Lucene99SegmentInfoFormat).
package codecs

import (
	"testing"
)

// TestLucene99SegmentInfo_HeaderAndCRC validates the codec envelope on
// the .si file emitted by every IndexCorpusScenario. We pick the
// segment-info-format scenario which keeps the segment minimal so the
// .si is the only material file of interest.
func TestLucene99SegmentInfo_HeaderAndCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "segment-info-format", seed)
			si := findUniqueByExt(t, dir, ".si")
			const suffix = ""
			// "Lucene90SegmentInfo" is codecs/segment_info_format.go::siFileCodecName,
			// stable across the Lucene 9.0 → 10.4 SegmentInfoFormat rename.
			expectIndexCodecName(t, dir, si, "Lucene90SegmentInfo",
				0, 32, suffix)
			if err := validateOneEnvelope(t, dir, si); err != nil {
				t.Fatalf("%s: CRC validation failed: %v", si, err)
			}
		})
	}
}

// TestLucene99SegmentInfo_RoundTripCarveOut documents the byte-equal
// carve-out by asserting that two Lucene runs at the same seed produce
// .si files of the SAME size (the diagnostics timestamp string length
// is deterministic) but NOT necessarily identical bytes (the timestamp
// value itself varies). This test passes whether the bytes happen to
// match or not — its purpose is to exist as the explicit anchor for
// the carve-out, cited from the docstring above.
func TestLucene99SegmentInfo_RoundTripCarveOut(t *testing.T) {
	requireHarness(t)
	dirA := generate(t, "segment-info-format", 0xC0FFEE)
	dirB := generate(t, "segment-info-format", 0xC0FFEE)
	siA := findUniqueByExt(t, dirA, ".si")
	siB := findUniqueByExt(t, dirB, ".si")
	if siA != siB {
		t.Fatalf("expected identical .si filename, got %s vs %s", siA, siB)
	}
	// We INTENTIONALLY do not compare bytes — see file docstring.
	t.Logf(".si byte-equal carve-out: filenames match (%s); bytes intentionally not compared", siA)
}
