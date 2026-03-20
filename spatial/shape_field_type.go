// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// ShapeFieldType represents the type of a spatial shape field.
// This defines how spatial data is indexed and stored.
//
// ShapeFieldType configures the indexing strategy, stored fields,
// and doc values settings for spatial shape fields.
//
// This is the Go port of Lucene's FieldType concept adapted for spatial data.
type ShapeFieldType struct {
	// name is the type name
	name string

	// indexed indicates if the field should be indexed for searching
	indexed bool

	// stored indicates if the field should be stored
	stored bool

	// docValues indicates if the field should have doc values
	docValues bool

	// tokenized indicates if the field should be tokenized
	tokenized bool

	// storeTermVectors indicates if term vectors should be stored
	storeTermVectors bool

	// storeTermVectorPositions indicates if term vector positions should be stored
	storeTermVectorPositions bool

	// storeTermVectorOffsets indicates if term vector offsets should be stored
	storeTermVectorOffsets bool

	// omitNorms indicates if norms should be omitted
	omitNorms bool

	// indexOptions specifies the indexing options
	indexOptions IndexOptions

	// docValuesType specifies the doc values type
	docValuesType DocValuesType

	// dimensionCount is the number of dimensions (for multi-dimensional shapes)
	dimensionCount int

	// spatialStrategy is the strategy used for indexing
	spatialStrategy SpatialStrategy
}

// IndexOptions specifies what should be indexed for a field.
type IndexOptions int

const (
	// IndexOptionsNone means the field is not indexed.
	IndexOptionsNone IndexOptions = iota

	// IndexOptionsDocs only indexes documents (no term frequencies or positions).
	IndexOptionsDocs

	// IndexOptionsDocsAndFreqs indexes documents and term frequencies.
	IndexOptionsDocsAndFreqs

	// IndexOptionsDocsAndFreqsAndPositions indexes documents, frequencies, and positions.
	IndexOptionsDocsAndFreqsAndPositions

	// IndexOptionsDocsAndFreqsAndPositionsAndOffsets indexes everything including offsets.
	IndexOptionsDocsAndFreqsAndPositionsAndOffsets
)

// DocValuesType specifies the type of doc values.
type DocValuesType int

const (
	// DocValuesTypeNone means no doc values.
	DocValuesTypeNone DocValuesType = iota

	// DocValuesTypeNumeric for numeric doc values.
	DocValuesTypeNumeric

	// DocValuesTypeBinary for binary doc values.
	DocValuesTypeBinary

	// DocValuesTypeSorted for sorted doc values.
	DocValuesTypeSorted

	// DocValuesTypeSortedNumeric for sorted numeric doc values.
	DocValuesTypeSortedNumeric

	// DocValuesTypeSortedSet for sorted set doc values.
	DocValuesTypeSortedSet
)

// NewShapeFieldType creates a new ShapeFieldType with default settings.
//
// By default:
//   - Indexed: true
//   - Stored: false
//   - DocValues: false
//   - Tokenized: true
//   - IndexOptions: IndexOptionsDocsAndFreqsAndPositions
//
// Returns a new ShapeFieldType instance.
func NewShapeFieldType() *ShapeFieldType {
	return &ShapeFieldType{
		name:                     "ShapeField",
		indexed:                  true,
		stored:                   false,
		docValues:                false,
		tokenized:                true,
		storeTermVectors:         false,
		storeTermVectorPositions: false,
		storeTermVectorOffsets:   false,
		omitNorms:                true,
		indexOptions:             IndexOptionsDocsAndFreqsAndPositions,
		docValuesType:            DocValuesTypeNone,
		dimensionCount:           2, // Default to 2D (x, y)
	}
}

// NewStoredShapeFieldType creates a ShapeFieldType that stores the field.
func NewStoredShapeFieldType() *ShapeFieldType {
	ft := NewShapeFieldType()
	ft.stored = true
	return ft
}

// NewIndexedShapeFieldType creates a ShapeFieldType optimized for indexing only.
func NewIndexedShapeFieldType() *ShapeFieldType {
	ft := NewShapeFieldType()
	ft.stored = false
	ft.indexed = true
	return ft
}

// NewDocValuesShapeFieldType creates a ShapeFieldType with doc values enabled.
func NewDocValuesShapeFieldType() *ShapeFieldType {
	ft := NewShapeFieldType()
	ft.docValues = true
	ft.docValuesType = DocValuesTypeBinary
	return ft
}

// GetName returns the type name.
func (ft *ShapeFieldType) GetName() string {
	return ft.name
}

// SetName sets the type name.
func (ft *ShapeFieldType) SetName(name string) {
	ft.name = name
}

// IsIndexed returns true if the field is indexed.
func (ft *ShapeFieldType) IsIndexed() bool {
	return ft.indexed
}

