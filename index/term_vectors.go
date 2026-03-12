// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"
)

// TermVector represents the term vector for a single field in a document.
// It contains all terms, their frequencies, and positions in the field.
type TermVector struct {
	Field          string
	Terms          []string
	TermFreqs      []int
	Positions      [][]int
	StartOffsets   [][]int
	EndOffsets     [][]int
}

// NewTermVector creates a new TermVector for the given field.
func NewTermVector(field string) *TermVector {
	return &TermVector{
		Field:        field,
		Terms:        make([]string, 0),
		TermFreqs:    make([]int, 0),
		Positions:    make([][]int, 0),
		StartOffsets: make([][]int, 0),
		EndOffsets:   make([][]int, 0),
	}
}

// AddTerm adds a term occurrence to the vector.
func (tv *TermVector) AddTerm(term string, freq int, positions, startOffsets, endOffsets []int) {
	tv.Terms = append(tv.Terms, term)
	tv.TermFreqs = append(tv.TermFreqs, freq)
	tv.Positions = append(tv.Positions, positions)
	tv.StartOffsets = append(tv.StartOffsets, startOffsets)
	tv.EndOffsets = append(tv.EndOffsets, endOffsets)
}

// HasPositions returns true if this term vector has position information.
func (tv *TermVector) HasPositions() bool {
	return len(tv.Positions) > 0 && len(tv.Positions[0]) > 0
}

// HasOffsets returns true if this term vector has offset information.
func (tv *TermVector) HasOffsets() bool {
	return len(tv.StartOffsets) > 0 && len(tv.StartOffsets[0]) > 0
}

// GetTermFreq returns the frequency of the given term, or 0 if not found.
func (tv *TermVector) GetTermFreq(term string) int {
	for i, t := range tv.Terms {
		if t == term {
			return tv.TermFreqs[i]
		}
	}
	return 0
}

// String returns a string representation of the term vector.
func (tv *TermVector) String() string {
	return fmt.Sprintf("TermVector(field=%s, terms=%d)", tv.Field, len(tv.Terms))
}

// TermVectorsWriter writes term vectors to the index.
type TermVectorsWriter interface {
	// StartDocument starts writing term vectors for a new document.
	StartDocument(docID int) error

	// StartField starts writing a term vector for the given field.
	StartField(field string, hasPositions, hasOffsets bool) error

	// AddTerm adds a term to the current field.
	AddTerm(term []byte, freq int, positions, startOffsets, endOffsets []int) error

	// FinishField finishes writing the current field.
	FinishField() error

	// FinishDocument finishes writing the current document.
	FinishDocument() error

	// Close closes the writer.
	Close() error
}

// TermVectorsReader reads term vectors from the index.
type TermVectorsReader interface {
	// Get retrieves term vectors for the given document ID.
	// Returns a map of field name to TermVector.
	Get(docID int) (map[string]*TermVector, error)

	// GetField retrieves the term vector for a specific field.
	GetField(docID int, field string) (*TermVector, error)

	// Close closes the reader.
	Close() error
}

// MemoryTermVectorsWriter is an in-memory implementation of TermVectorsWriter.
type MemoryTermVectorsWriter struct {
	documents    map[int]map[string]*TermVector
	currentDocID int
	currentField *TermVector
	hasPositions bool
	hasOffsets   bool
}

// NewMemoryTermVectorsWriter creates a new MemoryTermVectorsWriter.
func NewMemoryTermVectorsWriter() *MemoryTermVectorsWriter {
	return &MemoryTermVectorsWriter{
		documents: make(map[int]map[string]*TermVector),
	}
}

// StartDocument starts writing term vectors for a new document.
func (w *MemoryTermVectorsWriter) StartDocument(docID int) error {
	w.currentDocID = docID
	w.documents[docID] = make(map[string]*TermVector)
	return nil
}

// StartField starts writing a term vector for the given field.
func (w *MemoryTermVectorsWriter) StartField(field string, hasPositions, hasOffsets bool) error {
	w.currentField = NewTermVector(field)
	w.hasPositions = hasPositions
	w.hasOffsets = hasOffsets
	return nil
}

// AddTerm adds a term to the current field.
func (w *MemoryTermVectorsWriter) AddTerm(term []byte, freq int, positions, startOffsets, endOffsets []int) error {
	if w.currentField == nil {
		return fmt.Errorf("no field started")
	}
	w.currentField.AddTerm(string(term), freq, positions, startOffsets, endOffsets)
	return nil
}

// FinishField finishes writing the current field.
func (w *MemoryTermVectorsWriter) FinishField() error {
	if w.currentField == nil {
		return fmt.Errorf("no field to finish")
	}
	w.documents[w.currentDocID][w.currentField.Field] = w.currentField
	w.currentField = nil
	return nil
}

// FinishDocument finishes writing the current document.
func (w *MemoryTermVectorsWriter) FinishDocument() error {
	w.currentDocID = -1
	w.currentField = nil
	return nil
}

// Close closes the writer.
func (w *MemoryTermVectorsWriter) Close() error {
	return nil
}

// GetDocument returns the term vectors for a document (for testing).
func (w *MemoryTermVectorsWriter) GetDocument(docID int) (map[string]*TermVector, bool) {
	vectors, ok := w.documents[docID]
	return vectors, ok
}

