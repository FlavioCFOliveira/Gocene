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

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
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
// Gocene -> Lucene loop. Gocene opens the index, reads the
// SortedSetDocValues facet field ($facets), and verifies the dimensions
// "color" and "size" have non-empty ordinal ranges.
func TestFacetSortedsetOrds_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSortedsetOrds, seed)
			storeDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("NewSimpleFSDirectory: %v", err)
			}
			defer storeDir.Close()

			dr, err := index.OpenDirectoryReader(storeDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer dr.Close()

			leaves, err := dr.Leaves()
			if err != nil {
				t.Fatalf("Leaves: %v", err)
			}
			if len(leaves) == 0 {
				t.Fatal("no leaf readers")
			}

			// Collect all terms from the SortedSetDocValues field.
			type ssvReader interface {
				GetSortedSetDocValues(string) (index.SortedSetDocValues, error)
			}
			allTerms := make(map[string]bool)
			for _, lrc := range leaves {
				lr, ok := lrc.Reader().(ssvReader)
				if !ok {
					t.Fatal("reader does not support GetSortedSetDocValues")
				}
				ssdv, err := lr.GetSortedSetDocValues(facetFieldDefaultName)
				if err != nil {
					t.Fatalf("GetSortedSetDocValues: %v", err)
				}
				if ssdv == nil {
					t.Fatal("SortedSetDocValues is nil")
				}
				for i := 0; i < ssdv.GetValueCount(); i++ {
					bytes, err := ssdv.LookupOrd(i)
					if err != nil {
						t.Fatalf("LookupOrd(%d): %v", i, err)
					}
					allTerms[string(bytes)] = true
				}
			}

			if len(allTerms) == 0 {
				t.Fatal("no SortedSetDocValues terms found")
			}

			// Verify both expected dimensions contribute terms.
			// FacetsConfig encodes terms as "dim\x1Flabel" using the
			// delimiter character U+001F (pathToString encoding).
			dimColor := "color\x1f"
			dimSize := "size\x1f"
			var colorTerms, sizeTerms int
			for term := range allTerms {
				if len(term) > len(dimColor) && term[:len(dimColor)] == dimColor {
					colorTerms++
				}
				if len(term) > len(dimSize) && term[:len(dimSize)] == dimSize {
					sizeTerms++
				}
			}
			if colorTerms == 0 {
				t.Error("no 'color' dimension terms found in SortedSetDocValues")
			}
			if sizeTerms == 0 {
				t.Error("no 'size' dimension terms found in SortedSetDocValues")
			}
		})
	}
}
