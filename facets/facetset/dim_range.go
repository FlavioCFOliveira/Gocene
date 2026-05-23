// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facetset

import (
	"fmt"
	"math"
)

// DimRange describes an inclusive interval [Min, Max] over a single
// comparable int64 dimension. Mirrors
// org.apache.lucene.facet.facetset.DimRange.
type DimRange struct {
	// Min is the inclusive lower bound (stored as a sortable int64).
	Min int64
	// Max is the inclusive upper bound (stored as a sortable int64).
	Max int64
}

// NewDimRange constructs an inclusive range from raw int64 values.
// The constructor swaps min/max if they are inverted so callers never
// have to worry about ordering.
func NewDimRange(min, max int64) DimRange {
	if min > max {
		min, max = max, min
	}
	return DimRange{Min: min, Max: max}
}

// FromLongs creates a DimRange for the given int64 (or int) bounds.
// minInclusive/maxInclusive control whether the bounds are open or closed.
// Mirrors DimRange.fromLongs.
func FromLongs(min int64, minInclusive bool, max int64, maxInclusive bool) (DimRange, error) {
	if !minInclusive {
		if min != math.MaxInt64 {
			min++
		} else {
			return DimRange{}, fmt.Errorf("dimrange: invalid min input: %d", min)
		}
	}
	if !maxInclusive {
		if max != math.MinInt64 {
			max--
		} else {
			return DimRange{}, fmt.Errorf("dimrange: invalid max input: %d", max)
		}
	}
	if min > max {
		return DimRange{}, fmt.Errorf("dimrange: minimum cannot be greater than maximum, min=%d max=%d", min, max)
	}
	return DimRange{Min: min, Max: max}, nil
}

// FromDoubles creates a DimRange for the given float64 bounds. The values are
// normalised to inclusive bounds using math.Nextafter and then encoded via the
// Lucene sortable-double encoding (sign-bit flip). NaN bounds are rejected.
// Mirrors DimRange.fromDoubles.
func FromDoubles(min float64, minInclusive bool, max float64, maxInclusive bool) (DimRange, error) {
	if math.IsNaN(min) {
		return DimRange{}, fmt.Errorf("dimrange: min cannot be NaN")
	}
	if !minInclusive {
		min = math.Nextafter(min, math.Inf(1))
	}
	if math.IsNaN(max) {
		return DimRange{}, fmt.Errorf("dimrange: max cannot be NaN")
	}
	if !maxInclusive {
		max = math.Nextafter(max, math.Inf(-1))
	}
	if min > max {
		return DimRange{}, fmt.Errorf("dimrange: minimum cannot be greater than maximum")
	}
	return DimRange{
		Min: doubleToSortableLong(min),
		Max: doubleToSortableLong(max),
	}, nil
}

// FromFloats creates a DimRange for the given float32 bounds. The values are
// normalised to inclusive bounds using math.Nextafter32 and then encoded via
// the Lucene sortable-float encoding (sign-bit flip). NaN bounds are rejected.
// Mirrors DimRange.fromFloats.
func FromFloats(min float32, minInclusive bool, max float32, maxInclusive bool) (DimRange, error) {
	if math.IsNaN(float64(min)) {
		return DimRange{}, fmt.Errorf("dimrange: min cannot be NaN")
	}
	if !minInclusive {
		min = math.Nextafter32(min, float32(math.Inf(1)))
	}
	if math.IsNaN(float64(max)) {
		return DimRange{}, fmt.Errorf("dimrange: max cannot be NaN")
	}
	if !maxInclusive {
		max = math.Nextafter32(max, float32(math.Inf(-1)))
	}
	if min > max {
		return DimRange{}, fmt.Errorf("dimrange: minimum cannot be greater than maximum")
	}
	return DimRange{
		Min: int64(floatToSortableInt(min)),
		Max: int64(floatToSortableInt(max)),
	}, nil
}

// Contains reports whether v lies within the range, inclusive on both ends.
func (r DimRange) Contains(v int64) bool {
	return v >= r.Min && v <= r.Max
}
