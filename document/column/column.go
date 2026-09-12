// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Density describes whether a column has a value for every document in the batch.
// This is a contract the column asserts up-front so the indexing chain can pick
// the right code path without probing the data.
// Mirrors org.apache.lucene.document.column.Column.Density.
type Density int

const (
	// DensityDense means the column has a value for every batch-local doc-id in [0, numDocs), in order.
	DensityDense Density = iota
	// DensitySparse means the column may be missing values or have multiple values for some doc-ids.
	DensitySparse
)

// Column describes a single field's values across multiple documents in a ColumnBatch.
// A Column carries only metadata (name, field type, and density); iteration is
// performed via cursors obtained from implementations like BinaryColumn or LongColumn.
// Mirrors org.apache.lucene.document.column.Column.
type Column interface {
	// Name returns the field name.
	Name() string
	// FieldType returns the field type describing how this field is indexed.
	FieldType() spi.IndexableFieldType
	// Density returns the density of this column (whether every doc has a value).
	Density() Density
}

// BaseColumn provides a basic implementation of the Column interface.
type BaseColumn struct {
	name      string
	fieldType spi.IndexableFieldType
	density   Density
}

// NewBaseColumn creates a new BaseColumn with the given metadata.
func NewBaseColumn(name string, fieldType spi.IndexableFieldType, density Density) BaseColumn {
	return BaseColumn{
		name:      name,
		fieldType: fieldType,
		density:   density,
	}
}

func (c BaseColumn) Name() string {
	return c.name
}

func (c BaseColumn) FieldType() spi.IndexableFieldType {
	return c.fieldType
}

func (c BaseColumn) Density() Density {
	return c.density
}
