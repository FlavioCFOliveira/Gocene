// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewPointVectorStrategy(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("failed to create strategy: %v", err)
	}

	if strategy.GetFieldName() != "location" {
		t.Errorf("expected field name 'location', got %s", strategy.GetFieldName())
	}

	if strategy.GetXFieldName() != "location_x" {
		t.Errorf("expected X field name 'location_x', got %s", strategy.GetXFieldName())
	}

	if strategy.GetYFieldName() != "location_y" {
		t.Errorf("expected Y field name 'location_y', got %s", strategy.GetYFieldName())
	}
}

func TestNewPointVectorStrategyWithFieldNames(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategyWithFieldNames("location", "lon", "lat", ctx)
	if err != nil {
		t.Fatalf("failed to create strategy: %v", err)
	}

	if strategy.GetXFieldName() != "lon" {
		t.Errorf("expected X field name 'lon', got %s", strategy.GetXFieldName())
	}

	if strategy.GetYFieldName() != "lat" {
		t.Errorf("expected Y field name 'lat', got %s", strategy.GetYFieldName())
	}
}

func TestPointVectorStrategyCreateIndexableFields(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Test with Point
	point := NewPoint(-9.1393, 38.7223) // Lisbon
	fields, err := strategy.CreateIndexableFields(point)
	if err != nil {
		t.Fatalf("failed to create indexable fields: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}

	// Check field names
	hasXField := false
	hasYField := false
	for _, field := range fields {
		if field.Name() == "location_x" {
			hasXField = true
		}
		if field.Name() == "location_y" {
			hasYField = true
		}
	}

	if !hasXField {
		t.Error("missing X field")
	}
	if !hasYField {
		t.Error("missing Y field")
	}
}

func TestPointVectorStrategyCreateIndexableFieldsFromRectangle(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Test with Rectangle (should use center)
	rect := NewRectangle(0, 0, 10, 10)
	fields, err := strategy.CreateIndexableFields(rect)
	if err != nil {
		t.Fatalf("failed to create indexable fields from rectangle: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestPointVectorStrategyCreateIndexableFieldsOutOfBounds(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Test with out-of-bounds coordinates
	point := NewPoint(200, 100) // Outside world bounds
	_, err := strategy.CreateIndexableFields(point)
	if err == nil {
		t.Error("expected error for out-of-bounds coordinates")
	}

	point2 := NewPoint(-200, -100)
	_, err = strategy.CreateIndexableFields(point2)
	if err == nil {
		t.Error("expected error for out-of-bounds coordinates")
	}
}

func TestPointVectorStrategyMakeQueryIntersects(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Create a bounding box query
	bbox := NewRectangle(-10, 35, -5, 45)
	query, err := strategy.MakeQuery(SpatialOperationIntersects, bbox)
	if err != nil {
		t.Fatalf("failed to create query: %v", err)
	}

	if query == nil {
		t.Fatal("expected non-nil query")
	}

	// Should be a BooleanQuery
	bq, ok := query.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery, got %T", query)
	}

	clauses := bq.Clauses()
	if len(clauses) != 2 {
		t.Errorf("expected 2 clauses, got %d", len(clauses))
	}
}

func TestPointVectorStrategyMakeQueryIsWithin(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	bbox := NewRectangle(-10, 35, -5, 45)
	query, err := strategy.MakeQuery(SpatialOperationIsWithin, bbox)
	if err != nil {
		t.Fatalf("failed to create query: %v", err)
	}

	if query == nil {
		t.Fatal("expected non-nil query")
	}
}

func TestPointVectorStrategyMakeQueryContains(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	bbox := NewRectangle(-10, 35, -5, 45)
	query, err := strategy.MakeQuery(SpatialOperationContains, bbox)
	if err != nil {
		t.Fatalf("failed to create query: %v", err)
	}

	if query == nil {
		t.Fatal("expected non-nil query")
	}
}

func TestPointVectorStrategyMakeQueryUnsupported(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Test unsupported operation
	bbox := NewRectangle(-10, 35, -5, 45)
	_, err := strategy.MakeQuery(SpatialOperationIsDisjointTo, bbox)
	if err == nil {
		t.Error("expected error for unsupported operation")
	}

	_, err = strategy.MakeQuery(SpatialOperationEquals, bbox)
	if err == nil {
		t.Error("expected error for unsupported operation")
	}
}

func TestPointVectorStrategyMakeDistanceValueSource(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	center := NewPoint(-9.1393, 38.7223) // Lisbon
	vs, err := strategy.MakeDistanceValueSource(center, 1.0)
	if err != nil {
		t.Fatalf("failed to create distance value source: %v", err)
	}

	if vs == nil {
		t.Fatal("expected non-nil value source")
	}

	desc := vs.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestDistanceValueSource(t *testing.T) {
	vs := NewDistanceValueSource("x_field", "y_field", NewPoint(0, 0), 1.0, &HaversineCalculator{})

	desc := vs.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if desc != "distance(x_field,y_field from Point(0.000000, 0.000000))" {
		t.Errorf("unexpected description: %s", desc)
	}
}

func TestPointVectorStrategyValidation(t *testing.T) {
	// Test with empty field name
	_, err := NewPointVectorStrategy("", NewSpatialContext())
	if err == nil {
		t.Error("expected error for empty field name")
	}

	// Test with nil context
	_, err = NewPointVectorStrategy("location", nil)
	if err == nil {
		t.Error("expected error for nil context")
	}

	// Test with empty custom field names
	_, err = NewPointVectorStrategyWithFieldNames("location", "", "lat", NewSpatialContext())
	if err == nil {
		t.Error("expected error for empty X field name")
	}

	_, err = NewPointVectorStrategyWithFieldNames("location", "lon", "", NewSpatialContext())
	if err == nil {
		t.Error("expected error for empty Y field name")
	}
}

func TestCartesianPointVectorStrategy(t *testing.T) {
	ctx := NewSpatialContextCartesian(0, 0, 100, 100)
	strategy, _ := NewPointVectorStrategy("location", ctx)

	point := NewPoint(50, 50)
	fields, err := strategy.CreateIndexableFields(point)
	if err != nil {
		t.Fatalf("failed to create fields: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

// BenchmarkCreateIndexableFields benchmarks field creation
func BenchmarkCreateIndexableFields(b *testing.B) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	point := NewPoint(-9.1393, 38.7223)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.CreateIndexableFields(point)
	}
}

// BenchmarkMakeQuery benchmarks query creation
func BenchmarkMakeQuery(b *testing.B) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	bbox := NewRectangle(-10, 35, -5, 45)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.MakeQuery(SpatialOperationIntersects, bbox)
	}
}

// BenchmarkDistanceValueSource benchmarks value source creation
func BenchmarkDistanceValueSource(b *testing.B) {
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	center := NewPoint(-9.1393, 38.7223)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.MakeDistanceValueSource(center, 1.0)
	}
}
