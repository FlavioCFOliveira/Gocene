// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"sync"
)

// Fields provides access to the terms for each field in the index.
// This is the Go port of Lucene's org.apache.lucene.index.Fields.
//
// Fields represents the collection of all indexed fields and their terms
// in a segment or reader. It provides access to the Terms for each field.
type Fields interface {
	// Iterator returns an iterator over all field names.
	// The returned iterator is positioned before the first field name.
	// Use Next() to advance to the first field.
	Iterator() (FieldIterator, error)

	// Size returns the number of fields.
	// Returns -1 if the size is unknown.
	Size() int

	// Terms returns the Terms for the specified field.
	// Returns nil if the field does not exist.
	Terms(field string) (Terms, error)
}

// FieldIterator provides an iterator over field names.
type FieldIterator interface {
	// Next advances to the next field name.
	// Returns the field name or empty string if there are no more fields.
	Next() (string, error)

	// HasNext returns true if there are more field names.
	HasNext() bool
}

// FieldsBase provides a base implementation of the Fields interface.
type FieldsBase struct{}

// Size returns -1 by default (unknown).
func (f *FieldsBase) Size() int {
	return -1
}

// EmptyFields is a Fields implementation with no fields.
type EmptyFields struct {
	FieldsBase
}

// Iterator returns an empty FieldIterator.
func (e *EmptyFields) Iterator() (FieldIterator, error) {
	return &EmptyFieldIterator{}, nil
}

// Size returns 0.
func (e *EmptyFields) Size() int {
	return 0
}

// Terms returns nil.
func (e *EmptyFields) Terms(field string) (Terms, error) {
	return nil, nil
}

// EmptyFieldIterator is a FieldIterator with no fields.
type EmptyFieldIterator struct{}

// Next returns empty string.
func (e *EmptyFieldIterator) Next() (string, error) {
	return "", nil
}

// HasNext returns false.
func (e *EmptyFieldIterator) HasNext() bool {
	return false
}

// MemoryFields is a Fields implementation backed by an in-memory map.
// This is useful for testing and small indexes.
type MemoryFields struct {
	FieldsBase
	mu     sync.RWMutex
	fields map[string]Terms
}

// NewMemoryFields creates a new empty MemoryFields.
func NewMemoryFields() *MemoryFields {
	return &MemoryFields{
		fields: make(map[string]Terms),
	}
}

// AddField adds a Terms for the specified field.
func (m *MemoryFields) AddField(field string, terms Terms) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fields[field] = terms
}

// RemoveField removes the Terms for the specified field.
func (m *MemoryFields) RemoveField(field string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fields, field)
}

// HasField returns true if the field exists.
func (m *MemoryFields) HasField(field string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.fields[field]
	return exists
}

// Iterator returns a FieldIterator over all field names.
func (m *MemoryFields) Iterator() (FieldIterator, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fields := make([]string, 0, len(m.fields))
	for field := range m.fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	return &MemoryFieldIterator{
		fields: fields,
		index:  -1,
	}, nil
}

// Size returns the number of fields.
func (m *MemoryFields) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.fields)
}

// Terms returns the Terms for the specified field.
func (m *MemoryFields) Terms(field string) (Terms, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	terms, exists := m.fields[field]
	if !exists {
		return nil, nil
	}
	return terms, nil
}

// GetFieldNames returns all field names sorted.
func (m *MemoryFields) GetFieldNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.fields))
	for field := range m.fields {
		names = append(names, field)
	}
	sort.Strings(names)
	return names
}

// MemoryFieldIterator is a FieldIterator for MemoryFields.
type MemoryFieldIterator struct {
	fields []string
	index  int
}

// Next advances to the next field name.
func (mi *MemoryFieldIterator) Next() (string, error) {
	mi.index++
	if mi.index >= len(mi.fields) {
		return "", nil
	}
	return mi.fields[mi.index], nil
}

// HasNext returns true if there are more field names.
func (mi *MemoryFieldIterator) HasNext() bool {
	return mi.index+1 < len(mi.fields)
}

