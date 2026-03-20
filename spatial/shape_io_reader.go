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

// ShapeIOReader provides efficient reading of spatial shape data from I/O streams.
// It supports multiple serialization formats (WKB, Spatial4j binary) and provides
// buffered reading for improved performance.
//
// The reader is designed to be used with the spatial index infrastructure,
// reading shape data that was previously written by ShapeIOWriter.
//
// This is the Go port of Lucene's ShapeIOReader concept.
type ShapeIOReader struct {
	// reader is the underlying reader
	reader io.Reader

	// bufReader provides buffering for better performance
	bufReader *bufio.Reader

	// byteOrder is the default byte order for binary reads
	byteOrder binary.ByteOrder

	// decoder is the geometry decoder for parsing shape data
	decoder GeometryDecoder

	// format specifies the serialization format
	format ShapeIOFormat
}

// ShapeIOFormat represents the serialization format for shape I/O.
type ShapeIOFormat int

const (
	// ShapeIOFormatWKB uses Well-Known Binary format (JTS/OGC standard).
	ShapeIOFormatWKB ShapeIOFormat = iota

	// ShapeIOFormatSpatial4j uses Spatial4j native binary format.
	ShapeIOFormatSpatial4j

	// ShapeIOFormatAuto detects the format automatically from data.
	ShapeIOFormatAuto
)

// NewShapeIOReader creates a new ShapeIOReader for reading shape data.
// Uses WKB format by default with LittleEndian byte order.
//
// Parameters:
//   - reader: The underlying reader to read from
//
// Returns a new ShapeIOReader instance.
func NewShapeIOReader(reader io.Reader) *ShapeIOReader {
	return &ShapeIOReader{
		reader:    reader,
		bufReader: bufio.NewReader(reader),
		byteOrder: binary.LittleEndian,
		format:    ShapeIOFormatWKB,
		decoder:   NewJTSGeometryDecoder(),
	}
}

// NewShapeIOReaderWithBuffer creates a new ShapeIOReader with a custom buffer size.
//
// Parameters:
//   - reader: The underlying reader
//   - bufferSize: The size of the read buffer in bytes
func NewShapeIOReaderWithBuffer(reader io.Reader, bufferSize int) *ShapeIOReader {
	return &ShapeIOReader{
		reader:    reader,
		bufReader: bufio.NewReaderSize(reader, bufferSize),
		byteOrder: binary.LittleEndian,
		format:    ShapeIOFormatWKB,
		decoder:   NewJTSGeometryDecoder(),
	}
}

// NewShapeIOReaderWithFormat creates a new ShapeIOReader with a specific format.
//
// Parameters:
//   - reader: The underlying reader
//   - format: The serialization format (WKB or Spatial4j)
func NewShapeIOReaderWithFormat(r io.Reader, format ShapeIOFormat) *ShapeIOReader {
	s := &ShapeIOReader{
		reader:    r,
		bufReader: bufio.NewReader(r),
		byteOrder: binary.LittleEndian,
		format:    format,
	}

	// Set appropriate decoder based on format
	switch format {
	case ShapeIOFormatSpatial4j:
		s.decoder = NewSpatial4jShapeDecoder(NewSpatialContext())
	default:
		s.decoder = NewJTSGeometryDecoder()
	}

	return s
}

// SetDecoder sets a custom geometry decoder for this reader.
func (s *ShapeIOReader) SetDecoder(decoder GeometryDecoder) {
	s.decoder = decoder
}

// GetDecoder returns the current geometry decoder.
func (s *ShapeIOReader) GetDecoder() GeometryDecoder {
	return s.decoder
}

// SetFormat sets the serialization format.
func (s *ShapeIOReader) SetFormat(format ShapeIOFormat) {
	s.format = format

	// Update decoder based on format
	switch format {
	case ShapeIOFormatSpatial4j:
		if _, ok := s.decoder.(*Spatial4jShapeDecoder); !ok {
			s.decoder = NewSpatial4jShapeDecoder(NewSpatialContext())
		}
	case ShapeIOFormatWKB:
		if _, ok := s.decoder.(*JTSGeometryDecoder); !ok {
			s.decoder = NewJTSGeometryDecoder()
		}
	}
}

// GetFormat returns the current serialization format.
func (s *ShapeIOReader) GetFormat() ShapeIOFormat {
	return s.format
}

// SetByteOrder sets the byte order for binary reads.
func (s *ShapeIOReader) SetByteOrder(order binary.ByteOrder) {
	s.byteOrder = order
}

