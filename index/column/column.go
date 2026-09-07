package column

import (
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// Density describes whether a column has a value for every document in the batch.
type Density int

const (
	// DensityDense indicates the column has a value for every batch-local doc-id in [0, numDocs), in order.
	DensityDense Density = iota
	// DensitySparse indicates the column may be missing values or have multiple values for some doc-ids.
	DensitySparse
)

// Column is a single field's values across multiple documents in a ColumnBatch.
type Column interface {
	// Name returns the field name.
	Name() string
	// FieldType returns the field type describing how this field is indexed.
	FieldType() schema.IndexableFieldType
	// Density returns the density of this column (whether every doc has a value).
	Density() Density
}

// NumericKind marks how the long bits should be interpreted in a LongColumn.
type NumericKind int

const (
	NumericKindInt NumericKind = iota
	NumericKindLong
	NumericKindFloat
	NumericKindDouble
)

// LongColumn is a Column that provides long values.
type LongColumn interface {
	Column
	// NumericKind returns the numeric interpretation of the column's long values.
	NumericKind() NumericKind
	// StoredType returns the stored-field variant emitted for this column.
	StoredType() document.StoredValueType
}

// BinaryColumn is a Column that provides variable-size binary values.
type BinaryColumn interface {
	Column
	// StoredType returns the stored-field variant emitted for this column.
	StoredType() document.StoredValueType
}

// DictionaryColumn is a Column that provides string or binary values via a pre-defined term dictionary.
type DictionaryColumn interface {
	Column
	// Dictionary returns the term dictionary.
	Dictionary() [][]byte
	// StoredType returns the stored-field variant emitted for this column.
	StoredType() document.StoredValueType
}

// TokenStreamColumn is a Column that provides caller-supplied TokenStreams for term inversion.
type TokenStreamColumn interface {
	Column
}

// VectorColumn is a Column that provides KNN vector values.
type VectorColumn interface {
	Column
}
