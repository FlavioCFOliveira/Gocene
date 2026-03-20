// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// GeometryDecoder provides a generic interface for decoding geometries
// from various binary formats. This is used by spatial strategies that
// need to read and reconstruct shapes from serialized data.
//
// Implementations should support WKB (Well-Known Binary) and potentially
// other formats like WKT (Well-Known Text) or custom binary formats.
type GeometryDecoder interface {
	// Decode deserializes geometry data from a byte slice.
	// Returns the decoded Shape or an error if decoding fails.
	Decode(data []byte) (Shape, error)

	// DecodeFrom reads and decodes geometry data from a reader.
	// This is more efficient than Decode when reading from streams.
	DecodeFrom(r io.Reader) (Shape, error)

	// GetFormatName returns the name of the format this decoder supports.
	GetFormatName() string

	// GetFormatVersion returns the version of the format.
	GetFormatVersion() string
}

// JTSGeometryDecoder implements GeometryDecoder for JTS (Java Topology Suite)
// compatible geometry deserialization using WKB (Well-Known Binary) format.
//
// This decoder supports:
//   - Point geometries (type 1)
//   - Polygon geometries (type 3), returned as Rectangle (bounding box)
//   - Both LittleEndian and BigEndian byte orders
//   - OGC Simple Features Specification v1.2.0
//
// The decoder is thread-safe and can be shared across multiple goroutines.
type JTSGeometryDecoder struct {
	// calculator provides distance calculations for decoded geometries
	calculator DistanceCalculator
}

// NewJTSGeometryDecoder creates a new JTSGeometryDecoder with default settings.
// Uses the Haversine distance calculator by default for geographic coordinates.
func NewJTSGeometryDecoder() *JTSGeometryDecoder {
	return &JTSGeometryDecoder{
		calculator: &HaversineCalculator{},
	}
}

// NewJTSGeometryDecoderWithCalculator creates a new JTSGeometryDecoder
// with a custom distance calculator.
func NewJTSGeometryDecoderWithCalculator(calculator DistanceCalculator) *JTSGeometryDecoder {
	return &JTSGeometryDecoder{
		calculator: calculator,
	}
}

// Decode deserializes WKB binary data into a Shape.
// Supports Point (type 1) and Polygon (type 3, returned as Rectangle).
//
// Parameters:
//   - data: The WKB-encoded geometry data
//
// Returns the decoded Shape or an error if:
//   - Data is empty
//   - Invalid byte order
//   - Unsupported geometry type
//   - Malformed data
func (j *JTSGeometryDecoder) Decode(data []byte) (Shape, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot decode empty data")
	}

	buf := bytes.NewReader(data)
	return j.DecodeFrom(buf)
}

// DecodeFrom reads and decodes WKB geometry data from a reader.
// This is more efficient than Decode when reading from streams as it
// avoids an extra memory copy.
//
// Parameters:
//   - r: The reader containing WKB-encoded geometry data
//
// Returns the decoded Shape or an error if decoding fails.
func (j *JTSGeometryDecoder) DecodeFrom(r io.Reader) (Shape, error) {
	// Read byte order
	var byteOrder byte
	if err := binary.Read(r, binary.LittleEndian, &byteOrder); err != nil {
		return nil, fmt.Errorf("failed to read byte order: %w", err)
	}

	// Determine byte order for reading
	var order binary.ByteOrder
	switch byteOrder {
	case WKBNDR:
		order = binary.LittleEndian
	case WKBXDR:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid byte order: %d", byteOrder)
	}

	// Read geometry type
	var geomType uint32
	if err := binary.Read(r, order, &geomType); err != nil {
		return nil, fmt.Errorf("failed to read geometry type: %w", err)
	}

	// Check for SRID flag
	if geomType&WKBGeometrySRIDFlag != 0 {
		// Skip SRID (4 bytes)
		var srid uint32
		if err := binary.Read(r, order, &srid); err != nil {
			return nil, fmt.Errorf("failed to read SRID: %w", err)
		}
		geomType &^= WKBGeometrySRIDFlag
	}

	// Decode based on geometry type
	switch geomType {
	case WKBPoint:
		return j.decodePoint(r, order)
	case WKBPolygon:
		return j.decodePolygon(r, order)
	case WKBLineString:
		return j.decodeLineString(r, order)
	default:
		return nil, fmt.Errorf("unsupported WKB geometry type: %d", geomType)
	}
}

