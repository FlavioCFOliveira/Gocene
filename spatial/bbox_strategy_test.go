// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestNewBBoxStrategy(t *testing.T) {
	ctx := NewSpatialContext()

	tests := []struct {
		name        string
		fieldName   string
		ctx         *SpatialContext
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid strategy",
			fieldName: "location",
			ctx:       ctx,
			wantErr:   false,
		},
		{
			name:        "empty field name",
			fieldName:   "",
			ctx:         ctx,
			wantErr:     true,
			errContains: "field name cannot be empty",
		},
		{
			name:        "nil context",
			fieldName:   "location",
			ctx:         nil,
			wantErr:     true,
			errContains: "spatial context cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewBBoxStrategy(tt.fieldName, tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewBBoxStrategy() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewBBoxStrategy() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewBBoxStrategy() unexpected error = %v", err)
				return
			}
			if strategy == nil {
				t.Error("NewBBoxStrategy() returned nil strategy")
				return
			}

			// Verify field names
			if strategy.GetFieldName() != tt.fieldName {
				t.Errorf("GetFieldName() = %v, want %v", strategy.GetFieldName(), tt.fieldName)
			}
			if strategy.GetMinXFieldName() != tt.fieldName+"_minX" {
				t.Errorf("GetMinXFieldName() = %v, want %v", strategy.GetMinXFieldName(), tt.fieldName+"_minX")
			}
			if strategy.GetMaxXFieldName() != tt.fieldName+"_maxX" {
				t.Errorf("GetMaxXFieldName() = %v, want %v", strategy.GetMaxXFieldName(), tt.fieldName+"_maxX")
			}
			if strategy.GetMinYFieldName() != tt.fieldName+"_minY" {
				t.Errorf("GetMinYFieldName() = %v, want %v", strategy.GetMinYFieldName(), tt.fieldName+"_minY")
			}
			if strategy.GetMaxYFieldName() != tt.fieldName+"_maxY" {
				t.Errorf("GetMaxYFieldName() = %v, want %v", strategy.GetMaxYFieldName(), tt.fieldName+"_maxY")
			}
		})
	}
}

func TestNewBBoxStrategyWithFieldNames(t *testing.T) {
	ctx := NewSpatialContext()

	tests := []struct {
		name         string
		fieldName    string
		minXField    string
		maxXField    string
		minYField    string
		maxYField    string
		ctx          *SpatialContext
		wantErr      bool
		errContains  string
	}{
		{
			name:      "valid custom field names",
			fieldName: "location",
			minXField: "custom_minX",
			maxXField: "custom_maxX",
			minYField: "custom_minY",
			maxYField: "custom_maxY",
			ctx:       ctx,
			wantErr:   false,
		},
		{
			name:        "empty minX field",
			fieldName:   "location",
			minXField:   "",
			maxXField:   "maxX",
			minYField:   "minY",
			maxYField:   "maxY",
			ctx:         ctx,
			wantErr:     true,
			errContains: "all bounding box field names must be non-empty",
		},
		{
			name:        "empty maxX field",
			fieldName:   "location",
			minXField:   "minX",
			maxXField:   "",
			minYField:   "minY",
			maxYField:   "maxY",
			ctx:         ctx,
			wantErr:     true,
			errContains: "all bounding box field names must be non-empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewBBoxStrategyWithFieldNames(
				tt.fieldName, tt.minXField, tt.maxXField, tt.minYField, tt.maxYField, tt.ctx,
			)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewBBoxStrategyWithFieldNames() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewBBoxStrategyWithFieldNames() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewBBoxStrategyWithFieldNames() unexpected error = %v", err)
				return
			}
			if strategy == nil {
				t.Error("NewBBoxStrategyWithFieldNames() returned nil strategy")
				return
			}

			// Verify custom field names
			if strategy.GetMinXFieldName() != tt.minXField {
				t.Errorf("GetMinXFieldName() = %v, want %v", strategy.GetMinXFieldName(), tt.minXField)
			}
			if strategy.GetMaxXFieldName() != tt.maxXField {
				t.Errorf("GetMaxXFieldName() = %v, want %v", strategy.GetMaxXFieldName(), tt.maxXField)
			}
			if strategy.GetMinYFieldName() != tt.minYField {
				t.Errorf("GetMinYFieldName() = %v, want %v", strategy.GetMinYFieldName(), tt.minYField)
			}
			if strategy.GetMaxYFieldName() != tt.maxYField {
				t.Errorf("GetMaxYFieldName() = %v, want %v", strategy.GetMaxYFieldName(), tt.maxYField)
			}
		})
	}
}

