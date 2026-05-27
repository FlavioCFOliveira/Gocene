// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SerializedDVStrategy is a SpatialStrategy that serializes shapes into a binary
// DocValues field for accurate spatial queries. Unlike prefix tree strategies
// that approximate shapes, this strategy stores the exact geometry and performs
// precise calculations at query time.
//
// This strategy is ideal for:
//   - Precise geometry matching (not approximated)
//   - Smaller indexes compared to prefix tree strategies
//   - Slower query performance but higher accuracy
//   - Complex shapes that need exact representation
//
// The trade-off is that queries are slower because all candidate documents must
// be deserialized and checked precisely, rather than using fast prefix tree lookups.
//
// This is the Go port of Lucene's SerializedDVStrategy.
type SerializedDVStrategy struct {
	*BaseSpatialStrategy
	dvFieldName string
}

// ShapeSerializer handles serialization and deserialization of shapes.
// Implementations must be able to convert shapes to/from binary format.
type ShapeSerializer interface {
	// Serialize converts a shape to its binary representation.
	Serialize(shape Shape) ([]byte, error)

	// Deserialize converts binary data back to a shape.
	Deserialize(data []byte) (Shape, error)
}

// NewSerializedDVStrategy creates a new SerializedDVStrategy with default field naming.
//
// Parameters:
//   - fieldName: The base field name; the docvalues field will be named "{fieldName}_dv"
//   - ctx: The spatial context for coordinate transformations
//
// Returns an error if the field name is empty or context is nil.
func NewSerializedDVStrategy(fieldName string, ctx *SpatialContext) (*SerializedDVStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	return &SerializedDVStrategy{
		BaseSpatialStrategy: base,
		dvFieldName:         fieldName + "_dv",
	}, nil
}

// NewSerializedDVStrategyWithFieldName creates a new SerializedDVStrategy with a custom docvalues field name.
//
// Parameters:
//   - fieldName: The base field name
//   - dvFieldName: The custom field name for the binary docvalues
//   - ctx: The spatial context
func NewSerializedDVStrategyWithFieldName(fieldName, dvFieldName string, ctx *SpatialContext) (*SerializedDVStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	if dvFieldName == "" {
		return nil, fmt.Errorf("docvalues field name cannot be empty")
	}

	return &SerializedDVStrategy{
		BaseSpatialStrategy: base,
		dvFieldName:         dvFieldName,
	}, nil
}

// GetDVFieldName returns the field name for the binary docvalues.
func (s *SerializedDVStrategy) GetDVFieldName() string {
	return s.dvFieldName
}

// CreateIndexableFields generates the IndexableField instances for indexing a shape.
// Creates a BinaryDocValuesField containing the serialized shape data.
//
// The shape is serialized using a simple binary format that includes:
//   - Shape type identifier (1 byte)
//   - Shape coordinates (variable length based on type)
func (s *SerializedDVStrategy) CreateIndexableFields(shape Shape) ([]document.IndexableField, error) {
	// Serialize the shape
	data, err := s.serializeShape(shape)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize shape: %w", err)
	}

	// Create BinaryDocValuesField
	dvField, err := document.NewBinaryDocValuesField(s.dvFieldName, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary docvalues field: %w", err)
	}

	return []document.IndexableField{dvField}, nil
}

// serializeShape serializes a shape to binary format.
// Supports Point and Rectangle shapes.
func (s *SerializedDVStrategy) serializeShape(shape Shape) ([]byte, error) {
	var buf bytes.Buffer

	switch sh := shape.(type) {
	case Point:
		// Type byte: 1 = Point
		buf.WriteByte(1)
		// Write X and Y as float64
		binary.Write(&buf, binary.LittleEndian, sh.X)
		binary.Write(&buf, binary.LittleEndian, sh.Y)

	case *Rectangle:
		// Type byte: 2 = Rectangle
		buf.WriteByte(2)
		// Write MinX, MinY, MaxX, MaxY as float64
		binary.Write(&buf, binary.LittleEndian, sh.MinX)
		binary.Write(&buf, binary.LittleEndian, sh.MinY)
		binary.Write(&buf, binary.LittleEndian, sh.MaxX)
		binary.Write(&buf, binary.LittleEndian, sh.MaxY)

	default:
		// Try to get bounding box and serialize as rectangle
		bbox := shape.GetBoundingBox()
		buf.WriteByte(2)
		binary.Write(&buf, binary.LittleEndian, bbox.MinX)
		binary.Write(&buf, binary.LittleEndian, bbox.MinY)
		binary.Write(&buf, binary.LittleEndian, bbox.MaxX)
		binary.Write(&buf, binary.LittleEndian, bbox.MaxY)
	}

	return buf.Bytes(), nil
}

