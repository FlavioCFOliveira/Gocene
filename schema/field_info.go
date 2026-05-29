// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

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

	// storePayloads records whether term-position payloads were observed
	// during indexing for this field. Mirrors Lucene FieldInfo.storePayloads
	// and is toggled by SetStorePayloads at flush time when FreqProx sees
	// the first non-empty payload.
	storePayloads bool

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

// HasPayloads returns true if payloads are stored for this field.
//
// Mirrors Java FieldInfo.hasPayloads(): returns the explicit storePayloads
// flag, which is set either at index time (via SetStorePayloads when the
// postings writer observes a non-empty payload) or restored from the wire
// format on read. A field that is indexed with positions but has never had
// a real payload returns false.
func (fi *FieldInfo) HasPayloads() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.storePayloads
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

// PutCodecAttribute sets a codec-owned attribute value, bypassing the frozen check.
//
// Lucene's FieldInfo is immutable after construction for regular attributes, but
// codec writers (notably PerFieldPostingsFormat, PerFieldDocValuesFormat, and
// PerFieldKnnVectorsFormat) need to record the chosen format-name and integer
// suffix on each FieldInfo at write time. This method is the Go analogue of
// FieldInfo.putAttribute in Java, which carries no frozen contract.
//
// Use PutCodecAttribute only from codec writers when recording per-field codec
// metadata. Application code must continue to use PutAttribute, which enforces
// immutability and panics on frozen FieldInfos.
func (fi *FieldInfo) PutCodecAttribute(key, value string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.attributes[key] = value
}

// SetStorePayloads marks this field as having observed payloads during
// indexing. Lucene's FreqProxTermsWriterPerField calls this from its
// finish() hook whenever the field saw at least one non-empty payload in
// the current segment, so the flushed FieldInfo can advertise hasPayloads
// in the index.
//
// Like Lucene's package-private setStorePayloads, this method bypasses the
// frozen contract. The flag is only honoured when the field is indexed with
// positions; for lower index options the call is a no-op, matching the
// guard in Lucene FieldInfo.setStorePayloads.
//
// Concurrent calls are serialised on the attribute mutex; reads of
// HasStoredPayloads use the same lock.
func (fi *FieldInfo) SetStorePayloads() {
	if fi.indexOptions < IndexOptionsDocsAndFreqsAndPositions {
		return
	}
	fi.mu.Lock()
	fi.storePayloads = true
	fi.mu.Unlock()
}

// HasStoredPayloads reports whether SetStorePayloads has been invoked for
// this field. Codec writers consult it to decide whether to encode the
// payload-bit in the postings stream. Existing callers should keep using
// HasPayloads(), which mirrors the older positions-based heuristic.
func (fi *FieldInfo) HasStoredPayloads() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.storePayloads
}

// OverrideStoreTermVectors forces the storeTermVectors flag to the
// given value, bypassing the auto-correction performed by NewFieldInfo
// and FieldInfoBuilder.Build. It exists as an escape hatch for tests
// that need to recreate inconsistent on-disk states; production code
// must use SetStoreTermVectors (which only sets the flag to true).
func (fi *FieldInfo) OverrideStoreTermVectors(v bool) {
	fi.mu.Lock()
	fi.storeTermVectors = v
	fi.mu.Unlock()
}

// OverrideStoreTermVectorOffsets is the offsets analogue of
// OverrideStoreTermVectors. Same caveats apply: tests only.
func (fi *FieldInfo) OverrideStoreTermVectorOffsets(v bool) {
	fi.mu.Lock()
	fi.storeTermVectorOffsets = v
	fi.mu.Unlock()
}

// SetStoreTermVectors marks this field as having stored term vectors.
// Lucene's TermVectorsConsumerPerField calls the package-private
// FieldInfo.setStoreTermVectors() from its finishDocument() hook once it
// has flushed the document's vectors, so the persisted FieldInfo can
// advertise hasVectors in the index.
//
// Like Lucene's package-private setStoreTermVectors, this method bypasses
// the frozen contract; it is the term-vectors-side analogue of
// SetStorePayloads. Concurrent calls are serialised on the attribute mutex,
// matching SetStorePayloads.
func (fi *FieldInfo) SetStoreTermVectors() {
	fi.mu.Lock()
	fi.storeTermVectors = true
	fi.mu.Unlock()
}

