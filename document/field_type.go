// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FieldType implements spi.IndexableFieldType, mirroring Java's
// "public class FieldType implements IndexableFieldType".
var _ spi.IndexableFieldType = (*FieldType)(nil)

// DocValuesSkipIndexType defines options for skip indexes on doc values.
//
// Mirrors org.apache.lucene.index.DocValuesSkipIndexType from Apache Lucene
// 10.5.0. Java declares the enum once; Gocene declares it once in package spi
// (the leaf package both document and index can import) and aliases it here
// and in package index, so that every package-qualified spelling names the
// one type.
type DocValuesSkipIndexType = spi.DocValuesSkipIndexType

const (
	// DocValuesSkipIndexTypeNone: No skip index should be created.
	DocValuesSkipIndexTypeNone = spi.DocValuesSkipIndexTypeNone
	// DocValuesSkipIndexTypeRange: Record range of values.
	DocValuesSkipIndexTypeRange = spi.DocValuesSkipIndexTypeRange
)

// Re-export vector types from schema to avoid circular imports.
// Tests and other code may reference these through the document/index packages.
// Mirrors org.apache.lucene.index.VectorEncoding and VectorSimilarityFunction.

// VectorEncodingByte stores vector values as signed bytes.
const VectorEncodingByte = util.VectorEncodingByte

// VectorEncodingFloat32 stores vector values as IEEE 32-bit floating point.
const VectorEncodingFloat32 = util.VectorEncodingFloat32

// VectorSimilarityFunctionEuclidean uses squared Euclidean distance.
var VectorSimilarityFunctionEuclidean = util.EuclideanSim

// VectorSimilarityFunctionDotProduct uses dot product similarity.
var VectorSimilarityFunctionDotProduct = util.DotProductSim

// VectorSimilarityFunctionCosine uses cosine similarity.
var VectorSimilarityFunctionCosine = util.CosineSim

// VectorSimilarityFunctionMaximumInnerProduct uses maximum inner product similarity.
var VectorSimilarityFunctionMaximumInnerProduct = util.MaximumInnerProductSim

// FieldType describes the properties of a field.
//
// This is the Go port of Lucene's org.apache.lucene.document.FieldType.
type FieldType struct {
	// Public fields for backward compatibility
	stored                   bool
	tokenized                bool
	storeTermVectors         bool
	storeTermVectorOffsets   bool
	storeTermVectorPositions bool
	storeTermVectorPayloads  bool
	omitNorms                bool
	indexed                  bool // Mirrors whether IndexOptions != NONE
	indexOptions             spi.IndexOptions
	docValuesType            spi.DocValuesType
	vectorDimension          int
	vectorEncoding           spi.VectorEncoding
	vectorSimilarityFunction spi.VectorSimilarityFunction
	docValuesSkipIndex       DocValuesSkipIndexType

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
		stored:                   false,
		tokenized:                false,
		storeTermVectors:         false,
		storeTermVectorOffsets:   false,
		storeTermVectorPositions: false,
		storeTermVectorPayloads:  false,
		omitNorms:                false,
		indexOptions:             spi.IndexOptionsNone,
		docValuesType:            spi.DocValuesTypeNone,
		pointDimensionCount:      0,
		pointIndexDimensionCount: 0,
		pointNumBytes:            0,
		vectorDimension:          0,
		vectorEncoding:           util.VectorEncodingFloat32,
		vectorSimilarityFunction: util.EuclideanSim,
		docValuesSkipIndex:       DocValuesSkipIndexTypeNone,
		frozen:                   false,
		attributes:               make(map[string]string),
	}
}

// NewLuceneFieldType creates a FieldType with Lucene's defaults
// (Tokenized=true instead of false).
func NewLuceneFieldType() *FieldType {
	ft := NewFieldType()
	ft.tokenized = true
	return ft
}

