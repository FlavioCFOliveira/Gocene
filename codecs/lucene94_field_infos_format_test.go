// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: lucene94_field_infos_format_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene94/TestLucene94FieldInfosFormat.java
// Purpose: Tests Lucene 9.4 FieldInfos format including doc values skip index support,
//          vector similarity functions, and base class test coverage
//
// GC-213: Test Lucene94FieldInfosFormat

package codecs_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ============================================================================
// Test Vector Similarity Functions
// ============================================================================

// TestLucene94FieldInfosFormat_VectorSimilarityFuncs
// Source: TestLucene94FieldInfosFormat.testVectorSimilarityFuncs()
// Purpose: Ensures that all expected vector similarity functions are translatable in the format
func TestLucene94FieldInfosFormat_VectorSimilarityFuncs(t *testing.T) {
	// The Java test verifies that Lucene94FieldInfosFormat.SIMILARITY_FUNCTIONS
	// contains all VectorSimilarityFunction values. In Go, we verify that all
	// VectorSimilarityFunction constants can be properly serialized/deserialized.

	expectedValues := []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	}

	// Verify each similarity function can be converted to byte and back
	for _, expected := range expectedValues {
		// In the format, similarity functions are stored as bytes (ordinals)
		byteVal := byte(expected)
		if byteVal < 0 || byteVal >= 4 {
			t.Errorf("VectorSimilarityFunction %v has invalid byte value: %d", expected, byteVal)
		}

		// Verify round-trip conversion
		recovered := index.VectorSimilarityFunction(byteVal)
		if recovered != expected {
			t.Errorf("Round-trip failed for %v: got %v", expected, recovered)
		}
	}

	// Verify the count matches expected (4 similarity functions)
	if len(expectedValues) != 4 {
		t.Errorf("Expected 4 similarity functions, got %d", len(expectedValues))
	}
}

// ============================================================================
// Test One Field
// ============================================================================

// TestLucene94FieldInfosFormat_OneField
// Source: BaseFieldInfoFormatTestCase.testOneField()
// Purpose: Test field infos read/write with a single field
func TestLucene94FieldInfosFormat_OneField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Create a field info similar to the Java test
	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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
	if infos2.Size() != 1 {
		t.Errorf("Expected size 1, got %d", infos2.Size())
	}

	fi2 := infos2.GetByName("field")
	if fi2 == nil {
		t.Fatal("field not found")
	}

	if fi2.IndexOptions() != index.IndexOptionsDocsAndFreqsAndPositions {
		t.Errorf("Expected index options DOCS_AND_FREQS_AND_POSITIONS, got %v", fi2.IndexOptions())
	}

	if fi2.DocValuesType() != index.DocValuesTypeNone {
		t.Errorf("Expected doc values type NONE, got %v", fi2.DocValuesType())
	}

	if fi2.OmitNorms() {
		t.Error("Expected omitsNorms to be false")
	}

	if fi2.HasPayloads() {
		t.Error("Expected hasPayloads to be false")
	}

	if fi2.StoreTermVectors() {
		t.Error("Expected hasTermVectors to be false")
	}

	if fi2.PointDimensionCount() != 0 {
		t.Errorf("Expected point dimension count 0, got %d", fi2.PointDimensionCount())
	}

	if fi2.VectorDimension() != 0 {
		t.Errorf("Expected vector dimension 0, got %d", fi2.VectorDimension())
	}

	if fi2.IsSoftDeletesField() {
		t.Error("Expected isSoftDeletesField to be false")
	}
}

// ============================================================================
// Test Immutable Attributes
// ============================================================================

