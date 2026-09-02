// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/schema"
)

// DocValuesSkipIndexType defines options for skip indexes on doc values.
// This is a local definition to avoid circular imports with the index package.
// Mirrors org.apache.lucene.index.DocValuesSkipIndexType from Apache Lucene 10.5.0.
type DocValuesSkipIndexType int

const (
	// DocValuesSkipIndexTypeNone: No skip index should be created.
	DocValuesSkipIndexTypeNone DocValuesSkipIndexType = iota
	// DocValuesSkipIndexTypeRange: Record range of values.
	DocValuesSkipIndexTypeRange
)

// Re-export vector types from schema to avoid circular imports.
// Tests and other code may reference these through the document/index packages.
// Mirrors org.apache.lucene.index.VectorEncoding and VectorSimilarityFunction.

// VectorEncodingByte stores vector values as signed bytes.
const VectorEncodingByte = schema.VectorEncodingByte

// VectorEncodingFloat32 stores vector values as IEEE 32-bit floating point.
const VectorEncodingFloat32 = schema.VectorEncodingFloat32

// VectorSimilarityFunctionEuclidean uses squared Euclidean distance.
const VectorSimilarityFunctionEuclidean = schema.VectorSimilarityFunctionEuclidean

// VectorSimilarityFunctionDotProduct uses dot product similarity.
const VectorSimilarityFunctionDotProduct = schema.VectorSimilarityFunctionDotProduct

// VectorSimilarityFunctionCosine uses cosine similarity.
const VectorSimilarityFunctionCosine = schema.VectorSimilarityFunctionCosine

// VectorSimilarityFunctionMaximumInnerProduct uses maximum inner product similarity.
const VectorSimilarityFunctionMaximumInnerProduct = schema.VectorSimilarityFunctionMaximumInnerProduct

// FieldType describes the properties of a field.
//
// This is the Go port of Lucene's org.apache.lucene.document.FieldType.
type FieldType struct {
	// Public fields for backward compatibility
	Stored                      bool
	Tokenized                   bool
	StoreTermVectors            bool
	StoreTermVectorOffsets      bool
	StoreTermVectorPositions    bool
	StoreTermVectorPayloads     bool
	OmitNorms                   bool
	Indexed                     bool // Mirrors whether IndexOptions != NONE
	IndexOptions                schema.IndexOptions
	DocValuesType               schema.DocValuesType
	VectorDimension             int
	VectorEncoding              schema.VectorEncoding
	VectorSimilarityFunction    schema.VectorSimilarityFunction
	DocValuesSkipIndex          DocValuesSkipIndexType

	// Private fields with getter methods
	pointDimensionCount      int
	pointIndexDimensionCount int
	pointNumBytes            int
	frozen                   bool
	attributes               map[string]string
}

// NewFieldType creates a new mutable FieldType with default properties.
func NewFieldType() *FieldType {
	return &FieldType{
		Stored:                   false,
		Tokenized:                false,
		StoreTermVectors:         false,
		StoreTermVectorOffsets:   false,
		StoreTermVectorPositions: false,
		StoreTermVectorPayloads:  false,
		OmitNorms:                false,
		IndexOptions:             schema.IndexOptionsNone,
		DocValuesType:            schema.DocValuesTypeNone,
		pointDimensionCount:      0,
		pointIndexDimensionCount: 0,
		pointNumBytes:            0,
		VectorDimension:          0,
		VectorEncoding:           schema.VectorEncodingFloat32,
		VectorSimilarityFunction: schema.VectorSimilarityFunctionEuclidean,
		DocValuesSkipIndex:       DocValuesSkipIndexTypeNone,
		frozen:                   false,
		attributes:               make(map[string]string),
	}
}

// NewLuceneFieldType creates a FieldType with Lucene's defaults
// (Tokenized=true instead of false).
func NewLuceneFieldType() *FieldType {
	ft := NewFieldType()
	ft.Tokenized = true
	return ft
}

