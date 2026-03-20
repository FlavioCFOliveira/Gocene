// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewDistanceRangeQuery(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test point (London)
	center := Point{X: -0.1257, Y: 51.5085}
	minDistance := 0.5
	maxDistance := 2.0
	calculator := &HaversineCalculator{}

	// Create the query
	query := NewDistanceRangeQuery("location_grid", center, minDistance, maxDistance, quadTree, 6, calculator)
	if query == nil {
		t.Fatal("NewDistanceRangeQuery() returned nil")
	}

	// Verify query properties
	if query.GetFieldName() != "location_grid" {
		t.Errorf("GetFieldName() = %s, want location_grid", query.GetFieldName())
	}

	if query.GetCenter() != center {
		t.Error("GetCenter() returned wrong center")
	}

	if query.GetMinDistance() != minDistance {
		t.Errorf("GetMinDistance() = %f, want %f", query.GetMinDistance(), minDistance)
	}

	if query.GetMaxDistance() != maxDistance {
		t.Errorf("GetMaxDistance() = %f, want %f", query.GetMaxDistance(), maxDistance)
	}

	// Test Clone
	cloned := query.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}

	// Test Equals
	if !query.Equals(cloned) {
		t.Error("Equals() returned false for identical query")
	}

	// Test String
	s := query.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Test HashCode
	hash := query.HashCode()
	if hash == 0 {
		t.Error("HashCode() returned 0")
	}
}

func TestDistanceRangeQuery_NotEquals(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	calculator := &HaversineCalculator{}

	// Create two different queries
	center1 := Point{X: -0.1257, Y: 51.5085}
	center2 := Point{X: -74.0, Y: 40.7}

	query1 := NewDistanceRangeQuery("location_grid", center1, 0.5, 2.0, quadTree, 6, calculator)
	query2 := NewDistanceRangeQuery("location_grid", center2, 0.5, 2.0, quadTree, 6, calculator)

	// Queries should not be equal
	if query1.Equals(query2) {
		t.Error("Equals() returned true for different queries")
	}
}

func TestNewShapeValues(t *testing.T) {
	// Create a test shape
	shape := NewPoint(0, 0)

	// Create ShapeValues
	sv := NewShapeValues(shape)
	if sv == nil {
		t.Fatal("NewShapeValues() returned nil")
	}

	// Verify shape
	if sv.GetShape() != shape {
		t.Error("GetShape() returned wrong shape")
	}

	// Verify bounding box
	bbox := sv.GetBoundingBox()
	if bbox == nil {
		t.Error("GetBoundingBox() returned nil")
	}

	// Verify center
	center := sv.GetCenter()
	if center.X != 0 || center.Y != 0 {
		t.Error("GetCenter() returned wrong center")
	}
}

func TestShapeValues_CalculateDistance(t *testing.T) {
	// Create a test shape
	shape := NewPoint(0, 0)
	sv := NewShapeValues(shape)

	// Create calculator
	calculator := &HaversineCalculator{}

	// Calculate distance to another point
	point := Point{X: 1, Y: 1}
	distance := sv.CalculateDistance(calculator, point)

	if distance < 0 {
		t.Error("CalculateDistance() returned negative distance")
	}

	// Test with nil calculator
	distance = sv.CalculateDistance(nil, point)
	if distance != -1 {
		t.Error("CalculateDistance() should return -1 with nil calculator")
	}

	// Test with nil shape
	svNil := NewShapeValues(nil)
	distance = svNil.CalculateDistance(calculator, point)
	if distance != -1 {
		t.Error("CalculateDistance() should return -1 with nil shape")
	}
}

func TestShapeValues_String(t *testing.T) {
	shape := NewPoint(0, 0)
	sv := NewShapeValues(shape)

	s := sv.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestNewShapeValuesSource(t *testing.T) {
	// Create a test point
	center := Point{X: -0.1257, Y: 51.5085}
	calculator := &HaversineCalculator{}

	// Create ShapeValuesSource
	svs := NewShapeValuesSource("location", center, calculator)
	if svs == nil {
		t.Fatal("NewShapeValuesSource() returned nil")
	}

	// Verify field name
	if svs.GetFieldName() != "location" {
		t.Errorf("GetFieldName() = %s, want location", svs.GetFieldName())
	}

	// Verify center
	if svs.GetCenter() != center {
		t.Error("GetCenter() returned wrong center")
	}

	// Verify description
	desc := svs.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestDistanceRangeQuery_InterfaceCompliance(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	center := Point{X: -0.1257, Y: 51.5085}
	calculator := &HaversineCalculator{}

	// Create the query
	query := NewDistanceRangeQuery("location_grid", center, 0.5, 2.0, quadTree, 6, calculator)

	// Verify that DistanceRangeQuery implements search.Query
	var _ search.Query = query
}

func TestShapeValues_NilShape(t *testing.T) {
	// Create ShapeValues with nil shape
	sv := NewShapeValues(nil)

	// GetShape should return nil
	if sv.GetShape() != nil {
		t.Error("GetShape() should return nil for nil shape")
	}

	// GetBoundingBox should return nil
	if sv.GetBoundingBox() != nil {
		t.Error("GetBoundingBox() should return nil for nil shape")
	}

	// GetCenter should return empty point
	center := sv.GetCenter()
	if center.X != 0 || center.Y != 0 {
		t.Error("GetCenter() should return empty point for nil shape")
	}
}
