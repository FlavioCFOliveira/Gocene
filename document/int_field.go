// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "strconv"

// IntField is a field for indexing int values.
type IntField struct {
	*Field
}

// NewIntField creates a new IntField.
func NewIntField(name string, value int, store bool) (*IntField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.Freeze()

	field, err := NewField(name, strconv.Itoa(value), ft)
	if err != nil {
		return nil, err
	}

	return &IntField{Field: field}, nil
}

// IntPoint is an indexed int point field for range queries.
type IntPoint struct {
	*Field
}

// NewIntPoint creates a new IntPoint.
func NewIntPoint(name string, value int) (*IntPoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.Freeze()

	field, err := NewField(name, strconv.Itoa(value), ft)
	if err != nil {
		return nil, err
	}

	return &IntPoint{Field: field}, nil
}
