// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs_test provides tests for the codecs package.
// This file contains tests ported from Apache Lucene's
// TestLucene90FieldInfosFormat.java
package codecs_test

import (
	"crypto/rand"
	"fmt"
	"math"
	mrand "math/rand"
	"testing"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Use math/rand for random number generation
var _ = mrand.Intn

// TestLucene90FieldInfosFormat_OneField tests field infos read/write with a single field.
// Ported from: BaseFieldInfoFormatTestCase.testOneField()
func TestLucene90FieldInfosFormat_OneField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create a simple field info
	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:              index.DocValuesTypeNone,
		DocValuesSkipIndexType:     index.DocValuesSkipIndexTypeNone,
		DocValuesGen:               -1,
		Stored:                     true,
		Tokenized:                true,
		OmitNorms:                false,
		StoreTermVectors:         false,
		StoreTermVectorPositions: false,
		StoreTermVectorOffsets:   false,
		StoreTermVectorPayloads:  false,
		PointDimensionCount:      0,
		PointIndexDimensionCount: 0,
		PointNumBytes:            0,
		VectorDimension:          0,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
		IsSoftDeletesField:       false,
		IsParentField:            false,
	}
	fi := index.NewFieldInfo("field", 0, opts)

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if infos2.Size() != 1 {
		t.Errorf("Expected size 1, got %d", infos2.Size())
	}

	fieldInfo := infos2.GetByName("field")
	if fieldInfo == nil {
		t.Fatal("field not found")
	}

	if fieldInfo.IndexOptions() == index.IndexOptionsNone {
		t.Error("Expected field to be indexed")
	}

	if fieldInfo.DocValuesType() != index.DocValuesTypeNone {
		t.Error("Expected no doc values")
	}

	if fieldInfo.OmitNorms() {
		t.Error("Expected norms not to be omitted")
	}

	if fieldInfo.HasPayloads() {
		t.Error("Expected no payloads")
	}

	if fieldInfo.HasTermVectors() {
		t.Error("Expected no term vectors")
	}

	if fieldInfo.PointDimensionCount() != 0 {
		t.Errorf("Expected 0 point dimensions, got %d", fieldInfo.PointDimensionCount())
	}

	if fieldInfo.VectorDimension() != 0 {
		t.Errorf("Expected 0 vector dimensions, got %d", fieldInfo.VectorDimension())
	}

	if fieldInfo.IsSoftDeletesField() {
		t.Error("Expected not to be soft deletes field")
	}
}

// TestLucene90FieldInfosFormat_ImmutableAttributes tests that field infos attributes are not mutable.
// Ported from: BaseFieldInfoFormatTestCase.testImmutableAttributes()
func TestLucene90FieldInfosFormat_ImmutableAttributes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Add attributes before building
	fi2 := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		SetAttribute("foo", "bar").
		SetAttribute("bar", "baz").
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi2)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if infos2.Size() != 1 {
		t.Errorf("Expected size 1, got %d", infos2.Size())
	}

	fieldInfo := infos2.GetByName("field")
	if fieldInfo == nil {
		t.Fatal("field not found")
	}

	// Get attributes and verify they are immutable (returned as a copy)
	attrs := fieldInfo.GetAttributes()

	// Try to modify the returned map - should not affect the original
	attrs["bogus"] = "bogus"

	// Verify the original is unchanged
	if fieldInfo.GetAttribute("bogus") != "" {
		t.Error("Attributes map should be immutable")
	}
}

// TestLucene90FieldInfosFormat_ExceptionOnCreateOutput tests exception handling on output creation.
// Ported from: BaseFieldInfoFormatTestCase.testExceptionOnCreateOutput()
func TestLucene90FieldInfosFormat_ExceptionOnCreateOutput(t *testing.T) {
	// Create a mock directory that will fail on create output
	failDir := &failingDirectory{
		Directory: store.NewByteBuffersDirectory(),
		failOn:    "CreateOutput",
	}
	defer failDir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, failDir.Directory)
	si.SetID(id)

	fi := createTestFieldInfo("field", 0)
	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Enable failure
	failDir.shouldFail = true

	// Write should fail
	err := format.Write(failDir, si, "", infos, context)
	if err == nil {
		t.Error("Expected write to fail")
	}
}

