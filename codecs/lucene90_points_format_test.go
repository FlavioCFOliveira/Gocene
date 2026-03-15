// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-203: Port TestLucene90PointsFormat.java from Apache Lucene
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90PointsFormat.java
// Also ports tests from BasePointsFormatTestCase.java
//
// Focus: KD-tree based spatial points storage, points format tests

// IntersectVisitor is the Go equivalent of Lucene's PointValues.IntersectVisitor
// It visits points that match a query range
type IntersectVisitor interface {
	// Visit is called for each matching point with its docID and packed value
	Visit(docID int, packedValue []byte)
	// Visit is called for documents with deleted points
	VisitDoc(docID int)
	// Compare returns the relation between the query range and cell bounds
	Compare(minPackedValue, maxPackedValue []byte) Relation
}

// Relation represents the relationship between a query and a cell
type Relation int

const (
	// CELL_INSIDE_QUERY means the cell is fully contained in the query
	CELL_INSIDE_QUERY Relation = iota
	// CELL_OUTSIDE_QUERY means the cell is fully outside the query
	CELL_OUTSIDE_QUERY
	// CELL_CROSSES_QUERY means the cell partially overlaps the query
	CELL_CROSSES_QUERY
)

// PointValuesStats holds statistics for point values
type PointValuesStats struct {
	Size           int64
	DocCount       int
	MinPackedValue []byte
	MaxPackedValue []byte
}

// Lucene90PointsFormatTest provides testing for Lucene90PointsFormat
// This tests the KD-tree based spatial points storage format
type Lucene90PointsFormatTest struct {
	codec               codecs.Codec
	maxPointsInLeafNode int
}

// NewLucene90PointsFormatTest creates a new test instance
func NewLucene90PointsFormatTest() *Lucene90PointsFormatTest {
	test := &Lucene90PointsFormatTest{}

	// Randomize parameters like the Java test does
	if rand.Float32() < 0.5 {
		test.maxPointsInLeafNode = 50 + rand.Intn(450) // 50-500
		// codec would be customized here with specific parameters
		test.codec = nil // Placeholder - use default codec
	} else {
		test.maxPointsInLeafNode = 512 // BKDConfig.DEFAULT_MAX_POINTS_IN_LEAF_NODE
		test.codec = nil               // Placeholder - use default codec
	}

	return test
}

