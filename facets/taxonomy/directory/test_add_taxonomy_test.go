// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

// TestAddTaxonomy ports assertions from
// org.apache.lucene.facet.taxonomy.directory.TestAddTaxonomy.
//
// All tests require:
//   - DirectoryTaxonomyWriter.AddTaxonomy(src Directory, map OrdinalMap)
//   - OrdinalMap implementations: DiskOrdinalMap and MemoryOrdinalMap
//   - DirectoryTaxonomyReader with GetPath/GetOrdinal/GetSize
//
// These components are not yet implemented in Gocene.
// All tests are deferred with t.Skip until the full pipeline is available.

import "testing"

// TestAddTaxonomy_Simple verifies that merging a source taxonomy into a destination
// taxonomy produces the correct ordinal mapping (no duplicates, ordinals consistent).
func TestAddTaxonomy_Simple(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyWriter.AddTaxonomy + OrdinalMap (MemoryOrdinalMap/DiskOrdinalMap)")
}

// TestAddTaxonomy_AddEmpty verifies that adding an empty source taxonomy is a no-op.
func TestAddTaxonomy_AddEmpty(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyWriter.AddTaxonomy + OrdinalMap pipeline")
}

// TestAddTaxonomy_AddToEmpty verifies that adding categories to an empty destination
// taxonomy via AddTaxonomy yields correct ordinals.
func TestAddTaxonomy_AddToEmpty(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyWriter.AddTaxonomy + OrdinalMap pipeline")
}

// TestAddTaxonomy_Concurrency verifies that AddTaxonomy and AddCategory can run in
// parallel without introducing duplicate categories.
func TestAddTaxonomy_Concurrency(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyWriter.AddTaxonomy + concurrent AddCategory + MemoryOrdinalMap pipeline")
}

// TestAddTaxonomy_Medium runs a parameterised random merge test with moderate sizes.
func TestAddTaxonomy_Medium(t *testing.T) {
	t.Skip("requires DirectoryTaxonomyWriter.AddTaxonomy + OrdinalMap pipeline")
}
