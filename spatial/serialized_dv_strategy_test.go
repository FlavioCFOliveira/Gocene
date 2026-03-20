// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNewSerializedDVStrategy(t *testing.T) {
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
			strategy, err := NewSerializedDVStrategy(tt.fieldName, tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSerializedDVStrategy() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("NewSerializedDVStrategy() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewSerializedDVStrategy() unexpected error = %v", err)
				return
			}
			if strategy == nil {
				t.Error("NewSerializedDVStrategy() returned nil strategy")
				return
			}

			// Verify field name
			if strategy.GetFieldName() != tt.fieldName {
				t.Errorf("GetFieldName() = %v, want %v", strategy.GetFieldName(), tt.fieldName)
			}
			if strategy.GetDVFieldName() != tt.fieldName+"_dv" {
				t.Errorf("GetDVFieldName() = %v, want %v", strategy.GetDVFieldName(), tt.fieldName+"_dv")
			}
		})
	}
}

func TestNewSerializedDVStrategyWithFieldName(t *testing.T) {
	ctx := NewSpatialContext()

	tests := []struct {
		name        string
		fieldName   string
		dvFieldName string
		ctx         *SpatialContext
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid custom field name",
			fieldName:   "location",
			dvFieldName: "custom_dv",
			ctx:         ctx,
			wantErr:     false,
		},
		{
			name:        "empty dv field name",
			fieldName:   "location",
			dvFieldName: "",
			ctx:         ctx,
			wantErr:     true,
			errContains: "docvalues field name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewSerializedDVStrategyWithFieldName(tt.fieldName, tt.dvFieldName, tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSerializedDVStrategyWithFieldName() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("NewSerializedDVStrategyWithFieldName() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewSerializedDVStrategyWithFieldName() unexpected error = %v", err)
				return
			}
			if strategy == nil {
				t.Error("NewSerializedDVStrategyWithFieldName() returned nil strategy")
				return
			}

			if strategy.GetDVFieldName() != tt.dvFieldName {
				t.Errorf("GetDVFieldName() = %v, want %v", strategy.GetDVFieldName(), tt.dvFieldName)
			}
		})
	}
}

func TestSerializedDVStrategy_CreateIndexableFields(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

	tests := []struct {
		name       string
		shape      Shape
		wantErr    bool
		fieldCount int
	}{
		{
			name:       "valid point",
			shape:      NewPoint(10.0, 20.0),
			wantErr:    false,
			fieldCount: 1,
		},
		{
			name:       "valid rectangle",
			shape:      NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:    false,
			fieldCount: 1,
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

func TestSerializedDVStrategy_SerializeDeserialize(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

	tests := []struct {
		name  string
		shape Shape
	}{
		{
			name:  "point",
			shape: NewPoint(10.0, 20.0),
		},
		{
			name:  "rectangle",
			shape: NewRectangle(10.0, 20.0, 30.0, 40.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := strategy.serializeShape(tt.shape)
			if err != nil {
				t.Errorf("serializeShape() error = %v", err)
				return
			}
			if len(data) == 0 {
				t.Error("serializeShape() returned empty data")
				return
			}

			// Deserialize
			restored, err := strategy.deserializeShape(data)
			if err != nil {
				t.Errorf("deserializeShape() error = %v", err)
				return
			}

			// Compare bounding boxes
			originalBbox := tt.shape.GetBoundingBox()
			restoredBbox := restored.GetBoundingBox()

			if originalBbox.MinX != restoredBbox.MinX ||
				originalBbox.MinY != restoredBbox.MinY ||
				originalBbox.MaxX != restoredBbox.MaxX ||
				originalBbox.MaxY != restoredBbox.MaxY {
				t.Errorf("Deserialized shape mismatch: got %v, want %v", restoredBbox, originalBbox)
			}
		})
	}
}

func TestSerializedDVStrategy_DeserializeErrors(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

	tests := []struct {
		name        string
		data        []byte
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty data",
			data:        []byte{},
			wantErr:     true,
			errContains: "empty data",
		},
		{
			name:        "unknown shape type",
			data:        []byte{99}, // Unknown type
			wantErr:     true,
			errContains: "unknown shape type",
		},
		{
			name:        "incomplete point data",
			data:        []byte{1, 0, 0}, // Type 1 (point) but incomplete data
			wantErr:     true,
			errContains: "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := strategy.deserializeShape(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("deserializeShape() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("deserializeShape() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("deserializeShape() unexpected error = %v", err)
			}
		})
	}
}

func TestSerializedDVStrategy_MakeQuery(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

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
			errContains: "not supported by SerializedDVStrategy",
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
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
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

			// Check that query is valid (currently returns MatchAllDocsQuery as placeholder)
			if query == nil {
				t.Error("MakeQuery() returned nil")
			}
		})
	}
}

