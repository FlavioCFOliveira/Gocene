// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// NormsFormat handles encoding/decoding of field norms.
// This is the Go port of Lucene's org.apache.lucene.codecs.NormsFormat.
//
// Field norms are per-document normalization factors that are used during
// scoring. They are typically a single byte per document that encode
// the field's boost value and length normalization.
//
// Norms are stored as NumericDocValues internally, so the format is similar
// to doc values but optimized for single-byte values.
type NormsFormat interface {
	// Name returns the name of this format.
	Name() string

	// NormsConsumer returns a consumer for writing norms.
	// The caller should close the returned consumer when done.
	NormsConsumer(state *SegmentWriteState) (NormsConsumer, error)

	// NormsProducer returns a producer for reading norms.
	// The caller should close the returned producer when done.
	NormsProducer(state *SegmentReadState) (NormsProducer, error)
}

// BaseNormsFormat provides common functionality for NormsFormat implementations.
type BaseNormsFormat struct {
	name string
}

// NewBaseNormsFormat creates a new BaseNormsFormat.
func NewBaseNormsFormat(name string) *BaseNormsFormat {
	return &BaseNormsFormat{name: name}
}

// Name returns the format name.
func (f *BaseNormsFormat) Name() string {
	return f.name
}

// NormsConsumer returns a norms consumer (must be implemented by subclasses).
func (f *BaseNormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return nil, fmt.Errorf("NormsConsumer not implemented")
}

// NormsProducer returns a norms producer (must be implemented by subclasses).
func (f *BaseNormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return nil, fmt.Errorf("NormsProducer not implemented")
}

// NormsConsumer is a consumer for writing field norms.
// This is the Go port of Lucene's org.apache.lucene.codecs.NormsConsumer.
type NormsConsumer interface {
	// AddNormsField writes a norms field.
	// The values are provided through the iterator.
	AddNormsField(field *index.FieldInfo, values NormsIterator) error

	// Close releases resources.
	Close() error
}

// NormsProducer is a producer for reading field norms.
// This is the Go port of Lucene's org.apache.lucene.codecs.NormsProducer.
type NormsProducer interface {
	// GetNorms returns a NumericDocValues for the given field.
	// Returns nil if the field has no norms.
	GetNorms(field *index.FieldInfo) (NumericDocValues, error)

	// CheckIntegrity checks the integrity of the norms.
	CheckIntegrity() error

	// Close releases resources.
	Close() error
}

// NormsIterator is an iterator over norms for writing.
type NormsIterator interface {
	// Next advances to the next document.
	// Returns true if there is a next document.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// LongValue returns the current document's norm value.
	// Norms are typically stored as a single byte (0-255) but
	// are returned as int64 for consistency with NumericDocValues.
	LongValue() int64
}

// MemoryNormsProducer is an in-memory implementation of NormsProducer.
type MemoryNormsProducer struct {
	fields map[string]NumericDocValues
	mu     sync.RWMutex
	closed bool
}

// NewMemoryNormsProducer creates a new MemoryNormsProducer.
func NewMemoryNormsProducer() *MemoryNormsProducer {
	return &MemoryNormsProducer{
		fields: make(map[string]NumericDocValues),
	}
}

// GetNorms returns a NumericDocValues for the given field.
func (p *MemoryNormsProducer) GetNorms(field *index.FieldInfo) (NumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	if dv, ok := p.fields[field.Name()]; ok {
		return dv, nil
	}
	return nil, nil
}

// CheckIntegrity checks the integrity of the norms.
func (p *MemoryNormsProducer) CheckIntegrity() error {
	return nil
}

// Close releases resources.
func (p *MemoryNormsProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	p.fields = nil
	return nil
}

// SetNormsField sets a norms field for testing.
func (p *MemoryNormsProducer) SetNormsField(name string, dv NumericDocValues) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fields[name] = dv
}

// MemoryNormsConsumer is an in-memory implementation of NormsConsumer.
type MemoryNormsConsumer struct {
	fields map[string]map[int]int64
	mu     sync.Mutex
	closed bool
}

// NewMemoryNormsConsumer creates a new MemoryNormsConsumer.
func NewMemoryNormsConsumer() *MemoryNormsConsumer {
	return &MemoryNormsConsumer{
		fields: make(map[string]map[int]int64),
	}
}

// AddNormsField writes a norms field.
func (c *MemoryNormsConsumer) AddNormsField(field *index.FieldInfo, values NormsIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	fieldValues := make(map[int]int64)
	for values.Next() {
		fieldValues[values.DocID()] = values.LongValue()
	}
	c.fields[field.Name()] = fieldValues
	return nil
}

// Close releases resources.
func (c *MemoryNormsConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// ToProducer creates a MemoryNormsProducer from the consumed data.
func (c *MemoryNormsConsumer) ToProducer() *MemoryNormsProducer {
	c.mu.Lock()
	defer c.mu.Unlock()

	producer := NewMemoryNormsProducer()
	for name, values := range c.fields {
		producer.SetNormsField(name, NewMemoryNumericDocValues(values))
	}
	return producer
}

// NormsWriter is a helper for writing norms.
type NormsWriter struct {
	out    store.IndexOutput
	closed bool
}

// NewNormsWriter creates a new NormsWriter.
func NewNormsWriter(out store.IndexOutput) *NormsWriter {
	return &NormsWriter{out: out}
}

// WriteHeader writes the norms file header.
func (w *NormsWriter) WriteHeader() error {
	// Write magic number (NRM = Norms)
	if err := store.WriteUint32(w.out, 0x4E524D00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the writer.
func (w *NormsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// NormsReader is a helper for reading norms.
type NormsReader struct {
	in     store.IndexInput
	closed bool
}

// NewNormsReader creates a new NormsReader.
func NewNormsReader(in store.IndexInput) *NormsReader {
	return &NormsReader{in: in}
}

// ReadHeader reads and validates the norms file header.
func (r *NormsReader) ReadHeader() error {
	// Read magic number
	magic, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x4E524D00 {
		return fmt.Errorf("invalid magic number: expected 0x4E524D00, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the reader.
func (r *NormsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}
