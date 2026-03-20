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

// ShapeValue represents a single spatial shape value.
// This is used to store and retrieve individual shape data from documents.
//
// Unlike ShapeValues (which is a collection), ShapeValue represents a single
// shape instance with its associated metadata.
//
// This is the Go port of Lucene's ShapeValue concept.
type ShapeValue struct {
	// shape is the actual spatial shape
	shape Shape

	// fieldName is the name of the field this value belongs to
	fieldName string

	// docID is the document ID this value is associated with
	docID int

	// version is the serialization version for backward compatibility
	version int
}

// ShapeValueVersion is the current serialization version.
const ShapeValueVersion = 1

// NewShapeValue creates a new ShapeValue with the given shape and metadata.
//
// Parameters:
//   - shape: The spatial shape
//   - fieldName: The name of the field
//   - docID: The document ID
//
// Returns a new ShapeValue instance.
func NewShapeValue(shape Shape, fieldName string, docID int) *ShapeValue {
	return &ShapeValue{
		shape:     shape,
		fieldName: fieldName,
		docID:     docID,
		version:   ShapeValueVersion,
	}
}

// GetShape returns the spatial shape.
func (sv *ShapeValue) GetShape() Shape {
	return sv.shape
}

// GetFieldName returns the field name.
func (sv *ShapeValue) GetFieldName() string {
	return sv.fieldName
}

// GetDocID returns the document ID.
func (sv *ShapeValue) GetDocID() int {
	return sv.docID
}

// GetVersion returns the serialization version.
func (sv *ShapeValue) GetVersion() int {
	return sv.version
}

// GetBoundingBox returns the bounding box of the shape.
func (sv *ShapeValue) GetBoundingBox() *Rectangle {
	if sv.shape == nil {
		return nil
	}
	return sv.shape.GetBoundingBox()
}

// GetCenter returns the center point of the shape.
func (sv *ShapeValue) GetCenter() Point {
	if sv.shape == nil {
		return Point{}
	}
	return sv.shape.GetCenter()
}

// IsEmpty returns true if this shape value is empty (has no shape).
func (sv *ShapeValue) IsEmpty() bool {
	return sv.shape == nil
}

// String returns a string representation of this shape value.
func (sv *ShapeValue) String() string {
	if sv.IsEmpty() {
		return fmt.Sprintf("ShapeValue(empty, doc=%d, field=%s)", sv.docID, sv.fieldName)
	}
	return fmt.Sprintf("ShapeValue(shape=%v, doc=%d, field=%s)", sv.shape, sv.docID, sv.fieldName)
}

// Serialize serializes this ShapeValue to a byte slice.
// This is used for storing shape values in the index.
//
// Returns the serialized bytes or an error if serialization fails.
func (sv *ShapeValue) Serialize() ([]byte, error) {
	if sv.shape == nil {
		return nil, fmt.Errorf("cannot serialize nil shape")
	}

	buf := new(bytes.Buffer)

	// Write version
	if err := binary.Write(buf, binary.LittleEndian, int32(sv.version)); err != nil {
		return nil, fmt.Errorf("failed to write version: %w", err)
	}

	// Write docID
	if err := binary.Write(buf, binary.LittleEndian, int32(sv.docID)); err != nil {
		return nil, fmt.Errorf("failed to write docID: %w", err)
	}

	// Write field name length and field name
	fieldNameBytes := []byte(sv.fieldName)
	if err := binary.Write(buf, binary.LittleEndian, int32(len(fieldNameBytes))); err != nil {
		return nil, fmt.Errorf("failed to write field name length: %w", err)
	}
	if _, err := buf.Write(fieldNameBytes); err != nil {
		return nil, fmt.Errorf("failed to write field name: %w", err)
	}

	// Write shape type
	shapeType := getShapeType(sv.shape)
	if err := binary.Write(buf, binary.LittleEndian, int32(shapeType)); err != nil {
		return nil, fmt.Errorf("failed to write shape type: %w", err)
	}

	// Write shape data
	if err := serializeShape(buf, sv.shape); err != nil {
		return nil, fmt.Errorf("failed to serialize shape: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeShapeValue deserializes a ShapeValue from a byte slice.
//
// Parameters:
//   - data: The serialized bytes
//
// Returns the deserialized ShapeValue or an error.
func DeserializeShapeValue(data []byte) (*ShapeValue, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot deserialize empty data")
	}

	buf := bytes.NewReader(data)

	// Read version
	var version int32
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	// Read docID
	var docID int32
	if err := binary.Read(buf, binary.LittleEndian, &docID); err != nil {
		return nil, fmt.Errorf("failed to read docID: %w", err)
	}

	// Read field name length and field name
	var fieldNameLen int32
	if err := binary.Read(buf, binary.LittleEndian, &fieldNameLen); err != nil {
		return nil, fmt.Errorf("failed to read field name length: %w", err)
	}

	fieldNameBytes := make([]byte, fieldNameLen)
	if _, err := io.ReadFull(buf, fieldNameBytes); err != nil {
		return nil, fmt.Errorf("failed to read field name: %w", err)
	}
	fieldName := string(fieldNameBytes)

	// Read shape type
	var shapeType int32
	if err := binary.Read(buf, binary.LittleEndian, &shapeType); err != nil {
		return nil, fmt.Errorf("failed to read shape type: %w", err)
	}

	// Deserialize shape
	shape, err := deserializeShape(buf, ShapeType(shapeType))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize shape: %w", err)
	}

	return &ShapeValue{
		shape:     shape,
		fieldName: fieldName,
		docID:     int(docID),
		version:   int(version),
	}, nil
}

// ShapeType represents the type of spatial shape for serialization.
type ShapeType int32

const (
	// ShapeTypePoint represents a Point shape.
	ShapeTypePoint ShapeType = 1

	// ShapeTypeRectangle represents a Rectangle shape.
	ShapeTypeRectangle ShapeType = 2

	// ShapeTypeUnknown represents an unknown shape type.
	ShapeTypeUnknown ShapeType = 0
)

// getShapeType returns the ShapeType for a given shape.
func getShapeType(shape Shape) ShapeType {
	switch shape.(type) {
	case Point:
		return ShapeTypePoint
	case *Rectangle:
		return ShapeTypeRectangle
	default:
		return ShapeTypeUnknown
	}
}

// serializeShape serializes a shape to the given writer.
func serializeShape(w io.Writer, shape Shape) error {
	switch s := shape.(type) {
	case Point:
		// Write X and Y coordinates
		if err := binary.Write(w, binary.LittleEndian, s.X); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, s.Y); err != nil {
			return err
		}
	case *Rectangle:
		// Write min/max coordinates
		if err := binary.Write(w, binary.LittleEndian, s.MinX); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, s.MinY); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, s.MaxX); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, s.MaxY); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported shape type: %T", shape)
	}
	return nil
}

