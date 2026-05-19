// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// MonotonicPackedBuilder returns a builder that subtracts the linear
// expectation expected(min, avg, i) from each value before packing the
// residual via the delta strategy — efficient for sequences that grow
// roughly linearly (e.g. doc-id deltas, monotonically increasing
// offsets).
//
// This is the Go port of
// org.apache.lucene.util.packed.MonotonicLongValues.Builder in Apache
// Lucene 10.4.0. Lucene models the variant through subclassing
// (MonotonicLongValues extends DeltaPackedLongValues); Gocene plugs a
// dedicated packStrategy into the shared PackedLongValues / Builder
// instead — see [[packed_long_values]].
func MonotonicPackedBuilder(pageSize int, acceptableOverheadRatio float32) (*PackedLongValuesBuilder, error) {
	pageShift, err := CheckBlockSize(pageSize, packedLongValuesMinPageSize, packedLongValuesMaxPageSize)
	if err != nil {
		return nil, err
	}
	b := &PackedLongValuesBuilder{
		pageShift:               pageShift,
		pageMask:                pageSize - 1,
		acceptableOverheadRatio: acceptableOverheadRatio,
		pending:                 make([]int64, pageSize),
		values:                  make([]Reader, 16),
		mins:                    make([]int64, 16),
		averages:                make([]float32, 16),
		strategy:                monotonicPackStrategy{},
	}
	return b, nil
}

// monotonicPackStrategy subtracts expected(0, average, i) from each
// value, then defers to the delta strategy for the residual. The
// per-block average is the slope inferred from the first and last
// pending value (matching Lucene exactly: (last - first) / (n - 1) as a
// float32).
type monotonicPackStrategy struct{}

func (monotonicPackStrategy) describe() string { return "monotonic" }

func (monotonicPackStrategy) get(plv *PackedLongValues, block, element int) int64 {
	return monotonicExpected(plv.mins[block], plv.averages[block], element) + plv.values[block].Get(element)
}

func (monotonicPackStrategy) pack(pending []int64, numValues, block int, ratio float32, plv *PackedLongValues) {
	if numValues <= 0 {
		return
	}
	var average float32
	if numValues > 1 {
		average = float32(pending[numValues-1]-pending[0]) / float32(numValues-1)
	}
	for i := 0; i < numValues; i++ {
		pending[i] -= monotonicExpected(0, average, i)
	}
	deltaPackStrategy{}.pack(pending, numValues, block, ratio, plv)
	if plv.averages == nil {
		// Defensive: should never happen when constructed via MonotonicPackedBuilder.
		plv.averages = make([]float32, len(plv.values))
	}
	plv.averages[block] = average
}