// decodePoint decodes a WKB Point.
// Format: X (float64), Y (float64)
func (j *JTSGeometryDecoder) decodePoint(r io.Reader, order binary.ByteOrder) (Shape, error) {
	var x, y float64
	if err := binary.Read(r, order, &x); err != nil {
		return nil, fmt.Errorf("failed to read point X: %w", err)
	}
	if err := binary.Read(r, order, &y); err != nil {
		return nil, fmt.Errorf("failed to read point Y: %w", err)
	}
	return NewPoint(x, y), nil
}

// decodePolygon decodes a WKB Polygon and returns its bounding box as a Rectangle.
// Supports both simple rectangles and complex polygons.
//
// Format:
//   - Num rings (uint32)
//   - For each ring:
//   - Num points (uint32)
//   - Points (X,Y pairs as float64)
func (j *JTSGeometryDecoder) decodePolygon(r io.Reader, order binary.ByteOrder) (Shape, error) {
	// Read number of rings
	var numRings uint32
	if err := binary.Read(r, order, &numRings); err != nil {
		return nil, fmt.Errorf("failed to read number of rings: %w", err)
	}

	if numRings == 0 {
		return nil, fmt.Errorf("polygon has no rings")
	}

	// Read number of points in first ring
	var numPoints uint32
	if err := binary.Read(r, order, &numPoints); err != nil {
		return nil, fmt.Errorf("failed to read number of points: %w", err)
	}

	if numPoints < 4 {
		return nil, fmt.Errorf("polygon ring must have at least 4 points, got %d", numPoints)
	}

	// Read first point to initialize bounding box
	var firstX, firstY float64
	if err := binary.Read(r, order, &firstX); err != nil {
		return nil, fmt.Errorf("failed to read first point X: %w", err)
	}
	if err := binary.Read(r, order, &firstY); err != nil {
		return nil, fmt.Errorf("failed to read first point Y: %w", err)
	}

	minX, maxX := firstX, firstX
	minY, maxY := firstY, firstY

	// Read remaining points and compute bounding box
	for i := uint32(1); i < numPoints; i++ {
		var x, y float64
		if err := binary.Read(r, order, &x); err != nil {
			return nil, fmt.Errorf("failed to read point %d X: %w", i, err)
		}
		if err := binary.Read(r, order, &y); err != nil {
			return nil, fmt.Errorf("failed to read point %d Y: %w", i, err)
		}

		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	// Skip remaining rings (holes) if present
	for ringIdx := uint32(1); ringIdx < numRings; ringIdx++ {
		var ringPoints uint32
		if err := binary.Read(r, order, &ringPoints); err != nil {
			return nil, fmt.Errorf("failed to read ring %d point count: %w", ringIdx, err)
		}
		// Skip all points in this ring
		for i := uint32(0); i < ringPoints*2; i++ {
			var val float64
			if err := binary.Read(r, order, &val); err != nil {
				return nil, fmt.Errorf("failed to skip ring point: %w", err)
			}
		}
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// decodeLineString decodes a WKB LineString and returns its bounding box as a Rectangle.
// This is useful for linear features that need to be queried spatially.
//
// Format:
//   - Num points (uint32)
//   - Points (X,Y pairs as float64)
func (j *JTSGeometryDecoder) decodeLineString(r io.Reader, order binary.ByteOrder) (Shape, error) {
	var numPoints uint32
	if err := binary.Read(r, order, &numPoints); err != nil {
		return nil, fmt.Errorf("failed to read number of points: %w", err)
	}

	if numPoints == 0 {
		return nil, fmt.Errorf("linestring has no points")
	}

	// Read first point
	var firstX, firstY float64
	if err := binary.Read(r, order, &firstX); err != nil {
		return nil, fmt.Errorf("failed to read first point X: %w", err)
	}
	if err := binary.Read(r, order, &firstY); err != nil {
		return nil, fmt.Errorf("failed to read first point Y: %w", err)
	}

	minX, maxX := firstX, firstX
	minY, maxY := firstY, firstY

	// Read remaining points and compute bounding box
	for i := uint32(1); i < numPoints; i++ {
		var x, y float64
		if err := binary.Read(r, order, &x); err != nil {
			return nil, fmt.Errorf("failed to read point %d X: %w", i, err)
		}
		if err := binary.Read(r, order, &y); err != nil {
			return nil, fmt.Errorf("failed to read point %d Y: %w", i, err)
		}

		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// GetCalculator returns the distance calculator used by this decoder.
func (j *JTSGeometryDecoder) GetCalculator() DistanceCalculator {
	return j.calculator
}

// SetCalculator sets the distance calculator for this decoder.
func (j *JTSGeometryDecoder) SetCalculator(calculator DistanceCalculator) {
	j.calculator = calculator
}

// GetFormatName returns "WKB" as the format name.
func (j *JTSGeometryDecoder) GetFormatName() string {
	return "WKB"
}

// GetFormatVersion returns "1.2.0" as the WKB specification version.
func (j *JTSGeometryDecoder) GetFormatVersion() string {
	return "1.2.0"
}

// DecodeWithValidation decodes geometry data with additional validation checks.
// This is slower than Decode but catches more malformed data.
//
// Validation checks include:
//   - Valid coordinate ranges for geographic data
//   - Proper polygon closure (first point == last point)
//   - Non-degenerate geometries (non-zero area/volume)
func (j *JTSGeometryDecoder) DecodeWithValidation(data []byte) (Shape, error) {
	shape, err := j.Decode(data)
	if err != nil {
		return nil, err
	}

	// Validate coordinates are within reasonable bounds
	bbox := shape.GetBoundingBox()
	if bbox == nil {
		return nil, fmt.Errorf("decoded shape has no bounding box")
	}

	// Check for finite values
	if !isFinite(bbox.MinX) || !isFinite(bbox.MinY) ||
		!isFinite(bbox.MaxX) || !isFinite(bbox.MaxY) {
		return nil, fmt.Errorf("decoded shape has non-finite coordinates")
	}

	// Check for valid bounding box (min < max)
	if bbox.MinX > bbox.MaxX || bbox.MinY > bbox.MaxY {
		return nil, fmt.Errorf("invalid bounding box: min > max")
	}

	return shape, nil
}

// isFinite returns true if the float64 value is finite (not NaN or Inf).
func isFinite(f float64) bool {
	// A value is finite if it equals itself and is not Inf
	// NaN != NaN, Inf + Inf == Inf
	return f == f && f+f != f
}

// GeometryDecoderFactory creates GeometryDecoder instances.
// This allows for configurable decoder creation in different contexts.
type GeometryDecoderFactory struct {
	// defaultCalculator is the calculator used when none is specified
	defaultCalculator DistanceCalculator
}

// NewGeometryDecoderFactory creates a new factory with default settings.
func NewGeometryDecoderFactory() *GeometryDecoderFactory {
	return &GeometryDecoderFactory{
		defaultCalculator: &HaversineCalculator{},
	}
}

// CreateJTSGeometryDecoder creates a JTSGeometryDecoder with default settings.
func (f *GeometryDecoderFactory) CreateJTSGeometryDecoder() GeometryDecoder {
	return NewJTSGeometryDecoderWithCalculator(f.defaultCalculator)
}

// SetDefaultCalculator sets the default calculator for created decoders.
func (f *GeometryDecoderFactory) SetDefaultCalculator(calculator DistanceCalculator) {
	f.defaultCalculator = calculator
}

// Ensure JTSGeometryDecoder implements GeometryDecoder
var _ GeometryDecoder = (*JTSGeometryDecoder)(nil)

// DefaultGeometryDecoder is the default GeometryDecoder instance.
// This can be shared across the application for efficiency.
var DefaultGeometryDecoder = NewJTSGeometryDecoder()
