// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestNewShapeValue(t *testing.T) {
	point := NewPoint(10, 20)
	sv := NewShapeValue(point, "location", 5)

	if sv == nil {
		t.Fatal("expected non-nil ShapeValue")
	}

	if sv.GetShape() != point {
		t.Error("shape mismatch")
	}

	if sv.GetFieldName() != "location" {
		t.Errorf("expected field name 'location', got '%s'", sv.GetFieldName())
	}

	if sv.GetDocID() != 5 {
		t.Errorf("expected docID 5, got %d", sv.GetDocID())
	}

	if sv.GetVersion() != ShapeValueVersion {
		t.Errorf("expected version %d, got %d", ShapeValueVersion, sv.GetVersion())
	}
}

func TestShapeValue_GetBoundingBox(t *testing.T) {
	tests := []struct {
		name     string
		shape    Shape
		expected *Rectangle
	}{
		{
			name:     "point bounding box",
			shape:    NewPoint(10, 20),
			expected: NewRectangle(10, 20, 10, 20),
		},
		{
			name:     "rectangle bounding box",
			shape:    NewRectangle(0, 0, 10, 10),
			expected: NewRectangle(0, 0, 10, 10),
		},
		{
			name:     "nil shape",
			shape:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := NewShapeValue(tt.shape, "test", 1)
			bbox := sv.GetBoundingBox()

			if tt.expected == nil {
				if bbox != nil {
					t.Error("expected nil bounding box")
				}
				return
			}

			if bbox == nil {
				t.Fatal("expected non-nil bounding box")
			}

			if bbox.MinX != tt.expected.MinX || bbox.MinY != tt.expected.MinY ||
				bbox.MaxX != tt.expected.MaxX || bbox.MaxY != tt.expected.MaxY {
				t.Errorf("bounding box mismatch: got %v, expected %v", bbox, tt.expected)
			}
		})
	}
}

func TestShapeValue_GetCenter(t *testing.T) {
	point := NewPoint(10, 20)
	sv := NewShapeValue(point, "location", 1)

	center := sv.GetCenter()
	if center.X != 10 || center.Y != 20 {
		t.Errorf("expected center (10, 20), got (%f, %f)", center.X, center.Y)
	}

	// Test with nil shape
	emptySV := NewShapeValue(nil, "empty", 2)
	emptyCenter := emptySV.GetCenter()
	if emptyCenter.X != 0 || emptyCenter.Y != 0 {
		t.Errorf("expected zero center for nil shape, got (%f, %f)", emptyCenter.X, emptyCenter.Y)
	}
}

func TestShapeValue_IsEmpty(t *testing.T) {
	svWithShape := NewShapeValue(NewPoint(1, 2), "location", 1)
	if svWithShape.IsEmpty() {
		t.Error("ShapeValue with shape should not be empty")
	}

	svWithoutShape := NewShapeValue(nil, "location", 1)
	if !svWithoutShape.IsEmpty() {
		t.Error("ShapeValue without shape should be empty")
	}
}

func TestShapeValue_String(t *testing.T) {
	sv := NewShapeValue(NewPoint(10, 20), "location", 5)
	str := sv.String()
	if str == "" {
		t.Error("String() should not return empty")
	}

	// Should contain field name and docID
	if len(str) < 10 {
		t.Error("String() should return meaningful content")
	}

	// Test empty shape
	emptySV := NewShapeValue(nil, "empty", 1)
	emptyStr := emptySV.String()
	if emptyStr == "" {
		t.Error("String() for empty shape should not return empty")
	}
}

