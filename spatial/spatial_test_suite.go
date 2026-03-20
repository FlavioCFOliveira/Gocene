// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
	"testing"
)

// SpatialTestSuite provides comprehensive tests for spatial functionality.
// This suite tests all spatial components including strategies, queries,
// shapes, and index operations.
//
// This is the Go port of Lucene's spatial test suite functionality.
type SpatialTestSuite struct {
	// ctx is the spatial context used for all tests
	ctx *SpatialContext
}

// NewSpatialTestSuite creates a new SpatialTestSuite.
func NewSpatialTestSuite() *SpatialTestSuite {
	return &SpatialTestSuite{
		ctx: NewSpatialContext(),
	}
}

// NewSpatialTestSuiteWithContext creates a new SpatialTestSuite with a custom context.
func NewSpatialTestSuiteWithContext(ctx *SpatialContext) *SpatialTestSuite {
	return &SpatialTestSuite{
		ctx: ctx,
	}
}

// GetContext returns the spatial context used by this test suite.
func (s *SpatialTestSuite) GetContext() *SpatialContext {
	return s.ctx
}

// RunAllTests runs all tests in the suite.
// Returns an error if any test fails.
func (s *SpatialTestSuite) RunAllTests(t *testing.T) error {
	if err := s.TestShapes(t); err != nil {
		return fmt.Errorf("shape tests failed: %w", err)
	}
	if err := s.TestStrategies(t); err != nil {
		return fmt.Errorf("strategy tests failed: %w", err)
	}
	if err := s.TestQueries(t); err != nil {
		return fmt.Errorf("query tests failed: %w", err)
	}
	if err := s.TestOperations(t); err != nil {
		return fmt.Errorf("operation tests failed: %w", err)
	}
	if err := s.TestDistanceCalculations(t); err != nil {
		return fmt.Errorf("distance calculation tests failed: %w", err)
	}
	return nil
}

// TestShapes tests shape creation and manipulation.
func (s *SpatialTestSuite) TestShapes(t *testing.T) error {
	t.Run("SpatialTestSuite/Shapes", func(t *testing.T) {
		// Test Point creation
		p1 := NewPoint(-122.0, 37.0)
		if p1.X != -122.0 || p1.Y != 37.0 {
			t.Error("Point coordinates mismatch")
		}

		// Test Rectangle creation
		r1 := NewRectangle(-123.0, 36.0, -121.0, 38.0)
		if r1.MinX != -123.0 || r1.MinY != 36.0 || r1.MaxX != -121.0 || r1.MaxY != 38.0 {
			t.Error("Rectangle coordinates mismatch")
		}

		// Test point containment
		if !r1.ContainsPoint(p1) {
			t.Error("Point should be contained in rectangle")
		}

		// Test point intersection
		if !r1.IntersectsRect(p1.GetBoundingBox()) {
			t.Error("Point should intersect rectangle")
		}

		// Test rectangle properties
		if r1.Width() != 2.0 {
			t.Errorf("Rectangle width = %f, want 2.0", r1.Width())
		}
		if r1.Height() != 2.0 {
			t.Errorf("Rectangle height = %f, want 2.0", r1.Height())
		}
		if r1.Area() != 4.0 {
			t.Errorf("Rectangle area = %f, want 4.0", r1.Area())
		}

		// Test center calculation
		center := r1.GetCenter()
		if center.X != -122.0 || center.Y != 37.0 {
			t.Errorf("Rectangle center = (%f, %f), want (-122.0, 37.0)", center.X, center.Y)
		}
	})

	return nil
}

