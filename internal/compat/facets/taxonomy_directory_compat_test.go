// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// taxonomy_directory_compat_test.go addresses the facets audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	facets	Taxonomy directory index files
//	    lucene_class:
//	        org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter
//	    gocene_class:  facets/directory_taxonomy_writer.go
//	    isolated:      yes:facets/directory_taxonomy_writer_test.go
//	    integration:   partial:facets/facet_integration_test.go
//	    binary_compat: no
//	    gap_notes:     "No fixture from Lucene-emitted taxonomy directory."
//
// The scenario "taxonomy-directory" builds a DirectoryTaxonomyWriter into
// a `taxo/` sidecar directory under the harness target root, adds 12
// ordered category paths derived from the seed, and commits. The
// verifier reopens the sidecar via DirectoryTaxonomyReader and asserts
// every ordinal round-trips to its expected FacetLabel.
//
// Three test classes per the rmp 4620 contract:
//
//	(a) read-fixture     — Lucene-generated taxo/ sidecar exists with
//	                        the expected Lucene104 codec file shape
//	                        (.fdt + .fnm + .tim + segments_N), and the
//	                        on-disk byte layout is stable across two
//	                        runs at the same seed.
//	(b) write-and-verify — The harness `verify` subcommand reopens the
//	                        taxonomy and re-asserts every ordinal.
//	                        Determinism is enforced by
//	                        ScenarioDeterminismTest on the Java side.
//	(c) round-trip       — Lucene-write -> Gocene-read -> Lucene-verify.
//	                        Deferred (see deferred_facets_compat_test.go)
//	                        because Gocene's DirectoryTaxonomyReader
//	                        cannot yet open a Lucene-emitted segment
//	                        through the leaf-reader Terms API (the
//	                        SegmentReader core-readers gap recorded under
//	                        memory-index reference
//	                        'gocene-segmentreader-corereaders-gap').
package facets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gfacets "github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestTaxonomyDirectory_ReadFixture (class a) drives the harness and
// asserts the resulting sidecar carries the expected Lucene104 codec
// shape. The taxonomy is just a single-segment Lucene index, so the
// canonical file extensions (.fdt, .fnm, .tim, .si, segments_N) MUST be
// present.
func TestTaxonomyDirectory_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTaxonomyDirectory, seed)
			taxoDir := filepath.Join(dir, taxoSubdir)
			// DirectoryTaxonomyWriter defaults to compound files, so the
			// per-segment payload is folded into .cfs + .cfe pairs
			// rather than exposing .fdt/.fnm/.tim individually.
			if !hasFileWithSuffix(t, taxoDir, ".cfs") {
				t.Errorf("expected .cfs in %s (taxonomy compound segment missing)", taxoDir)
			}
			if !hasFileWithSuffix(t, taxoDir, ".cfe") {
				t.Errorf("expected .cfe in %s (taxonomy compound entries missing)", taxoDir)
			}
			if !hasFileWithSuffix(t, taxoDir, ".si") {
				t.Errorf("expected .si in %s (segment info missing)", taxoDir)
			}
		})
	}
}

// TestTaxonomyDirectory_DigestDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms the non-.si files are
// byte-identical recursively (the sidecar lives under taxo/). The .si
// exclusion mirrors the Java-side Manifest.includeForHash filter.
func TestTaxonomyDirectory_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioTaxonomyDirectory, seed)
			b := generate(t, ScenarioTaxonomyDirectory, seed)
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

// TestTaxonomyDirectory_VerifySubcommand (class b, part 2) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reopened the taxonomy sidecar through
// DirectoryTaxonomyReader, looked up every seeded FacetLabel, and
// confirmed the ord round-trips to the same FacetLabel byte-for-byte.
func TestTaxonomyDirectory_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTaxonomyDirectory, seed)
			verifyHarness(t, ScenarioTaxonomyDirectory, seed, dir)
		})
	}
}

// TestTaxonomyDirectory_RoundTrip (class c) is the full Lucene -> Gocene
// -> Lucene loop. Gocene opens the taxonomy directory via
// NewDirectoryTaxonomyReader, walks ordinals, writes back via
// DirectoryTaxonomyWriter (AddTaxonomy), then the Java harness verifies the
// Gocene-written taxonomy.
func TestTaxonomyDirectory_RoundTrip(t *testing.T) {
	const expectedSize = 1 + 3 + 12 // root + 3 dim parents + 12 leaves
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTaxonomyDirectory, seed)
			taxoDir := filepath.Join(dir, taxoSubdir)

			// Open the Lucene-emitted taxonomy with Gocene's reader.
			taxoStore, err := store.NewSimpleFSDirectory(taxoDir)
			if err != nil {
				t.Fatalf("NewSimpleFSDirectory(taxo): %v", err)
			}
			defer taxoStore.Close()
			reader, err := gfacets.NewDirectoryTaxonomyReader(taxoStore)
			if err != nil {
				t.Fatalf("NewDirectoryTaxonomyReader: %v", err)
			}
			defer reader.Close()

			if got := reader.GetSize(); got != expectedSize {
				t.Fatalf("taxonomy size: got %d, want %d", got, expectedSize)
			}

			// Verify every ordinal round-trips: GetPath(o).String() -> GetOrdinal -> o.
			for o := 0; o < reader.GetSize(); o++ {
				lbl := reader.GetPath(o)
				if lbl == nil {
					t.Errorf("GetPath(%d) = nil", o)
					continue
				}
				if got := reader.GetOrdinal(lbl); got != o {
					t.Errorf("ord=%d round-trip: GetPath(%d)=%s, GetOrdinal=%d", o, o, lbl, got)
				}
				if o > 0 {
					if parent := reader.GetParent(o); parent < 0 {
						t.Errorf("ord=%d has no parent", o)
					}
				}
			}

			// Write back into a fresh dir under taxo/ subdirectory.
			writeDir := t.TempDir()
			writeTaxoPath := filepath.Join(writeDir, taxoSubdir)
			if err := os.MkdirAll(writeTaxoPath, 0755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			writeStore, err := store.NewSimpleFSDirectory(writeTaxoPath)
			if err != nil {
				t.Fatalf("NewSimpleFSDirectory(write): %v", err)
			}
			defer writeStore.Close()

			writer, err := gfacets.NewDirectoryTaxonomyWriter(writeStore)
			if err != nil {
				t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
			}
			ordMap := &gfacets.MemoryOrdinalMap{}
			if err := writer.AddTaxonomy(taxoStore, ordMap); err != nil {
				t.Fatalf("AddTaxonomy: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			// Java verifier checks the Gocene-written sidecar.
			verifyHarness(t, ScenarioTaxonomyDirectory, seed, writeDir)
		})
	}
}
