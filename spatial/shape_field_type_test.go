// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestNewShapeFieldType(t *testing.T) {
	ft := NewShapeFieldType()

	if ft == nil {
		t.Fatal("expected non-nil ShapeFieldType")
	}

	// Check defaults
	if !ft.IsIndexed() {
		t.Error("default should be indexed")
	}

	if ft.IsStored() {
		t.Error("default should not be stored")
	}

	if ft.HasDocValues() {
		t.Error("default should not have doc values")
	}

	if !ft.IsTokenized() {
		t.Error("default should be tokenized")
	}

	if ft.GetIndexOptions() != IndexOptionsDocsAndFreqsAndPositions {
		t.Error("default index options should be DocsAndFreqsAndPositions")
	}

	if ft.GetDimensionCount() != 2 {
		t.Errorf("default dimension count should be 2, got %d", ft.GetDimensionCount())
	}
}

func TestNewStoredShapeFieldType(t *testing.T) {
	ft := NewStoredShapeFieldType()

	if !ft.IsStored() {
		t.Error("stored field type should be stored")
	}

	if !ft.IsIndexed() {
		t.Error("stored field type should still be indexed")
	}
}

func TestNewIndexedShapeFieldType(t *testing.T) {
	ft := NewIndexedShapeFieldType()

	if !ft.IsIndexed() {
		t.Error("indexed field type should be indexed")
	}

	if ft.IsStored() {
		t.Error("indexed field type should not be stored")
	}
}

func TestNewDocValuesShapeFieldType(t *testing.T) {
	ft := NewDocValuesShapeFieldType()

	if !ft.HasDocValues() {
		t.Error("doc values field type should have doc values")
	}

	if ft.GetDocValuesType() != DocValuesTypeBinary {
		t.Error("doc values field type should have binary doc values type")
	}
}

func TestShapeFieldType_Setters(t *testing.T) {
	ft := NewShapeFieldType()

	// Test SetIndexed
	ft.SetIndexed(false)
	if ft.IsIndexed() {
		t.Error("SetIndexed(false) failed")
	}

	// Test SetStored
	ft.SetStored(true)
	if !ft.IsStored() {
		t.Error("SetStored(true) failed")
	}

	// Test SetDocValues
	ft.SetDocValues(true)
	if !ft.HasDocValues() {
		t.Error("SetDocValues(true) failed")
	}

	// Test SetTokenized
	ft.SetTokenized(false)
	if ft.IsTokenized() {
		t.Error("SetTokenized(false) failed")
	}

	// Test SetStoreTermVectors
	ft.SetStoreTermVectors(true)
	if !ft.StoreTermVectors() {
		t.Error("SetStoreTermVectors(true) failed")
	}

	// Test SetStoreTermVectorPositions
	ft.SetStoreTermVectorPositions(true)
	if !ft.StoreTermVectorPositions() {
		t.Error("SetStoreTermVectorPositions(true) failed")
	}

	// Test SetStoreTermVectorOffsets
	ft.SetStoreTermVectorOffsets(true)
	if !ft.StoreTermVectorOffsets() {
		t.Error("SetStoreTermVectorOffsets(true) failed")
	}

	// Test SetOmitNorms
	ft.SetOmitNorms(false)
	if ft.OmitNorms() {
		t.Error("SetOmitNorms(false) failed")
	}

	// Test SetIndexOptions
	ft.SetIndexOptions(IndexOptionsDocs)
	if ft.GetIndexOptions() != IndexOptionsDocs {
		t.Error("SetIndexOptions failed")
	}

	// Test SetDocValuesType
	ft.SetDocValuesType(DocValuesTypeSorted)
	if ft.GetDocValuesType() != DocValuesTypeSorted {
		t.Error("SetDocValuesType failed")
	}

	// Test SetDimensionCount
	ft.SetDimensionCount(3)
	if ft.GetDimensionCount() != 3 {
		t.Errorf("expected dimension count 3, got %d", ft.GetDimensionCount())
	}
}

func TestShapeFieldType_SetDimensionCount_Invalid(t *testing.T) {
	ft := NewShapeFieldType()

	// Setting to 0 or negative should not change the value
	ft.SetDimensionCount(0)
	if ft.GetDimensionCount() != 2 {
		t.Error("SetDimensionCount(0) should not change value")
	}

	ft.SetDimensionCount(-1)
	if ft.GetDimensionCount() != 2 {
		t.Error("SetDimensionCount(-1) should not change value")
	}
}

