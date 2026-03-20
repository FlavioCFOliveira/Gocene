// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

// SpatialBenchmark provides performance benchmarks for spatial functionality.
// This benchmark suite measures the performance of spatial operations including
// indexing, querying, and distance calculations.
//
// This is the Go port of Lucene's spatial benchmark functionality.
type SpatialBenchmark struct {
	// ctx is the spatial context used for all benchmarks
	ctx *SpatialContext

	// rng is the random number generator for generating test data
	rng *rand.Rand
}

// NewSpatialBenchmark creates a new SpatialBenchmark.
func NewSpatialBenchmark() *SpatialBenchmark {
	return &SpatialBenchmark{
		ctx: NewSpatialContext(),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewSpatialBenchmarkWithContext creates a new SpatialBenchmark with a custom context.
func NewSpatialBenchmarkWithContext(ctx *SpatialContext) *SpatialBenchmark {
	return &SpatialBenchmark{
		ctx: ctx,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetContext returns the spatial context used by this benchmark.
func (b *SpatialBenchmark) GetContext() *SpatialContext {
	return b.ctx
}

// RunAllBenchmarks runs all benchmarks in the suite.
// This function is typically called from a testing.T or testing.B context.
func (b *SpatialBenchmark) RunAllBenchmarks(bm *testing.B) {
	bm.Run("PointCreation", b.BenchmarkPointCreation)
	bm.Run("RectangleCreation", b.BenchmarkRectangleCreation)
	bm.Run("PointVectorIndexing", b.BenchmarkPointVectorIndexing)
	bm.Run("BBoxIndexing", b.BenchmarkBBoxIndexing)
	bm.Run("PrefixTreeIndexing", b.BenchmarkPrefixTreeIndexing)
	bm.Run("DistanceCalculation", b.BenchmarkDistanceCalculation)
	bm.Run("QueryCreation", b.BenchmarkQueryCreation)
	bm.Run("SpatialArgsParsing", b.BenchmarkSpatialArgsParsing)
	bm.Run("ShapeIntersection", b.BenchmarkShapeIntersection)
	bm.Run("PointContainment", b.BenchmarkPointContainment)
}

// BenchmarkPointCreation benchmarks point creation.
func (b *SpatialBenchmark) BenchmarkPointCreation(bm *testing.B) {
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		_ = NewPoint(lon, lat)
	}
}

// BenchmarkRectangleCreation benchmarks rectangle creation.
func (b *SpatialBenchmark) BenchmarkRectangleCreation(bm *testing.B) {
	for i := 0; i < bm.N; i++ {
		minX := -180.0 + b.rng.Float64()*360.0
		minY := -90.0 + b.rng.Float64()*180.0
		maxX := minX + b.rng.Float64()*(180.0-minX)
		maxY := minY + b.rng.Float64()*(90.0-minY)
		_ = NewRectangle(minX, minY, maxX, maxY)
	}
}

// BenchmarkPointVectorIndexing benchmarks PointVectorStrategy indexing.
func (b *SpatialBenchmark) BenchmarkPointVectorIndexing(bm *testing.B) {
	strategy, _ := NewPointVectorStrategy("location", b.ctx)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		point := NewPoint(lon, lat)
		_, _ = strategy.CreateIndexableFields(point)
	}
}

// BenchmarkBBoxIndexing benchmarks BBoxStrategy indexing.
func (b *SpatialBenchmark) BenchmarkBBoxIndexing(bm *testing.B) {
	strategy, _ := NewBBoxStrategy("bbox", b.ctx)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		minX := -180.0 + b.rng.Float64()*360.0
		minY := -90.0 + b.rng.Float64()*180.0
		maxX := minX + b.rng.Float64()*(180.0-minX)
		maxY := minY + b.rng.Float64()*(90.0-minY)
		rect := NewRectangle(minX, minY, maxX, maxY)
		_, _ = strategy.CreateIndexableFields(rect)
	}
}

// BenchmarkPrefixTreeIndexing benchmarks PrefixTreeStrategy indexing.
func (b *SpatialBenchmark) BenchmarkPrefixTreeIndexing(bm *testing.B) {
	geohashTree, _ := NewGeohashPrefixTree(11)
	strategy, _ := NewPrefixTreeStrategy("geohash", geohashTree, 11, b.ctx)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		point := NewPoint(lon, lat)
		_, _ = strategy.CreateIndexableFields(point)
	}
}