func TestShapeValue_SerializeDeserialize_Point(t *testing.T) {
	original := NewShapeValue(NewPoint(10.5, 20.5), "location", 5)

	// Serialize
	data, err := original.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	if len(data) == 0 {
		t.Error("serialized data should not be empty")
	}

	// Deserialize
	restored, err := DeserializeShapeValue(data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify
	if restored.GetFieldName() != original.GetFieldName() {
		t.Errorf("field name mismatch: got '%s', expected '%s'", restored.GetFieldName(), original.GetFieldName())
	}

	if restored.GetDocID() != original.GetDocID() {
		t.Errorf("docID mismatch: got %d, expected %d", restored.GetDocID(), original.GetDocID())
	}

	restoredPoint, ok := restored.GetShape().(Point)
	if !ok {
		t.Fatal("restored shape should be a Point")
	}

	originalPoint := original.GetShape().(Point)
	if restoredPoint.X != originalPoint.X || restoredPoint.Y != originalPoint.Y {
		t.Errorf("point mismatch: got (%f, %f), expected (%f, %f)", restoredPoint.X, restoredPoint.Y, originalPoint.X, originalPoint.Y)
	}
}

func TestShapeValue_SerializeDeserialize_Rectangle(t *testing.T) {
	original := NewShapeValue(NewRectangle(0, 0, 100, 100), "bbox", 10)

	// Serialize
	data, err := original.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Deserialize
	restored, err := DeserializeShapeValue(data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify
	restoredRect, ok := restored.GetShape().(*Rectangle)
	if !ok {
		t.Fatal("restored shape should be a Rectangle")
	}

	originalRect := original.GetShape().(*Rectangle)
	if restoredRect.MinX != originalRect.MinX || restoredRect.MinY != originalRect.MinY ||
		restoredRect.MaxX != originalRect.MaxX || restoredRect.MaxY != originalRect.MaxY {
		t.Errorf("rectangle mismatch: got %v, expected %v", restoredRect, originalRect)
	}
}

func TestShapeValue_Serialize_NilShape(t *testing.T) {
	sv := NewShapeValue(nil, "empty", 1)
	_, err := sv.Serialize()
	if err == nil {
		t.Error("expected error when serializing nil shape")
	}
}

func TestDeserializeShapeValue_EmptyData(t *testing.T) {
	_, err := DeserializeShapeValue([]byte{})
	if err == nil {
		t.Error("expected error when deserializing empty data")
	}
}

func TestCompareByDocID(t *testing.T) {
	sv1 := NewShapeValue(NewPoint(0, 0), "field1", 1)
	sv2 := NewShapeValue(NewPoint(0, 0), "field2", 2)
	sv3 := NewShapeValue(NewPoint(0, 0), "field3", 1)

	if CompareByDocID(sv1, sv2) >= 0 {
		t.Error("sv1(doc=1) should be less than sv2(doc=2)")
	}

	if CompareByDocID(sv2, sv1) <= 0 {
		t.Error("sv2(doc=2) should be greater than sv1(doc=1)")
	}

	if CompareByDocID(sv1, sv3) != 0 {
		t.Error("sv1(doc=1) should equal sv3(doc=1)")
	}
}

func TestCompareByFieldName(t *testing.T) {
	sv1 := NewShapeValue(NewPoint(0, 0), "aaa", 1)
	sv2 := NewShapeValue(NewPoint(0, 0), "bbb", 1)
	sv3 := NewShapeValue(NewPoint(0, 0), "aaa", 1)

	if CompareByFieldName(sv1, sv2) >= 0 {
		t.Error("'aaa' should be less than 'bbb'")
	}

	if CompareByFieldName(sv2, sv1) <= 0 {
		t.Error("'bbb' should be greater than 'aaa'")
	}

	if CompareByFieldName(sv1, sv3) != 0 {
		t.Error("'aaa' should equal 'aaa'")
	}
}

func TestFilterByFieldName(t *testing.T) {
	sv1 := NewShapeValue(NewPoint(0, 0), "location", 1)
	sv2 := NewShapeValue(NewPoint(0, 0), "bbox", 1)
	sv3 := NewShapeValue(NewPoint(0, 0), "location", 2)

	filter := FilterByFieldName("location")

	if !filter(sv1) {
		t.Error("sv1 should match 'location' filter")
	}

	if filter(sv2) {
		t.Error("sv2 should not match 'location' filter")
	}

	if !filter(sv3) {
		t.Error("sv3 should match 'location' filter")
	}
}

func TestFilterByDocID(t *testing.T) {
	sv1 := NewShapeValue(NewPoint(0, 0), "field1", 1)
	sv2 := NewShapeValue(NewPoint(0, 0), "field2", 2)

	filter := FilterByDocID(1)

	if !filter(sv1) {
		t.Error("sv1 should match docID 1 filter")
	}

	if filter(sv2) {
		t.Error("sv2 should not match docID 1 filter")
	}
}

func TestFilterNonEmpty(t *testing.T) {
	svWithShape := NewShapeValue(NewPoint(0, 0), "field", 1)
	svWithoutShape := NewShapeValue(nil, "field", 1)

	filter := FilterNonEmpty()

	if !filter(svWithShape) {
		t.Error("svWithShape should pass non-empty filter")
	}

	if filter(svWithoutShape) {
		t.Error("svWithoutShape should not pass non-empty filter")
	}
}

func TestShapeValueList_Filter(t *testing.T) {
	list := ShapeValueList{
		NewShapeValue(NewPoint(0, 0), "location", 1),
		NewShapeValue(NewPoint(0, 0), "bbox", 1),
		NewShapeValue(NewPoint(0, 0), "location", 2),
		NewShapeValue(nil, "empty", 1),
	}

	// Filter by field name
	locationList := list.Filter(FilterByFieldName("location"))
	if len(locationList) != 2 {
		t.Errorf("expected 2 location items, got %d", len(locationList))
	}

	// Filter non-empty
	nonEmptyList := list.Filter(FilterNonEmpty())
	if len(nonEmptyList) != 3 {
		t.Errorf("expected 3 non-empty items, got %d", len(nonEmptyList))
	}
}

func TestShapeValueList_GetByDocID(t *testing.T) {
	list := ShapeValueList{
		NewShapeValue(NewPoint(0, 0), "field1", 1),
		NewShapeValue(NewPoint(0, 0), "field2", 2),
		NewShapeValue(NewPoint(0, 0), "field3", 1),
	}

	result := list.GetByDocID(1)
	if len(result) != 2 {
		t.Errorf("expected 2 items with docID 1, got %d", len(result))
	}
}

func TestShapeValueList_GetByFieldName(t *testing.T) {
	list := ShapeValueList{
		NewShapeValue(NewPoint(0, 0), "location", 1),
		NewShapeValue(NewPoint(0, 0), "bbox", 1),
		NewShapeValue(NewPoint(0, 0), "location", 2),
	}

	result := list.GetByFieldName("location")
	if len(result) != 2 {
		t.Errorf("expected 2 items with field name 'location', got %d", len(result))
	}
}

func TestShapeValueList_LenSwap(t *testing.T) {
	list := ShapeValueList{
		NewShapeValue(NewPoint(0, 0), "field1", 1),
		NewShapeValue(NewPoint(0, 0), "field2", 2),
	}

	if list.Len() != 2 {
		t.Errorf("expected len 2, got %d", list.Len())
	}

	// Test swap
	first := list[0]
	second := list[1]
	list.Swap(0, 1)

	if list[0] != second || list[1] != first {
		t.Error("swap did not work correctly")
	}
}

func TestShapeType(t *testing.T) {
	tests := []struct {
		shape    Shape
		expected ShapeType
	}{
		{NewPoint(0, 0), ShapeTypePoint},
		{NewRectangle(0, 0, 1, 1), ShapeTypeRectangle},
		{nil, ShapeTypeUnknown},
	}

	for _, tt := range tests {
		result := getShapeType(tt.shape)
		if result != tt.expected {
			t.Errorf("getShapeType(%v) = %d, expected %d", tt.shape, result, tt.expected)
		}
	}
}

func TestShapeValueVersion(t *testing.T) {
	if ShapeValueVersion != 1 {
		t.Errorf("expected ShapeValueVersion to be 1, got %d", ShapeValueVersion)
	}
}