// GetByteOrder returns the current byte order.
func (s *ShapeIOReader) GetByteOrder() binary.ByteOrder {
	return s.byteOrder
}

// ReadShape reads a single shape from the input stream.
// The shape data is expected to be prefixed with a 4-byte length field.
//
// Returns the shape or an error if:
//   - End of stream reached
//   - Data is malformed
//   - Decoder fails to parse the data
func (s *ShapeIOReader) ReadShape() (Shape, error) {
	// Read the length prefix (4 bytes, uint32)
	var length uint32
	if err := binary.Read(s.bufReader, s.byteOrder, &length); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read shape length: %w", err)
	}

	if length == 0 {
		return nil, fmt.Errorf("invalid shape length: 0")
	}

	// Read the shape data
	data := make([]byte, length)
	if _, err := io.ReadFull(s.bufReader, data); err != nil {
		return nil, fmt.Errorf("failed to read shape data: %w", err)
	}

	// Decode the shape
	shape, err := s.decoder.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode shape: %w", err)
	}

	return shape, nil
}

// ReadShapeWithLength reads a shape with an explicit length.
// This is useful when the length is known beforehand (e.g., from an index).
//
// Parameters:
//   - length: The number of bytes to read
//
// Returns the shape or an error.
func (s *ShapeIOReader) ReadShapeWithLength(length int) (Shape, error) {
	if length <= 0 {
		return nil, fmt.Errorf("invalid length: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(s.bufReader, data); err != nil {
		return nil, fmt.Errorf("failed to read shape data: %w", err)
	}

	shape, err := s.decoder.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode shape: %w", err)
	}

	return shape, nil
}

// ReadRaw reads raw bytes from the underlying reader.
// This is useful for reading format-specific headers or metadata.
//
// Parameters:
//   - p: The buffer to read into
//
// Returns the number of bytes read or an error.
func (s *ShapeIOReader) ReadRaw(p []byte) (int, error) {
	return s.bufReader.Read(p)
}

// ReadAllShapes reads all remaining shapes from the stream.
// This reads until EOF is encountered.
//
// Returns a slice of shapes or an error. If EOF is reached with no shapes,
// returns an empty slice and nil error.
func (s *ShapeIOReader) ReadAllShapes() ([]Shape, error) {
	var shapes []Shape

	for {
		shape, err := s.ReadShape()
		if err != nil {
			if err == io.EOF {
				break
			}
			return shapes, err
		}
		shapes = append(shapes, shape)
	}

	return shapes, nil
}

// SkipShape skips the next shape in the stream without decoding it.
// This is more efficient than ReadShape when the shape data is not needed.
//
// Returns the number of bytes skipped or an error.
func (s *ShapeIOReader) SkipShape() (int64, error) {
	// Read the length prefix
	var length uint32
	if err := binary.Read(s.bufReader, s.byteOrder, &length); err != nil {
		return 0, err
	}

	// Skip the shape data
	skipped, err := s.bufReader.Discard(int(length))
	return int64(skipped) + 4, err // +4 for length field
}

// Reset resets the reader to read from a new source.
// This allows reusing the reader buffer.
//
// Parameters:
//   - reader: The new source reader
func (s *ShapeIOReader) Reset(reader io.Reader) {
	s.reader = reader
	s.bufReader.Reset(reader)
}

// GetPosition returns an estimate of the number of bytes read.
// This is the position in the buffered reader, which may differ
// from the actual underlying reader position due to buffering.
func (s *ShapeIOReader) GetPosition() int64 {
	// This is an approximation; exact position tracking would require
	// wrapping the reader in a counting reader
	return 0
}

// Close closes the reader and releases resources.
// Note: This does NOT close the underlying reader.
func (s *ShapeIOReader) Close() error {
	// Clear references to allow GC
	s.reader = nil
	s.bufReader = nil
	s.decoder = nil
	return nil
}

// ShapeBatchReader provides efficient batch reading of shapes.
// This is useful for bulk operations where reading shapes one at a time
// would be inefficient.
type ShapeBatchReader struct {
	reader *ShapeIOReader
	batch  []Shape
}

// NewShapeBatchReader creates a new batch reader.
//
// Parameters:
//   - reader: The underlying ShapeIOReader
//   - batchSize: The number of shapes to read in each batch
func NewShapeBatchReader(reader *ShapeIOReader, batchSize int) *ShapeBatchReader {
	return &ShapeBatchReader{
		reader: reader,
		batch:  make([]Shape, 0, batchSize),
	}
}

