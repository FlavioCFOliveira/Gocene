// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"strings"
	"testing"
)

func TestSpatialQueryParserPlugin_New(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}

	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)
	if plugin == nil {
		t.Fatal("Expected plugin to be non-nil")
	}

	if plugin.GetName() != "spatial" {
		t.Errorf("Expected name 'spatial', got '%s'", plugin.GetName())
	}

	if plugin.GetDefaultField() != "location" {
		t.Errorf("Expected default field 'location', got '%s'", plugin.GetDefaultField())
	}

	if plugin.GetDefaultStrategy() != strategy {
		t.Error("Expected strategy to match")
	}
}

func TestSpatialQueryParserPlugin_CanParse(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"Intersects query", "Intersects(POINT(-1 1))", true},
		{"IsWithin query", "IsWithin(CIRCLE(-1 1 d=10))", true},
		{"geo_distance function", "geo_distance(field:location point:POINT(-1 1) distance:10)", true},
		{"geo_box function", "geo_box(field:location minX:-10 maxX:10)", true},
		{"Field prefixed spatial", "location:Intersects(POINT(-1 1))", true},
		{"Regular term query", "hello", false},
		{"Field query", "title:hello", false},
		{"Empty query", "", false},
		{"Boolean query", "hello AND world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := plugin.CanParse(tt.query)
			if result != tt.expected {
				t.Errorf("CanParse('%s'): expected %v, got %v", tt.query, tt.expected, result)
			}
		})
	}
}

func TestSpatialQueryParserPlugin_Parse(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	tests := []struct {
		name        string
		query       string
		shouldError bool
	}{
		{"Intersects with POINT", "Intersects(POINT(-1 1))", false},
		{"IsWithin with CIRCLE", "IsWithin(CIRCLE(-1 1 d=10))", false},
		{"Contains with ENVELOPE", "Contains(ENVELOPE(-1, 1, 1, -1))", false},
		{"geo_distance query", "geo_distance(field:location, point:POINT(-1 1), distance:10)", false},
		{"geo_box query", "geo_box(field:location, minX:-10, maxX:10, minY:-10, maxY:10)", false},
		{"Field prefixed query", "location:Intersects(POINT(-1 1))", false},
		{"Regular term query", "hello", false}, // Returns nil, nil
		{"Empty query", "", false},             // Returns nil, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := plugin.Parse(tt.query)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for '%s'", tt.query)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for '%s': %v", tt.query, err)
				}
				// For spatial queries, we expect a non-nil query
				// For non-spatial queries, we expect nil query (not error)
				if plugin.CanParse(tt.query) && query == nil {
					t.Errorf("Expected query for '%s' but got nil", tt.query)
				}
			}
		})
	}
}

func TestSpatialQueryParserPlugin_FieldMappings(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	// Add custom field mapping
	plugin.AddFieldMapping("coords", "location")

	// Check mapping
	spatialField, ok := plugin.GetFieldMapping("coords")
	if !ok {
		t.Error("Expected field mapping for 'coords'")
	}
	if spatialField != "location" {
		t.Errorf("Expected mapped field 'location', got '%s'", spatialField)
	}

	// Check that coords is now recognized as a spatial field
	if !plugin.isSpatialField("coords") {
		t.Error("Expected 'coords' to be recognized as spatial field")
	}

	// Remove mapping
	plugin.RemoveFieldMapping("coords")
	_, ok = plugin.GetFieldMapping("coords")
	if ok {
		t.Error("Expected field mapping to be removed")
	}
}

func TestSpatialQueryParserPlugin_SettersAndGetters(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	// Test default field
	plugin.SetDefaultField("geo")
	if plugin.GetDefaultField() != "geo" {
		t.Errorf("Expected default field 'geo', got '%s'", plugin.GetDefaultField())
	}

	// Test default strategy
	newStrategy, err := NewBBoxStrategy("bbox", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin.SetDefaultStrategy(newStrategy)
	if plugin.GetDefaultStrategy() != newStrategy {
		t.Error("SetDefaultStrategy did not work correctly")
	}

	// Test config
	config := plugin.GetConfig()
	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}
	if config.DefaultField != "geo" {
		t.Errorf("Expected config default field 'geo', got '%s'", config.DefaultField)
	}
}