// TestLucene90FieldInfosFormat_ExceptionOnCloseOutput tests exception handling on output close.
// Ported from: BaseFieldInfoFormatTestCase.testExceptionOnCloseOutput()
func TestLucene90FieldInfosFormat_ExceptionOnCloseOutput(t *testing.T) {
	// Create a mock directory that will fail on close
	failDir := &failingDirectory{
		Directory: store.NewByteBuffersDirectory(),
		failOn:    "Close",
	}
	defer failDir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, failDir.Directory)
	si.SetID(id)

	fi := createTestFieldInfo("field", 0)
	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Enable failure
	failDir.shouldFail = true

	// Write should fail
	err := format.Write(failDir, si, "", infos, context)
	if err == nil {
		t.Error("Expected write to fail")
	}
}

// TestLucene90FieldInfosFormat_ExceptionOnOpenInput tests exception handling on input open.
// Ported from: BaseFieldInfoFormatTestCase.testExceptionOnOpenInput()
func TestLucene90FieldInfosFormat_ExceptionOnOpenInput(t *testing.T) {
	// Create a mock directory that will fail on open input
	failDir := &failingDirectory{
		Directory: store.NewByteBuffersDirectory(),
		failOn:    "OpenInput",
	}
	defer failDir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, failDir.Directory)
	si.SetID(id)

	fi := createTestFieldInfo("field", 0)
	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// First write without failure
	failDir.shouldFail = false
	err := format.Write(failDir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Enable failure for read
	failDir.shouldFail = true

	// Read should fail
	_, err = format.Read(failDir, si, "", store.IOContextRead)
	if err == nil {
		t.Error("Expected read to fail")
	}
}

// TestLucene90FieldInfosFormat_ExceptionOnCloseInput tests exception handling on input close.
// Ported from: BaseFieldInfoFormatTestCase.testExceptionOnCloseInput()
func TestLucene90FieldInfosFormat_ExceptionOnCloseInput(t *testing.T) {
	// Create a mock directory that will fail on close during read
	failDir := &failingDirectory{
		Directory: store.NewByteBuffersDirectory(),
		failOn:    "Close",
	}
	defer failDir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, failDir.Directory)
	si.SetID(id)

	fi := createTestFieldInfo("field", 0)
	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// First write without failure
	failDir.shouldFail = false
	err := format.Write(failDir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Enable failure for read close
	failDir.shouldFail = true

	// Read should fail
	_, err = format.Read(failDir, si, "", store.IOContextRead)
	if err == nil {
		t.Error("Expected read to fail")
	}
}

// TestLucene90FieldInfosFormat_Random tests field infos read/write with random fields.
// Ported from: BaseFieldInfoFormatTestCase.testRandom()
// Lucene90 does NOT support doc values skip index, so all skip index types should be NONE.
func TestLucene90FieldInfosFormat_Random(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Generate a bunch of fields with random properties
	numFields := 200
	if testing.Short() {
		numFields = 50
	}

	builder := index.NewFieldInfosBuilder()
	fieldNames := make(map[string]bool)

	for i := 0; i < numFields; i++ {
		fieldName := generateRandomUnicodeString()
		// Ensure unique field names
		for fieldNames[fieldName] {
			fieldName = generateRandomUnicodeString()
		}
		fieldNames[fieldName] = true

		fi := generateRandomFieldInfo(fieldName, i)
		builder.Add(fi)
	}

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify
	if infos2.Size() != infos.Size() {
		t.Errorf("Expected size %d, got %d", infos.Size(), infos2.Size())
	}

	// Compare each field
	iter := infos.Iterator()
	for iter.HasNext() {
		expected := iter.Next()
		actual := infos2.GetByNumber(expected.Number())
		if actual == nil {
			t.Errorf("Field %s (number %d) not found", expected.Name(), expected.Number())
			continue
		}

		assertFieldInfoEqual(t, expected, actual)
	}
}