// ReadBatch reads the next batch of shapes.
// Returns the batch and an error. If EOF is reached, returns the batch
// and io.EOF (batch may be partial).
func (b *ShapeBatchReader) ReadBatch() ([]Shape, error) {
	b.batch = b.batch[:0] // Clear without deallocating

	cap := cap(b.batch)
	for i := 0; i < cap; i++ {
		shape, err := b.reader.ReadShape()
		if err != nil {
			if err == io.EOF {
				return b.batch, err
			}
			return b.batch, err
		}
		b.batch = append(b.batch, shape)
	}

	return b.batch, nil
}

// SetBatchSize changes the batch size for future reads.
// This does not affect shapes already in the current batch.
func (b *ShapeBatchReader) SetBatchSize(size int) {
	if size != cap(b.batch) {
		newBatch := make([]Shape, 0, size)
		copy(newBatch, b.batch)
		b.batch = newBatch
	}
}

// GetBatchSize returns the current batch size.
func (b *ShapeBatchReader) GetBatchSize() int {
	return cap(b.batch)
}

// SeekableShapeIOReader provides random access to shape data.
// This requires an io.ReadSeeker as the underlying reader.
type SeekableShapeIOReader struct {
	*ShapeIOReader
	seeker io.ReadSeeker
}

// NewSeekableShapeIOReader creates a new seekable shape reader.
//
// Parameters:
//   - seeker: The underlying ReadSeeker
func NewSeekableShapeIOReader(seeker io.ReadSeeker) *SeekableShapeIOReader {
	return &SeekableShapeIOReader{
		ShapeIOReader: NewShapeIOReader(seeker),
		seeker:        seeker,
	}
}

// Seek seeks to a specific position in the stream.
//
// Parameters:
//   - offset: The position to seek to
//   - whence: The seek reference (io.SeekStart, io.SeekCurrent, io.SeekEnd)
//
// Returns the new position or an error.
func (s *SeekableShapeIOReader) Seek(offset int64, whence int) (int64, error) {
	return s.seeker.Seek(offset, whence)
}

// ReadShapeAt reads a shape at a specific position.
// This seeks to the position, reads the shape, and returns to the original position.
//
// Parameters:
//   - position: The position to read from
//
// Returns the shape or an error.
func (s *SeekableShapeIOReader) ReadShapeAt(position int64) (Shape, error) {
	// Save current position
	currentPos, err := s.seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}

	// Seek to target position
	if _, err := s.seeker.Seek(position, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	// Reset the buffered reader to use the new position
	s.bufReader.Reset(s.seeker)

	// Read the shape
	shape, err := s.ReadShape()

	// Restore original position
	s.seeker.Seek(currentPos, io.SeekStart)
	s.bufReader.Reset(s.seeker)

	return shape, err
}

// GetPosition returns the current position in the stream.
func (s *SeekableShapeIOReader) GetPosition() (int64, error) {
	return s.seeker.Seek(0, io.SeekCurrent)
}

// ShapeIOReaderFactory creates ShapeIOReader instances with common configurations.
type ShapeIOReaderFactory struct {
	defaultFormat     ShapeIOFormat
	defaultBufferSize int
	defaultDecoder    GeometryDecoder
}

// NewShapeIOReaderFactory creates a new factory with default settings.
func NewShapeIOReaderFactory() *ShapeIOReaderFactory {
	return &ShapeIOReaderFactory{
		defaultFormat:     ShapeIOFormatWKB,
		defaultBufferSize: 8192,
		defaultDecoder:    NewJTSGeometryDecoder(),
	}
}

// CreateReader creates a ShapeIOReader with factory defaults.
func (f *ShapeIOReaderFactory) CreateReader(reader io.Reader) *ShapeIOReader {
	s := NewShapeIOReaderWithBuffer(reader, f.defaultBufferSize)
	s.SetFormat(f.defaultFormat)
	s.SetDecoder(f.defaultDecoder)
	return s
}

// SetDefaultFormat sets the default format for created readers.
func (f *ShapeIOReaderFactory) SetDefaultFormat(format ShapeIOFormat) {
	f.defaultFormat = format
}

// SetDefaultBufferSize sets the default buffer size for created readers.
func (f *ShapeIOReaderFactory) SetDefaultBufferSize(size int) {
	f.defaultBufferSize = size
}

// SetDefaultDecoder sets the default decoder for created readers.
func (f *ShapeIOReaderFactory) SetDefaultDecoder(decoder GeometryDecoder) {
	f.defaultDecoder = decoder
}

// DefaultShapeIOReaderFactory is the default factory instance.
var DefaultShapeIOReaderFactory = NewShapeIOReaderFactory()