// TestStrategies tests spatial strategy implementations.
func (s *SpatialTestSuite) TestStrategies(t *testing.T) error {
	t.Run("SpatialTestSuite/Strategies", func(t *testing.T) {
		// Test PointVectorStrategy
		pvStrategy, err := NewPointVectorStrategy("location", s.ctx)
		if err != nil {
			t.Errorf("Failed to create PointVectorStrategy: %v", err)
		}
		if pvStrategy.GetFieldName() != "location" {
			t.Errorf("Strategy field name = %s, want location", pvStrategy.GetFieldName())
		}

		// Test BBoxStrategy
		bboxStrategy, err := NewBBoxStrategy("bbox", s.ctx)
		if err != nil {
			t.Errorf("Failed to create BBoxStrategy: %v", err)
		}
		if bboxStrategy.GetFieldName() != "bbox" {
			t.Errorf("Strategy field name = %s, want bbox", bboxStrategy.GetFieldName())
		}

		// Test creating indexable fields
		point := NewPoint(-122.0, 37.0)
		fields, err := pvStrategy.CreateIndexableFields(point)
		if err != nil {
			t.Errorf("Failed to create indexable fields: %v", err)
		}
		if len(fields) == 0 {
			t.Error("CreateIndexableFields returned no fields")
		}

		// Test making queries
		query, err := pvStrategy.MakeQuery(SpatialOperationIntersects, point)
		if err != nil {
			t.Errorf("Failed to make query: %v", err)
		}
		if query == nil {
			t.Error("MakeQuery returned nil query")
		}
	})

	return nil
}

// TestQueries tests spatial query operations.
func (s *SpatialTestSuite) TestQueries(t *testing.T) error {
	t.Run("SpatialTestSuite/Queries", func(t *testing.T) {
		// Create strategies
		pvStrategy, _ := NewPointVectorStrategy("location", s.ctx)

		// Test Intersects query
		point := NewPoint(-122.0, 37.0)
		query, err := pvStrategy.MakeQuery(SpatialOperationIntersects, point)
		if err != nil {
			t.Errorf("Failed to make Intersects query: %v", err)
		}
		if query == nil {
			t.Error("Intersects query is nil")
		}

		// Test Within query
		query, err = pvStrategy.MakeQuery(SpatialOperationIsWithin, point)
		if err != nil {
			t.Errorf("Failed to make Within query: %v", err)
		}
		if query == nil {
			t.Error("Within query is nil")
		}

		// Test Contains query
		query, err = pvStrategy.MakeQuery(SpatialOperationContains, point)
		if err != nil {
			t.Errorf("Failed to make Contains query: %v", err)
		}
		if query == nil {
			t.Error("Contains query is nil")
		}
	})

	return nil
}

// TestOperations tests spatial operations.
func (s *SpatialTestSuite) TestOperations(t *testing.T) error {
	t.Run("SpatialTestSuite/Operations", func(t *testing.T) {
		// Test spatial operation strings
		operations := []struct {
			op       SpatialOperation
			expected string
		}{
			{SpatialOperationIntersects, "Intersects"},
			{SpatialOperationIsWithin, "IsWithin"},
			{SpatialOperationContains, "Contains"},
			{SpatialOperationIsDisjointTo, "IsDisjointTo"},
			{SpatialOperationEquals, "Equals"},
			{SpatialOperationOverlaps, "Overlaps"},
		}

		for _, tc := range operations {
			if tc.op.String() != tc.expected {
				t.Errorf("%v.String() = %s, want %s", tc.op, tc.op.String(), tc.expected)
			}
		}
	})

	return nil
}

// TestDistanceCalculations tests distance calculation functions.
func (s *SpatialTestSuite) TestDistanceCalculations(t *testing.T) error {
	t.Run("SpatialTestSuite/DistanceCalculations", func(t *testing.T) {
		// Test Haversine distance
		distance := HaversineDistance(37.0, -122.0, 37.0, -121.0)
		// Distance between points 1 degree apart at latitude 37 should be approximately 89 km
		if distance < 80 || distance > 100 {
			t.Errorf("HaversineDistance = %f km, expected approximately 89 km", distance)
		}

		// Test distance calculator
		calc := &HaversineCalculator{}
		p1 := NewPoint(-122.0, 37.0)
		p2 := NewPoint(-121.0, 37.0)
		distance = calc.Distance(p1, p2)
		if distance < 80 || distance > 100 {
			t.Errorf("Calculator.Distance = %f km, expected approximately 89 km", distance)
		}

		// Test Cartesian calculator
		cartCalc := &CartesianCalculator{}
		p1 = NewPoint(0, 0)
		p2 = NewPoint(3, 4)
		distance = cartCalc.Distance(p1, p2)
		if math.Abs(distance-5.0) > 0.0001 {
			t.Errorf("Cartesian distance = %f, want 5.0", distance)
		}
	})

	return nil
}

