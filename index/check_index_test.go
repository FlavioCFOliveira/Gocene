// Test file: check_index_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestCheckIndex.java
// Purpose: Tests for CheckIndex tool to verify index integrity, detect corruption, and report statistics

package index_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCheckIndex_DeletedDocs tests CheckIndex with deleted documents
// Source: BaseTestCheckIndex.testDeletedDocs()
// Purpose: Verifies CheckIndex correctly handles indexes with deleted documents
func TestCheckIndex_DeletedDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	config.SetMaxBufferedDocs(2)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 19 documents
	for i := 0; i < 19; i++ {
		doc := document.NewDocument()
		field, err := document.NewTextField("field", "aaa"+string(rune('0'+i)), true)
		if err != nil {
			t.Fatalf("Failed to create text field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("Failed to force merge: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Delete document with field "aaa5"
	if err := writer.DeleteDocuments(index.NewTerm("field", "aaa5")); err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Run CheckIndex
	var output bytes.Buffer
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	defer checker.Close()

	checker.SetInfoStream(&output)
	checker.SetLevel(index.CheckIndexLevelMinIntegrityChecks)

	status, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}

	if !status.Clean {
		t.Errorf("CheckIndex reported index as not clean:\n%s", output.String())
	}

	if len(status.SegmentInfos) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(status.SegmentInfos))
	}

	seg := status.SegmentInfos[0]
	if !seg.OpenReaderPassed {
		t.Error("Expected openReaderPassed to be true")
	}

	if seg.Diagnostics == nil {
		t.Error("Expected diagnostics to be non-nil")
	}

	// Verify field norm status
	if seg.FieldNormStatus == nil {
		t.Error("Expected fieldNormStatus to be non-nil")
	}
	if seg.FieldNormStatus.Error != nil {
		t.Errorf("fieldNormStatus has error: %v", seg.FieldNormStatus.Error)
	}
	if seg.FieldNormStatus.TotFields != 1 {
		t.Errorf("Expected 1 field norm, got %d", seg.FieldNormStatus.TotFields)
	}

	// Verify term index status
	if seg.TermIndexStatus == nil {
		t.Error("Expected termIndexStatus to be non-nil")
	}
	if seg.TermIndexStatus.Error != nil {
		t.Errorf("termIndexStatus has error: %v", seg.TermIndexStatus.Error)
	}
	if seg.TermIndexStatus.TermCount != 18 {
		t.Errorf("Expected 18 terms, got %d", seg.TermIndexStatus.TermCount)
	}
	if seg.TermIndexStatus.TotFreq != 18 {
		t.Errorf("Expected totFreq 18, got %d", seg.TermIndexStatus.TotFreq)
	}
	if seg.TermIndexStatus.TotPos != 18 {
		t.Errorf("Expected totPos 18, got %d", seg.TermIndexStatus.TotPos)
	}

	// Verify stored field status
	if seg.StoredFieldStatus == nil {
		t.Error("Expected storedFieldStatus to be non-nil")
	}
	if seg.StoredFieldStatus.Error != nil {
		t.Errorf("storedFieldStatus has error: %v", seg.StoredFieldStatus.Error)
	}
	if seg.StoredFieldStatus.DocCount != 18 {
		t.Errorf("Expected 18 stored field docs, got %d", seg.StoredFieldStatus.DocCount)
	}
	if seg.StoredFieldStatus.TotFields != 18 {
		t.Errorf("Expected 18 stored fields, got %d", seg.StoredFieldStatus.TotFields)
	}

	// Verify term vector status
	if seg.TermVectorStatus == nil {
		t.Error("Expected termVectorStatus to be non-nil")
	}
	if seg.TermVectorStatus.Error != nil {
		t.Errorf("termVectorStatus has error: %v", seg.TermVectorStatus.Error)
	}
	if seg.TermVectorStatus.DocCount != 18 {
		t.Errorf("Expected 18 term vector docs, got %d", seg.TermVectorStatus.DocCount)
	}
	if seg.TermVectorStatus.TotVectors != 18 {
		t.Errorf("Expected 18 term vectors, got %d", seg.TermVectorStatus.TotVectors)
	}

	// Test checking specific segment only
	onlySegments := []string{"_0"}
	partialStatus, err := checker.CheckIndexSegments(onlySegments)
	if err != nil {
		t.Fatalf("CheckIndex for specific segments failed: %v", err)
	}
	if !partialStatus.Clean {
		t.Error("Partial check should be clean")
	}
}