// TestLucene90FieldInfosFormat_NoDocValuesSkipIndex verifies that Lucene90 format
// does not support doc values skip index (always NONE).
// This is specific to Lucene90 format which returns false for supportDocValuesSkipIndex().
func TestLucene90FieldInfosFormat_NoDocValuesSkipIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create field with doc values - Lucene90 should always store skip index as NONE
	fi := index.NewFieldInfoBuilder("field_with_dv", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange). // Try to set RANGE
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("field_with_dv")
	if fieldInfo == nil {
		t.Fatal("field_with_dv not found")
	}

	// Lucene90 format should always return NONE for skip index type
	// because it doesn't support doc values skip index
	if fieldInfo.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		t.Errorf("Lucene90 format should always have DocValuesSkipIndexTypeNone, got %v",
			fieldInfo.DocValuesSkipIndexType())
	}
}

// TestLucene90FieldInfosFormat_VariousDocValuesTypes tests various doc values types.
func TestLucene90FieldInfosFormat_VariousDocValuesTypes(t *testing.T) {
	docValuesTypes := []index.DocValuesType{
		index.DocValuesTypeNone,
		index.DocValuesTypeNumeric,
		index.DocValuesTypeBinary,
		index.DocValuesTypeSorted,
		index.DocValuesTypeSortedSet,
		index.DocValuesTypeSortedNumeric,
	}

	for _, dvType := range docValuesTypes {
		t.Run(dvType.String(), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			id := make([]byte, 16)
			rand.Read(id)

			si := index.NewSegmentInfo("_123", 10000, dir)
			si.SetID(id)

			fi := index.NewFieldInfoBuilder("field", 0).
				SetIndexOptions(index.IndexOptionsDocs).
				SetDocValuesType(dvType).
				Build()

			builder := index.NewFieldInfosBuilder()
			builder.Add(fi)
			infos := builder.Build()

			format := codecs.NewLucene94FieldInfosFormat()
			context := store.IOContextRead

			// Write
			err := format.Write(dir, si, "", infos, context)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read
			infos2, err := format.Read(dir, si, "", store.IOContextRead)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			fieldInfo := infos2.GetByName("field")
			if fieldInfo == nil {
				t.Fatal("field not found")
			}

			if fieldInfo.DocValuesType() != dvType {
				t.Errorf("Expected doc values type %v, got %v", dvType, fieldInfo.DocValuesType())
			}
		})
	}
}

// TestLucene90FieldInfosFormat_VariousIndexOptions tests various index options.
func TestLucene90FieldInfosFormat_VariousIndexOptions(t *testing.T) {
	indexOptions := []index.IndexOptions{
		index.IndexOptionsNone,
		index.IndexOptionsDocs,
		index.IndexOptionsDocsAndFreqs,
		index.IndexOptionsDocsAndFreqsAndPositions,
		index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
	}

	for _, opts := range indexOptions {
		t.Run(opts.String(), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			id := make([]byte, 16)
			rand.Read(id)

			si := index.NewSegmentInfo("_123", 10000, dir)
			si.SetID(id)

			fi := index.NewFieldInfoBuilder("field", 0).
				SetIndexOptions(opts).
				Build()

			builder := index.NewFieldInfosBuilder()
			builder.Add(fi)
			infos := builder.Build()

			format := codecs.NewLucene94FieldInfosFormat()
			context := store.IOContextRead

			// Write
			err := format.Write(dir, si, "", infos, context)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read
			infos2, err := format.Read(dir, si, "", store.IOContextRead)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			fieldInfo := infos2.GetByName("field")
			if fieldInfo == nil {
				t.Fatal("field not found")
			}

			if fieldInfo.IndexOptions() != opts {
				t.Errorf("Expected index options %v, got %v", opts, fieldInfo.IndexOptions())
			}
		})
	}
}

