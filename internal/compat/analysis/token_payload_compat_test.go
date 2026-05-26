// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// token_payload_compat_test.go addresses the audit row (verbatim from
// docs/compat-coverage.tsv):
//
//	"Token payload byte serialisation"
//	    lucene_class: org.apache.lucene.analysis.payloads.PayloadHelper
//	    gap_notes:    "No Lucene-side parity test for payload byte layout."
//
// The scenario "token-payload-bytes" indexes a 5-doc / 6-tokens-per-doc
// corpus where every token carries a deterministic 4-byte payload
// (seed XOR (docId*31 + tokenIndex), little-endian). The payloads are
// persisted into the segment's .pos / .pay files of the Lucene 10.4
// default codec.
//
// Three test classes per the rmp 4618 contract:
//
//	(a) read-fixture        — Lucene-generated segment exists, contains
//	                          the .pos / .pay files that carry payloads
//	                          (postings format Lucene104PostingsFormat),
//	                          and at least one .si segment-info file is
//	                          present (segment was committed).
//	(b) write-and-verify    — The harness `verify` subcommand reopens the
//	                          index, walks every term, and asserts the
//	                          payload bytes match the seeded expectation
//	                          byte-for-byte. Determinism is enforced via
//	                          ScenarioDeterminismTest on the Java side
//	                          (digest equality across two `gen` runs at
//	                          the same seed); the Go test cross-checks
//	                          that the per-file shapes are stable.
//	(c) round-trip          — Gocene-write -> Lucene-verify. Deferred (see
//	                          deferred_analysis_compat_test.go) because
//	                          Gocene's IndexWriter is not yet end-to-end
//	                          for payloaded postings (see memory-index
//	                          reference 'gocene-freqprox-port' / backlog).
package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTokenPayload_ReadFixture (class a) drives the harness, asserts
// the resulting directory has the expected segment-info + postings
// (.pos / .pay) shape and that the IndexCorpusScenario contract is
// honoured (single committed segment, files present).
func TestTokenPayload_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTokenPayload, seed)
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("readdir %s: %v", dir, err)
			}
			var hasSI, hasPos, hasPay bool
			for _, e := range entries {
				name := e.Name()
				switch {
				case strings.HasSuffix(name, ".si"):
					hasSI = true
				case strings.HasSuffix(name, ".pos"):
					hasPos = true
				case strings.HasSuffix(name, ".pay"):
					hasPay = true
				}
			}
			if !hasSI {
				t.Errorf("expected at least one .si file in fixture dir")
			}
			if !hasPos {
				t.Errorf("expected at least one .pos file (positions enabled with payloads)")
			}
			if !hasPay {
				t.Errorf("expected at least one .pay file (payload stream)")
			}
		})
	}
}

// TestTokenPayload_DigestDeterminism (class b, part 1) runs the harness
// twice at the same seed and compares the normalised file shape. We
// cannot compare raw bytes across the two runs because Lucene's .si
// stamps a wall-clock value into its diagnostics map — that's why the
// Manifest snapshot excludes .si. Here we use the same exclusion: we
// compare the (filename, size) tuples of every non-.si file. Identical
// tuples + identical content-by-name implies determinism.
func TestTokenPayload_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioTokenPayload, seed)
			b := generate(t, ScenarioTokenPayload, seed)
			ma := fileMap(t, a)
			mb := fileMap(t, b)
			if len(ma) != len(mb) {
				t.Fatalf("file count mismatch: A=%d B=%d", len(ma), len(mb))
			}
			for name, ba := range ma {
				bb, ok := mb[name]
				if !ok {
					t.Errorf("file %q present in A but missing from B", name)
					continue
				}
				if len(ba) != len(bb) {
					t.Errorf("file %q size mismatch: A=%d B=%d", name, len(ba), len(bb))
					continue
				}
				// Use string comparison for compactness; bytes.Equal would
				// allocate the same on a copy. The shared content path is
				// 100s of KB at most.
				if string(ba) != string(bb) {
					t.Errorf("file %q content drift between two runs at seed=%d", name, seed)
				}
			}
		})
	}
}

// TestTokenPayload_VerifySubcommand (class b, part 2) drives the harness
// `verify` subcommand against a fresh fixture. A clean exit proves the
// Java verifier reopened the index with PostingsEnum.PAYLOADS, walked
// every term, and confirmed every payload byte against the seeded
// expectation.
func TestTokenPayload_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTokenPayload, seed)
			verifyHarness(t, ScenarioTokenPayload, seed, dir)
		})
	}
}

// fileMap reads every non-.si regular file in dir into a map keyed by
// filename. The .si exclusion mirrors Manifest.includeForHash: Lucene
// stamps a wall-clock timestamp into the .si diagnostics map and we
// must not let that drift contaminate determinism checks.
func fileMap(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	out := make(map[string][]byte, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".si") || name == "write.lock" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s/%s: %v", dir, name, err)
		}
		out[name] = b
	}
	return out
}
