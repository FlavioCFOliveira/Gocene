// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"sync"
)

// FieldInfos manages a collection of FieldInfo objects.
// This is the Go port of Lucene's org.apache.lucene.index.FieldInfos.
//
// FieldInfos provides access to FieldInfo by name or number, and
// provides aggregate information about the fields in an index.
type FieldInfos struct {
	// mu protects the internal maps
	mu sync.RWMutex

	// byName maps field names to FieldInfo
	byName map[string]*FieldInfo

	// byNumber maps field numbers to FieldInfo
	byNumber map[int]*FieldInfo

	// names contains all field names in sorted order
	names []string

	// nextFieldNumber is the next available field number
	nextFieldNumber int

	// frozen is set to true to prevent modification
	frozen bool
}

// NewFieldInfos creates a new empty FieldInfos.
func NewFieldInfos() *FieldInfos {
	return &FieldInfos{
		byName:          make(map[string]*FieldInfo),
		byNumber:        make(map[int]*FieldInfo),
		names:           make([]string, 0, 16), // Pre-allocate capacity
		nextFieldNumber: 0,
		frozen:          false,
	}
}

// Add adds a FieldInfo to this collection.
// Returns an error if a field with the same name already exists
// or if the field number conflicts with an existing field.
func (fi *FieldInfos) Add(fieldInfo *FieldInfo) error {
	if fi.frozen {
		return fmt.Errorf("FieldInfos is immutable")
	}

	fi.mu.Lock()
	defer fi.mu.Unlock()

	name := fieldInfo.Name()
	number := fieldInfo.Number()

	// Check if field already exists
	if existing, ok := fi.byName[name]; ok {
		if existing.Number() != number {
			return fmt.Errorf("field '%s' already exists with different number: %d vs %d",
				name, existing.Number(), number)
		}
		// Same field, ignore
		return nil
	}

	// Check if number is already used
	if existing, ok := fi.byNumber[number]; ok {
		if existing.Name() != name {
			return fmt.Errorf("field number %d already used by field '%s'",
				number, existing.Name())
		}
	}

	// Add the field
	fi.byName[name] = fieldInfo
	fi.byNumber[number] = fieldInfo
	fi.names = append(fi.names, name)
	sort.Strings(fi.names)

	// Update next field number
	if number >= fi.nextFieldNumber {
		fi.nextFieldNumber = number + 1
	}

	return nil
}

// GetByName returns the FieldInfo for the specified field name.
// Returns nil if the field does not exist.
func (fi *FieldInfos) GetByName(name string) *FieldInfo {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.byName[name]
}

// GetByNumber returns the FieldInfo for the specified field number.
// Returns nil if the field does not exist.
func (fi *FieldInfos) GetByNumber(number int) *FieldInfo {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.byNumber[number]
}

// Size returns the number of fields.
func (fi *FieldInfos) Size() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return len(fi.byName)
}

// Iterator returns an iterator over all FieldInfo objects.
// The fields are returned in order of field number.
func (fi *FieldInfos) Iterator() FieldInfosIterator {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	// Create sorted list by field number
	infos := make([]*FieldInfo, 0, len(fi.byNumber))
	numbers := make([]int, 0, len(fi.byNumber))
	for num := range fi.byNumber {
		numbers = append(numbers, num)
	}
	sort.Ints(numbers)
	for _, num := range numbers {
		infos = append(infos, fi.byNumber[num])
	}

	return &fieldInfosIterator{
		infos: infos,
		index: -1,
	}
}

// Names returns all field names in sorted order.
func (fi *FieldInfos) Names() []string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	// Return a copy
	result := make([]string, len(fi.names))
	copy(result, fi.names)
	return result
}

// HasProx returns true if any field has positions stored.
func (fi *FieldInfos) HasProx() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.IndexOptions().HasPositions() {
			return true
		}
	}
	return false
}

// HasFreq returns true if any field has term frequencies stored.
func (fi *FieldInfos) HasFreq() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.IndexOptions().HasFreqs() {
			return true
		}
	}
	return false
}

// HasOffsets returns true if any field has offsets stored.
func (fi *FieldInfos) HasOffsets() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.IndexOptions().HasOffsets() {
			return true
		}
	}
	return false
}

