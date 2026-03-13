// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// FieldInfo stores metadata about a field.
// This is immutable after construction and is the Go port of
// Lucene's org.apache.lucene.index.FieldInfo.
//
// FieldInfo contains the indexing options for a field and
// any per-field custom attributes.
type FieldInfo struct {
	// Name is the field name
	name string

	// Number is the field number (position in FieldInfos)
	number int

	// IndexOptions controls how the field is indexed
	indexOptions IndexOptions

	// DocValuesType is the type of doc values, if any
	docValuesType DocValuesType

	// docValuesSkipIndexType is the type of doc values skip index
	docValuesSkipIndexType DocValuesSkipIndexType

	// docValuesGen is the generation count of the field's DocValues
	docValuesGen int64

	// Stored determines if the field value is stored
	stored bool

	// Tokenized determines if the field is tokenized
	tokenized bool

	// OmitNorms determines if norms are omitted
	omitNorms bool

	// StoreTermVectors determines if term vectors are stored
	storeTermVectors bool

	// StoreTermVectorPositions determines if term vector positions are stored
	storeTermVectorPositions bool

	// StoreTermVectorOffsets determines if term vector offsets are stored
	storeTermVectorOffsets bool

	// StoreTermVectorPayloads determines if term vector payloads are stored
	storeTermVectorPayloads bool

	// pointDimensionCount is the number of dimensions for point values
	pointDimensionCount int

	// pointIndexDimensionCount is the number of index dimensions for point values
	pointIndexDimensionCount int

	// pointNumBytes is the number of bytes per dimension for point values
	pointNumBytes int

	// vectorDimension is the number of dimensions for vector values
	vectorDimension int

	// vectorEncoding is the encoding of vector values
	vectorEncoding VectorEncoding

	// vectorSimilarityFunction is the similarity function for vector values
	vectorSimilarityFunction VectorSimilarityFunction

	// isSoftDeletesField is true if this field is used for soft deletes
	isSoftDeletesField bool

	// isParentField is true if this field is used for parent-child relationship
	isParentField bool

	// Attributes holds custom per-field attributes
	attributes map[string]string

	// frozen is set to true after construction to prevent modification
	frozen bool

	// mu protects attributes during construction
	mu sync.RWMutex
}

// FieldInfoOptions holds the options for creating a FieldInfo.
type FieldInfoOptions struct {
	IndexOptions             IndexOptions
	DocValuesType            DocValuesType
	DocValuesSkipIndexType   DocValuesSkipIndexType
	DocValuesGen             int64
	Stored                   bool
	Tokenized                bool
	OmitNorms                bool
	StoreTermVectors         bool
	StoreTermVectorPositions bool
	StoreTermVectorOffsets   bool
	StoreTermVectorPayloads  bool
	PointDimensionCount      int
	PointIndexDimensionCount int
	PointNumBytes            int
	VectorDimension          int
	VectorEncoding           VectorEncoding
	VectorSimilarityFunction VectorSimilarityFunction
	IsSoftDeletesField       bool
	IsParentField            bool
}

// DefaultFieldInfoOptions returns default FieldInfoOptions.
func DefaultFieldInfoOptions() FieldInfoOptions {
	return FieldInfoOptions{
		IndexOptions:             IndexOptionsNone,
		DocValuesType:            DocValuesTypeNone,
		DocValuesSkipIndexType:   DocValuesSkipIndexTypeNone,
		DocValuesGen:             -1,
		VectorEncoding:           VectorEncodingFloat32,
		VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
	}
}

