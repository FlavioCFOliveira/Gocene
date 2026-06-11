// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// TestMultipleIndexFields ports assertions from
// org.apache.lucene.facet.TestMultipleIndexFields.
//
// Unit-testable parts (FacetsConfig SetIndexFieldName + SetHierarchical routing)
// run unconditionally.

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
	// Dimension without explicit field name: GetIndexFieldName returns the
	// default index field name ("$facets"), mirroring Lucene's default.
	if got := cfg.GetIndexFieldName("Unknown"); got != "$facets" {
		t.Errorf("Unknown dim: want %q, got %q", "$facets", got)
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

// TestMultipleIndexFields_Default verifies that dimensions use the default
// index field name ("$facets") when no custom name is set.
func TestMultipleIndexFields_Default(t *testing.T) {
	cfg := NewFacetsConfig()
	if got := cfg.GetIndexFieldName("Author"); got != "$facets" {
		t.Errorf("Author without explicit field: want %q, got %q", "$facets", got)
	}
}

// TestMultipleIndexFields_Custom verifies that a dimension with a custom
// index field name returns that custom name.
func TestMultipleIndexFields_Custom(t *testing.T) {
	cfg := NewFacetsConfig()
	cfg.SetIndexFieldName("Author", "$author")
	if got := cfg.GetIndexFieldName("Author"); got != "$author" {
		t.Errorf("Author with custom field: want %q, got %q", "$author", got)
	}
}

// TestMultipleIndexFields_DifferentFieldsAndText verifies that multiple
// dimensions with different custom field names are all independently set.
func TestMultipleIndexFields_DifferentFieldsAndText(t *testing.T) {
	cfg := NewFacetsConfig()
	cfg.SetIndexFieldName("Author", "$author")
	cfg.SetIndexFieldName("Band", "$music")
	cfg.SetIndexFieldName("Year", "$year")

	if cfg.GetDimConfig("Author").IndexFieldName != "$author" {
		t.Errorf("Author: want $author, got %q", cfg.GetDimConfig("Author").IndexFieldName)
	}
	if cfg.GetDimConfig("Band").IndexFieldName != "$music" {
		t.Errorf("Band: want $music, got %q", cfg.GetDimConfig("Band").IndexFieldName)
	}
	if cfg.GetDimConfig("Year").IndexFieldName != "$year" {
		t.Errorf("Year: want $year, got %q", cfg.GetDimConfig("Year").IndexFieldName)
	}
}

// TestMultipleIndexFields_SomeSameSomeDifferent verifies that some dimensions
// sharing a custom field and others using the default are all correctly tracked.
func TestMultipleIndexFields_SomeSameSomeDifferent(t *testing.T) {
	cfg := NewFacetsConfig()
	cfg.SetIndexFieldName("Author", "$author")
	cfg.SetIndexFieldName("Band", "$music")
	cfg.SetIndexFieldName("Composer", "$music")

	if cfg.GetDimConfig("Author").IndexFieldName != "$author" {
		t.Errorf("Author: want $author, got %q", cfg.GetDimConfig("Author").IndexFieldName)
	}
	if cfg.GetDimConfig("Band").IndexFieldName != "$music" {
		t.Errorf("Band: want $music, got %q", cfg.GetDimConfig("Band").IndexFieldName)
	}
	if cfg.GetDimConfig("Composer").IndexFieldName != "$music" {
		t.Errorf("Composer: want $music, got %q", cfg.GetDimConfig("Composer").IndexFieldName)
	}
	// A dimension without explicit field should use the default.
	if got := cfg.GetIndexFieldName("Year"); got != "$facets" {
		t.Errorf("Year: want %q, got %q", "$facets", got)
	}
}