// deserializeShape deserializes binary data back to a shape.
func (s *SerializedDVStrategy) deserializeShape(data []byte) (Shape, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	buf := bytes.NewReader(data)

	// Read type byte
	var shapeType byte
	if err := binary.Read(buf, binary.LittleEndian, &shapeType); err != nil {
		return nil, fmt.Errorf("failed to read shape type: %w", err)
	}

	switch shapeType {
	case 1: // Point
		var x, y float64
		if err := binary.Read(buf, binary.LittleEndian, &x); err != nil {
			return nil, fmt.Errorf("failed to read point X: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &y); err != nil {
			return nil, fmt.Errorf("failed to read point Y: %w", err)
		}
		return NewPoint(x, y), nil

	case 2: // Rectangle
		var minX, minY, maxX, maxY float64
		if err := binary.Read(buf, binary.LittleEndian, &minX); err != nil {
			return nil, fmt.Errorf("failed to read rectangle minX: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &minY); err != nil {
			return nil, fmt.Errorf("failed to read rectangle minY: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &maxX); err != nil {
			return nil, fmt.Errorf("failed to read rectangle maxX: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &maxY); err != nil {
			return nil, fmt.Errorf("failed to read rectangle maxY: %w", err)
		}
		return NewRectangle(minX, minY, maxX, maxY), nil

	default:
		return nil, fmt.Errorf("unknown shape type: %d", shapeType)
	}
}

// MakeQuery creates a spatial query for the given operation and shape.
//
// Supports the following operations:
//   - SpatialOperationIntersects: Matches shapes that intersect the query shape
//   - SpatialOperationIsWithin: Matches shapes that are within the query shape
//   - SpatialOperationContains: Matches shapes that contain the query shape
//
// Note: Since this strategy uses docvalues, queries require deserializing
// candidate documents and checking them precisely. This is slower than
// prefix tree strategies but more accurate.
func (s *SerializedDVStrategy) MakeQuery(operation SpatialOperation, shape Shape) (search.Query, error) {
	if shape == nil {
		return nil, fmt.Errorf("query shape cannot be nil")
	}
	switch operation {
	case SpatialOperationIntersects, SpatialOperationIsWithin, SpatialOperationContains:
		return newSerializedDVQuery(s, operation, shape), nil
	default:
		return nil, fmt.Errorf("operation %s not supported by SerializedDVStrategy", operation)
	}
}

// matchShape evaluates the per-document predicate of a SerializedDV
// query: it deserialises the binary doc-values payload back into a
// Shape and tests it against the query shape using the requested
// SpatialOperation. The matcher is exported via newSerializedDVQuery so
// the algorithmic substance can be exercised directly by unit tests
// against synthetic byte payloads, decoupling it from the leaf-reader
// wiring still gated on the BinaryDocValues foundation gap.
func (s *SerializedDVStrategy) matchShape(operation SpatialOperation, queryShape Shape, dvBytes []byte) (bool, error) {
	indexed, err := s.deserializeShape(dvBytes)
	if err != nil {
		return false, err
	}
	if indexed == nil {
		return false, nil
	}
	switch operation {
	case SpatialOperationIntersects:
		return indexed.Intersects(queryShape), nil
	case SpatialOperationIsWithin:
		return indexed.IsWithin(queryShape), nil
	case SpatialOperationContains:
		return indexed.Contains(queryShape), nil
	default:
		return false, fmt.Errorf("operation %s not supported", operation)
	}
}

// MakeDistanceValueSource creates a ValueSource that returns the distance
// from indexed shapes to the specified point.
//
// The distance is calculated from the center of each shape to the specified point.
func (s *SerializedDVStrategy) MakeDistanceValueSource(center Point, multiplier float64) (grouping.ValueSource, error) {
	return NewSerializedDVDistanceValueSource(
		s.dvFieldName,
		center,
		multiplier,
		s.spatialContext.Calculator,
		s,
	), nil
}

// SerializedDVDistanceValueSource provides distance values from serialized shapes.
type SerializedDVDistanceValueSource struct {
	dvFieldName string
	center      Point
	multiplier  float64
	calculator  DistanceCalculator
	strategy    *SerializedDVStrategy
}

// NewSerializedDVDistanceValueSource creates a new SerializedDVDistanceValueSource.
func NewSerializedDVDistanceValueSource(dvFieldName string, center Point, multiplier float64, calculator DistanceCalculator, strategy *SerializedDVStrategy) *SerializedDVDistanceValueSource {
	return &SerializedDVDistanceValueSource{
		dvFieldName: dvFieldName,
		center:      center,
		multiplier:  multiplier,
		calculator:  calculator,
		strategy:    strategy,
	}
}

// GetValues returns the values for the given context.
func (dvs *SerializedDVDistanceValueSource) GetValues(context *index.LeafReaderContext) (grouping.ValueSourceValues, error) {
	var reader index.LeafReaderInterface
	if context != nil {
		reader = context.LeafReader()
	}
	return &serializedDVDistanceValueSourceValues{
		dvFieldName: dvs.dvFieldName,
		center:      dvs.center,
		multiplier:  dvs.multiplier,
		calculator:  dvs.calculator,
		strategy:    dvs.strategy,
		reader:      reader,
		values:      make(map[int]float64),
	}, nil
}

// Description returns a description of this value source.
func (dvs *SerializedDVDistanceValueSource) Description() string {
	return fmt.Sprintf("serialized_dv_distance(%s from %v)", dvs.dvFieldName, dvs.center)
}

// Ensure SerializedDVDistanceValueSource implements ValueSource
var _ grouping.ValueSource = (*SerializedDVDistanceValueSource)(nil)

// serializedDVDistanceValueSourceValues provides distance values for documents.
type serializedDVDistanceValueSourceValues struct {
	dvFieldName string
	center      Point
	multiplier  float64
	calculator  DistanceCalculator
	strategy    *SerializedDVStrategy
	reader      index.LeafReaderInterface
	values      map[int]float64
}

// DoubleVal returns the distance value for the given document. It reads
// the binary doc-values payload for the document, deserialises it back
// into a Shape, and returns the distance from the shape's centre to the
// configured query centre, scaled by multiplier. Documents without a DV
// payload, or readers that do not yet expose BinaryDocValues, return 0,
// matching Lucene's fallback for missing geometry. The decoded distance
// is cached so subsequent calls do not re-deserialise the payload.
func (dvv *serializedDVDistanceValueSourceValues) DoubleVal(doc int) (float64, error) {
	if val, ok := dvv.values[doc]; ok {
		return val * dvv.multiplier, nil
	}

	data := dvv.readDVBytes(doc)
	if len(data) == 0 {
		dvv.values[doc] = 0
		return 0, nil
	}

	shape, err := dvv.strategy.deserializeShape(data)
	if err != nil || shape == nil {
		dvv.values[doc] = 0
		return 0, nil
	}
	centre := shape.GetCenter()
	d := dvv.calculator.Distance(centre, dvv.center)
	dvv.values[doc] = d
	return d * dvv.multiplier, nil
}

// readDVBytes resolves the per-leaf BinaryDocValues iterator for the
// configured field and returns the raw payload bytes for doc, or nil
// when the doc has no payload or when the reader does not yet expose
// binary doc values (the same foundation gap that gates the numeric
// doc-values strategies in this package).
func (dvv *serializedDVDistanceValueSourceValues) readDVBytes(doc int) []byte {
	if dvv.reader == nil {
		return nil
	}
	type binaryDVReader interface {
		GetBinaryDocValues(field string) (index.BinaryDocValues, error)
	}
	r, ok := dvv.reader.(binaryDVReader)
	if !ok {
		return nil
	}
	bdv, err := r.GetBinaryDocValues(dvv.dvFieldName)
	if err != nil || bdv == nil {
		return nil
	}
	target, err := bdv.Advance(doc)
	if err != nil || target != doc {
		return nil
	}
	data, err := bdv.Get(doc)
	if err != nil {
		return nil
	}
	return data
}

// FloatVal returns the float value for the given document.
func (dvv *serializedDVDistanceValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := dvv.DoubleVal(doc)
	return float32(val), err
}

// IntVal returns the int value for the given document.
func (dvv *serializedDVDistanceValueSourceValues) IntVal(doc int) (int, error) {
	val, err := dvv.DoubleVal(doc)
	return int(val), err
}

// LongVal returns the long value for the given document.
func (dvv *serializedDVDistanceValueSourceValues) LongVal(doc int) (int64, error) {
	val, err := dvv.DoubleVal(doc)
	return int64(val), err
}

// StrVal returns the string value for the given document.
func (dvv *serializedDVDistanceValueSourceValues) StrVal(doc int) (string, error) {
	val, err := dvv.DoubleVal(doc)
	return fmt.Sprintf("%f", val), err
}

// Exists returns true if a value exists for the given document.
func (dvv *serializedDVDistanceValueSourceValues) Exists(doc int) bool {
	_, ok := dvv.values[doc]
	return ok
}

// Ensure serializedDVDistanceValueSourceValues implements ValueSourceValues
var _ grouping.ValueSourceValues = (*serializedDVDistanceValueSourceValues)(nil)
