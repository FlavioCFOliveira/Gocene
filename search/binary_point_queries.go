// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"fmt"
	"sort"
)

// NewBinaryPointExactQuery returns a query matching documents whose
// binary-point field carries exactly the given 1-dimensional value. It
// mirrors Lucene 10.4.0's BinaryPoint.newExactQuery(String, byte[]).
//
// Lucene defines this as the package-static helper on the document-side
// BinaryPoint class. Gocene cannot mirror that placement because
// document/ would need to import search/ — which would close a cycle
// since search/ already imports document/ for FieldType and friends.
// Hosting the factory in search/ keeps the public-facing names identical
// (callers wrote BinaryPoint.newExactQuery in Java; they now write
// search.NewBinaryPointExactQuery in Go) without forcing the cycle.
func NewBinaryPointExactQuery(field string, value []byte) (Query, error) {
	return NewBinaryPointRangeQuery(field, value, value)
}

// NewBinaryPointRangeQuery returns a query matching documents whose
// binary-point field falls within the inclusive 1-dimensional range
// [lowerValue, upperValue]. Mirrors Lucene 10.4.0's
// BinaryPoint.newRangeQuery(String, byte[], byte[]).
//
// Both bounds must be non-nil and of equal length. The query is a thin
// wrapper around NewPointRangeQueryMultiDim with numDims=1.
func NewBinaryPointRangeQuery(field string, lowerValue, upperValue []byte) (Query, error) {
	if field == "" {
		return nil, fmt.Errorf("BinaryPoint range query: field must not be empty")
	}
	if lowerValue == nil {
		return nil, fmt.Errorf("BinaryPoint range query: lowerValue must not be nil")
	}
	if upperValue == nil {
		return nil, fmt.Errorf("BinaryPoint range query: upperValue must not be nil")
	}
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf(
			"BinaryPoint range query: lowerValue and upperValue must have the same length (%d != %d)",
			len(lowerValue), len(upperValue),
		)
	}
	return NewPointRangeQueryMultiDim(field, lowerValue, upperValue, 1)
}

// NewBinaryPointMultiDimRangeQuery returns a multi-dimensional range
// query. Each entry in lowerValue and upperValue corresponds to one
// dimension; all dimensions must share the same byte width. Mirrors
// Lucene 10.4.0's BinaryPoint.newRangeQuery(String, byte[][], byte[][]).
//
// The 2-D byte arrays are packed into the same contiguous layout
// PointRangeQuery expects: dim0 bytes followed by dim1 bytes and so on.
func NewBinaryPointMultiDimRangeQuery(field string, lowerValue, upperValue [][]byte) (Query, error) {
	if field == "" {
		return nil, fmt.Errorf("BinaryPoint multi-dim range query: field must not be empty")
	}
	if lowerValue == nil || upperValue == nil {
		return nil, fmt.Errorf("BinaryPoint multi-dim range query: bounds must not be nil")
	}
	if len(lowerValue) != len(upperValue) {
		return nil, fmt.Errorf(
			"BinaryPoint multi-dim range query: dimension counts differ (%d vs %d)",
			len(lowerValue), len(upperValue),
		)
	}
	if len(lowerValue) == 0 {
		return nil, fmt.Errorf("BinaryPoint multi-dim range query: at least one dimension required")
	}

	packedLower, packedUpper, err := packBinaryPoint(lowerValue, upperValue)
	if err != nil {
		return nil, fmt.Errorf("BinaryPoint multi-dim range query: %w", err)
	}
	return NewPointRangeQueryMultiDim(field, packedLower, packedUpper, len(lowerValue))
}

// NewBinaryPointSetQuery returns a query matching any of the given 1-D
// binary values. Mirrors Lucene 10.4.0's
// BinaryPoint.newSetQuery(String, byte[]...).
//
// All values must share the same byte width. When values is empty the
// query returns no hits (MatchNoDocsQuery), matching Lucene's "empty
// BinaryPoint.newSetQuery" sentinel.
func NewBinaryPointSetQuery(field string, values ...[]byte) (Query, error) {
	if field == "" {
		return nil, fmt.Errorf("BinaryPoint set query: field must not be empty")
	}
	if len(values) == 0 {
		return NewMatchNoDocsQueryWithReason("empty BinaryPoint.newSetQuery"), nil
	}

	bytesPerDim := -1
	for i, v := range values {
		if v == nil {
			return nil, fmt.Errorf("BinaryPoint set query: value %d is nil", i)
		}
		if bytesPerDim == -1 {
			if len(v) == 0 {
				return nil, fmt.Errorf("BinaryPoint set query: value %d has zero length", i)
			}
			bytesPerDim = len(v)
		} else if len(v) != bytesPerDim {
			return nil, fmt.Errorf(
				"BinaryPoint set query: all values must share the same length, but value %d has %d bytes (want %d)",
				i, len(v), bytesPerDim,
			)
		}
	}

	// Don't mutate the caller's slice. Sort lexicographically unsigned so
	// PointInSetQuery's BKD walk encounters the values in order.
	sorted := make([][]byte, len(values))
	for i, v := range values {
		buf := make([]byte, len(v))
		copy(buf, v)
		sorted[i] = buf
	}
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i], sorted[j]) < 0
	})

	return NewPointInSetQuery(field, 1, bytesPerDim, sorted), nil
}

// packBinaryPoint concatenates equal-length dimension byte slices into a
// single contiguous packed slice. It returns the packed lower and upper
// bounds, or an error when the dimensions are not uniform.
func packBinaryPoint(lower, upper [][]byte) (packedLower, packedUpper []byte, err error) {
	bytesPerDim := -1
	for i, l := range lower {
		if l == nil || upper[i] == nil {
			return nil, nil, fmt.Errorf("dimension %d has a nil bound", i)
		}
		if len(l) != len(upper[i]) {
			return nil, nil, fmt.Errorf("dimension %d: lower length %d != upper length %d", i, len(l), len(upper[i]))
		}
		if bytesPerDim == -1 {
			if len(l) == 0 {
				return nil, nil, fmt.Errorf("dimension %d has zero length", i)
			}
			bytesPerDim = len(l)
		} else if len(l) != bytesPerDim {
			return nil, nil, fmt.Errorf("all dimensions must have the same byte length; got %d and %d", bytesPerDim, len(l))
		}
	}

	packedLower = make([]byte, 0, bytesPerDim*len(lower))
	packedUpper = make([]byte, 0, bytesPerDim*len(upper))
	for i := range lower {
		packedLower = append(packedLower, lower[i]...)
		packedUpper = append(packedUpper, upper[i]...)
	}
	return packedLower, packedUpper, nil
}
