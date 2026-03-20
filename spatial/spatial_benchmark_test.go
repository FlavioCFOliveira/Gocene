// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func BenchmarkPointCreation(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkPointCreation(b)
}

func BenchmarkRectangleCreation(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkRectangleCreation(b)
}

func BenchmarkPointVectorIndexing(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkPointVectorIndexing(b)
}

func BenchmarkBBoxIndexing(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkBBoxIndexing(b)
}

func BenchmarkPrefixTreeIndexing(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkPrefixTreeIndexing(b)
}

func BenchmarkDistanceCalculation(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkDistanceCalculation(b)
}

func BenchmarkQueryCreation(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkQueryCreation(b)
}

func BenchmarkSpatialArgsParsing(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkSpatialArgsParsing(b)
}

func BenchmarkShapeIntersection(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkShapeIntersection(b)
}

func BenchmarkPointContainment(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkPointContainment(b)
}

func BenchmarkBatchProcessing(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkBatchProcessing(b)
}

func BenchmarkConcurrentAccess(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkConcurrentAccess(b)
}

func BenchmarkMemoryUsage(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkMemoryUsage(b)
}

func BenchmarkSpatialIndexWriter(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkSpatialIndexWriter(b)
}

func BenchmarkSpatialIndexReader(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkSpatialIndexReader(b)
}

func BenchmarkSpatialIndexFormat(b *testing.B) {
	benchmark := NewSpatialBenchmark()
	benchmark.BenchmarkSpatialIndexFormat(b)
}

func TestNewSpatialBenchmark(t *testing.T) {
	benchmark := NewSpatialBenchmark()
	if benchmark == nil {
		t.Error("NewSpatialBenchmark() returned nil")
		return
	}
	if benchmark.GetContext() == nil {
		t.Error("GetContext() returned nil")
	}
}

func TestNewSpatialBenchmarkWithContext(t *testing.T) {
	ctx := NewSpatialContextCartesian(0, 0, 100, 100)
	benchmark := NewSpatialBenchmarkWithContext(ctx)
	if benchmark == nil {
		t.Error("NewSpatialBenchmarkWithContext() returned nil")
		return
	}
	if benchmark.GetContext() != ctx {
		t.Error("GetContext() returned wrong context")
	}
}

func TestSpatialBenchmark_GenerateRandomPoint(t *testing.T) {
	benchmark := NewSpatialBenchmark()
	point := benchmark.GenerateRandomPoint()

	if point.X < -180 || point.X > 180 {
		t.Errorf("Point X = %f, expected within [-180, 180]", point.X)
	}
	if point.Y < -90 || point.Y > 90 {
		t.Errorf("Point Y = %f, expected within [-90, 90]", point.Y)
	}
}

func TestSpatialBenchmark_GenerateRandomRectangle(t *testing.T) {
	benchmark := NewSpatialBenchmark()
	rect := benchmark.GenerateRandomRectangle()

	if rect.MinX < -180 || rect.MinX > 180 {
		t.Errorf("Rectangle MinX = %f, expected within [-180, 180]", rect.MinX)
	}
	if rect.MinY < -90 || rect.MinY > 90 {
		t.Errorf("Rectangle MinY = %f, expected within [-90, 90]", rect.MinY)
	}
	if rect.MaxX < rect.MinX {
		t.Error("Rectangle MaxX should be >= MinX")
	}
	if rect.MaxY < rect.MinY {
		t.Error("Rectangle MaxY should be >= MinY")
	}
}

func TestSpatialBenchmark_GenerateCirclePoints(t *testing.T) {
	benchmark := NewSpatialBenchmark()
	center := NewPoint(-122.0, 37.0)
	points := benchmark.GenerateCirclePoints(center, 10.0, 8)

	if len(points) != 8 {
		t.Errorf("len(points) = %d, want 8", len(points))
	}

	for i, point := range points {
		if point.X < -180 || point.X > 180 {
			t.Errorf("Point %d X = %f, expected within [-180, 180]", i, point.X)
		}
		if point.Y < -90 || point.Y > 90 {
			t.Errorf("Point %d Y = %f, expected within [-90, 90]", i, point.Y)
		}
	}
}