// TestLucene90FieldInfosFormat_Points tests point value field info.
func TestLucene90FieldInfosFormat_Points(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create field with points
	fi := index.NewFieldInfoBuilder("point_field", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetPointDimensions(3, 2, 4).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("point_field")
	if fieldInfo == nil {
		t.Fatal("point_field not found")
	}

	if fieldInfo.PointDimensionCount() != 3 {
		t.Errorf("Expected point dimension count 3, got %d", fieldInfo.PointDimensionCount())
	}

	if fieldInfo.PointIndexDimensionCount() != 2 {
		t.Errorf("Expected point index dimension count 2, got %d", fieldInfo.PointIndexDimensionCount())
	}

	if fieldInfo.PointNumBytes() != 4 {
		t.Errorf("Expected point num bytes 4, got %d", fieldInfo.PointNumBytes())
	}
}

// TestLucene90FieldInfosFormat_Vectors tests vector field info.
func TestLucene90FieldInfosFormat_Vectors(t *testing.T) {
	vectorConfigs := []struct {
		dim      int
		encoding index.VectorEncoding
		simFunc  index.VectorSimilarityFunction
	}{
		{128, index.VectorEncodingFloat32, index.VectorSimilarityFunctionEuclidean},
		{256, index.VectorEncodingFloat32, index.VectorSimilarityFunctionCosine},
		{64, index.VectorEncodingByte, index.VectorSimilarityFunctionDotProduct},
		{512, index.VectorEncodingFloat32, index.VectorSimilarityFunctionMaximumInnerProduct},
	}

	for i, config := range vectorConfigs {
		t.Run(fmt.Sprintf("config_%d", i), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			id := make([]byte, 16)
			rand.Read(id)

			si := index.NewSegmentInfo("_123", 10000, dir)
			si.SetID(id)

			fi := index.NewFieldInfoBuilder("vector_field", 0).
				SetIndexOptions(index.IndexOptionsNone).
				SetVectorAttributes(config.dim, config.encoding, config.simFunc).
				Build()

			builder := index.NewFieldInfosBuilder()
			builder.Add(fi)
			infos := builder.Build()

			format := codecs.NewLucene94FieldInfosFormat()
			context := store.IOContextRead

			// Write
			err := format.Write(dir, si, "", infos, context)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read
			infos2, err := format.Read(dir, si, "", store.IOContextRead)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			fieldInfo := infos2.GetByName("vector_field")
			if fieldInfo == nil {
				t.Fatal("vector_field not found")
			}

			if fieldInfo.VectorDimension() != config.dim {
				t.Errorf("Expected vector dimension %d, got %d", config.dim, fieldInfo.VectorDimension())
			}

			if fieldInfo.VectorEncoding() != config.encoding {
				t.Errorf("Expected vector encoding %v, got %v", config.encoding, fieldInfo.VectorEncoding())
			}

			if fieldInfo.VectorSimilarityFunction() != config.simFunc {
				t.Errorf("Expected vector similarity function %v, got %v", config.simFunc, fieldInfo.VectorSimilarityFunction())
			}
		})
	}
}

// TestLucene90FieldInfosFormat_TermVectors tests term vector options.
func TestLucene90FieldInfosFormat_TermVectors(t *testing.T) {
	testCases := []struct {
		name       string
		storeTV    bool
		storePos   bool
		storeOff   bool
		storePay   bool
	}{
		{"term_vectors_only", true, false, false, false},
		{"term_vectors_positions", true, true, false, false},
		{"term_vectors_offsets", true, false, true, false},
		{"term_vectors_all", true, true, true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			id := make([]byte, 16)
			rand.Read(id)

			si := index.NewSegmentInfo("_123", 10000, dir)
			si.SetID(id)

			builder := index.NewFieldInfoBuilder("field", 0).
				SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
				SetStoreTermVectors(tc.storeTV)

			if tc.storePos {
				builder.SetStoreTermVectorPositions(true)
			}
			if tc.storeOff {
				builder.SetStoreTermVectorOffsets(true)
			}
			if tc.storePay {
				builder.SetStoreTermVectorPayloads(true)
			}

			fi := builder.Build()

			fib := index.NewFieldInfosBuilder()
			fib.Add(fi)
			infos := fib.Build()

			format := codecs.NewLucene94FieldInfosFormat()
			context := store.IOContextRead

			// Write
			err := format.Write(dir, si, "", infos, context)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read
			infos2, err := format.Read(dir, si, "", store.IOContextRead)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			fieldInfo := infos2.GetByName("field")
			if fieldInfo == nil {
				t.Fatal("field not found")
			}

			if fieldInfo.StoreTermVectors() != tc.storeTV {
				t.Errorf("Expected StoreTermVectors %v, got %v", tc.storeTV, fieldInfo.StoreTermVectors())
			}

			if fieldInfo.StoreTermVectorPositions() != tc.storePos {
				t.Errorf("Expected StoreTermVectorPositions %v, got %v", tc.storePos, fieldInfo.StoreTermVectorPositions())
			}

			if fieldInfo.StoreTermVectorOffsets() != tc.storeOff {
				t.Errorf("Expected StoreTermVectorOffsets %v, got %v", tc.storeOff, fieldInfo.StoreTermVectorOffsets())
			}

			if fieldInfo.StoreTermVectorPayloads() != tc.storePay {
				t.Errorf("Expected StoreTermVectorPayloads %v, got %v", tc.storePay, fieldInfo.StoreTermVectorPayloads())
			}
		})
	}
}

