// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// facet_set_compat_test.go addresses the facets audit row (verbatim
// from docs/compat-coverage.tsv):
//
//	facets	FacetSet packed-bytes encoding
//	    lucene_class:  org.apache.lucene.facet.facetset.FacetSet
//	    gocene_class:  facets/facetset/facet_set.go
//	    isolated:      yes:facets/facetset/facet_set_decoder_test.go
//	    integration:   yes:facets/facetset/matching_facet_sets_counts_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-produced FacetSet bytes used in tests."
//
// The scenario "facet-set-packed-bytes" indexes 6 docs, each carrying
// two 3-dimensional LongFacetSets packed into a single FacetSetsField
// BinaryDocValues entry. The (long, long, long) tuples are packed BE per
// the canonical Lucene 10.4.0 layout decoded by
// FacetSetDecoder.decodeLongs. The verifier reopens the index and uses
// MatchingFacetSetsCounts + ExactFacetSetMatcher to count the canonical
// (1, 2, 3) packed tuple; at least one matching doc must be returned.
//
// Three test classes per the rmp 4620 contract:
//
//	(a) read-fixture     — Lucene-generated segment exposes the expected
//	                        BinaryDocValues file shape (.dvd + .dvm) plus
//	                        the canonical segment-info bookkeeping.
//	(b) write-and-verify — Two `gen` runs at the same seed produce
//	                        byte-identical non-.si files; the harness
//	                        `verify` subcommand reopens the index and
//	                        confirms the ExactFacetSetMatcher count
//	                        equals NUM_DOCS (every doc carries the
//	                        anchor).
//	(c) round-trip       — Lucene-write -> Gocene-read -> Lucene-verify.
//	                        Deferred (see deferred_facets_compat_test.go)
//	                        because Gocene's FacetSetDecoder cannot yet
//	                        consume a Lucene-emitted BinaryDocValues
//	                        stream (the SegmentReader core-readers gap
//	                        recorded under memory-index reference
//	                        'gocene-segmentreader-corereaders-gap').
package facets

import (
	"bytes"
	"testing"
)

// TestFacetSetPackedBytes_ReadFixture (class a) drives the harness and
// asserts the resulting directory carries the BinaryDocValues codec
// shape (.dvd, .dvm) plus the field-infos descriptor and segment-info
// bookkeeping.
func TestFacetSetPackedBytes_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSetPackedBytes, seed)
			if !hasFileWithSuffix(t, dir, ".dvd") {
				t.Errorf("expected .dvd in %s (BinaryDocValues data missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".dvm") {
				t.Errorf("expected .dvm in %s (BinaryDocValues meta missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".fnm") {
				t.Errorf("expected .fnm in %s (field infos missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".si") {
				t.Errorf("expected .si in %s (segment info missing)", dir)
			}
		})
	}
}

// TestFacetSetPackedBytes_DigestDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms the non-.si files are
// byte-identical. The packed-bytes encoding multiplexes a 4-byte
// dim-count prefix followed by N x sizePackedBytes per set; any drift
// in the per-set packing surfaces here.
func TestFacetSetPackedBytes_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioFacetSetPackedBytes, seed)
			b := generate(t, ScenarioFacetSetPackedBytes, seed)
			ma := fileMapRecursive(t, a)
			mb := fileMapRecursive(t, b)
			if len(ma) != len(mb) {
				t.Fatalf("file count mismatch: A=%d B=%d", len(ma), len(mb))
			}
			for name, ba := range ma {
				bb, ok := mb[name]
				if !ok {
					t.Errorf("file %q present in A but missing from B", name)
					continue
				}
				if !bytes.Equal(ba, bb) {
					t.Errorf("file %q content drift between two runs at seed=%d", name, seed)
				}
			}
		})
	}
}

// TestFacetSetPackedBytes_VerifySubcommand (class b, part 2) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reopened the index, decoded the FacetSet
// packed-bytes stream through MatchingFacetSetsCounts +
// ExactFacetSetMatcher, and confirmed the anchor tuple count equals
// NUM_DOCS (every doc carries it).
func TestFacetSetPackedBytes_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSetPackedBytes, seed)
			verifyHarness(t, ScenarioFacetSetPackedBytes, seed, dir)
		})
	}
}

// TestFacetSetPackedBytes_RoundTrip (class c) is the full Lucene ->
// Gocene -> Lucene loop. Deferred: Gocene's FacetSetDecoder (see
// facets/facetset/facet_set_decoder.go) cannot yet consume a Lucene-
// emitted BinaryDocValues stream — the SegmentReader core-readers gap
// blocks the leaf-reader path before the decoder is reached. Recorded
// verbatim in deferred_facets_compat_test.go.
func TestFacetSetPackedBytes_RoundTrip(t *testing.T) {
	const auditGap = "No Lucene-produced FacetSet bytes used in tests."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for facet-set-packed-bytes at seed=%d "+
				"is blocked on the SegmentReader core-readers gap "+
				"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
				"audit gap_notes (verbatim): %q",
				seed, auditGap)
		})
	}
}
