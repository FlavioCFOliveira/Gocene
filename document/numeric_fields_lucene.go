// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file ports the modern Lucene 10.x IntField/LongField/FloatField/
// DoubleField combined point+doc-values FieldType constants and adds
// Lucene-canonical NewXxxFieldLucene constructors. The pre-existing
// NewIntField/NewLongField/NewFloatField/NewDoubleField constructors are
// preserved unchanged for back-compat — they produce legacy stored-string
// fields rather than point+doc-values entries.
//
// Divergences from Java:
//   - Java's setDimensions(1, Integer.BYTES) etc. is mirrored by
//     PointDimensionCount=1, IndexDimensionCount=1, PointNumBytes=N.
//   - Static query factories are deferred (search.PointRangeQuery etc. not
//     yet ported). See backlog #2695.
//   - BinaryValue() on a Lucene IntField returns the sortable-bytes
//     encoding; this is exposed via the EncodeXxxLucene helpers below
//     rather than implicitly through the field's BinaryValue accessor
//     (which would conflict with the legacy NewIntField stringification).

var (
	// IntFieldTYPE is the FieldType for Lucene-modern IntField:
	// dimensionCount=1, numBytes=4, docValuesType=SORTED_NUMERIC, not stored.
	IntFieldTYPE *FieldType

	// IntFieldTYPESTORED is the stored variant of IntFieldTYPE.
	IntFieldTYPESTORED *FieldType

	// LongFieldTYPE / TYPESTORED — same as IntFieldTYPE with numBytes=8.
	LongFieldTYPE        *FieldType
	LongFieldTYPESTORED  *FieldType
	FloatFieldTYPE       *FieldType
	FloatFieldTYPESTORED *FieldType
	// DoubleFieldTYPE and DoubleFieldTYPESTORED are exported for parity with
	// IntFieldTYPE etc. Pre-existing DoubleFieldType (legacy) is left alone.
	DoubleFieldTYPE       *FieldType
	DoubleFieldTYPESTORED *FieldType
)

func init() {
	IntFieldTYPE = newNumericPointDocValuesType(false, 4)
	IntFieldTYPESTORED = newNumericPointDocValuesType(true, 4)
	LongFieldTYPE = newNumericPointDocValuesType(false, 8)
	LongFieldTYPESTORED = newNumericPointDocValuesType(true, 8)
	FloatFieldTYPE = newNumericPointDocValuesType(false, 4)
	FloatFieldTYPESTORED = newNumericPointDocValuesType(true, 4)
	DoubleFieldTYPE = newNumericPointDocValuesType(false, 8)
	DoubleFieldTYPESTORED = newNumericPointDocValuesType(true, 8)
}

func newNumericPointDocValuesType(stored bool, numBytes int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetTokenized(false)
	ft.SetStored(stored)
	ft.SetDimensions(1, numBytes)
	ft.SetDocValuesType(index.DocValuesTypeSortedNumeric)
	ft.Freeze()
	return ft
}

// IntFieldLucene is the modern Lucene 10.x IntField — a single struct
// holding a sortable-bytes-encoded int32 value backed by Point dimensions
// (1 × 4 bytes) and a SORTED_NUMERIC doc-values column. The pre-existing
// IntField in int_field.go is preserved for back-compat with shipped code.
type IntFieldLucene struct {
	*Field
}

// NewIntFieldLucene creates a new modern IntField backed by Point+DocValues.
// Mirrors Lucene's IntField(name, value, Store).
func NewIntFieldLucene(name string, value int32, stored bool) (*IntFieldLucene, error) {
	ft := IntFieldTYPE
	if stored {
		ft = IntFieldTYPESTORED
	}
	bytes := make([]byte, 4)
	util.IntToSortableBytes(value, bytes, 0)
	field, err := NewField(name, bytes, ft)
	if err != nil {
		return nil, err
	}
	return &IntFieldLucene{Field: field}, nil
}

// LongFieldLucene is the modern Lucene 10.x LongField — Point+DocValues
// backed int64.
type LongFieldLucene struct {
	*Field
}

// NewLongFieldLucene creates a new modern LongField. Mirrors Lucene's
// LongField(name, value, Store).
func NewLongFieldLucene(name string, value int64, stored bool) (*LongFieldLucene, error) {
	ft := LongFieldTYPE
	if stored {
		ft = LongFieldTYPESTORED
	}
	bytes := make([]byte, 8)
	util.LongToSortableBytes(value, bytes, 0)
	field, err := NewField(name, bytes, ft)
	if err != nil {
		return nil, err
	}
	return &LongFieldLucene{Field: field}, nil
}

// FloatFieldLucene is the modern Lucene 10.x FloatField — Point+DocValues
// backed float32.
type FloatFieldLucene struct {
	*Field
}

// NewFloatFieldLucene creates a new modern FloatField. The float value is
// converted to a sortable int32 (FloatToSortableInt) and then to sortable
// bytes, matching Lucene's NumericUtils.floatToSortableBytes path.
func NewFloatFieldLucene(name string, value float32, stored bool) (*FloatFieldLucene, error) {
	ft := FloatFieldTYPE
	if stored {
		ft = FloatFieldTYPESTORED
	}
	bytes := make([]byte, 4)
	util.IntToSortableBytes(util.FloatToSortableInt(value), bytes, 0)
	field, err := NewField(name, bytes, ft)
	if err != nil {
		return nil, err
	}
	return &FloatFieldLucene{Field: field}, nil
}

// DoubleFieldLucene is the modern Lucene 10.x DoubleField — Point+DocValues
// backed float64.
type DoubleFieldLucene struct {
	*Field
}

// NewDoubleFieldLucene creates a new modern DoubleField. The double value
// is converted to a sortable int64 (DoubleToSortableLong) and then to
// sortable bytes, matching Lucene's NumericUtils.doubleToSortableBytes path.
func NewDoubleFieldLucene(name string, value float64, stored bool) (*DoubleFieldLucene, error) {
	ft := DoubleFieldTYPE
	if stored {
		ft = DoubleFieldTYPESTORED
	}
	bytes := make([]byte, 8)
	util.LongToSortableBytes(util.DoubleToSortableLong(value), bytes, 0)
	field, err := NewField(name, bytes, ft)
	if err != nil {
		return nil, err
	}
	return &DoubleFieldLucene{Field: field}, nil
}