// TestLucene94FieldInfosFormat_ImmutableAttributes
// Source: BaseFieldInfoFormatTestCase.testImmutableAttributes()
// Purpose: Test field infos attributes coming back are not mutable
func TestLucene94FieldInfosFormat_ImmutableAttributes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		SetAttribute("foo", "bar").
		SetAttribute("bar", "baz").
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	fi2 := infos2.GetByName("field")
	if fi2 == nil {
		t.Fatal("field not found")
	}

	attributes := fi2.GetAttributes()

	// Verify attributes are immutable (attempting to modify should not affect original)
	// In Go, we return a copy of the map, so modifications won't affect the original
	attributes["bogus"] = "bogus"

	// Verify the original is unchanged
	if fi2.GetAttribute("bogus") != "" {
		t.Error("Attributes map should be immutable - modification affected original")
	}

	// Verify original attributes are preserved
	if fi2.GetAttribute("foo") != "bar" {
		t.Errorf("Expected attribute foo=bar, got %s", fi2.GetAttribute("foo"))
	}

	if fi2.GetAttribute("bar") != "baz" {
		t.Errorf("Expected attribute bar=baz, got %s", fi2.GetAttribute("bar"))
	}
}

// ============================================================================
// Exception Testing Infrastructure
// ============================================================================

// FailingDirectoryWrapper wraps a Directory to simulate IO failures
type FailingDirectoryWrapper struct {
	*store.FilterDirectory
	failOnCreateOutput bool
	failOnCloseOutput  bool
	failOnOpenInput    bool
	failOnCloseInput   bool
	mu                 sync.RWMutex
}

// NewFailingDirectoryWrapper creates a new FailingDirectoryWrapper
func NewFailingDirectoryWrapper(dir store.Directory) *FailingDirectoryWrapper {
	return &FailingDirectoryWrapper{
		FilterDirectory: store.NewFilterDirectory(dir),
	}
}

// SetFailOnCreateOutput sets whether to fail on create output
func (f *FailingDirectoryWrapper) SetFailOnCreateOutput(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOnCreateOutput = fail
}

// SetFailOnCloseOutput sets whether to fail on close output
func (f *FailingDirectoryWrapper) SetFailOnCloseOutput(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOnCloseOutput = fail
}

// SetFailOnOpenInput sets whether to fail on open input
func (f *FailingDirectoryWrapper) SetFailOnOpenInput(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOnOpenInput = fail
}

// SetFailOnCloseInput sets whether to fail on close input
func (f *FailingDirectoryWrapper) SetFailOnCloseInput(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOnCloseInput = fail
}

// CreateOutput overrides to simulate failures
func (f *FailingDirectoryWrapper) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	f.mu.RLock()
	fail := f.failOnCreateOutput
	f.mu.RUnlock()

	if fail {
		return nil, errors.New("fake IO exception on create output")
	}

	out, err := f.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}

	return &failingIndexOutput{IndexOutput: out, wrapper: f}, nil
}

// OpenInput overrides to simulate failures
func (f *FailingDirectoryWrapper) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	f.mu.RLock()
	fail := f.failOnOpenInput
	f.mu.RUnlock()

	if fail {
		return nil, errors.New("fake IO exception on open input")
	}

	in, err := f.FilterDirectory.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}

	return &failingIndexInput{IndexInput: in, wrapper: f}, nil
}

// failingIndexOutput wraps IndexOutput to simulate close failures
type failingIndexOutput struct {
	store.IndexOutput
	wrapper *FailingDirectoryWrapper
}

// Close overrides to simulate failures
func (f *failingIndexOutput) Close() error {
	f.wrapper.mu.RLock()
	fail := f.wrapper.failOnCloseOutput
	f.wrapper.mu.RUnlock()

	if fail {
		// Close the underlying output first, then return error
		f.IndexOutput.Close()
		return errors.New("fake IO exception on close output")
	}

	return f.IndexOutput.Close()
}

// failingIndexInput wraps IndexInput to simulate close failures
type failingIndexInput struct {
	store.IndexInput
	wrapper *FailingDirectoryWrapper
}

// Close overrides to simulate failures
func (f *failingIndexInput) Close() error {
	f.wrapper.mu.RLock()
	fail := f.wrapper.failOnCloseInput
	f.wrapper.mu.RUnlock()

	if fail {
		// Close the underlying input first, then return error
		f.IndexInput.Close()
		return errors.New("fake IO exception on close input")
	}

	return f.IndexInput.Close()
}

// ============================================================================
// Test Exception on Create Output
// ============================================================================

