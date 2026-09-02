// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file holds the static helpers Apache Lucene 10.5.0 declares on
// org.apache.lucene.index.PointValues: the cross-leaf aggregations
// getMinPackedValue, getMaxPackedValue and getDocCount.
//
// The PointValues interface itself is declared in doc_values_interfaces.go,
// which is the declaration the rest of the module (join, sandbox, search) is
// wired to.

// indexDimensionedPointValues is the optional view of a PointValues that knows
// how many of its dimensions are indexed.
//
// PORT NOTE: Java's PointValues declares getNumIndexDimensions() on the class
// itself, so the min/max aggregations below can always ask for it. Gocene's
// PointValues interface is deliberately narrower — it is implemented by
// consumers in several packages — so the concrete BKD-backed readers expose
// the accessor and the helpers probe for it, falling back to the data
// dimension count. For a single-dimension field, and for every point field
// whose data and index dimension counts agree, the two are identical.
type indexDimensionedPointValues interface {
	// GetNumIndexDimensions returns the number of dimensions used for indexing.
	GetNumIndexDimensions() int
}

// numIndexDimensions returns the number of indexed dimensions of values,
// preferring the exact count when the implementation exposes it.
func numIndexDimensions(values PointValues) int {
	if v, ok := values.(indexDimensionedPointValues); ok {
		return v.GetNumIndexDimensions()
	}
	return values.GetNumDimensions()
}

// PointValuesGetMinPackedValue returns the minimum packed value across every
// leaf of reader for the given field, or nil when no leaf indexes it. Mirrors
// the static PointValues.getMinPackedValue(IndexReader, String).
func PointValuesGetMinPackedValue(reader IndexReader, field string) []byte {
	var minValue []byte
	for _, ctx := range reader.Leaves() {
		values, err := ctx.Reader().GetPointValues(field)
		if err != nil || values == nil {
			continue
		}
		leafMinValue, err := values.GetMinPackedValue()
		if err != nil || leafMinValue == nil {
			continue
		}
		if minValue == nil {
			minValue = append([]byte(nil), leafMinValue...)
			continue
		}
		numDims := numIndexDimensions(values)
		bytesPerDim := values.GetBytesPerDimension()
		for i := 0; i < numDims; i++ {
			offset := i * bytesPerDim
			if compareUnsigned(leafMinValue[offset:], minValue[offset:], bytesPerDim) < 0 {
				copy(minValue[offset:], leafMinValue[offset:offset+bytesPerDim])
			}
		}
	}
	return minValue
}

// PointValuesGetMaxPackedValue returns the maximum packed value across every
// leaf of reader for the given field, or nil when no leaf indexes it. Mirrors
// the static PointValues.getMaxPackedValue(IndexReader, String).
func PointValuesGetMaxPackedValue(reader IndexReader, field string) []byte {
	var maxValue []byte
	for _, ctx := range reader.Leaves() {
		values, err := ctx.Reader().GetPointValues(field)
		if err != nil || values == nil {
			continue
		}
		leafMaxValue, err := values.GetMaxPackedValue()
		if err != nil || leafMaxValue == nil {
			continue
		}
		if maxValue == nil {
			maxValue = append([]byte(nil), leafMaxValue...)
			continue
		}
		numDims := numIndexDimensions(values)
		bytesPerDim := values.GetBytesPerDimension()
		for i := 0; i < numDims; i++ {
			offset := i * bytesPerDim
			if compareUnsigned(leafMaxValue[offset:], maxValue[offset:], bytesPerDim) > 0 {
				copy(maxValue[offset:], leafMaxValue[offset:offset+bytesPerDim])
			}
		}
	}
	return maxValue
}

// PointValuesGetDocCount returns the cumulative number of documents that have
// points for the given field across every leaf of reader. Mirrors the static
// PointValues.getDocCount(IndexReader, String).
func PointValuesGetDocCount(reader IndexReader, field string) int {
	count := 0
	for _, ctx := range reader.Leaves() {
		values, err := ctx.Reader().GetPointValues(field)
		if err != nil || values == nil {
			continue
		}
		count += values.GetDocCount()
	}
	return count
}

// compareUnsigned compares the first length bytes of a and b as unsigned
// values, mirroring java.util.Arrays.compareUnsigned over a packed dimension.
func compareUnsigned(a, b []byte, length int) int {
	for i := 0; i < length; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
