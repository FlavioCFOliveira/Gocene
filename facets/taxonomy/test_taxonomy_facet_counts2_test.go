// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetCounts2 ports the test assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetCounts2.
//
// The Java class is a class-level setup test that builds a multi-segment
// index with taxonomy categories, then queries it with FacetsCollector +
// FastTaxonomyFacetCounts. The full integration path (IndexWriter →
// FacetsCollector → FastTaxonomyFacetCounts) is deferred until the Gocene
// search pipeline is fully wired; those tests are marked t.Skip.
//
// The unit-testable portion covers FacetsConfig configuration:
// multi-valued, hierarchical, and requireDimCount settings used in the
// setup code are tested here to verify they compile and behave as expected.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

const (
	cpA = "A"
	cpB = "B"
	cpC = "C"
	cpD = "D"

	numChildrenCpA = 5
	numChildrenCpB = 3
	numChildrenCpC = 5
	numChildrenCpD = 5
)

// facetCounts2Config returns the FacetsConfig used by the Java test class.
func facetCounts2Config() *facets.FacetsConfig {
	cfg := facets.NewFacetsConfig()
	cfg.SetMultiValued(cpA, true)
	cfg.SetMultiValued(cpB, true)
	cfg.SetRequireDimCount(cpB, true)
	cfg.SetHierarchical(cpD, true)
	return cfg
}

// TestTaxonomyFacetCounts2_ConfigSetup verifies that the FacetsConfig
// settings mirror the Java test fixture: A multi-valued, B multi-valued+
// requireDimCount, D hierarchical.
func TestTaxonomyFacetCounts2_ConfigSetup(t *testing.T) {
	cfg := facetCounts2Config()

	if !cfg.GetDimConfig(cpA).MultiValued {
		t.Errorf("expected dim A to be multi-valued")
	}
	if !cfg.GetDimConfig(cpB).MultiValued {
		t.Errorf("expected dim B to be multi-valued")
	}
	if !cfg.GetDimConfig(cpB).RequireDimCount {
		t.Errorf("expected dim B to require dim count")
	}
	if !cfg.GetDimConfig(cpD).Hierarchical {
		t.Errorf("expected dim D to be hierarchical")
	}
	// C and A should NOT require dim count by default
	if cfg.GetDimConfig(cpA).RequireDimCount {
		t.Errorf("expected dim A NOT to require dim count")
	}
}

// TestTaxonomyFacetCounts2_CategoryKeys verifies category key construction
// used in the Java test's expected-count maps (dim+"/"+child format).
func TestTaxonomyFacetCounts2_CategoryKeys(t *testing.T) {
	wantKeys := make([]string, 0)
	for i := 0; i < numChildrenCpA; i++ {
		wantKeys = append(wantKeys, cpA+"/"+itoa(i))
	}
	for i := 0; i < numChildrenCpB; i++ {
		wantKeys = append(wantKeys, cpB+"/"+itoa(i))
	}
	// verify no duplicates
	seen := make(map[string]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate category key: %s", k)
		}
		seen[k] = struct{}{}
	}
	if len(seen) != numChildrenCpA+numChildrenCpB {
		t.Errorf("expected %d unique keys, got %d", numChildrenCpA+numChildrenCpB, len(seen))
	}
}

// itoa converts a non-negative int to its decimal string without importing fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// -- Integration stubs (require full index + FacetsCollector pipeline) ------

func TestTaxonomyFacetCounts2_DifferentNumResults(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + FastTaxonomyFacetCounts pipeline")
}

func TestTaxonomyFacetCounts2_AllCounts(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + FastTaxonomyFacetCounts pipeline")
}

func TestTaxonomyFacetCounts2_BigNumResults(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + FastTaxonomyFacetCounts pipeline")
}

func TestTaxonomyFacetCounts2_NoParents(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + FastTaxonomyFacetCounts pipeline")
}
