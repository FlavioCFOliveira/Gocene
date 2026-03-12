// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// FieldExistsQuery matches documents that have a value in the given field.
// This query finds all documents where the specified field exists,
// regardless of the field's value.
type FieldExistsQuery struct {
	*BaseQuery
	field string
}

// NewFieldExistsQuery creates a new FieldExistsQuery.
// The field parameter is the name of the field to check for existence.
func NewFieldExistsQuery(field string) *FieldExistsQuery {
	return &FieldExistsQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
	}
}

// GetField returns the field name.
func (q *FieldExistsQuery) GetField() string {
	return q.field
}

// Clone creates a copy of this query.
func (q *FieldExistsQuery) Clone() Query {
	return NewFieldExistsQuery(q.field)
}

// Equals checks if this query equals another.
func (q *FieldExistsQuery) Equals(other Query) bool {
	if o, ok := other.(*FieldExistsQuery); ok {
		return q.field == o.field
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *FieldExistsQuery) HashCode() int {
	h := 0
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	return h
}
