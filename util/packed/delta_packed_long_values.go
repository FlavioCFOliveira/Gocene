// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// DeltaPackedBuilderDefault returns a builder that subtracts a per-block
// minimum before packing — efficient for sequences whose values cluster
// around a per-block constant (typical for sorted or near-sorted long
// streams such as document-frequency arrays or offset tables).
//
// This is the Go port of
// org.apache.lucene.util.packed.DeltaPackedLongValues.Builder in Apache
// Lucene 10.4.0. Lucene models the variant through subclassing
// (DeltaPackedLongValues extends PackedLongValues); Gocene plugs a
// dedicated packStrategy into the shared PackedLongValues / Builder
// instead — see [[packed_long_values]] and the sibling
// [[monotonic_long_values]].
//
// The returned builder is byte-for-byte equivalent to
// PackedLongValues.deltaPackedBuilder(pageSize, acceptableOverheadRatio)
// in the Java reference: the per-block min is stored alongside the
// PackedInts.Reader and Get(i) reconstructs the original value as
// mins[block] + values[block].Get(element).
func DeltaPackedBuilderDefault(pageSize int, acceptableOverheadRatio float32) (*PackedLongValuesBuilder, error) {
	return DeltaPackedBuilder(pageSize, acceptableOverheadRatio)
}

// deltaPackedGet mirrors DeltaPackedLongValues.get(block, element) from
// the Java reference. It is provided here as a documentation anchor for
// the delta-decoding contract; the actual call site lives in
// deltaPackStrategy.get within [[packed_long_values]].
func deltaPackedGet(plv *PackedLongValues, block, element int) int64 {
	return plv.mins[block] + plv.values[block].Get(element)
}
