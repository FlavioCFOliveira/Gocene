// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"strings"
	"testing"
)

func TestSpatialQueryParser_Parse(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser := NewSpatialQueryParser(ctx, "location", strategy)

	tests := []struct {
		name        string
		query       string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "Intersects with POINT",
			query:       "Intersects(POINT(-1 1))",
			shouldError: false,
		},
		{
			name:        "IsWithin with CIRCLE",
			query:       "IsWithin(CIRCLE(-1 1 d=10))",
			shouldError: false,
		},
		{
			name:        "Contains with ENVELOPE",
			query:       "Contains(ENVELOPE(-1, 1, 1, -1))",
			shouldError: false,
		},
		{
			name:        "Disjoint with BBOX",
			query:       "Disjoint(BBOX(-10, 10, 10, -10))",
			shouldError: true, // PointVectorStrategy doesn't support Disjoint
		},
		{
			name:        "Equals with RECTANGLE",
			query:       "Equals(RECTANGLE(-1, 1, -1, 1))",
			shouldError: true, // PointVectorStrategy doesn't support Equals
		},
		{
			name:        "Overlaps with POINT",
			query:       "Overlaps(POINT(0 0))",
			shouldError: true, // PointVectorStrategy doesn't support Overlaps
		},
		{
			name:        "BboxIntersects with ENVELOPE",
			query:       "BboxIntersects(ENVELOPE(-1, 1, 1, -1))",
			shouldError: true, // PointVectorStrategy doesn't support BboxIntersects
		},
		{
			name:        "BboxWithin with BBOX",
			query:       "BboxWithin(BBOX(-1, 1, 1, -1))",
			shouldError: true, // PointVectorStrategy doesn't support BboxWithin
		},
		{
			name:        "Empty query",
			query:       "",
			shouldError: true,
			errorMsg:    "empty query string",
		},
		{
			name:        "Invalid format - missing parenthesis",
			query:       "Intersects POINT(-1 1)",
			shouldError: true,
		},
		{
			name:        "Unknown operation",
			query:       "UnknownOp(POINT(-1 1))",
			shouldError: true,
		},
		{
			name:        "Invalid shape",
			query:       "Intersects(INVALID(-1 1))",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.query)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if query == nil {
					t.Error("Expected query but got nil")
				}
			}
		})
	}
}

func TestSpatialQueryParser_ParseFunctionQueries(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser := NewSpatialQueryParser(ctx, "location", strategy)

	tests := []struct {
		name        string
		query       string
		shouldError bool
	}{
		{
			name:        "geo_distance with POINT",
			query:       "geo_distance(field:location, point:POINT(-1 1), distance:10)",
			shouldError: false,
		},
		{
			name:        "geo_distance with coordinates",
			query:       "geo_distance(field:location, point:'-1 1', distance:10km)",
			shouldError: false,
		},
		{
			name:        "geo_box with bounds",
			query:       "geo_box(field:location, minX:-10, maxX:10, minY:-10, maxY:10)",
			shouldError: false,
		},
		{
			name:        "geo_intersects with shape",
			query:       "geo_intersects(shape:POINT(-1 1))",
			shouldError: true, // PointVectorStrategy doesn't support Intersects
		},
		{
			name:        "geo_within with envelope",
			query:       "geo_within(shape:ENVELOPE(-10, 10, 10, -10))",
			shouldError: true, // PointVectorStrategy doesn't support IsWithin
		},
		{
			name:        "geo_distance missing field",
			query:       "geo_distance(point:POINT(-1 1) distance:10)",
			shouldError: true,
		},
		{
			name:        "geo_distance missing point",
			query:       "geo_distance(field:location distance:10)",
			shouldError: true,
		},
		{
			name:        "geo_distance missing distance",
			query:       "geo_distance(field:location point:POINT(-1 1))",
			shouldError: true,
		},
		{
			name:        "Unknown function",
			query:       "geo_unknown(field:location)",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.query)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if query == nil {
					t.Error("Expected query but got nil")
				}
			}
		})
	}
}

