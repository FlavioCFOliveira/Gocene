// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// big_endian_store_compat_test.go addresses the backward_codecs audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Legacy big-endian store wrappers
//	    lucene_class:  org.apache.lucene.backward_codecs.store.EndiannessReverserUtil
//	    gocene_class:  backward_codecs/store/store.go
//	    isolated:      yes:backward_codecs/store/store_test.go
//	    integration:   yes:backward_codecs/store/test_endianness_reverser_checksum_index_input_test.go
//	    binary_compat: no
//	    gap_notes:     "No fixture from an old big-endian Lucene index."
//
// Scenario "bwc-big-endian-store" writes a tiny BE-framed payload (magic
// + version int + count + 16 records of short/int/long/string) via
// EndiannessReverserUtil.createOutput. The Java verifier re-opens through
// the reverser, reads every record, AND opens a SECOND time WITHOUT the
// reverser to prove the version int's raw LE-interpretation equals
// Integer.reverseBytes(VERSION) — i.e. that the wrapper actually emitted
// big-endian bytes rather than being a no-op.
//
// Three test classes per the rmp 4634 contract:
//
//	(a) read-fixture     — Lucene-generated payload exists and the byte
//	                        layout is stable across two runs at the same seed.
//	(b) write-and-verify — Deferred: backward_codecs/store/store.go exposes
//	                        EndiannessReverser*Input/Output as types but does
//	                        NOT yet expose a Directory-level openInput /
//	                        createOutput entry point on a wired-up Lucene-
//	                        compatible Directory wrapper.
//	(c) round-trip       — Deferred for the same reason.
package backward_codecs

import (
	"path/filepath"
	"testing"
)

// TestBigEndianStore_ReadFixture (class a) drives the harness and asserts
// the resulting fixture carries the expected single-file shape.
func TestBigEndianStore_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcBigEndianStore, seed)
			path := filepath.Join(dir, fileBwcBigEndianStore)
			if !hasFile(t, dir, fileBwcBigEndianStore) {
				t.Fatalf("expected %s in %s (BE store payload missing)", path, dir)
			}
			assertDigestStable(t, ScenarioBwcBigEndianStore, seed)
		})
	}
}

// TestBigEndianStore_VerifySubcommand (class b, harness leg) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier read the records back through the reverser
// AND confirmed the on-disk bytes are big-endian.
func TestBigEndianStore_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcBigEndianStore, seed)
			verifyHarness(t, ScenarioBwcBigEndianStore, seed, dir)
		})
	}
}

// TestBigEndianStore_WriteAndVerify (class b, Gocene-side leg) would have
// Gocene write its own bwc-big-endian-store.dat and re-verify with the
// Java harness. Deferred: backward_codecs/store/store.go's
// EndiannessReverser primitives exist as Go types but no Directory-level
// constructors are wired to a Gocene Directory implementation yet, so
// Gocene cannot emit a Lucene-compatible BE-framed .dat file end-to-end.
func TestBigEndianStore_WriteAndVerify(t *testing.T) {
	const auditGap = "No fixture from an old big-endian Lucene index."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene backward_codecs/store has no Directory-"+
				"wired BE wrapper yet (backward_codecs/store/store.go); seed=%d; "+
				"audit gap_notes (verbatim): %q", seed, auditGap)
		})
	}
}

// TestBigEndianStore_RoundTrip (class c) is the full Lucene -> Gocene ->
// Lucene loop. Deferred for the same reason as the write-and-verify leg.
func TestBigEndianStore_RoundTrip(t *testing.T) {
	const auditGap = "No fixture from an old big-endian Lucene index."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for bwc-big-endian-store at "+
				"seed=%d requires a Directory-wired BE wrapper in "+
				"backward_codecs/store; audit gap_notes (verbatim): %q",
				seed, auditGap)
		})
	}
}