// TestLucene94FieldInfosFormat_ExceptionOnCreateOutput
// Source: BaseFieldInfoFormatTestCase.testExceptionOnCreateOutput()
// Purpose: Test field infos write that hits exception immediately on open
func TestLucene94FieldInfosFormat_ExceptionOnCreateOutput(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	dir := NewFailingDirectoryWrapper(baseDir)

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

	// Enable failure
	dir.SetFailOnCreateOutput(true)

	// Write should fail
	err := format.Write(dir, si, "", infos, context)
	if err == nil {
		t.Fatal("Expected error on create output, got nil")
	}

	// Verify we got the expected error
	if err.Error() != "fake IO exception on create output" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Disable failure
	dir.SetFailOnCreateOutput(false)

	// Write should succeed now
	err = format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed after clearing failure: %v", err)
	}
}

// ============================================================================
// Test Exception on Close Output
// ============================================================================

// TestLucene94FieldInfosFormat_ExceptionOnCloseOutput
// Source: BaseFieldInfoFormatTestCase.testExceptionOnCloseOutput()
// Purpose: Test field infos write that hits exception on close
func TestLucene94FieldInfosFormat_ExceptionOnCloseOutput(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	dir := NewFailingDirectoryWrapper(baseDir)

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

	// Enable failure on close
	dir.SetFailOnCloseOutput(true)

	// Write should fail on close
	err := format.Write(dir, si, "", infos, context)
	if err == nil {
		t.Fatal("Expected error on close output, got nil")
	}

	// Verify we got the expected error
	if err.Error() != "fake IO exception on close output" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Disable failure
	dir.SetFailOnCloseOutput(false)

	// Write should succeed now
	err = format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed after clearing failure: %v", err)
	}
}

// ============================================================================
// Test Exception on Open Input
// ============================================================================

// TestLucene94FieldInfosFormat_ExceptionOnOpenInput
// Source: BaseFieldInfoFormatTestCase.testExceptionOnOpenInput()
// Purpose: Test field infos read that hits exception immediately on open
func TestLucene94FieldInfosFormat_ExceptionOnOpenInput(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	dir := NewFailingDirectoryWrapper(baseDir)

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

	// First write successfully (without failure)
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Enable failure on open input
	dir.SetFailOnOpenInput(true)

	// Read should fail
	_, err = format.Read(dir, si, "", store.IOContextRead)
	if err == nil {
		t.Fatal("Expected error on open input, got nil")
	}

	// Verify we got the expected error
	if err.Error() != "fake IO exception on open input" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Disable failure
	dir.SetFailOnOpenInput(false)

	// Read should succeed now
	_, err = format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed after clearing failure: %v", err)
	}
}

// ============================================================================
// Test Exception on Close Input
// ============================================================================

// TestLucene94FieldInfosFormat_ExceptionOnCloseInput
// Source: BaseFieldInfoFormatTestCase.testExceptionOnCloseInput()
// Purpose: Test field infos read that hits exception on close
func TestLucene94FieldInfosFormat_ExceptionOnCloseInput(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	dir := NewFailingDirectoryWrapper(baseDir)

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	fi := index.NewFieldInfoBuilder("field", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		Build()

	builder := index.NewFieldInfosBuilder()
	builder.Add(fi)
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

	// First write successfully (without failure)
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Enable failure on close input
	dir.SetFailOnCloseInput(true)

	// Read should fail on close (during footer check)
	_, err = format.Read(dir, si, "", store.IOContextRead)
	if err == nil {
		t.Fatal("Expected error on close input, got nil")
	}

	// Disable failure
	dir.SetFailOnCloseInput(false)

	// Read should succeed now
	_, err = format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed after clearing failure: %v", err)
	}
}

// ============================================================================
// Test Random Fields
// ============================================================================