// NewFieldInfo creates a new FieldInfo.
// After creation, the FieldInfo is immutable.
func NewFieldInfo(name string, number int, opts FieldInfoOptions) *FieldInfo {
	fi := &FieldInfo{
		name:                     name,
		number:                   number,
		indexOptions:             opts.IndexOptions,
		docValuesType:            opts.DocValuesType,
		docValuesSkipIndexType:   opts.DocValuesSkipIndexType,
		docValuesGen:             opts.DocValuesGen,
		stored:                   opts.Stored,
		tokenized:                opts.Tokenized,
		omitNorms:                opts.OmitNorms,
		storeTermVectors:         opts.StoreTermVectors,
		storeTermVectorPositions: opts.StoreTermVectorPositions,
		storeTermVectorOffsets:   opts.StoreTermVectorOffsets,
		storeTermVectorPayloads:  opts.StoreTermVectorPayloads,
		pointDimensionCount:      opts.PointDimensionCount,
		pointIndexDimensionCount: opts.PointIndexDimensionCount,
		pointNumBytes:            opts.PointNumBytes,
		vectorDimension:          opts.VectorDimension,
		vectorEncoding:           opts.VectorEncoding,
		vectorSimilarityFunction: opts.VectorSimilarityFunction,
		isSoftDeletesField:       opts.IsSoftDeletesField,
		isParentField:            opts.IsParentField,
		attributes:               make(map[string]string),
		frozen:                   false,
	}

	// Validate that term vector components require term vectors
	if fi.storeTermVectorPositions && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}
	if fi.storeTermVectorOffsets && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}
	if fi.storeTermVectorPayloads && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}

	// Validate that tokenized requires indexing
	if fi.tokenized && !fi.indexOptions.IsIndexed() {
		fi.tokenized = false
	}

	fi.frozen = true
	return fi
}

// Name returns the field name.
func (fi *FieldInfo) Name() string {
	return fi.name
}

// Number returns the field number.
func (fi *FieldInfo) Number() int {
	return fi.number
}

// IndexOptions returns the index options.
func (fi *FieldInfo) IndexOptions() IndexOptions {
	return fi.indexOptions
}

// DocValuesType returns the doc values type.
func (fi *FieldInfo) DocValuesType() DocValuesType {
	return fi.docValuesType
}

// DocValuesSkipIndexType returns the doc values skip index type.
func (fi *FieldInfo) DocValuesSkipIndexType() DocValuesSkipIndexType {
	return fi.docValuesSkipIndexType
}

// DocValuesGen returns the doc values generation count.
func (fi *FieldInfo) DocValuesGen() int64 {
	return fi.docValuesGen
}

// IsStored returns true if the field value is stored.
func (fi *FieldInfo) IsStored() bool {
	return fi.stored
}

// IsTokenized returns true if the field is tokenized.
func (fi *FieldInfo) IsTokenized() bool {
	return fi.tokenized
}

// OmitNorms returns true if norms are omitted.
func (fi *FieldInfo) OmitNorms() bool {
	return fi.omitNorms
}

// StoreTermVectors returns true if term vectors are stored.
func (fi *FieldInfo) StoreTermVectors() bool {
	return fi.storeTermVectors
}

// StoreTermVectorPositions returns true if term vector positions are stored.
func (fi *FieldInfo) StoreTermVectorPositions() bool {
	return fi.storeTermVectorPositions
}

// StoreTermVectorOffsets returns true if term vector offsets are stored.
func (fi *FieldInfo) StoreTermVectorOffsets() bool {
	return fi.storeTermVectorOffsets
}

// StoreTermVectorPayloads returns true if term vector payloads are stored.
func (fi *FieldInfo) StoreTermVectorPayloads() bool {
	return fi.storeTermVectorPayloads
}

// PointDimensionCount returns the number of dimensions for point values.
func (fi *FieldInfo) PointDimensionCount() int {
	return fi.pointDimensionCount
}

// PointIndexDimensionCount returns the number of index dimensions for point values.
func (fi *FieldInfo) PointIndexDimensionCount() int {
	return fi.pointIndexDimensionCount
}

// PointNumBytes returns the number of bytes per dimension for point values.
func (fi *FieldInfo) PointNumBytes() int {
	return fi.pointNumBytes
}

// VectorDimension returns the number of dimensions for vector values.
func (fi *FieldInfo) VectorDimension() int {
	return fi.vectorDimension
}

