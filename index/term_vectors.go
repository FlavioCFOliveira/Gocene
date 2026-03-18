// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectors provides access to term vectors for documents.
// This is the Go port of Lucene's org.apache.lucene.index.TermVectors.
//
// TermVectors allows retrieving term vectors for documents.
// It wraps a TermVectorsReader and provides thread-safe access.
type TermVectors interface {
	// Prefetch prefetches term vectors for the given document IDs.
	// This is a hint to the implementation that these documents
	// will likely be accessed soon.
	Prefetch(docIDs []int) error

	// Get retrieves the term vectors for a single document.
	// Returns a Fields object containing all term vectors for the document.
	Get(docID int) (Fields, error)

	// GetField retrieves the term vector for a specific field in a document.
	// Returns nil if the field has no term vector.
	GetField(docID int, field string) (Terms, error)
}

// TermVectorsImpl is an implementation of TermVectors that wraps a TermVectorsReader.
type TermVectorsImpl struct {
	reader   TermVectorsReader
	liveDocs util.Bits
	mu       sync.RWMutex
}

// NewTermVectors creates a new TermVectors from a TermVectorsReader.
func NewTermVectors(reader TermVectorsReader, liveDocs util.Bits) *TermVectorsImpl {
	return &TermVectorsImpl{
		reader:   reader,
		liveDocs: liveDocs,
	}
}

// Prefetch prefetches term vectors for the given document IDs.
func (tv *TermVectorsImpl) Prefetch(docIDs []int) error {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if tv.reader == nil {
		return fmt.Errorf("term vectors reader is closed")
	}

	// Default implementation: no-op
	// Subclasses can override to implement actual prefetching
	return nil
}

// Get retrieves the term vectors for a single document.
func (tv *TermVectorsImpl) Get(docID int) (Fields, error) {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if tv.reader == nil {
		return nil, fmt.Errorf("term vectors reader is closed")
	}

	// Check if document is live
	if tv.liveDocs != nil && !tv.liveDocs.Get(docID) {
		return nil, fmt.Errorf("document %d is deleted", docID)
	}

	return tv.reader.Get(docID)
}

// GetField retrieves the term vector for a specific field in a document.
func (tv *TermVectorsImpl) GetField(docID int, field string) (Terms, error) {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if tv.reader == nil {
		return nil, fmt.Errorf("term vectors reader is closed")
	}

	// Check if document is live
	if tv.liveDocs != nil && !tv.liveDocs.Get(docID) {
		return nil, fmt.Errorf("document %d is deleted", docID)
	}

	return tv.reader.GetField(docID, field)
}

// EmptyTermVectors is a TermVectors implementation with no term vectors.
type EmptyTermVectors struct{}

// NewEmptyTermVectors creates a new EmptyTermVectors.
func NewEmptyTermVectors() *EmptyTermVectors {
	return &EmptyTermVectors{}
}

// Prefetch does nothing.
func (e *EmptyTermVectors) Prefetch(docIDs []int) error {
	return nil
}

// Get returns nil (no term vectors).
func (e *EmptyTermVectors) Get(docID int) (Fields, error) {
	return nil, nil
}

// GetField returns nil (no term vectors).
func (e *EmptyTermVectors) GetField(docID int, field string) (Terms, error) {
	return nil, nil
}

// Ensure EmptyTermVectors implements TermVectors
var _ TermVectors = (*EmptyTermVectors)(nil)

// Ensure TermVectorsImpl implements TermVectors
var _ TermVectors = (*TermVectorsImpl)(nil)

