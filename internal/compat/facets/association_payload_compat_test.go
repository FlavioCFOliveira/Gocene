// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// association_payload_compat_test.go addresses the facets audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	facets	FacetField association payload encoding
//	    lucene_class:
//	        org.apache.lucene.facet.taxonomy.AssociationFacetField
//	    gocene_class:  facets/taxonomy/association_facet_field.go
//	    isolated:      yes:facets/taxonomy/association_aggregation_function_test.go
//	    integration:   partial:facets/taxonomy/test_taxonomy_facet_associations_test.go
//	    binary_compat: no
//	    gap_notes:     "No byte-level fixture for association payloads."
//
// The scenario "facet-association-payload" indexes 8 docs, each carrying
// an Int and a Float AssociationFacetField. FacetsConfig flattens the
// dim/value tuples into BinaryDocValues fields ($facets.int /
// $facets.float) — with compound files disabled the association payload
// bytes are exposed in the segment's binary doc-values files (.dvd/.dvm).
// The verifier reopens the index and asserts every (ord, value) pair
// against the seeded expectation byte-for-byte.
//
// Three test classes per the rmp 4620 contract:
//
//	(a) read-fixture     — Lucene-generated segment exposes the expected
//	                        Lucene104 / Lucene90DocValues file extensions
//	                        (.dvd + .dvm + .fnm + .si + segments_N), and
//	                        the taxonomy sidecar exists under taxo/.
//	(b) write-and-verify — Two `gen` runs at the same seed produce
//	                        byte-identical non-.si files; the harness
//	                        `verify` subcommand reopens the index, walks
//	                        $facets.int / $facets.float, and confirms
//	                        every (ord, int|float) pair matches the
//	                        seeded expectation.
//	(c) round-trip       — Lucene-write -> Gocene-read -> Lucene-verify.
//	                        Deferred (see deferred_facets_compat_test.go)
//	                        because Gocene's association decoders cannot
//	                        yet consume a Lucene-emitted BinaryDocValues
//	                        stream (the SegmentReader core-readers gap
//	                        recorded under memory-index reference
//	                        'gocene-segmentreader-corereaders-gap').
package facets

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestFacetAssociationPayload_ReadFixture (class a) drives the harness
// and asserts the resulting directory carries the expected codec shape:
// the BinaryDocValues files (.dvd, .dvm) that host the packed association
// payloads, the field-infos descriptor, and the taxonomy sidecar.
func TestFacetAssociationPayload_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetAssociationPayload, seed)
			if !hasFileWithSuffix(t, dir, ".dvd") {
				t.Errorf("expected .dvd in %s (BinaryDocValues data missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".dvm") {
				t.Errorf("expected .dvm in %s (BinaryDocValues meta missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".fnm") {
				t.Errorf("expected .fnm in %s (field infos missing)", dir)
			}
			taxoDir := filepath.Join(dir, taxoSubdir)
			if !hasFileWithSuffix(t, taxoDir, ".si") {
				t.Errorf("expected taxonomy sidecar at %s with .si", taxoDir)
			}
		})
	}
}

// TestFacetAssociationPayload_DigestDeterminism (class b, part 1) runs
// the scenario twice at the same seed and confirms the non-.si files
// are byte-identical recursively across both the main index and the
// taxonomy sidecar.
func TestFacetAssociationPayload_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioFacetAssociationPayload, seed)
			b := generate(t, ScenarioFacetAssociationPayload, seed)
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

// TestFacetAssociationPayload_VerifySubcommand (class b, part 2) drives
// the harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reopened the index, walked
// $facets.int / $facets.float, and confirmed every (ord, value) pair
// matches the seeded expectation byte-for-byte.
func TestFacetAssociationPayload_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetAssociationPayload, seed)
			verifyHarness(t, ScenarioFacetAssociationPayload, seed, dir)
		})
	}
}

// TestFacetAssociationPayload_RoundTrip (class c) is the full Lucene ->
// Gocene -> Lucene loop. Deferred: Gocene's association decoders
// (facets/taxonomy/association_facet_field.go) cannot yet consume a
// Lucene-emitted BinaryDocValues stream — the SegmentReader core-readers
// gap blocks the leaf-reader path before the decoders are reached.
// Recorded verbatim in deferred_facets_compat_test.go.
func TestFacetAssociationPayload_RoundTrip(t *testing.T) {
	const auditGap = "No byte-level fixture for association payloads."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for facet-association-payload at seed=%d "+
				"is blocked on the SegmentReader core-readers gap "+
				"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
				"audit gap_notes (verbatim): %q",
				seed, auditGap)
		})
	}
}
