// Package document implements org.apache.lucene.sandbox.document.
package document

import "math/big"

// BigIntegerPoint indexes a *big.Int value as a sortable Point. Mirrors
// org.apache.lucene.sandbox.document.BigIntegerPoint.
type BigIntegerPoint struct {
	Field string
	Value *big.Int
}

// NewBigIntegerPoint builds the field.
func NewBigIntegerPoint(field string, value *big.Int) *BigIntegerPoint {
	return &BigIntegerPoint{Field: field, Value: new(big.Int).Set(value)}
}

// HalfFloatPoint indexes a half-precision (16-bit) float as a Point. Mirrors
// org.apache.lucene.sandbox.document.HalfFloatPoint.
type HalfFloatPoint struct {
	Field string
	Value float32
}

// NewHalfFloatPoint builds the field.
func NewHalfFloatPoint(field string, value float32) *HalfFloatPoint {
	return &HalfFloatPoint{Field: field, Value: value}
}

// LatLonBoundingBox indexes a bounding-box-typed Point. Mirrors
// org.apache.lucene.sandbox.document.LatLonBoundingBox.
type LatLonBoundingBox struct {
	Field  string
	MinLat float64
	MinLon float64
	MaxLat float64
	MaxLon float64
}

// NewLatLonBoundingBox builds the field.
func NewLatLonBoundingBox(field string, minLat, minLon, maxLat, maxLon float64) *LatLonBoundingBox {
	return &LatLonBoundingBox{Field: field, MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}
}

// FloatPointNearestNeighbor scans a slice of float32 points for the nearest
// neighbour of a query point. Mirrors
// org.apache.lucene.sandbox.document.FloatPointNearestNeighbor.
type FloatPointNearestNeighbor struct{}

// Search returns the indices of the k nearest neighbours of target.
func (FloatPointNearestNeighbor) Search(points [][]float32, target []float32, k int) []int {
	if k < 1 {
		k = 1
	}
	type pair struct {
		idx int
		d   float64
	}
	scored := make([]pair, len(points))
	for i, p := range points {
		var sum float64
		for j := range p {
			diff := float64(p[j] - target[j])
			sum += diff * diff
		}
		scored[i] = pair{idx: i, d: sum}
	}
	// selection sort first k
	if k > len(scored) {
		k = len(scored)
	}
	for i := 0; i < k; i++ {
		minIdx := i
		for j := i + 1; j < len(scored); j++ {
			if scored[j].d < scored[minIdx].d {
				minIdx = j
			}
		}
		scored[i], scored[minIdx] = scored[minIdx], scored[i]
	}
	out := make([]int, k)
	for i := 0; i < k; i++ {
		out[i] = scored[i].idx
	}
	return out
}

// DoublePointMultiRangeBuilder accumulates per-dim ranges before lowering to
// a single PointRangeQuery. Mirrors the homonymous Java builder.
type DoublePointMultiRangeBuilder struct {
	Field  string
	Ranges []DoublePointRange
}

// DoublePointRange is one (min, max) pair per dimension.
type DoublePointRange struct {
	Min []float64
	Max []float64
}

// NewDoublePointMultiRangeBuilder builds the builder.
func NewDoublePointMultiRangeBuilder(field string) *DoublePointMultiRangeBuilder {
	return &DoublePointMultiRangeBuilder{Field: field}
}

// AddRange records a (min, max) tuple.
func (b *DoublePointMultiRangeBuilder) AddRange(min, max []float64) {
	b.Ranges = append(b.Ranges, DoublePointRange{Min: append([]float64(nil), min...), Max: append([]float64(nil), max...)})
}

// FloatPointMultiRangeBuilder is the float32 counterpart.
type FloatPointMultiRangeBuilder struct {
	Field  string
	Ranges []FloatPointRange
}

// FloatPointRange is the float32 (min, max) pair.
type FloatPointRange struct {
	Min []float32
	Max []float32
}

// NewFloatPointMultiRangeBuilder builds the builder.
func NewFloatPointMultiRangeBuilder(field string) *FloatPointMultiRangeBuilder {
	return &FloatPointMultiRangeBuilder{Field: field}
}

// AddRange records a (min, max) tuple.
func (b *FloatPointMultiRangeBuilder) AddRange(min, max []float32) {
	b.Ranges = append(b.Ranges, FloatPointRange{Min: append([]float32(nil), min...), Max: append([]float32(nil), max...)})
}

// IntPointMultiRangeBuilder is the int32 variant.
type IntPointMultiRangeBuilder struct {
	Field  string
	Ranges []IntPointRange
}

// IntPointRange is the int32 (min, max) pair.
type IntPointRange struct {
	Min []int32
	Max []int32
}

// NewIntPointMultiRangeBuilder builds the builder.
func NewIntPointMultiRangeBuilder(field string) *IntPointMultiRangeBuilder {
	return &IntPointMultiRangeBuilder{Field: field}
}

// AddRange records a (min, max) tuple.
func (b *IntPointMultiRangeBuilder) AddRange(min, max []int32) {
	b.Ranges = append(b.Ranges, IntPointRange{Min: append([]int32(nil), min...), Max: append([]int32(nil), max...)})
}

// LongPointMultiRangeBuilder is the int64 variant.
type LongPointMultiRangeBuilder struct {
	Field  string
	Ranges []LongPointRange
}

// LongPointRange is the int64 (min, max) pair.
type LongPointRange struct {
	Min []int64
	Max []int64
}

// NewLongPointMultiRangeBuilder builds the builder.
func NewLongPointMultiRangeBuilder(field string) *LongPointMultiRangeBuilder {
	return &LongPointMultiRangeBuilder{Field: field}
}

// AddRange records a (min, max) tuple.
func (b *LongPointMultiRangeBuilder) AddRange(min, max []int64) {
	b.Ranges = append(b.Ranges, LongPointRange{Min: append([]int64(nil), min...), Max: append([]int64(nil), max...)})
}