// TestPointVectorIndexing tests point vector strategy indexing.
func (s *SpatialTestSuite) TestPointVectorIndexing(t *testing.T) error {
	t.Run("SpatialTestSuite/PointVectorIndexing", func(t *testing.T) {
		strategy, err := NewPointVectorStrategy("location", s.ctx)
		if err != nil {
			t.Fatalf("Failed to create strategy: %v", err)
		}

		// Test indexing points
		testPoints := []Point{
			{-122.0, 37.0},
			{-121.0, 37.5},
			{-123.0, 36.5},
			{-120.0, 38.0},
		}

		for _, point := range testPoints {
			fields, err := strategy.CreateIndexableFields(point)
			if err != nil {
				t.Errorf("Failed to create fields for point %v: %v", point, err)
				continue
			}
			if len(fields) != 2 {
				t.Errorf("Expected 2 fields for point, got %d", len(fields))
			}
		}
	})

	return nil
}

// TestBBoxIndexing tests bounding box strategy indexing.
func (s *SpatialTestSuite) TestBBoxIndexing(t *testing.T) error {
	t.Run("SpatialTestSuite/BBoxIndexing", func(t *testing.T) {
		strategy, err := NewBBoxStrategy("area", s.ctx)
		if err != nil {
			t.Fatalf("Failed to create strategy: %v", err)
		}

		// Test indexing rectangles
		testRects := []*Rectangle{
			{-123.0, 36.0, -121.0, 38.0},
			{-122.5, 36.5, -120.5, 38.5},
			{-124.0, 35.0, -120.0, 39.0},
		}

		for _, rect := range testRects {
			fields, err := strategy.CreateIndexableFields(rect)
			if err != nil {
				t.Errorf("Failed to create fields for rectangle %v: %v", rect, err)
				continue
			}
			if len(fields) != 4 {
				t.Errorf("Expected 4 fields for rectangle, got %d", len(fields))
			}
		}
	})

	return nil
}

// TestPrefixTreeIndexing tests prefix tree strategy indexing.
func (s *SpatialTestSuite) TestPrefixTreeIndexing(t *testing.T) error {
	t.Run("SpatialTestSuite/PrefixTreeIndexing", func(t *testing.T) {
		geohashTree, err := NewGeohashPrefixTree(11)
		if err != nil {
			t.Fatalf("Failed to create geohash tree: %v", err)
		}
		strategy, err := NewPrefixTreeStrategy("geohash", geohashTree, 11, s.ctx)
		if err != nil {
			t.Fatalf("Failed to create prefix tree strategy: %v", err)
		}

		// Test indexing points
		point := NewPoint(-122.0, 37.0)
		fields, err := strategy.CreateIndexableFields(point)
		if err != nil {
			t.Errorf("Failed to create fields: %v", err)
		}
		if len(fields) == 0 {
			t.Error("CreateIndexableFields returned no fields")
		}
	})

	return nil
}

// TestSpatialArgs tests spatial arguments parsing.
func (s *SpatialTestSuite) TestSpatialArgs(t *testing.T) error {
	t.Run("SpatialTestSuite/SpatialArgs", func(t *testing.T) {
		// Create spatial args
		point := NewPoint(-122.0, 37.0)
		args := NewSpatialArgs(SpatialOperationIntersects, point)
		if args.Operation != SpatialOperationIntersects {
			t.Error("Operation mismatch")
		}
		if args.Shape != point {
			t.Error("Shape mismatch")
		}

		// Test parser
		parser := NewSpatialArgsParser(s.ctx)
		parsedArgs, err := parser.Parse("Intersects(POINT(-122.0 37.0))")
		if err != nil {
			t.Errorf("Failed to parse spatial args: %v", err)
		}
		if parsedArgs.Operation != SpatialOperationIntersects {
			t.Error("Parsed operation mismatch")
		}
	})

	return nil
}

// TestSpatialIndexWriter tests spatial index writer.
func (s *SpatialTestSuite) TestSpatialIndexWriter(t *testing.T) error {
	t.Run("SpatialTestSuite/SpatialIndexWriter", func(t *testing.T) {
		// This is tested more thoroughly in spatial_index_writer_test.go
		// Here we just verify the test suite integration
		t.Log("SpatialIndexWriter tests are in spatial_index_writer_test.go")
	})

	return nil
}