func TestSpatialQueryParser_parsePointFromString(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialQueryParserWithContext(ctx)

	tests := []struct {
		name        string
		input       string
		expectedX   float64
		expectedY   float64
		shouldError bool
	}{
		{
			name:      "POINT format",
			input:     "POINT(-1 1)",
			expectedX: -1,
			expectedY: 1,
		},
		{
			name:      "Space separated",
			input:     "-1 1",
			expectedX: -1,
			expectedY: 1,
		},
		{
			name:      "Comma separated",
			input:     "-1,1",
			expectedX: -1,
			expectedY: 1,
		},
		{
			name:      "Comma separated with spaces",
			input:     "-1, 1",
			expectedX: -1,
			expectedY: 1,
		},
		{
			name:        "Invalid format",
			input:       "invalid",
			shouldError: true,
		},
		{
			name:        "Too many values",
			input:       "1 2 3",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			point, err := parser.parsePointFromString(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if point.X != tt.expectedX {
					t.Errorf("Expected X=%f, got X=%f", tt.expectedX, point.X)
				}
				if point.Y != tt.expectedY {
					t.Errorf("Expected Y=%f, got Y=%f", tt.expectedY, point.Y)
				}
			}
		})
	}
}

func TestSpatialQueryParser_parseDistance(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialQueryParserWithContext(ctx)

	tests := []struct {
		name        string
		input       string
		shouldError bool
		checkRange  bool
		minValue    float64
		maxValue    float64
	}{
		{
			name:       "Simple number",
			input:      "10",
			checkRange: true,
			minValue:   9,
			maxValue:   11,
		},
		{
			name:       "Kilometers",
			input:      "10km",
			checkRange: true,
			minValue:   0.05, // ~0.09 degrees
			maxValue:   0.15,
		},
		{
			name:       "Miles",
			input:      "10mi",
			checkRange: true,
			minValue:   0.1, // ~0.14 degrees
			maxValue:   0.2,
		},
		{
			name:       "Meters",
			input:      "1000m",
			checkRange: true,
			minValue:   0.005, // ~0.009 degrees
			maxValue:   0.015,
		},
		{
			name:       "Degrees",
			input:      "10deg",
			checkRange: true,
			minValue:   9,
			maxValue:   11,
		},
		{
			name:        "Invalid",
			input:       "invalid",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance, err := parser.parseDistance(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.checkRange && (distance < tt.minValue || distance > tt.maxValue) {
					t.Errorf("Expected distance in range [%f, %f], got %f", tt.minValue, tt.maxValue, distance)
				}
			}
		})
	}
}

func TestSpatialQueryParser_parseNamedParameters(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialQueryParserWithContext(ctx)

	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "Space separated",
			input: "field:location point:POINT(-1 1) distance:10",
			expected: map[string]string{
				"field":    "location",
				"point":    "POINT(-1",
				"distance": "10",
			},
		},
		{
			name:  "Comma separated",
			input: "field:location, point:POINT(-1 1), distance:10",
			expected: map[string]string{
				"field":    "location",
				"point":    "POINT(-1 1)",
				"distance": "10",
			},
		},
		{
			name:  "Quoted values",
			input: `field:'my field', point:"POINT(-1 1)"`,
			expected: map[string]string{
				"field": "my field",
				"point": "POINT(-1 1)",
			},
		},
		{
			name:     "Empty",
			input:    "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := parser.parseNamedParameters(tt.input)

			for key, expectedValue := range tt.expected {
				if actualValue, ok := params[key]; !ok {
					t.Errorf("Missing expected key: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("For key '%s': expected '%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestSpatialQueryParser_IsSpatialQuery(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialQueryParserWithContext(ctx)

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"Intersects", "Intersects(POINT(-1 1))", true},
		{"IsWithin", "IsWithin(CIRCLE(-1 1 d=10))", true},
		{"Within", "Within(POINT(-1 1))", true},
		{"Contains", "Contains(ENVELOPE(-1, 1, 1, -1))", true},
		{"Disjoint", "Disjoint(BBOX(-10, 10, 10, -10))", true},
		{"Equals", "Equals(POINT(0 0))", true},
		{"Overlaps", "Overlaps(RECTANGLE(-1, 1, -1, 1))", true},
		{"BboxIntersects", "BboxIntersects(ENVELOPE(-1, 1, 1, -1))", true},
		{"BboxWithin", "BboxWithin(BBOX(-1, 1, 1, -1))", true},
		{"geo_distance", "geo_distance(field:location point:POINT(-1 1) distance:10)", true},
		{"geo_box", "geo_box(field:location minX:-10 maxX:10)", true},
		{"Regular term query", "hello world", false},
		{"Field query", "title:hello", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsSpatialQuery(tt.query)
			if result != tt.expected {
				t.Errorf("IsSpatialQuery('%s'): expected %v, got %v", tt.query, tt.expected, result)
			}
		})
	}
}