func TestBBoxStrategy_CreateIndexableFields(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewBBoxStrategy("location", ctx)

	tests := []struct {
		name        string
		shape       Shape
		wantErr     bool
		errContains string
		fieldCount  int
	}{
		{
			name:       "valid point",
			shape:      NewPoint(10.0, 20.0),
			wantErr:    false,
			fieldCount: 4,
		},
		{
			name:       "valid rectangle",
			shape:      NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:    false,
			fieldCount: 4,
		},
		{
			name:        "coordinates out of bounds",
			shape:       NewPoint(200.0, 0.0), // X > 180
			wantErr:     true,
			errContains: "outside world bounds",
		},
		{
			name:        "min greater than max",
			shape:       NewRectangle(30.0, 20.0, 10.0, 40.0), // minX > maxX
			wantErr:     true,
			errContains: "cannot be greater than",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := strategy.CreateIndexableFields(tt.shape)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateIndexableFields() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("CreateIndexableFields() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("CreateIndexableFields() unexpected error = %v", err)
				return
			}
			if len(fields) != tt.fieldCount {
				t.Errorf("CreateIndexableFields() returned %d fields, want %d", len(fields), tt.fieldCount)
			}
		})
	}
}

func TestBBoxStrategy_MakeQuery(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewBBoxStrategy("location", ctx)

	tests := []struct {
		name        string
		operation   SpatialOperation
		shape       Shape
		wantErr     bool
		errContains string
	}{
		{
			name:      "intersects with rectangle",
			operation: SpatialOperationIntersects,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:      "isWithin with rectangle",
			operation: SpatialOperationIsWithin,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:      "contains with rectangle",
			operation: SpatialOperationContains,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:        "unsupported operation",
			operation:   SpatialOperationEquals,
			shape:       NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:     true,
			errContains: "not supported by BBoxStrategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := strategy.MakeQuery(tt.operation, tt.shape)
			if tt.wantErr {
				if err == nil {
					t.Errorf("MakeQuery() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("MakeQuery() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("MakeQuery() unexpected error = %v", err)
				return
			}
			if query == nil {
				t.Error("MakeQuery() returned nil query")
			}
		})
	}
}

func TestBBoxStrategy_MakeDistanceValueSource(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewBBoxStrategy("location", ctx)
	center := NewPoint(0.0, 0.0)

	vs, err := strategy.MakeDistanceValueSource(center, 1.0)
	if err != nil {
		t.Errorf("MakeDistanceValueSource() unexpected error = %v", err)
		return
	}
	if vs == nil {
		t.Error("MakeDistanceValueSource() returned nil value source")
		return
	}

	// Check description
	desc := vs.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestBBoxDistanceValueSource(t *testing.T) {
	calculator := &HaversineCalculator{}
	vs := NewBBoxDistanceValueSource(
		"minX", "maxX", "minY", "maxY",
		NewPoint(0.0, 0.0),
		1.0,
		calculator,
	)

	if vs == nil {
		t.Error("NewBBoxDistanceValueSource() returned nil")
		return
	}

	// Test description
	desc := vs.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}

	// Test GetValues
	values, err := vs.GetValues(nil)
	if err != nil {
		t.Errorf("GetValues() unexpected error = %v", err)
		return
	}
	if values == nil {
		t.Error("GetValues() returned nil")
		return
	}

	// Test DoubleVal (placeholder implementation returns 0)
	val, err := values.DoubleVal(0)
	if err != nil {
		t.Errorf("DoubleVal() unexpected error = %v", err)
		return
	}
	if val != 0 {
		t.Errorf("DoubleVal() = %v, want 0 (placeholder)", val)
	}
}

func TestBBoxStrategy_BoundingBoxExtraction(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewBBoxStrategy("location", ctx)

	// Test with Point - should use point as bounding box
	point := NewPoint(10.0, 20.0)
	fields, err := strategy.CreateIndexableFields(point)
	if err != nil {
		t.Errorf("CreateIndexableFields() with Point error = %v", err)
		return
	}
	if len(fields) != 4 {
		t.Errorf("CreateIndexableFields() with Point returned %d fields, want 4", len(fields))
	}

	// Test with Rectangle
	rect := NewRectangle(10.0, 20.0, 30.0, 40.0)
	fields, err = strategy.CreateIndexableFields(rect)
	if err != nil {
		t.Errorf("CreateIndexableFields() with Rectangle error = %v", err)
		return
	}
	if len(fields) != 4 {
		t.Errorf("CreateIndexableFields() with Rectangle returned %d fields, want 4", len(fields))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
