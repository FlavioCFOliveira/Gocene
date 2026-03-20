// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestSpatialOperation_String(t *testing.T) {
	tests := []struct {
		op       SpatialOperation
		expected string
	}{
		{SpatialOperationIntersects, "Intersects"},
		{SpatialOperationIsWithin, "IsWithin"},
		{SpatialOperationContains, "Contains"},
		{SpatialOperationIsDisjointTo, "IsDisjointTo"},
		{SpatialOperationEquals, "Equals"},
		{SpatialOperationOverlaps, "Overlaps"},
		{SpatialOperationBboxIntersects, "BBoxIntersects"},
		{SpatialOperationBboxWithin, "BBoxWithin"},
		{SpatialOperation(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.op.String()
			if got != tt.expected {
				t.Errorf("String() = %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestGetSpatialOperationFromString(t *testing.T) {
	tests := []struct {
		input         string
		expectedOp    SpatialOperation
		expectedValid bool
	}{
		{"Intersects", SpatialOperationIntersects, true},
		{"intersects", SpatialOperationIntersects, true},
		{"INTERSECTS", SpatialOperationIntersects, true},
		{"IsWithin", SpatialOperationIsWithin, true},
		{"within", SpatialOperationIsWithin, true},
		{"Contains", SpatialOperationContains, true},
		{"Disjoint", SpatialOperationIsDisjointTo, true},
		{"Equals", SpatialOperationEquals, true},
		{"Overlaps", SpatialOperationOverlaps, true},
		{"BBoxIntersects", SpatialOperationBboxIntersects, true},
		{"BBoxWithin", SpatialOperationBboxWithin, true},
		{"Unknown", SpatialOperationIntersects, false},
		{"", SpatialOperationIntersects, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotOp, gotValid := GetSpatialOperationFromString(tt.input)
			if gotValid != tt.expectedValid {
				t.Errorf("GetSpatialOperationFromString() valid = %v, expected %v", gotValid, tt.expectedValid)
			}
			if gotValid && gotOp != tt.expectedOp {
				t.Errorf("GetSpatialOperationFromString() op = %v, expected %v", gotOp, tt.expectedOp)
			}
		})
	}
}

func TestAllSpatialOperations(t *testing.T) {
	ops := AllSpatialOperations()
	if len(ops) != 8 {
		t.Errorf("AllSpatialOperations() returned %d operations, expected 8", len(ops))
	}

	// Verify all operations are unique
	seen := make(map[SpatialOperation]bool)
	for _, op := range ops {
		if seen[op] {
			t.Errorf("Duplicate operation: %v", op)
		}
		seen[op] = true
	}
}

func TestNewSpatialArgs(t *testing.T) {
	shape := NewPoint(0, 0)
	args := NewSpatialArgs(SpatialOperationIntersects, shape)

	if args == nil {
		t.Fatal("NewSpatialArgs() returned nil")
	}

	if args.Operation != SpatialOperationIntersects {
		t.Errorf("Operation = %v, expected Intersects", args.Operation)
	}

	if args.Shape != shape {
		t.Error("Shape does not match")
	}

	if args.DistErrPct != 0.025 {
		t.Errorf("DistErrPct = %f, expected 0.025", args.DistErrPct)
	}

	if args.DistErr != -1 {
		t.Errorf("DistErr = %f, expected -1", args.DistErr)
	}
}

func TestSpatialArgs_GettersAndSetters(t *testing.T) {
	shape := NewPoint(0, 0)
	args := NewSpatialArgs(SpatialOperationIntersects, shape)

	// Test GetOperation
	if args.GetOperation() != SpatialOperationIntersects {
		t.Error("GetOperation() returned wrong value")
	}

	// Test GetShape
	if args.GetShape() != shape {
		t.Error("GetShape() returned wrong value")
	}

	// Test GetDistErrPct
	if args.GetDistErrPct() != 0.025 {
		t.Error("GetDistErrPct() returned wrong value")
	}

	// Test SetDistErrPct
	args.SetDistErrPct(0.05)
	if args.GetDistErrPct() != 0.05 {
		t.Error("SetDistErrPct() did not work")
	}

	// Test SetDistErr
	args.SetDistErr(1.0)
	if args.DistErr != 1.0 {
		t.Error("SetDistErr() did not work")
	}
}

func TestSpatialArgs_GetDistErr(t *testing.T) {
	ctx := NewSpatialContext()

	tests := []struct {
		name       string
		shape      Shape
		distErrPct float64
		distErr    float64
		wantErr    float64
		tolerance  float64
	}{
		{
			name:       "Explicit distErr",
			shape:      NewRectangle(-1, -1, 1, 1),
			distErrPct: 0.025,
			distErr:    0.5,
			wantErr:    0.5,
			tolerance:  0.001,
		},
		{
			name:       "Calculated from DistErrPct",
			shape:      NewRectangle(-10, -10, 10, 10),
			distErrPct: 0.1,
			distErr:    -1,
			wantErr:    2.0, // 20 * 0.1
			tolerance:  0.001,
		},
		{
			name:       "No shape",
			shape:      nil,
			distErrPct: 0.025,
			distErr:    -1,
			wantErr:    0,
			tolerance:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := NewSpatialArgs(SpatialOperationIntersects, tt.shape)
			args.SetDistErrPct(tt.distErrPct)
			args.SetDistErr(tt.distErr)

			got := args.GetDistErr(ctx)
			if got < tt.wantErr-tt.tolerance || got > tt.wantErr+tt.tolerance {
				t.Errorf("GetDistErr() = %f, want %f (tolerance %f)", got, tt.wantErr, tt.tolerance)
			}
		})
	}
}

func TestSpatialArgs_String(t *testing.T) {
	shape := NewPoint(0, 0)
	args := NewSpatialArgs(SpatialOperationIntersects, shape)

	s := args.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Should contain operation name
	if !containsStr(s, "Intersects") {
		t.Error("String() should contain operation name")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewSpatialArgsParser(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialArgsParser(ctx)

	if parser == nil {
		t.Fatal("NewSpatialArgsParser() returned nil")
	}

	if parser.ctx != ctx {
		t.Error("Parser context does not match")
	}
}

func TestSpatialArgsParser_Parse(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialArgsParser(ctx)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Point Intersects",
			input:   "Intersects(POINT(-1 1))",
			wantErr: false,
		},
		{
			name:    "Point IsWithin",
			input:   "IsWithin(POINT(-1 1))",
			wantErr: false,
		},
		{
			name:    "Point Contains",
			input:   "Contains(POINT(-1 1))",
			wantErr: false,
		},
		{
			name:    "Circle",
			input:   "Intersects(CIRCLE(-1 1 d=10))",
			wantErr: false,
		},
		{
			name:    "Envelope",
			input:   "Intersects(ENVELOPE(-1, 1, 1, -1))",
			wantErr: false,
		},
		{
			name:    "BBOX",
			input:   "Intersects(BBOX(-1, 1, 1, -1))",
			wantErr: false,
		},
		{
			name:    "Rectangle",
			input:   "Intersects(RECTANGLE(-1, 1, -1, 1))",
			wantErr: false,
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Missing opening paren",
			input:   "Intersects POINT(-1 1)",
			wantErr: true,
		},
		{
			name:    "Missing closing paren",
			input:   "Intersects(POINT(-1 1)",
			wantErr: true,
		},
		{
			name:    "Unknown operation",
			input:   "Unknown(POINT(-1 1))",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parser.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && args == nil {
				t.Error("Parse() returned nil without error")
			}
		})
	}
}

func TestSpatialArgsParser_ParsePoint(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialArgsParser(ctx)

	tests := []struct {
		name    string
		input   string
		wantX   float64
		wantY   float64
		wantErr bool
	}{
		{
			name:    "Valid point",
			input:   "Intersects(POINT(-1 1))",
			wantX:   -1,
			wantY:   1,
			wantErr: false,
		},
		{
			name:    "Point with extra spaces",
			input:   "Intersects( POINT ( -1   1 ) )",
			wantX:   -1,
			wantY:   1,
			wantErr: false,
		},
		{
			name:    "Point with only one coordinate",
			input:   "Intersects(POINT(-1))",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parser.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				point, ok := args.Shape.(Point)
				if !ok {
					t.Error("Shape is not a Point")
					return
				}
				if point.X != tt.wantX {
					t.Errorf("X = %f, want %f", point.X, tt.wantX)
				}
				if point.Y != tt.wantY {
					t.Errorf("Y = %f, want %f", point.Y, tt.wantY)
				}
			}
		})
	}
}

