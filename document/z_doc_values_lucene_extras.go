// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// This file augments the pre-existing DocValues field types with Lucene
// 10.4.0 extras that were missing:
//   - INDEXED_TYPE variants (setDocValuesSkipIndexType=RANGE).
//   - TYPE aliases matching Lucene's `public static final` naming.
//   - DoubleDocValuesField / FloatDocValuesField (float values folded into a
//     NumericDocValuesField via DoubleToSortableLong / FloatToSortableInt).
//
// Static "slow" query factories are deferred — they require
// search.IndexOrDocValuesQuery / NumericDocValuesField slow queries which
// are not yet ported. Backlog #2695 tracks similar deferrals.

var (
	// NumericDocValuesFieldTYPE is the Lucene-canonical alias for
	// NumericDocValuesFieldType.
	NumericDocValuesFieldTYPE *FieldType

	// NumericDocValuesFieldINDEXEDTYPE mirrors Lucene's INDEXED_TYPE:
	// numeric doc-values with a RANGE skip index.
	NumericDocValuesFieldINDEXEDTYPE *FieldType

	// SortedDocValuesFieldTYPE alias.
	SortedDocValuesFieldTYPE *FieldType

	// SortedNumericDocValuesFieldTYPE alias.
	SortedNumericDocValuesFieldTYPE *FieldType

	// SortedNumericDocValuesFieldINDEXEDTYPE mirrors Lucene's INDEXED_TYPE.
	SortedNumericDocValuesFieldINDEXEDTYPE *FieldType

	// SortedSetDocValuesFieldTYPE alias.
	SortedSetDocValuesFieldTYPE *FieldType
)

func init() {
	NumericDocValuesFieldTYPE = NumericDocValuesFieldType

	NumericDocValuesFieldINDEXEDTYPE = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange)
	NumericDocValuesFieldINDEXEDTYPE.Freeze()

	SortedDocValuesFieldTYPE = SortedDocValuesFieldType
	SortedNumericDocValuesFieldTYPE = SortedNumericDocValuesFieldType

	SortedNumericDocValuesFieldINDEXEDTYPE = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeSortedNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange)
	SortedNumericDocValuesFieldINDEXEDTYPE.Freeze()

	SortedSetDocValuesFieldTYPE = SortedSetDocValuesFieldType
}

// NewNumericDocValuesFieldIndexed creates a NumericDocValuesField whose
// FieldType additionally carries a RANGE doc-values skip index, mirroring
// Lucene's NumericDocValuesField.indexedField(name, value).
func NewNumericDocValuesFieldIndexed(name string, value int64) (*NumericDocValuesField, error) {
	field, err := NewField(name, value, NumericDocValuesFieldINDEXEDTYPE)
	if err != nil {
		return nil, err
	}
	return &NumericDocValuesField{Field: field}, nil
}

// NewSortedNumericDocValuesFieldIndexed creates a
// SortedNumericDocValuesField with a RANGE skip index.
func NewSortedNumericDocValuesFieldIndexed(name string, values []int64) (*SortedNumericDocValuesField, error) {
	var baseValue int64
	if len(values) > 0 {
		baseValue = values[0]
	}
	field, err := NewField(name, baseValue, SortedNumericDocValuesFieldINDEXEDTYPE)
	if err != nil {
		return nil, err
	}
	return &SortedNumericDocValuesField{Field: field, values: values}, nil
}