// SetIndexed sets whether the field should be indexed.
func (ft *ShapeFieldType) SetIndexed(indexed bool) *ShapeFieldType {
	ft.indexed = indexed
	return ft
}

// IsStored returns true if the field is stored.
func (ft *ShapeFieldType) IsStored() bool {
	return ft.stored
}

// SetStored sets whether the field should be stored.
func (ft *ShapeFieldType) SetStored(stored bool) *ShapeFieldType {
	ft.stored = stored
	return ft
}

// HasDocValues returns true if the field has doc values.
func (ft *ShapeFieldType) HasDocValues() bool {
	return ft.docValues
}

// SetDocValues sets whether the field should have doc values.
func (ft *ShapeFieldType) SetDocValues(docValues bool) *ShapeFieldType {
	ft.docValues = docValues
	return ft
}

// IsTokenized returns true if the field is tokenized.
func (ft *ShapeFieldType) IsTokenized() bool {
	return ft.tokenized
}

// SetTokenized sets whether the field should be tokenized.
func (ft *ShapeFieldType) SetTokenized(tokenized bool) *ShapeFieldType {
	ft.tokenized = tokenized
	return ft
}

// StoreTermVectors returns true if term vectors should be stored.
func (ft *ShapeFieldType) StoreTermVectors() bool {
	return ft.storeTermVectors
}

// SetStoreTermVectors sets whether term vectors should be stored.
func (ft *ShapeFieldType) SetStoreTermVectors(store bool) *ShapeFieldType {
	ft.storeTermVectors = store
	return ft
}

// StoreTermVectorPositions returns true if term vector positions should be stored.
func (ft *ShapeFieldType) StoreTermVectorPositions() bool {
	return ft.storeTermVectorPositions
}

// SetStoreTermVectorPositions sets whether term vector positions should be stored.
func (ft *ShapeFieldType) SetStoreTermVectorPositions(store bool) *ShapeFieldType {
	ft.storeTermVectorPositions = store
	return ft
}

// StoreTermVectorOffsets returns true if term vector offsets should be stored.
func (ft *ShapeFieldType) StoreTermVectorOffsets() bool {
	return ft.storeTermVectorOffsets
}

// SetStoreTermVectorOffsets sets whether term vector offsets should be stored.
func (ft *ShapeFieldType) SetStoreTermVectorOffsets(store bool) *ShapeFieldType {
	ft.storeTermVectorOffsets = store
	return ft
}

// OmitNorms returns true if norms should be omitted.
func (ft *ShapeFieldType) OmitNorms() bool {
	return ft.omitNorms
}

// SetOmitNorms sets whether norms should be omitted.
func (ft *ShapeFieldType) SetOmitNorms(omit bool) *ShapeFieldType {
	ft.omitNorms = omit
	return ft
}

// GetIndexOptions returns the indexing options.
func (ft *ShapeFieldType) GetIndexOptions() IndexOptions {
	return ft.indexOptions
}

// SetIndexOptions sets the indexing options.
func (ft *ShapeFieldType) SetIndexOptions(options IndexOptions) *ShapeFieldType {
	ft.indexOptions = options
	return ft
}

// GetDocValuesType returns the doc values type.
func (ft *ShapeFieldType) GetDocValuesType() DocValuesType {
	return ft.docValuesType
}

// SetDocValuesType sets the doc values type.
func (ft *ShapeFieldType) SetDocValuesType(docValuesType DocValuesType) *ShapeFieldType {
	ft.docValuesType = docValuesType
	ft.docValues = docValuesType != DocValuesTypeNone
	return ft
}

// GetDimensionCount returns the number of dimensions.
func (ft *ShapeFieldType) GetDimensionCount() int {
	return ft.dimensionCount
}

// SetDimensionCount sets the number of dimensions.
func (ft *ShapeFieldType) SetDimensionCount(count int) *ShapeFieldType {
	if count > 0 {
		ft.dimensionCount = count
	}
	return ft
}

// GetSpatialStrategy returns the spatial strategy.
func (ft *ShapeFieldType) GetSpatialStrategy() SpatialStrategy {
	return ft.spatialStrategy
}

// SetSpatialStrategy sets the spatial strategy.
func (ft *ShapeFieldType) SetSpatialStrategy(strategy SpatialStrategy) *ShapeFieldType {
	ft.spatialStrategy = strategy
	return ft
}

// Freeze makes this field type immutable.
// After freezing, any attempts to modify the type will panic.
// This is not fully implemented - returns self for compatibility.
func (ft *ShapeFieldType) Freeze() *ShapeFieldType {
	// In a full implementation, this would make the type immutable
	// For now, just return self
	return ft
}

