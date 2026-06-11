// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package memory provides in-memory indexing capabilities.
// This package is useful for highlighting and other operations
// that need to analyze documents without writing to disk.
package memory

import (
	"fmt"
	"strings"
	"sync"
)

// docValue holds a single doc-value entry stored in MemoryIndex.
type docValue struct {
	numeric       *int64   // NumericDocValues
	binary        []byte   // BinaryDocValues
	sorted        []byte   // SortedDocValues
	sortedNumeric []int64  // SortedNumericDocValues
	sortedSet     [][]byte // SortedSetDocValues
}

// pointValue holds a single point-value entry stored in MemoryIndex.
type pointValue struct {
	packedValue []byte
	docID       int
}

// pointFieldMeta holds metadata for a point field stored in MemoryIndex.
type pointFieldMeta struct {
	numDims     int
	bytesPerDim int
	values      []pointValue
}

// MemoryIndex is an in-memory index that stores a single document.
// It is useful for highlighting and other operations that need to
// analyze a single document without writing to disk.
type MemoryIndex struct {
	// fields stores the field data
	fields map[string]*memoryField

	// docValues stores doc values per field
	docValues map[string]*docValue

	// pointFields stores point value metadata per field
	pointFields map[string]*pointFieldMeta

	// maxReusedBytes is the maximum number of bytes that can be reused
	maxReusedBytes int

	// mu protects the index
	mu sync.RWMutex

	// frozen indicates if the index is frozen
	frozen bool
}

// memoryField stores data for a single field in memory.
type memoryField struct {
	// fieldName is the name of the field
	fieldName string

	// terms stores the terms and their frequencies
	terms map[string]int

	// termPositions stores the positions of each term
	termPositions map[string][]int

	// termOffsets stores the offsets of each term
	termOffsets map[string][][2]int

	// positions is a list of all positions with their terms
	positions []positionInfo

	// boost is the boost for this field
	boost float32
}

// positionInfo stores information about a term position.
type positionInfo struct {
	term        string
	position    int
	startOffset int
	endOffset   int
}

// NewMemoryIndex creates a new MemoryIndex.
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{
		fields:         make(map[string]*memoryField),
		docValues:      make(map[string]*docValue),
		pointFields:    make(map[string]*pointFieldMeta),
		maxReusedBytes: 1024 * 1024, // 1MB default
	}
}

// NewMemoryIndexWithMaxReusedBytes creates a new MemoryIndex with a custom maxReusedBytes.
func NewMemoryIndexWithMaxReusedBytes(maxReusedBytes int) *MemoryIndex {
	return &MemoryIndex{
		fields:         make(map[string]*memoryField),
		docValues:      make(map[string]*docValue),
		pointFields:    make(map[string]*pointFieldMeta),
		maxReusedBytes: maxReusedBytes,
	}
}

// AddField adds a field to the index.
func (mi *MemoryIndex) AddField(fieldName string, value string) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}

	return mi.addFieldInternal(fieldName, value, 1.0)
}

// AddFieldWithBoost adds a field with a boost.
func (mi *MemoryIndex) AddFieldWithBoost(fieldName string, value string, boost float32) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}

	return mi.addFieldInternal(fieldName, value, boost)
}

// addFieldInternal adds a field to the index.
func (mi *MemoryIndex) addFieldInternal(fieldName string, value string, boost float32) error {
	if value == "" {
		return nil
	}

	mf := &memoryField{
		fieldName:     fieldName,
		terms:         make(map[string]int),
		termPositions: make(map[string][]int),
		termOffsets:   make(map[string][][2]int),
		boost:         boost,
	}

	// Simple tokenization by splitting on whitespace and punctuation
	position := 0
	start := 0
	for i, ch := range value {
		if isSeparator(ch) {
			if i > start {
				term := value[start:i]
				mf.addTerm(term, position, start, i)
				position++
			}
			start = i + 1
		}
	}
	// Add final term
	if start < len(value) {
		term := value[start:]
		mf.addTerm(term, position, start, len(value))
	}

	mi.fields[fieldName] = mf
	return nil
}

// isSeparator returns true if the rune is a separator.
func isSeparator(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
		r == '.' || r == ',' || r == '!' || r == '?' ||
		r == ';' || r == ':' || r == '-' || r == '_' ||
		r == '(' || r == ')' || r == '[' || r == ']' ||
		r == '{' || r == '}' || r == '<' || r == '>' ||
		r == '/' || r == '\\' || r == '"' || r == '\''
}

// addTerm adds a term occurrence to the field.
func (mf *memoryField) addTerm(term string, position int, startOffset, endOffset int) {
	mf.terms[term]++
	mf.termPositions[term] = append(mf.termPositions[term], position)
	mf.termOffsets[term] = append(mf.termOffsets[term], [2]int{startOffset, endOffset})
	mf.positions = append(mf.positions, positionInfo{
		term:        term,
		position:    position,
		startOffset: startOffset,
		endOffset:   endOffset,
	})
}