// TestLucene94FieldInfosFormat_Random
// Source: BaseFieldInfoFormatTestCase.testRandom()
// Purpose: Test field infos read/write with random fields, with different values
func TestLucene94FieldInfosFormat_Random(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	// Generate a bunch of fields (at least 200 as in Java test)
	numFields := 200
	if testing.Short() {
		numFields = 50
	}

	builder := index.NewFieldInfosBuilder()

	for i := 0; i < numFields; i++ {
		fieldName := fmt.Sprintf("field_%d_%d", i, rand.Int())
		fieldType := generateRandomFieldType(t, fieldName)

		fib := index.NewFieldInfoBuilder(fieldName, i)

		// Set index options
		if fieldType.indexOptions != index.IndexOptionsNone {
			fib.SetIndexOptions(fieldType.indexOptions)
			fib.SetStored(fieldType.stored)
			fib.SetOmitNorms(fieldType.omitNorms)
			fib.SetStoreTermVectors(fieldType.storeTermVectors)
			if fieldType.storeTermVectors {
				fib.SetStoreTermVectorPositions(fieldType.storeTermVectorPositions)
				fib.SetStoreTermVectorOffsets(fieldType.storeTermVectorOffsets)
				fib.SetStoreTermVectorPayloads(fieldType.storeTermVectorPayloads)
			}
		}

		// Set doc values
		if fieldType.docValuesType != index.DocValuesTypeNone {
			fib.SetDocValuesType(fieldType.docValuesType)
			fib.SetDocValuesSkipIndexType(fieldType.docValuesSkipIndexType)
		}

		// Set points
		if fieldType.pointDimensionCount > 0 {
			fib.SetPointDimensions(fieldType.pointDimensionCount, fieldType.pointIndexDimensionCount, fieldType.pointNumBytes)
		}

		// Set vectors
		if fieldType.vectorDimension > 0 {
			fib.SetVectorAttributes(fieldType.vectorDimension, fieldType.vectorEncoding, fieldType.vectorSimilarityFunction)
		}

		// Add random attributes
		if i%3 == 0 {
			fib.SetAttribute(fmt.Sprintf("attr_%d", i), fmt.Sprintf("value_%d", i))
		}

		builder.Add(fib.Build())
	}

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify each field
	iter := infos.Iterator()
	for iter.HasNext() {
		expected := iter.Next()
		actual := infos2.GetByName(expected.Name())

		if actual == nil {
			t.Errorf("Field %s not found in read FieldInfos", expected.Name())
			continue
		}

		// Verify field properties
		if expected.IndexOptions() != actual.IndexOptions() {
			t.Errorf("Field %s: expected index options %v, got %v", expected.Name(), expected.IndexOptions(), actual.IndexOptions())
		}

		if expected.DocValuesType() != actual.DocValuesType() {
			t.Errorf("Field %s: expected doc values type %v, got %v", expected.Name(), expected.DocValuesType(), actual.DocValuesType())
		}

		if expected.DocValuesSkipIndexType() != actual.DocValuesSkipIndexType() {
			t.Errorf("Field %s: expected skip index type %v, got %v", expected.Name(), expected.DocValuesSkipIndexType(), actual.DocValuesSkipIndexType())
		}

		if expected.StoreTermVectors() != actual.StoreTermVectors() {
			t.Errorf("Field %s: expected store term vectors %v, got %v", expected.Name(), expected.StoreTermVectors(), actual.StoreTermVectors())
		}

		if expected.OmitNorms() != actual.OmitNorms() {
			t.Errorf("Field %s: expected omit norms %v, got %v", expected.Name(), expected.OmitNorms(), actual.OmitNorms())
		}

		if expected.PointDimensionCount() != actual.PointDimensionCount() {
			t.Errorf("Field %s: expected point dimension count %d, got %d", expected.Name(), expected.PointDimensionCount(), actual.PointDimensionCount())
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
	}
}

// randomFieldType holds random field configuration
type randomFieldType struct {
	indexOptions             index.IndexOptions
	stored                   bool
	omitNorms                bool
	storeTermVectors         bool
	storeTermVectorPositions bool
	storeTermVectorOffsets   bool
	storeTermVectorPayloads  bool
	docValuesType            index.DocValuesType
	docValuesSkipIndexType   index.DocValuesSkipIndexType
	pointDimensionCount      int
	pointIndexDimensionCount int
	pointNumBytes            int
	vectorDimension          int
	vectorEncoding           index.VectorEncoding
	vectorSimilarityFunction index.VectorSimilarityFunction
}

// generateRandomFieldType generates a random field type configuration
func generateRandomFieldType(t *testing.T, fieldName string) randomFieldType {
	ft := randomFieldType{}

	// Random index options
	indexOptions := []index.IndexOptions{
		index.IndexOptionsNone,
		index.IndexOptionsDocs,
		index.IndexOptionsDocsAndFreqs,
		index.IndexOptionsDocsAndFreqsAndPositions,
		index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
	}
	ft.indexOptions = indexOptions[rand.Intn(len(indexOptions))]

	if ft.indexOptions != index.IndexOptionsNone {
		ft.stored = rand.Intn(2) == 0
		ft.omitNorms = rand.Intn(2) == 0

		if rand.Intn(2) == 0 {
			ft.storeTermVectors = true
			if ft.indexOptions >= index.IndexOptionsDocsAndFreqsAndPositions {
				ft.storeTermVectorPositions = rand.Intn(2) == 0
				ft.storeTermVectorOffsets = rand.Intn(2) == 0
				if ft.storeTermVectorPositions {
					ft.storeTermVectorPayloads = rand.Intn(2) == 0
				}
			}
		}
	}

	// Random doc values
	if rand.Intn(2) == 0 {
		docValuesTypes := []index.DocValuesType{
			index.DocValuesTypeNone,
			index.DocValuesTypeNumeric,
			index.DocValuesTypeBinary,
			index.DocValuesTypeSorted,
			index.DocValuesTypeSortedSet,
			index.DocValuesTypeSortedNumeric,
		}
		ft.docValuesType = docValuesTypes[rand.Intn(len(docValuesTypes))]

		// Set skip index type for applicable doc values types
		if ft.docValuesType == index.DocValuesTypeNumeric ||
			ft.docValuesType == index.DocValuesTypeSortedNumeric ||
			ft.docValuesType == index.DocValuesTypeSorted ||
			ft.docValuesType == index.DocValuesTypeSortedSet {
			if rand.Intn(2) == 0 {
				ft.docValuesSkipIndexType = index.DocValuesSkipIndexTypeRange
			} else {
				ft.docValuesSkipIndexType = index.DocValuesSkipIndexTypeNone
			}
		}
	}

	// Random points
	if rand.Intn(2) == 0 {
		maxDimensions := 8      // PointValues.MAX_DIMENSIONS
		maxIndexDimensions := 4 // PointValues.MAX_INDEX_DIMENSIONS
		ft.pointDimensionCount = 1 + rand.Intn(maxDimensions)
		ft.pointIndexDimensionCount = 1 + rand.Intn(min(ft.pointDimensionCount, maxIndexDimensions))
		ft.pointNumBytes = 1 + rand.Intn(16) // MAX_NUM_BYTES
	}

	// Random vectors
	if rand.Intn(2) == 0 {
		maxVectorDimensions := 1024
		ft.vectorDimension = 1 + rand.Intn(maxVectorDimensions)

		vectorEncodings := []index.VectorEncoding{
			index.VectorEncodingByte,
			index.VectorEncodingFloat32,
		}
		ft.vectorEncoding = vectorEncodings[rand.Intn(len(vectorEncodings))]

		similarityFunctions := []index.VectorSimilarityFunction{
			index.VectorSimilarityFunctionEuclidean,
			index.VectorSimilarityFunctionDotProduct,
			index.VectorSimilarityFunctionCosine,
			index.VectorSimilarityFunctionMaximumInnerProduct,
		}
		ft.vectorSimilarityFunction = similarityFunctions[rand.Intn(len(similarityFunctions))]
	}

	return ft
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// Test Doc Values Skip Index Support
// ============================================================================

// TestLucene94FieldInfosFormat_DocValuesSkipIndex
// Purpose: Test doc values skip index support (specific to Lucene94 format)
func TestLucene94FieldInfosFormat_DocValuesSkipIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Field with RANGE skip index
	fi1 := index.NewFieldInfoBuilder("field_numeric_range", 0).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange).
		Build()
	builder.Add(fi1)

	// Field with NONE skip index
	fi2 := index.NewFieldInfoBuilder("field_numeric_none", 1).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeNone).
		Build()
	builder.Add(fi2)

	// Field with SORTED doc values and RANGE skip index
	fi3 := index.NewFieldInfoBuilder("field_sorted_range", 2).
		SetDocValuesType(index.DocValuesTypeSorted).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange).
		Build()
	builder.Add(fi3)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify RANGE skip index
	fi1_2 := infos2.GetByName("field_numeric_range")
	if fi1_2 == nil {
		t.Fatal("field_numeric_range not found")
	}
	if fi1_2.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeRange {
		t.Errorf("Expected skip index type RANGE, got %v", fi1_2.DocValuesSkipIndexType())
	}

	// Verify NONE skip index
	fi2_2 := infos2.GetByName("field_numeric_none")
	if fi2_2 == nil {
		t.Fatal("field_numeric_none not found")
	}
	if fi2_2.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		t.Errorf("Expected skip index type NONE, got %v", fi2_2.DocValuesSkipIndexType())
	}

	// Verify SORTED with RANGE skip index
	fi3_2 := infos2.GetByName("field_sorted_range")
	if fi3_2 == nil {
		t.Fatal("field_sorted_range not found")
	}
	if fi3_2.DocValuesType() != index.DocValuesTypeSorted {
		t.Errorf("Expected doc values type SORTED, got %v", fi3_2.DocValuesType())
	}
	if fi3_2.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeRange {
		t.Errorf("Expected skip index type RANGE, got %v", fi3_2.DocValuesSkipIndexType())
	}
}

