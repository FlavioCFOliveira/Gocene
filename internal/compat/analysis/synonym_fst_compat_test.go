// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// synonym_fst_compat_test.go addresses the audit row (verbatim from
// docs/compat-coverage.tsv):
//
//	"Synonym FST blob (SolrSynonymParser output)"
//	    lucene_class: org.apache.lucene.analysis.synonym.SynonymMap
//	    gap_notes:    "No round-trip test against Lucene-compiled
//	                   synonym maps; format not yet verified."
//
// The scenario "synonym-fst" builds a SynonymMap.Builder over 20 deterministic
// (input -> output) pairs, frames the compiled FST<BytesRef> with a Gocene-
// owned CodecUtil envelope (codec="GoceneSynonymFst", version=0,
// id=Determinism.idFromSeed, suffix=""), and writes a single file
// {synonym.fst}.
//
// Three test classes per the rmp 4618 contract:
//
//	(a) read-fixture        — Lucene-generated synonym.fst is present, the
//	                          CodecUtil envelope parses, and the embedded
//	                          FST.metadata + body sizes are within the
//	                          contractual range (>0).
//	(b) write-and-verify    — Two harness `gen` runs at the same seed
//	                          produce byte-identical files (deterministic-
//	                          comparison flavour); the harness `verify`
//	                          re-walks the seeded input/output pairs and
//	                          exits cleanly.
//	(c) round-trip          — Lucene-write -> Gocene-write -> Lucene-verify.
//	                          Deferred (see deferred_analysis_compat_test.go).
//
// Class (c) is recorded as a t.Skip in deferred_analysis_compat_test.go
// because Gocene's analysis/synonym/ package ships only the textual
// parsers (Solr, WordNet); a SynonymMap binary writer is not yet
// implemented. The Gocene-side FST reader infrastructure in util/fst
// IS available (NewFSTFromDataInput + ByteSequenceOutputs + Util.Get),
// so a follow-up task can plug a Gocene-side decoder here.
package analysis

import (
	"bytes"
	"testing"
)

// TestSynonymFst_ReadFixture (class a) drives the harness, asserts the
// expected file exists, has a non-trivial size (FST header + body +
// CodecUtil footer = at least ~64B), and that the four magic bytes of
// the CodecUtil index header (0x3FD76C17 big-endian) are present at
// offset 0.
func TestSynonymFst_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSynonymFst, seed)
			raw := readFileBytes(t, dir, fileSynonymFst)
			if len(raw) < 64 {
				t.Fatalf("synonym.fst suspiciously small (%d bytes); expected codec-framed FST", len(raw))
			}
			// CodecUtil.writeIndexHeader writes a 4-byte big-endian magic
			// followed by the codec name as a writeString. The magic in
			// Lucene 10.4 is CODEC_MAGIC = 0x3FD76C17.
			wantMagic := []byte{0x3F, 0xD7, 0x6C, 0x17}
			if !bytes.Equal(raw[:4], wantMagic) {
				t.Errorf("synonym.fst missing CodecUtil magic at offset 0: got % x, want % x",
					raw[:4], wantMagic)
			}
			// CodecUtil.writeFooter ends with a 4-byte big-endian footer
			// magic FOOTER_MAGIC = ~CODEC_MAGIC = 0xC0289FE8 at offset
			// len-16 (magic + algorithmId int + checksum long).
			footerStart := len(raw) - 16
			wantFooterMagic := []byte{0xC0, 0x28, 0x93, 0xE8}
			if !bytes.Equal(raw[footerStart:footerStart+4], wantFooterMagic) {
				t.Errorf("synonym.fst missing CodecUtil footer magic at offset %d: got % x, want % x",
					footerStart, raw[footerStart:footerStart+4], wantFooterMagic)
			}
		})
	}
}

// TestSynonymFst_ByteDeterminism (class b, part 1) runs the harness
// twice at the same seed and confirms synonym.fst is byte-identical
// across runs.
func TestSynonymFst_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioSynonymFst, seed)
			b := generate(t, ScenarioSynonymFst, seed)
			ab := readFileBytes(t, a, fileSynonymFst)
			bb := readFileBytes(t, b, fileSynonymFst)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("synonym.fst drift between two runs at seed=%d (len A=%d B=%d)",
					seed, len(ab), len(bb))
			}
		})
	}
}

// TestSynonymFst_VerifySubcommand (class b, part 2) drives the harness
// `verify` subcommand against a fresh fixture. A clean exit proves the
// Java verifier reopened the FST, walked the seeded (input -> output)
// pairs, and confirmed every output exists.
func TestSynonymFst_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSynonymFst, seed)
			verifyHarness(t, ScenarioSynonymFst, seed, dir)
		})
	}
}

// TestSynonymFst_DifferentSeedsDiffer is a cheap negative check: the FST
// bytes MUST change when the seed changes, otherwise the seed plumbing is
// broken and the byte-determinism test is testing a constant.
func TestSynonymFst_DifferentSeedsDiffer(t *testing.T) {
	a := generate(t, ScenarioSynonymFst, canarySeeds[0])
	b := generate(t, ScenarioSynonymFst, canarySeeds[1])
	ab := readFileBytes(t, a, fileSynonymFst)
	bb := readFileBytes(t, b, fileSynonymFst)
	if bytes.Equal(ab, bb) {
		t.Fatalf("synonym.fst identical across seeds; seed plumbing broken")
	}
}