// TermVector represents the term vector for a single field in a document.
// It contains all terms, their frequencies, and positions in the field.
type TermVector struct {
	Field        string
	Terms        []string
	TermFreqs    []int
	Positions    [][]int
	StartOffsets [][]int
	EndOffsets   [][]int
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
// Returns a Fields object containing all term vectors for the document.
func (r *MemoryTermVectorsReader) Get(docID int) (Fields, error) {
	vectors, ok := r.writer.GetDocument(docID)
	if !ok {
		return nil, fmt.Errorf("document %d not found", docID)
	}

	// Convert map[string]*TermVector to Fields
	fields := NewMemoryFields()
	for fieldName, tv := range vectors {
		// Create a Terms implementation for the TermVector
		terms := NewTermVectorTerms(tv)
		fields.AddField(fieldName, terms)
	}
	return fields, nil
}

// GetField retrieves the term vector for a specific field.
// Returns a Terms object for that field, or nil if the field doesn't exist.
func (r *MemoryTermVectorsReader) GetField(docID int, field string) (Terms, error) {
	vectors, ok := r.writer.GetDocument(docID)
	if !ok {
		return nil, fmt.Errorf("document %d not found", docID)
	}
	vector, ok := vectors[field]
	if !ok {
		return nil, nil // field not found, return nil (not an error)
	}
	return NewTermVectorTerms(vector), nil
}

// Close closes the reader.
func (r *MemoryTermVectorsReader) Close() error {
	return nil
}

// TermVectorTerms is a Terms implementation backed by a TermVector.
type TermVectorTerms struct {
	tv *TermVector
}

// NewTermVectorTerms creates a new Terms from a TermVector.
func NewTermVectorTerms(tv *TermVector) *TermVectorTerms {
	return &TermVectorTerms{tv: tv}
}

// GetIterator returns an iterator over all terms in this field.
func (t *TermVectorTerms) GetIterator() (TermsEnum, error) {
	return NewTermVectorTermsEnum(t.tv), nil
}

// GetIteratorWithSeek returns an iterator positioned at or after the given term.
func (t *TermVectorTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	iter := NewTermVectorTermsEnum(t.tv)
	// Seek to the term or after
	if seekTerm != nil {
		for {
			term, err := iter.Next()
			if err != nil {
				return nil, err
			}
			if term == nil || term.CompareTo(seekTerm) >= 0 {
				break
			}
		}
	}
	return iter, nil
}

// Size returns the number of unique terms.
func (t *TermVectorTerms) Size() int64 {
	return int64(len(t.tv.Terms))
}

// GetDocCount returns the number of documents containing at least one term.
func (t *TermVectorTerms) GetDocCount() (int, error) {
	return 1, nil // This is for a single document
}

// GetSumDocFreq returns the sum of docFreq across all terms.
func (t *TermVectorTerms) GetSumDocFreq() (int64, error) {
	// Each term appears in exactly one doc
	return int64(len(t.tv.Terms)), nil
}

// GetSumTotalTermFreq returns the total occurrences of all terms.
func (t *TermVectorTerms) GetSumTotalTermFreq() (int64, error) {
	var total int64
	for _, freq := range t.tv.TermFreqs {
		total += int64(freq)
	}
	return total, nil
}

// HasFreqs returns true if term frequencies are available.
func (t *TermVectorTerms) HasFreqs() bool {
	return len(t.tv.TermFreqs) > 0
}

// HasOffsets returns true if term offsets are available.
func (t *TermVectorTerms) HasOffsets() bool {
	return t.tv.HasOffsets()
}

// HasPositions returns true if term positions are available.
func (t *TermVectorTerms) HasPositions() bool {
	return t.tv.HasPositions()
}

// HasPayloads returns true if payloads are available.
func (t *TermVectorTerms) HasPayloads() bool {
	return false // TermVector doesn't support payloads
}

// GetMin returns the smallest term.
func (t *TermVectorTerms) GetMin() (*Term, error) {
	if len(t.tv.Terms) == 0 {
		return nil, nil
	}
	return NewTerm(t.tv.Field, t.tv.Terms[0]), nil
}

// GetMax returns the largest term.
func (t *TermVectorTerms) GetMax() (*Term, error) {
	if len(t.tv.Terms) == 0 {
		return nil, nil
	}
	return NewTerm(t.tv.Field, t.tv.Terms[len(t.tv.Terms)-1]), nil
}

// TermVectorTermsEnum is a TermsEnum implementation backed by a TermVector.
type TermVectorTermsEnum struct {
	tv    *TermVector
	index int
}

// NewTermVectorTermsEnum creates a new TermsEnum from a TermVector.
func NewTermVectorTermsEnum(tv *TermVector) *TermVectorTermsEnum {
	return &TermVectorTermsEnum{
		tv:    tv,
		index: -1,
	}
}

// Next advances to the next term.
func (e *TermVectorTermsEnum) Next() (*Term, error) {
	e.index++
	if e.index >= len(e.tv.Terms) {
		return nil, nil
	}
	return NewTerm(e.tv.Field, e.tv.Terms[e.index]), nil
}

// DocFreq returns the document frequency of the current term.
func (e *TermVectorTermsEnum) DocFreq() (int, error) {
	return 1, nil // In term vectors, each term appears in exactly one doc
}

// TotalTermFreq returns the total frequency of the current term.
func (e *TermVectorTermsEnum) TotalTermFreq() (int64, error) {
	if e.index < 0 || e.index >= len(e.tv.TermFreqs) {
		return 0, nil
	}
	return int64(e.tv.TermFreqs[e.index]), nil
}

// Postings returns a PostingsEnum for the current term.
func (e *TermVectorTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return nil, fmt.Errorf("postings not supported for term vectors")
}

// SeekCeil seeks to the specified term or, if the term doesn't exist,
// to the next term after it (ceiling).
func (e *TermVectorTermsEnum) SeekCeil(term *Term) (*Term, error) {
	if term == nil {
		return e.Next()
	}
	// Find the first term >= seek term
	for i, t := range e.tv.Terms {
		if t >= term.Text() {
			e.index = i - 1 // Will be incremented by Next
			return e.Next()
		}
	}
	return nil, nil
}

// SeekExact seeks to the specified term.
func (e *TermVectorTermsEnum) SeekExact(term *Term) (bool, error) {
	if term == nil {
		return false, nil
	}
	for i, t := range e.tv.Terms {
		if t == term.Text() {
			e.index = i
			return true, nil
		}
	}
	return false, nil
}

// Term returns the current term.
func (e *TermVectorTermsEnum) Term() *Term {
	if e.index < 0 || e.index >= len(e.tv.Terms) {
		return nil
	}
	return NewTerm(e.tv.Field, e.tv.Terms[e.index])
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term with live docs.
func (e *TermVectorTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return nil, fmt.Errorf("postings not supported for term vectors")
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
	Field string
	Terms []string
	Freqs []int
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
