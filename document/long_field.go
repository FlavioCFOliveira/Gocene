// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LongField is a field for indexing int64 values.
type LongField struct {
	*Field
}

// NewLongField creates a new LongField.
func NewLongField(name string, value int64, store bool) (*LongField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatInt(value, 10), ft)
	if err != nil {
		return nil, err
	}

	return &LongField{Field: field}, nil
}

// LongPoint is an indexed int64 point field for range queries.
type LongPoint struct {
	*Field
}

// NewLongPoint creates a new LongPoint.
func NewLongPoint(name string, value int64) (*LongPoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatInt(value, 10), ft)
	if err != nil {
		return nil, err
	}

	return &LongPoint{Field: field}, nil
}
