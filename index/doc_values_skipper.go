// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DocValuesSkipper is an interface for skipping through DocValues.
// This is the Go port of Lucene's org.apache.lucene.index.DocValuesSkipper.
type DocValuesSkipper interface {
	// Advance this skipper so that all levels contain the next document on or after target.
	// The behavior is undefined if target is less than or equal to MaxDocID(0).
	Advance(target int) error

	// NumLevels returns the number of levels. This number may change when moving to a different interval.
	NumLevels() int

	// MinDocID returns the minimum doc ID of the interval on the given level, inclusive.
	// This returns -1 if Advance has not been called yet and search.NO_MORE_DOCS if the iterator is exhausted.
	// This method is non-increasing when level increases: MinDocID(level+1) <= MinDocID(level).
	MinDocID(level int) int

	// MaxDocID returns the maximum doc ID of the interval on the given level, inclusive.
	// This returns -1 if Advance has not been called yet and search.NO_MORE_DOCS if the iterator is exhausted.
	// This method is non-decreasing when level decreases: MaxDocID(level+1) >= MaxDocID(level).
	MaxDocID(level int) int

	// MinValue returns the minimum value of the interval at the given level, inclusive.
	// It is only guaranteed that values in this interval are greater than or equal to the returned value.
	MinValue(level int) int64

	// MaxValue returns the maximum value of the interval at the given level, inclusive.
	// It is only guaranteed that values in this interval are less than or equal to the returned value.
	MaxValue(level int) int64

	// DocCount returns the number of documents that have a value in the interval associated with the given level.
	DocCount(level int) int

	// MinValueGlobal returns the global minimum value.
	MinValueGlobal() int64

	// MaxValueGlobal returns the global maximum value.
	MaxValueGlobal() int64

	// DocCountGlobal returns the global number of documents with a value for the field.
	DocCountGlobal() int
}

// MaxValueCount returns the global maximum number of values that any single document has for the field.
// Returns -1 if the exact value is unavailable.
func MaxValueCount(s DocValuesSkipper) int {
	if s.DocCountGlobal() == 0 {
		return 0
	}
	return -1
}

// AdvanceRange advances this skipper so that all levels intersect the range given by minValue and maxValue.
// If there are no intersecting levels, the skipper is exhausted.
func AdvanceRange(s DocValuesSkipper, minValue, maxValue int64) error {
	if s.MinDocID(0) == -1 {
		// Advance has not been called yet
		if err := s.Advance(0); err != nil {
			return err
		}
	}

	// check if the current interval intersects the provided range
	for s.MinDocID(0) != search.NO_MORE_DOCS && (s.MinValue(0) > maxValue || s.MaxValue(0) < minValue) {
		maxDocID := s.MaxDocID(0)
		nextLevel := 1
		// check if the next levels intersect to skip as many docs as possible
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

// GlobalMinValue returns the minimum value for a field across all segments.
func GlobalMinValue(reader IndexReader, field string) (int64, error) {
	var minValue int64 = 9223372036854775807 // math.MaxInt64

	leaves := reader.Leaves()
	for _, ctx := range leaves {
		fi := ctx.Reader().GetFieldInfos().FieldInfo(field)
		if fi == nil {
			continue // no field values in this segment, so we can ignore it
		}
		skipper, err := ctx.Reader().GetDocValuesSkipper(field)
		if err != nil {
			return 0, err
		}
		if skipper == nil {
			// minimum cannot be computed correctly since skipper is not enabled for some leaf
			return -9223372036854775808, nil // math.MinInt64
		}
		if val := skipper.MinValueGlobal(); val < minValue {
			minValue = val
		}
	}
	return minValue, nil
}

// GlobalMaxValue returns the maximum value for a field across all segments.
func GlobalMaxValue(reader IndexReader, field string) (int64, error) {
	var maxValue int64 = -9223372036854775808 // math.MinInt64

	leaves := reader.Leaves()
	for _, ctx := range leaves {
		fi := ctx.Reader().GetFieldInfos().FieldInfo(field)
		if fi == nil {
			continue // no field values in this segment, so we can ignore it
		}
		skipper, err := ctx.Reader().GetDocValuesSkipper(field)
		if err != nil {
			return 0, err
		}
		if skipper == nil {
			// maximum cannot be computed correctly since skipper is not enabled for some leaf
			return 9223372036854775807, nil // math.MaxInt64
		}
		if val := skipper.MaxValueGlobal(); val > maxValue {
			maxValue = val
		}
	}
	return maxValue, nil
}

// GlobalDocCount returns the total skipper document count for a field across all segments.
func GlobalDocCount(reader IndexReader, field string) (int, error) {
	docCount := 0
	leaves := reader.Leaves()
	for _, ctx := range leaves {
		skipper, err := ctx.Reader().GetDocValuesSkipper(field)
		if err != nil {
			return 0, err
		}
		if skipper != nil {
			docCount += skipper.DocCountGlobal()
		}
	}
	return docCount, nil
}