// AddNumericDocValues stores a numeric doc value for the given field.
func (mi *MemoryIndex) AddNumericDocValues(field string, value int64) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	dv := mi.docValues[field]
	if dv == nil {
		dv = &docValue{}
		mi.docValues[field] = dv
	}
	dv.numeric = &value
	return nil
}

// AddBinaryDocValues stores a binary doc value for the given field.
func (mi *MemoryIndex) AddBinaryDocValues(field string, value []byte) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	dv := mi.docValues[field]
	if dv == nil {
		dv = &docValue{}
		mi.docValues[field] = dv
	}
	dv.binary = value
	return nil
}

// AddSortedDocValues stores a sorted doc value for the given field.
func (mi *MemoryIndex) AddSortedDocValues(field string, value []byte) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	dv := mi.docValues[field]
	if dv == nil {
		dv = &docValue{}
		mi.docValues[field] = dv
	}
	dv.sorted = value
	return nil
}

// AddSortedNumericDocValues stores sorted numeric doc values for the given field.
func (mi *MemoryIndex) AddSortedNumericDocValues(field string, values []int64) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	dv := mi.docValues[field]
	if dv == nil {
		dv = &docValue{}
		mi.docValues[field] = dv
	}
	dv.sortedNumeric = values
	return nil
}

// AddSortedSetDocValues stores sorted set doc values for the given field.
func (mi *MemoryIndex) AddSortedSetDocValues(field string, values [][]byte) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	dv := mi.docValues[field]
	if dv == nil {
		dv = &docValue{}
		mi.docValues[field] = dv
	}
	dv.sortedSet = values
	return nil
}

// AddPointField adds a point value for the given field.
func (mi *MemoryIndex) AddPointField(field string, packedValue []byte, numDims, bytesPerDim int) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if mi.frozen {
		return fmt.Errorf("index is frozen")
	}
	pfm := mi.pointFields[field]
	if pfm == nil {
		pfm = &pointFieldMeta{
			numDims:     numDims,
			bytesPerDim: bytesPerDim,
		}
		mi.pointFields[field] = pfm
	}
	pfm.values = append(pfm.values, pointValue{packedValue: packedValue, docID: 0})
	return nil
}

// Freeze freezes the index, preventing further modifications.
func (mi *MemoryIndex) Freeze() {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.frozen = true
}

// IsFrozen returns true if the index is frozen.
func (mi *MemoryIndex) IsFrozen() bool {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.frozen
}

// GetFieldTerms returns the terms for a field.
func (mi *MemoryIndex) GetFieldTerms(fieldName string) map[string]int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	mf, ok := mi.fields[fieldName]
	if !ok {
		return nil
	}

	// Return a copy
	result := make(map[string]int, len(mf.terms))
	for term, freq := range mf.terms {
		result[term] = freq
	}
	return result
}

// GetTermFrequency returns the frequency of a term in a field.
func (mi *MemoryIndex) GetTermFrequency(fieldName string, term string) int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	mf, ok := mi.fields[fieldName]
	if !ok {
		return 0
	}

	return mf.terms[term]
}

// GetTermPositions returns the positions of a term in a field.
func (mi *MemoryIndex) GetTermPositions(fieldName string, term string) []int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	mf, ok := mi.fields[fieldName]
	if !ok {
		return nil
	}

	// Return a copy
	positions := mf.termPositions[term]
	result := make([]int, len(positions))
	copy(result, positions)
	return result
}

// GetTermOffsets returns the start and end offsets of a term in a field.
func (mi *MemoryIndex) GetTermOffsets(fieldName string, term string) [][2]int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	mf, ok := mi.fields[fieldName]
	if !ok {
		return nil
	}

	// Return a copy
	offsets := mf.termOffsets[term]
	result := make([][2]int, len(offsets))
	copy(result, offsets)
	return result
}

// Size returns the number of fields in the index.
func (mi *MemoryIndex) Size() int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return len(mi.fields)
}

// GetFields returns the names of all fields in the index.
func (mi *MemoryIndex) GetFields() []string {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	result := make([]string, 0, len(mi.fields))
	for fieldName := range mi.fields {
		result = append(result, fieldName)
	}
	return result
}

// Reset clears the index.
func (mi *MemoryIndex) Reset() {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	mi.fields = make(map[string]*memoryField)
	mi.docValues = make(map[string]*docValue)
	mi.pointFields = make(map[string]*pointFieldMeta)
	mi.frozen = false
}

// String returns a string representation of the index.
func (mi *MemoryIndex) String() string {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("MemoryIndex{fields=%d}\n", len(mi.fields)))

	for fieldName, mf := range mi.fields {
		sb.WriteString(fmt.Sprintf("  %s: %d terms\n", fieldName, len(mf.terms)))
		for term, freq := range mf.terms {
			sb.WriteString(fmt.Sprintf("    %s: %d\n", term, freq))
		}
	}

	return sb.String()
}

// Ensure MemoryIndex implements Stringer
var _ fmt.Stringer = (*MemoryIndex)(nil)
