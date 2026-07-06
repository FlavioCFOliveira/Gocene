// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// packed64_legacy_compat_test.go addresses the backward_codecs audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Legacy Packed64 / Packed64SingleBlock
//	    lucene_class:  org.apache.lucene.backward_codecs.packed.LegacyPacked64
//	    gocene_class:  backward_codecs/packed/legacy_packed64.go
//	    isolated:      yes:backward_codecs/packed/legacy_packed64_test.go
//	    integration:   yes:backward_codecs/packed/test_legacy_direct_monotonic_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene fixture; covered by self-roundtrip only."
//
// Scenario "bwc-packed64-legacy" writes a seeded array of values at every
// supported bitsPerValue (1..64 from LegacyDirectWriter.SUPPORTED_BITS_PER_VALUE)
// into a single big-endian file via LegacyDirectWriter wrapped through
// EndiannessReverserUtil — matching the canonical wire used by Lucene
// versions prior to the 8.6 LE flip. The Java verifier re-reads every
// value via LegacyDirectReader and asserts equality.
//
// Three test classes per the rmp 4634 contract:
//
//	(a) read-fixture     — Lucene-generated payload exists and the byte
//	                        layout is stable across two runs at the same seed.
//	(b) write-and-verify — Deferred: Gocene's backward_codecs/packed/ does
//	                        not yet implement the EndiannessReverser-wrapped
//	                        write path (the Go port covers the reader and
//	                        the self-roundtrip writer test, not the BE
//	                        bytestream wrapper).
//	(c) round-trip       — Deferred for the same reason.
package backward_codecs

import (
	"path/filepath"
	"testing"
)

// TestPacked64Legacy_ReadFixture (class a) drives the harness and asserts
// the resulting fixture carries the expected single-file shape.
func TestPacked64Legacy_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcPacked64Legacy, seed)
			path := filepath.Join(dir, fileBwcPacked64Legacy)
			if !hasFile(t, dir, fileBwcPacked64Legacy) {
				t.Fatalf("expected %s in %s (LegacyDirectWriter payload missing)", path, dir)
			}
			assertDigestStable(t, ScenarioBwcPacked64Legacy, seed)
		})
	}
}

// TestPacked64Legacy_VerifySubcommand (class b harness leg) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier read every value via LegacyDirectReader.
func TestPacked64Legacy_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcPacked64Legacy, seed)
			verifyHarness(t, ScenarioBwcPacked64Legacy, seed, dir)
		})
	}
}

// TestPacked64Legacy_WriteAndVerify (class b, Gocene-side leg) is deferred:
// backward_codecs/packed/legacy_packed64.go ships only the reader half
// plus a self-roundtrip writer test; the big-endian bytestream wrapper
// (Gocene mirror of org.apache.lucene.backward_codecs.store.
// EndiannessReverserUtil) is not yet wired to a LegacyDirectWriter in Go.
// Gocene self-roundtrip coverage exists in the backward_codecs/packed/
// package itself; the cross-engine leg requires the BE bytestream writer.
func TestPacked64Legacy_WriteAndVerify(t *testing.T) {
	const auditGap = "No Lucene fixture; covered by self-roundtrip only."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene backward_codecs/packed has no public "+
				"EndiannessReverser-wrapped writer yet "+
				"(backward_codecs/store/store.go); seed=%d; "+
				"audit gap_notes (verbatim): %q", seed, auditGap)
		})
	}
}

// TestPacked64Legacy_RoundTrip (class c) is deferred for the same reason
// as the write-and-verify leg: no EndiannessReverser-wrapped writer exists
// in backward_codecs/packed/ for LegacyDirectWriter.
func TestPacked64Legacy_RoundTrip(t *testing.T) {
	const auditGap = "No Lucene fixture; covered by self-roundtrip only."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for bwc-packed64-legacy at "+
				"seed=%d requires a BE-wrapped writer in "+
				"backward_codecs/store; audit gap_notes (verbatim): %q",
				seed, auditGap)
		})
	}
}