// TestLucene90FieldInfosFormat_SoftDeletesField tests soft deletes field flag.
func TestLucene90FieldInfosFormat_SoftDeletesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("soft_deletes_field", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetSoftDeletesField(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("soft_deletes_field")
	if fieldInfo == nil {
		t.Fatal("soft_deletes_field not found")
	}

	if !fieldInfo.IsSoftDeletesField() {
		t.Error("Expected field to be soft deletes field")
	}
}

// TestLucene90FieldInfosFormat_ParentField tests parent field flag.
func TestLucene90FieldInfosFormat_ParentField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("parent_field", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetParentField(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("parent_field")
	if fieldInfo == nil {
		t.Fatal("parent_field not found")
	}

	if !fieldInfo.IsParentField() {
		t.Error("Expected field to be parent field")
	}
}

// TestLucene90FieldInfosFormat_Attributes tests custom attributes.
func TestLucene90FieldInfosFormat_Attributes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetAttribute("custom_key1", "custom_value1").
		SetAttribute("custom_key2", "custom_value2").
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("field")
	if fieldInfo == nil {
		t.Fatal("field not found")
	}

	if fieldInfo.GetAttribute("custom_key1") != "custom_value1" {
		t.Errorf("Expected attribute custom_key1=custom_value1, got %s", fieldInfo.GetAttribute("custom_key1"))
	}

	if fieldInfo.GetAttribute("custom_key2") != "custom_value2" {
		t.Errorf("Expected attribute custom_key2=custom_value2, got %s", fieldInfo.GetAttribute("custom_key2"))
	}
}

// TestLucene90FieldInfosFormat_MultipleFields tests multiple fields in a single segment.
func TestLucene90FieldInfosFormat_MultipleFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Add various types of fields
	fields := []*index.FieldInfo{
		index.NewFieldInfoBuilder("text_field", 0).
			SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
			SetStored(true).
			Build(),
		index.NewFieldInfoBuilder("numeric_dv", 1).
			SetIndexOptions(index.IndexOptionsNone).
			SetDocValuesType(index.DocValuesTypeNumeric).
			Build(),
		index.NewFieldInfoBuilder("point_field", 2).
			SetIndexOptions(index.IndexOptionsDocs).
			SetPointDimensions(2, 2, 8).
			Build(),
		index.NewFieldInfoBuilder("vector_field", 3).
			SetIndexOptions(index.IndexOptionsNone).
			SetVectorAttributes(128, index.VectorEncodingFloat32, index.VectorSimilarityFunctionCosine).
			Build(),
		index.NewFieldInfoBuilder("stored_only", 4).
			SetIndexOptions(index.IndexOptionsNone).
			SetStored(true).
			Build(),
	}

	for _, fi := range fields {
		builder.Add(fi)
	}

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if infos2.Size() != len(fields) {
		t.Errorf("Expected %d fields, got %d", len(fields), infos2.Size())
	}

	// Verify each field
	for _, fi := range fields {
		actual := infos2.GetByName(fi.Name())
		if actual == nil {
			t.Errorf("Field %s not found", fi.Name())
			continue
		}
		assertFieldInfoEqual(t, fi, actual)
	}
}

// TestLucene90FieldInfosFormat_EmptyFieldInfos tests empty field infos.
func TestLucene90FieldInfosFormat_EmptyFieldInfos(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if infos2.Size() != 0 {
		t.Errorf("Expected 0 fields, got %d", infos2.Size())
	}
}

// Helper functions