// SingleFieldFields is a Fields implementation with exactly one field.
type SingleFieldFields struct {
	FieldsBase
	field string
	terms Terms
}

// NewSingleFieldFields creates a new SingleFieldFields.
func NewSingleFieldFields(field string, terms Terms) *SingleFieldFields {
	return &SingleFieldFields{
		field: field,
		terms: terms,
	}
}

// Iterator returns a FieldIterator for the single field.
func (s *SingleFieldFields) Iterator() (FieldIterator, error) {
	return &SingleFieldIterator{
		field: s.field,
		used:  false,
	}, nil
}

// Size returns 1.
func (s *SingleFieldFields) Size() int {
	return 1
}

// Terms returns the Terms if the field matches.
func (s *SingleFieldFields) Terms(field string) (Terms, error) {
	if s.field == field {
		return s.terms, nil
	}
	return nil, nil
}

// SingleFieldIterator is a FieldIterator for a single field.
type SingleFieldIterator struct {
	field string
	used  bool
}

// Next returns the field name if not used.
func (s *SingleFieldIterator) Next() (string, error) {
	if !s.used {
		s.used = true
		return s.field, nil
	}
	return "", nil
}

// HasNext returns true if not used.
func (s *SingleFieldIterator) HasNext() bool {
	return !s.used
}

// MultiFields combines multiple Fields into a single Fields.
type MultiFields struct {
	FieldsBase
	fieldsList []Fields
}

// NewMultiFields creates a new MultiFields from a list of Fields.
func NewMultiFields(fields ...Fields) *MultiFields {
	return &MultiFields{
		fieldsList: fields,
	}
}

// Iterator returns a FieldIterator over all fields in all contained Fields.
func (m *MultiFields) Iterator() (FieldIterator, error) {
	// Collect all field names
	fieldSet := make(map[string]struct{})
	for _, fields := range m.fieldsList {
		if fields == nil {
			continue
		}
		iter, err := fields.Iterator()
		if err != nil {
			return nil, err
		}
		for {
			field, err := iter.Next()
			if err != nil {
				return nil, err
			}
			if field == "" {
				break
			}
			fieldSet[field] = struct{}{}
		}
	}

	// Sort field names
	allFields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		allFields = append(allFields, field)
	}
	sort.Strings(allFields)

	return &MemoryFieldIterator{
		fields: allFields,
		index:  -1,
	}, nil
}

// Size returns the total number of unique fields.
func (m *MultiFields) Size() int {
	fieldSet := make(map[string]struct{})
	for _, fields := range m.fieldsList {
		if fields == nil {
			continue
		}
		size := fields.Size()
		if size < 0 {
			return -1 // Unknown
		}
		iter, err := fields.Iterator()
		if err != nil {
			return -1
		}
		for {
			field, err := iter.Next()
			if err != nil {
				return -1
			}
			if field == "" {
				break
			}
			fieldSet[field] = struct{}{}
		}
	}
	return len(fieldSet)
}

// Terms returns the Terms for the specified field from the first Fields that has it.
func (m *MultiFields) Terms(field string) (Terms, error) {
	for _, fields := range m.fieldsList {
		if fields == nil {
			continue
		}
		terms, err := fields.Terms(field)
		if err != nil {
			return nil, err
		}
		if terms != nil {
			return terms, nil
		}
	}
	return nil, nil
}

// FieldsStats holds statistics for a Fields instance.
type FieldsStats struct {
	// NumFields is the number of fields
	NumFields int

	// NumTerms is the total number of unique terms across all fields
	NumTerms int64

	// NumDocs is the total number of documents
	NumDocs int
}

// String returns a string representation of FieldsStats.
func (fs *FieldsStats) String() string {
	return fmt.Sprintf("FieldsStats(numFields=%d, numTerms=%d, numDocs=%d)",
		fs.NumFields, fs.NumTerms, fs.NumDocs)
}
