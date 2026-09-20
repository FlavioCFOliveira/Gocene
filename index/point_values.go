package index

import "github.com/FlavioCFOliveira/Gocene/util/bkd"

// PointValues static members mirror the static members of
// org.apache.lucene.index.PointValues (Apache Lucene 10.5.0): the three
// constants and the four helpers that aggregate a field's point statistics
// across every leaf of an IndexReader.
//
// They live here rather than beside the [PointValues] interface, which is
// declared in spi so that org.apache.lucene.util.bkd and the codec packages
// can name it without an import cycle: MAX_DIMENSIONS and MAX_INDEX_DIMENSIONS
// are defined in terms of org.apache.lucene.util.bkd.BKDConfig, and the four
// helpers take an IndexReader, neither of which spi can reference.

const (
	// PointValuesMaxNumBytes is the maximum number of bytes for each
	// dimension. Renders `public static final int MAX_NUM_BYTES = 16`.
	PointValuesMaxNumBytes = 16

	// PointValuesMaxDimensions is the maximum number of dimensions. Renders
	// `public static final int MAX_DIMENSIONS = BKDConfig.MAX_DIMS`.
	PointValuesMaxDimensions = bkd.MaxDims

	// PointValuesMaxIndexDimensions is the maximum number of index
	// dimensions. Renders `public static final int MAX_INDEX_DIMENSIONS =
	// BKDConfig.MAX_INDEX_DIMS`.
	PointValuesMaxIndexDimensions = bkd.MaxIndexDims
)

// PointValuesSize returns the cumulated number of points across all leaves of
// the given IndexReader. Leaves that do not have points for the given field
// are ignored. Renders `public static long size(IndexReader, String)`.
func PointValuesSize(reader IndexReader, field string) (int64, error) {
	var size int64
	leaves, err := reader.Leaves()
	if err != nil {
		return 0, err
	}
	for _, ctx := range leaves {
		values, err := ctx.LeafReader().GetPointValues(field)
		if err != nil {
			return 0, err
		}
		if values != nil {
			size += values.Size()
		}
	}
	return size, nil
}

// PointValuesGetDocCount returns the cumulated number of docs that have points
// across all leaves of the given IndexReader. Leaves that do not have points
// for the given field are ignored. Renders `public static int
// getDocCount(IndexReader, String)`.
func PointValuesGetDocCount(reader IndexReader, field string) (int, error) {
	count := 0
	leaves, err := reader.Leaves()
	if err != nil {
		return 0, err
	}
	for _, ctx := range leaves {
		values, err := ctx.LeafReader().GetPointValues(field)
		if err != nil {
			return 0, err
		}
		if values != nil {
			count += values.GetDocCount()
		}
	}
	return count, nil
}

// PointValuesGetMinPackedValue returns the minimum packed values across all
// leaves of the given IndexReader. Leaves that do not have points for the
// given field are ignored. Renders `public static byte[]
// getMinPackedValue(IndexReader, String)`.
func PointValuesGetMinPackedValue(reader IndexReader, field string) ([]byte, error) {
	var minValue []byte
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}
	for _, ctx := range leaves {
		values, err := ctx.LeafReader().GetPointValues(field)
		if err != nil {
			return nil, err
		}
		if values == nil {
			continue
		}
		leafMinValue, err := values.GetMinPackedValue()
		if err != nil {
			return nil, err
		}
		if leafMinValue == nil {
			continue
		}
		if minValue == nil {
			minValue = append([]byte(nil), leafMinValue...)
		} else {
			numDimensions, err := values.GetNumIndexDimensions()
			if err != nil {
				return nil, err
			}
			numBytesPerDimension, err := values.GetBytesPerDimension()
			if err != nil {
				return nil, err
			}
			for i := 0; i < numDimensions; i++ {
				offset := i * numBytesPerDimension
				if compareUnsigned(leafMinValue[offset:], minValue[offset:], numBytesPerDimension) < 0 {
					copy(minValue[offset:], leafMinValue[offset:offset+numBytesPerDimension])
				}
			}
		}
	}
	return minValue, nil
}

// PointValuesGetMaxPackedValue returns the maximum packed values across all
// leaves of the given IndexReader. Leaves that do not have points for the
// given field are ignored. Renders `public static byte[]
// getMaxPackedValue(IndexReader, String)`.
func PointValuesGetMaxPackedValue(reader IndexReader, field string) ([]byte, error) {
	var maxValue []byte
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}
	for _, ctx := range leaves {
		values, err := ctx.LeafReader().GetPointValues(field)
		if err != nil {
			return nil, err
		}
		if values == nil {
			continue
		}
		leafMaxValue, err := values.GetMaxPackedValue()
		if err != nil {
			return nil, err
		}
		if leafMaxValue == nil {
			continue
		}
		if maxValue == nil {
			maxValue = append([]byte(nil), leafMaxValue...)
		} else {
			numDimensions, err := values.GetNumIndexDimensions()
			if err != nil {
				return nil, err
			}
			numBytesPerDimension, err := values.GetBytesPerDimension()
			if err != nil {
				return nil, err
			}
			for i := 0; i < numDimensions; i++ {
				offset := i * numBytesPerDimension
				if compareUnsigned(leafMaxValue[offset:], maxValue[offset:], numBytesPerDimension) > 0 {
					copy(maxValue[offset:], leafMaxValue[offset:offset+numBytesPerDimension])
				}
			}
		}
	}
	return maxValue, nil
}

// compareUnsigned renders ArrayUtil.getUnsignedComparator(int)'s comparison of
// one dimension: an unsigned, length-bounded lexicographic comparison of two
// byte slices.
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
