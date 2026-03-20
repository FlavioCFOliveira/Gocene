// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
)

// BKDTree implements a Block KD-Tree for multi-dimensional point indexing.
// This is the Lucene implementation of a BKD tree for efficient range queries
// on numeric and multi-dimensional data.
type BKDTree struct {
	numDims             int // Number of dimensions
	bytesPerDim         int // Bytes per dimension
	maxPointsInLeafNode int
	minPackedValue      []byte // Min value per dimension
	maxPackedValue      []byte // Max value per dimension
	pointCount          int
}

// NewBKDTree creates a new BKDTree with the specified dimensions.
// numDims: number of dimensions (e.g., 1 for single numeric field, 2 for lat/lon)
// bytesPerDim: bytes used to encode each dimension (1-8 for long/double)
func NewBKDTree(numDims, bytesPerDim int) (*BKDTree, error) {
	if numDims < 1 {
		return nil, fmt.Errorf("numDims must be >= 1, got %d", numDims)
	}
	if bytesPerDim < 1 || bytesPerDim > 8 {
		return nil, fmt.Errorf("bytesPerDim must be between 1 and 8, got %d", bytesPerDim)
	}

	tree := &BKDTree{
		numDims:             numDims,
		bytesPerDim:         bytesPerDim,
		maxPointsInLeafNode: 512, // Default block size
		minPackedValue:      make([]byte, numDims*bytesPerDim),
		maxPackedValue:      make([]byte, numDims*bytesPerDim),
	}

	// Initialize max values to 0xFF
	for i := range tree.maxPackedValue {
		tree.maxPackedValue[i] = 0xFF
	}

	return tree, nil
}

// NumDims returns the number of dimensions in this tree.
func (t *BKDTree) NumDims() int {
	return t.numDims
}

// BytesPerDim returns the number of bytes per dimension.
func (t *BKDTree) BytesPerDim() int {
	return t.bytesPerDim
}

// Pack packs the given values into a byte slice.
// values must have length == numDims.
func (t *BKDTree) Pack(values []int64) ([]byte, error) {
	if len(values) != t.numDims {
		return nil, fmt.Errorf("expected %d values, got %d", t.numDims, len(values))
	}

	packed := make([]byte, t.numDims*t.bytesPerDim)
	for dim := 0; dim < t.numDims; dim++ {
		offset := dim * t.bytesPerDim
		encodeDimension(values[dim], packed[offset:offset+t.bytesPerDim], t.bytesPerDim)
	}

	return packed, nil
}

// Unpack unpacks the given byte slice into int64 values.
// Returns a slice of length numDims.
func (t *BKDTree) Unpack(packed []byte) ([]int64, error) {
	if len(packed) != t.numDims*t.bytesPerDim {
		return nil, fmt.Errorf("expected %d bytes, got %d", t.numDims*t.bytesPerDim, len(packed))
	}

	values := make([]int64, t.numDims)
	for dim := 0; dim < t.numDims; dim++ {
		offset := dim * t.bytesPerDim
		values[dim] = decodeDimension(packed[offset:offset+t.bytesPerDim], t.bytesPerDim)
	}

	return values, nil
}

// Compare compares two packed values at the given dimension.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (t *BKDTree) Compare(a, b []byte, dim int) int {
	offset := dim * t.bytesPerDim
	aVal := decodeDimension(a[offset:offset+t.bytesPerDim], t.bytesPerDim)
	bVal := decodeDimension(b[offset:offset+t.bytesPerDim], t.bytesPerDim)

	if aVal < bVal {
		return -1
	} else if aVal > bVal {
		return 1
	}
	return 0
}

// encodeDimension encodes an int64 value into bytes.
func encodeDimension(value int64, buf []byte, bytesPerDim int) {
	for i := bytesPerDim - 1; i >= 0; i-- {
		buf[i] = byte(value & 0xFF)
		value >>= 8
	}
}

// decodeDimension decodes bytes into an int64 value.
func decodeDimension(buf []byte, bytesPerDim int) int64 {
	var value int64
	for i := 0; i < bytesPerDim; i++ {
		value = (value << 8) | int64(buf[i])
	}

	// Sign extension for negative numbers (two's complement)
	if bytesPerDim < 8 {
		signBit := int64(1) << ((bytesPerDim * 8) - 1)
		if value&signBit != 0 {
			value |= ^((int64(1) << (bytesPerDim * 8)) - 1)
		}
	}

	return value
}

// PointValuesImpl represents a set of points in the BKD tree.
type PointValuesImpl struct {
	tree   *BKDTree
	points [][]byte
	docIDs []int
}

// NewPointValues creates a new PointValuesImpl for the given tree.
func NewPointValues(tree *BKDTree) *PointValuesImpl {
	return &PointValuesImpl{
		tree:   tree,
		points: make([][]byte, 0),
		docIDs: make([]int, 0),
	}
}

