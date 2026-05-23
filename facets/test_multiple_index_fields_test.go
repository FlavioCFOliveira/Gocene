// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// TestMultipleIndexFields ports assertions from
// org.apache.lucene.facet.TestMultipleIndexFields.
//
// Unit-testable parts (FacetsConfig SetIndexFieldName + SetHierarchical routing)
// run unconditionally.
//
// Integration tests (testDefault, testCustom, testTwoCustomsSameField,
// testDifferentFieldsAndText, testSomeSameSomeDifferent) require:
//   - RandomIndexWriter + DirectoryTaxonomyWriter + FacetsConfig.Build pipeline
//   - FacetsCollectorManager.search + getTaxonomyFacetCounts
//   - MultiFacets routing across multiple index fields
//
// These are deferred with t.Skip.

import "testing"

// TestMultipleIndexFields_ConfigRouting verifies that FacetsConfig correctly
// stores SetIndexFieldName and SetHierarchical settings per dimension.
func TestMultipleIndexFields_ConfigRouting(t *testing.T) {
	cfg := NewFacetsConfig()
	cfg.SetHierarchical("Band", true)
	cfg.SetIndexFieldName("Author", "$author")
	cfg.SetIndexFieldName("Band", "$music")
	cfg.SetIndexFieldName("Composer", "$music")

	if !cfg.GetDimConfig("Band").Hierarchical {
		t.Error("Band: expected Hierarchical=true")
	}
	if cfg.GetDimConfig("Author").IndexFieldName != "$author" {
		t.Errorf("Author index field: want $author, got %q", cfg.GetDimConfig("Author").IndexFieldName)
	}
	if cfg.GetDimConfig("Band").IndexFieldName != "$music" {
		t.Errorf("Band index field: want $music, got %q", cfg.GetDimConfig("Band").IndexFieldName)
	}
	if cfg.GetDimConfig("Composer").IndexFieldName != "$music" {
		t.Errorf("Composer index field: want $music, got %q", cfg.GetDimConfig("Composer").IndexFieldName)
	}
	// Dimension without explicit field name: GetIndexFieldName returns the dim name itself.
	if got := cfg.GetIndexFieldName("Unknown"); got != "Unknown" {
		t.Errorf("Unknown dim: want dimension name as field, got %q", got)
	}
}

// TestMultipleIndexFields_TwoCustomsSameField verifies the config when two
// dimensions share the same custom index field name.
func TestMultipleIndexFields_TwoCustomsSameField(t *testing.T) {
	cfg := NewFacetsConfig()
	cfg.SetIndexFieldName("Band", "$music")
	cfg.SetIndexFieldName("Composer", "$music")

	if cfg.GetDimConfig("Band").IndexFieldName != "$music" {
		t.Errorf("Band: want $music, got %q", cfg.GetDimConfig("Band").IndexFieldName)
	}
	if cfg.GetDimConfig("Composer").IndexFieldName != "$music" {
		t.Errorf("Composer: want $music, got %q", cfg.GetDimConfig("Composer").IndexFieldName)
	}
}

// -- Integration stubs -------------------------------------------------------

func TestMultipleIndexFields_Default(t *testing.T) {
	t.Skip("requires RandomIndexWriter + DirectoryTaxonomyWriter + FacetsCollectorManager + getTaxonomyFacetCounts pipeline")
}

func TestMultipleIndexFields_Custom(t *testing.T) {
	t.Skip("requires RandomIndexWriter + DirectoryTaxonomyWriter + MultiFacets routing pipeline")
}

func TestMultipleIndexFields_TwoCustomsSameFieldIntegration(t *testing.T) {
	t.Skip("requires RandomIndexWriter + DirectoryTaxonomyWriter + MultiFacets pipeline")
}

func TestMultipleIndexFields_DifferentFieldsAndText(t *testing.T) {
	t.Skip("requires RandomIndexWriter + DirectoryTaxonomyWriter + MultiFacets pipeline")
}

func TestMultipleIndexFields_SomeSameSomeDifferent(t *testing.T) {
	t.Skip("requires RandomIndexWriter + DirectoryTaxonomyWriter + MultiFacets pipeline")
}