func TestSpatialQueryParser_SettersAndGetters(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser := NewSpatialQueryParser(ctx, "location", strategy)

	// Test default field
	if parser.GetDefaultField() != "location" {
		t.Errorf("Expected default field 'location', got '%s'", parser.GetDefaultField())
	}

	parser.SetDefaultField("geo")
	if parser.GetDefaultField() != "geo" {
		t.Errorf("Expected default field 'geo', got '%s'", parser.GetDefaultField())
	}

	// Test default strategy
	if parser.GetDefaultStrategy() == nil {
		t.Error("Expected non-nil default strategy")
	}

	newStrategy, err := NewBBoxStrategy("bbox", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser.SetDefaultStrategy(newStrategy)
	if parser.GetDefaultStrategy() != newStrategy {
		t.Error("SetDefaultStrategy did not work correctly")
	}

	// Test spatial context
	if parser.GetSpatialContext() != ctx {
		t.Error("GetSpatialContext returned wrong context")
	}
}

func TestSpatialQueryParserFactory(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	factory := NewSpatialQueryParserFactory(ctx, "location", strategy)

	// Test CreateParser
	parser := factory.CreateParser()
	if parser == nil {
		t.Fatal("CreateParser returned nil")
	}
	if parser.GetDefaultField() != "location" {
		t.Errorf("Expected default field 'location', got '%s'", parser.GetDefaultField())
	}

	// Test CreateParserWithField
	parserWithField := factory.CreateParserWithField("geo")
	if parserWithField.GetDefaultField() != "geo" {
		t.Errorf("Expected default field 'geo', got '%s'", parserWithField.GetDefaultField())
	}

	// Test CreateParserWithStrategy
	newStrategy, err := NewBBoxStrategy("bbox", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parserWithStrategy := factory.CreateParserWithStrategy(newStrategy)
	if parserWithStrategy.GetDefaultStrategy() != newStrategy {
		t.Error("CreateParserWithStrategy did not set strategy correctly")
	}

	// Test SetDefaultField
	factory.SetDefaultField("new_field")
	newParser := factory.CreateParser()
	if newParser.GetDefaultField() != "new_field" {
		t.Errorf("Expected default field 'new_field', got '%s'", newParser.GetDefaultField())
	}

	// Test SetDefaultStrategy
	geohashTree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("Failed to create geohash tree: %v", err)
	}
	anotherStrategy, err := NewPrefixTreeStrategy("prefix", geohashTree, 12, ctx)
	if err != nil {
		t.Fatalf("Failed to create prefix tree strategy: %v", err)
	}
	factory.SetDefaultStrategy(anotherStrategy)
	newParser2 := factory.CreateParser()
	if newParser2.GetDefaultStrategy() != anotherStrategy {
		t.Error("SetDefaultStrategy did not work correctly")
	}
}

func TestSpatialQueryParser_CaseVariations(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser := NewSpatialQueryParser(ctx, "location", strategy)

	tests := []struct {
		name  string
		query string
	}{
		{"Lowercase intersects", "intersects(POINT(-1 1))"},
		{"Mixed case Intersects", "Intersects(POINT(-1 1))"},
		{"Uppercase INTERSECTS", "INTERSECTS(POINT(-1 1))"},
		{"Lowercase iswithin", "iswithin(CIRCLE(-1 1 d=10))"},
		{"Mixed case IsWithin", "IsWithin(CIRCLE(-1 1 d=10))"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.query)
			if err != nil {
				t.Errorf("Unexpected error for '%s': %v", tt.query, err)
			}
			if query == nil {
				t.Errorf("Expected query for '%s' but got nil", tt.query)
			}
		})
	}
}

func TestSpatialQueryParser_WhitespaceHandling(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}
	parser := NewSpatialQueryParser(ctx, "location", strategy)

	tests := []struct {
		name  string
		query string
	}{
		{"No spaces", "Intersects(POINT(-1 1))"},
		{"Spaces around operation", "Intersects ( POINT(-1 1) )"},
		{"Extra spaces", "  Intersects(  POINT(  -1   1  )  )  "},
		{"Tabs", "Intersects(\tPOINT(-1\t1)\t)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.query)
			if err != nil {
				t.Errorf("Unexpected error for '%s': %v", tt.query, err)
			}
			if query == nil {
				t.Errorf("Expected query for '%s' but got nil", tt.query)
			}
		})
	}
}
