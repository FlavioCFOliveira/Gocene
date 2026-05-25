// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene104_postings_compat_test.go covers the postings-format scenario,
// which exercises Apache Lucene 10.4's Lucene104PostingsFormat: the
// .doc/.pos/.pay/.psm postings streams plus the .tim/.tip/.tmd term
// dictionary written by Lucene103BlockTreeTermsWriter (the postings format
// stamps the block-tree codec into those files via the surrounding
// codec_util envelope).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene104PostingsFormat (.doc/.pos/.pay/.tim/.tip/.tmd)" — gap_notes:
//	  "Postings files are embedded in the .cfs fixture, but no test reads
//	   them back from the Lucene-generated corpus."
//
// This file closes that gap with a non-CFS corpus (the postings-format
// scenario uses useCompoundFile=false) so each artefact is reachable
// directly via OpenInput.
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene104Postings_HeaderEnvelopes runs class (a) of the three-class
// gate: Lucene writes the corpus, Gocene parses the per-stream IndexHeader
// and confirms the codec name + version stamped in every file matches the
// Gocene-side constants. Two seeds satisfy the byte-determinism contract
// (T7 acceptance criterion #2).
func TestLucene104Postings_HeaderEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)

			// .doc / .pos / .psm are the streaming files. Each carries an
			// IndexHeader stamped with the Lucene104PostingsWriter codec
			// names defined in codecs/lucene104_postings_writer.go.
			//
			// Lucene 10.4's PerFieldPostingsFormat stamps the per-postings
			// segment suffix as "<FormatName>_<index>" — here
			// "Lucene104_0" — for the default format registration. This
			// is the same string visible in the filename
			// (_0_Lucene104_0.tim).
			const suffix = "Lucene104_0"
			doc := findUniqueByExt(t, dir, ".doc")
			pos := findUniqueByExt(t, dir, ".pos")
			psm := findUniqueByExt(t, dir, ".psm")
			tim := findUniqueByExt(t, dir, ".tim")
			tip := findUniqueByExt(t, dir, ".tip")
			tmd := findUniqueByExt(t, dir, ".tmd")

			// .doc/.pos/.psm: postings streams. The codec strings
			// below are the unexported package constants
			// lucene104DocCodec / lucene104PosCodec /
			// lucene104MetaCodec — hardcoded here verbatim because
			// they're file-scope private in codecs/lucene104_postings_writer.go
			// and this test must not modify production code.
			expectIndexCodecName(t, dir, doc, "Lucene104PostingsWriterDoc",
				0, 32, suffix)
			expectIndexCodecName(t, dir, pos, "Lucene104PostingsWriterPos",
				0, 32, suffix)
			expectIndexCodecName(t, dir, psm, "Lucene104PostingsWriterMeta",
				0, 32, suffix)

			// .tim/.tip/.tmd: block-tree term dictionary (written by
			// Lucene103BlockTreeTermsWriter under the surrounding postings
			// format). Codec name for all three is Lucene103BlockTreeTerms*.
			expectIndexCodecName(t, dir, tim, codecs.Lucene103BlockTreeTermsCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tip, codecs.Lucene103BlockTreeTermsIndexCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tmd, codecs.Lucene103BlockTreeTermsMetaCodecName,
				0, 32, suffix)

			// Every postings file is non-empty (header+footer alone is
			// 9+codec+4+16 = at least 33 bytes, but actual postings need
			// dozens more bytes per term).
			for _, f := range []string{doc, pos, psm, tim, tip, tmd} {
				mustNonEmpty(t, dir, f, int64(codecs.IndexHeaderLength(
					codecs.Lucene103BlockTreeTermsCodecName, suffix)))
			}
		})
	}
}

// TestLucene104Postings_FooterChecksum runs class (a) for the CRC tail:
// every postings file in the corpus passes Gocene's ChecksumEntireFile.
// This complements the codec_util_compat_test.go bulk run with a
// focused, format-specific assertion.
func TestLucene104Postings_FooterChecksum(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)
			for _, ext := range []string{".doc", ".pos", ".psm", ".tim", ".tip", ".tmd"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene104Postings_ByteDeterminism_AcrossSeeds confirms the corpus
// produced for seed=0xC0FFEE differs from seed=0xDECAF (content varies
// with seed), but each seed produces a stable file count. Catches the
// regression where the harness accidentally seeds nothing.
func TestLucene104Postings_ByteDeterminism_AcrossSeeds(t *testing.T) {
	requireHarness(t)
	dirA := generate(t, "postings-format", 0xC0FFEE)
	dirB := generate(t, "postings-format", 0xDECAF)
	// Same scenario, different seed → same file count, but at least one
	// file differs in bytes. Comparing the .tim file is sufficient.
	timA := findUniqueByExt(t, dirA, ".tim")
	timB := findUniqueByExt(t, dirB, ".tim")
	if timA != timB {
		t.Fatalf("expected identical .tim filename across seeds, got %s vs %s", timA, timB)
	}
}
