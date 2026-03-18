// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"strings"
	"testing"
)

func TestNewPerDimConfig(t *testing.T) {
	config := NewPerDimConfig("category")

	if config.GetDim() != "category" {
		t.Errorf("Expected dim to be 'category', got %q", config.GetDim())
	}

	if config.GetIndexFieldName() != "category" {
		t.Errorf("Expected index field name to be 'category', got %q", config.GetIndexFieldName())
	}

	if config.IsMultiValued() {
		t.Error("Expected multiValued to be false by default")
	}

	if config.IsHierarchical() {
		t.Error("Expected hierarchical to be false by default")
	}

	if config.IsRequireDimCount() {
		t.Error("Expected requireDimCount to be false by default")
	}

	if config.GetDrillDownTerms() != 1 {
		t.Errorf("Expected drillDownTerms to be 1, got %d", config.GetDrillDownTerms())
	}
}

func TestPerDimConfig_SetIndexFieldName(t *testing.T) {
	config := NewPerDimConfig("category")
	config.SetIndexFieldName("custom_category")

	if config.GetIndexFieldName() != "custom_category" {
		t.Errorf("Expected index field name to be 'custom_category', got %q", config.GetIndexFieldName())
	}
}

func TestPerDimConfig_SetMultiValued(t *testing.T) {
	config := NewPerDimConfig("tags")
	config.SetMultiValued(true)

	if !config.IsMultiValued() {
		t.Error("Expected multiValued to be true")
	}
}

func TestPerDimConfig_SetHierarchical(t *testing.T) {
	config := NewPerDimConfig("path")
	config.SetHierarchical(true)

	if !config.IsHierarchical() {
		t.Error("Expected hierarchical to be true")
	}
}

func TestPerDimConfig_SetRequireDimCount(t *testing.T) {
	config := NewPerDimConfig("category")
	config.SetRequireDimCount(true)

	if !config.IsRequireDimCount() {
		t.Error("Expected requireDimCount to be true")
	}
}

func TestPerDimConfig_SetDrillDownTerms(t *testing.T) {
	config := NewPerDimConfig("category")

	// Test valid value
	config.SetDrillDownTerms(5)
	if config.GetDrillDownTerms() != 5 {
		t.Errorf("Expected drillDownTerms to be 5, got %d", config.GetDrillDownTerms())
	}

	// Test invalid value (should not change)
	config.SetDrillDownTerms(0)
	if config.GetDrillDownTerms() != 5 {
		t.Errorf("Expected drillDownTerms to remain 5, got %d", config.GetDrillDownTerms())
	}

	// Test negative value
	config.SetDrillDownTerms(-1)
	if config.GetDrillDownTerms() != 5 {
		t.Errorf("Expected drillDownTerms to remain 5, got %d", config.GetDrillDownTerms())
	}
}

func TestPerDimConfig_SetIndexFieldNamePrefix(t *testing.T) {
	config := NewPerDimConfig("category")
	config.SetIndexFieldName("field")
	config.SetIndexFieldNamePrefix("prefix_")

	expected := "prefix_field"
	if config.GetIndexFieldName() != expected {
		t.Errorf("Expected index field name to be %q, got %q", expected, config.GetIndexFieldName())
	}

	if config.GetIndexFieldNamePrefix() != "prefix_" {
		t.Errorf("Expected index field name prefix to be 'prefix_', got %q", config.GetIndexFieldNamePrefix())
	}
}