func TestSpatialQueryParserPlugin_Config(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}

	// Test default config
	defaultConfig := DefaultSpatialPluginConfig()
	if defaultConfig == nil {
		t.Fatal("Expected default config to be non-nil")
	}
	if defaultConfig.MaxDistanceErrorPct != 0.025 {
		t.Errorf("Expected default MaxDistanceErrorPct 0.025, got %f", defaultConfig.MaxDistanceErrorPct)
	}

	// Create plugin with custom config
	customConfig := &SpatialPluginConfig{
		DefaultField:               "custom_location",
		DefaultStrategy:            strategy,
		AllowSpatialInDefaultField: false,
		SupportedOperations:        []SpatialOperation{SpatialOperationIntersects},
		MaxDistanceErrorPct:        0.05,
		AutoDetectSpatialFields:    false,
		SpatialFieldPrefixes:       []string{"geo_"},
	}

	plugin := NewSpatialQueryParserPluginWithConfig(ctx, "location", strategy, customConfig)
	config := plugin.GetConfig()

	if config.DefaultField != "custom_location" {
		t.Errorf("Expected custom field 'custom_location', got '%s'", config.DefaultField)
	}
	if config.AllowSpatialInDefaultField {
		t.Error("Expected AllowSpatialInDefaultField to be false")
	}
	if config.MaxDistanceErrorPct != 0.05 {
		t.Errorf("Expected MaxDistanceErrorPct 0.05, got %f", config.MaxDistanceErrorPct)
	}

	// Test SetConfig
	newConfig := DefaultSpatialPluginConfig()
	newConfig.DefaultField = "new_field"
	plugin.SetConfig(newConfig)

	if plugin.GetDefaultField() != "new_field" {
		t.Errorf("Expected default field 'new_field', got '%s'", plugin.GetDefaultField())
	}
}

func TestSpatialQueryParserPlugin_IsOperationSupported(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}

	// Plugin with all operations supported (empty SupportedOperations)
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	if !plugin.IsOperationSupported(SpatialOperationIntersects) {
		t.Error("Expected Intersects to be supported")
	}
	if !plugin.IsOperationSupported(SpatialOperationIsWithin) {
		t.Error("Expected IsWithin to be supported")
	}

	// Plugin with limited operations
	config := DefaultSpatialPluginConfig()
	config.SupportedOperations = []SpatialOperation{SpatialOperationIntersects}
	limitedPlugin := NewSpatialQueryParserPluginWithConfig(ctx, "location", strategy, config)

	if !limitedPlugin.IsOperationSupported(SpatialOperationIntersects) {
		t.Error("Expected Intersects to be supported")
	}
	if limitedPlugin.IsOperationSupported(SpatialOperationIsWithin) {
		t.Error("Expected IsWithin NOT to be supported")
	}
}

func TestSpatialQueryParserPlugin_SupportedOperationNames(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	names := plugin.SupportedOperationNames()
	if len(names) == 0 {
		t.Error("Expected non-empty operation names")
	}

	// Check that common operations are included
	hasIntersects := false
	for _, name := range names {
		if strings.EqualFold(name, "Intersects") {
			hasIntersects = true
			break
		}
	}
	if !hasIntersects {
		t.Error("Expected 'Intersects' to be in supported operations")
	}

	// Test with limited operations
	config := DefaultSpatialPluginConfig()
	config.SupportedOperations = []SpatialOperation{SpatialOperationIntersects, SpatialOperationIsWithin}
	limitedPlugin := NewSpatialQueryParserPluginWithConfig(ctx, "location", strategy, config)

	limitedNames := limitedPlugin.SupportedOperationNames()
	if len(limitedNames) != 2 {
		t.Errorf("Expected 2 operation names, got %d", len(limitedNames))
	}
}

func TestSpatialQueryParserPlugin_String(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	s := plugin.String()
	if !strings.Contains(s, "SpatialQueryParserPlugin") {
		t.Errorf("Expected String() to contain 'SpatialQueryParserPlugin', got '%s'", s)
	}
	if !strings.Contains(s, "location") {
		t.Errorf("Expected String() to contain 'location', got '%s'", s)
	}
}