// CheckConsistency checks if the field type configuration is consistent.
// Returns an error if there are inconsistencies.
func (ft *ShapeFieldType) CheckConsistency() error {
	if ft.indexed && ft.indexOptions == IndexOptionsNone {
		return fmt.Errorf("indexed field must have non-None index options")
	}

	if ft.docValues && ft.docValuesType == DocValuesTypeNone {
		return fmt.Errorf("field with doc values must have non-None doc values type")
	}

	if ft.dimensionCount < 1 {
		return fmt.Errorf("dimension count must be at least 1")
	}

	return nil
}

// String returns a string representation of this field type.
func (ft *ShapeFieldType) String() string {
	return fmt.Sprintf("ShapeFieldType(name=%s, indexed=%v, stored=%v, docValues=%v, strategy=%v)",
		ft.name, ft.indexed, ft.stored, ft.docValues, ft.spatialStrategy != nil)
}

// Equals checks if this field type equals another.
func (ft *ShapeFieldType) Equals(other *ShapeFieldType) bool {
	if other == nil {
		return false
	}

	return ft.name == other.name &&
		ft.indexed == other.indexed &&
		ft.stored == other.stored &&
		ft.docValues == other.docValues &&
		ft.tokenized == other.tokenized &&
		ft.storeTermVectors == other.storeTermVectors &&
		ft.storeTermVectorPositions == other.storeTermVectorPositions &&
		ft.storeTermVectorOffsets == other.storeTermVectorOffsets &&
		ft.omitNorms == other.omitNorms &&
		ft.indexOptions == other.indexOptions &&
		ft.docValuesType == other.docValuesType &&
		ft.dimensionCount == other.dimensionCount
}

// HashCode returns a hash code for this field type.
func (ft *ShapeFieldType) HashCode() int {
	result := 17
	result = 31*result + hashCode(ft.name)
	result = 31*result + boolHash(ft.indexed)
	result = 31*result + boolHash(ft.stored)
	result = 31*result + boolHash(ft.docValues)
	result = 31*result + int(ft.indexOptions)
	result = 31*result + ft.dimensionCount
	return result
}

// boolHash returns a hash code for a boolean.
func boolHash(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ShapeField represents a spatial shape field in a document.
// This is a convenience wrapper that creates the appropriate indexable fields
// based on the spatial strategy and field type.
type ShapeField struct {
	name      string
	shape     Shape
	fieldType *ShapeFieldType
}

// NewShapeField creates a new ShapeField.
//
// Parameters:
//   - name: The field name
//   - shape: The spatial shape
//   - fieldType: The field type configuration
//
// Returns a new ShapeField or an error if parameters are invalid.
func NewShapeField(name string, shape Shape, fieldType *ShapeFieldType) (*ShapeField, error) {
	if name == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if shape == nil {
		return nil, fmt.Errorf("shape cannot be nil")
	}
	if fieldType == nil {
		return nil, fmt.Errorf("field type cannot be nil")
	}

	return &ShapeField{
		name:      name,
		shape:     shape,
		fieldType: fieldType,
	}, nil
}

// GetName returns the field name.
func (f *ShapeField) GetName() string {
	return f.name
}

// GetShape returns the spatial shape.
func (f *ShapeField) GetShape() Shape {
	return f.shape
}

// GetFieldType returns the field type.
func (f *ShapeField) GetFieldType() *ShapeFieldType {
	return f.fieldType
}

// CreateIndexableFields generates the indexable fields for this shape field.
// This uses the spatial strategy from the field type to create the appropriate
// Lucene fields for indexing.
//
// Returns a slice of IndexableField instances.
func (f *ShapeField) CreateIndexableFields() ([]document.IndexableField, error) {
	strategy := f.fieldType.GetSpatialStrategy()
	if strategy == nil {
		return nil, fmt.Errorf("field type has no spatial strategy")
	}

	return strategy.CreateIndexableFields(f.shape)
}

// String returns a string representation of this field.
func (f *ShapeField) String() string {
	return fmt.Sprintf("ShapeField(name=%s, shape=%v, type=%s)",
		f.name, f.shape, f.fieldType.GetName())
}

// Common ShapeFieldType presets

// ShapeFieldTypeDefault is the default shape field type (indexed, not stored).
var ShapeFieldTypeDefault = NewShapeFieldType()

// ShapeFieldTypeStored is a shape field type that stores the field.
var ShapeFieldTypeStored = NewStoredShapeFieldType()

// ShapeFieldTypeIndexedOnly is a shape field type for indexing only (no storage).
var ShapeFieldTypeIndexedOnly = NewIndexedShapeFieldType()

// ShapeFieldTypeDocValues is a shape field type with doc values enabled.
var ShapeFieldTypeDocValues = NewDocValuesShapeFieldType()

// ShapeFieldTypeIndexedAndStored is a shape field type that both indexes and stores.
var ShapeFieldTypeIndexedAndStored = NewShapeFieldType().SetIndexed(true).SetStored(true).Freeze()
