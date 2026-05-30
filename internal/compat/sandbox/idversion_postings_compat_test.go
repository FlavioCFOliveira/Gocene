// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// idversion_postings_compat_test.go addresses the sandbox audit row for
// IDVersionPostingsFormat (verbatim from the rmp 4631 task contract):
// "IDVersionPostingsFormat: Pure port without tests, fixtures, or writer
// parity". Three classes per the contract: (a) read-fixture, (b) byte-
// determinism + verify-sandbox idversion CLI, (c) round-trip Skip.
package sandbox

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// luceneCodecUtilIndexHeaderMagic is the BE int32 0x3FD76C17 that prefixes
// every CodecUtil-framed Lucene file. Used as a cheap structural assertion
// against the IDVersion-suffixed terms file.
var luceneCodecUtilIndexHeaderMagic = []byte{0x3F, 0xD7, 0x6C, 0x17}

// auditGapIDVersion is the Sprint 114 T23 task-contract reframing of the
// docs/compat-coverage.tsv audit row "No tests at all for the IDVersion
// postings port." — used verbatim by the deferred / round-trip Skip
// messages so the row is visible in `go test -v` output.
const auditGapIDVersion = "IDVersionPostingsFormat: Pure port without tests, fixtures, or writer parity"

// TestSandboxIDVersionPostings_ReadFixture (class a) pins the structural
// shape: the IDVersion-suffixed terms (_0_IDVersion_0.tiv) and
// terms-index (_0_IDVersion_0.tipv) files must both exist; the terms
// file must start with CodecUtil header magic and be non-trivial. The
// "_IDVersion_" suffix proves PerFieldPostingsFormat dispatched `id` to
// IDVersionPostingsFormat (not the default Lucene104PostingsFormat).
func TestSandboxIDVersionPostings_ReadFixture(t *testing.T) {
	const minTermsBytes = 32 // CodecUtil header + footer floor.
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSandboxIDVersionPostings, seed)
			files := listFiles(t, dir)
			if !containsFile(files, fileIDVersionTerms) {
				t.Fatalf("expected %q under fixture dir, files=%v",
					fileIDVersionTerms, files)
			}
			if !containsFile(files, fileIDVersionTermsIndex) {
				t.Fatalf("expected %q under fixture dir, files=%v",
					fileIDVersionTermsIndex, files)
			}
			terms := readFileBytes(t, dir, fileIDVersionTerms)
			if len(terms) <= minTermsBytes {
				t.Fatalf("%s suspiciously small (%d bytes); CodecUtil framing alone "+
					"should comfortably exceed %d",
					fileIDVersionTerms, len(terms), minTermsBytes)
			}
			if !bytes.HasPrefix(terms, luceneCodecUtilIndexHeaderMagic) {
				t.Errorf("%s does not start with CodecUtil IndexHeader magic %x; got prefix %x",
					fileIDVersionTerms, luceneCodecUtilIndexHeaderMagic, terms[:4])
			}
		})
	}
}

// TestSandboxIDVersionPostings_ByteDeterminism (class b, part 1)
// re-runs the scenario at the same seed and asserts the IDVersion files
// are byte-identical — catches IDVersionPostingsWriter iteration drift
// and any header-id non-determinism upstream of Determinism.seed.
func TestSandboxIDVersionPostings_ByteDeterminism(t *testing.T) {
	files := []string{fileIDVersionTerms, fileIDVersionTermsIndex}
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioSandboxIDVersionPostings, seed)
			b := generate(t, ScenarioSandboxIDVersionPostings, seed)
			for _, name := range files {
				ab := readFileBytes(t, a, name)
				bb := readFileBytes(t, b, name)
				if !bytes.Equal(ab, bb) {
					t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
						name, seed, len(ab), len(bb))
				}
			}
		})
	}
}

// TestSandboxIDVersionPostings_VerifySubcommand (class b, part 2)
// drives `verify-sandbox idversion`. Clean exit proves the Java
// verifier reopens the index, builds IDVersionSegmentTermsEnum, and
// asserts every (id, version) probe — at the exact version AND at
// version+1 (must fail) — matches the seed expectation.
func TestSandboxIDVersionPostings_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSandboxIDVersionPostings, seed)
			out, err := runHarness(t, "verify-sandbox", "idversion", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-sandbox idversion failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-sandbox variant=idversion") {
				t.Errorf("expected 'ok verify-sandbox variant=idversion' in stdout, got: %s", out)
			}
		})
	}
}

// TestSandboxIDVersionPostings_RoundTrip (class c) — full L -> G -> L
// replay is blocked on Gocene's sandbox/codecs/idversion port: the
// reader/writer/segment-terms-enum types exist but no end-to-end
// binary-parity gate. The Java-side verifier IS exercised by class b
// (TestSandboxIDVersionPostings_VerifySubcommand).
func TestSandboxIDVersionPostings_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene sandbox/codecs/idversion port — "+
				"the reader/writer/segment-terms-enum types exist "+
				"(sandbox/codecs/idversion/*.go) but the package ships "+
				"no end-to-end binary-parity gate; Gocene-produced segments "+
				"have never been read back by Lucene's IDVersionPostingsFormat "+
				"nor the other way round. The Lucene-side verifier IS "+
				"exercised by TestSandboxIDVersionPostings_VerifySubcommand. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioSandboxIDVersionPostings, seed, auditGapIDVersion)
		})
	}
}
