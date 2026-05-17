// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"reflect"
	"testing"
)

// stubIndexableFieldType implements the full IndexableFieldType surface so we
// can verify the interface signatures compile and are satisfiable.
type stubIndexableFieldType struct{}

func (stubIndexableFieldType) Stored() bool                                      { return true }
func (stubIndexableFieldType) Tokenized() bool                                   { return false }
func (stubIndexableFieldType) StoreTermVectors() bool                            { return false }
func (stubIndexableFieldType) StoreTermVectorOffsets() bool                      { return false }
func (stubIndexableFieldType) StoreTermVectorPositions() bool                    { return false }
func (stubIndexableFieldType) StoreTermVectorPayloads() bool                     { return false }
func (stubIndexableFieldType) OmitNorms() bool                                   { return false }
func (stubIndexableFieldType) IndexOptions() IndexOptions                        { return IndexOptionsDocs }
func (stubIndexableFieldType) DocValuesType() DocValuesType                      { return DocValuesTypeNone }
func (stubIndexableFieldType) DocValuesSkipIndexType() DocValuesSkipIndexType    { return DocValuesSkipIndexTypeNone }
func (stubIndexableFieldType) PointDimensionCount() int                          { return 0 }
func (stubIndexableFieldType) PointIndexDimensionCount() int                     { return 0 }
func (stubIndexableFieldType) PointNumBytes() int                                { return 0 }
func (stubIndexableFieldType) VectorDimension() int                              { return 0 }
func (stubIndexableFieldType) VectorEncoding() VectorEncoding                    { return 0 }
func (stubIndexableFieldType) VectorSimilarityFunction() VectorSimilarityFunction {
	return 0
}
func (stubIndexableFieldType) GetAttributes() map[string]string { return nil }

func TestIndexableFieldType_InterfaceContract(t *testing.T) {
	var ft IndexableFieldType = stubIndexableFieldType{}
	if !ft.Stored() || ft.Tokenized() {
		t.Errorf("Stored/Tokenized accessor mismatch")
	}
	if ft.IndexOptions() != IndexOptionsDocs {
		t.Errorf("IndexOptions accessor mismatch")
	}
	// Reflect ensures all 17 Lucene methods are declared on the interface.
	want := []string{
		"Stored", "Tokenized", "StoreTermVectors", "StoreTermVectorOffsets",
		"StoreTermVectorPositions", "StoreTermVectorPayloads", "OmitNorms",
		"IndexOptions", "DocValuesType", "DocValuesSkipIndexType",
		"PointDimensionCount", "PointIndexDimensionCount", "PointNumBytes",
		"VectorDimension", "VectorEncoding", "VectorSimilarityFunction",
		"GetAttributes",
	}
	ifaceType := reflect.TypeOf((*IndexableFieldType)(nil)).Elem()
	got := make([]string, 0, ifaceType.NumMethod())
	for i := 0; i < ifaceType.NumMethod(); i++ {
		got = append(got, ifaceType.Method(i).Name)
	}
	if len(got) != len(want) {
		t.Errorf("method count = %d, want %d (got %v)", len(got), len(want), got)
	}
}