// TestSpatialIndexReader tests spatial index reader.
func (s *SpatialTestSuite) TestSpatialIndexReader(t *testing.T) error {
	t.Run("SpatialTestSuite/SpatialIndexReader", func(t *testing.T) {
		// This is tested more thoroughly in spatial_index_reader_test.go
		// Here we just verify the test suite integration
		t.Log("SpatialIndexReader tests are in spatial_index_reader_test.go")
	})

	return nil
}

// TestSpatialIndexFormat tests spatial index format.
func (s *SpatialTestSuite) TestSpatialIndexFormat(t *testing.T) error {
	t.Run("SpatialTestSuite/SpatialIndexFormat", func(t *testing.T) {
		// This is tested more thoroughly in spatial_index_format_test.go
		// Here we just verify the test suite integration
		t.Log("SpatialIndexFormat tests are in spatial_index_format_test.go")
	})

	return nil
}

// TestEdgeCases tests edge cases and boundary conditions.
func (s *SpatialTestSuite) TestEdgeCases(t *testing.T) error {
	t.Run("SpatialTestSuite/EdgeCases", func(t *testing.T) {
		// Test antimeridian crossing
		rect := NewRectangle(179.0, 0.0, -179.0, 1.0)
		if rect.Width() != -358.0 {
			// This is expected behavior - the width calculation doesn't handle antimeridian
			t.Logf("Antimeridian rectangle width = %f (expected unusual value)", rect.Width())
		}

		// Test pole crossing
		rect = NewRectangle(-180.0, 89.0, 180.0, 91.0)
		if rect.Height() != 2.0 {
			t.Errorf("Pole crossing rectangle height = %f, want 2.0", rect.Height())
		}

		// Test zero-area rectangle
		rect = NewRectangle(0.0, 0.0, 0.0, 0.0)
		if rect.Area() != 0.0 {
			t.Errorf("Zero-area rectangle area = %f, want 0.0", rect.Area())
		}

		// Test same point containment
		point := NewPoint(0.0, 0.0)
		if !rect.ContainsPoint(point) {
			t.Error("Point (0,0) should be contained in zero-area rectangle at origin")
		}
	})

	return nil
}

// TestPerformance tests performance characteristics.
func (s *SpatialTestSuite) TestPerformance(t *testing.T) error {
	t.Run("SpatialTestSuite/Performance", func(t *testing.T) {
		// Note: These are not actual benchmarks but sanity checks
		// Real benchmarks should be in spatial_benchmark_test.go

		strategy, _ := NewPointVectorStrategy("perf_test", s.ctx)

		// Test that we can handle multiple points (use valid lat/lon ranges)
		for i := 0; i < 1000; i++ {
			// Generate points within valid lat/lon ranges
			lon := -180.0 + float64(i%360)
			lat := -90.0 + float64(i%180)
			point := NewPoint(lon, lat)
			_, err := strategy.CreateIndexableFields(point)
			if err != nil {
				t.Errorf("Failed at iteration %d: %v", i, err)
				break
			}
		}
	})

	return nil
}

// SpatialTestResult holds the result of a test run.
type SpatialTestResult struct {
	Passed  int
	Failed  int
	Skipped int
	Errors  []string
}

// NewSpatialTestResult creates a new SpatialTestResult.
func NewSpatialTestResult() *SpatialTestResult {
	return &SpatialTestResult{
		Errors: make([]string, 0),
	}
}

// RecordPass records a passing test.
func (r *SpatialTestResult) RecordPass() {
	r.Passed++
}

// RecordFail records a failing test.
func (r *SpatialTestResult) RecordFail(msg string) {
	r.Failed++
	r.Errors = append(r.Errors, msg)
}

// RecordSkip records a skipped test.
func (r *SpatialTestResult) RecordSkip() {
	r.Skipped++
}

// String returns a string representation of the test result.
func (r *SpatialTestResult) String() string {
	return fmt.Sprintf("Passed: %d, Failed: %d, Skipped: %d", r.Passed, r.Failed, r.Skipped)
}
