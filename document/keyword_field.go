// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// KeywordField is an indexed and doc-values-stored single-token field
// suited for exact-match filtering, sorting and faceting.
//
// This is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.KeywordField. The field indexes a per-document
// string (or byte slice) as a single token into the inverted index and
// stores the value in a SORTED_SET doc-values column.
//
// Divergences from Java:
//   - Static query factories (NewExactQuery, NewSetQuery, NewSortField)
//     are deferred — they require search.ConstantScoreQuery,
//     search.TermInSetQuery and search.SortedSetSortField which are not
//     yet ported. Tracked by backlog task #2695 (Point factories) which
//     covers similar deferrals.
type KeywordField struct {
	*Field
}

var (
	// KeywordFieldType is the FieldType used by KeywordField when the value
	// is not stored. Mirrors Lucene's static FIELD_TYPE constant.
	KeywordFieldType *FieldType

	// KeywordFieldTypeStored is the FieldType used when the value is also
	// stored. Mirrors Lucene's static FIELD_TYPE_STORED constant.
	KeywordFieldTypeStored *FieldType

	// KeywordFieldFIELDTYPE is the Lucene-canonical alias for KeywordFieldType.
	KeywordFieldFIELDTYPE *FieldType

	// KeywordFieldFIELDTYPESTORED is the Lucene-canonical alias for
	// KeywordFieldTypeStored.
	KeywordFieldFIELDTYPESTORED *FieldType
)

func init() {
	KeywordFieldType = NewFieldType().
		SetIndexed(true).
		SetTokenized(false).
		SetOmitNorms(true).
		SetIndexOptions(index.IndexOptionsDocs).
		SetDocValuesType(index.DocValuesTypeSortedSet)
	KeywordFieldType.Freeze()

	KeywordFieldTypeStored = NewFieldType().
		SetIndexed(true).
		SetStored(true).
		SetTokenized(false).
		SetOmitNorms(true).
		SetIndexOptions(index.IndexOptionsDocs).
		SetDocValuesType(index.DocValuesTypeSortedSet)
	KeywordFieldTypeStored.Freeze()

	KeywordFieldFIELDTYPE = KeywordFieldType
	KeywordFieldFIELDTYPESTORED = KeywordFieldTypeStored
}

// NewKeywordField creates a new KeywordField with a string value.
// If stored is true, the value is also stored alongside the inverted
// index and SORTED_SET doc-values entry.
func NewKeywordField(name, value string, stored bool) (*KeywordField, error) {
	if name == "" {
		return nil, fmt.Errorf("KeywordField name cannot be empty")
	}
	ft := KeywordFieldType
	if stored {
		ft = KeywordFieldTypeStored
	}
	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}
	return &KeywordField{Field: field}, nil
}

// NewKeywordFieldFromBytes creates a new KeywordField with a binary value.
func NewKeywordFieldFromBytes(name string, value []byte, stored bool) (*KeywordField, error) {
	if name == "" {
		return nil, fmt.Errorf("KeywordField name cannot be empty")
	}
	if value == nil {
		return nil, fmt.Errorf("KeywordField value cannot be nil")
	}
	ft := KeywordFieldType
	if stored {
		ft = KeywordFieldTypeStored
	}
	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}
	return &KeywordField{Field: field}, nil
}
