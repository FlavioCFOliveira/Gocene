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

// JTSGeometrySerializer implements ShapeSerializer for JTS (Java Topology Suite)
// compatible geometry serialization using WKB (Well-Known Binary) format.
//
// WKB is the OGC standard binary format for geometry representation.
// This serializer supports Point, Rectangle (as Polygon), and provides
// extensibility for other geometry types.
//
// This is the Go port of Lucene's JTSGeometrySerializer.
type JTSGeometrySerializer struct {
	// byteOrder defines the byte order for serialization (default: LittleEndian)
	byteOrder binary.ByteOrder
}

// WKB Byte Order values
const (
	WKBXDR byte = 0 // Big Endian
	WKBNDR byte = 1 // Little Endian
)

// WKB Geometry Type codes (OGC standard)
const (
	WKBPoint              uint32 = 1
	WKBLineString         uint32 = 2
	WKBPolygon            uint32 = 3
	WKBMultiPoint         uint32 = 4
	WKBMultiLineString    uint32 = 5
	WKBMultiPolygon       uint32 = 6
	WKBGeometryCollection uint32 = 7
)

// WKB SRID flag - if set, SRID is included after byte order
const WKBGeometrySRIDFlag uint32 = 0x20000000

// NewJTSGeometrySerializer creates a new JTSGeometrySerializer with default settings.
// Uses LittleEndian byte order by default for efficiency on most modern hardware.
func NewJTSGeometrySerializer() *JTSGeometrySerializer {
	return &JTSGeometrySerializer{
		byteOrder: binary.LittleEndian,
	}
}

// NewJTSGeometrySerializerWithByteOrder creates a new JTSGeometrySerializer
// with a specific byte order.
func NewJTSGeometrySerializerWithByteOrder(order binary.ByteOrder) *JTSGeometrySerializer {
	return &JTSGeometrySerializer{
		byteOrder: order,
	}
}

// Serialize converts a shape to its WKB binary representation.
// Supports Point and Rectangle shapes. Rectangles are serialized as WKB Polygons.
func (j *JTSGeometrySerializer) Serialize(shape Shape) ([]byte, error) {
	if shape == nil {
		return nil, fmt.Errorf("cannot serialize nil shape")
	}

	var buf bytes.Buffer

	switch s := shape.(type) {
	case Point:
		return j.serializePoint(&buf, s)
	case *Rectangle:
		return j.serializeRectangle(&buf, s)
	default:
		// Try to serialize as bounding box rectangle
		bbox := shape.GetBoundingBox()
		if bbox != nil {
			return j.serializeRectangle(&buf, bbox)
		}
		return nil, fmt.Errorf("unsupported shape type for JTS serialization: %T", shape)
	}
}

