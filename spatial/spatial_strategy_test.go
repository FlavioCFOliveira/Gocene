// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestPoint(t *testing.T) {
	p := NewPoint(10.5, 20.3)

	if p.X != 10.5 {
		t.Errorf("expected X to be 10.5, got %f", p.X)
	}
	if p.Y != 20.3 {
		t.Errorf("expected Y to be 20.3, got %f", p.Y)
	}

	str := p.String()
	expected := "Point(10.500000, 20.300000)"
	if str != expected {
		t.Errorf("expected String() to return %s, got %s", expected, str)
	}
}

func TestRectangle(t *testing.T) {
	r := NewRectangle(0, 0, 10, 10)

	if r.MinX != 0 || r.MinY != 0 || r.MaxX != 10 || r.MaxY != 10 {
		t.Errorf("rectangle bounds incorrect: %v", r)
	}

	center := r.Center()
	if center.X != 5 || center.Y != 5 {
		t.Errorf("expected center (5, 5), got (%f, %f)", center.X, center.Y)
	}

	if !r.ContainsPoint(NewPoint(5, 5)) {
		t.Error("expected rectangle to contain point (5, 5)")
	}

	if r.ContainsPoint(NewPoint(15, 15)) {
		t.Error("expected rectangle to not contain point (15, 15)")
	}

	// Test width and height
	if r.Width() != 10 {
		t.Errorf("expected width 10, got %f", r.Width())
	}
	if r.Height() != 10 {
		t.Errorf("expected height 10, got %f", r.Height())
	}
	if r.Area() != 100 {
		t.Errorf("expected area 100, got %f", r.Area())
	}
}

func TestRectangleIntersects(t *testing.T) {
	r1 := NewRectangle(0, 0, 10, 10)

	// Overlapping rectangle
	r2 := NewRectangle(5, 5, 15, 15)
	if !r1.Intersects(r2) {
		t.Error("expected rectangles to intersect")
	}

	// Non-overlapping rectangle
	r3 := NewRectangle(20, 20, 30, 30)
	if r1.Intersects(r3) {
		t.Error("expected rectangles to not intersect")
	}

	// Touching edges - should intersect
	r4 := NewRectangle(10, 0, 20, 10)
	if !r1.Intersects(r4) {
		t.Error("expected touching rectangles to intersect")
	}
}

func TestSpatialContext(t *testing.T) {
	// Test default geographic context
	ctx := NewSpatialContext()
	if !ctx.Geo {
		t.Error("expected Geo to be true")
	}
	if ctx.WorldBounds.MinX != -180 {
		t.Errorf("expected MinX -180, got %f", ctx.WorldBounds.MinX)
	}
	if ctx.WorldBounds.MaxY != 90 {
		t.Errorf("expected MaxY 90, got %f", ctx.WorldBounds.MaxY)
	}
	if ctx.Calculator == nil {
		t.Error("expected Calculator to not be nil")
	}

	// Test Cartesian context
	ctxCart := NewSpatialContextCartesian(0, 0, 100, 100)
	if ctxCart.Geo {
		t.Error("expected Geo to be false for Cartesian")
	}
	if ctxCart.WorldBounds.MaxX != 100 {
		t.Errorf("expected MaxX 100, got %f", ctxCart.WorldBounds.MaxX)
	}
}

func TestBaseSpatialStrategy(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewBaseSpatialStrategy("location", ctx)
	if err != nil {
		t.Fatalf("failed to create strategy: %v", err)
	}

	if strategy.GetFieldName() != "location" {
		t.Errorf("expected field name 'location', got %s", strategy.GetFieldName())
	}

	if strategy.GetSpatialContext() != ctx {
		t.Error("expected spatial context to match")
	}
}

func TestBaseSpatialStrategyValidation(t *testing.T) {
	// Test empty field name
	_, err := NewBaseSpatialStrategy("", NewSpatialContext())
	if err == nil {
		t.Error("expected error for empty field name")
	}

	// Test nil context
	_, err = NewBaseSpatialStrategy("location", nil)
	if err == nil {
		t.Error("expected error for nil context")
	}
}

func TestSpatialOperation(t *testing.T) {
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
		{SpatialOperation(999), "Unknown"},
	}

	for _, test := range tests {
		result := test.op.String()
		if result != test.expected {
			t.Errorf("expected %s for operation %d, got %s", test.expected, test.op, result)
		}
	}
}

func TestHaversineDistance(t *testing.T) {
	// Test known distance: Lisbon to New York (approximately)
	// Lisbon: 38.7223° N, 9.1393° W
	// New York: 40.7128° N, 74.0060° W
	distance := HaversineDistance(38.7223, -9.1393, 40.7128, -74.0060)

	// Expected distance is approximately 5436 km
	// Allow 5% tolerance
	expected := 5436.0
	tolerance := expected * 0.05

	if distance < expected-tolerance || distance > expected+tolerance {
		t.Errorf("distance %f km is outside tolerance of expected %f km", distance, expected)
	}

	// Test same point - should be 0
	distanceZero := HaversineDistance(0, 0, 0, 0)
	if distanceZero != 0 {
		t.Errorf("expected distance 0 for same point, got %f", distanceZero)
	}
}

