package rangefacets

import "sort"

// DynamicRangeUtil computes equal-frequency bucket boundaries over a slice
// of observed numeric values. Mirrors
// org.apache.lucene.facet.range.DynamicRangeUtil.

// ComputeDynamicLongRanges returns numBuckets LongRanges produced by
// partitioning the supplied values into equal-frequency buckets (the
// boundaries fall on the sorted-values quantiles). When numBuckets >=
// len(values) every value gets its own range.
func ComputeDynamicLongRanges(values []int64, numBuckets int) []*LongRange {
	if numBuckets < 1 {
		return nil
	}
	if len(values) == 0 {
		return nil
	}
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if numBuckets >= len(sorted) {
		numBuckets = len(sorted)
	}
	bucketSize := len(sorted) / numBuckets
	ranges := make([]*LongRange, 0, numBuckets)
	for i := 0; i < numBuckets; i++ {
		from := sorted[i*bucketSize]
		var to int64
		if i == numBuckets-1 {
			to = sorted[len(sorted)-1]
		} else {
			to = sorted[(i+1)*bucketSize-1]
		}
		ranges = append(ranges, NewLongRange("", from, true, to, true))
	}
	return ranges
}
