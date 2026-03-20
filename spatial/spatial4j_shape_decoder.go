// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Spatial4jShapeDecoder implements ShapeDecoder for Spatial4j-compatible formats.
// Spatial4j is a Java spatial library that supports multiple shape representations:
// - WKT (Well-Known Text)
// - Spatial4j's native binary format
// - Point/Rectangle/Circle representations
//
// This decoder is designed to be compatible with Lucene Spatial's Spatial4j
// integration, allowing interoperability with existing Lucene spatial indexes.
//
// The decoder is thread-safe and can be shared across multiple goroutines.
type Spatial4jShapeDecoder struct {
	// ctx is the spatial context for coordinate transformations
	ctx *SpatialContext

	// defaultCalculator is the distance calculator for the context
	defaultCalculator DistanceCalculator
}

// NewSpatial4jShapeDecoder creates a new Spatial4jShapeDecoder with the given context.
// Uses the Haversine distance calculator by default for geographic coordinates.
func NewSpatial4jShapeDecoder(ctx *SpatialContext) *Spatial4jShapeDecoder {
	return &Spatial4jShapeDecoder{
		ctx:               ctx,
		defaultCalculator: &HaversineCalculator{},
	}
}

// NewSpatial4jShapeDecoderWithCalculator creates a new decoder with a custom calculator.
func NewSpatial4jShapeDecoderWithCalculator(ctx *SpatialContext, calculator DistanceCalculator) *Spatial4jShapeDecoder {
	return &Spatial4jShapeDecoder{
		ctx:               ctx,
		defaultCalculator: calculator,
	}
}

// GetContext returns the spatial context used by this decoder.
func (s *Spatial4jShapeDecoder) GetContext() *SpatialContext {
	return s.ctx
}

// SetContext sets the spatial context for this decoder.
func (s *Spatial4jShapeDecoder) SetContext(ctx *SpatialContext) {
	s.ctx = ctx
}

// DecodeFromWKT parses a Well-Known Text (WKT) string and returns a Shape.
// Supported WKT types:
//   - POINT(x y)
//   - ENVELOPE(minX, maxX, maxY, minY) - Spatial4j specific
//   - RECTANGLE(minX, maxX, minY, maxY)
//
// Parameters:
//   - wkt: The WKT string to parse
//
// Returns the decoded Shape or an error if parsing fails.
func (s *Spatial4jShapeDecoder) DecodeFromWKT(wkt string) (Shape, error) {
	if wkt == "" {
		return nil, fmt.Errorf("empty WKT string")
	}

	// Normalize the WKT string
	wkt = strings.TrimSpace(wkt)
	upperWKT := strings.ToUpper(wkt)

	switch {
	case strings.HasPrefix(upperWKT, "POINT"):
		return s.parsePointWKT(wkt)
	case strings.HasPrefix(upperWKT, "ENVELOPE"):
		return s.parseEnvelopeWKT(wkt)
	case strings.HasPrefix(upperWKT, "RECTANGLE"):
		return s.parseRectangleWKT(wkt)
	default:
		return nil, fmt.Errorf("unsupported WKT type: %s", wkt)
	}
}

// parsePointWKT parses a POINT WKT string.
// Format: POINT(x y) or POINT(x, y) or POINT Z(x y z)
func (s *Spatial4jShapeDecoder) parsePointWKT(wkt string) (Shape, error) {
	// Extract coordinates between parentheses
	start := strings.Index(wkt, "(")
	end := strings.LastIndex(wkt, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid POINT WKT format: %s", wkt)
	}

	coordsStr := wkt[start+1 : end]
	coordsStr = strings.TrimSpace(coordsStr)

	// Split by space or comma
	coords := strings.FieldsFunc(coordsStr, func(r rune) bool {
		return r == ' ' || r == ','
	})

	if len(coords) < 2 {
		return nil, fmt.Errorf("POINT requires at least 2 coordinates: %s", wkt)
	}

	x, err := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid X coordinate: %w", err)
	}

	y, err := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Y coordinate: %w", err)
	}

	return NewPoint(x, y), nil
}

