package facets

import (
	"strings"
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

// Tests for FacetsConfig Extensions (GC-441)

func TestFacetsConfig_DefaultValues(t *testing.T) {
	fc := NewFacetsConfig()

	// Test default values
	if fc.GetDefaultIndexFieldName() != "$facets" {
		t.Errorf("Expected default index field name to be '$facets', got %q", fc.GetDefaultIndexFieldName())
	}

	if fc.GetDrillDownFieldName() != "$facets.drilldown" {
		t.Errorf("Expected default drill-down field name to be '$facets.drilldown', got %q", fc.GetDrillDownFieldName())
	}

	if !fc.IsValidateFields() {
		t.Error("Expected validate fields to be true by default")
	}

	if !fc.IsAutoDetectHierarchical() {
		t.Error("Expected auto-detect hierarchical to be true by default")
	}

	if fc.IsDefaultMultiValued() {
		t.Error("Expected default multi-valued to be false")
	}

	if fc.IsDefaultHierarchical() {
		t.Error("Expected default hierarchical to be false")
	}

	if fc.IsDefaultRequireDimCount() {
		t.Error("Expected default require dim count to be false")
	}
}

func TestFacetsConfig_SetDefaultIndexFieldName(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDefaultIndexFieldName("custom_facets")

	if fc.GetDefaultIndexFieldName() != "custom_facets" {
		t.Errorf("Expected index field name to be 'custom_facets', got %q", fc.GetDefaultIndexFieldName())
	}
}

func TestFacetsConfig_SetDrillDownFieldName(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDrillDownFieldName("custom_drilldown")

	if fc.GetDrillDownFieldName() != "custom_drilldown" {
		t.Errorf("Expected drill-down field name to be 'custom_drilldown', got %q", fc.GetDrillDownFieldName())
	}
}

func TestFacetsConfig_SetValidateFields(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetValidateFields(false)

	if fc.IsValidateFields() {
		t.Error("Expected validate fields to be false")
	}
}

func TestFacetsConfig_SetAutoDetectHierarchical(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetAutoDetectHierarchical(false)

	if fc.IsAutoDetectHierarchical() {
		t.Error("Expected auto-detect hierarchical to be false")
	}
}

func TestFacetsConfig_SetDefaultMultiValued(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDefaultMultiValued(true)

	if !fc.IsDefaultMultiValued() {
		t.Error("Expected default multi-valued to be true")
	}
}

func TestFacetsConfig_SetDefaultHierarchical(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDefaultHierarchical(true)

	if !fc.IsDefaultHierarchical() {
		t.Error("Expected default hierarchical to be true")
	}
}

func TestFacetsConfig_SetDefaultRequireDimCount(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDefaultRequireDimCount(true)

	if !fc.IsDefaultRequireDimCount() {
		t.Error("Expected default require dim count to be true")
	}
}

func TestFacetsConfig_HasDimension(t *testing.T) {
	fc := NewFacetsConfig()

	if fc.HasDimension("category") {
		t.Error("Expected HasDimension to be false for unconfigured dimension")
	}

	fc.SetMultiValued("category", true)

	if !fc.HasDimension("category") {
		t.Error("Expected HasDimension to be true after configuration")
	}
}

func TestFacetsConfig_RemoveDimension(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("category", true)

	if !fc.HasDimension("category") {
		t.Error("Expected dimension to exist before removal")
	}

	removed := fc.RemoveDimension("category")
	if !removed {
		t.Error("Expected RemoveDimension to return true")
	}

	if fc.HasDimension("category") {
		t.Error("Expected dimension to be removed")
	}

	// Try to remove non-existent dimension
	removed = fc.RemoveDimension("nonexistent")
	if removed {
		t.Error("Expected RemoveDimension to return false for non-existent dimension")
	}
}

func TestFacetsConfig_Clear(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("category", true)
	fc.SetHierarchical("path", true)

	if fc.GetDimensionCount() != 2 {
		t.Errorf("Expected 2 dimensions before clear, got %d", fc.GetDimensionCount())
	}

	fc.Clear()

	if fc.GetDimensionCount() != 0 {
		t.Errorf("Expected 0 dimensions after clear, got %d", fc.GetDimensionCount())
	}

	if !fc.IsEmpty() {
		t.Error("Expected IsEmpty to be true after clear")
	}
}

func TestFacetsConfig_GetDimensionCount(t *testing.T) {
	fc := NewFacetsConfig()

	if fc.GetDimensionCount() != 0 {
		t.Errorf("Expected 0 dimensions initially, got %d", fc.GetDimensionCount())
	}

	fc.SetMultiValued("category", true)
	fc.SetHierarchical("path", true)

	if fc.GetDimensionCount() != 2 {
		t.Errorf("Expected 2 dimensions, got %d", fc.GetDimensionCount())
	}
}

func TestFacetsConfig_IsEmpty(t *testing.T) {
	fc := NewFacetsConfig()

	if !fc.IsEmpty() {
		t.Error("Expected IsEmpty to be true initially")
	}

	fc.SetMultiValued("category", true)

	if fc.IsEmpty() {
		t.Error("Expected IsEmpty to be false after adding dimension")
	}
}

func TestFacetsConfig_Clone(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetDefaultIndexFieldName("custom_facets")
	fc.SetDrillDownFieldName("custom_drilldown")
	fc.SetValidateFields(false)
	fc.SetMultiValued("category", true)
	fc.SetHierarchical("path", true)

	clone := fc.Clone()

	// Verify clone has same values
	if clone.GetDefaultIndexFieldName() != fc.GetDefaultIndexFieldName() {
		t.Error("Expected cloned index field name to match")
	}

	if clone.GetDrillDownFieldName() != fc.GetDrillDownFieldName() {
		t.Error("Expected cloned drill-down field name to match")
	}

	if clone.IsValidateFields() != fc.IsValidateFields() {
		t.Error("Expected cloned validate fields to match")
	}

	if clone.GetDimensionCount() != fc.GetDimensionCount() {
		t.Error("Expected cloned dimension count to match")
	}

	// Verify clone is independent
	clone.SetMultiValued("new_dim", true)
	if fc.GetDimensionCount() == clone.GetDimensionCount() {
		t.Error("Expected clone to be independent")
	}
}

func TestFacetsConfig_Merge(t *testing.T) {
	fc1 := NewFacetsConfig()
	fc1.SetMultiValued("category", true)

	fc2 := NewFacetsConfig()
	fc2.SetHierarchical("path", true)

	err := fc1.Merge(fc2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !fc1.HasDimension("category") || !fc1.HasDimension("path") {
		t.Error("Expected both dimensions to be present after merge")
	}

	// Test merge with nil
	err = fc1.Merge(nil)
	if err == nil {
		t.Error("Expected error when merging nil config")
	}
}

func TestFacetsConfig_GetAllDimConfigs(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("category", true)
	fc.SetHierarchical("path", true)

	configs := fc.GetAllDimConfigs()
	if len(configs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(configs))
	}

	if _, exists := configs["category"]; !exists {
		t.Error("Expected category config to exist")
	}

	if _, exists := configs["path"]; !exists {
		t.Error("Expected path config to exist")
	}
}

func TestFacetsConfig_GetHierarchicalDims(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetHierarchical("path", true)
	fc.SetMultiValued("tags", true)

	dims := fc.GetHierarchicalDims()
	if len(dims) != 1 || dims[0] != "path" {
		t.Errorf("Expected [path], got %v", dims)
	}
}

func TestFacetsConfig_GetMultiValuedDims(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("tags", true)
	fc.SetHierarchical("path", true)

	dims := fc.GetMultiValuedDims()
	if len(dims) != 1 || dims[0] != "tags" {
		t.Errorf("Expected [tags], got %v", dims)
	}
}

func TestFacetsConfig_Validate(t *testing.T) {
	fc := NewFacetsConfig()

	// Valid configuration
	fc.SetMultiValued("category", true)
	err := fc.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}

	// Test duplicate index field names
	fc2 := NewFacetsConfig()
	fc2.SetIndexFieldName("dim1", "same_field")
	fc2.SetIndexFieldName("dim2", "same_field")

	err = fc2.Validate()
	if err == nil {
		t.Error("Expected error for duplicate index field names")
	}
}

func TestFacetsConfig_String(t *testing.T) {
	fc := NewFacetsConfig()
	fc.SetMultiValued("category", true)

	str := fc.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check that string contains expected content
	if !strings.Contains(str, "FacetsConfig{") {
		t.Error("Expected string to contain 'FacetsConfig{'")
	}

	if !strings.Contains(str, "category") {
		t.Error("Expected string to contain 'category'")
	}
}