// createTestFieldInfo creates a simple test field info.
func createTestFieldInfo(name string, number int) *index.FieldInfo {
	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:              index.DocValuesTypeNone,
		DocValuesSkipIndexType:     index.DocValuesSkipIndexTypeNone,
		DocValuesGen:               -1,
		Stored:                     true,
		Tokenized:                true,
		OmitNorms:                false,
		StoreTermVectors:         false,
		StoreTermVectorPositions: false,
		StoreTermVectorOffsets:   false,
		StoreTermVectorPayloads:  false,
		PointDimensionCount:      0,
		PointIndexDimensionCount: 0,
		PointNumBytes:            0,
		VectorDimension:          0,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
		IsSoftDeletesField:       false,
		IsParentField:            false,
	}
	return index.NewFieldInfo(name, number, opts)
}

// assertFieldInfoEqual compares two FieldInfo objects for equality.
func assertFieldInfoEqual(t *testing.T, expected, actual *index.FieldInfo) {
	t.Helper()

	if expected.Number() != actual.Number() {
		t.Errorf("Field %s: expected number %d, got %d", expected.Name(), expected.Number(), actual.Number())
	}

	if expected.Name() != actual.Name() {
		t.Errorf("Expected name %s, got %s", expected.Name(), actual.Name())
	}

	if expected.DocValuesType() != actual.DocValuesType() {
		t.Errorf("Field %s: expected doc values type %v, got %v", expected.Name(), expected.DocValuesType(), actual.DocValuesType())
	}

	// Lucene90 does not support doc values skip index, so always expect NONE
	if actual.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		t.Errorf("Field %s: Lucene90 format should have DocValuesSkipIndexTypeNone, got %v", expected.Name(), actual.DocValuesSkipIndexType())
	}

	if expected.IndexOptions() != actual.IndexOptions() {
		t.Errorf("Field %s: expected index options %v, got %v", expected.Name(), expected.IndexOptions(), actual.IndexOptions())
	}

	if expected.HasNorms() != actual.HasNorms() {
		t.Errorf("Field %s: expected has norms %v, got %v", expected.Name(), expected.HasNorms(), actual.HasNorms())
	}

	if expected.HasPayloads() != actual.HasPayloads() {
		t.Errorf("Field %s: expected has payloads %v, got %v", expected.Name(), expected.HasPayloads(), actual.HasPayloads())
	}

	if expected.HasTermVectors() != actual.HasTermVectors() {
		t.Errorf("Field %s: expected has term vectors %v, got %v", expected.Name(), expected.HasTermVectors(), actual.HasTermVectors())
	}

	if expected.OmitNorms() != actual.OmitNorms() {
		t.Errorf("Field %s: expected omits norms %v, got %v", expected.Name(), expected.OmitNorms(), actual.OmitNorms())
	}

	if expected.DocValuesGen() != actual.DocValuesGen() {
		t.Errorf("Field %s: expected doc values gen %d, got %d", expected.Name(), expected.DocValuesGen(), actual.DocValuesGen())
	}

	if expected.PointDimensionCount() != actual.PointDimensionCount() {
		t.Errorf("Field %s: expected point dimension count %d, got %d", expected.Name(), expected.PointDimensionCount(), actual.PointDimensionCount())
	}

	if expected.PointIndexDimensionCount() != actual.PointIndexDimensionCount() {
		t.Errorf("Field %s: expected point index dimension count %d, got %d", expected.Name(), expected.PointIndexDimensionCount(), actual.PointIndexDimensionCount())
	}

	if expected.PointNumBytes() != actual.PointNumBytes() {
		t.Errorf("Field %s: expected point num bytes %d, got %d", expected.Name(), expected.PointNumBytes(), actual.PointNumBytes())
	}

	if expected.VectorDimension() != actual.VectorDimension() {
		t.Errorf("Field %s: expected vector dimension %d, got %d", expected.Name(), expected.VectorDimension(), actual.VectorDimension())
	}

	if expected.VectorEncoding() != actual.VectorEncoding() {
		t.Errorf("Field %s: expected vector encoding %v, got %v", expected.Name(), expected.VectorEncoding(), actual.VectorEncoding())
	}

	if expected.VectorSimilarityFunction() != actual.VectorSimilarityFunction() {
		t.Errorf("Field %s: expected vector similarity function %v, got %v", expected.Name(), expected.VectorSimilarityFunction(), actual.VectorSimilarityFunction())
	}

	if expected.IsSoftDeletesField() != actual.IsSoftDeletesField() {
		t.Errorf("Field %s: expected is soft deletes field %v, got %v", expected.Name(), expected.IsSoftDeletesField(), actual.IsSoftDeletesField())
	}

	if expected.IsParentField() != actual.IsParentField() {
		t.Errorf("Field %s: expected is parent field %v, got %v", expected.Name(), expected.IsParentField(), actual.IsParentField())
	}
}

