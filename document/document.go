// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
)

// Document is a collection of IndexableField objects.
// Documents are the unit of indexing and search.
//
// This is the Go port of Lucene's org.apache.lucene.document.Document.
type Document struct {
	fields []IndexableField
}

// NewDocument creates a new empty Document.
func NewDocument() *Document {
	return &Document{
		fields: make([]IndexableField, 0),
	}
}

// Add adds a field to the document.
func (d *Document) Add(field IndexableField) {
	if field == nil {
		panic("field cannot be nil")
	}
	d.fields = append(d.fields, field)
}

// AddField adds a field to the document with the given name and value.
// This is a convenience method that creates a Field and adds it.
func (d *Document) AddField(name string, value interface{}, ft *FieldType) error {
	field, err := NewField(name, value, ft)
	if err != nil {
		return err
	}
	d.Add(field)
	return nil
}

// Get returns the first field with the given name.
// Returns nil if no field with that name exists.
func (d *Document) Get(name string) IndexableField {
	for _, field := range d.fields {
		if field.Name() == name {
			return field
		}
	}
	return nil
}

// GetFields returns all fields in this document as interface{} slice.
// This is used to satisfy index.Document interface without circular imports.
func (d *Document) GetFields() []interface{} {
	result := make([]interface{}, len(d.fields))
	for i, field := range d.fields {
		result[i] = field
	}
	return result
}

// GetFieldsByName returns all fields with the given name.
// Returns an empty slice if no fields with that name exist.
func (d *Document) GetFieldsByName(name string) []IndexableField {
	var result []IndexableField
	for _, field := range d.fields {
		if field.Name() == name {
			result = append(result, field)
		}
	}
	return result
}

// GetAllFields returns all fields in this document.
// Returns a copy of the internal field slice.
func (d *Document) GetAllFields() []IndexableField {
	result := make([]IndexableField, len(d.fields))
	copy(result, d.fields)
	return result
}

// GetFieldNames returns a slice of all unique field names.
func (d *Document) GetFieldNames() []string {
	seen := make(map[string]struct{})
	var names []string
	for _, field := range d.fields {
		name := field.Name()
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// GetValues returns all values for the given field name.
// Returns a slice of string values.
func (d *Document) GetValues(name string) []string {
	var values []string
	for _, field := range d.fields {
		if field.Name() == name {
			values = append(values, field.StringValue())
		}
	}
	return values
}

// GetBinaryValues returns all binary values for the given field name.
func (d *Document) GetBinaryValues(name string) [][]byte {
	var values [][]byte
	for _, field := range d.fields {
		if field.Name() == name {
			if bv := field.BinaryValue(); bv != nil {
				values = append(values, bv)
			}
		}
	}
	return values
}

// RemoveField removes the first field with the given name.
// Returns true if a field was removed.
func (d *Document) RemoveField(name string) bool {
	for i, field := range d.fields {
		if field.Name() == name {
			// Remove the field by slicing
			d.fields = append(d.fields[:i], d.fields[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveFields removes all fields with the given name.
// Returns the number of fields removed.
func (d *Document) RemoveFields(name string) int {
	count := 0
	for i := len(d.fields) - 1; i >= 0; i-- {
		if d.fields[i].Name() == name {
			d.fields = append(d.fields[:i], d.fields[i+1:]...)
			count++
		}
	}
	return count
}

// Clear removes all fields from the document.
func (d *Document) Clear() {
	d.fields = d.fields[:0]
}

// Size returns the number of fields in the document.
func (d *Document) Size() int {
	return len(d.fields)
}

// IsEmpty returns true if the document has no fields.
func (d *Document) IsEmpty() bool {
	return len(d.fields) == 0
}

// HasField returns true if a field with the given name exists.
func (d *Document) HasField(name string) bool {
	for _, field := range d.fields {
		if field.Name() == name {
			return true
		}
	}
	return false
}

// GetFieldCount returns the number of fields with the given name.
func (d *Document) GetFieldCount(name string) int {
	count := 0
	for _, field := range d.fields {
		if field.Name() == name {
			count++
		}
	}
	return count
}

// String returns a string representation of the document.
func (d *Document) String() string {
	var result string
	for i, field := range d.fields {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("%s: %s", field.Name(), field.StringValue())
	}
	return result
}