func TestSpatialArgsParser_ParseCircle(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialArgsParser(ctx)

	tests := []struct {
		name        string
		input       string
		wantCenterX float64
		wantCenterY float64
		wantRadius  float64
		wantErr     bool
	}{
		{
			name:        "Circle with d parameter",
			input:       "Intersects(CIRCLE(-1 1 d=10))",
			wantCenterX: -1,
			wantCenterY: 1,
			wantRadius:  10,
			wantErr:     false,
		},
		{
			name:        "Circle with radius parameter",
			input:       "Intersects(CIRCLE(-1 1 radius=5))",
			wantCenterX: -1,
			wantCenterY: 1,
			wantRadius:  5,
			wantErr:     false,
		},
		{
			name:        "Circle without radius",
			input:       "Intersects(CIRCLE(-1 1))",
			wantCenterX: -1,
			wantCenterY: 1,
			wantRadius:  1, // Default radius
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parser.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				circle, ok := args.Shape.(*Circle)
				if !ok {
					t.Error("Shape is not a Circle")
					return
				}
				if circle.Center.X != tt.wantCenterX {
					t.Errorf("Center.X = %f, want %f", circle.Center.X, tt.wantCenterX)
				}
				if circle.Center.Y != tt.wantCenterY {
					t.Errorf("Center.Y = %f, want %f", circle.Center.Y, tt.wantCenterY)
				}
				if circle.Radius != tt.wantRadius {
					t.Errorf("Radius = %f, want %f", circle.Radius, tt.wantRadius)
				}
			}
		})
	}
}