func TestSerializedDVStrategy_MakeDistanceValueSource(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)
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

func TestSerializedDVQuery(t *testing.T) {
	strategy, _ := NewSerializedDVStrategy("location", NewSpatialContext())

	// Test that MakeQuery returns a valid query
	query, err := strategy.MakeQuery(SpatialOperationIntersects, NewRectangle(10.0, 20.0, 30.0, 40.0))
	if err != nil {
		t.Errorf("MakeQuery() error = %v", err)
		return
	}
	if query == nil {
		t.Error("MakeQuery() returned nil query")
	}
}

func TestSerializedDVDistanceValueSource(t *testing.T) {
	strategy, _ := NewSerializedDVStrategy("location", NewSpatialContext())
	vs := NewSerializedDVDistanceValueSource(
		"location_dv",
		NewPoint(0.0, 0.0),
		1.0,
		&HaversineCalculator{},
		strategy,
	)

	if vs == nil {
		t.Error("NewSerializedDVDistanceValueSource() returned nil")
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

	// Test Exists (before caching)
	exists := values.Exists(0)
	if exists {
		t.Error("Exists() should return false for uncached doc")
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

	// Test Exists (after caching)
	exists = values.Exists(0)
	if !exists {
		t.Error("Exists() should return true for cached doc")
	}
}

func TestSerializedDVDistanceValueSource_Types(t *testing.T) {
	strategy, _ := NewSerializedDVStrategy("location", NewSpatialContext())
	vs := NewSerializedDVDistanceValueSource(
		"location_dv",
		NewPoint(0.0, 0.0),
		1.0,
		&HaversineCalculator{},
		strategy,
	)

	values, _ := vs.GetValues(nil)

	// Test FloatVal
	floatVal, err := values.FloatVal(0)
	if err != nil {
		t.Errorf("FloatVal() error = %v", err)
	}
	if floatVal != 0 {
		t.Errorf("FloatVal() = %v, want 0", floatVal)
	}

	// Test IntVal
	intVal, err := values.IntVal(0)
	if err != nil {
		t.Errorf("IntVal() error = %v", err)
	}
	if intVal != 0 {
		t.Errorf("IntVal() = %v, want 0", intVal)
	}

	// Test LongVal
	longVal, err := values.LongVal(0)
	if err != nil {
		t.Errorf("LongVal() error = %v", err)
	}
	if longVal != 0 {
		t.Errorf("LongVal() = %v, want 0", longVal)
	}

	// Test StrVal
	strVal, err := values.StrVal(0)
	if err != nil {
		t.Errorf("StrVal() error = %v", err)
	}
	if strVal != "0.000000" {
		t.Errorf("StrVal() = %v, want \"0.000000\"", strVal)
	}
}

func TestSerializedDVStrategy_PointSerialization(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

	point := NewPoint(123.456, 78.901)
	data, err := strategy.serializeShape(point)
	if err != nil {
		t.Fatalf("serializeShape() error = %v", err)
	}

	// Verify the serialized format
	if len(data) != 17 { // 1 byte type + 8 bytes X + 8 bytes Y
		t.Errorf("Serialized point length = %d, want 17", len(data))
	}

	// Verify type byte
	if data[0] != 1 {
		t.Errorf("Type byte = %d, want 1", data[0])
	}

	// Verify coordinates
	buf := bytes.NewReader(data[1:])
	var x, y float64
	binary.Read(buf, binary.LittleEndian, &x)
	binary.Read(buf, binary.LittleEndian, &y)

	if x != 123.456 {
		t.Errorf("X coordinate = %f, want 123.456", x)
	}
	if y != 78.901 {
		t.Errorf("Y coordinate = %f, want 78.901", y)
	}
}

func TestSerializedDVStrategy_RectangleSerialization(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewSerializedDVStrategy("location", ctx)

	rect := NewRectangle(10.0, 20.0, 30.0, 40.0)
	data, err := strategy.serializeShape(rect)
	if err != nil {
		t.Fatalf("serializeShape() error = %v", err)
	}

	// Verify the serialized format
	if len(data) != 33 { // 1 byte type + 4 * 8 bytes coordinates
		t.Errorf("Serialized rectangle length = %d, want 33", len(data))
	}

	// Verify type byte
	if data[0] != 2 {
		t.Errorf("Type byte = %d, want 2", data[0])
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}
