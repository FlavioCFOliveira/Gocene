// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// ShapeIOWriter provides efficient writing of spatial shape data to I/O streams.
// It supports multiple serialization formats (WKB, Spatial4j binary) and provides
// buffered writing for improved performance.
//
// The writer is designed to be used with the spatial index infrastructure,
// writing shape data that can be read by ShapeIOReader.
//
// This is the Go port of Lucene's ShapeIOWriter concept.
type ShapeIOWriter struct {
	// writer is the underlying writer
	writer io.Writer

	// bufWriter provides buffering for better performance
	bufWriter *bufio.Writer

	// byteOrder is the default byte order for binary writes
	byteOrder binary.ByteOrder

	// encoder is the geometry encoder for serializing shape data
	encoder ShapeEncoder

	// format specifies the serialization format
	format ShapeIOFormat

	// bytesWritten tracks the number of bytes written
	bytesWritten int64
}

// ShapeEncoder is the interface for encoding shapes to binary format.
type ShapeEncoder interface {
	// Encode serializes a shape to bytes.
	Encode(shape Shape) ([]byte, error)
}

// shapeEncoderAdapter wraps ShapeSerializer to implement ShapeEncoder
type shapeEncoderAdapter struct {
	serializer ShapeSerializer
}

// Encode implements ShapeEncoder by delegating to Serialize
func (a *shapeEncoderAdapter) Encode(shape Shape) ([]byte, error) {
	return a.serializer.Serialize(shape)
}

// spatial4jEncoder wraps Spatial4jShapeDecoder to implement ShapeEncoder
// using its EncodeToBytes method
type spatial4jEncoder struct {
	decoder *Spatial4jShapeDecoder
}

// Encode implements ShapeEncoder using Spatial4j's EncodeToBytes
func (e *spatial4jEncoder) Encode(shape Shape) ([]byte, error) {
	return e.decoder.EncodeToBytes(shape)
}

// NewShapeEncoderFromSerializer creates a ShapeEncoder from a ShapeSerializer
func NewShapeEncoderFromSerializer(serializer ShapeSerializer) ShapeEncoder {
	return &shapeEncoderAdapter{serializer: serializer}
}

// NewShapeEncoderFromSpatial4j creates a ShapeEncoder from a Spatial4jShapeDecoder
func NewShapeEncoderFromSpatial4j(decoder *Spatial4jShapeDecoder) ShapeEncoder {
	return &spatial4jEncoder{decoder: decoder}
}

// NewShapeIOWriter creates a new ShapeIOWriter for writing shape data.
// Uses WKB format by default with LittleEndian byte order.
//
// Parameters:
//   - writer: The underlying writer to write to
//
// Returns a new ShapeIOWriter instance.
func NewShapeIOWriter(writer io.Writer) *ShapeIOWriter {
	return &ShapeIOWriter{
		writer:    writer,
		bufWriter: bufio.NewWriter(writer),
		byteOrder: binary.LittleEndian,
		format:    ShapeIOFormatWKB,
		encoder:   NewShapeEncoderFromSerializer(NewJTSGeometrySerializer()),
	}
}

// NewShapeIOWriterWithBuffer creates a new ShapeIOWriter with a custom buffer size.
//
// Parameters:
//   - writer: The underlying writer
//   - bufferSize: The size of the write buffer in bytes
func NewShapeIOWriterWithBuffer(writer io.Writer, bufferSize int) *ShapeIOWriter {
	return &ShapeIOWriter{
		writer:    writer,
		bufWriter: bufio.NewWriterSize(writer, bufferSize),
		byteOrder: binary.LittleEndian,
		format:    ShapeIOFormatWKB,
		encoder:   NewShapeEncoderFromSerializer(NewJTSGeometrySerializer()),
	}
}

// NewShapeIOWriterWithFormat creates a new ShapeIOWriter with a specific format.
//
// Parameters:
//   - writer: The underlying writer
//   - format: The serialization format (WKB or Spatial4j)
func NewShapeIOWriterWithFormat(writer io.Writer, format ShapeIOFormat) *ShapeIOWriter {
	s := &ShapeIOWriter{
		writer:    writer,
		bufWriter: bufio.NewWriter(writer),
		byteOrder: binary.LittleEndian,
		format:    format,
	}

	// Set appropriate encoder based on format
	switch format {
	case ShapeIOFormatSpatial4j:
		s.encoder = NewShapeEncoderFromSpatial4j(NewSpatial4jShapeDecoder(NewSpatialContext()))
	default:
		s.encoder = NewShapeEncoderFromSerializer(NewJTSGeometrySerializer())
	}

	return s
}

// SetEncoder sets a custom geometry encoder for this writer.
func (s *ShapeIOWriter) SetEncoder(encoder ShapeEncoder) {
	s.encoder = encoder
}

// GetEncoder returns the current geometry encoder.
func (s *ShapeIOWriter) GetEncoder() ShapeEncoder {
	return s.encoder
}

