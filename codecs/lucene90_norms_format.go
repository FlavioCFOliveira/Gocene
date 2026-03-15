// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90NormsFormat is the Lucene 9.0 norms format.
//
// This format stores field norms as compressed numeric values.
// Each norm is a single byte (0-255) that encodes the field's boost
// and length normalization.
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene90.Lucene90NormsFormat.
type Lucene90NormsFormat struct {
	*BaseNormsFormat
}

// NewLucene90NormsFormat creates a new Lucene90NormsFormat.
func NewLucene90NormsFormat() *Lucene90NormsFormat {
	return &Lucene90NormsFormat{
		BaseNormsFormat: NewBaseNormsFormat("Lucene90NormsFormat"),
	}
}

// NormsConsumer returns a consumer for writing norms.
func (f *Lucene90NormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return NewLucene90NormsConsumer(state), nil
}

// NormsProducer returns a producer for reading norms.
func (f *Lucene90NormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return NewLucene90NormsProducer(state)
}

// Lucene90NormsConsumer writes norms in Lucene 9.0 format.
type Lucene90NormsConsumer struct {
	state  *SegmentWriteState
	closed bool
}

// NewLucene90NormsConsumer creates a new Lucene90NormsConsumer.
func NewLucene90NormsConsumer(state *SegmentWriteState) *Lucene90NormsConsumer {
	return &Lucene90NormsConsumer{
		state: state,
	}
}

// AddNormsField writes a norms field.
func (c *Lucene90NormsConsumer) AddNormsField(field *index.FieldInfo, values NormsIterator) error {
	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	// Generate file name
	segmentName := c.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.nrm", segmentName)

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
	var norms []byte
	for values.Next() {
		docIDs = append(docIDs, values.DocID())
		// Norms are stored as a single byte (0-255)
		normValue := byte(values.LongValue() & 0xFF)
		norms = append(norms, normValue)
	}

	// Write field metadata
	if err := store.WriteVInt(out, int32(len(docIDs))); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write doc IDs and norms
	for i, docID := range docIDs {
		if err := store.WriteVInt(out, int32(docID)); err != nil {
			return fmt.Errorf("failed to write doc id: %w", err)
		}
		if err := out.WriteByte(norms[i]); err != nil {
			return fmt.Errorf("failed to write norm: %w", err)
		}
	}

	return nil
}

// writeHeader writes the norms file header.
func (c *Lucene90NormsConsumer) writeHeader(out store.IndexOutput) error {
	// Write magic number (NRM = Norms)
	if err := store.WriteUint32(out, 0x4E524D00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, 90); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close releases resources.
func (c *Lucene90NormsConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// Lucene90NormsProducer reads norms in Lucene 9.0 format.
type Lucene90NormsProducer struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90NormsProducer creates a new Lucene90NormsProducer.
func NewLucene90NormsProducer(state *SegmentReadState) (*Lucene90NormsProducer, error) {
	p := &Lucene90NormsProducer{
		state: state,
	}
	return p, nil
}

// GetNorms returns a NumericDocValues for the given field.
func (p *Lucene90NormsProducer) GetNorms(field *index.FieldInfo) (NumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	// Generate file name
	segmentName := p.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.nrm", segmentName)

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
		norm, err := in.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read norm: %w", err)
		}
		values[int(docID)] = int64(norm)
	}

	return NewMemoryNumericDocValues(values), nil
}

// readHeader reads and validates the norms file header.
func (p *Lucene90NormsProducer) readHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x4E524D00 {
		return fmt.Errorf("invalid magic number: expected 0x4E524D00, got 0x%08x", magic)
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

// CheckIntegrity checks the integrity of the norms.
func (p *Lucene90NormsProducer) CheckIntegrity() error {
	return nil
}

// Close releases resources.
func (p *Lucene90NormsProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}