// serializePoint serializes a Point to WKB format.
// WKB Point format:
//   - Byte order (1 byte)
//   - Geometry type (4 bytes): 1 for Point
//   - X coordinate (8 bytes, float64)
//   - Y coordinate (8 bytes, float64)
func (j *JTSGeometrySerializer) serializePoint(buf *bytes.Buffer, p Point) ([]byte, error) {
	// Write byte order
	if j.byteOrder == binary.LittleEndian {
		buf.WriteByte(WKBNDR)
	} else {
		buf.WriteByte(WKBXDR)
	}

	// Write geometry type (Point = 1)
	if err := binary.Write(buf, j.byteOrder, WKBPoint); err != nil {
		return nil, fmt.Errorf("failed to write point geometry type: %w", err)
	}

	// Write X coordinate
	if err := binary.Write(buf, j.byteOrder, p.X); err != nil {
		return nil, fmt.Errorf("failed to write point X: %w", err)
	}

	// Write Y coordinate
	if err := binary.Write(buf, j.byteOrder, p.Y); err != nil {
		return nil, fmt.Errorf("failed to write point Y: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeRectangle serializes a Rectangle as a WKB Polygon.
// WKB Polygon format:
//   - Byte order (1 byte)
//   - Geometry type (4 bytes): 3 for Polygon
//   - Num rings (4 bytes): 1 for simple rectangle
//   - Num points in ring (4 bytes): 5 (4 corners + closing point)
//   - Points: (minX, minY), (maxX, minY), (maxX, maxY), (minX, maxY), (minX, minY)
func (j *JTSGeometrySerializer) serializeRectangle(buf *bytes.Buffer, r *Rectangle) ([]byte, error) {
	// Write byte order
	if j.byteOrder == binary.LittleEndian {
		buf.WriteByte(WKBNDR)
	} else {
		buf.WriteByte(WKBXDR)
	}

	// Write geometry type (Polygon = 3)
	if err := binary.Write(buf, j.byteOrder, WKBPolygon); err != nil {
		return nil, fmt.Errorf("failed to write polygon geometry type: %w", err)
	}

	// Write number of rings (1 for rectangle)
	if err := binary.Write(buf, j.byteOrder, uint32(1)); err != nil {
		return nil, fmt.Errorf("failed to write number of rings: %w", err)
	}

	// Write number of points in ring (5: 4 corners + closing point)
	if err := binary.Write(buf, j.byteOrder, uint32(5)); err != nil {
		return nil, fmt.Errorf("failed to write number of points: %w", err)
	}

	// Write points in order: SW, SE, NE, NW, SW (closing)
	points := [5]struct{ x, y float64 }{
		{r.MinX, r.MinY}, // SW
		{r.MaxX, r.MinY}, // SE
		{r.MaxX, r.MaxY}, // NE
		{r.MinX, r.MaxY}, // NW
		{r.MinX, r.MinY}, // SW (closing)
	}

	for i, pt := range points {
		if err := binary.Write(buf, j.byteOrder, pt.x); err != nil {
			return nil, fmt.Errorf("failed to write point %d X: %w", i, err)
		}
		if err := binary.Write(buf, j.byteOrder, pt.y); err != nil {
			return nil, fmt.Errorf("failed to write point %d Y: %w", i, err)
		}
	}

	return buf.Bytes(), nil
}

// Deserialize converts WKB binary data back to a shape.
// Supports Point and Polygon (returned as Rectangle) types.
func (j *JTSGeometrySerializer) Deserialize(data []byte) (Shape, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot deserialize empty data")
	}

	buf := bytes.NewReader(data)

	// Read byte order
	var byteOrder byte
	if err := binary.Read(buf, binary.LittleEndian, &byteOrder); err != nil {
		return nil, fmt.Errorf("failed to read byte order: %w", err)
	}

	// Determine byte order for reading
	var order binary.ByteOrder
	if byteOrder == WKBNDR {
		order = binary.LittleEndian
	} else if byteOrder == WKBXDR {
		order = binary.BigEndian
	} else {
		return nil, fmt.Errorf("invalid byte order: %d", byteOrder)
	}

	// Read geometry type
	var geomType uint32
	if err := binary.Read(buf, order, &geomType); err != nil {
		return nil, fmt.Errorf("failed to read geometry type: %w", err)
	}

	// Check for SRID flag (not supported in this implementation)
	if geomType&WKBGeometrySRIDFlag != 0 {
		// Skip SRID (4 bytes)
		var srid uint32
		if err := binary.Read(buf, order, &srid); err != nil {
			return nil, fmt.Errorf("failed to read SRID: %w", err)
		}
		geomType &^= WKBGeometrySRIDFlag
	}

	switch geomType {
	case WKBPoint:
		return j.deserializePoint(buf, order)
	case WKBPolygon:
		return j.deserializePolygon(buf, order)
	default:
		return nil, fmt.Errorf("unsupported WKB geometry type: %d", geomType)
	}
}

// deserializePoint deserializes a WKB Point.
func (j *JTSGeometrySerializer) deserializePoint(r io.Reader, order binary.ByteOrder) (Shape, error) {
	var x, y float64
	if err := binary.Read(r, order, &x); err != nil {
		return nil, fmt.Errorf("failed to read point X: %w", err)
	}
	if err := binary.Read(r, order, &y); err != nil {
		return nil, fmt.Errorf("failed to read point Y: %w", err)
	}
	return NewPoint(x, y), nil
}

// deserializePolygon deserializes a WKB Polygon.
// For rectangles (single ring with 5 points), returns a Rectangle.
// Otherwise, returns the bounding box of the polygon.
func (j *JTSGeometrySerializer) deserializePolygon(r io.Reader, order binary.ByteOrder) (Shape, error) {
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

	// Read all points and compute bounding box
	var minX, minY, maxX, maxY float64
	firstPoint := true

	for i := uint32(0); i < numPoints; i++ {
		var x, y float64
		if err := binary.Read(r, order, &x); err != nil {
			return nil, fmt.Errorf("failed to read point %d X: %w", i, err)
		}
		if err := binary.Read(r, order, &y); err != nil {
			return nil, fmt.Errorf("failed to read point %d Y: %w", i, err)
		}

		if firstPoint {
			minX, maxX = x, x
			minY, maxY = y, y
			firstPoint = false
		} else {
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
	}

	// Skip remaining rings if any
	for i := uint32(1); i < numRings; i++ {
		var ringPoints uint32
		if err := binary.Read(r, order, &ringPoints); err != nil {
			return nil, fmt.Errorf("failed to read ring %d point count: %w", i, err)
		}
		// Skip points
		for j := uint32(0); j < ringPoints*2; j++ {
			var val float64
			if err := binary.Read(r, order, &val); err != nil {
				return nil, fmt.Errorf("failed to skip ring point: %w", err)
			}
		}
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// GetByteOrder returns the byte order used by this serializer.
func (j *JTSGeometrySerializer) GetByteOrder() binary.ByteOrder {
	return j.byteOrder
}

// SetByteOrder sets the byte order for this serializer.
func (j *JTSGeometrySerializer) SetByteOrder(order binary.ByteOrder) {
	j.byteOrder = order
}

// SerializeTo serializes a shape to the given writer in WKB format.
// This is more efficient than Serialize when writing directly to an output stream.
func (j *JTSGeometrySerializer) SerializeTo(w io.Writer, shape Shape) error {
	if shape == nil {
		return fmt.Errorf("cannot serialize nil shape")
	}

	switch s := shape.(type) {
	case Point:
		return j.serializePointTo(w, s)
	case *Rectangle:
		return j.serializeRectangleTo(w, s)
	default:
		bbox := shape.GetBoundingBox()
		if bbox != nil {
			return j.serializeRectangleTo(w, bbox)
		}
		return fmt.Errorf("unsupported shape type for JTS serialization: %T", shape)
	}
}

// serializePointTo serializes a Point directly to a writer.
func (j *JTSGeometrySerializer) serializePointTo(w io.Writer, p Point) error {
	// Write byte order
	if j.byteOrder == binary.LittleEndian {
		if _, err := w.Write([]byte{WKBNDR}); err != nil {
			return err
		}
	} else {
		if _, err := w.Write([]byte{WKBXDR}); err != nil {
			return err
		}
	}

	// Write geometry type
	if err := binary.Write(w, j.byteOrder, WKBPoint); err != nil {
		return err
	}

	// Write coordinates
	if err := binary.Write(w, j.byteOrder, p.X); err != nil {
		return err
	}
	if err := binary.Write(w, j.byteOrder, p.Y); err != nil {
		return err
	}

	return nil
}

// serializeRectangleTo serializes a Rectangle directly to a writer.
func (j *JTSGeometrySerializer) serializeRectangleTo(w io.Writer, r *Rectangle) error {
	// Write byte order
	if j.byteOrder == binary.LittleEndian {
		if _, err := w.Write([]byte{WKBNDR}); err != nil {
			return err
		}
	} else {
		if _, err := w.Write([]byte{WKBXDR}); err != nil {
			return err
		}
	}

	// Write geometry type
	if err := binary.Write(w, j.byteOrder, WKBPolygon); err != nil {
		return err
	}

	// Write number of rings
	if err := binary.Write(w, j.byteOrder, uint32(1)); err != nil {
		return err
	}

	// Write number of points
	if err := binary.Write(w, j.byteOrder, uint32(5)); err != nil {
		return err
	}

	// Write points
	points := [5]struct{ x, y float64 }{
		{r.MinX, r.MinY},
		{r.MaxX, r.MinY},
		{r.MaxX, r.MaxY},
		{r.MinX, r.MaxY},
		{r.MinX, r.MinY},
	}

	for _, pt := range points {
		if err := binary.Write(w, j.byteOrder, pt.x); err != nil {
			return err
		}
		if err := binary.Write(w, j.byteOrder, pt.y); err != nil {
			return err
		}
	}

	return nil
}

// GetFormatName returns the name of the serialization format.
func (j *JTSGeometrySerializer) GetFormatName() string {
	return "WKB"
}

// GetFormatVersion returns the version of the serialization format.
func (j *JTSGeometrySerializer) GetFormatVersion() string {
	return "1.2.0" // OGC Simple Features Specification
}

// Ensure JTSGeometrySerializer implements ShapeSerializer
var _ ShapeSerializer = (*JTSGeometrySerializer)(nil)