// SetFormat sets the serialization format.
func (s *ShapeIOWriter) SetFormat(format ShapeIOFormat) {
	s.format = format

	// Update encoder based on format
	switch format {
	case ShapeIOFormatSpatial4j:
		if _, ok := s.encoder.(*spatial4jEncoder); !ok {
			s.encoder = NewShapeEncoderFromSpatial4j(NewSpatial4jShapeDecoder(NewSpatialContext()))
		}
	case ShapeIOFormatWKB:
		if _, ok := s.encoder.(*shapeEncoderAdapter); !ok {
			s.encoder = NewShapeEncoderFromSerializer(NewJTSGeometrySerializer())
		}
	}
}

// GetFormat returns the current serialization format.
func (s *ShapeIOWriter) GetFormat() ShapeIOFormat {
	return s.format
}

// SetByteOrder sets the byte order for binary writes.
func (s *ShapeIOWriter) SetByteOrder(order binary.ByteOrder) {
	s.byteOrder = order
}

// GetByteOrder returns the current byte order.
func (s *ShapeIOWriter) GetByteOrder() binary.ByteOrder {
	return s.byteOrder
}

// WriteShape writes a single shape to the output stream.
// The shape data is prefixed with a 4-byte length field.
//
// Parameters:
//   - shape: The shape to write
//
// Returns the number of bytes written or an error.
func (s *ShapeIOWriter) WriteShape(shape Shape) (int, error) {
	if shape == nil {
		return 0, fmt.Errorf("cannot write nil shape")
	}

	// Encode the shape
	data, err := s.encoder.Encode(shape)
	if err != nil {
		return 0, fmt.Errorf("failed to encode shape: %w", err)
	}

	// Write length prefix
	if err := binary.Write(s.bufWriter, s.byteOrder, uint32(len(data))); err != nil {
		return 0, fmt.Errorf("failed to write length: %w", err)
	}

	// Write shape data
	if _, err := s.bufWriter.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write shape data: %w", err)
	}

	bytesWritten := 4 + len(data)
	s.bytesWritten += int64(bytesWritten)

	return bytesWritten, nil
}

// WriteShapeData writes raw shape data to the output stream.
// This is useful when the shape is already encoded.
//
// Parameters:
//   - data: The encoded shape data
//
// Returns the number of bytes written or an error.
func (s *ShapeIOWriter) WriteShapeData(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("cannot write empty shape data")
	}

	// Write length prefix
	if err := binary.Write(s.bufWriter, s.byteOrder, uint32(len(data))); err != nil {
		return 0, fmt.Errorf("failed to write length: %w", err)
	}

	// Write shape data
	if _, err := s.bufWriter.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write shape data: %w", err)
	}

	bytesWritten := 4 + len(data)
	s.bytesWritten += int64(bytesWritten)

	return bytesWritten, nil
}

// WriteRaw writes raw bytes to the underlying writer.
// This is useful for writing format-specific headers or metadata.
//
// Parameters:
//   - p: The data to write
//
// Returns the number of bytes written or an error.
func (s *ShapeIOWriter) WriteRaw(p []byte) (int, error) {
	n, err := s.bufWriter.Write(p)
	s.bytesWritten += int64(n)
	return n, err
}

// WriteShapes writes multiple shapes to the output stream.
//
// Parameters:
//   - shapes: The shapes to write
//
// Returns the total number of bytes written or an error.
func (s *ShapeIOWriter) WriteShapes(shapes []Shape) (int, error) {
	totalBytes := 0
	for i, shape := range shapes {
		n, err := s.WriteShape(shape)
		if err != nil {
			return totalBytes, fmt.Errorf("failed to write shape %d: %w", i, err)
		}
		totalBytes += n
	}
	return totalBytes, nil
}

// Flush flushes any buffered data to the underlying writer.
// This should be called before closing the writer or when data
// needs to be persisted immediately.
//
// Returns an error if flushing fails.
func (s *ShapeIOWriter) Flush() error {
	return s.bufWriter.Flush()
}

// Reset resets the writer to write to a new destination.
// This allows reusing the writer buffer.
//
// Parameters:
//   - writer: The new destination writer
func (s *ShapeIOWriter) Reset(writer io.Writer) {
	s.writer = writer
	s.bufWriter.Reset(writer)
	s.bytesWritten = 0
}

// GetBytesWritten returns the total number of bytes written.
func (s *ShapeIOWriter) GetBytesWritten() int64 {
	return s.bytesWritten
}

// Close flushes any remaining data and releases resources.
// Note: This does NOT close the underlying writer.
//
// Returns an error if flushing fails.
func (s *ShapeIOWriter) Close() error {
	// Flush remaining data
	if err := s.Flush(); err != nil {
		return err
	}

	// Clear references to allow GC
	s.writer = nil
	s.bufWriter = nil
	s.encoder = nil

	return nil
}