// NewFieldTypeFrom creates a new mutable FieldType with all properties from src.
// The frozen state is not copied.
func NewFieldTypeFrom(src *FieldType) *FieldType {
	ft := &FieldType{
		Stored:                      src.Stored,
		Tokenized:                   src.Tokenized,
		StoreTermVectors:            src.StoreTermVectors,
		StoreTermVectorOffsets:      src.StoreTermVectorOffsets,
		StoreTermVectorPositions:    src.StoreTermVectorPositions,
		StoreTermVectorPayloads:     src.StoreTermVectorPayloads,
		OmitNorms:                   src.OmitNorms,
		IndexOptions:                src.IndexOptions,
		DocValuesType:               src.DocValuesType,
		pointDimensionCount:         src.pointDimensionCount,
		pointIndexDimensionCount:    src.pointIndexDimensionCount,
		pointNumBytes:               src.pointNumBytes,
		VectorDimension:             src.VectorDimension,
		VectorEncoding:              src.VectorEncoding,
		VectorSimilarityFunction:    src.VectorSimilarityFunction,
		DocValuesSkipIndex:          src.DocValuesSkipIndex,
		frozen:                      false,
		attributes:                  make(map[string]string),
	}
	// Copy attributes
	for k, v := range src.attributes {
		ft.attributes[k] = v
	}
	return ft
}

// checkIfFrozen throws a panic if this FieldType is frozen.
func (ft *FieldType) checkIfFrozen() {
	if ft.frozen {
		panic("this FieldType is already frozen and cannot be changed")
	}
}

// Freeze prevents future changes to this FieldType.
func (ft *FieldType) Freeze() {
	ft.frozen = true
}

// IsFrozen returns whether this FieldType is frozen.
func (ft *FieldType) IsFrozen() bool {
	return ft.frozen
}

// SetStored sets whether this field should be stored.
func (ft *FieldType) SetStored(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.Stored = value
	return ft
}

// IsStored returns whether this field is stored.
func (ft *FieldType) IsStored() bool {
	return ft.Stored
}

// SetTokenized sets whether this field is tokenized.
func (ft *FieldType) SetTokenized(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.Tokenized = value
	return ft
}

// IsTokenized returns whether this field is tokenized.
func (ft *FieldType) IsTokenized() bool {
	return ft.Tokenized
}

// SetStoreTermVectors sets whether term vectors should be stored.
func (ft *FieldType) SetStoreTermVectors(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.StoreTermVectors = value
	return ft
}

// StoresTermVectors returns whether term vectors are stored.
func (ft *FieldType) StoresTermVectors() bool {
	return ft.StoreTermVectors
}

// SetStoreTermVectorOffsets sets whether term vector offsets should be stored.
func (ft *FieldType) SetStoreTermVectorOffsets(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.StoreTermVectorOffsets = value
	return ft
}

// StoresTermVectorOffsets returns whether term vector offsets are stored.
func (ft *FieldType) StoresTermVectorOffsets() bool {
	return ft.StoreTermVectorOffsets
}

// SetStoreTermVectorPositions sets whether term vector positions should be stored.
func (ft *FieldType) SetStoreTermVectorPositions(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.StoreTermVectorPositions = value
	return ft
}

// StoresTermVectorPositions returns whether term vector positions are stored.
func (ft *FieldType) StoresTermVectorPositions() bool {
	return ft.StoreTermVectorPositions
}

// SetStoreTermVectorPayloads sets whether term vector payloads should be stored.
func (ft *FieldType) SetStoreTermVectorPayloads(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.StoreTermVectorPayloads = value
	return ft
}

// StoresTermVectorPayloads returns whether term vector payloads are stored.
func (ft *FieldType) StoresTermVectorPayloads() bool {
	return ft.StoreTermVectorPayloads
}

// SetOmitNorms sets whether norms should be omitted.
func (ft *FieldType) SetOmitNorms(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.OmitNorms = value
	return ft
}

// OmitsNorms returns whether norms are omitted.
func (ft *FieldType) OmitsNorms() bool {
	return ft.OmitNorms
}

// SetIndexOptions sets the indexing options.
func (ft *FieldType) SetIndexOptions(value schema.IndexOptions) *FieldType {
	ft.checkIfFrozen()
	ft.IndexOptions = value
	ft.Indexed = (value != schema.IndexOptionsNone)
	return ft
}

// GetIndexOptions returns the indexing options.
func (ft *FieldType) GetIndexOptions() schema.IndexOptions {
	return ft.IndexOptions
}

// SetIndexed is a convenience method that sets IndexOptions based on whether indexed is true/false.
func (ft *FieldType) SetIndexed(indexed bool) *FieldType {
	ft.checkIfFrozen()
	ft.Indexed = indexed
	if indexed {
		ft.IndexOptions = schema.IndexOptionsDocsAndFreqsAndPositions
	} else {
		ft.IndexOptions = schema.IndexOptionsNone
	}
	return ft
}

// IsIndexed returns whether this field is indexed.
func (ft *FieldType) IsIndexed() bool {
	return ft.IndexOptions != schema.IndexOptionsNone
}

// SetDocValuesType sets the doc values type.
func (ft *FieldType) SetDocValuesType(value schema.DocValuesType) *FieldType {
	ft.checkIfFrozen()
	ft.DocValuesType = value
	return ft
}