// VerifyAndUpdate accumulates the schema carried by opts into this FieldInfo,
// mirroring the per-field accumulation that Apache Lucene 10.4.0 performs in
// IndexingChain.FieldSchema (setIndexOptions / setDocValues / setPoints /
// setVectors) before a FieldInfo reaches FieldInfos.Builder.add.
//
// The accumulation rule is, per attribute group: if this FieldInfo has not yet
// recorded the attribute (the group is at its NONE / zero default) the value
// from opts is adopted; if the attribute is already set, opts is only allowed
// to repeat the same value (NONE / zero in opts never clears an already-set
// value, and a conflicting non-default value is reported as an error). This is
// exactly what lets a single field name carry BOTH an indexed contribution
// (e.g. StringField) and a DocValues contribution (e.g. SortedDocValuesField):
// whichever IndexableField is processed first sets its group, and the later
// one fills the remaining group instead of overwriting the first with its own
// NONE default (the dual-purpose-field bug, rmp #4780).
//
// Like SetStorePayloads / SetStoreTermVectors, this method bypasses the frozen
// contract because the indexing chain mutates the in-RAM FieldInfo across the
// fields of a document. Concurrent calls are serialised on the attribute mutex.
//
// Boolean store flags (Stored, OmitNorms, term-vector flags) are OR-accumulated:
// a field observed as stored or with norms in any contribution keeps that flag.
func (fi *FieldInfo) VerifyAndUpdate(opts FieldInfoOptions) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	// Index options (and the index-coupled omitNorms / storeTermVector flags,
	// which Lucene only adopts alongside a NONE->set index-options transition).
	if opts.IndexOptions != IndexOptionsNone {
		if fi.indexOptions == IndexOptionsNone {
			fi.indexOptions = opts.IndexOptions
			fi.omitNorms = opts.OmitNorms
			fi.tokenized = opts.Tokenized
		} else if fi.indexOptions != opts.IndexOptions {
			return fmt.Errorf("inconsistent index options for field %q: have %s, got %s",
				fi.name, fi.indexOptions, opts.IndexOptions)
		}
	}

	// Doc values type (and its skip-index companion).
	if opts.DocValuesType != DocValuesTypeNone {
		if fi.docValuesType == DocValuesTypeNone {
			fi.docValuesType = opts.DocValuesType
			fi.docValuesSkipIndexType = opts.DocValuesSkipIndexType
		} else if fi.docValuesType != opts.DocValuesType {
			return fmt.Errorf("inconsistent doc values type for field %q: have %s, got %s",
				fi.name, fi.docValuesType, opts.DocValuesType)
		}
	}

	// Point dimensions. Lucene guards on pointIndexDimensionCount == 0.
	if opts.PointDimensionCount > 0 {
		if fi.pointIndexDimensionCount == 0 {
			fi.pointDimensionCount = opts.PointDimensionCount
			fi.pointIndexDimensionCount = opts.PointIndexDimensionCount
			if fi.pointIndexDimensionCount == 0 {
				fi.pointIndexDimensionCount = opts.PointDimensionCount
			}
			fi.pointNumBytes = opts.PointNumBytes
		} else if fi.pointDimensionCount != opts.PointDimensionCount ||
			fi.pointNumBytes != opts.PointNumBytes {
			return fmt.Errorf("inconsistent point dimensions for field %q", fi.name)
		}
	}

	// Vector dimensions. Lucene guards on vectorDimension == 0.
	if opts.VectorDimension > 0 {
		if fi.vectorDimension == 0 {
			fi.vectorDimension = opts.VectorDimension
			fi.vectorEncoding = opts.VectorEncoding
			fi.vectorSimilarityFunction = opts.VectorSimilarityFunction
		} else if fi.vectorDimension != opts.VectorDimension ||
			fi.vectorEncoding != opts.VectorEncoding ||
			fi.vectorSimilarityFunction != opts.VectorSimilarityFunction {
			return fmt.Errorf("inconsistent vector schema for field %q", fi.name)
		}
	}

	// Store-side boolean flags accumulate (OR): a field stored or carrying term
	// vectors in any contribution keeps that flag set across the others.
	if opts.Stored {
		fi.stored = true
	}
	if opts.StoreTermVectors {
		fi.storeTermVectors = true
	}
	if opts.StoreTermVectorPositions {
		fi.storeTermVectorPositions = true
		fi.storeTermVectors = true
	}
	if opts.StoreTermVectorOffsets {
		fi.storeTermVectorOffsets = true
		fi.storeTermVectors = true
	}
	if opts.StoreTermVectorPayloads {
		fi.storeTermVectorPayloads = true
		fi.storeTermVectors = true
	}

	return nil
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

// IsFrozen reports whether this FieldInfo is immutable. Construction
// freezes the FieldInfo, after which only codec-owned attribute writes
// (PutCodecAttribute) and the indexing-chain payload / term-vector
// promotion methods (SetStorePayloads, SetStoreTermVectors) may
// mutate it.
func (fi *FieldInfo) IsFrozen() bool {
	return fi.frozen
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
