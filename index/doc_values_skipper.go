package index

import (
	"math"
)

// DocValuesSkipper is a skipper for DocValues.
//
// A skipper has a position that can only be advanced via Advance(int). The next advance
// position must be greater than MaxDocID(0) at level 0. A skipper's position, along with
// a level, determines the interval at which the skipper is currently situated.
//
// This is the Go port of Lucene's org.apache.lucene.index.DocValuesSkipper.
type DocValuesSkipper interface {
	// Advance advances this skipper so that all levels contain the next document on or after target.
	//
	// NOTE: The behavior is undefined if target is less than or equal to MaxDocID(0).
	//
	// NOTE: MinDocID(0) may return a doc ID that is greater than target if
	// the target document doesn't have a value.
	Advance(target int) error

	// NumLevels returns the number of levels. This number may change when moving to a different interval.
	NumLevels() int

	// MinDocID returns the minimum doc ID of the interval on the given level, inclusive.
	// This returns -1 if Advance(int) has not been called yet and NO_MORE_DOCS
	// if the iterator is exhausted. This method is non-increasing when level increases.
	// Said otherwise MinDocID(level+1) <= MinDocID(level).
	MinDocID(level int) int

	// MaxDocID returns the maximum doc ID of the interval on the given level, inclusive.
	// This returns -1 if Advance(int) has not been called yet and NO_MORE_DOCS
	// if the iterator is exhausted. This method is non-decreasing when level decreases.
	// Said otherwise MaxDocID(level+1) >= MaxDocID(level).
	MaxDocID(level int) int

	// MinValue returns the minimum value of the interval at the given level, inclusive.
	//
	// NOTE: It is only guaranteed that values in this interval are greater than or equal
	// the returned value. There is no guarantee that one document actually has this value.
	MinValue(level int) int64

	// MaxValue returns the maximum value of the interval at the given level, inclusive.
	//
	// NOTE: It is only guaranteed that values in this interval are less than or equal the
	// returned value. There is no guarantee that one document actually has this value.
	MaxValue(level int) int64

	// DocCount returns the number of documents that have a value in the interval associated with the given
	// level.
	DocCount(level int) int

	// MinValue returns the global minimum value.
	//
	// NOTE: It is only guaranteed that values are greater than or equal the returned value.
	// There is no guarantee that one document actually has this value.
	GlobalMinValue() int64

	// MaxValue returns the global maximum value.
	//
	// NOTE: It is only guaranteed that values are less than or equal the returned value.
	// There is no guarantee that one document actually has this value.
	GlobalMaxValue() int64

	// DocCount returns the global number of documents with a value for the field.
	GlobalDocCount() int
}

// MaxValueCount returns the global maximum number of values that any single document has for the field.
// Returns -1 if the exact value is unavailable.
// This is the Go port of DocValuesSkipper.maxValueCount().
func MaxValueCount(s DocValuesSkipper) int {
	if s.GlobalDocCount() == 0 {
		return 0
	}
	return -1
}

// AdvanceRange advances this skipper so that all levels intersect the range given by minValue and
// maxValue. If there are no intersecting levels, the skipper is exhausted.
// This is the Go port of DocValuesSkipper.advance(long, long).
func AdvanceRange(s DocValuesSkipper, minValue, maxValue int64) error {
	if s.MinDocID(0) == -1 {
		// #Advance has not been called yet
		if err := s.Advance(0); err != nil {
			return err
		}
	}
	// check if the current interval intersects the provided range
	for s.MinDocID(0) != NO_MORE_DOCS && (s.MinValue(0) > maxValue || s.MaxValue(0) < minValue) {
		maxDocID := s.MaxDocID(0)
		nextLevel := 1
		// check if the next levels intersects to skip as many docs as possible
		for nextLevel < s.NumLevels() && (s.MinValue(nextLevel) > maxValue || s.MaxValue(nextLevel) < minValue) {
			maxDocID = s.MaxDocID(nextLevel)
			nextLevel++
		}
		if err := s.Advance(maxDocID + 1); err != nil {
			return err
		}
	}
	return nil
}

// GlobalMinValue returns the minimum value for a field across all segments, or math.MinInt64 if not
// available.
// This is the Go port of DocValuesSkipper.globalMinValue.
func GlobalMinValue(reader IndexReader, field string) (int64, error) {
	minValue := int64(math.MaxInt64)
	for _, ctx := range reader.Leaves() {
		if ctx.Reader().GetFieldInfos().FieldInfo(field) == nil {
			continue // no field values in this segment, so we can ignore it
		}
		skipper := ctx.Reader().GetDocValuesSkipper(field)
		if skipper == nil {
			// minimum cannot be computed correctly since skipper is not enabled for some leaf
			return math.MinInt64, nil
		}
		val := skipper.GlobalMinValue()
		if val < minValue {
			minValue = val
		}
	}
	return minValue, nil
}

// GlobalMaxValue returns the maximum value for a field across all segments, or math.MinInt64 if not
// available.
// This is the Go port of DocValuesSkipper.globalMaxValue.
func GlobalMaxValue(reader IndexReader, field string) (int64, error) {
	maxValue := int64(math.MinInt64)
	for _, ctx := range reader.Leaves() {
		if ctx.Reader().GetFieldInfos().FieldInfo(field) == nil {
			continue // no field values in this segment, so we can ignore it
		}
		skipper := ctx.Reader().GetDocValuesSkipper(field)
		if skipper == nil {
			// maximum cannot be computed correctly since skipper is not enabled for some leaf
			return math.MaxInt64, nil
		}
		val := skipper.GlobalMaxValue()
		if val > maxValue {
			maxValue = val
		}
	}
	return maxValue, nil
}

// GlobalDocCount returns the total skipper document count for a field across all segments.
// This is the Go port of DocValuesSkipper.globalDocCount.
func GlobalDocCount(reader IndexReader, field string) (int, error) {
	docCount := 0
	for _, ctx := range reader.Leaves() {
		skipper := ctx.Reader().GetDocValuesSkipper(field)
		if skipper != nil {
			docCount += skipper.GlobalDocCount()
		}
	}
	return docCount, nil
}
