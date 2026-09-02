// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// MemoryDocValuesWriter is an in-memory implementation of DocValuesWriter.
// Used for testing and simple use cases.
type MemoryDocValuesWriter struct {
	docValuesType DocValuesType
	numericValues map[int]int64
	binaryValues  map[int][]byte
	sortedValues  map[int]int // docID -> ord
	ordsToValues  map[int][]byte
	nextOrd       int
}

// NewMemoryDocValuesWriter creates a new MemoryDocValuesWriter.
func NewMemoryDocValuesWriter(docValuesType DocValuesType) *MemoryDocValuesWriter {
	return &MemoryDocValuesWriter{
		docValuesType: docValuesType,
		numericValues: make(map[int]int64),
		binaryValues:  make(map[int][]byte),
		sortedValues:  make(map[int]int),
		ordsToValues:  make(map[int][]byte),
		nextOrd:       0,
	}
}

// AddValue adds a value for the specified document.
func (w *MemoryDocValuesWriter) AddValue(docID int, value interface{}) error {
	switch w.docValuesType {
	case DocValuesTypeNumeric:
		v, ok := value.(int64)
		if !ok {
			return fmt.Errorf("expected int64 for numeric DocValues, got %T", value)
		}
		w.numericValues[docID] = v

	case DocValuesTypeBinary:
		v, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte for binary DocValues, got %T", value)
		}
		w.binaryValues[docID] = v

	case DocValuesTypeSorted:
		v, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte for sorted DocValues, got %T", value)
		}
		// Check if value already exists
		ord := -1
		for i, existing := range w.ordsToValues {
			if string(existing) == string(v) {
				ord = i
				break
			}
		}
		if ord == -1 {
			ord = w.nextOrd
			w.ordsToValues[ord] = v
			w.nextOrd++
		}
		w.sortedValues[docID] = ord

	default:
		return fmt.Errorf("unsupported DocValuesType: %v", w.docValuesType)
	}
	return nil
}

// Finish finalizes writing and returns metadata.
func (w *MemoryDocValuesWriter) Finish() (*DocValuesMetadata, error) {
	meta := &DocValuesMetadata{
		Type:         w.docValuesType,
		UniqueValues: w.nextOrd,
	}

	switch w.docValuesType {
	case DocValuesTypeNumeric:
		meta.NumDocs = len(w.numericValues)
		// Calculate min/max
		for _, v := range w.numericValues {
			if v < meta.MinValue || meta.NumDocs == 1 {
				meta.MinValue = v
			}
			if v > meta.MaxValue || meta.NumDocs == 1 {
				meta.MaxValue = v
			}
		}
	case DocValuesTypeBinary:
		meta.NumDocs = len(w.binaryValues)
	case DocValuesTypeSorted:
		meta.NumDocs = len(w.sortedValues)
	}

	return meta, nil
}

// GetDocValuesType returns the type of DocValues being written.
func (w *MemoryDocValuesWriter) GetDocValuesType() DocValuesType {
	return w.docValuesType
}

// MemoryDocValuesReader is an in-memory implementation of DocValuesReader.
type MemoryDocValuesReader struct {
	docValuesType DocValuesType
	numericValues map[int]int64
	binaryValues  map[int][]byte
	sortedValues  map[int]int // docID -> ord
	ordsToValues  map[int][]byte
}

// NewMemoryDocValuesReader creates a new MemoryDocValuesReader from a writer.
func NewMemoryDocValuesReader(writer *MemoryDocValuesWriter) *MemoryDocValuesReader {
	return &MemoryDocValuesReader{
		docValuesType: writer.docValuesType,
		numericValues: writer.numericValues,
		binaryValues:  writer.binaryValues,
		sortedValues:  writer.sortedValues,
		ordsToValues:  writer.ordsToValues,
	}
}

// GetValue returns the value for the specified document.
func (r *MemoryDocValuesReader) GetValue(docID int) (interface{}, error) {
	switch r.docValuesType {
	case DocValuesTypeNumeric:
		if v, ok := r.numericValues[docID]; ok {
			return v, nil
		}
		return int64(0), nil

	case DocValuesTypeBinary:
		if v, ok := r.binaryValues[docID]; ok {
			return v, nil
		}
		return []byte{}, nil

	case DocValuesTypeSorted:
		if ord, ok := r.sortedValues[docID]; ok {
			if v, ok2 := r.ordsToValues[ord]; ok2 {
				return v, nil
			}
		}
		return []byte{}, nil

	default:
		return nil, fmt.Errorf("unsupported DocValuesType: %v", r.docValuesType)
	}
}

// GetDocValuesType returns the type of DocValues being read.
func (r *MemoryDocValuesReader) GetDocValuesType() DocValuesType {
	return r.docValuesType
}

// Close closes the reader and releases resources.
func (r *MemoryDocValuesReader) Close() error {
	// Nothing to close for in-memory implementation
	return nil
}

// MemoryNumericDocValuesReader is an in-memory NumericDocValuesReader.
type MemoryNumericDocValuesReader struct {
	*MemoryDocValuesReader
}

// Get returns the numeric value for the document.
func (r *MemoryNumericDocValuesReader) Get(docID int) (int64, error) {
	if v, ok := r.numericValues[docID]; ok {
		return v, nil
	}
	return 0, nil
}

// MemoryBinaryDocValuesReader is an in-memory BinaryDocValuesReader.
type MemoryBinaryDocValuesReader struct {
	*MemoryDocValuesReader
}

// Get returns the binary value for the document.
func (r *MemoryBinaryDocValuesReader) Get(docID int) ([]byte, error) {
	if v, ok := r.binaryValues[docID]; ok {
		return v, nil
	}
	return nil, nil
}

// MemorySortedDocValuesReader is an in-memory SortedDocValuesReader.
type MemorySortedDocValuesReader struct {
	*MemoryDocValuesReader
}

// GetOrd returns the ord for the document.
func (r *MemorySortedDocValuesReader) GetOrd(docID int) (int, error) {
	if ord, ok := r.sortedValues[docID]; ok {
		return ord, nil
	}
	return -1, nil
}

// LookupOrd returns the value for the given ord.
func (r *MemorySortedDocValuesReader) LookupOrd(ord int) ([]byte, error) {
	if v, ok := r.ordsToValues[ord]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("ord %d not found", ord)
}

// GetValueCount returns the number of unique values.
func (r *MemorySortedDocValuesReader) GetValueCount() int {
	return len(r.ordsToValues)
}
