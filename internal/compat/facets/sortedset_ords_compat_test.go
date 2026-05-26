// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// sortedset_ords_compat_test.go addresses the facets audit row (verbatim
// from docs/compat-coverage.tsv):
//
//	facets	SortedSetDocValues facet ord encoding
//	    lucene_class:
//	        org.apache.lucene.facet.sortedset.DefaultSortedSetDocValuesReaderState
//	    gocene_class:  facets/sortedset/default_sorted_set_doc_values_reader_state.go
//	    isolated:      yes:facets/sortedset/default_sorted_set_doc_values_reader_state_test.go
//	    integration:   partial:facets/sortedset/sorted_set_doc_values_facets_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-emitted sorted-set ord file consumed by tests."
//
// The scenario "facet-sortedset-ords" indexes 6 docs over two
// (dim, value) pairs stored as SortedSetDocValuesFacetField. FacetsConfig
// flattens the dim/value tuples into "dim/value" terms in a
// SortedSetDocValues field. The verifier reopens the index through
// DefaultSortedSetDocValuesReaderState and asserts every dim has a
// non-empty OrdRange.
//
// Three test classes per the rmp 4620 contract:
//
//	(a) read-fixture     — Lucene-generated segment exposes the expected
//	                        SortedSetDocValues file shape (.dvd + .dvm +
//	                        .fnm + .si + segments_N).
//	(b) write-and-verify — Two `gen` runs at the same seed produce
//	                        byte-identical non-.si files; the harness
//	                        `verify` subcommand reopens the index through
//	                        DefaultSortedSetDocValuesReaderState and
//	                        re-asserts the ord encoding for every
//	                        configured dim.
//	(c) round-trip       — Lucene-write -> Gocene-read -> Lucene-verify.
//	                        Deferred (see deferred_facets_compat_test.go)
//	                        because Gocene's
//	                        DefaultSortedSetDocValuesReaderState cannot
//	                        yet open a Lucene-emitted SortedSetDocValues
//	                        stream (the SegmentReader core-readers gap
//	                        recorded under memory-index reference
//	                        'gocene-segmentreader-corereaders-gap').
package facets

import (
	"bytes"
	"testing"
)

// TestFacetSortedsetOrds_ReadFixture (class a) drives the harness and
// asserts the directory carries the expected SortedSetDocValues codec
// shape (.dvd / .dvm) plus the canonical segment-info bookkeeping.
func TestFacetSortedsetOrds_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSortedsetOrds, seed)
			if !hasFileWithSuffix(t, dir, ".dvd") {
				t.Errorf("expected .dvd in %s (SortedSetDocValues data missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".dvm") {
				t.Errorf("expected .dvm in %s (SortedSetDocValues meta missing)", dir)
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

// TestFacetSortedsetOrds_DigestDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms the non-.si files are
// byte-identical. SortedSetDocValues ord assignment is sensitive to
// term insertion order; this gate proves it is fully seed-driven.
func TestFacetSortedsetOrds_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioFacetSortedsetOrds, seed)
			b := generate(t, ScenarioFacetSortedsetOrds, seed)
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

// TestFacetSortedsetOrds_VerifySubcommand (class b, part 2) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reopened the index through
// DefaultSortedSetDocValuesReaderState and re-asserted the ord encoding
// for every configured dim.
func TestFacetSortedsetOrds_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSortedsetOrds, seed)
			verifyHarness(t, ScenarioFacetSortedsetOrds, seed, dir)
		})
	}
}

// TestFacetSortedsetOrds_RoundTrip (class c) is the full Lucene ->
// Gocene -> Lucene loop. Deferred: Gocene's
// DefaultSortedSetDocValuesReaderState (see
// facets/sortedset/default_sorted_set_doc_values_reader_state.go) cannot
// yet open a Lucene-emitted SortedSetDocValues stream — the
// SegmentReader core-readers gap blocks the leaf-reader path before the
// reader state is built. Recorded verbatim in
// deferred_facets_compat_test.go.
func TestFacetSortedsetOrds_RoundTrip(t *testing.T) {
	const auditGap = "No Lucene-emitted sorted-set ord file consumed by tests."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for facet-sortedset-ords at seed=%d "+
				"is blocked on the SegmentReader core-readers gap "+
				"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
				"audit gap_notes (verbatim): %q",
				seed, auditGap)
		})
	}
}
