// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DictionaryValuesCursor is a values cursor over a dense DictionaryColumn.
// The cursor produces exactly Size() values for consecutive batch-local doc-ids
// starting at 0, one per call to NextValue().
type DictionaryValuesCursor interface {
	// Size returns the total number of values this cursor will produce.
	Size() int
	// NextValue returns the next binary value.
	NextValue() *util.BytesRef
}

// DictionaryColumn is a Column that provides dictionary-encoded binary values.
// Used for SORTED and SORTED_SET doc values, and for stored binary or text fields.
// Mirrors org.apache.lucene.document.column.DictionaryColumn.
type DictionaryColumn interface {
	Column
	// StoredType returns the StoredValueType to emit when this column is written to stored fields.
	StoredType() document.StoredValueType
	// Tuples returns a fresh tuple cursor starting at the beginning of the batch.
	Tuples() ObjectTupleCursor[*util.BytesRef]
	// Values returns a fresh values cursor iterating dense binary values for doc-ids [0, numDocs).
	Values() DictionaryValuesCursor
}
