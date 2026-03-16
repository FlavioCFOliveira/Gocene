package facets

import (
	"testing"
)

func TestNewFacetsConfig(t *testing.T) {
	fc := NewFacetsConfig()

	if fc.dimConfigs == nil {
		t.Error("Expected dimConfigs to be initialized")
	}

	if len(fc.dimConfigs) != 0 {
		t.Errorf("Expected dimConfigs to be empty, got %d items", len(fc.dimConfigs))
	}
}

func TestFacetsConfigSetMultiValued(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("tags", true)

	config := fc.GetDimConfig("tags")
	if config == nil {
		t.Fatal("Expected to get config for 'tags'")
	}

	if !config.MultiValued {
		t.Error("Expected MultiValued to be true")
	}

	// Test default (false)
	fc.SetMultiValued("category", false)
	config = fc.GetDimConfig("category")
	if config.MultiValued {
		t.Error("Expected MultiValued to be false")
	}
}

func TestFacetsConfigSetRequireDimCount(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetRequireDimCount("category", true)

	config := fc.GetDimConfig("category")
	if config == nil {
		t.Fatal("Expected to get config for 'category'")
	}

	if !config.RequireDimCount {
		t.Error("Expected RequireDimCount to be true")
	}
}

func TestFacetsConfigSetHierarchical(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetHierarchical("path", true)

	config := fc.GetDimConfig("path")
	if config == nil {
		t.Fatal("Expected to get config for 'path'")
	}

	if !config.Hierarchical {
		t.Error("Expected Hierarchical to be true")
	}
}

func TestFacetsConfigSetIndexFieldName(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetIndexFieldName("category", "cat_field")

	config := fc.GetDimConfig("category")
	if config == nil {
		t.Fatal("Expected to get config for 'category'")
	}

	if config.IndexFieldName != "cat_field" {
		t.Errorf("Expected IndexFieldName to be 'cat_field', got '%s'", config.IndexFieldName)
	}
}

func TestFacetsConfigGetIndexFieldName(t *testing.T) {
	fc := NewFacetsConfig()

	// Test default (dimension name)
	name := fc.GetIndexFieldName("category")
	if name != "category" {
		t.Errorf("Expected default IndexFieldName to be 'category', got '%s'", name)
	}

	// Test custom name
	fc.SetIndexFieldName("category", "custom_cat")
	name = fc.GetIndexFieldName("category")
	if name != "custom_cat" {
		t.Errorf("Expected IndexFieldName to be 'custom_cat', got '%s'", name)
	}
}

func TestFacetsConfigIsMultiValued(t *testing.T) {
	fc := NewFacetsConfig()

	// Test default (false)
	if fc.IsMultiValued("tags") {
		t.Error("Expected IsMultiValued to be false for unconfigured dimension")
	}

	fc.SetMultiValued("tags", true)
	if !fc.IsMultiValued("tags") {
		t.Error("Expected IsMultiValued to be true")
	}
}

func TestFacetsConfigIsHierarchical(t *testing.T) {
	fc := NewFacetsConfig()

	// Test default (false)
	if fc.IsHierarchical("path") {
		t.Error("Expected IsHierarchical to be false for unconfigured dimension")
	}

	fc.SetHierarchical("path", true)
	if !fc.IsHierarchical("path") {
		t.Error("Expected IsHierarchical to be true")
	}
}

func TestFacetsConfigIsRequireDimCount(t *testing.T) {
	fc := NewFacetsConfig()

	// Test default (false)
	if fc.IsRequireDimCount("category") {
		t.Error("Expected IsRequireDimCount to be false for unconfigured dimension")
	}

	fc.SetRequireDimCount("category", true)
	if !fc.IsRequireDimCount("category") {
		t.Error("Expected IsRequireDimCount to be true")
	}
}

func TestFacetsConfigGetDims(t *testing.T) {
	fc := NewFacetsConfig()

	// Empty initially
	dims := fc.GetDims()
	if len(dims) != 0 {
		t.Errorf("Expected 0 dimensions, got %d", len(dims))
	}

	// Add some dimensions
	fc.SetMultiValued("tags", true)
	fc.SetHierarchical("path", true)
	fc.SetRequireDimCount("category", true)

	dims = fc.GetDims()
	if len(dims) != 3 {
		t.Errorf("Expected 3 dimensions, got %d", len(dims))
	}
}

func TestFacetsConfigBuild(t *testing.T) {
	fc := NewFacetsConfig()

	// Build should not error
	err := fc.Build(nil)
	if err != nil {
		t.Errorf("Expected Build to not error, got %v", err)
	}
}

func TestNewFacetsConfigField(t *testing.T) {
	fc := NewFacetsConfig()
	fcf := NewFacetsConfigField(fc)

	if fcf == nil {
		t.Fatal("Expected FacetsConfigField to be created")
	}

	if fcf.GetConfig() != fc {
		t.Error("Expected GetConfig to return the same FacetsConfig")
	}
}