// VectorEncoding returns the encoding of vector values.
func (fi *FieldInfo) VectorEncoding() VectorEncoding {
	return fi.vectorEncoding
}

// VectorSimilarityFunction returns the similarity function for vector values.
func (fi *FieldInfo) VectorSimilarityFunction() VectorSimilarityFunction {
	return fi.vectorSimilarityFunction
}

// IsSoftDeletesField returns true if this field is used for soft deletes.
func (fi *FieldInfo) IsSoftDeletesField() bool {
	return fi.isSoftDeletesField
}

// IsParentField returns true if this field is used for parent-child relationship.
func (fi *FieldInfo) IsParentField() bool {
	return fi.isParentField
}

// HasNorms returns true if the field has norms (for scoring).
func (fi *FieldInfo) HasNorms() bool {
	return fi.indexOptions.IsIndexed() && !fi.omitNorms &&
		fi.indexOptions.HasFreqs()
}

// HasPayloads returns true if payloads are indexed.
func (fi *FieldInfo) HasPayloads() bool {
	return fi.indexOptions.HasPositions()
}

// HasTermVectors returns true if term vectors are stored.
func (fi *FieldInfo) HasTermVectors() bool {
	return fi.storeTermVectors
}

// GetAttribute returns a custom attribute value.
// Returns empty string if the attribute is not set.
func (fi *FieldInfo) GetAttribute(key string) string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.attributes[key]
}

// GetAttributes returns a copy of all custom attributes.
func (fi *FieldInfo) GetAttributes() map[string]string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	copy := make(map[string]string, len(fi.attributes))
	for k, v := range fi.attributes {
		copy[k] = v
	}
	return copy
}

// PutAttribute sets a custom attribute value.
// Panics if the FieldInfo is frozen (should only be called during construction).
func (fi *FieldInfo) PutAttribute(key, value string) {
	if fi.frozen {
		panic("FieldInfo is immutable")
	}
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.attributes[key] = value
}

// Clone creates a copy of this FieldInfo with a new number.
func (fi *FieldInfo) Clone(newNumber int) *FieldInfo {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	opts := FieldInfoOptions{
		IndexOptions:             fi.indexOptions,
		DocValuesType:            fi.docValuesType,
		DocValuesSkipIndexType:   fi.docValuesSkipIndexType,
		DocValuesGen:             fi.docValuesGen,
		Stored:                   fi.stored,
		Tokenized:                fi.tokenized,
		OmitNorms:                fi.omitNorms,
		StoreTermVectors:         fi.storeTermVectors,
		StoreTermVectorPositions: fi.storeTermVectorPositions,
		StoreTermVectorOffsets:   fi.storeTermVectorOffsets,
		StoreTermVectorPayloads:  fi.storeTermVectorPayloads,
		PointDimensionCount:      fi.pointDimensionCount,
		PointIndexDimensionCount: fi.pointIndexDimensionCount,
		PointNumBytes:            fi.pointNumBytes,
		VectorDimension:          fi.vectorDimension,
		VectorEncoding:           fi.vectorEncoding,
		VectorSimilarityFunction: fi.vectorSimilarityFunction,
		IsSoftDeletesField:       fi.isSoftDeletesField,
		IsParentField:            fi.isParentField,
	}

	clone := NewFieldInfo(fi.name, newNumber, opts)

	// Copy attributes - need to temporarily unfreeze
	clone.mu.Lock()
	clone.frozen = false
	for k, v := range fi.attributes {
		clone.attributes[k] = v
	}
	clone.frozen = true
	clone.mu.Unlock()

	return clone
}

