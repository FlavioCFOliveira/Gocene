// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FieldType describes the properties of a field.
// This is the Go port of Lucene's org.apache.lucene.document.FieldType.
type FieldType struct {
	// Indexed determines whether the field is indexed for searching.
	// If false, the field is not searchable.
	Indexed bool

	// Stored determines whether the field value is stored in the index.
	// Stored fields can be retrieved in search results.
	Stored bool

	// Tokenized determines whether the field value should be tokenized.
	// If true, the field value is analyzed and tokens are indexed.
	// If false, the field value is indexed as a single term.
	Tokenized bool

	// StoreTermVectors determines whether term vectors are stored.
	// Term vectors include term frequencies and positions per document.
	StoreTermVectors bool

	// StoreTermVectorPositions determines whether term vector positions are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorPositions bool

	// StoreTermVectorOffsets determines whether term vector offsets are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorOffsets bool

	// StoreTermVectorPayloads determines whether term vector payloads are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorPayloads bool

	// OmitNorms determines whether field norms (boost factors) are omitted.
	// If true, the field will not have scoring based on field length.
	OmitNorms bool

	// IndexOptions controls what information is stored in the postings lists.
	// See index.IndexOptions for details.
	IndexOptions index.IndexOptions

	// DocValuesType determines the type of per-document values stored.
	// See index.DocValuesType for details.
	DocValuesType index.DocValuesType

	// frozen tracks whether this FieldType has been frozen (made immutable).
	frozen bool
}

// NewFieldType creates a new FieldType with default values.
func NewFieldType() *FieldType {
	return &FieldType{
		Indexed:                  false,
		Stored:                   false,
		Tokenized:                false,
		StoreTermVectors:         false,
		StoreTermVectorPositions: false,
		StoreTermVectorOffsets:   false,
		StoreTermVectorPayloads:  false,
		OmitNorms:                false,
		IndexOptions:             index.IndexOptionsNone,
		DocValuesType:            index.DocValuesTypeNone,
		frozen:                   false,
	}
}

// Freeze makes this FieldType immutable.
// After freezing, any attempt to modify the FieldType will panic.
func (ft *FieldType) Freeze() {
	ft.frozen = true
}

// IsFrozen returns true if this FieldType has been frozen.
func (ft *FieldType) IsFrozen() bool {
	return ft.frozen
}

// checkFrozen panics if the FieldType is frozen.
func (ft *FieldType) checkFrozen() {
	if ft.frozen {
		panic("FieldType is frozen and cannot be modified")
	}
}

// SetIndexed sets whether the field is indexed.
func (ft *FieldType) SetIndexed(indexed bool) *FieldType {
	ft.checkFrozen()
	ft.Indexed = indexed
	return ft
}

// SetStored sets whether the field value is stored.
func (ft *FieldType) SetStored(stored bool) *FieldType {
	ft.checkFrozen()
	ft.Stored = stored
	return ft
}

// SetTokenized sets whether the field value should be tokenized.
func (ft *FieldType) SetTokenized(tokenized bool) *FieldType {
	ft.checkFrozen()
	ft.Tokenized = tokenized
	return ft
}

// SetStoreTermVectors sets whether term vectors are stored.
func (ft *FieldType) SetStoreTermVectors(store bool) *FieldType {
	ft.checkFrozen()
	ft.StoreTermVectors = store
	return ft
}

// SetIndexOptions sets the indexing options.
func (ft *FieldType) SetIndexOptions(options index.IndexOptions) *FieldType {
	ft.checkFrozen()
	ft.IndexOptions = options
	return ft
}

// SetDocValuesType sets the doc values type.
func (ft *FieldType) SetDocValuesType(docValuesType index.DocValuesType) *FieldType {
	ft.checkFrozen()
	ft.DocValuesType = docValuesType
	return ft
}

// SetOmitNorms sets whether norms are omitted.
func (ft *FieldType) SetOmitNorms(omit bool) *FieldType {
	ft.checkFrozen()
	ft.OmitNorms = omit
	return ft
}

// Validate validates this FieldType configuration.
// Returns an error if the configuration is invalid.
func (ft *FieldType) Validate() error {
	// If term vectors positions/offsets/payloads are set, term vectors must be enabled
	if ft.StoreTermVectorPositions && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector positions without storing term vectors"}
	}
	if ft.StoreTermVectorOffsets && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector offsets without storing term vectors"}
	}
	if ft.StoreTermVectorPayloads && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector payloads without storing term vectors"}
	}

	// If indexed, IndexOptions must be valid
	if ft.Indexed && ft.IndexOptions == index.IndexOptionsNone {
		return &FieldTypeValidationError{msg: "indexed field cannot have IndexOptionsNone"}
	}

	// If tokenized is true, indexed must also be true
	if ft.Tokenized && !ft.Indexed {
		return &FieldTypeValidationError{msg: "tokenized field must be indexed"}
	}

	return nil
}

// FieldTypeValidationError is returned when FieldType validation fails.
type FieldTypeValidationError struct {
	msg string
}

// Error returns the error message.
func (e *FieldTypeValidationError) Error() string {
	return "FieldType validation error: " + e.msg
}
