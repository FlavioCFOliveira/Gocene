// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/document"
)

// VectorValuesCursor is a values cursor over a dense VectorColumn.
// The cursor produces exactly Size() values for consecutive batch-local doc-ids
// starting at 0, one per call to NextValue().
type VectorValuesCursor interface {
	// Size returns the total number of values this cursor will produce.
	Size() int
	// NextValue returns the next vector value.
	NextValue() []float32
}

// VectorColumn is a Column that provides vector values.
// Used exclusively for kNN vector indexing.
// Mirrors org.apache.lucene.document.column.VectorColumn.
type VectorColumn interface {
	Column
	// StoredType returns the StoredValueType to emit when this column is written to stored fields.
	StoredType() document.StoredValueType
	// Tuples returns a fresh tuple cursor starting at the beginning of the batch.
	Tuples() ObjectTupleCursor[[]float32]
	// VectorDimension returns the dimension of the vectors in this column.
	VectorDimension() int
	// Values returns a fresh values cursor iterating dense vector values for doc-ids [0, numDocs).
	Values() VectorValuesCursor
}
