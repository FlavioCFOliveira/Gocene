// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

import (
	"fmt"
	"sort"
)

// RangeFacetCounts computes facet counts for numeric ranges over a
// schema-defined field backed by DocValues.
//
// This is the Go port of Lucene's sandbox RangeFacetCounts, which
// counts documents that fall into pre-configured numeric ranges.
type RangeFacetCounts struct {
	field  string
	ranges []RangeFacetRequest
	counts []int
	total  int
}

// RangeFacetRequest describes a single range bucket for faceting.
type RangeFacetRequest struct {
	// Label is a human-readable name for this range (e.g. "0-100").
	Label string

	// Min is the inclusive lower bound. nil means unbounded.
	Min *float64

	// Max is the exclusive upper bound. nil means unbounded.
	Max *float64

	// MinInclusive controls whether the lower bound is inclusive.
	MinInclusive bool

	// MaxInclusive controls whether the upper bound is inclusive.
	MaxInclusive bool
}

// RangeFacetResult holds the count for a single range bucket.
type RangeFacetResult struct {
	Label string
	Count int
	Min   *float64
	Max   *float64
}

// NewRangeFacetCounts creates a facet counter for the given field and ranges.
func NewRangeFacetCounts(field string, ranges []RangeFacetRequest) *RangeFacetCounts {
	return &RangeFacetCounts{
		field:  field,
		ranges: ranges,
		counts: make([]int, len(ranges)),
	}
}

// Field returns the field being faceted.
func (r *RangeFacetCounts) Field() string { return r.field }

// NumRanges returns the number of configured ranges.
func (r *RangeFacetCounts) NumRanges() int { return len(r.ranges) }

// Accumulate processes a numeric value, incrementing the matching range count.
// Returns the index of the matched range, or -1 if the value falls outside
// all configured ranges.
func (r *RangeFacetCounts) Accumulate(value float64) int {
	for i, rng := range r.ranges {
		if r.matches(value, rng) {
			r.counts[i]++
			r.total++
			return i
		}
	}
	return -1
}

// NumericDocValuesReader is the minimal DocValues interface consumed by
// RangeFacetCounts. It avoids an import cycle by living in the schema
// package; callers in index/ can pass their NumericDocValues directly
// because spi.NumericDocValues satisfies this interface.
type NumericDocValuesReader interface {
	NextDoc() (int, error)
	LongValue() (int64, error)
}

// AccumulateDocValues reads all values from the doc-values iterator and
// accumulates them into the appropriate ranges. The iterator is exhausted
// when NextDoc returns NO_MORE_DOCS (-1).
func (r *RangeFacetCounts) AccumulateDocValues(docValues NumericDocValuesReader) error {
	if docValues == nil {
		return fmt.Errorf("range facet: no DocValues for field %q", r.field)
	}
	for {
		docID, err := docValues.NextDoc()
		if err != nil {
			return fmt.Errorf("range facet: advance: %w", err)
		}
		if docID == NO_MORE_DOCS {
			break
		}
		v, err := docValues.LongValue()
		if err != nil {
			return fmt.Errorf("range facet: long value: %w", err)
		}
		r.Accumulate(float64(v))
	}
	return nil
}

// GetResult returns the accumulated counts for the i-th range.
func (r *RangeFacetCounts) GetResult(i int) RangeFacetResult {
	rng := r.ranges[i]
	return RangeFacetResult{
		Label: rng.Label,
		Count: r.counts[i],
		Min:   rng.Min,
		Max:   rng.Max,
	}
}

// GetResults returns all range results sorted by their original order.
func (r *RangeFacetCounts) GetResults() []RangeFacetResult {
	results := make([]RangeFacetResult, len(r.ranges))
	for i := range r.ranges {
		results[i] = r.GetResult(i)
	}
	return results
}

// GetTopResults returns range results sorted by count descending.
func (r *RangeFacetCounts) GetTopResults(n int) []RangeFacetResult {
	results := r.GetResults()
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})
	if n > 0 && n < len(results) {
		results = results[:n]
	}
	return results
}

// Total returns the total number of values accumulated (across all ranges).
func (r *RangeFacetCounts) Total() int { return r.total }

// matches reports whether value falls inside the given range.
func (r *RangeFacetCounts) matches(value float64, rng RangeFacetRequest) bool {
	if rng.Min != nil {
		if rng.MinInclusive {
			if value < *rng.Min {
				return false
			}
		} else {
			if value <= *rng.Min {
				return false
			}
		}
	}
	if rng.Max != nil {
		if rng.MaxInclusive {
			if value > *rng.Max {
				return false
			}
		} else {
			if value >= *rng.Max {
				return false
			}
		}
	}
	return true
}

// String returns a human-readable summary.
func (r *RangeFacetCounts) String() string {
	return fmt.Sprintf("RangeFacetCounts{field=%s ranges=%d total=%d}", r.field, len(r.ranges), r.total)
}

// --- Convenience constructors for common range configurations ---

// NewRangeFacetCountsWithBounds creates ranges from a list of boundary values.
// For N boundaries, N-1 ranges are created: [b0,b1), [b1,b2), ..., [bn-2,bn-1).
// The last range includes the upper bound.
func NewRangeFacetCountsWithBounds(field string, bounds []float64) *RangeFacetCounts {
	if len(bounds) < 2 {
		return NewRangeFacetCounts(field, nil)
	}
	ranges := make([]RangeFacetRequest, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		min, max := bounds[i], bounds[i+1]
		ranges = append(ranges, RangeFacetRequest{
			Label:        fmt.Sprintf("%g-%g", min, max),
			Min:          &min,
			Max:          &max,
			MinInclusive: true,
			MaxInclusive: i == len(bounds)-2, // last range includes upper bound
		})
	}
	return NewRangeFacetCounts(field, ranges)
}

// NewRangeFacetCountsWithGap creates evenly-spaced ranges from min to max
// with the given gap size.
func NewRangeFacetCountsWithGap(field string, min, max, gap float64) *RangeFacetCounts {
	var bounds []float64
	for v := min; v <= max; v += gap {
		bounds = append(bounds, v)
	}
	if bounds[len(bounds)-1] < max {
		bounds = append(bounds, max)
	}
	return NewRangeFacetCountsWithBounds(field, bounds)
}
