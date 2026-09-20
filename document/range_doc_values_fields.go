// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// This file ports the Range*DocValuesField family from Lucene 10.4.0:
// IntRangeDocValuesField, LongRangeDocValuesField, FloatRangeDocValuesField,
// DoubleRangeDocValuesField. Each is a BinaryDocValuesField wrapper whose
// payload is the Lucene-encoded packed range bytes (see EncodeXxxRangeLucene
// in range_fields_lucene.go).
//
// NewSlowIntersectsQuery factories are deferred — depend on
// search.RangeFieldQuery (task 249). See backlog #2695.


func checkRangeDocValuesArgs(nMin, nMax int) error {
	if nMin == 0 || nMax == 0 {
		return fmt.Errorf("range doc-values field requires at least one dimension")
	}
	if nMin > 4 {
		return fmt.Errorf("range doc-values field supports at most 4 dimensions; got %d", nMin)
	}
	if nMin != nMax {
		return fmt.Errorf("min/max dimension count mismatch: %d vs %d", nMin, nMax)
	}
	return nil
}

func mustDim(dim, max int) {
	if dim < 0 || dim >= max {
		panic(fmt.Sprintf("dimension %d out of range [0, %d)", dim, max))
	}
}
