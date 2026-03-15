// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MemoryDocValuesProducer is an in-memory implementation of DocValuesProducer.
// This is useful for testing and for small indexes that fit in memory.
type MemoryDocValuesProducer struct {
	numericFields      map[string]*memoryNumericDocValues
	binaryFields       map[string]*memoryBinaryDocValues
	sortedFields       map[string]*memorySortedDocValues
	sortedSetFields    map[string]*memorySortedSetDocValues
	sortedNumericFields map[string]*memorySortedNumericDocValues
	mu                 sync.RWMutex
	closed             bool
}

// NewMemoryDocValuesProducer creates a new MemoryDocValuesProducer.
func NewMemoryDocValuesProducer() *MemoryDocValuesProducer {
	return &MemoryDocValuesProducer{
		numericFields:       make(map[string]*memoryNumericDocValues),
		binaryFields:        make(map[string]*memoryBinaryDocValues),
		sortedFields:        make(map[string]*memorySortedDocValues),
		sortedSetFields:     make(map[string]*memorySortedSetDocValues),
		sortedNumericFields: make(map[string]*memorySortedNumericDocValues),
	}
}

// GetNumeric returns a NumericDocValues for the given field.
func (p *MemoryDocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.numericFields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// GetBinary returns a BinaryDocValues for the given field.
func (p *MemoryDocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.binaryFields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// GetSorted returns a SortedDocValues for the given field.
func (p *MemoryDocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.sortedFields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// GetSortedSet returns a SortedSetDocValues for the given field.
func (p *MemoryDocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.sortedSetFields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// GetSortedNumeric returns a SortedNumericDocValues for the given field.
func (p *MemoryDocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.sortedNumericFields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// CheckIntegrity checks the integrity of the doc values.
func (p *MemoryDocValuesProducer) CheckIntegrity() error {
	// In-memory implementation is always valid
	return nil
}

// Close releases resources.
func (p *MemoryDocValuesProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	p.numericFields = nil
	p.binaryFields = nil
	p.sortedFields = nil
	p.sortedSetFields = nil
	p.sortedNumericFields = nil
	return nil
}

// SetNumericField sets a numeric field for testing.
func (p *MemoryDocValuesProducer) SetNumericField(name string, dv *memoryNumericDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.numericFields[name] = dv
}

// SetBinaryField sets a binary field for testing.
func (p *MemoryDocValuesProducer) SetBinaryField(name string, dv *memoryBinaryDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.binaryFields[name] = dv
}

// SetSortedField sets a sorted field for testing.
func (p *MemoryDocValuesProducer) SetSortedField(name string, dv *memorySortedDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sortedFields[name] = dv
}

// SetSortedSetField sets a sorted set field for testing.
func (p *MemoryDocValuesProducer) SetSortedSetField(name string, dv *memorySortedSetDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sortedSetFields[name] = dv
}

// SetSortedNumericField sets a sorted numeric field for testing.
func (p *MemoryDocValuesProducer) SetSortedNumericField(name string, dv *memorySortedNumericDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sortedNumericFields[name] = dv
}

// memoryNumericDocValues is an in-memory implementation of NumericDocValues.
type memoryNumericDocValues struct {
	values   map[int]int64
	docIDs   []int
	pos      int
	currDoc  int
}

// NewMemoryNumericDocValues creates a new memoryNumericDocValues.
func NewMemoryNumericDocValues(values map[int]int64) *memoryNumericDocValues {
	docIDs := make([]int, 0, len(values))
	for docID := range values {
		docIDs = append(docIDs, docID)
	}
	// Sort docIDs for deterministic iteration
	util.IntroSortOrdered(docIDs)

	return &memoryNumericDocValues{
		values:  values,
		docIDs:  docIDs,
		pos:     -1,
		currDoc: -1,
	}
}

// DocID returns the current document ID.
func (dv *memoryNumericDocValues) DocID() int {
	return dv.currDoc
}

// NextDoc advances to the next document that has a value.
func (dv *memoryNumericDocValues) NextDoc() (int, error) {
	dv.pos++
	if dv.pos >= len(dv.docIDs) {
		dv.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	dv.currDoc = dv.docIDs[dv.pos]
	return dv.currDoc, nil
}

// Advance advances to the first document >= target that has a value.
func (dv *memoryNumericDocValues) Advance(target int) (int, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] >= target {
			dv.currDoc = dv.docIDs[dv.pos]
			return dv.currDoc, nil
		}
	}
	dv.currDoc = index.NO_MORE_DOCS
	return index.NO_MORE_DOCS, nil
}

// LongValue returns the current document's value.
func (dv *memoryNumericDocValues) LongValue() (int64, error) {
	if dv.currDoc == index.NO_MORE_DOCS {
		return 0, fmt.Errorf("no more documents")
	}
	if val, ok := dv.values[dv.currDoc]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("no value for doc %d", dv.currDoc)
}

// Cost returns an estimate of the cost.
func (dv *memoryNumericDocValues) Cost() int64 {
	return int64(len(dv.docIDs))
}

// memoryBinaryDocValues is an in-memory implementation of BinaryDocValues.
type memoryBinaryDocValues struct {
	values  map[int][]byte
	docIDs  []int
	pos     int
	currDoc int
}

// NewMemoryBinaryDocValues creates a new memoryBinaryDocValues.
func NewMemoryBinaryDocValues(values map[int][]byte) *memoryBinaryDocValues {
	docIDs := make([]int, 0, len(values))
	for docID := range values {
		docIDs = append(docIDs, docID)
	}
	util.IntroSortOrdered(docIDs)

	return &memoryBinaryDocValues{
		values:  values,
		docIDs:  docIDs,
		pos:     -1,
		currDoc: -1,
	}
}

// DocID returns the current document ID.
func (dv *memoryBinaryDocValues) DocID() int {
	return dv.currDoc
}

// NextDoc advances to the next document that has a value.
func (dv *memoryBinaryDocValues) NextDoc() (int, error) {
	dv.pos++
	if dv.pos >= len(dv.docIDs) {
		dv.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	dv.currDoc = dv.docIDs[dv.pos]
	return dv.currDoc, nil
}

// Advance advances to the first document >= target that has a value.
func (dv *memoryBinaryDocValues) Advance(target int) (int, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] >= target {
			dv.currDoc = dv.docIDs[dv.pos]
			return dv.currDoc, nil
		}
	}
	dv.currDoc = index.NO_MORE_DOCS
	return index.NO_MORE_DOCS, nil
}

// BinaryValue returns the current document's value.
func (dv *memoryBinaryDocValues) BinaryValue() ([]byte, error) {
	if dv.currDoc == index.NO_MORE_DOCS {
		return nil, fmt.Errorf("no more documents")
	}
	if val, ok := dv.values[dv.currDoc]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("no value for doc %d", dv.currDoc)
}

// Cost returns an estimate of the cost.
func (dv *memoryBinaryDocValues) Cost() int64 {
	return int64(len(dv.docIDs))
}

// memorySortedDocValues is an in-memory implementation of SortedDocValues.
type memorySortedDocValues struct {
	*memoryNumericDocValues
	ordToValue map[int][]byte
	valueToOrd map[string]int
}

// NewMemorySortedDocValues creates a new memorySortedDocValues.
func NewMemorySortedDocValues(values map[int][]byte) *memorySortedDocValues {
	// Build ord mapping
	ordToValue := make(map[int][]byte)
	valueToOrd := make(map[string]int)
	numericValues := make(map[int]int64)

	ord := 0
	for docID, val := range values {
		valStr := string(val)
		if _, ok := valueToOrd[valStr]; !ok {
			valueToOrd[valStr] = ord
			ordToValue[ord] = val
			ord++
		}
		numericValues[docID] = int64(valueToOrd[valStr])
	}

	return &memorySortedDocValues{
		memoryNumericDocValues: NewMemoryNumericDocValues(numericValues),
		ordToValue:             ordToValue,
		valueToOrd:             valueToOrd,
	}
}

// OrdValue returns the ordinal of the current document's value.
func (dv *memorySortedDocValues) OrdValue() (int, error) {
	return int(dv.memoryNumericDocValues.values[dv.currDoc]), nil
}

// LookupOrd returns the value for the given ordinal.
func (dv *memorySortedDocValues) LookupOrd(ord int) ([]byte, error) {
	if val, ok := dv.ordToValue[ord]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("invalid ordinal: %d", ord)
}

// GetValueCount returns the number of unique values.
func (dv *memorySortedDocValues) GetValueCount() int {
	return len(dv.ordToValue)
}

// memorySortedSetDocValues is an in-memory implementation of SortedSetDocValues.
type memorySortedSetDocValues struct {
	values   map[int][]int // docID -> list of ords
	docIDs   []int
	pos      int
	currDoc  int
	currOrd  int
	ordToValue map[int][]byte
}

// NewMemorySortedSetDocValues creates a new memorySortedSetDocValues.
func NewMemorySortedSetDocValues(values map[int][][]byte) *memorySortedSetDocValues {
	// Build ord mapping
	ordToValue := make(map[int][]byte)
	valueToOrd := make(map[string]int)
	ordsMap := make(map[int][]int)

	ord := 0
	for docID, vals := range values {
		ords := make([]int, len(vals))
		for i, val := range vals {
			valStr := string(val)
			if existingOrd, ok := valueToOrd[valStr]; ok {
				ords[i] = existingOrd
			} else {
				valueToOrd[valStr] = ord
				ordToValue[ord] = val
				ords[i] = ord
				ord++
			}
		}
		ordsMap[docID] = ords
	}

	docIDs := make([]int, 0, len(values))
	for docID := range values {
		docIDs = append(docIDs, docID)
	}
	util.IntroSortOrdered(docIDs)

	return &memorySortedSetDocValues{
		values:     ordsMap,
		docIDs:     docIDs,
		pos:        -1,
		currDoc:    -1,
		currOrd:    -1,
		ordToValue: ordToValue,
	}
}

// DocID returns the current document ID.
func (dv *memorySortedSetDocValues) DocID() int {
	return dv.currDoc
}

// NextDoc advances to the next document that has values.
func (dv *memorySortedSetDocValues) NextDoc() (int, error) {
	dv.pos++
	if dv.pos >= len(dv.docIDs) {
		dv.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	dv.currDoc = dv.docIDs[dv.pos]
	dv.currOrd = -1
	return dv.currDoc, nil
}

// Advance advances to the first document >= target that has values.
func (dv *memorySortedSetDocValues) Advance(target int) (int, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] >= target {
			dv.currDoc = dv.docIDs[dv.pos]
			dv.currOrd = -1
			return dv.currDoc, nil
		}
	}
	dv.currDoc = index.NO_MORE_DOCS
	return index.NO_MORE_DOCS, nil
}

// NextOrd advances to the next ordinal for the current document.
func (dv *memorySortedSetDocValues) NextOrd() (int, error) {
	if dv.currDoc == index.NO_MORE_DOCS {
		return -1, nil
	}
	dv.currOrd++
	if ords, ok := dv.values[dv.currDoc]; ok {
		if dv.currOrd < len(ords) {
			return ords[dv.currOrd], nil
		}
	}
	return -1, nil
}

// LookupOrd returns the value for the given ordinal.
func (dv *memorySortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	if val, ok := dv.ordToValue[ord]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("invalid ordinal: %d", ord)
}

// GetValueCount returns the number of unique values.
func (dv *memorySortedSetDocValues) GetValueCount() int {
	return len(dv.ordToValue)
}

// Cost returns an estimate of the cost.
func (dv *memorySortedSetDocValues) Cost() int64 {
	return int64(len(dv.docIDs))
}

// memorySortedNumericDocValues is an in-memory implementation of SortedNumericDocValues.
type memorySortedNumericDocValues struct {
	*memoryNumericDocValues
	values map[int][]int64 // docID -> list of values
	pos    int
}

// NewMemorySortedNumericDocValues creates a new memorySortedNumericDocValues.
func NewMemorySortedNumericDocValues(values map[int][]int64) *memorySortedNumericDocValues {
	// For the base, we use the first value of each doc
	numericValues := make(map[int]int64)
	docIDs := make([]int, 0, len(values))
	for docID, vals := range values {
		docIDs = append(docIDs, docID)
		if len(vals) > 0 {
			numericValues[docID] = vals[0]
		}
	}
	util.IntroSortOrdered(docIDs)

	return &memorySortedNumericDocValues{
		memoryNumericDocValues: NewMemoryNumericDocValues(numericValues),
		values:                 values,
		pos:                    -1,
	}
}

// NextValue advances to the next value for the current document.
func (dv *memorySortedNumericDocValues) NextValue() (int64, error) {
	if dv.currDoc == index.NO_MORE_DOCS {
		return 0, fmt.Errorf("no more documents")
	}
	dv.pos++
	if vals, ok := dv.values[dv.currDoc]; ok {
		if dv.pos < len(vals) {
			return vals[dv.pos], nil
		}
	}
	return 0, fmt.Errorf("no more values for doc %d", dv.currDoc)
}

// DocValueCount returns the number of values for the current document.
func (dv *memorySortedNumericDocValues) DocValueCount() (int, error) {
	if dv.currDoc == index.NO_MORE_DOCS {
		return 0, fmt.Errorf("no more documents")
	}
	if vals, ok := dv.values[dv.currDoc]; ok {
		return len(vals), nil
	}
	return 0, nil
}
