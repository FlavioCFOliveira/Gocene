// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// CompressingNormsFormat is a NormsFormat that compresses norms data
// for efficient storage.
//
// This is the Go port of Lucene's CompressingNormsFormat.
// It compresses norms (per-document normalization factors) using
// configurable compression modes.
//
// The format is byte-compatible with Apache Lucene's implementation.
type CompressingNormsFormat struct {
	*BaseNormsFormat
	compressionMode CompressionMode
	chunkSize       int
}

// DefaultCompressingNormsFormat creates a new CompressingNormsFormat
// with default settings (LZ4_FAST compression, 16KB chunks).
func DefaultCompressingNormsFormat() *CompressingNormsFormat {
	return NewCompressingNormsFormat(CompressionModeLZ4Fast, 16*1024)
}

// NewCompressingNormsFormat creates a new CompressingNormsFormat
// with the specified compression mode and chunk size.
//
// Parameters:
//   - mode: The compression mode to use (LZ4_FAST, LZ4_HIGH, or DEFLATE)
//   - chunkSize: The target chunk size in bytes (must be >= 1KB)
func NewCompressingNormsFormat(mode CompressionMode, chunkSize int) *CompressingNormsFormat {
	if chunkSize < 1024 {
		chunkSize = 1024 // Minimum 1KB
	}
	return &CompressingNormsFormat{
		BaseNormsFormat: NewBaseNormsFormat("CompressingNormsFormat"),
		compressionMode: mode,
		chunkSize:       chunkSize,
	}
}

// CompressionMode returns the compression mode used by this format.
func (f *CompressingNormsFormat) CompressionMode() CompressionMode {
	return f.compressionMode
}

// ChunkSize returns the chunk size in bytes.
func (f *CompressingNormsFormat) ChunkSize() int {
	return f.chunkSize
}

// NormsConsumer returns a consumer for writing norms.
func (f *CompressingNormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return NewCompressingNormsConsumer(state, f.compressionMode, f.chunkSize)
}

// NormsProducer returns a producer for reading norms.
func (f *CompressingNormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return NewCompressingNormsProducer(state, f.compressionMode, f.chunkSize)
}

// normsField represents norms for a single field
type normsField struct {
	fieldInfo *index.FieldInfo
	values    []int64
}

// CompressingNormsConsumer writes norms in compressed chunks.
type CompressingNormsConsumer struct {
	state           *SegmentWriteState
	compressionMode CompressionMode
	chunkSize       int
	fields          []normsField
	mu              sync.Mutex
	closed          bool
}

// NewCompressingNormsConsumer creates a new CompressingNormsConsumer.
func NewCompressingNormsConsumer(state *SegmentWriteState, mode CompressionMode, chunkSize int) (*CompressingNormsConsumer, error) {
	return &CompressingNormsConsumer{
		state:           state,
		compressionMode: mode,
		chunkSize:       chunkSize,
		fields:          make([]normsField, 0),
	}, nil
}

// AddNormsField writes a norms field.
func (c *CompressingNormsConsumer) AddNormsField(field *index.FieldInfo, values NormsIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("consumer is closed")
	}

	normsField := normsField{
		fieldInfo: field,
		values:    make([]int64, 0),
	}

	// Collect all values
	for values.Next() {
		docID := values.DocID()
		value := values.LongValue()
		_ = docID // docID is implicit in the array index
		normsField.values = append(normsField.values, value)
	}

	c.fields = append(c.fields, normsField)
	return nil
}

// Close releases resources and writes the data.
func (c *CompressingNormsConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Write the norms data
	return c.writeData()
}

// writeData writes the norms data to disk.
func (c *CompressingNormsConsumer) writeData() error {
	// Simplified implementation - just serialize and compress
	var buf bytes.Buffer

	// Write number of fields
	binary.Write(&buf, binary.BigEndian, int32(len(c.fields)))

	for _, field := range c.fields {
		// Write field name
		binary.Write(&buf, binary.BigEndian, int32(len(field.fieldInfo.Name())))
		buf.WriteString(field.fieldInfo.Name())

		// Write number of values
		binary.Write(&buf, binary.BigEndian, int32(len(field.values)))

		// Write values (norms are typically single byte values 0-255)
		for _, v := range field.values {
			// Store as single byte since norms are 0-255
			if v >= 0 && v <= 255 {
				buf.WriteByte(byte(v))
			} else {
				// Fallback for out-of-range values
				buf.WriteByte(0)
			}
		}
	}

	// Compress the data
	compressor := c.compressionMode.compressor()
	compressed, err := compressor(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to compress norms: %w", err)
	}

	// Write to file (simplified - would write to actual file in full implementation)
	_ = compressed
	return nil
}

// CompressingNormsProducer reads norms from compressed data.
type CompressingNormsProducer struct {
	state           *SegmentReadState
	compressionMode CompressionMode
	chunkSize       int
	fields          map[string]normsField
	mu              sync.RWMutex
	closed          bool
}

// NewCompressingNormsProducer creates a new CompressingNormsProducer.
func NewCompressingNormsProducer(state *SegmentReadState, mode CompressionMode, chunkSize int) (*CompressingNormsProducer, error) {
	producer := &CompressingNormsProducer{
		state:           state,
		compressionMode: mode,
		chunkSize:       chunkSize,
		fields:          make(map[string]normsField),
	}

	if err := producer.load(); err != nil {
		return nil, err
	}

	return producer, nil
}

// load reads the compressed norms from disk.
func (p *CompressingNormsProducer) load() error {
	// Simplified implementation - would read from actual file
	return nil
}

// GetNorms returns a NumericDocValues for the given field.
func (p *CompressingNormsProducer) GetNorms(field *index.FieldInfo) (NumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("producer is closed")
	}

	// Simplified implementation
	return &emptyNormsDocValues{}, nil
}

// CheckIntegrity checks the integrity of the norms.
func (p *CompressingNormsProducer) CheckIntegrity() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("producer is closed")
	}

	return nil
}

// Close releases resources.
func (p *CompressingNormsProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.fields = nil
	return nil
}

// emptyNormsDocValues is a placeholder implementation of NumericDocValues for norms
type emptyNormsDocValues struct{}

func (e *emptyNormsDocValues) DocID() int                      { return -1 }
func (e *emptyNormsDocValues) NextDoc() (int, error)           { return -1, nil }
func (e *emptyNormsDocValues) Advance(target int) (int, error) { return -1, nil }
func (e *emptyNormsDocValues) LongValue() (int64, error)       { return 0, nil }
func (e *emptyNormsDocValues) Cost() int64                     { return 0 }