func TestSpatialArgsParser_ParseEnvelope(t *testing.T) {
	ctx := NewSpatialContext()
	parser := NewSpatialArgsParser(ctx)

	tests := []struct {
		name    string
		input   string
		minX    float64
		maxX    float64
		minY    float64
		maxY    float64
		wantErr bool
	}{
		{
			name:    "Valid envelope",
			input:   "Intersects(ENVELOPE(-1, 1, 1, -1))",
			minX:    -1,
			maxX:    1,
			minY:    -1,
			maxY:    1,
			wantErr: false,
		},
		{
			name:    "Valid BBOX",
			input:   "Intersects(BBOX(-1, 1, 1, -1))",
			minX:    -1,
			maxX:    1,
			minY:    -1,
			maxY:    1,
			wantErr: false,
		},
		{
			name:    "Envelope with only 3 values",
			input:   "Intersects(ENVELOPE(-1, 1, 1))",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parser.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				rect, ok := args.Shape.(*Rectangle)
				if !ok {
					t.Error("Shape is not a Rectangle")
					return
				}
				if rect.MinX != tt.minX {
					t.Errorf("MinX = %f, want %f", rect.MinX, tt.minX)
				}
				if rect.MaxX != tt.maxX {
					t.Errorf("MaxX = %f, want %f", rect.MaxX, tt.maxX)
				}
				if rect.MinY != tt.minY {
					t.Errorf("MinY = %f, want %f", rect.MinY, tt.minY)
				}
				if rect.MaxY != tt.maxY {
					t.Errorf("MaxY = %f, want %f", rect.MaxY, tt.maxY)
				}
			}
		})
	}
}

func TestNewCircle(t *testing.T) {
	circle := NewCircle(0, 0, 5)
	if circle == nil {
		t.Fatal("NewCircle() returned nil")
	}

	if circle.Center.X != 0 || circle.Center.Y != 0 {
		t.Error("Center coordinates do not match")
	}

	if circle.Radius != 5 {
		t.Errorf("Radius = %f, expected 5", circle.Radius)
	}
}

func TestCircle_GetBoundingBox(t *testing.T) {
	circle := NewCircle(0, 0, 5)
	bbox := circle.GetBoundingBox()

	if bbox == nil {
		t.Fatal("GetBoundingBox() returned nil")
	}

	if bbox.MinX != -5 || bbox.MaxX != 5 {
		t.Errorf("X bounds = [%f, %f], expected [-5, 5]", bbox.MinX, bbox.MaxX)
	}

	if bbox.MinY != -5 || bbox.MaxY != 5 {
		t.Errorf("Y bounds = [%f, %f], expected [-5, 5]", bbox.MinY, bbox.MaxY)
	}
}

func TestCircle_Intersects(t *testing.T) {
	circle := NewCircle(0, 0, 5)

	tests := []struct {
		name     string
		shape    Shape
		expected bool
	}{
		{
			name:     "Point inside",
			shape:    NewPoint(0, 0),
			expected: true,
		},
		{
			name:     "Point outside",
			shape:    NewPoint(10, 10),
			expected: false,
		},
		{
			name:     "Rectangle overlapping",
			shape:    NewRectangle(-2, -2, 2, 2),
			expected: true,
		},
		{
			name:     "Rectangle far away",
			shape:    NewRectangle(100, 100, 110, 110),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := circle.Intersects(tt.shape)
			if result != tt.expected {
				t.Errorf("Intersects() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCircle_Contains(t *testing.T) {
	circle := NewCircle(0, 0, 5)

	tests := []struct {
		name     string
		shape    Shape
		expected bool
	}{
		{
			name:     "Point inside",
			shape:    NewPoint(0, 0),
			expected: true,
		},
		{
			name:     "Point outside",
			shape:    NewPoint(10, 10),
			expected: false,
		},
		{
			name:     "Small rectangle inside",
			shape:    NewRectangle(-1, -1, 1, 1),
			expected: true,
		},
		{
			name:     "Large rectangle not fully inside",
			shape:    NewRectangle(-10, -10, 10, 10),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := circle.Contains(tt.shape)
			if result != tt.expected {
				t.Errorf("Contains() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCircle_String(t *testing.T) {
	circle := NewCircle(0, 0, 5)
	s := circle.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}
