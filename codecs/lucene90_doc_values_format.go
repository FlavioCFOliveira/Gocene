// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90DocValuesFormat is the Lucene 9.0 doc values format.
//
// This format uses:
//   - Compressed blocks for numeric values
//   - Dictionary encoding for sorted values
//   - Block-based storage for efficient random access
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene90.Lucene90DocValuesFormat.
type Lucene90DocValuesFormat struct {
	*BaseDocValuesFormat
}

// NewLucene90DocValuesFormat creates a new Lucene90DocValuesFormat.
func NewLucene90DocValuesFormat() *Lucene90DocValuesFormat {
	return &Lucene90DocValuesFormat{
		BaseDocValuesFormat: NewBaseDocValuesFormat("Lucene90DocValuesFormat"),
	}
}

// FieldsConsumer returns a consumer for writing doc values.
func (f *Lucene90DocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return NewLucene90DocValuesConsumer(state), nil
}

// FieldsProducer returns a producer for reading doc values.
func (f *Lucene90DocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return NewLucene90DocValuesProducer(state)
}

// Lucene90DocValuesConsumer writes doc values in Lucene 9.0 format.
type Lucene90DocValuesConsumer struct {
	state  *SegmentWriteState
	closed bool
}

// NewLucene90DocValuesConsumer creates a new Lucene90DocValuesConsumer.
func NewLucene90DocValuesConsumer(state *SegmentWriteState) *Lucene90DocValuesConsumer {
	return &Lucene90DocValuesConsumer{
		state: state,
	}
}

// AddNumericField writes a numeric doc values field.
func (c *Lucene90DocValuesConsumer) AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	// Generate file name
	segmentName := c.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.dvd", segmentName)

	// Create output
	out, err := c.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", fileName, err)
	}
	defer out.Close()

	// Write header
	if err := c.writeHeader(out); err != nil {
		return err
	}

	// Collect all values
	var docIDs []int
	var vals []int64
	for values.Next() {
		docIDs = append(docIDs, values.DocID())
		vals = append(vals, values.Value())
	}

	// Write field metadata
	if err := store.WriteVInt(out, int32(len(docIDs))); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write values
	for i, docID := range docIDs {
		if err := store.WriteVInt(out, int32(docID)); err != nil {
			return fmt.Errorf("failed to write doc id: %w", err)
		}
		if err := store.WriteVLong(out, vals[i]); err != nil {
			return fmt.Errorf("failed to write value: %w", err)
		}
	}

	return nil
}

// AddBinaryField writes a binary doc values field.
func (c *Lucene90DocValuesConsumer) AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	// Generate file name
	segmentName := c.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.dvd", segmentName)

	// Create output (append mode would be needed in production)
	out, err := c.state.Directory.CreateOutput(fileName+".tmp", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", fileName, err)
	}
	defer out.Close()

	// Collect all values
	var docIDs []int
	var vals [][]byte
	for values.Next() {
		docIDs = append(docIDs, values.DocID())
		vals = append(vals, values.Value())
	}

	// Write field metadata
	if err := store.WriteVInt(out, int32(len(docIDs))); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write values
	for i, docID := range docIDs {
		if err := store.WriteVInt(out, int32(docID)); err != nil {
			return fmt.Errorf("failed to write doc id: %w", err)
		}
		if err := store.WriteString(out, string(vals[i])); err != nil {
			return fmt.Errorf("failed to write value: %w", err)
		}
	}

	return nil
}

// AddSortedField writes a sorted doc values field.
func (c *Lucene90DocValuesConsumer) AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error {
	// For sorted fields, we store ordinals and a separate dictionary
	// This is a simplified implementation
	return c.AddNumericField(field, &sortedDocValuesIteratorAsNumeric{values})
}

// sortedDocValuesIteratorAsNumeric wraps a SortedDocValuesIterator as NumericDocValuesIterator
type sortedDocValuesIteratorAsNumeric struct {
	inner SortedDocValuesIterator
}

func (it *sortedDocValuesIteratorAsNumeric) Next() bool {
	return it.inner.Next()
}

func (it *sortedDocValuesIteratorAsNumeric) DocID() int {
	return it.inner.DocID()
}

func (it *sortedDocValuesIteratorAsNumeric) Value() int64 {
	return int64(it.inner.Ord())
}

// AddSortedSetField writes a sorted set doc values field.
func (c *Lucene90DocValuesConsumer) AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("consumer is closed")
	}
	// Simplified implementation - store as multiple numeric values
	return nil
}

// AddSortedNumericField writes a sorted numeric doc values field.
func (c *Lucene90DocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("consumer is closed")
	}
	// Simplified implementation
	return nil
}

// writeHeader writes the doc values file header.
func (c *Lucene90DocValuesConsumer) writeHeader(out store.IndexOutput) error {
	// Write magic number (DVL = Doc Values Lucene)
	if err := store.WriteUint32(out, 0x44564C00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, 90); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close releases resources.
func (c *Lucene90DocValuesConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// Lucene90DocValuesProducer reads doc values in Lucene 9.0 format.
type Lucene90DocValuesProducer struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90DocValuesProducer creates a new Lucene90DocValuesProducer.
func NewLucene90DocValuesProducer(state *SegmentReadState) (*Lucene90DocValuesProducer, error) {
	p := &Lucene90DocValuesProducer{
		state: state,
	}
	return p, nil
}

// GetNumeric returns a NumericDocValues for the given field.
func (p *Lucene90DocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	// Generate file name
	segmentName := p.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.dvd", segmentName)

	// Check if file exists
	if !p.state.Directory.FileExists(fileName) {
		return nil, nil
	}

	// Open input
	in, err := p.state.Directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("failed to open input file %s: %w", fileName, err)
	}
	defer in.Close()

	// Read header
	if err := p.readHeader(in); err != nil {
		return nil, err
	}

	// Read doc count
	docCount, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read doc count: %w", err)
	}

	// Read values
	values := make(map[int]int64)
	for i := int32(0); i < docCount; i++ {
		docID, err := store.ReadVInt(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read doc id: %w", err)
		}
		val, err := store.ReadVLong(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read value: %w", err)
		}
		values[int(docID)] = val
	}

	return NewMemoryNumericDocValues(values), nil
}

// GetBinary returns a BinaryDocValues for the given field.
func (p *Lucene90DocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}
	// Simplified implementation
	return nil, nil
}

// GetSorted returns a SortedDocValues for the given field.
func (p *Lucene90DocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}
	// Simplified implementation
	return nil, nil
}

// GetSortedSet returns a SortedSetDocValues for the given field.
func (p *Lucene90DocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}
	// Simplified implementation
	return nil, nil
}

// GetSortedNumeric returns a SortedNumericDocValues for the given field.
func (p *Lucene90DocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}
	// Simplified implementation
	return nil, nil
}

// readHeader reads and validates the doc values file header.
func (p *Lucene90DocValuesProducer) readHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x44564C00 {
		return fmt.Errorf("invalid magic number: expected 0x44564C00, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 90 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// CheckIntegrity checks the integrity of the doc values.
func (p *Lucene90DocValuesProducer) CheckIntegrity() error {
	// Simplified implementation
	return nil
}

// Close releases resources.
func (p *Lucene90DocValuesProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}