// HasDocValues returns true if any field has doc values.
func (fi *FieldInfos) HasDocValues() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.DocValuesType().HasDocValues() {
			return true
		}
	}
	return false
}

// HasNorms returns true if any field has norms.
func (fi *FieldInfos) HasNorms() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.HasNorms() {
			return true
		}
	}
	return false
}

// HasTermVectors returns true if any field has term vectors.
func (fi *FieldInfos) HasTermVectors() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.StoreTermVectors() {
			return true
		}
	}
	return false
}

// HasPostings returns true if any field has postings (is indexed).
func (fi *FieldInfos) HasPostings() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.IndexOptions().IsIndexed() {
			return true
		}
	}
	return false
}

// HasStoredFields returns true if any field has stored content.
func (fi *FieldInfos) HasStoredFields() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	for _, fieldInfo := range fi.byName {
		if fieldInfo.IsStored() {
			return true
		}
	}
	return false
}

// GetNextFieldNumber returns the next available field number.
func (fi *FieldInfos) GetNextFieldNumber() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.nextFieldNumber
}

// Freeze makes this FieldInfos immutable.
func (fi *FieldInfos) Freeze() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.frozen = true
}

// IsFrozen returns true if this FieldInfos is immutable.
func (fi *FieldInfos) IsFrozen() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.frozen
}

// Clear removes all fields.
func (fi *FieldInfos) Clear() {
	if fi.frozen {
		return
	}

	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.byName = make(map[string]*FieldInfo)
	fi.byNumber = make(map[int]*FieldInfo)
	fi.names = make([]string, 0, 16)
	fi.nextFieldNumber = 0
}

// String returns a string representation of FieldInfos.
func (fi *FieldInfos) String() string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fmt.Sprintf("FieldInfos(size=%d)", len(fi.byName))
}

// FieldInfosIterator provides iteration over FieldInfo objects.
type FieldInfosIterator interface {
	// Next advances to the next FieldInfo.
	// Returns nil if there are no more FieldInfo objects.
	Next() *FieldInfo

	// HasNext returns true if there are more FieldInfo objects.
	HasNext() bool
}

// fieldInfosIterator implements FieldInfosIterator.
type fieldInfosIterator struct {
	infos []*FieldInfo
	index int
}

// Next returns the next FieldInfo.
func (it *fieldInfosIterator) Next() *FieldInfo {
	it.index++
	if it.index >= len(it.infos) {
		return nil
	}
	return it.infos[it.index]
}

// HasNext returns true if there are more FieldInfo objects.
func (it *fieldInfosIterator) HasNext() bool {
	return it.index+1 < len(it.infos)
}

// EmptyFieldInfos is a FieldInfos with no fields.
var EmptyFieldInfos = &FieldInfos{
	byName:          make(map[string]*FieldInfo),
	byNumber:        make(map[int]*FieldInfo),
	names:           make([]string, 0),
	nextFieldNumber: 0,
	frozen:          true,
}

// FieldInfosBuilder helps construct FieldInfos with a fluent API.
type FieldInfosBuilder struct {
	fieldInfos *FieldInfos
}

// NewFieldInfosBuilder creates a new FieldInfosBuilder.
func NewFieldInfosBuilder() *FieldInfosBuilder {
	return &FieldInfosBuilder{
		fieldInfos: NewFieldInfos(),
	}
}

// Add adds a FieldInfo to the builder.
func (b *FieldInfosBuilder) Add(fieldInfo *FieldInfo) *FieldInfosBuilder {
	b.fieldInfos.Add(fieldInfo)
	return b
}

// AddFromOptions creates and adds a FieldInfo from options.
func (b *FieldInfosBuilder) AddFromOptions(name string, opts FieldInfoOptions) *FieldInfosBuilder {
	fieldInfo := NewFieldInfo(name, b.fieldInfos.GetNextFieldNumber(), opts)
	b.fieldInfos.Add(fieldInfo)
	return b
}

// Build creates the FieldInfos.
func (b *FieldInfosBuilder) Build() *FieldInfos {
	b.fieldInfos.Freeze()
	return b.fieldInfos
}