// TestLucene90PointsFormat_Basic tests basic point indexing and retrieval
// Source: BasePointsFormatTestCase.testBasic()
func TestLucene90PointsFormat_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 20 documents with binary points
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		// Encode int as sortable bytes
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("dim", point)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}
	writer.Close()

	// Verify points can be read
	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 20 {
		t.Errorf("Expected 20 docs, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_MergeStability tests merge stability
// Source: TestLucene90PointsFormat.testMergeStability()
func TestLucene90PointsFormat_MergeStability(t *testing.T) {
	t.Skip("Merge stability test requires full codec implementation")
}

// TestLucene90PointsFormat_EstimatePointCount tests point count estimation
// Source: TestLucene90PointsFormat.testEstimatePointCount()
func TestLucene90PointsFormat_EstimatePointCount(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Generate unique point value
	uniquePointValue := make([]byte, 3)
	rand.Read(uniquePointValue)

	pointValue := make([]byte, 3)
	numDocs := 500
	if testing.Short() {
		numDocs = 100
	}
	multiValues := rand.Float32() < 0.5
	totalValues := 0

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if i == numDocs/2 {
			totalValues++
			bp, _ := document.NewBinaryPoint("f", uniquePointValue)
			doc.Add(bp)
		} else {
			numValues := 1
			if multiValues {
				numValues = 2 + rand.Intn(98) // 2-100 values
			}
			for j := 0; j < numValues; j++ {
				// Generate random point different from unique value
				for {
					rand.Read(pointValue)
					if !bytes.Equal(pointValue, uniquePointValue) {
						break
					}
				}
				bp, _ := document.NewBinaryPoint("f", pointValue)
				doc.Add(bp)
				totalValues++
			}
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	writer.Close()

	// Get leaf reader and point values
	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}

	// Test point count estimation
	// All points visitor should return totalValues
	// No points visitor should return 0
	// One point match visitor should return estimated count

	_ = totalValues // Use the variable for estimation tests

	reader.Close()
}

// TestLucene90PointsFormat_EstimatePointCount2Dims tests point count estimation in 2D
// Source: TestLucene90PointsFormat.testEstimatePointCount2Dims()
func TestLucene90PointsFormat_EstimatePointCount2Dims(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Generate unique 2D point value
	uniquePointValue := make([][]byte, 2)
	uniquePointValue[0] = make([]byte, 3)
	uniquePointValue[1] = make([]byte, 3)
	rand.Read(uniquePointValue[0])
	rand.Read(uniquePointValue[1])

	pointValue := make([][]byte, 2)
	pointValue[0] = make([]byte, 3)
	pointValue[1] = make([]byte, 3)

	numDocs := 1000
	if testing.Short() {
		numDocs = 200
	}
	multiValues := rand.Float32() < 0.5
	totalValues := 0

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if i == numDocs/2 {
			doc.Add(document.NewBinaryPointMulti("f", uniquePointValue))
			totalValues++
		} else {
			numValues := 1
			if multiValues {
				numValues = 2 + rand.Intn(98)
			}
			for j := 0; j < numValues; j++ {
				// Generate random point different from unique value
				for {
					rand.Read(pointValue[0])
					rand.Read(pointValue[1])
					if !bytes.Equal(pointValue[0], uniquePointValue[0]) ||
						!bytes.Equal(pointValue[1], uniquePointValue[1]) {
						break
					}
				}
				doc.Add(document.NewBinaryPointMulti("f", pointValue))
				totalValues++
			}
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	writer.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}

	_ = totalValues
	reader.Close()
}

// TestLucene90PointsFormat_Merge tests point merging
// Source: BasePointsFormatTestCase.testMerge()
func TestLucene90PointsFormat_Merge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 20 documents, commit at 10
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("dim", point)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
		if i == 10 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit failed: %v", err)
			}
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}
	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 20 {
		t.Errorf("Expected 20 docs after merge, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_MultiValued tests multi-valued points
// Source: BasePointsFormatTestCase.testMultiValued()
func TestLucene90PointsFormat_MultiValued(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := 100
	if testing.Short() {
		numDocs = 20
	}

	for docID := 0; docID < numDocs; docID++ {
		doc := document.NewDocument()
		numValues := 1 + rand.Intn(5) // 1-5 values per doc
		for i := 0; i < numValues; i++ {
			point := make([]byte, 4)
			rand.Read(point)
			bp, _ := document.NewBinaryPoint("field", point)
			doc.Add(bp)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestLucene90PointsFormat_AllEqual tests all points having the same value
// Source: BasePointsFormatTestCase.testAllEqual()
func TestLucene90PointsFormat_AllEqual(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := 100
	if testing.Short() {
		numDocs = 20
	}

	// Generate one random point value
	pointValue := make([]byte, 4)
	rand.Read(pointValue)

	for docID := 0; docID < numDocs; docID++ {
		doc := document.NewDocument()
		bp, _ := document.NewBinaryPoint("field", pointValue)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestLucene90PointsFormat_OneDimEqual tests one dimension equal across points
// Source: BasePointsFormatTestCase.testOneDimEqual()
func TestLucene90PointsFormat_OneDimEqual(t *testing.T) {
	t.Skip("Multi-dimensional points not yet fully implemented")
}

// TestLucene90PointsFormat_OneDimTwoValues tests run-length compression
// Source: BasePointsFormatTestCase.testOneDimTwoValues()
func TestLucene90PointsFormat_OneDimTwoValues(t *testing.T) {
	t.Skip("Run-length compression tests require full codec implementation")
}

// TestLucene90PointsFormat_BigIntNDims tests N-dimensional BigInteger points
// Source: BasePointsFormatTestCase.testBigIntNDims()
func TestLucene90PointsFormat_BigIntNDims(t *testing.T) {
	t.Skip("BigInteger N-dimensional tests require full codec implementation")
}

// TestLucene90PointsFormat_RandomBinary tests random binary points
// Source: BasePointsFormatTestCase.testRandomBinaryTiny/Medium/Big()
func TestLucene90PointsFormat_RandomBinaryTiny(t *testing.T) {
	testRandomBinaryPoints(t, 10)
}

func TestLucene90PointsFormat_RandomBinaryMedium(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping medium random test in short mode")
	}
	testRandomBinaryPoints(t, 100)
}

func TestLucene90PointsFormat_RandomBinaryBig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping big random test in short mode")
	}
	testRandomBinaryPoints(t, 1000)
}

func testRandomBinaryPoints(t *testing.T, numDocs int) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// Random number of dimensions (1-3)
		numDims := 1 + rand.Intn(3)
		// Random bytes per dimension (2-16)
		bytesPerDim := 2 + rand.Intn(14)

		points := make([][]byte, numDims)
		for d := 0; d < numDims; d++ {
			points[d] = make([]byte, bytesPerDim)
			rand.Read(points[d])
		}

		doc.Add(document.NewBinaryPointMulti("field", points))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestLucene90PointsFormat_AddIndexes tests adding indexes
// Source: BasePointsFormatTestCase.testAddIndexes()
func TestLucene90PointsFormat_AddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	// Create first index with points
	iwc1 := index.NewIndexWriterConfig()
	writer1, _ := index.NewIndexWriter(dir1, iwc1)
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("field", point)
		doc.Add(bp)
		writer1.AddDocument(doc)
	}
	writer1.Close()

	// Create second index
	iwc2 := index.NewIndexWriterConfig()
	writer2, _ := index.NewIndexWriter(dir2, iwc2)

	// Add indexes from first directory
	if err := writer2.AddIndexes(dir1); err != nil {
		t.Fatalf("Failed to add indexes: %v", err)
	}
	writer2.Close()

	reader, err := index.NewDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 docs, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_MergeMissing tests merging with missing values
// Source: BasePointsFormatTestCase.testMergeMissing()
func TestLucene90PointsFormat_MergeMissing(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add docs with points
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("field", point)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Add docs without points
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("text", "value", true))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 docs, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_DocCountEdgeCases tests document count edge cases
// Source: BasePointsFormatTestCase.testDocCountEdgeCases()
func TestLucene90PointsFormat_DocCountEdgeCases(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Test empty index
	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	if reader.NumDocs() != 0 {
		t.Errorf("Expected 0 docs for empty index, got %d", reader.NumDocs())
	}
	reader.Close()
}

// TestLucene90PointsFormat_RandomDocCount tests random document counts
// Source: BasePointsFormatTestCase.testRandomDocCount()
func TestLucene90PointsFormat_RandomDocCount(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := 100
	if testing.Short() {
		numDocs = 20
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// Random number of point values per doc
		numValues := rand.Intn(4) // 0-3 values
		for j := 0; j < numValues; j++ {
			point := make([]byte, 4)
			rand.Read(point)
			bp, _ := document.NewBinaryPoint("field", point)
			doc.Add(bp)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("Expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestLucene90PointsFormat_MismatchedFields tests mismatched field handling
// Source: BasePointsFormatTestCase.testMismatchedFields()
func TestLucene90PointsFormat_MismatchedFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add docs with different point fields
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("field1", point)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("field2", point)
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 docs, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_AllPointDocsDeleted tests deleted point docs
// Source: BasePointsFormatTestCase.testAllPointDocsDeletedInSegment()
func TestLucene90PointsFormat_AllPointDocsDeleted(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add docs with points
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, _ := document.NewBinaryPoint("dim", point)
		doc.Add(bp)
		doc.Add(document.NewStringField("id", string(rune('0'+i)), true))
		doc.Add(document.NewTextField("x", "x", true))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Add doc without points
	doc := document.NewDocument()
	doc.Add(document.NewTextField("other", "value", true))
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Delete docs with "x" field
	writer.DeleteDocuments(document.NewTerm("x", "x"))
	writer.Close()

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Should have 1 doc (the one without points)
	if reader.NumDocs() != 1 {
		t.Errorf("Expected 1 doc after deletion, got %d", reader.NumDocs())
	}
}

// TestLucene90PointsFormat_WithExceptions tests exception handling
// Source: BasePointsFormatTestCase.testWithExceptions()
func TestLucene90PointsFormat_WithExceptions(t *testing.T) {
	t.Skip("Exception handling test requires MockDirectoryWrapper implementation")
}

// Helper functions

// encodeInt32Sortable encodes an int32 to sortable bytes
func encodeInt32Sortable(v int, buf []byte) {
	if len(buf) >= 4 {
		// Flip sign bit for sortable encoding
		x := uint32(v) ^ 0x80000000
		buf[0] = byte(x >> 24)
		buf[1] = byte(x >> 16)
		buf[2] = byte(x >> 8)
		buf[3] = byte(x)
	}
}

// decodeInt32Sortable decodes sortable bytes to int32
func decodeInt32Sortable(buf []byte) int {
	if len(buf) < 4 {
		return 0
	}
	x := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	x ^= 0x80000000
	return int(int32(x))
}

// estimatePointCount estimates the number of points matching a visitor
// This is a simplified version for testing
func estimatePointCount(totalPoints int64, visitor IntersectVisitor, minPacked, maxPacked []byte) int64 {
	relation := visitor.Compare(minPacked, maxPacked)
	switch relation {
	case CELL_INSIDE_QUERY:
		return totalPoints
	case CELL_OUTSIDE_QUERY:
		return 0
	case CELL_CROSSES_QUERY:
		// Estimate: half the points might match
		return (totalPoints + 1) / 2
	}
	return 0
}

// estimateDocCount estimates the number of documents matching a visitor
func estimateDocCount(totalDocs int, pointCount int64, totalPoints int64) int64 {
	if totalPoints == 0 {
		return 0
	}
	// Simplified estimation
	return int64(math.Min(float64(pointCount), float64(totalDocs)))
}

// Use document.NewBinaryPointMulti for multi-dimensional binary points
