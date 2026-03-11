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
	Stored                   bool
	Tokenized                bool
	OmitNorms                bool
	StoreTermVectors         bool
	StoreTermVectorPositions bool
	StoreTermVectorOffsets   bool
	StoreTermVectorPayloads  bool
}

// DefaultFieldInfoOptions returns default FieldInfoOptions.
func DefaultFieldInfoOptions() FieldInfoOptions {
	return FieldInfoOptions{
		IndexOptions:  IndexOptionsNone,
		DocValuesType: DocValuesTypeNone,
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
		stored:                   opts.Stored,
		tokenized:                opts.Tokenized,
		omitNorms:                opts.OmitNorms,
		storeTermVectors:         opts.StoreTermVectors,
		storeTermVectorPositions: opts.StoreTermVectorPositions,
		storeTermVectorOffsets:   opts.StoreTermVectorOffsets,
		storeTermVectorPayloads:  opts.StoreTermVectorPayloads,
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
		Stored:                   fi.stored,
		Tokenized:                fi.tokenized,
		OmitNorms:                fi.omitNorms,
		StoreTermVectors:         fi.storeTermVectors,
		StoreTermVectorPositions: fi.storeTermVectorPositions,
		StoreTermVectorOffsets:   fi.storeTermVectorOffsets,
		StoreTermVectorPayloads:  fi.storeTermVectorPayloads,
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
		stored:                   b.opts.Stored,
		tokenized:                b.opts.Tokenized,
		omitNorms:                b.opts.OmitNorms,
		storeTermVectors:         b.opts.StoreTermVectors,
		storeTermVectorPositions: b.opts.StoreTermVectorPositions,
		storeTermVectorOffsets:   b.opts.StoreTermVectorOffsets,
		storeTermVectorPayloads:  b.opts.StoreTermVectorPayloads,
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