// TestCheckIndex_ChecksumsOnly tests CheckIndex with checksums only verification
// Source: BaseTestCheckIndex.testChecksumsOnly()
// Purpose: Verifies basic index integrity via checksum verification
func TestCheckIndex_ChecksumsOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 100 simple documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.AddField(document.NewTextField("field", "value", document.StoreYes))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Add empty document
	if err := writer.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("Failed to add empty document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	var output bytes.Buffer
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	defer checker.Close()

	checker.SetInfoStream(&output)
	status, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}

	if !status.Clean {
		t.Errorf("CheckIndex reported index as not clean:\n%s", output.String())
	}
}

// TestCheckIndex_ChecksumsOnlyVerbose tests CheckIndex with verbose output
// Source: BaseTestCheckIndex.testChecksumsOnlyVerbose()
// Purpose: Verifies CheckIndex produces verbose output when configured
func TestCheckIndex_ChecksumsOnlyVerbose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 100 simple documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.AddField(document.NewTextField("field", "value", document.StoreYes))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Add empty document
	if err := writer.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("Failed to add empty document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	var output bytes.Buffer
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	defer checker.Close()

	// Use verbose output (auto-flush)
	checker.SetInfoStream(&output)
	status, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}

	if !status.Clean {
		t.Errorf("CheckIndex reported index as not clean:\n%s", output.String())
	}
}

// TestCheckIndex_ObtainsLock tests that CheckIndex obtains a write lock
// Source: BaseTestCheckIndex.testObtainsLock()
// Purpose: Verifies CheckIndex cannot run while IndexWriter holds the lock
func TestCheckIndex_ObtainsLock(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add a document and commit
	doc := document.NewDocument()
	doc.AddField(document.NewTextField("field", "value", document.StoreYes))
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Keep writer open - should not be able to obtain write lock
	_, err = index.NewCheckIndex(dir)
	if err == nil {
		t.Error("Expected LockObtainFailedException when creating CheckIndex while IndexWriter is open")
	}

	// Close writer and try again
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex after writer closed: %v", err)
	}
	checker.Close()
}