// generateRandomUnicodeString generates a random unicode string similar to Lucene's TestUtil.randomUnicodeString().
func generateRandomUnicodeString() string {
	// Generate a random length between 1 and 20
	length := 1 + mrand.Intn(20)

	var result []rune
	for i := 0; i < length; i++ {
		// Generate a random unicode code point
		// Focus on BMP (Basic Multilingual Plane) for simplicity
		codePoint := mrand.Intn(0x10000)

		// Ensure valid unicode
		if utf8.ValidRune(rune(codePoint)) && codePoint != 0 {
			result = append(result, rune(codePoint))
		} else {
			// Fallback to ASCII
			result = append(result, rune('a'+mrand.Intn(26)))
		}
	}

	return string(result)
}

// generateRandomFieldInfo generates a random field info for testing.
func generateRandomFieldInfo(name string, number int) *index.FieldInfo {
	builder := index.NewFieldInfoBuilder(name, number)

	// Random index options
	indexOpts := []index.IndexOptions{
		index.IndexOptionsNone,
		index.IndexOptionsDocs,
		index.IndexOptionsDocsAndFreqs,
		index.IndexOptionsDocsAndFreqsAndPositions,
		index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
	}
	selectedIndexOpts := indexOpts[mrand.Intn(len(indexOpts))]
	builder.SetIndexOptions(selectedIndexOpts)

	if selectedIndexOpts.IsIndexed() {
		builder.SetOmitNorms(mrand.Intn(2) == 0)
		builder.SetStored(mrand.Intn(2) == 0)

		if mrand.Intn(2) == 0 {
			builder.SetStoreTermVectors(true)
			if selectedIndexOpts.HasPositions() {
				if mrand.Intn(2) == 0 {
					builder.SetStoreTermVectorPositions(true)
				}
				if mrand.Intn(2) == 0 {
					builder.SetStoreTermVectorOffsets(true)
				}
			}
		}
	}

	// Random doc values type
	if mrand.Intn(2) == 0 {
		dvTypes := []index.DocValuesType{
			index.DocValuesTypeNone,
			index.DocValuesTypeNumeric,
			index.DocValuesTypeBinary,
			index.DocValuesTypeSorted,
			index.DocValuesTypeSortedSet,
			index.DocValuesTypeSortedNumeric,
		}
		selectedDvType := dvTypes[mrand.Intn(len(dvTypes))]
		builder.SetDocValuesType(selectedDvType)

		// Lucene90 does not support doc values skip index, so always NONE
		builder.SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeNone)
	}

	// Random points
	if mrand.Intn(2) == 0 {
		dimension := 1 + mrand.Intn(8) // 1-8 dimensions
		indexDimension := 1 + mrand.Intn(dimension)
		numBytes := 1 + mrand.Intn(16) // 1-16 bytes
		builder.SetPointDimensions(dimension, indexDimension, numBytes)
	}

	// Random vectors
	if mrand.Intn(2) == 0 {
		vectorDim := 1 + mrand.Intn(1024)
		vectorEncodings := []index.VectorEncoding{
			index.VectorEncodingByte,
			index.VectorEncodingFloat32,
		}
		vectorSims := []index.VectorSimilarityFunction{
			index.VectorSimilarityFunctionEuclidean,
			index.VectorSimilarityFunctionDotProduct,
			index.VectorSimilarityFunctionCosine,
			index.VectorSimilarityFunctionMaximumInnerProduct,
		}
		selectedEncoding := vectorEncodings[mrand.Intn(len(vectorEncodings))]
		selectedSim := vectorSims[mrand.Intn(len(vectorSims))]
		builder.SetVectorAttributes(vectorDim, selectedEncoding, selectedSim)
	}

	// Random attributes
	if mrand.Intn(2) == 0 {
		builder.SetAttribute("random_attr", fmt.Sprintf("value_%d", mrand.Intn(1000)))
	}

	return builder.Build()
}