func TestPerDimConfig_CustomProperties(t *testing.T) {
	config := NewPerDimConfig("category")

	// Test set and get
	config.SetCustomProperty("key1", "value1")
	if config.GetCustomProperty("key1") != "value1" {
		t.Errorf("Expected custom property 'key1' to be 'value1'")
	}

	// Test has
	if !config.HasCustomProperty("key1") {
		t.Error("Expected HasCustomProperty to return true for existing key")
	}

	if config.HasCustomProperty("nonexistent") {
		t.Error("Expected HasCustomProperty to return false for non-existent key")
	}

	// Test get non-existent
	if config.GetCustomProperty("nonexistent") != "" {
		t.Error("Expected GetCustomProperty to return empty string for non-existent key")
	}

	// Test remove
	config.RemoveCustomProperty("key1")
	if config.HasCustomProperty("key1") {
		t.Error("Expected custom property to be removed")
	}

	// Test get keys
	config.SetCustomProperty("key2", "value2")
	config.SetCustomProperty("key3", "value3")
	keys := config.GetCustomPropertyKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestPerDimConfig_Clone(t *testing.T) {
	config := NewPerDimConfig("category")
	config.SetMultiValued(true).
		SetHierarchical(true).
		SetRequireDimCount(true).
		SetDrillDownTerms(3).
		SetIndexFieldNamePrefix("prefix_").
		SetCustomProperty("key", "value")

	clone := config.Clone()

	// Verify clone has same values
	if clone.GetDim() != config.GetDim() {
		t.Error("Expected cloned dim to match")
	}

	if clone.IsMultiValued() != config.IsMultiValued() {
		t.Error("Expected cloned multiValued to match")
	}

	if clone.IsHierarchical() != config.IsHierarchical() {
		t.Error("Expected cloned hierarchical to match")
	}

	if clone.IsRequireDimCount() != config.IsRequireDimCount() {
		t.Error("Expected cloned requireDimCount to match")
	}

	if clone.GetDrillDownTerms() != config.GetDrillDownTerms() {
		t.Error("Expected cloned drillDownTerms to match")
	}

	if clone.GetIndexFieldNamePrefix() != config.GetIndexFieldNamePrefix() {
		t.Error("Expected cloned indexFieldNamePrefix to match")
	}

	if clone.GetCustomProperty("key") != config.GetCustomProperty("key") {
		t.Error("Expected cloned custom property to match")
	}

	// Verify clone is independent
	clone.SetMultiValued(false)
	if config.IsMultiValued() == clone.IsMultiValued() {
		t.Error("Expected clone to be independent")
	}
}

func TestPerDimConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *PerDimConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  NewPerDimConfig("category"),
			wantErr: false,
		},
		{
			name:    "empty dim",
			config:  NewPerDimConfig(""),
			wantErr: true,
		},
		{
			name: "empty index field name",
			config: func() *PerDimConfig {
				c := NewPerDimConfig("category")
				c.indexFieldName = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid drill down terms",
			config: func() *PerDimConfig {
				c := NewPerDimConfig("category")
				c.drillDownTerms = 0
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPerDimConfig_Equals(t *testing.T) {
	config1 := NewPerDimConfig("category")
	config1.SetMultiValued(true).SetHierarchical(true)

	config2 := NewPerDimConfig("category")
	config2.SetMultiValued(true).SetHierarchical(true)

	config3 := NewPerDimConfig("category")
	config3.SetMultiValued(false).SetHierarchical(true)

	if !config1.Equals(config2) {
		t.Error("Expected configs to be equal")
	}

	if config1.Equals(config3) {
		t.Error("Expected configs to not be equal")
	}

	if config1.Equals(nil) {
		t.Error("Expected Equals to return false for nil")
	}
}

func TestPerDimConfig_String(t *testing.T) {
	config := NewPerDimConfig("category")
	config.SetMultiValued(true)

	str := config.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	if !strings.Contains(str, "PerDimConfig{") {
		t.Error("Expected string to contain 'PerDimConfig{'")
	}

	if !strings.Contains(str, "category") {
		t.Error("Expected string to contain 'category'")
	}
}

func TestNewPerDimConfigBuilder(t *testing.T) {
	config, err := NewPerDimConfigBuilder("category").
		SetIndexFieldName("custom_category").
		SetMultiValued(true).
		SetHierarchical(true).
		SetRequireDimCount(true).
		SetDrillDownTerms(5).
		SetIndexFieldNamePrefix("prefix_").
		SetCustomProperty("key", "value").
		Build()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if config.GetDim() != "category" {
		t.Errorf("Expected dim to be 'category', got %q", config.GetDim())
	}

	if config.GetIndexFieldName() != "prefix_custom_category" {
		t.Errorf("Expected index field name to be 'prefix_custom_category', got %q", config.GetIndexFieldName())
	}

	if !config.IsMultiValued() {
		t.Error("Expected multiValued to be true")
	}

	if !config.IsHierarchical() {
		t.Error("Expected hierarchical to be true")
	}

	if !config.IsRequireDimCount() {
		t.Error("Expected requireDimCount to be true")
	}

	if config.GetDrillDownTerms() != 5 {
		t.Errorf("Expected drillDownTerms to be 5, got %d", config.GetDrillDownTerms())
	}

	if config.GetCustomProperty("key") != "value" {
		t.Error("Expected custom property to be set")
	}
}

func TestPerDimConfigBuilder_BuildError(t *testing.T) {
	_, err := NewPerDimConfigBuilder("").Build()
	if err == nil {
		t.Error("Expected error for empty dimension")
	}
}

func TestNewPerDimConfigRegistry(t *testing.T) {
	registry := NewPerDimConfigRegistry()

	if !registry.IsEmpty() {
		t.Error("Expected registry to be empty initially")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected count to be 0, got %d", registry.Count())
	}
}

func TestPerDimConfigRegistry_Register(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	config := NewPerDimConfig("category")

	err := registry.Register(config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !registry.Has("category") {
		t.Error("Expected registry to have 'category'")
	}

	if registry.Count() != 1 {
		t.Errorf("Expected count to be 1, got %d", registry.Count())
	}

	// Test register nil
	err = registry.Register(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	// Test register invalid config
	invalidConfig := NewPerDimConfig("")
	err = registry.Register(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestPerDimConfigRegistry_Get(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	config := NewPerDimConfig("category")
	config.SetMultiValued(true)

	registry.Register(config)

	retrieved := registry.Get("category")
	if retrieved == nil {
		t.Fatal("Expected to retrieve config")
	}

	if retrieved.GetDim() != "category" {
		t.Errorf("Expected dim to be 'category', got %q", retrieved.GetDim())
	}

	if !retrieved.IsMultiValued() {
		t.Error("Expected multiValued to be true")
	}

	// Test get non-existent
	retrieved = registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for non-existent dimension")
	}
}

func TestPerDimConfigRegistry_Remove(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	config := NewPerDimConfig("category")

	registry.Register(config)

	removed := registry.Remove("category")
	if !removed {
		t.Error("Expected Remove to return true")
	}

	if registry.Has("category") {
		t.Error("Expected dimension to be removed")
	}

	// Test remove non-existent
	removed = registry.Remove("nonexistent")
	if removed {
		t.Error("Expected Remove to return false for non-existent dimension")
	}
}

func TestPerDimConfigRegistry_Clear(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	registry.Register(NewPerDimConfig("category"))
	registry.Register(NewPerDimConfig("tags"))

	if registry.Count() != 2 {
		t.Errorf("Expected count to be 2, got %d", registry.Count())
	}

	registry.Clear()

	if !registry.IsEmpty() {
		t.Error("Expected registry to be empty after clear")
	}
}

func TestPerDimConfigRegistry_GetAll(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	registry.Register(NewPerDimConfig("category"))
	registry.Register(NewPerDimConfig("tags"))

	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(all))
	}

	if _, exists := all["category"]; !exists {
		t.Error("Expected 'category' to be in all configs")
	}
}

func TestPerDimConfigRegistry_GetDims(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	registry.Register(NewPerDimConfig("category"))
	registry.Register(NewPerDimConfig("tags"))

	dims := registry.GetDims()
	if len(dims) != 2 {
		t.Errorf("Expected 2 dimensions, got %d", len(dims))
	}

	// Check sorted
	if dims[0] != "category" || dims[1] != "tags" {
		t.Errorf("Expected sorted dimensions [category, tags], got %v", dims)
	}
}

func TestPerDimConfigRegistry_Merge(t *testing.T) {
	registry1 := NewPerDimConfigRegistry()
	registry1.Register(NewPerDimConfig("category"))

	registry2 := NewPerDimConfigRegistry()
	config := NewPerDimConfig("tags")
	config.SetMultiValued(true)
	registry2.Register(config)

	err := registry1.Merge(registry2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !registry1.Has("category") || !registry1.Has("tags") {
		t.Error("Expected both dimensions to be present after merge")
	}

	// Test merge nil
	err = registry1.Merge(nil)
	if err == nil {
		t.Error("Expected error when merging nil registry")
	}
}

func TestPerDimConfigRegistry_Clone(t *testing.T) {
	registry := NewPerDimConfigRegistry()
	registry.Register(NewPerDimConfig("category"))

	clone := registry.Clone()

	if clone.Count() != registry.Count() {
		t.Error("Expected cloned count to match")
	}

	if !clone.Has("category") {
		t.Error("Expected cloned registry to have 'category'")
	}

	// Verify independence
	clone.Register(NewPerDimConfig("tags"))
	if registry.Has("tags") {
		t.Error("Expected clone to be independent")
	}
}