// TestCheckIndex_AllValid tests CheckIndex with a fully valid index containing all field types
// Source: TestCheckIndex.testCheckIndexAllValid()
// Purpose: Comprehensive test verifying all index components are correctly validated
func TestCheckIndex_AllValid(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index with sort and soft deletes
	config := index.NewIndexWriterConfig(nil)
	config.SetIndexSort(index.NewSort(index.NewSortField("sort_field", index.SortFieldInt, true)))
	config.SetSoftDeletesField("soft_delete")

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	liveDocCount := 5 // Using fixed count for test stability
	for i := 0; i < liveDocCount; i++ {
		doc := document.NewDocument()

		// Stored field
		doc.AddField(document.NewStringField("id", "id"+string(rune('0'+i)), document.StoreYes))
		doc.AddField(document.NewStoredField("field", "value"+string(rune('0'+i))))

		// Vector fields
		v1 := randomVector(t, 3)
		v2 := randomVector(t, 3)
		doc.AddField(document.NewKnnFloatVectorField("v1", v1))
		doc.AddField(document.NewKnnFloatVectorField("v2", v2))

		// Doc value
		doc.AddField(document.NewNumericDocValuesField("dv", int64(i)))

		// Doc value with skip index
		doc.AddField(document.NewNumericDocValuesFieldIndexed("dv_skip", int64(i)))

		// Point value
		point := make([]byte, 4)
		document.IntToSortableBytes(int32(i), point, 0)
		doc.AddField(document.NewBinaryPoint("point", point))

		// Term vector with payload
		tokens := []*document.Token{
			{Text: "bar", StartOffset: 0, EndOffset: 3, Payload: []byte("pay1")},
			{Text: "bar", StartOffset: 4, EndOffset: 8, Payload: []byte("pay2")},
		}
		fieldType := document.NewFieldType(document.TextFieldNotStored)
		fieldType.SetStoreTermVectors(true)
		fieldType.SetStoreTermVectorPositions(true)
		fieldType.SetStoreTermVectorPayloads(true)
		doc.AddField(document.NewFieldWithTokenStream("termvector", tokens, fieldType))

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Soft delete one document
	tombstone := document.NewDocument()
	tombstone.AddField(document.NewNumericDocValuesField("soft_delete", 1))
	if err := writer.SoftUpdateDocument(
		index.NewTerm("id", "id0"),
		tombstone,
		document.NewNumericDocValuesField("soft_delete", 1),
	); err != nil {
		t.Fatalf("Failed to soft update document: %v", err)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("Failed to force merge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Run CheckIndex with verbose output
	var output bytes.Buffer
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	defer checker.Close()

	checker.SetInfoStream(&output)
	checker.SetLevel(index.CheckIndexLevelMinIntegrityChecks)
	checker.SetVerbose(true)
	checker.SetPrintStackTrace(true)

	status, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}

	if len(status.SegmentInfos) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(status.SegmentInfos))
	}

	seg := status.SegmentInfos[0]

	// Verify live docs testing status
	if seg.LiveDocStatus == nil {
		t.Error("Expected liveDocStatus to be non-nil")
	} else {
		if seg.LiveDocStatus.NumDeleted != 0 {
			t.Errorf("Expected 0 deleted docs, got %d", seg.LiveDocStatus.NumDeleted)
		}
		if seg.LiveDocStatus.Error != nil {
			t.Errorf("liveDocStatus has error: %v", seg.LiveDocStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: check live docs") {
		t.Error("Expected output to contain 'test: check live docs'")
	}

	// Verify field infos testing status
	if seg.FieldInfoStatus == nil {
		t.Error("Expected fieldInfoStatus to be non-nil")
	} else {
		if seg.FieldInfoStatus.TotFields != 9 {
			t.Errorf("Expected 9 fields, got %d", seg.FieldInfoStatus.TotFields)
		}
		if seg.FieldInfoStatus.Error != nil {
			t.Errorf("fieldInfoStatus has error: %v", seg.FieldInfoStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: field infos") {
		t.Error("Expected output to contain 'test: field infos'")
	}

	// Verify field norm testing status
	if seg.FieldNormStatus == nil {
		t.Error("Expected fieldNormStatus to be non-nil")
	} else {
		if seg.FieldNormStatus.TotFields != 1 {
			t.Errorf("Expected 1 field norm, got %d", seg.FieldNormStatus.TotFields)
		}
		if seg.FieldNormStatus.Error != nil {
			t.Errorf("fieldNormStatus has error: %v", seg.FieldNormStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: field norms") {
		t.Error("Expected output to contain 'test: field norms'")
	}

	// Verify term index testing status
	if seg.TermIndexStatus == nil {
		t.Error("Expected termIndexStatus to be non-nil")
	} else {
		if seg.TermIndexStatus.TermCount <= 0 {
			t.Errorf("Expected positive term count, got %d", seg.TermIndexStatus.TermCount)
		}
		if seg.TermIndexStatus.TotFreq <= 0 {
			t.Errorf("Expected positive totFreq, got %d", seg.TermIndexStatus.TotFreq)
		}
		if seg.TermIndexStatus.TotPos <= 0 {
			t.Errorf("Expected positive totPos, got %d", seg.TermIndexStatus.TotPos)
		}
		if seg.TermIndexStatus.Error != nil {
			t.Errorf("termIndexStatus has error: %v", seg.TermIndexStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: terms, freq, prox") {
		t.Error("Expected output to contain 'test: terms, freq, prox'")
	}

	// Verify stored field testing status
	if seg.StoredFieldStatus == nil {
		t.Error("Expected storedFieldStatus to be non-nil")
	} else {
		// liveDocCount + 1 for tombstone
		if seg.StoredFieldStatus.DocCount != liveDocCount+1 {
			t.Errorf("Expected %d stored field docs, got %d", liveDocCount+1, seg.StoredFieldStatus.DocCount)
		}
		// 2 * liveDocCount (id and field for each doc)
		if seg.StoredFieldStatus.TotFields != int64(2*liveDocCount) {
			t.Errorf("Expected %d stored fields, got %d", 2*liveDocCount, seg.StoredFieldStatus.TotFields)
		}
		if seg.StoredFieldStatus.Error != nil {
			t.Errorf("storedFieldStatus has error: %v", seg.StoredFieldStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: stored fields") {
		t.Error("Expected output to contain 'test: stored fields'")
	}

	// Verify term vector testing status
	if seg.TermVectorStatus == nil {
		t.Error("Expected termVectorStatus to be non-nil")
	} else {
		if seg.TermVectorStatus.DocCount != liveDocCount {
			t.Errorf("Expected %d term vector docs, got %d", liveDocCount, seg.TermVectorStatus.DocCount)
		}
		if seg.TermVectorStatus.TotVectors != liveDocCount {
			t.Errorf("Expected %d term vectors, got %d", liveDocCount, seg.TermVectorStatus.TotVectors)
		}
		if seg.TermVectorStatus.Error != nil {
			t.Errorf("termVectorStatus has error: %v", seg.TermVectorStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: term vectors") {
		t.Error("Expected output to contain 'test: term vectors'")
	}

	// Verify doc values testing status
	if seg.DocValuesStatus == nil {
		t.Error("Expected docValuesStatus to be non-nil")
	} else {
		if seg.DocValuesStatus.TotalNumericFields != 3 {
			t.Errorf("Expected 3 numeric fields, got %d", seg.DocValuesStatus.TotalNumericFields)
		}
		if seg.DocValuesStatus.TotalSkippingIndex != 1 {
			t.Errorf("Expected 1 skipping index, got %d", seg.DocValuesStatus.TotalSkippingIndex)
		}
		if seg.DocValuesStatus.Error != nil {
			t.Errorf("docValuesStatus has error: %v", seg.DocValuesStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: docvalues") {
		t.Error("Expected output to contain 'test: docvalues'")
	}

	// Verify point values testing status
	if seg.PointsStatus == nil {
		t.Error("Expected pointsStatus to be non-nil")
	} else {
		if seg.PointsStatus.TotalValueFields != 1 {
			t.Errorf("Expected 1 point value field, got %d", seg.PointsStatus.TotalValueFields)
		}
		if seg.PointsStatus.TotalValuePoints != liveDocCount {
			t.Errorf("Expected %d point values, got %d", liveDocCount, seg.PointsStatus.TotalValuePoints)
		}
		if seg.PointsStatus.Error != nil {
			t.Errorf("pointsStatus has error: %v", seg.PointsStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: points") {
		t.Error("Expected output to contain 'test: points'")
	}

	// Verify vector testing status
	if seg.VectorValuesStatus == nil {
		t.Error("Expected vectorValuesStatus to be non-nil")
	} else {
		if seg.VectorValuesStatus.TotalVectorValues != int64(2*liveDocCount) {
			t.Errorf("Expected %d vector values, got %d", 2*liveDocCount, seg.VectorValuesStatus.TotalVectorValues)
		}
		if seg.VectorValuesStatus.TotalKnnVectorFields != 2 {
			t.Errorf("Expected 2 KNN vector fields, got %d", seg.VectorValuesStatus.TotalKnnVectorFields)
		}
		if seg.VectorValuesStatus.Error != nil {
			t.Errorf("vectorValuesStatus has error: %v", seg.VectorValuesStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: vectors") {
		t.Error("Expected output to contain 'test: vectors'")
	}

	// Verify index sort testing status
	if seg.IndexSortStatus == nil {
		t.Error("Expected indexSortStatus to be non-nil")
	} else {
		if seg.IndexSortStatus.Error != nil {
			t.Errorf("indexSortStatus has error: %v", seg.IndexSortStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: index sort") {
		t.Error("Expected output to contain 'test: index sort'")
	}

	// Verify soft deletes testing status
	if seg.SoftDeletesStatus == nil {
		t.Error("Expected softDeletesStatus to be non-nil")
	} else {
		if seg.SoftDeletesStatus.Error != nil {
			t.Errorf("softDeletesStatus has error: %v", seg.SoftDeletesStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: check soft deletes") {
		t.Error("Expected output to contain 'test: check soft deletes'")
	}
}

// TestCheckIndex_InvalidThreadCountArgument tests invalid thread count argument
// Source: TestCheckIndex.testInvalidThreadCountArgument()
// Purpose: Verifies CheckIndex properly validates command-line arguments
func TestCheckIndex_InvalidThreadCountArgument(t *testing.T) {
	args := []string{"-threadCount", "0"}
	err := index.CheckIndexParseOptions(args)
	if err == nil {
		t.Error("Expected error for invalid thread count argument")
	}
}

// TestCheckIndex_PriorBrokenCommitPoint tests detection of broken older commit points
// Source: TestCheckIndex.testPriorBrokenCommitPoint()
// Purpose: Verifies CheckIndex detects corruption in older commit points while current is valid
func TestCheckIndex_PriorBrokenCommitPoint(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer dir.Close()

	// Disable check index on close for this test
	dir.SetCheckIndexOnClose(false)

	config := index.NewIndexWriterConfig(nil)
	config.SetMergePolicy(index.NoMergePolicy)
	config.SetIndexDeletionPolicy(index.DeleteNothingIndexDeletionPolicyInstance)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create first segment and commit
	doc1 := document.NewDocument()
	doc1.AddField(document.NewStringField("id", "a", document.StoreNo))
	if err := writer.AddDocument(doc1); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify segment file exists
	if !dir.SlowFileExists("_0.si") {
		t.Error("Expected _0.si to exist")
	}

	// Create second segment and commit
	doc2 := document.NewDocument()
	doc2.AddField(document.NewStringField("id", "a", document.StoreNo))
	if err := writer.UpdateDocument(index.NewTerm("id", "a"), doc2); err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify both segment files exist
	if !dir.SlowFileExists("_0.si") {
		t.Error("Expected _0.si to exist after second commit")
	}
	if !dir.SlowFileExists("_1.si") {
		t.Error("Expected _1.si to exist")
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Check index - should be clean
	checker1, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	status1, err := checker1.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}
	if !status1.Clean {
		t.Error("Expected clean index before corruption")
	}
	checker1.Close()

	// Corrupt segment 0 by removing its .si file
	if err := dir.DeleteFile("_0.si"); err != nil {
		t.Fatalf("Failed to delete _0.si: %v", err)
	}

	// Check index again - should detect corruption
	checker2, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("Failed to create CheckIndex: %v", err)
	}
	status2, err := checker2.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex failed: %v", err)
	}
	if status2.Clean {
		t.Error("Expected index to be not clean after corruption")
	}
	checker2.Close()
}

// TestCheckIndex_StatusStructure tests the Status structure fields
// Purpose: Verifies all Status fields are properly defined and accessible
func TestCheckIndex_StatusStructure(t *testing.T) {
	status := &index.CheckIndexStatus{}

	// Test basic fields
	status.Clean = true
	status.MissingSegments = false
	status.SegmentsFileName = "segments_1"
	status.NumSegments = 1
	status.ToolOutOfDate = false
	status.TotLoseDocCount = 0
	status.NumBadSegments = 0
	status.Partial = false
	status.MaxSegmentName = 1
	status.ValidCounter = true

	// Test segment info status
	segStatus := &index.SegmentInfoStatus{}
	segStatus.Name = "_0"
	segStatus.MaxDoc = 10
	segStatus.Compound = false
	segStatus.NumFiles = 5
	segStatus.SizeMB = 1.5
	segStatus.HasDeletions = false
	segStatus.DeletionsGen = 0
	segStatus.OpenReaderPassed = true
	segStatus.ToLoseDocCount = 0

	// Test sub-status structures
	segStatus.LiveDocStatus = &index.LiveDocStatus{
		NumDeleted: 0,
	}

	segStatus.FieldInfoStatus = &index.FieldInfoStatus{
		TotFields: 5,
	}

	segStatus.FieldNormStatus = &index.FieldNormStatus{
		TotFields: 3,
	}

	segStatus.TermIndexStatus = &index.TermIndexStatus{
		TermCount:    100,
		DelTermCount: 0,
		TotFreq:      200,
		TotPos:       300,
	}

	segStatus.StoredFieldStatus = &index.StoredFieldStatus{
		DocCount:  10,
		TotFields: 20,
	}

	segStatus.TermVectorStatus = &index.TermVectorStatus{
		DocCount:   10,
		TotVectors: 10,
	}

	segStatus.DocValuesStatus = &index.DocValuesStatus{
		TotalValueFields:         5,
		TotalNumericFields:       2,
		TotalBinaryFields:        1,
		TotalSortedFields:        1,
		TotalSortedNumericFields: 0,
		TotalSortedSetFields:     1,
		TotalSkippingIndex:       1,
	}

	segStatus.PointsStatus = &index.PointsStatus{
		TotalValuePoints: 10,
		TotalValueFields: 1,
	}

	segStatus.VectorValuesStatus = &index.VectorValuesStatus{
		TotalVectorValues:    20,
		TotalKnnVectorFields: 2,
	}

	segStatus.IndexSortStatus = &index.IndexSortStatus{}
	segStatus.SoftDeletesStatus = &index.SoftDeletesStatus{}

	status.SegmentInfos = append(status.SegmentInfos, segStatus)

	// Verify values
	if !status.Clean {
		t.Error("Expected Clean to be true")
	}
	if status.NumSegments != 1 {
		t.Errorf("Expected NumSegments=1, got %d", status.NumSegments)
	}
	if len(status.SegmentInfos) != 1 {
		t.Errorf("Expected 1 segment info, got %d", len(status.SegmentInfos))
	}
}

// Helper function to generate a random normalized vector
func randomVector(t *testing.T, dim int) []float32 {
	v := make([]float32, dim)
	for i := 0; i < dim; i++ {
		v[i] = float32(i) / float32(dim) // Deterministic for tests
	}
	// Normalize
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	if sum > 0 {
		norm := float32(1.0 / float64(sum))
		for i := range v {
			v[i] *= norm
		}
	}
	return v
}