// deserializeShape deserializes a shape from the given reader.
func deserializeShape(r io.Reader, shapeType ShapeType) (Shape, error) {
	switch shapeType {
	case ShapeTypePoint:
		var x, y float64
		if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
			return nil, err
		}
		return NewPoint(x, y), nil

	case ShapeTypeRectangle:
		var minX, minY, maxX, maxY float64
		if err := binary.Read(r, binary.LittleEndian, &minX); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &minY); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &maxX); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &maxY); err != nil {
			return nil, err
		}
		return NewRectangle(minX, minY, maxX, maxY), nil

	default:
		return nil, fmt.Errorf("unknown shape type: %d", shapeType)
	}
}

// ShapeValueComparator compares two ShapeValue instances.
// Returns:
//   - negative if sv1 < sv2
//   - zero if sv1 == sv2
//   - positive if sv1 > sv2
type ShapeValueComparator func(sv1, sv2 *ShapeValue) int

// CompareByDocID compares ShapeValues by document ID.
func CompareByDocID(sv1, sv2 *ShapeValue) int {
	return sv1.docID - sv2.docID
}

// CompareByFieldName compares ShapeValues by field name.
func CompareByFieldName(sv1, sv2 *ShapeValue) int {
	if sv1.fieldName < sv2.fieldName {
		return -1
	}
	if sv1.fieldName > sv2.fieldName {
		return 1
	}
	return 0
}

// ShapeValueFilter is a function type for filtering ShapeValues.
type ShapeValueFilter func(sv *ShapeValue) bool

// FilterByFieldName returns a filter that matches a specific field name.
func FilterByFieldName(fieldName string) ShapeValueFilter {
	return func(sv *ShapeValue) bool {
		return sv.fieldName == fieldName
	}
}

// FilterByDocID returns a filter that matches a specific document ID.
func FilterByDocID(docID int) ShapeValueFilter {
	return func(sv *ShapeValue) bool {
		return sv.docID == docID
	}
}

// FilterNonEmpty returns a filter that matches non-empty shape values.
func FilterNonEmpty() ShapeValueFilter {
	return func(sv *ShapeValue) bool {
		return !sv.IsEmpty()
	}
}

// ShapeValueList is a list of ShapeValue instances.
type ShapeValueList []*ShapeValue

// Filter returns a new list containing only ShapeValues that match the filter.
func (list ShapeValueList) Filter(filter ShapeValueFilter) ShapeValueList {
	result := make(ShapeValueList, 0)
	for _, sv := range list {
		if filter(sv) {
			result = append(result, sv)
		}
	}
	return result
}

// GetByDocID returns all ShapeValues for a specific document ID.
func (list ShapeValueList) GetByDocID(docID int) ShapeValueList {
	return list.Filter(FilterByDocID(docID))
}

// GetByFieldName returns all ShapeValues for a specific field name.
func (list ShapeValueList) GetByFieldName(fieldName string) ShapeValueList {
	return list.Filter(FilterByFieldName(fieldName))
}

// Len returns the length of the list (for sort.Interface).
func (list ShapeValueList) Len() int {
	return len(list)
}

// Swap swaps two elements in the list (for sort.Interface).
func (list ShapeValueList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// ShapeValueListByDocID sorts ShapeValueList by document ID.
type ShapeValueListByDocID ShapeValueList

func (list ShapeValueListByDocID) Len() int      { return len(list) }
func (list ShapeValueListByDocID) Swap(i, j int) { list[i], list[j] = list[j], list[i] }
func (list ShapeValueListByDocID) Less(i, j int) bool {
	return list[i].GetDocID() < list[j].GetDocID()
}

// ShapeValueListByFieldName sorts ShapeValueList by field name.
type ShapeValueListByFieldName ShapeValueList

func (list ShapeValueListByFieldName) Len() int      { return len(list) }
func (list ShapeValueListByFieldName) Swap(i, j int) { list[i], list[j] = list[j], list[i] }
func (list ShapeValueListByFieldName) Less(i, j int) bool {
	return list[i].GetFieldName() < list[j].GetFieldName()
}