// GetDocValuesType returns the doc values type.
func (ft *FieldType) GetDocValuesType() schema.DocValuesType {
	return ft.DocValuesType
}

// SetDimensions enables points indexing with the same dimension count for storage and indexing.
func (ft *FieldType) SetDimensions(dimensionCount, dimensionNumBytes int) {
	ft.SetDimensionsIndexed(dimensionCount, dimensionCount, dimensionNumBytes)
}

// SetDimensionsIndexed enables points indexing with selectable dimension indexing.
func (ft *FieldType) SetDimensionsIndexed(dimensionCount, indexDimensionCount, dimensionNumBytes int) {
	ft.checkIfFrozen()

	if dimensionCount < 0 {
		panic(fmt.Sprintf("dimensionCount must be >= 0; got %d", dimensionCount))
	}
	if indexDimensionCount < 0 {
		panic(fmt.Sprintf("indexDimensionCount must be >= 0; got %d", indexDimensionCount))
	}
	if indexDimensionCount > dimensionCount {
		panic(fmt.Sprintf("indexDimensionCount must be <= dimensionCount: %d; got %d", dimensionCount, indexDimensionCount))
	}
	if dimensionNumBytes < 0 {
		panic(fmt.Sprintf("dimensionNumBytes must be >= 0; got %d", dimensionNumBytes))
	}
	if dimensionCount == 0 {
		if indexDimensionCount != 0 {
			panic(fmt.Sprintf("indexDimensionCount must be 0 when dimensionCount is 0; got %d", indexDimensionCount))
		}
		if dimensionNumBytes != 0 {
			panic(fmt.Sprintf("dimensionNumBytes must be 0 when dimensionCount is 0; got %d", dimensionNumBytes))
		}
	} else {
		if indexDimensionCount == 0 {
			panic(fmt.Sprintf("indexDimensionCount must be > 0 when dimensionCount is > 0; got %d", indexDimensionCount))
		}
		if dimensionNumBytes == 0 {
			panic(fmt.Sprintf("dimensionNumBytes must be > 0 when dimensionCount is > 0; got %d", dimensionNumBytes))
		}
	}

	ft.pointDimensionCount = dimensionCount
	ft.pointIndexDimensionCount = indexDimensionCount
	ft.pointNumBytes = dimensionNumBytes
}

// PointDimensionCount returns the point dimension count.
func (ft *FieldType) PointDimensionCount() int {
	return ft.pointDimensionCount
}

// PointIndexDimensionCount returns the point index dimension count.
func (ft *FieldType) PointIndexDimensionCount() int {
	return ft.pointIndexDimensionCount
}

// PointNumBytes returns the number of bytes per point dimension.
func (ft *FieldType) PointNumBytes() int {
	return ft.pointNumBytes
}

// GetPointDimensionCount returns the point dimension count.
func (ft *FieldType) GetPointDimensionCount() int {
	return ft.pointDimensionCount
}

// GetPointIndexDimensionCount returns the point index dimension count.
func (ft *FieldType) GetPointIndexDimensionCount() int {
	return ft.pointIndexDimensionCount
}

// GetPointNumBytes returns the number of bytes per point dimension.
func (ft *FieldType) GetPointNumBytes() int {
	return ft.pointNumBytes
}

// DimensionNumBytes is an alias for PointNumBytes (for backward compatibility).
func (ft *FieldType) DimensionNumBytes() int {
	return ft.pointNumBytes
}

// SetVectorAttributes sets vector attributes.
func (ft *FieldType) SetVectorAttributes(vectorDimension int, vectorEncoding schema.VectorEncoding, vectorSimilarityFunction schema.VectorSimilarityFunction) {
	ft.checkIfFrozen()
	if vectorDimension <= 0 {
		panic(fmt.Sprintf("vectorDimension must be > 0; got %d", vectorDimension))
	}
	ft.VectorDimension = vectorDimension
	ft.VectorEncoding = vectorEncoding
	ft.VectorSimilarityFunction = vectorSimilarityFunction
}

// GetVectorDimension returns the vector dimension.
func (ft *FieldType) GetVectorDimension() int {
	return ft.VectorDimension
}

// GetVectorEncoding returns the vector encoding.
func (ft *FieldType) GetVectorEncoding() schema.VectorEncoding {
	return ft.VectorEncoding
}

// GetVectorSimilarityFunction returns the vector similarity function.
func (ft *FieldType) GetVectorSimilarityFunction() schema.VectorSimilarityFunction {
	return ft.VectorSimilarityFunction
}