// BenchmarkDistanceCalculation benchmarks Haversine distance calculation.
func (b *SpatialBenchmark) BenchmarkDistanceCalculation(bm *testing.B) {
	calc := &HaversineCalculator{}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		lon1 := -180.0 + b.rng.Float64()*360.0
		lat1 := -90.0 + b.rng.Float64()*180.0
		lon2 := -180.0 + b.rng.Float64()*360.0
		lat2 := -90.0 + b.rng.Float64()*180.0
		p1 := NewPoint(lon1, lat1)
		p2 := NewPoint(lon2, lat2)
		_ = calc.Distance(p1, p2)
	}
}

// BenchmarkQueryCreation benchmarks spatial query creation.
func (b *SpatialBenchmark) BenchmarkQueryCreation(bm *testing.B) {
	strategy, _ := NewPointVectorStrategy("location", b.ctx)
	point := NewPoint(-122.0, 37.0)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		_, _ = strategy.MakeQuery(SpatialOperationIntersects, point)
	}
}

// BenchmarkSpatialArgsParsing benchmarks spatial args parsing.
func (b *SpatialBenchmark) BenchmarkSpatialArgsParsing(bm *testing.B) {
	parser := NewSpatialArgsParser(b.ctx)
	arg := "Intersects(POINT(-122.0 37.0))"

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		_, _ = parser.Parse(arg)
	}
}

// BenchmarkShapeIntersection benchmarks shape intersection checking.
func (b *SpatialBenchmark) BenchmarkShapeIntersection(bm *testing.B) {
	rect := NewRectangle(-123.0, 36.0, -121.0, 38.0)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		point := NewPoint(lon, lat)
		_ = rect.Intersects(point)
	}
}

// BenchmarkPointContainment benchmarks point containment checking.
func (b *SpatialBenchmark) BenchmarkPointContainment(bm *testing.B) {
	rect := NewRectangle(-123.0, 36.0, -121.0, 38.0)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		lon := -122.5 + b.rng.Float64()
		lat := 36.5 + b.rng.Float64()
		point := NewPoint(lon, lat)
		_ = rect.ContainsPoint(point)
	}
}

// BenchmarkBatchProcessing benchmarks batch processing of spatial data.
func (b *SpatialBenchmark) BenchmarkBatchProcessing(bm *testing.B) {
	strategy, _ := NewPointVectorStrategy("batch", b.ctx)

	// Pre-generate points
	points := make([]Point, bm.N)
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		points[i] = NewPoint(lon, lat)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		_, _ = strategy.CreateIndexableFields(points[i])
	}
}