// ShapeBatchWriter provides efficient batch writing of shapes.
// This is useful for bulk operations where writing shapes one at a time
// would be inefficient.
type ShapeBatchWriter struct {
	writer    *ShapeIOWriter
	batch     []Shape
	batchSize int
}

// NewShapeBatchWriter creates a new batch writer.
//
// Parameters:
//   - writer: The underlying ShapeIOWriter
//   - batchSize: The number of shapes to buffer before flushing
func NewShapeBatchWriter(writer *ShapeIOWriter, batchSize int) *ShapeBatchWriter {
	return &ShapeBatchWriter{
		writer:    writer,
		batch:     make([]Shape, 0, batchSize),
		batchSize: batchSize,
	}
}

// WriteShape adds a shape to the batch.
// If the batch is full, it flushes automatically.
//
// Parameters:
//   - shape: The shape to add
//
// Returns the number of bytes written (if flushed) or an error.
func (b *ShapeBatchWriter) WriteShape(shape Shape) (int, error) {
	b.batch = append(b.batch, shape)

	if len(b.batch) >= b.batchSize {
		return b.Flush()
	}

	return 0, nil
}

// Flush writes all buffered shapes to the underlying writer.
//
// Returns the total number of bytes written or an error.
func (b *ShapeBatchWriter) Flush() (int, error) {
	if len(b.batch) == 0 {
		return 0, nil
	}

	totalBytes, err := b.writer.WriteShapes(b.batch)
	if err != nil {
		return totalBytes, err
	}

	// Clear the batch
	b.batch = b.batch[:0]

	return totalBytes, nil
}

// Close flushes any remaining shapes and closes the batch writer.
//
// Returns an error if flushing fails.
func (b *ShapeBatchWriter) Close() error {
	_, err := b.Flush()
	return err
}

// GetBatchSize returns the current batch size.
func (b *ShapeBatchWriter) GetBatchSize() int {
	return b.batchSize
}

// SetBatchSize changes the batch size for future writes.
// This does not affect shapes already in the current batch.
func (b *ShapeBatchWriter) SetBatchSize(size int) {
	if size != b.batchSize {
		newBatch := make([]Shape, 0, size)
		copy(newBatch, b.batch)
		b.batch = newBatch
		b.batchSize = size
	}
}

// GetBufferedCount returns the number of shapes currently buffered.
func (b *ShapeBatchWriter) GetBufferedCount() int {
	return len(b.batch)
}

// ShapeIOWriterFactory creates ShapeIOWriter instances with common configurations.
type ShapeIOWriterFactory struct {
	defaultFormat     ShapeIOFormat
	defaultBufferSize int
	defaultEncoder    ShapeEncoder
}

// NewShapeIOWriterFactory creates a new factory with default settings.
func NewShapeIOWriterFactory() *ShapeIOWriterFactory {
	return &ShapeIOWriterFactory{
		defaultFormat:     ShapeIOFormatWKB,
		defaultBufferSize: 8192,
		defaultEncoder:    NewShapeEncoderFromSerializer(NewJTSGeometrySerializer()),
	}
}

// CreateWriter creates a ShapeIOWriter with factory defaults.
func (f *ShapeIOWriterFactory) CreateWriter(writer io.Writer) *ShapeIOWriter {
	s := NewShapeIOWriterWithBuffer(writer, f.defaultBufferSize)
	s.SetFormat(f.defaultFormat)
	s.SetEncoder(f.defaultEncoder)
	return s
}

// SetDefaultFormat sets the default format for created writers.
func (f *ShapeIOWriterFactory) SetDefaultFormat(format ShapeIOFormat) {
	f.defaultFormat = format
}

// SetDefaultBufferSize sets the default buffer size for created writers.
func (f *ShapeIOWriterFactory) SetDefaultBufferSize(size int) {
	f.defaultBufferSize = size
}

// SetDefaultEncoder sets the default encoder for created writers.
func (f *ShapeIOWriterFactory) SetDefaultEncoder(encoder ShapeEncoder) {
	f.defaultEncoder = encoder
}

// SetDefaultEncoderFromSerializer sets the default encoder from a ShapeSerializer.
func (f *ShapeIOWriterFactory) SetDefaultEncoderFromSerializer(serializer ShapeSerializer) {
	f.defaultEncoder = NewShapeEncoderFromSerializer(serializer)
}

// SetDefaultEncoderFromSpatial4j sets the default encoder from a Spatial4jShapeDecoder.
func (f *ShapeIOWriterFactory) SetDefaultEncoderFromSpatial4j(decoder *Spatial4jShapeDecoder) {
	f.defaultEncoder = NewShapeEncoderFromSpatial4j(decoder)
}

// DefaultShapeIOWriterFactory is the default factory instance.
var DefaultShapeIOWriterFactory = NewShapeIOWriterFactory()