func TestSpatialQueryParserPlugin_GetSpatialQueryParser(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	parser := plugin.GetSpatialQueryParser()
	if parser == nil {
		t.Fatal("Expected parser to be non-nil")
	}

	// Verify it's the same parser we configured
	if parser.GetDefaultField() != "location" {
		t.Errorf("Expected parser default field 'location', got '%s'", parser.GetDefaultField())
	}
}

func TestSpatialPluginRegistry(t *testing.T) {
	ctx := NewSpatialContext()

	registry := NewSpatialPluginRegistry()
	if registry == nil {
		t.Fatal("Expected registry to be non-nil")
	}

	// Create and register plugins
	strategy1, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin1 := NewSpatialQueryParserPlugin(ctx, "location", strategy1)
	registry.Register(plugin1)

	strategy2, err := NewBBoxStrategy("bbox", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin2 := NewSpatialQueryParserPlugin(ctx, "bbox", strategy2)
	registry.Register(plugin2)

	// Test Get
	retrieved, ok := registry.Get("location")
	if !ok {
		t.Error("Expected to retrieve 'location' plugin")
	}
	if retrieved != plugin1 {
		t.Error("Retrieved plugin does not match expected")
	}

	// Test GetAll
	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(all))
	}

	// Test FindPluginForQuery
	found, ok := registry.FindPluginForQuery("Intersects(POINT(-1 1))")
	if !ok {
		t.Error("Expected to find a plugin for query")
	}
	if found != plugin1 && found != plugin2 {
		t.Error("Found plugin does not match any registered plugin")
	}

	// Test Unregister
	registry.Unregister("location")
	_, ok = registry.Get("location")
	if ok {
		t.Error("Expected 'location' plugin to be unregistered")
	}
}

func TestParseSpatialOperation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    SpatialOperation
		shouldError bool
	}{
		{"Intersects", "intersects", SpatialOperationIntersects, false},
		{"IsWithin", "iswithin", SpatialOperationIsWithin, false},
		{"Within", "within", SpatialOperationIsWithin, false},
		{"Contains", "contains", SpatialOperationContains, false},
		{"Disjoint", "disjoint", SpatialOperationIsDisjointTo, false},
		{"IsDisjointTo", "isdisjointto", SpatialOperationIsDisjointTo, false},
		{"Equals", "equals", SpatialOperationEquals, false},
		{"Overlaps", "overlaps", SpatialOperationOverlaps, false},
		{"BboxIntersects", "bboxintersects", SpatialOperationBboxIntersects, false},
		{"BboxWithin", "bboxwithin", SpatialOperationBboxWithin, false},
		{"Unknown", "unknown", SpatialOperationIntersects, true},
		{"Empty", "", SpatialOperationIntersects, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := ParseSpatialOperation(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for '%s'", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for '%s': %v", tt.input, err)
				}
				if op != tt.expected {
					t.Errorf("Expected operation %v, got %v", tt.expected, op)
				}
			}
		})
	}
}

func TestSpatialQueryParserPlugin_AutoDetectSpatialFields(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	// Test auto-detection of spatial fields with prefixes
	spatialFields := []string{
		"geo_point",
		"location_latlon",
		"spatial_index",
		"location",
	}

	for _, field := range spatialFields {
		if !plugin.isSpatialField(field) {
			t.Errorf("Expected '%s' to be recognized as spatial field", field)
		}
	}

	// Disable auto-detection
	config := plugin.GetConfig()
	config.AutoDetectSpatialFields = false

	nonSpatialFields := []string{"title", "content", "name"}
	for _, field := range nonSpatialFields {
		if plugin.isSpatialField(field) {
			t.Errorf("Expected '%s' NOT to be recognized as spatial field", field)
		}
	}
}

func TestSpatialQueryParserPlugin_CaseInsensitivity(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)

	// Test that CanParse is case-insensitive for function queries
	functionQueries := []string{
		"GEO_DISTANCE(field:location point:POINT(-1 1) distance:10)",
		"Geo_Distance(field:location point:POINT(-1 1) distance:10)",
		"geo_DISTANCE(field:location point:POINT(-1 1) distance:10)",
	}

	for _, query := range functionQueries {
		if !plugin.CanParse(query) {
			t.Errorf("Expected CanParse to handle '%s' case-insensitively", query)
		}
	}
}