// failingDirectory is a mock directory that can simulate failures for testing.
type failingDirectory struct {
	store.Directory
	failOn     string
	shouldFail bool
}

func (fd *failingDirectory) CreateOutput(name string, context store.IOContext) (store.IndexOutput, error) {
	if fd.shouldFail && fd.failOn == "CreateOutput" {
		return nil, fmt.Errorf("simulated create output failure")
	}
	return fd.Directory.CreateOutput(name, context)
}

func (fd *failingDirectory) OpenInput(name string, context store.IOContext) (store.IndexInput, error) {
	if fd.shouldFail && fd.failOn == "OpenInput" {
		return nil, fmt.Errorf("simulated open input failure")
	}
	return fd.Directory.OpenInput(name, context)
}

func (fd *failingDirectory) Close() error {
	if fd.shouldFail && fd.failOn == "Close" {
		return fmt.Errorf("simulated close failure")
	}
	return fd.Directory.Close()
}

// Ensure failingDirectory implements the Directory interface
var _ store.Directory = (*failingDirectory)(nil)

// Additional tests for edge cases

// TestLucene90FieldInfosFormat_LargeFieldNumber tests field with large field number.
func TestLucene90FieldInfosFormat_LargeFieldNumber(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create field with large field number
	fi := index.NewFieldInfoBuilder("large_number_field", math.MaxInt32/2).
		SetIndexOptions(index.IndexOptionsDocs).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByNumber(math.MaxInt32 / 2)
	if fieldInfo == nil {
		t.Fatal("large_number_field not found")
	}

	if fieldInfo.Name() != "large_number_field" {
		t.Errorf("Expected name large_number_field, got %s", fieldInfo.Name())
	}
}

// TestLucene90FieldInfosFormat_UnicodeFieldNames tests unicode field names.
func TestLucene90FieldInfosFormat_UnicodeFieldNames(t *testing.T) {
	unicodeNames := []string{
		"field_日本語",
		"field_中文",
		"field_한국어",
		"field_العربية",
		"field_emoji_🎉",
		"field_with_spaces",
		"field.with.dots",
		"field-with-dashes",
		"field_with_underscores",
		"123numeric_start",
	}

	for _, name := range unicodeNames {
		t.Run(name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			id := make([]byte, 16)
			rand.Read(id)

			si := index.NewSegmentInfo("_123", 10000, dir)
			si.SetID(id)

			fi := index.NewFieldInfoBuilder(name, 0).
				SetIndexOptions(index.IndexOptionsDocs).
				Build()

			builder := index.NewFieldInfosBuilder()
			builder.Add(fi)
			infos := builder.Build()

			format := codecs.NewLucene94FieldInfosFormat()
			context := store.IOContextRead

			// Write
			err := format.Write(dir, si, "", infos, context)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read
			infos2, err := format.Read(dir, si, "", store.IOContextRead)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			fieldInfo := infos2.GetByName(name)
			if fieldInfo == nil {
				t.Fatalf("Field %s not found", name)
			}

			if fieldInfo.Name() != name {
				t.Errorf("Expected name %s, got %s", name, fieldInfo.Name())
			}
		})
	}
}

// TestLucene90FieldInfosFormat_DocValuesGen tests doc values generation.
func TestLucene90FieldInfosFormat_DocValuesGen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create field with specific doc values generation
	fi := index.NewFieldInfoBuilder("dv_gen_field", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesGen(42).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("dv_gen_field")
	if fieldInfo == nil {
		t.Fatal("dv_gen_field not found")
	}

	if fieldInfo.DocValuesGen() != 42 {
		t.Errorf("Expected doc values gen 42, got %d", fieldInfo.DocValuesGen())
	}
}

// TestLucene90FieldInfosFormat_OmitNorms tests omit norms flag.
func TestLucene90FieldInfosFormat_OmitNorms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("omit_norms_field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetOmitNorms(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextRead

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	fieldInfo := infos2.GetByName("omit_norms_field")
	if fieldInfo == nil {
		t.Fatal("omit_norms_field not found")
	}

	if !fieldInfo.OmitNorms() {
		t.Error("Expected field to omit norms")
	}

	if fieldInfo.HasNorms() {
		t.Error("Expected field to not have norms")
	}
}
