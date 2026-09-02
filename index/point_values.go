//go:build ignore

package index

import (
	"fmt"
)

// PointValues provides access to indexed numeric values (KD-tree).
type PointValues interface {
	GetMinPackedValue() ([]byte, error)
	GetMaxPackedValue() ([]byte, error)
	GetNumDimensions() (int, error)
	GetNumIndexDimensions() (int, error)
	GetBytesPerDimension() (int, error)
	Size() (int64, error)
	GetDocCount() (int, error)
}

// PointValuesGetMinPackedValue returns the minimum packed values across all leaves of the given IndexReader.
func PointValuesGetMinPackedValue(reader IndexReader, field string) []byte {
	var minValue []byte
	for _, ctx := range reader.Leaves() {
		values := ctx.Reader().GetPointValues(field)
		if values == nil {
			continue
		}
		leafMinValue, err := values.GetMinPackedValue()
		if err != nil || leafMinValue == nil {
			continue
		}
		if minValue == nil {
			minValue = append([]byte(nil), leafMinValue...)
		} else {
			numDims, _ := values.GetNumIndexDimensions()
			bytesPerDim, _ := values.GetBytesPerDimension()
			for i := 0; i < numDims; i++ {
				offset := i * bytesPerDim
				if compareUnsigned(leafMinValue[offset:], minValue[offset:], bytesPerDim) < 0 {
					copy(minValue[offset:], leafMinValue[offset:offset+bytesPerDim])
				}
			}
		}
	}
	return minValue
}

// PointValuesGetMaxPackedValue returns the maximum packed values across all leaves of the given IndexReader.
func PointValuesGetMaxPackedValue(reader IndexReader, field string) []byte {
	var maxValue []byte
	for _, ctx := range reader.Leaves() {
		values := ctx.Reader().GetPointValues(field)
		if values == nil {
			continue
		}
		leafMaxValue, err := values.GetMaxPackedValue()
		if err != nil || leafMaxValue == nil {
			continue
		}
		if maxValue == nil {
			maxValue = append([]byte(nil), leafMaxValue...)
		} else {
			numDims, _ := values.GetNumIndexDimensions()
			bytesPerDim, _ := values.GetBytesPerDimension()
			for i := 0; i < numDims; i++ {
				offset := i * bytesPerDim
				if compareUnsigned(leafMaxValue[offset:], maxValue[offset:], bytesPerDim) > 0 {
					copy(maxValue[offset:], leafMaxValue[offset:offset+bytesPerDim])
				}
			}
		}
	}
	return maxValue
}

// PointValuesGetDocCount returns the cumulated number of docs that have points across all leaves.
func PointValuesGetDocCount(reader IndexReader, field string) int {
	count := 0
	for _, ctx := range reader.Leaves() {
		values := ctx.Reader().GetPointValues(field)
		if values != nil {
			c, _ := values.GetDocCount()
			count += c
		}
	}
	return count
}

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