// SetDocValuesSkipIndexType sets the doc values skip index type.
func (ft *FieldType) SetDocValuesSkipIndexType(value DocValuesSkipIndexType) {
	ft.checkIfFrozen()
	ft.DocValuesSkipIndex = value
}

// DocValuesSkipIndexType returns the doc values skip index type.
func (ft *FieldType) DocValuesSkipIndexType() DocValuesSkipIndexType {
	return ft.DocValuesSkipIndex
}

// PutAttribute stores an attribute key-value pair.
// Returns the previous value associated with key, or "" if key was not present.
func (ft *FieldType) PutAttribute(key, value string) string {
	ft.checkIfFrozen()
	prev := ft.attributes[key]
	ft.attributes[key] = value
	return prev
}

// GetAttributes returns a copy of the attributes map.
func (ft *FieldType) GetAttributes() map[string]string {
	result := make(map[string]string)
	for k, v := range ft.attributes {
		result[k] = v
	}
	return result
}

// Equals returns whether two FieldTypes are equal.
func (ft *FieldType) Equals(other *FieldType) bool {
	if other == nil {
		return false
	}
	return ft.Stored == other.Stored &&
		ft.Tokenized == other.Tokenized &&
		ft.StoreTermVectors == other.StoreTermVectors &&
		ft.StoreTermVectorOffsets == other.StoreTermVectorOffsets &&
		ft.StoreTermVectorPositions == other.StoreTermVectorPositions &&
		ft.StoreTermVectorPayloads == other.StoreTermVectorPayloads &&
		ft.OmitNorms == other.OmitNorms &&
		ft.IndexOptions == other.IndexOptions &&
		ft.DocValuesType == other.DocValuesType &&
		ft.PointDimensionCount() == other.PointDimensionCount() &&
		ft.PointIndexDimensionCount() == other.PointIndexDimensionCount() &&
		ft.PointNumBytes() == other.PointNumBytes() &&
		ft.VectorDimension == other.VectorDimension &&
		ft.VectorEncoding == other.VectorEncoding &&
		ft.VectorSimilarityFunction == other.VectorSimilarityFunction &&
		ft.DocValuesSkipIndex == other.DocValuesSkipIndex &&
		ft.attributesEqual(other.attributes)
}

// attributesEqual checks if two attribute maps are equal.
func (ft *FieldType) attributesEqual(other map[string]string) bool {
	if len(ft.attributes) != len(other) {
		return false
	}
	for k, v := range ft.attributes {
		if other[k] != v {
			return false
		}
	}
	return true
}

// Validate checks if the FieldType configuration is valid.
func (ft *FieldType) Validate() error {
	// If Indexed is true, IndexOptions must not be NONE
	if ft.Indexed && ft.IndexOptions == schema.IndexOptionsNone {
		return fmt.Errorf("if Indexed is true, IndexOptions must not be NONE")
	}
	// If Tokenized is true, field must be indexed
	if ft.Tokenized && !ft.Indexed {
		return fmt.Errorf("if Tokenized is true, field must be indexed")
	}
	return nil
}

// String returns a string representation of the FieldType.
func (ft *FieldType) String() string {
	var parts []string

	if ft.Stored {
		parts = append(parts, "stored")
	}
	if ft.IsIndexed() {
		parts = append(parts, "indexed")
		if ft.Tokenized {
			parts = append(parts, "tokenized")
		}
		parts = append(parts, "indexOptions="+ft.IndexOptions.String())
	}
	if ft.StoreTermVectors {
		parts = append(parts, "termVectors")
	}
	if ft.StoreTermVectorOffsets {
		parts = append(parts, "termVectorOffsets")
	}
	if ft.StoreTermVectorPositions {
		parts = append(parts, "termVectorPositions")
	}
	if ft.StoreTermVectorPayloads {
		parts = append(parts, "termVectorPayloads")
	}
	if ft.OmitNorms {
		parts = append(parts, "omitNorms")
	}
	if ft.DocValuesType != schema.DocValuesTypeNone {
		parts = append(parts, "docValuesType="+ft.DocValuesType.String())
	}
	if ft.PointDimensionCount() > 0 {
		parts = append(parts, fmt.Sprintf("pointDimensions=%d/%d/%d", ft.PointDimensionCount(), ft.PointIndexDimensionCount(), ft.PointNumBytes()))
	}
	if ft.VectorDimension > 0 {
		parts = append(parts, fmt.Sprintf("vectorDimension=%d", ft.VectorDimension))
	}

	return fmt.Sprintf("FieldType(%s)", strings.Join(parts, ", "))
}