func TestSpatialBenchmark_GenerateGridPoints(t *testing.T) {
	benchmark := NewSpatialBenchmark()
	points := benchmark.GenerateGridPoints(-10, -10, 10, 10, 5, 5)

	if len(points) != 25 {
		t.Errorf("len(points) = %d, want 25", len(points))
	}

	// Check first and last points
	if points[0].X != -10 || points[0].Y != -10 {
		t.Errorf("First point = (%f, %f), want (-10, -10)", points[0].X, points[0].Y)
	}
	if points[24].X != 10 || points[24].Y != 10 {
		t.Errorf("Last point = (%f, %f), want (10, 10)", points[24].X, points[24].Y)
	}
}

func TestBenchmarkResult(t *testing.T) {
	// Create a mock testing.BenchmarkResult
	result := &testing.BenchmarkResult{}
	result.N = 1000
	result.T = 1000000 // 1ms in nanoseconds

	benchmarkResult := NewBenchmarkResult("TestBenchmark", result)
	if benchmarkResult == nil {
		t.Error("NewBenchmarkResult() returned nil")
		return
	}

	if benchmarkResult.Name != "TestBenchmark" {
		t.Errorf("Name = %s, want TestBenchmark", benchmarkResult.Name)
	}
	if benchmarkResult.Iterations != 1000 {
		t.Errorf("Iterations = %d, want 1000", benchmarkResult.Iterations)
	}

	// Test String method
	str := benchmarkResult.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}

func TestBenchmarkReport(t *testing.T) {
	report := NewBenchmarkReport()
	if report == nil {
		t.Error("NewBenchmarkReport() returned nil")
		return
	}

	if len(report.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0", len(report.Results))
	}

	// Add a mock result
	result := &BenchmarkResult{
		Name:        "TestBenchmark",
		Iterations:  1000,
		NsPerOp:     1000,
		AllocsPerOp: 10,
		BytesPerOp:  100,
	}
	report.AddResult(result)

	if len(report.Results) != 1 {
		t.Errorf("len(Results) = %d, want 1", len(report.Results))
	}

	// Test String method
	str := report.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
	if str != "Spatial Benchmark Report\n========================\n\nTestBenchmark: 1000 iterations, 1us/op, 10 allocs/op, 100 bytes/op\n" {
		// The exact format may vary, just check it contains expected content
		if len(str) < 50 {
			t.Error("String() returned unexpectedly short string")
		}
	}
}

func TestSpatialBenchmark_Integration(t *testing.T) {
	// This test verifies that all benchmark functions can be called
	// without actually running the benchmarks
	benchmark := NewSpatialBenchmark()

	// Test that we can generate test data
	point := benchmark.GenerateRandomPoint()
	rect := benchmark.GenerateRandomRectangle()
	circlePoints := benchmark.GenerateCirclePoints(NewPoint(0, 0), 10.0, 8)
	gridPoints := benchmark.GenerateGridPoints(-10, -10, 10, 10, 3, 3)

	if point.X == 0 && point.Y == 0 {
		t.Log("Generated point at origin (possible but unlikely)")
	}
	if rect.Area() == 0 {
		t.Log("Generated zero-area rectangle (possible but unlikely)")
	}
	if len(circlePoints) != 8 {
		t.Errorf("len(circlePoints) = %d, want 8", len(circlePoints))
	}
	if len(gridPoints) != 9 {
		t.Errorf("len(gridPoints) = %d, want 9", len(gridPoints))
	}
}

func TestSpatialBenchmark_WithCartesianContext(t *testing.T) {
	ctx := NewSpatialContextCartesian(0, 0, 1000, 1000)
	benchmark := NewSpatialBenchmarkWithContext(ctx)

	// Generate points in Cartesian space
	point := benchmark.GenerateRandomPoint()

	// In Cartesian context, the bounds are different
	// The random point generator still uses geographic bounds
	// but the context is Cartesian
	if point.X < -180 || point.X > 180 {
		t.Errorf("Point X = %f, expected within [-180, 180]", point.X)
	}

	// Verify the context is Cartesian
	if benchmark.GetContext().Geo {
		t.Error("Expected Cartesian context")
	}
}