// CheckConsistency verifies that the FieldInfo is valid.
func (fi *FieldInfo) CheckConsistency() error {
	if fi.name == "" {
		return fmt.Errorf("field name cannot be empty")
	}
	if fi.number < 0 {
		return fmt.Errorf("field number cannot be negative")
	}

	// If indexed, index options must be valid
	if fi.indexOptions.IsIndexed() && fi.indexOptions == IndexOptionsNone {
		return fmt.Errorf("indexed field cannot have IndexOptionsNone")
	}

	// Tokenized requires indexing
	if fi.tokenized && !fi.indexOptions.IsIndexed() {
		return fmt.Errorf("tokenized field must be indexed")
	}

	// Term vector components require term vectors
	if fi.storeTermVectorPositions && !fi.storeTermVectors {
		return fmt.Errorf("cannot store term vector positions without storing term vectors")
	}
	if fi.storeTermVectorOffsets && !fi.storeTermVectors {
		return fmt.Errorf("cannot store term vector offsets without storing term vectors")
	}
	if fi.storeTermVectorPayloads && !fi.storeTermVectors {
		return fmt.Errorf("cannot store term vector payloads without storing term vectors")
	}

	// Payloads require positions
	if fi.storeTermVectorPayloads && !fi.storeTermVectorPositions {
		return fmt.Errorf("cannot store term vector payloads without storing term vector positions")
	}

	return nil
}

// String returns a string representation of FieldInfo.
func (fi *FieldInfo) String() string {
	return fmt.Sprintf("FieldInfo(name=%s, number=%d, indexed=%v, stored=%v, tokenized=%v)",
		fi.name, fi.number, fi.indexOptions.IsIndexed(), fi.stored, fi.tokenized)
}

// FieldInfoBuilder helps construct FieldInfo with a fluent API.
type FieldInfoBuilder struct {
	name   string
	number int
	opts   FieldInfoOptions
	attrs  map[string]string
}

// NewFieldInfoBuilder creates a new FieldInfoBuilder.
func NewFieldInfoBuilder(name string, number int) *FieldInfoBuilder {
	return &FieldInfoBuilder{
		name:   name,
		number: number,
		opts:   DefaultFieldInfoOptions(),
		attrs:  make(map[string]string),
	}
}

// SetIndexOptions sets the index options.
func (b *FieldInfoBuilder) SetIndexOptions(opts IndexOptions) *FieldInfoBuilder {
	b.opts.IndexOptions = opts
	return b
}

// SetDocValuesType sets the doc values type.
func (b *FieldInfoBuilder) SetDocValuesType(dvt DocValuesType) *FieldInfoBuilder {
	b.opts.DocValuesType = dvt
	return b
}

// SetDocValuesSkipIndexType sets the doc values skip index type.
func (b *FieldInfoBuilder) SetDocValuesSkipIndexType(dvst DocValuesSkipIndexType) *FieldInfoBuilder {
	b.opts.DocValuesSkipIndexType = dvst
	return b
}

// SetDocValuesGen sets the doc values generation count.
func (b *FieldInfoBuilder) SetDocValuesGen(gen int64) *FieldInfoBuilder {
	b.opts.DocValuesGen = gen
	return b
}

// SetStored sets whether the field is stored.
func (b *FieldInfoBuilder) SetStored(stored bool) *FieldInfoBuilder {
	b.opts.Stored = stored
	return b
}

// SetTokenized sets whether the field is tokenized.
func (b *FieldInfoBuilder) SetTokenized(tokenized bool) *FieldInfoBuilder {
	b.opts.Tokenized = tokenized
	return b
}

// SetOmitNorms sets whether norms are omitted.
func (b *FieldInfoBuilder) SetOmitNorms(omit bool) *FieldInfoBuilder {
	b.opts.OmitNorms = omit
	return b
}

// SetStoreTermVectors sets whether term vectors are stored.
func (b *FieldInfoBuilder) SetStoreTermVectors(store bool) *FieldInfoBuilder {
	b.opts.StoreTermVectors = store
	return b
}

// SetStoreTermVectorPositions sets whether term vector positions are stored.
func (b *FieldInfoBuilder) SetStoreTermVectorPositions(store bool) *FieldInfoBuilder {
	b.opts.StoreTermVectorPositions = store
	return b
}