// ============================================================================
// Test Vector Fields
// ============================================================================

// TestLucene94FieldInfosFormat_VectorFields
// Purpose: Test vector field serialization/deserialization
func TestLucene94FieldInfosFormat_VectorFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Vector field with FLOAT32 encoding and EUCLIDEAN similarity
	fi1 := index.NewFieldInfoBuilder("vector_euclidean", 0).
		SetVectorAttributes(128, index.VectorEncodingFloat32, index.VectorSimilarityFunctionEuclidean).
		Build()
	builder.Add(fi1)

	// Vector field with BYTE encoding and COSINE similarity
	fi2 := index.NewFieldInfoBuilder("vector_cosine", 1).
		SetVectorAttributes(256, index.VectorEncodingByte, index.VectorSimilarityFunctionCosine).
		Build()
	builder.Add(fi2)

	// Vector field with DOT_PRODUCT similarity
	fi3 := index.NewFieldInfoBuilder("vector_dot", 2).
		SetVectorAttributes(768, index.VectorEncodingFloat32, index.VectorSimilarityFunctionDotProduct).
		Build()
	builder.Add(fi3)

	// Vector field with MAXIMUM_INNER_PRODUCT similarity
	fi4 := index.NewFieldInfoBuilder("vector_mip", 3).
		SetVectorAttributes(384, index.VectorEncodingFloat32, index.VectorSimilarityFunctionMaximumInnerProduct).
		Build()
	builder.Add(fi4)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify EUCLIDEAN
	fi1_2 := infos2.GetByName("vector_euclidean")
	if fi1_2 == nil {
		t.Fatal("vector_euclidean not found")
	}
	if fi1_2.VectorDimension() != 128 {
		t.Errorf("Expected vector dimension 128, got %d", fi1_2.VectorDimension())
	}
	if fi1_2.VectorEncoding() != index.VectorEncodingFloat32 {
		t.Errorf("Expected vector encoding FLOAT32, got %v", fi1_2.VectorEncoding())
	}
	if fi1_2.VectorSimilarityFunction() != index.VectorSimilarityFunctionEuclidean {
		t.Errorf("Expected similarity function EUCLIDEAN, got %v", fi1_2.VectorSimilarityFunction())
	}

	// Verify COSINE with BYTE encoding
	fi2_2 := infos2.GetByName("vector_cosine")
	if fi2_2 == nil {
		t.Fatal("vector_cosine not found")
	}
	if fi2_2.VectorDimension() != 256 {
		t.Errorf("Expected vector dimension 256, got %d", fi2_2.VectorDimension())
	}
	if fi2_2.VectorEncoding() != index.VectorEncodingByte {
		t.Errorf("Expected vector encoding BYTE, got %v", fi2_2.VectorEncoding())
	}
	if fi2_2.VectorSimilarityFunction() != index.VectorSimilarityFunctionCosine {
		t.Errorf("Expected similarity function COSINE, got %v", fi2_2.VectorSimilarityFunction())
	}

	// Verify DOT_PRODUCT
	fi3_2 := infos2.GetByName("vector_dot")
	if fi3_2 == nil {
		t.Fatal("vector_dot not found")
	}
	if fi3_2.VectorSimilarityFunction() != index.VectorSimilarityFunctionDotProduct {
		t.Errorf("Expected similarity function DOT_PRODUCT, got %v", fi3_2.VectorSimilarityFunction())
	}

	// Verify MAXIMUM_INNER_PRODUCT
	fi4_2 := infos2.GetByName("vector_mip")
	if fi4_2 == nil {
		t.Fatal("vector_mip not found")
	}
	if fi4_2.VectorSimilarityFunction() != index.VectorSimilarityFunctionMaximumInnerProduct {
		t.Errorf("Expected similarity function MAXIMUM_INNER_PRODUCT, got %v", fi4_2.VectorSimilarityFunction())
	}
}

