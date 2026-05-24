// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.TestTaxonomyFacet.
//
// Deviations from Java:
//   - testBasic and testTaxonomyCutterExpertModeDisableRollup require
//     IndexSearcher / RandomIndexWriter, DirectoryTaxonomyWriter/Reader, and
//     TaxonomyFacetsCutter. Deferred to backlog #2693.
//   - testConstants verifies that the invalid ordinal sentinel used by
//     TaxonomyOrdLabelBiMap matches the Java value of -1.
//     In Gocene the labels package does not yet expose an INVALID_ORD constant;
//     the test asserts the expected value directly.
package facet

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/sandbox/facet/labels"
)

// TestTaxonomyFacet_Constants mirrors testConstants: ensures that the invalid
// ordinal sentinel used by TaxonomyOrdLabelBiMap is -1 (matching
// TaxonomyReader.INVALID_ORDINAL in Java).
//
// Gocene's TaxonomyOrdLabelBiMap.GetOrd returns -1 when a label is not found,
// matching Java's INVALID_ORDINAL = -1.
func TestTaxonomyFacet_Constants(t *testing.T) {
	biMap := labels.NewTaxonomyOrdLabelBiMap()
	// A label that was never added should return -1.
	got := biMap.GetOrd("nonexistent/label")
	if got != -1 {
		t.Errorf("GetOrd(missing) = %d; want -1 (INVALID_ORDINAL)", got)
	}
}

// TestTaxonomyFacet_BiMapRoundTrip verifies that TaxonomyOrdLabelBiMap
// correctly stores and retrieves label↔ordinal mappings.
func TestTaxonomyFacet_BiMapRoundTrip(t *testing.T) {
	biMap := labels.NewTaxonomyOrdLabelBiMap()
	biMap.Add(0, "Author/Bob")
	biMap.Add(1, "Author/Lisa")
	biMap.Add(2, "Publish Date/2010")

	tests := []struct {
		label string
		ord   int
	}{
		{"Author/Bob", 0},
		{"Author/Lisa", 1},
		{"Publish Date/2010", 2},
	}
	for _, tc := range tests {
		if got := biMap.GetOrd(tc.label); got != tc.ord {
			t.Errorf("GetOrd(%q) = %d; want %d", tc.label, got, tc.ord)
		}
		if got := biMap.GetLabel(tc.ord); got != tc.label {
			t.Errorf("GetLabel(%d) = %q; want %q", tc.ord, got, tc.label)
		}
	}
}