// SetStoreTermVectorOffsets sets whether term vector offsets are stored.
func (b *FieldInfoBuilder) SetStoreTermVectorOffsets(store bool) *FieldInfoBuilder {
	b.opts.StoreTermVectorOffsets = store
	return b
}

// SetStoreTermVectorPayloads sets whether term vector payloads are stored.
func (b *FieldInfoBuilder) SetStoreTermVectorPayloads(store bool) *FieldInfoBuilder {
	b.opts.StoreTermVectorPayloads = store
	return b
}

// SetPointDimensions sets the point dimension information.
func (b *FieldInfoBuilder) SetPointDimensions(dataCount, indexCount, numBytes int) *FieldInfoBuilder {
	b.opts.PointDimensionCount = dataCount
	b.opts.PointIndexDimensionCount = indexCount
	b.opts.PointNumBytes = numBytes
	return b
}

// SetVectorAttributes sets the vector attribute information.
func (b *FieldInfoBuilder) SetVectorAttributes(dimension int, encoding VectorEncoding, similarity VectorSimilarityFunction) *FieldInfoBuilder {
	b.opts.VectorDimension = dimension
	b.opts.VectorEncoding = encoding
	b.opts.VectorSimilarityFunction = similarity
	return b
}

// SetSoftDeletesField sets whether this field is used for soft deletes.
func (b *FieldInfoBuilder) SetSoftDeletesField(isSoftDeletes bool) *FieldInfoBuilder {
	b.opts.IsSoftDeletesField = isSoftDeletes
	return b
}

// SetParentField sets whether this field is used for parent-child relationship.
func (b *FieldInfoBuilder) SetParentField(isParent bool) *FieldInfoBuilder {
	b.opts.IsParentField = isParent
	return b
}

// SetAttribute sets a custom attribute.
func (b *FieldInfoBuilder) SetAttribute(key, value string) *FieldInfoBuilder {
	b.attrs[key] = value
	return b
}

// Build creates the FieldInfo.
func (b *FieldInfoBuilder) Build() *FieldInfo {
	fi := &FieldInfo{
		name:                     b.name,
		number:                   b.number,
		indexOptions:             b.opts.IndexOptions,
		docValuesType:            b.opts.DocValuesType,
		docValuesSkipIndexType:   b.opts.DocValuesSkipIndexType,
		docValuesGen:             b.opts.DocValuesGen,
		stored:                   b.opts.Stored,
		tokenized:                b.opts.Tokenized,
		omitNorms:                b.opts.OmitNorms,
		storeTermVectors:         b.opts.StoreTermVectors,
		storeTermVectorPositions: b.opts.StoreTermVectorPositions,
		storeTermVectorOffsets:   b.opts.StoreTermVectorOffsets,
		storeTermVectorPayloads:  b.opts.StoreTermVectorPayloads,
		pointDimensionCount:      b.opts.PointDimensionCount,
		pointIndexDimensionCount: b.opts.PointIndexDimensionCount,
		pointNumBytes:            b.opts.PointNumBytes,
		vectorDimension:          b.opts.VectorDimension,
		vectorEncoding:           b.opts.VectorEncoding,
		vectorSimilarityFunction: b.opts.VectorSimilarityFunction,
		isSoftDeletesField:       b.opts.IsSoftDeletesField,
		isParentField:            b.opts.IsParentField,
		attributes:               make(map[string]string),
		frozen:                   false,
	}

	// Validate auto-enable term vectors
	if fi.storeTermVectorPositions && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}
	if fi.storeTermVectorOffsets && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}
	if fi.storeTermVectorPayloads && !fi.storeTermVectors {
		fi.storeTermVectors = true
	}

	// Validate tokenized requires indexing
	if fi.tokenized && !fi.indexOptions.IsIndexed() {
		fi.tokenized = false
	}

	// Set attributes before freezing
	for k, v := range b.attrs {
		fi.attributes[k] = v
	}

	fi.frozen = true
	return fi
}