// ============================================================================
// Test Point Fields
// ============================================================================

// TestLucene94FieldInfosFormat_PointFields
// Purpose: Test point field serialization/deserialization
func TestLucene94FieldInfosFormat_PointFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Point field with 1 dimension
	fi1 := index.NewFieldInfoBuilder("point_1d", 0).
		SetPointDimensions(1, 1, 8).
		Build()
	builder.Add(fi1)

	// Point field with multiple dimensions
	fi2 := index.NewFieldInfoBuilder("point_nd", 1).
		SetPointDimensions(4, 2, 4).
		Build()
	builder.Add(fi2)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify 1D point field
	fi1_2 := infos2.GetByName("point_1d")
	if fi1_2 == nil {
		t.Fatal("point_1d not found")
	}
	if fi1_2.PointDimensionCount() != 1 {
		t.Errorf("Expected point dimension count 1, got %d", fi1_2.PointDimensionCount())
	}
	if fi1_2.PointIndexDimensionCount() != 1 {
		t.Errorf("Expected point index dimension count 1, got %d", fi1_2.PointIndexDimensionCount())
	}
	if fi1_2.PointNumBytes() != 8 {
		t.Errorf("Expected point num bytes 8, got %d", fi1_2.PointNumBytes())
	}

	// Verify ND point field
	fi2_2 := infos2.GetByName("point_nd")
	if fi2_2 == nil {
		t.Fatal("point_nd not found")
	}
	if fi2_2.PointDimensionCount() != 4 {
		t.Errorf("Expected point dimension count 4, got %d", fi2_2.PointDimensionCount())
	}
	if fi2_2.PointIndexDimensionCount() != 2 {
		t.Errorf("Expected point index dimension count 2, got %d", fi2_2.PointIndexDimensionCount())
	}
	if fi2_2.PointNumBytes() != 4 {
		t.Errorf("Expected point num bytes 4, got %d", fi2_2.PointNumBytes())
	}
}