// MemoryTermVectorsReader is an in-memory implementation of TermVectorsReader.
type MemoryTermVectorsReader struct {
	writer *MemoryTermVectorsWriter
}

// NewMemoryTermVectorsReader creates a new MemoryTermVectorsReader from a writer.
func NewMemoryTermVectorsReader(writer *MemoryTermVectorsWriter) *MemoryTermVectorsReader {
	return &MemoryTermVectorsReader{writer: writer}
}

// Get retrieves term vectors for the given document ID.
func (r *MemoryTermVectorsReader) Get(docID int) (map[string]*TermVector, error) {
	vectors, ok := r.writer.GetDocument(docID)
	if !ok {
		return nil, fmt.Errorf("document %d not found", docID)
	}
	return vectors, nil
}

// GetField retrieves the term vector for a specific field.
func (r *MemoryTermVectorsReader) GetField(docID int, field string) (*TermVector, error) {
	vectors, ok := r.writer.GetDocument(docID)
	if !ok {
		return nil, fmt.Errorf("document %d not found", docID)
	}
	vector, ok := vectors[field]
	if !ok {
		return nil, fmt.Errorf("field %s not found in document %d", field, docID)
	}
	return vector, nil
}

// Close closes the reader.
func (r *MemoryTermVectorsReader) Close() error {
	return nil
}

// TermVectorsFormat handles the encoding/decoding of term vectors.
type TermVectorsFormat struct {
	// Version of the format
	Version int
}

// NewTermVectorsFormat creates a new TermVectorsFormat with the default version.
func NewTermVectorsFormat() *TermVectorsFormat {
	return &TermVectorsFormat{Version: 1}
}

// TermVectorsMetadata holds metadata about term vectors for a segment.
type TermVectorsMetadata struct {
	NumDocuments int
	NumFields    int
	HasPositions bool
	HasOffsets   bool
}

// TermFreqVector represents a simple term-frequency vector.
// This is a simplified version for basic use cases.
type TermFreqVector struct {
	Field  string
	Terms  []string
	Freqs  []int
}

// NewTermFreqVector creates a new TermFreqVector.
func NewTermFreqVector(field string) *TermFreqVector {
	return &TermFreqVector{
		Field: field,
		Terms: make([]string, 0),
		Freqs: make([]int, 0),
	}
}

// Add adds a term with its frequency.
func (tfv *TermFreqVector) Add(term string, freq int) {
	tfv.Terms = append(tfv.Terms, term)
	tfv.Freqs = append(tfv.Freqs, freq)
}

// IndexOf returns the index of the given term, or -1 if not found.
func (tfv *TermFreqVector) IndexOf(term string) int {
	for i, t := range tfv.Terms {
		if t == term {
			return i
		}
	}
	return -1
}

// String returns a string representation.
func (tfv *TermFreqVector) String() string {
	return fmt.Sprintf("TermFreqVector(field=%s, terms=%d)", tfv.Field, len(tfv.Terms))
}

// TermVectorMapper is used to map/filter term vectors during reading.
type TermVectorMapper interface {
	// Map is called for each term in the vector.
	// Returns true if the term should be included, false to skip.
	Map(term string, freq int, positions, startOffsets, endOffsets []int) bool
}

// FilterTermVector filters a term vector using the given mapper.
func FilterTermVector(tv *TermVector, mapper TermVectorMapper) *TermVector {
	filtered := NewTermVector(tv.Field)
	for i, term := range tv.Terms {
		if mapper.Map(term, tv.TermFreqs[i], tv.Positions[i], tv.StartOffsets[i], tv.EndOffsets[i]) {
			filtered.AddTerm(term, tv.TermFreqs[i], tv.Positions[i], tv.StartOffsets[i], tv.EndOffsets[i])
		}
	}
	return filtered
}

// BytesToTermVector converts encoded bytes to a TermVector.
// This is a placeholder for actual serialization.
func BytesToTermVector(data []byte) (*TermVector, error) {
	// Check for empty data
	if len(data) == 0 {
		return nil, fmt.Errorf("empty term vector data")
	}

	// Simple format: field\0term1\0freq1\0term2\0freq2...
	parts := bytes.Split(data, []byte{0})
	if len(parts) < 1 || (len(parts) == 1 && len(parts[0]) == 0) {
		return nil, fmt.Errorf("invalid term vector data")
	}

	tv := NewTermVector(string(parts[0]))
	for i := 1; i+1 < len(parts); i += 2 {
		term := string(parts[i])
		freq := 0
		fmt.Sscanf(string(parts[i+1]), "%d", &freq)
		tv.AddTerm(term, freq, nil, nil, nil)
	}
	return tv, nil
}

// TermVectorToBytes converts a TermVector to encoded bytes.
// This is a placeholder for actual serialization.
func TermVectorToBytes(tv *TermVector) []byte {
	var buf bytes.Buffer
	buf.WriteString(tv.Field)
	buf.WriteByte(0)
	for i, term := range tv.Terms {
		buf.WriteString(term)
		buf.WriteByte(0)
		buf.WriteString(fmt.Sprintf("%d", tv.TermFreqs[i]))
		buf.WriteByte(0)
	}
	return buf.Bytes()
}