// NewFieldTypeFrom creates a new mutable FieldType with all properties from src.
// The frozen state is not copied.
func NewFieldTypeFrom(src *FieldType) *FieldType {
	ft := &FieldType{
		stored:                   src.stored,
		tokenized:                src.tokenized,
		storeTermVectors:         src.storeTermVectors,
		storeTermVectorOffsets:   src.storeTermVectorOffsets,
		storeTermVectorPositions: src.storeTermVectorPositions,
		storeTermVectorPayloads:  src.storeTermVectorPayloads,
		omitNorms:                src.omitNorms,
		indexOptions:             src.indexOptions,
		docValuesType:            src.docValuesType,
		pointDimensionCount:      src.pointDimensionCount,
		pointIndexDimensionCount: src.pointIndexDimensionCount,
		pointNumBytes:            src.pointNumBytes,
		vectorDimension:          src.vectorDimension,
		vectorEncoding:           src.vectorEncoding,
		vectorSimilarityFunction: src.vectorSimilarityFunction,
		docValuesSkipIndex:       src.docValuesSkipIndex,
		frozen:                   false,
		attributes:               make(map[string]string),
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
	ft.stored = value
	return ft
}

// Stored returns whether this field is stored.
func (ft *FieldType) Stored() bool {
	return ft.stored
}

// SetTokenized sets whether this field is tokenized.
func (ft *FieldType) SetTokenized(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.tokenized = value
	return ft
}

// Tokenized returns whether this field is tokenized.
func (ft *FieldType) Tokenized() bool {
	return ft.tokenized
}

// SetStoreTermVectors sets whether term vectors should be stored.
func (ft *FieldType) SetStoreTermVectors(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.storeTermVectors = value
	return ft
}

// StoreTermVectors returns whether term vectors are stored.
func (ft *FieldType) StoreTermVectors() bool {
	return ft.storeTermVectors
}

// SetStoreTermVectorOffsets sets whether term vector offsets should be stored.
func (ft *FieldType) SetStoreTermVectorOffsets(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.storeTermVectorOffsets = value
	return ft
}

// StoreTermVectorOffsets returns whether term vector offsets are stored.
func (ft *FieldType) StoreTermVectorOffsets() bool {
	return ft.storeTermVectorOffsets
}

// SetStoreTermVectorPositions sets whether term vector positions should be stored.
func (ft *FieldType) SetStoreTermVectorPositions(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.storeTermVectorPositions = value
	return ft
}

// StoreTermVectorPositions returns whether term vector positions are stored.
func (ft *FieldType) StoreTermVectorPositions() bool {
	return ft.storeTermVectorPositions
}

// SetStoreTermVectorPayloads sets whether term vector payloads should be stored.
func (ft *FieldType) SetStoreTermVectorPayloads(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.storeTermVectorPayloads = value
	return ft
}

// StoreTermVectorPayloads returns whether term vector payloads are stored.
func (ft *FieldType) StoreTermVectorPayloads() bool {
	return ft.storeTermVectorPayloads
}

// SetOmitNorms sets whether norms should be omitted.
func (ft *FieldType) SetOmitNorms(value bool) *FieldType {
	ft.checkIfFrozen()
	ft.omitNorms = value
	return ft
}

// OmitNorms returns whether norms are omitted.
func (ft *FieldType) OmitNorms() bool {
	return ft.omitNorms
}

// SetIndexOptions sets the indexing options.
func (ft *FieldType) SetIndexOptions(value spi.IndexOptions) *FieldType {
	ft.checkIfFrozen()
	ft.indexOptions = value
	ft.indexed = (value != spi.IndexOptionsNone)
	return ft
}

// IndexOptions returns the indexing options.
func (ft *FieldType) IndexOptions() spi.IndexOptions {
	return ft.indexOptions
}

// SetIndexed is a convenience method that sets IndexOptions based on whether indexed is true/false.
func (ft *FieldType) SetIndexed(indexed bool) *FieldType {
	ft.checkIfFrozen()
	ft.indexed = indexed
	if indexed {
		ft.indexOptions = spi.IndexOptionsDocsAndFreqsAndPositions
	} else {
		ft.indexOptions = spi.IndexOptionsNone
	}
	return ft
}

// IsIndexed returns whether this field is indexed.
func (ft *FieldType) IsIndexed() bool {
	return ft.indexOptions != spi.IndexOptionsNone
}

// SetDocValuesType sets the doc values type.
func (ft *FieldType) SetDocValuesType(value spi.DocValuesType) *FieldType {
	ft.checkIfFrozen()
	ft.docValuesType = value
	return ft
}

// DocValuesType returns the doc values type.
func (ft *FieldType) DocValuesType() spi.DocValuesType {
	return ft.docValuesType
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

// SetVectorAttributes sets vector attributes.
func (ft *FieldType) SetVectorAttributes(vectorDimension int, vectorEncoding spi.VectorEncoding, vectorSimilarityFunction spi.VectorSimilarityFunction) {
	ft.checkIfFrozen()
	if vectorDimension <= 0 {
		panic(fmt.Sprintf("vectorDimension must be > 0; got %d", vectorDimension))
	}
	ft.vectorDimension = vectorDimension
	ft.vectorEncoding = vectorEncoding
	ft.vectorSimilarityFunction = vectorSimilarityFunction
}

// VectorDimension returns the vector dimension.
func (ft *FieldType) VectorDimension() int {
	return ft.vectorDimension
}

// VectorEncoding returns the vector encoding.
func (ft *FieldType) VectorEncoding() spi.VectorEncoding {
	return ft.vectorEncoding
}

// VectorSimilarityFunction returns the vector similarity function.
func (ft *FieldType) VectorSimilarityFunction() spi.VectorSimilarityFunction {
	return ft.vectorSimilarityFunction
}

// SetDocValuesSkipIndexType sets the doc values skip index type.
func (ft *FieldType) SetDocValuesSkipIndexType(value DocValuesSkipIndexType) *FieldType {
	ft.checkIfFrozen()
	ft.docValuesSkipIndex = value
	return ft
}

// DocValuesSkipIndexType returns the doc values skip index type.
func (ft *FieldType) DocValuesSkipIndexType() DocValuesSkipIndexType {
	return ft.docValuesSkipIndex
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
	return ft.stored == other.stored &&
		ft.tokenized == other.tokenized &&
		ft.storeTermVectors == other.storeTermVectors &&
		ft.storeTermVectorOffsets == other.storeTermVectorOffsets &&
		ft.storeTermVectorPositions == other.storeTermVectorPositions &&
		ft.storeTermVectorPayloads == other.storeTermVectorPayloads &&
		ft.omitNorms == other.omitNorms &&
		ft.indexOptions == other.indexOptions &&
		ft.docValuesType == other.docValuesType &&
		ft.PointDimensionCount() == other.PointDimensionCount() &&
		ft.PointIndexDimensionCount() == other.PointIndexDimensionCount() &&
		ft.PointNumBytes() == other.PointNumBytes() &&
		ft.vectorDimension == other.vectorDimension &&
		ft.vectorEncoding == other.vectorEncoding &&
		ft.vectorSimilarityFunction == other.vectorSimilarityFunction &&
		ft.docValuesSkipIndex == other.docValuesSkipIndex &&
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
	if ft.indexed && ft.indexOptions == spi.IndexOptionsNone {
		return fmt.Errorf("if Indexed is true, IndexOptions must not be NONE")
	}
	// If Tokenized is true, field must be indexed
	if ft.tokenized && !ft.indexed {
		return fmt.Errorf("if Tokenized is true, field must be indexed")
	}
	return nil
}

// String returns a string representation of the FieldType.
func (ft *FieldType) String() string {
	var parts []string

	if ft.stored {
		parts = append(parts, "stored")
	}
	if ft.IsIndexed() {
		parts = append(parts, "indexed")
		if ft.tokenized {
			parts = append(parts, "tokenized")
		}
		parts = append(parts, "indexOptions="+ft.indexOptions.String())
	}
	if ft.storeTermVectors {
		parts = append(parts, "termVectors")
	}
	if ft.storeTermVectorOffsets {
		parts = append(parts, "termVectorOffsets")
	}
	if ft.storeTermVectorPositions {
		parts = append(parts, "termVectorPositions")
	}
	if ft.storeTermVectorPayloads {
		parts = append(parts, "termVectorPayloads")
	}
	if ft.omitNorms {
		parts = append(parts, "omitNorms")
	}
	if ft.docValuesType != spi.DocValuesTypeNone {
		parts = append(parts, "docValuesType="+ft.docValuesType.String())
	}
	if ft.PointDimensionCount() > 0 {
		parts = append(parts, fmt.Sprintf("pointDimensions=%d/%d/%d", ft.PointDimensionCount(), ft.PointIndexDimensionCount(), ft.PointNumBytes()))
	}
	if ft.vectorDimension > 0 {
		parts = append(parts, fmt.Sprintf("vectorDimension=%d", ft.vectorDimension))
	}

	return fmt.Sprintf("FieldType(%s)", strings.Join(parts, ", "))
}