func TestShapeFieldType_SetDocValuesType_UpdatesDocValues(t *testing.T) {
	ft := NewShapeFieldType()

	// Initially no doc values
	if ft.HasDocValues() {
		t.Error("initially should not have doc values")
	}

	// Set doc values type
	ft.SetDocValuesType(DocValuesTypeNumeric)
	if !ft.HasDocValues() {
		t.Error("should have doc values after setting type")
	}

	// Set to None
	ft.SetDocValuesType(DocValuesTypeNone)
	if ft.HasDocValues() {
		t.Error("should not have doc values after setting type to None")
	}
}

func TestShapeFieldType_CheckConsistency(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*ShapeFieldType)
		wantErr bool
	}{
		{
			name:    "valid default",
			setup:   func(ft *ShapeFieldType) {},
			wantErr: false,
		},
		{
			name: "indexed with None options",
			setup: func(ft *ShapeFieldType) {
				ft.SetIndexed(true)
				ft.SetIndexOptions(IndexOptionsNone)
			},
			wantErr: true,
		},
		{
			name: "invalid dimension count",
			setup: func(ft *ShapeFieldType) {
				// Manually set invalid dimension count
				ft.dimensionCount = 0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewShapeFieldType()
			tt.setup(ft)

			err := ft.CheckConsistency()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestShapeFieldType_String(t *testing.T) {
	ft := NewShapeFieldType()
	str := ft.String()

	if str == "" {
		t.Error("String() should not return empty")
	}

	// Should contain field type name
	if len(str) < 10 {
		t.Error("String() should return meaningful content")
	}
}

func TestShapeFieldType_Equals(t *testing.T) {
	ft1 := NewShapeFieldType()
	ft2 := NewShapeFieldType()

	if !ft1.Equals(ft2) {
		t.Error("two default field types should be equal")
	}

	// Modify one
	ft2.SetStored(true)
	if ft1.Equals(ft2) {
		t.Error("field types with different settings should not be equal")
	}

	// Test nil
	if ft1.Equals(nil) {
		t.Error("should not equal nil")
	}
}

func TestShapeFieldType_HashCode(t *testing.T) {
	ft := NewShapeFieldType()
	hash := ft.HashCode()

	// Hash code should be deterministic
	hash2 := ft.HashCode()
	if hash != hash2 {
		t.Error("HashCode should be deterministic")
	}

	// Different field types should (usually) have different hash codes
	ft2 := NewShapeFieldType()
	ft2.SetStored(true)
	if ft.Equals(ft2) {
		return // Skip if equal
	}
}

func TestShapeFieldType_Freeze(t *testing.T) {
	ft := NewShapeFieldType()
	frozen := ft.Freeze()

	if frozen != ft {
		t.Error("Freeze should return self (in current implementation)")
	}
}

func TestShapeFieldType_SetSpatialStrategy(t *testing.T) {
	ft := NewShapeFieldType()

	ctx := NewSpatialContext()
	strategy, err := NewPointVectorStrategy("location", ctx)
	if err != nil {
		t.Fatalf("failed to create strategy: %v", err)
	}

	ft.SetSpatialStrategy(strategy)

	if ft.GetSpatialStrategy() != strategy {
		t.Error("spatial strategy not set correctly")
	}
}

func TestNewShapeField(t *testing.T) {
	ft := NewShapeFieldType()
	shape := NewPoint(10, 20)

	field, err := NewShapeField("location", shape, ft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if field.GetName() != "location" {
		t.Errorf("expected name 'location', got '%s'", field.GetName())
	}

	if field.GetShape() != shape {
		t.Error("shape mismatch")
	}

	if field.GetFieldType() != ft {
		t.Error("field type mismatch")
	}
}

func TestNewShapeField_Errors(t *testing.T) {
	ft := NewShapeFieldType()

	tests := []struct {
		name      string
		fieldName string
		shape     Shape
		fieldType *ShapeFieldType
		wantErr   bool
	}{
		{
			name:      "empty name",
			fieldName: "",
			shape:     NewPoint(0, 0),
			fieldType: ft,
			wantErr:   true,
		},
		{
			name:      "nil shape",
			fieldName: "location",
			shape:     nil,
			fieldType: ft,
			wantErr:   true,
		},
		{
			name:      "nil field type",
			fieldName: "location",
			shape:     NewPoint(0, 0),
			fieldType: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewShapeField(tt.fieldName, tt.shape, tt.fieldType)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestShapeField_CreateIndexableFields_NoStrategy(t *testing.T) {
	ft := NewShapeFieldType()
	// No spatial strategy set
	field, _ := NewShapeField("location", NewPoint(0, 0), ft)

	_, err := field.CreateIndexableFields()
	if err == nil {
		t.Error("expected error when no spatial strategy")
	}
}

func TestShapeField_String(t *testing.T) {
	ft := NewShapeFieldType()
	field, _ := NewShapeField("location", NewPoint(10, 20), ft)

	str := field.String()
	if str == "" {
		t.Error("String() should not return empty")
	}
}

func TestIndexOptions(t *testing.T) {
	// Test that IndexOptions constants are ordered correctly
	if IndexOptionsNone >= IndexOptionsDocs {
		t.Error("IndexOptionsNone should be less than IndexOptionsDocs")
	}
	if IndexOptionsDocs >= IndexOptionsDocsAndFreqs {
		t.Error("IndexOptionsDocs should be less than IndexOptionsDocsAndFreqs")
	}
	if IndexOptionsDocsAndFreqs >= IndexOptionsDocsAndFreqsAndPositions {
		t.Error("IndexOptionsDocsAndFreqs should be less than IndexOptionsDocsAndFreqsAndPositions")
	}
	if IndexOptionsDocsAndFreqsAndPositions >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets {
		t.Error("IndexOptionsDocsAndFreqsAndPositions should be less than IndexOptionsDocsAndFreqsAndPositionsAndOffsets")
	}
}

func TestDocValuesType(t *testing.T) {
	// Test that DocValuesType constants are distinct
	types := []DocValuesType{
		DocValuesTypeNone,
		DocValuesTypeNumeric,
		DocValuesTypeBinary,
		DocValuesTypeSorted,
		DocValuesTypeSortedNumeric,
		DocValuesTypeSortedSet,
	}

	seen := make(map[DocValuesType]bool)
	for _, dt := range types {
		if seen[dt] {
			t.Errorf("duplicate DocValuesType value: %d", dt)
		}
		seen[dt] = true
	}
}

func TestShapeFieldTypePresets(t *testing.T) {
	// Test ShapeFieldTypeDefault
	if ShapeFieldTypeDefault == nil {
		t.Error("ShapeFieldTypeDefault should not be nil")
	}
	if !ShapeFieldTypeDefault.IsIndexed() {
		t.Error("ShapeFieldTypeDefault should be indexed")
	}

	// Test ShapeFieldTypeStored
	if ShapeFieldTypeStored == nil {
		t.Error("ShapeFieldTypeStored should not be nil")
	}
	if !ShapeFieldTypeStored.IsStored() {
		t.Error("ShapeFieldTypeStored should be stored")
	}

	// Test ShapeFieldTypeIndexedOnly
	if ShapeFieldTypeIndexedOnly == nil {
		t.Error("ShapeFieldTypeIndexedOnly should not be nil")
	}
	if ShapeFieldTypeIndexedOnly.IsStored() {
		t.Error("ShapeFieldTypeIndexedOnly should not be stored")
	}

	// Test ShapeFieldTypeDocValues
	if ShapeFieldTypeDocValues == nil {
		t.Error("ShapeFieldTypeDocValues should not be nil")
	}
	if !ShapeFieldTypeDocValues.HasDocValues() {
		t.Error("ShapeFieldTypeDocValues should have doc values")
	}

	// Test ShapeFieldTypeIndexedAndStored
	if ShapeFieldTypeIndexedAndStored == nil {
		t.Error("ShapeFieldTypeIndexedAndStored should not be nil")
	}
	if !ShapeFieldTypeIndexedAndStored.IsIndexed() {
		t.Error("ShapeFieldTypeIndexedAndStored should be indexed")
	}
	if !ShapeFieldTypeIndexedAndStored.IsStored() {
		t.Error("ShapeFieldTypeIndexedAndStored should be stored")
	}
}

func TestShapeFieldType_Name(t *testing.T) {
	ft := NewShapeFieldType()

	if ft.GetName() != "ShapeField" {
		t.Errorf("expected default name 'ShapeField', got '%s'", ft.GetName())
	}

	ft.SetName("CustomShape")
	if ft.GetName() != "CustomShape" {
		t.Errorf("expected name 'CustomShape', got '%s'", ft.GetName())
	}
}

func TestBoolHash(t *testing.T) {
	// Test that true and false return different values
	if boolHash(true) == boolHash(false) {
		t.Error("boolHash(true) should not equal boolHash(false)")
	}

	// Test that same values return same hash
	if boolHash(true) != boolHash(true) {
		t.Error("boolHash(true) should be consistent")
	}

	if boolHash(false) != boolHash(false) {
		t.Error("boolHash(false) should be consistent")
	}
}