// parseEnvelopeWKT parses an ENVELOPE WKT string (Spatial4j specific).
// Format: ENVELOPE(minX, maxX, maxY, minY)
// Note: Spatial4j uses (minX, maxX, maxY, minY) order
func (s *Spatial4jShapeDecoder) parseEnvelopeWKT(wkt string) (Shape, error) {
	// Extract coordinates between parentheses
	start := strings.Index(wkt, "(")
	end := strings.LastIndex(wkt, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid ENVELOPE WKT format: %s", wkt)
	}

	coordsStr := wkt[start+1 : end]
	coordsStr = strings.TrimSpace(coordsStr)

	// Split by comma
	coords := strings.Split(coordsStr, ",")
	if len(coords) < 4 {
		return nil, fmt.Errorf("ENVELOPE requires 4 coordinates (minX, maxX, maxY, minY): %s", wkt)
	}

	minX, err := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid minX: %w", err)
	}

	maxX, err := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid maxX: %w", err)
	}

	maxY, err := strconv.ParseFloat(strings.TrimSpace(coords[2]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid maxY: %w", err)
	}

	minY, err := strconv.ParseFloat(strings.TrimSpace(coords[3]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid minY: %w", err)
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// parseRectangleWKT parses a RECTANGLE WKT string.
// Format: RECTANGLE(minX maxX minY maxY) or RECTANGLE(minX, maxX, minY, maxY)
func (s *Spatial4jShapeDecoder) parseRectangleWKT(wkt string) (Shape, error) {
	// Extract coordinates between parentheses
	start := strings.Index(wkt, "(")
	end := strings.LastIndex(wkt, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid RECTANGLE WKT format: %s", wkt)
	}

	coordsStr := wkt[start+1 : end]
	coordsStr = strings.TrimSpace(coordsStr)

	// Split by space or comma
	coords := strings.FieldsFunc(coordsStr, func(r rune) bool {
		return r == ' ' || r == ','
	})

	if len(coords) < 4 {
		return nil, fmt.Errorf("RECTANGLE requires 4 coordinates (minX, maxX, minY, maxY): %s", wkt)
	}

	minX, err := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid minX: %w", err)
	}

	maxX, err := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid maxX: %w", err)
	}

	minY, err := strconv.ParseFloat(strings.TrimSpace(coords[2]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid minY: %w", err)
	}

	maxY, err := strconv.ParseFloat(strings.TrimSpace(coords[3]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid maxY: %w", err)
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// DecodeFromBytes decodes a Spatial4j native binary format.
// Format:
//   - Type marker (1 byte): 1=Point, 2=Rectangle, 3=Circle
//   - For Point: X (8 bytes), Y (8 bytes)
//   - For Rectangle: minX (8), minY (8), maxX (8), maxY (8)
//   - For Circle: centerX (8), centerY (8), radius (8)
func (s *Spatial4jShapeDecoder) DecodeFromBytes(data []byte) (Shape, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	buf := bytes.NewReader(data)
	return s.DecodeFromReader(buf)
}

// DecodeFromReader decodes a shape from a binary reader.
func (s *Spatial4jShapeDecoder) DecodeFromReader(r io.Reader) (Shape, error) {
	// Read type marker
	var shapeType byte
	if err := binary.Read(r, binary.LittleEndian, &shapeType); err != nil {
		return nil, fmt.Errorf("failed to read shape type: %w", err)
	}

	switch shapeType {
	case spatial4jTypePoint:
		return s.decodePointBinary(r)
	case spatial4jTypeRectangle:
		return s.decodeRectangleBinary(r)
	case spatial4jTypeCircle:
		return s.decodeCircleBinary(r)
	default:
		return nil, fmt.Errorf("unsupported Spatial4j shape type: %d", shapeType)
	}
}

// Spatial4j binary type markers
const (
	spatial4jTypePoint     byte = 1
	spatial4jTypeRectangle byte = 2
	spatial4jTypeCircle    byte = 3
)

// decodePointBinary decodes a Point from binary format.
func (s *Spatial4jShapeDecoder) decodePointBinary(r io.Reader) (Shape, error) {
	var x, y float64
	if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
		return nil, fmt.Errorf("failed to read point X: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
		return nil, fmt.Errorf("failed to read point Y: %w", err)
	}
	return NewPoint(x, y), nil
}

// decodeRectangleBinary decodes a Rectangle from binary format.
func (s *Spatial4jShapeDecoder) decodeRectangleBinary(r io.Reader) (Shape, error) {
	var minX, minY, maxX, maxY float64
	if err := binary.Read(r, binary.LittleEndian, &minX); err != nil {
		return nil, fmt.Errorf("failed to read minX: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &minY); err != nil {
		return nil, fmt.Errorf("failed to read minY: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &maxX); err != nil {
		return nil, fmt.Errorf("failed to read maxX: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &maxY); err != nil {
		return nil, fmt.Errorf("failed to read maxY: %w", err)
	}
	return NewRectangle(minX, minY, maxX, maxY), nil
}

// decodeCircleBinary decodes a Circle from binary format.
// Returns a Rectangle representing the circle's bounding box.
func (s *Spatial4jShapeDecoder) decodeCircleBinary(r io.Reader) (Shape, error) {
	var centerX, centerY, radius float64
	if err := binary.Read(r, binary.LittleEndian, &centerX); err != nil {
		return nil, fmt.Errorf("failed to read center X: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &centerY); err != nil {
		return nil, fmt.Errorf("failed to read center Y: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &radius); err != nil {
		return nil, fmt.Errorf("failed to read radius: %w", err)
	}

	// For simplicity, return a Rectangle representing the circle's bounding box
	minX := centerX - radius
	maxX := centerX + radius
	minY := centerY - radius
	maxY := centerY + radius

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// EncodeToBytes encodes a Shape to Spatial4j binary format.
func (s *Spatial4jShapeDecoder) EncodeToBytes(shape Shape) ([]byte, error) {
	if shape == nil {
		return nil, fmt.Errorf("cannot encode nil shape")
	}

	var buf bytes.Buffer

	switch sh := shape.(type) {
	case Point:
		// Write type
		buf.WriteByte(spatial4jTypePoint)
		// Write coordinates
		binary.Write(&buf, binary.LittleEndian, sh.X)
		binary.Write(&buf, binary.LittleEndian, sh.Y)

	case *Rectangle:
		// Write type
		buf.WriteByte(spatial4jTypeRectangle)
		// Write bounds
		binary.Write(&buf, binary.LittleEndian, sh.MinX)
		binary.Write(&buf, binary.LittleEndian, sh.MinY)
		binary.Write(&buf, binary.LittleEndian, sh.MaxX)
		binary.Write(&buf, binary.LittleEndian, sh.MaxY)

	default:
		// Try to encode as rectangle (bounding box)
		bbox := shape.GetBoundingBox()
		if bbox == nil {
			return nil, fmt.Errorf("unsupported shape type: %T", shape)
		}
		buf.WriteByte(spatial4jTypeRectangle)
		binary.Write(&buf, binary.LittleEndian, bbox.MinX)
		binary.Write(&buf, binary.LittleEndian, bbox.MinY)
		binary.Write(&buf, binary.LittleEndian, bbox.MaxX)
		binary.Write(&buf, binary.LittleEndian, bbox.MaxY)
	}

	return buf.Bytes(), nil
}

// EncodeToWKT encodes a Shape to WKT format.
func (s *Spatial4jShapeDecoder) EncodeToWKT(shape Shape) (string, error) {
	if shape == nil {
		return "", fmt.Errorf("cannot encode nil shape")
	}

	switch sh := shape.(type) {
	case Point:
		return fmt.Sprintf("POINT(%.10g %.10g)", sh.X, sh.Y), nil
	case *Rectangle:
		// Use ENVELOPE format (Spatial4j specific)
		return fmt.Sprintf("ENVELOPE(%.10g, %.10g, %.10g, %.10g)",
			sh.MinX, sh.MaxX, sh.MaxY, sh.MinY), nil
	default:
		// Encode bounding box
		bbox := shape.GetBoundingBox()
		if bbox == nil {
			return "", fmt.Errorf("unsupported shape type: %T", shape)
		}
		return fmt.Sprintf("ENVELOPE(%.10g, %.10g, %.10g, %.10g)",
			bbox.MinX, bbox.MaxX, bbox.MaxY, bbox.MinY), nil
	}
}

// GetFormatName returns the format name.
func (s *Spatial4jShapeDecoder) GetFormatName() string {
	return "Spatial4j"
}

// GetFormatVersion returns the format version.
func (s *Spatial4jShapeDecoder) GetFormatVersion() string {
	return "0.8"
}

// Decode parses shape data based on format.
// For Spatial4j, this uses the binary format.
func (s *Spatial4jShapeDecoder) Decode(data []byte) (Shape, error) {
	return s.DecodeFromBytes(data)
}

// DecodeFrom reads shape data from a reader.
func (s *Spatial4jShapeDecoder) DecodeFrom(r io.Reader) (Shape, error) {
	return s.DecodeFromReader(r)
}

// Spatial4jShapeDecoderFactory creates Spatial4jShapeDecoder instances.
type Spatial4jShapeDecoderFactory struct {
	ctx *SpatialContext
}

// NewSpatial4jShapeDecoderFactory creates a new factory.
func NewSpatial4jShapeDecoderFactory(ctx *SpatialContext) *Spatial4jShapeDecoderFactory {
	return &Spatial4jShapeDecoderFactory{ctx: ctx}
}

// CreateDecoder creates a new Spatial4jShapeDecoder.
func (f *Spatial4jShapeDecoderFactory) CreateDecoder() *Spatial4jShapeDecoder {
	return NewSpatial4jShapeDecoder(f.ctx)
}

// Ensure Spatial4jShapeDecoder implements ShapeDecoder interfaces
var _ GeometryDecoder = (*Spatial4jShapeDecoder)(nil)