func TestHaversineCalculator(t *testing.T) {
	calc := &HaversineCalculator{}

	p1 := NewPoint(-9.1393, 38.7223)  // Lisbon
	p2 := NewPoint(-74.0060, 40.7128) // New York

	distance := calc.Distance(p1, p2)
	expected := 5436.0
	tolerance := expected * 0.05

	if distance < expected-tolerance || distance > expected+tolerance {
		t.Errorf("calculator distance %f km is outside tolerance", distance)
	}

	// Test DistanceFromDegrees
	distDeg := calc.DistanceFromDegrees(1.0)
	// At the equator, 1 degree is approximately 111.32 km
	expectedDeg := 111.32
	toleranceDeg := expectedDeg * 0.05
	if distDeg < expectedDeg-toleranceDeg || distDeg > expectedDeg+toleranceDeg {
		t.Errorf("DistanceFromDegrees %f is outside tolerance of expected %f", distDeg, expectedDeg)
	}
}

func TestCartesianCalculator(t *testing.T) {
	calc := &CartesianCalculator{}

	p1 := NewPoint(0, 0)
	p2 := NewPoint(3, 4)

	// Distance should be 5 (3-4-5 triangle)
	distance := calc.Distance(p1, p2)
	if distance != 5 {
		t.Errorf("expected distance 5 for 3-4-5 triangle, got %f", distance)
	}

	// DistanceFromDegrees should return same value
	distDeg := calc.DistanceFromDegrees(5.0)
	if distDeg != 5.0 {
		t.Errorf("expected DistanceFromDegrees to return same value, got %f", distDeg)
	}
}

func TestMathHelpers(t *testing.T) {
	// Test sqrt
	if sqrt(16) != 4 {
		t.Errorf("expected sqrt(16) = 4, got %f", sqrt(16))
	}
	if sqrt(0) != 0 {
		t.Errorf("expected sqrt(0) = 0, got %f", sqrt(0))
	}

	// Test sin (approximate)
	pi := 3.141592653589793
	sinPi2 := sin(pi / 2)
	if sinPi2 < 0.99 || sinPi2 > 1.01 {
		t.Errorf("expected sin(pi/2) ≈ 1, got %f", sinPi2)
	}

	// Test cos (approximate)
	cosZero := cos(0)
	if cosZero < 0.99 || cosZero > 1.01 {
		t.Errorf("expected cos(0) ≈ 1, got %f", cosZero)
	}
}

// MockShape is a simple shape implementation for testing
type MockShape struct {
	bbox   *Rectangle
	center Point
}

func (m *MockShape) GetBoundingBox() *Rectangle {
	return m.bbox
}

func (m *MockShape) GetCenter() Point {
	return m.center
}

func (m *MockShape) Intersects(other Shape) bool {
	return m.bbox.Intersects(other.GetBoundingBox())
}

func (m *MockShape) Contains(other Shape) bool {
	// Contains: this shape fully contains the other shape
	otherBbox := other.GetBoundingBox()
	return m.bbox.MinX <= otherBbox.MinX && m.bbox.MaxX >= otherBbox.MaxX &&
		m.bbox.MinY <= otherBbox.MinY && m.bbox.MaxY >= otherBbox.MaxY
}

func (m *MockShape) IsWithin(other Shape) bool {
	// IsWithin: this shape is fully within the other shape
	otherBbox := other.GetBoundingBox()
	return otherBbox.MinX <= m.bbox.MinX && otherBbox.MaxX >= m.bbox.MaxX &&
		otherBbox.MinY <= m.bbox.MinY && otherBbox.MaxY >= m.bbox.MaxY
}

func (m *MockShape) String() string {
	return "MockShape"
}

func TestShapeInterface(t *testing.T) {
	mock1 := &MockShape{
		bbox:   NewRectangle(0, 0, 10, 10),
		center: Point{X: 5, Y: 5},
	}

	mock2 := &MockShape{
		bbox:   NewRectangle(5, 5, 15, 15),
		center: Point{X: 10, Y: 10},
	}

	// Test GetBoundingBox
	if mock1.GetBoundingBox().MinX != 0 {
		t.Error("GetBoundingBox failed")
	}

	// Test GetCenter
	if mock1.GetCenter().X != 5 || mock1.GetCenter().Y != 5 {
		t.Error("GetCenter failed")
	}

	// Test Intersects
	if !mock1.Intersects(mock2) {
		t.Error("expected shapes to intersect")
	}

	// Test Contains (mock1 does not fully contain mock2)
	if mock1.Contains(mock2) {
		t.Error("expected mock1 to not contain mock2")
	}

	// Test IsWithin (mock1 is not fully within mock2)
	if mock1.IsWithin(mock2) {
		t.Error("expected mock1 to not be within mock2")
	}

	// Create a shape that IS contained within mock1
	mockSmall := &MockShape{
		bbox:   NewRectangle(2, 2, 8, 8),
		center: Point{X: 5, Y: 5},
	}
	if !mock1.Contains(mockSmall) {
		t.Error("expected mock1 to contain mockSmall")
	}
	if !mockSmall.IsWithin(mock1) {
		t.Error("expected mockSmall to be within mock1")
	}
}

func BenchmarkHaversineDistance(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HaversineDistance(38.7223, -9.1393, 40.7128, -74.0060)
	}
}

func BenchmarkPointDistance(b *testing.B) {
	calc := &HaversineCalculator{}
	p1 := NewPoint(-9.1393, 38.7223)
	p2 := NewPoint(-74.0060, 40.7128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Distance(p1, p2)
	}
}

func BenchmarkRectangleIntersects(b *testing.B) {
	r1 := NewRectangle(0, 0, 10, 10)
	r2 := NewRectangle(5, 5, 15, 15)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r1.Intersects(r2)
	}
}
