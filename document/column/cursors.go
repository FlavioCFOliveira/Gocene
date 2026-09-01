// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ObjectTupleCursor is a tuple cursor over a Column whose values are objects.
// It yields (docID, value) pairs. Batch-local doc-ids are returned in non-decreasing order.
// Mirrors org.apache.lucene.document.column.ObjectTupleCursor.
type ObjectTupleCursor[T any] interface {
	// NextDoc advances to the next tuple and returns its doc-id, or -1 if exhausted.
	// Returned doc-ids are batch-local (0 to numDocs - 1) and are emitted in non-decreasing order.
	NextDoc() int
	// Value returns the value at the current cursor position.
	Value() T
}

// BytesRefValuesCursor is a values cursor over a dense BinaryColumn.
// The cursor produces exactly Size() values for consecutive batch-local doc-ids
// starting at 0, one per call to NextValue().
// Mirrors org.apache.lucene.document.column.BytesRefValuesCursor.
type BytesRefValuesCursor interface {
	// Size returns the total number of values this cursor will produce.
	Size() int
	// NextValue returns the next BytesRef value.
	NextValue() *util.BytesRef
	// FillPackedPoints bulk-fills length fixed-width packed point records into dst
	// starting at byte offset, advancing the cursor by length.
	FillPackedPoints(dst []byte, offset, length, width int)
}
