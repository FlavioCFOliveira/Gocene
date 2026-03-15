// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MemoryDocValuesConsumer is an in-memory implementation of DocValuesConsumer.
// This stores doc values in memory and can be used to create a MemoryDocValuesProducer.
type MemoryDocValuesConsumer struct {
	numericFields       map[string]map[int]int64
	binaryFields        map[string]map[int][]byte
	sortedFields        map[string]map[int][]byte
	sortedSetFields     map[string]map[int][][]byte
	sortedNumericFields map[string]map[int][]int64
	mu                  sync.Mutex
	closed              bool
}

// NewMemoryDocValuesConsumer creates a new MemoryDocValuesConsumer.
func NewMemoryDocValuesConsumer() *MemoryDocValuesConsumer {
	return &MemoryDocValuesConsumer{
		numericFields:       make(map[string]map[int]int64),
		binaryFields:        make(map[string]map[int][]byte),
		sortedFields:        make(map[string]map[int][]byte),
		sortedSetFields:     make(map[string]map[int][][]byte),
		sortedNumericFields: make(map[string]map[int][]int64),
	}
}

// AddNumericField writes a numeric doc values field.
func (c *MemoryDocValuesConsumer) AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	fieldValues := make(map[int]int64)
	for values.Next() {
		fieldValues[values.DocID()] = values.Value()
	}
	c.numericFields[field.Name()] = fieldValues
	return nil
}

// AddBinaryField writes a binary doc values field.
func (c *MemoryDocValuesConsumer) AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	fieldValues := make(map[int][]byte)
	for values.Next() {
		// Make a copy of the value
		val := values.Value()
		fieldValues[values.DocID()] = append([]byte(nil), val...)
	}
	c.binaryFields[field.Name()] = fieldValues
	return nil
}

// AddSortedField writes a sorted doc values field.
func (c *MemoryDocValuesConsumer) AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	// For sorted fields, we need to get the actual values, not just ordinals
	// This is a simplified implementation - in production, we'd need to
	// deduplicate and sort values
	fieldValues := make(map[int][]byte)
	for values.Next() {
		// The iterator should provide the actual value, not just ord
		// For now, we'll store a placeholder
		fieldValues[values.DocID()] = []byte(fmt.Sprintf("ord_%d", values.Ord()))
	}
	c.sortedFields[field.Name()] = fieldValues
	return nil
}

// AddSortedSetField writes a sorted set doc values field.
func (c *MemoryDocValuesConsumer) AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	fieldValues := make(map[int][][]byte)
	for values.NextDoc() {
		docID := values.DocID()
		var vals [][]byte
		for {
			ord := values.NextOrd()
			if ord == -1 {
				break
			}
			vals = append(vals, []byte(fmt.Sprintf("ord_%d", ord)))
		}
		fieldValues[docID] = vals
	}
	c.sortedSetFields[field.Name()] = fieldValues
	return nil
}

// AddSortedNumericField writes a sorted numeric doc values field.
func (c *MemoryDocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	fieldValues := make(map[int][]int64)
	for values.NextDoc() {
		docID := values.DocID()
		var vals []int64
		count := values.DocValueCount()
		for i := 0; i < count; i++ {
			val := values.NextValue()
			vals = append(vals, val)
		}
		fieldValues[docID] = vals
	}
	c.sortedNumericFields[field.Name()] = fieldValues
	return nil
}

// Close releases resources and creates a producer from the collected data.
func (c *MemoryDocValuesConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// ToProducer creates a MemoryDocValuesProducer from the consumed data.
func (c *MemoryDocValuesConsumer) ToProducer() *MemoryDocValuesProducer {
	c.mu.Lock()
	defer c.mu.Unlock()

	producer := NewMemoryDocValuesProducer()

	// Convert numeric fields
	for name, values := range c.numericFields {
		producer.SetNumericField(name, NewMemoryNumericDocValues(values))
	}

	// Convert binary fields
	for name, values := range c.binaryFields {
		producer.SetBinaryField(name, NewMemoryBinaryDocValues(values))
	}

	// Convert sorted fields
	for name, values := range c.sortedFields {
		producer.SetSortedField(name, NewMemorySortedDocValues(values))
	}

	// Convert sorted set fields
	for name, values := range c.sortedSetFields {
		producer.SetSortedSetField(name, NewMemorySortedSetDocValues(values))
	}

	// Convert sorted numeric fields
	for name, values := range c.sortedNumericFields {
		producer.SetSortedNumericField(name, NewMemorySortedNumericDocValues(values))
	}

	return producer
}