// Add adds a point with its associated document ID.
// values must have length == tree.NumDims().
func (pv *PointValuesImpl) Add(docID int, values []int64) error {
	packed, err := pv.tree.Pack(values)
	if err != nil {
		return err
	}

	pv.points = append(pv.points, packed)
	pv.docIDs = append(pv.docIDs, docID)
	return nil
}

// Sort sorts the points by the given dimension.
func (pv *PointValuesImpl) Sort(dim int) {
	if dim < 0 || dim >= pv.tree.numDims {
		return
	}

	sort.Slice(pv.points, func(i, j int) bool {
		return pv.tree.Compare(pv.points[i], pv.points[j], dim) < 0
	})
}

// Intersect finds all points intersecting the given range.
// minPacked and maxPacked define the query range.
func (pv *PointValuesImpl) Intersect(minPacked, maxPacked []byte) []int {
	var result []int
	for i, point := range pv.points {
		if pv.pointInRange(point, minPacked, maxPacked) {
			result = append(result, pv.docIDs[i])
		}
	}
	return result
}

// pointInRange checks if a point is within the given range.
func (pv *PointValuesImpl) pointInRange(point, minPacked, maxPacked []byte) bool {
	for dim := 0; dim < pv.tree.numDims; dim++ {
		offset := dim * pv.tree.bytesPerDim
		dimVal := decodeDimension(point[offset:offset+pv.tree.bytesPerDim], pv.tree.bytesPerDim)
		minVal := decodeDimension(minPacked[offset:offset+pv.tree.bytesPerDim], pv.tree.bytesPerDim)
		maxVal := decodeDimension(maxPacked[offset:offset+pv.tree.bytesPerDim], pv.tree.bytesPerDim)

		if dimVal < minVal || dimVal > maxVal {
			return false
		}
	}
	return true
}

// Size returns the number of points.
func (pv *PointValuesImpl) Size() int {
	return len(pv.points)
}

// Clear clears all points.
func (pv *PointValuesImpl) Clear() {
	pv.points = pv.points[:0]
	pv.docIDs = pv.docIDs[:0]
}

// PointRangeQuery represents a range query over point values.
type PointRangeQuery struct {
	field          string
	lowerValue     []int64
	upperValue     []int64
	lowerInclusive bool
	upperInclusive bool
}

// NewPointRangeQuery creates a new PointRangeQuery.
func NewPointRangeQuery(field string, lowerValue, upperValue []int64) *PointRangeQuery {
	return &PointRangeQuery{
		field:          field,
		lowerValue:     lowerValue,
		upperValue:     upperValue,
		lowerInclusive: true,
		upperInclusive: true,
	}
}

// SetInclusive sets whether the range bounds are inclusive.
func (q *PointRangeQuery) SetInclusive(lower, upper bool) {
	q.lowerInclusive = lower
	q.upperInclusive = upper
}

// Matches checks if the given values match this query.
func (q *PointRangeQuery) Matches(values []int64) bool {
	if len(values) != len(q.lowerValue) || len(values) != len(q.upperValue) {
		return false
	}

	for i := 0; i < len(values); i++ {
		// Check lower bound
		if q.lowerInclusive {
			if values[i] < q.lowerValue[i] {
				return false
			}
		} else {
			if values[i] <= q.lowerValue[i] {
				return false
			}
		}

		// Check upper bound
		if q.upperInclusive {
			if values[i] > q.upperValue[i] {
				return false
			}
		} else {
			if values[i] >= q.upperValue[i] {
				return false
			}
		}
	}

	return true
}

// String returns a string representation of the query.
func (q *PointRangeQuery) String() string {
	return fmt.Sprintf("PointRangeQuery(field=%s)", q.field)
}

// PointValuesIterator iterates over point values.
// This is the Go port of Lucene's org.apache.lucene.index.PointValuesIterator.
type PointValuesIterator struct {
	points [][]byte
	docIDs []int
	index  int
}

// NewPointValuesIterator creates a new PointValuesIterator.
func NewPointValuesIterator(points [][]byte, docIDs []int) *PointValuesIterator {
	return &PointValuesIterator{
		points: points,
		docIDs: docIDs,
		index:  -1,
	}
}

// Next advances to the next value.
func (it *PointValuesIterator) Next() bool {
	it.index++
	return it.index < len(it.points)
}

// DocID returns the current document ID.
func (it *PointValuesIterator) DocID() int {
	if it.index < 0 || it.index >= len(it.docIDs) {
		return -1
	}
	return it.docIDs[it.index]
}

// Value returns the current packed value.
func (it *PointValuesIterator) Value() []byte {
	if it.index < 0 || it.index >= len(it.points) {
		return nil
	}
	return it.points[it.index]
}

// Reset resets the iterator to the beginning.
func (it *PointValuesIterator) Reset() {
	it.index = -1
}

// Size returns the number of values.
func (it *PointValuesIterator) Size() int {
	return len(it.points)
}
