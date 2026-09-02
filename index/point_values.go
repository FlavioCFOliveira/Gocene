package index

// PointValues static helpers mirror the org.apache.lucene.index.PointValues
// static utility methods (getMinPackedValue/getMaxPackedValue/getDocCount over
// an IndexReader). The PointValues interface itself is declared once, in
// doc_values_interfaces.go, matching the shape actually implemented by the
// BKD-backed codec readers (see codecs/lucene90's `pointValues` and its
// `var _ index.PointValues = (*pointValues)(nil)` assertion).

// PointValuesGetMinPackedValue returns the minimum packed values across all leaves of the given IndexReader.
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
		} else {
			numDims := values.GetNumDimensions()
			bytesPerDim := values.GetBytesPerDimension()
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
		} else {
			numDims := values.GetNumDimensions()
			bytesPerDim := values.GetBytesPerDimension()
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
		values, err := ctx.Reader().GetPointValues(field)
		if err == nil && values != nil {
			count += values.GetDocCount()
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