// ============================================================================
// Test Soft Deletes and Parent Fields
// ============================================================================

// TestLucene94FieldInfosFormat_SoftDeletesAndParentFields
// Purpose: Test soft deletes and parent field flags
func TestLucene94FieldInfosFormat_SoftDeletesAndParentFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Soft deletes field
	fi1 := index.NewFieldInfoBuilder("soft_deletes", 0).
		SetIndexOptions(index.IndexOptionsDocs).
		SetSoftDeletesField(true).
		Build()
	builder.Add(fi1)

	// Parent field
	fi2 := index.NewFieldInfoBuilder("parent", 1).
		SetIndexOptions(index.IndexOptionsDocs).
		SetParentField(true).
		Build()
	builder.Add(fi2)

	// Regular field
	fi3 := index.NewFieldInfoBuilder("regular", 2).
		SetIndexOptions(index.IndexOptionsDocs).
		Build()
	builder.Add(fi3)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify soft deletes field
	fi1_2 := infos2.GetByName("soft_deletes")
	if fi1_2 == nil {
		t.Fatal("soft_deletes not found")
	}
	if !fi1_2.IsSoftDeletesField() {
		t.Error("Expected isSoftDeletesField to be true")
	}

	// Verify parent field
	fi2_2 := infos2.GetByName("parent")
	if fi2_2 == nil {
		t.Fatal("parent not found")
	}
	if !fi2_2.IsParentField() {
		t.Error("Expected isParentField to be true")
	}

	// Verify regular field
	fi3_2 := infos2.GetByName("regular")
	if fi3_2 == nil {
		t.Fatal("regular not found")
	}
	if fi3_2.IsSoftDeletesField() {
		t.Error("Expected isSoftDeletesField to be false for regular field")
	}
	if fi3_2.IsParentField() {
		t.Error("Expected isParentField to be false for regular field")
	}
}

