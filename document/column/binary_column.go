// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BinaryColumn is a Column that provides variable-size binary values via a tuple cursor.
// Used for BINARY, SORTED, and SORTED_SET doc values, and for stored/indexed binary or text fields.
// Mirrors org.apache.lucene.document.column.BinaryColumn.
type BinaryColumn interface {
	Column
	// StoredType returns the StoredValueType to emit when this column is written to stored fields.
	StoredType() document.StoredValueType
	// Tuples returns a fresh tuple cursor starting at the beginning of the batch.
	Tuples() ObjectTupleCursor[*util.BytesRef]
	// Values returns a fresh values cursor iterating dense binary values for doc-ids [0, numDocs).
	Values() BytesRefValuesCursor
}

// BaseBinaryColumn provides a basic implementation of the BinaryColumn interface.
// It implements the default StoredType and a default Values() that throws for sparse columns.
type BaseBinaryColumn struct {
	BaseColumn
}

// NewBaseBinaryColumn creates a new BaseBinaryColumn.
func NewBaseBinaryColumn(name string, fieldType schema.IndexableFieldType, density Density) BaseBinaryColumn {
	return BaseBinaryColumn{
		BaseColumn: NewBaseColumn(name, fieldType, density),
	}
}

// StoredType returns the StoredValueType to emit when this column is written to stored fields.
// The default is StoredValueTypeBinary.
func (c BaseBinaryColumn) StoredType() document.StoredValueType {
	return document.StoredValueTypeBinary
}

// Values returns a fresh values cursor iterating dense binary values for doc-ids [0, numDocs).
// This default implementation panics if density is not DENSE, matching Lucene's UnsupportedOperationException.
func (c BaseBinaryColumn) Values() BytesRefValuesCursor {
	if c.Density() != DensityDense {
		panic(fmt.Sprintf("values() requires density() == DENSE for column %q", c.Name()))
	}
	// Actual implementation is provided by subclasses.
	// In Go, since this is a base struct, the final implementation must override this.
	panic("Values() must be implemented by the specific BinaryColumn implementation")
}
