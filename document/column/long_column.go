// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/document"
)

// LongNumericKind discriminates the numeric type of a LongColumn.
type LongNumericKind int

const (
	// LongNumericKindInt means the column carries 32-bit integers.
	LongNumericKindInt LongNumericKind = iota
	// LongNumericKindLong means the column carries 64-bit integers.
	LongNumericKindLong
	// LongNumericKindFloat means the column carries 32-bit floats.
	LongNumericKindFloat
	// LongNumericKindDouble means the column carries 64-bit floats.
	LongNumericKindDouble
)

// LongValuesCursor is a values cursor over a dense LongColumn.
// The cursor produces exactly Size() values for consecutive batch-local doc-ids
// starting at 0, one per call to NextValue().
type LongValuesCursor interface {
	// Size returns the total number of values this cursor will produce.
	Size() int
	// NextValue returns the next numeric value as an int64 (bit-cast for floats).
	NextValue() int64
}

// LongColumn is a Column that provides numeric values via a tuple cursor.
// Used for NUMERIC and SORTED_NUMERIC doc values, and for numeric stored fields.
// Mirrors org.apache.lucene.document.column.LongColumn.
type LongColumn interface {
	Column
	// NumericKind returns the numeric type of the values in this column.
	NumericKind() LongNumericKind
	// StoredType returns the StoredValueType to emit when this column is written to stored fields.
	StoredType() document.StoredValueType
	// Tuples returns a fresh tuple cursor starting at the beginning of the batch.
	Tuples() ObjectTupleCursor[int64]
	// Values returns a fresh values cursor iterating dense numeric values for doc-ids [0, numDocs).
	Values() LongValuesCursor
}