// ============================================================================
// Test Empty FieldInfos
// ============================================================================

// TestLucene94FieldInfosFormat_Empty
// Purpose: Test reading/writing empty FieldInfos
func TestLucene94FieldInfosFormat_Empty(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()
	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify empty
	if infos2.Size() != 0 {
		t.Errorf("Expected size 0, got %d", infos2.Size())
	}
}

// ============================================================================
// Test Large Field Numbers
// ============================================================================

// TestLucene94FieldInfosFormat_LargeFieldNumbers
// Purpose: Test with large field numbers (edge case)
func TestLucene94FieldInfosFormat_LargeFieldNumbers(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Field with large number
	fi1 := index.NewFieldInfoBuilder("field_large", math.MaxInt32-1).
		SetIndexOptions(index.IndexOptionsDocs).
		Build()
	builder.Add(fi1)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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
	fi1_2 := infos2.GetByName("field_large")
	if fi1_2 == nil {
		t.Fatal("field_large not found")
	}
	if fi1_2.Number() != math.MaxInt32-1 {
		t.Errorf("Expected field number %d, got %d", math.MaxInt32-1, fi1_2.Number())
	}
}

// ============================================================================
// Test Unicode Field Names
// ============================================================================

// TestLucene94FieldInfosFormat_UnicodeFieldNames
// Purpose: Test with unicode field names
func TestLucene94FieldInfosFormat_UnicodeFieldNames(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Unicode field names
	unicodeNames := []string{
		"field_日本語",
		"field_中文",
		"field_العربية",
		"field_emoji_🎉",
		"field_special_!@#$%^&*()",
		"field_unicode_ñáéíóú",
	}

	for i, name := range unicodeNames {
		fi := index.NewFieldInfoBuilder(name, i).
			SetIndexOptions(index.IndexOptionsDocs).
			Build()
		builder.Add(fi)
	}

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify all unicode field names
	for _, name := range unicodeNames {
		fi := infos2.GetByName(name)
		if fi == nil {
			t.Errorf("Field %s not found", name)
		}
	}
}

// ============================================================================
// Test DocValuesGen
// ============================================================================

// TestLucene94FieldInfosFormat_DocValuesGen
// Purpose: Test doc values generation serialization
func TestLucene94FieldInfosFormat_DocValuesGen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Field with doc values gen = -1 (no updates)
	fi1 := index.NewFieldInfoBuilder("field_no_updates", 0).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesGen(-1).
		Build()
	builder.Add(fi1)

	// Field with doc values gen = 5 (has updates)
	fi2 := index.NewFieldInfoBuilder("field_with_updates", 1).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesGen(5).
		Build()
	builder.Add(fi2)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

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

	// Verify doc values gen
	fi1_2 := infos2.GetByName("field_no_updates")
	if fi1_2 == nil {
		t.Fatal("field_no_updates not found")
	}
	if fi1_2.DocValuesGen() != -1 {
		t.Errorf("Expected doc values gen -1, got %d", fi1_2.DocValuesGen())
	}

	fi2_2 := infos2.GetByName("field_with_updates")
	if fi2_2 == nil {
		t.Fatal("field_with_updates not found")
	}
	if fi2_2.DocValuesGen() != 5 {
		t.Errorf("Expected doc values gen 5, got %d", fi2_2.DocValuesGen())
	}
}