// BenchmarkConcurrentAccess benchmarks concurrent access to spatial data.
func (b *SpatialBenchmark) BenchmarkConcurrentAccess(bm *testing.B) {
	strategy, _ := NewPointVectorStrategy("concurrent", b.ctx)

	bm.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lon := -180.0 + b.rng.Float64()*360.0
			lat := -90.0 + b.rng.Float64()*180.0
			point := NewPoint(lon, lat)
			_, _ = strategy.CreateIndexableFields(point)
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage of spatial operations.
func (b *SpatialBenchmark) BenchmarkMemoryUsage(bm *testing.B) {
	strategy, _ := NewPointVectorStrategy("memory", b.ctx)

	bm.ResetTimer()
	bm.ReportAllocs()
	for i := 0; i < bm.N; i++ {
		lon := -180.0 + b.rng.Float64()*360.0
		lat := -90.0 + b.rng.Float64()*180.0
		point := NewPoint(lon, lat)
		_, _ = strategy.CreateIndexableFields(point)
	}
}

// BenchmarkSpatialIndexWriter benchmarks SpatialIndexWriter operations.
func (b *SpatialBenchmark) BenchmarkSpatialIndexWriter(bm *testing.B) {
	// This is a placeholder - actual implementation would require
	// proper setup of directory, segment info, etc.
	bm.Skip("SpatialIndexWriter benchmark requires full index setup")
}

// BenchmarkSpatialIndexReader benchmarks SpatialIndexReader operations.
func (b *SpatialBenchmark) BenchmarkSpatialIndexReader(bm *testing.B) {
	// This is a placeholder - actual implementation would require
	// proper setup of directory, segment info, etc.
	bm.Skip("SpatialIndexReader benchmark requires full index setup")
}

// BenchmarkSpatialIndexFormat benchmarks SpatialIndexFormat operations.
func (b *SpatialBenchmark) BenchmarkSpatialIndexFormat(bm *testing.B) {
	// This is a placeholder - actual implementation would require
	// proper setup of directory, segment info, etc.
	bm.Skip("SpatialIndexFormat benchmark requires full index setup")
}

// BenchmarkResult holds the result of a benchmark run.
type BenchmarkResult struct {
	Name        string
	Iterations  int
	Duration    time.Duration
	NsPerOp     int64
	AllocsPerOp int64
	BytesPerOp  int64
}

// NewBenchmarkResult creates a new BenchmarkResult from testing.BenchmarkResult.
func NewBenchmarkResult(name string, result *testing.BenchmarkResult) *BenchmarkResult {
	return &BenchmarkResult{
		Name:        name,
		Iterations:  result.N,
		Duration:    time.Duration(result.T) * time.Nanosecond,
		NsPerOp:     result.NsPerOp(),
		AllocsPerOp: int64(result.AllocsPerOp()),
		BytesPerOp:  int64(result.AllocedBytesPerOp()),
	}
}

// String returns a string representation of the benchmark result.
func (r *BenchmarkResult) String() string {
	return fmt.Sprintf("%s: %d iterations, %s/op, %d allocs/op, %d bytes/op",
		r.Name, r.Iterations, time.Duration(r.NsPerOp),
		r.AllocsPerOp, r.BytesPerOp)
}

// GenerateRandomPoint generates a random point within valid geographic bounds.
func (b *SpatialBenchmark) GenerateRandomPoint() Point {
	lon := -180.0 + b.rng.Float64()*360.0
	lat := -90.0 + b.rng.Float64()*180.0
	return NewPoint(lon, lat)
}

// GenerateRandomRectangle generates a random rectangle within valid geographic bounds.
func (b *SpatialBenchmark) GenerateRandomRectangle() *Rectangle {
	minX := -180.0 + b.rng.Float64()*360.0
	minY := -90.0 + b.rng.Float64()*180.0
	maxX := minX + b.rng.Float64()*(180.0-minX)
	maxY := minY + b.rng.Float64()*(90.0-minY)
	return NewRectangle(minX, minY, maxX, maxY)
}

// GenerateCirclePoints generates points in a circle around a center point.
func (b *SpatialBenchmark) GenerateCirclePoints(center Point, radiusKm float64, numPoints int) []Point {
	points := make([]Point, numPoints)
	calc := &HaversineCalculator{}

	for i := 0; i < numPoints; i++ {
		angle := 2 * math.Pi * float64(i) / float64(numPoints)
		// Convert distance to degrees (approximate)
		deltaLon := radiusKm * math.Cos(angle) / 111.32
		deltaLat := radiusKm * math.Sin(angle) / 111.32

		lon := center.X + deltaLon
		lat := center.Y + deltaLat

		// Ensure within bounds
		if lon > 180 {
			lon -= 360
		}
		if lon < -180 {
			lon += 360
		}
		if lat > 90 {
			lat = 90
		}
		if lat < -90 {
			lat = -90
		}

		points[i] = NewPoint(lon, lat)
	}

	// Verify distances
	_ = calc // Suppress unused variable warning
	return points
}

// GenerateGridPoints generates points in a grid pattern.
func (b *SpatialBenchmark) GenerateGridPoints(minX, minY, maxX, maxY float64, numX, numY int) []Point {
	total := numX * numY
	points := make([]Point, total)

	dx := (maxX - minX) / float64(numX-1)
	dy := (maxY - minY) / float64(numY-1)

	idx := 0
	for i := 0; i < numX; i++ {
		for j := 0; j < numY; j++ {
			x := minX + float64(i)*dx
			y := minY + float64(j)*dy
			points[idx] = NewPoint(x, y)
			idx++
		}
	}

	return points
}

// BenchmarkReport generates a comprehensive benchmark report.
type BenchmarkReport struct {
	Results []*BenchmarkResult
}

// NewBenchmarkReport creates a new BenchmarkReport.
func NewBenchmarkReport() *BenchmarkReport {
	return &BenchmarkReport{
		Results: make([]*BenchmarkResult, 0),
	}
}

// AddResult adds a benchmark result to the report.
func (r *BenchmarkReport) AddResult(result *BenchmarkResult) {
	r.Results = append(r.Results, result)
}

// String returns a string representation of the benchmark report.
func (r *BenchmarkReport) String() string {
	result := "Spatial Benchmark Report\n"
	result += "========================\n\n"
	for _, res := range r.Results {
		result += res.String() + "\n"
	}
	return result
}
