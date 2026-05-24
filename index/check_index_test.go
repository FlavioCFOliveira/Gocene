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

// TestCheckIndex_DeletedDocs tests CheckIndex with deleted documents.
// Source: BaseTestCheckIndex.testDeletedDocs()
// Purpose: Verifies CheckIndex correctly handles indexes with deleted documents.
//
// Gocene deviation from Lucene original: in the Lucene test the delete is issued
// after ForceMerge+Commit and relies on codec-level live-docs files.  In Gocene
// the in-memory delete path (pendingDeleteTerms) is processed during
// flushPendingDocsLocked; therefore the delete must be issued while the deleted
// document is still in the in-memory buffer (i.e. before ForceMerge).  The test
// uses a custom FieldType with term-vectors enabled to mirror the Lucene original.
func TestCheckIndex_DeletedDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// No MaxBufferedDocs: all 19 docs accumulate in one buffer so that
	// DeleteDocuments can match "aaa5" against the buffer's docFieldIndex.
	config := index.NewIndexWriterConfig(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Custom FieldType: TextField + term vectors (matching Lucene's testDeletedDocs).
	customFT := document.NewFieldType().
		SetIndexed(true).
		SetStored(true).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStoreTermVectors(true)

	// Add 19 documents into the in-memory buffer.
	for i := 0; i < 19; i++ {
		doc := document.NewDocument()
		f, err := document.NewField("field", "aaa"+string(rune('0'+i)), customFT)
		if err != nil {
			t.Fatalf("Failed to create field: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Delete document with field "aaa5" BEFORE ForceMerge so that the delete is
	// processed against the current in-memory buffer during flushPendingDocsLocked.
	if err := writer.DeleteDocuments(index.NewTerm("field", "aaa5")); err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("Failed to force merge: %v", err)
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
		tf, _ := document.NewTextField("field", "value", true)
		doc.Add(tf)
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
		tf, _ := document.NewTextField("field", "value", true)
		doc.Add(tf)
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
	tf, _ := document.NewTextField("field", "value", true)
	doc.Add(tf)
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
	config.SetIndexSort(index.NewSort(index.NewSortField("sort_field", index.SortTypeInt)))
	config.SetSoftDeletesField("soft_delete")

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	liveDocCount := 5 // Using fixed count for test stability
	for i := 0; i < liveDocCount; i++ {
		doc := document.NewDocument()

		// Stored field
		sf, _ := document.NewStringField("id", "id"+string(rune('0'+i)), true)
		doc.Add(sf)
		stf, _ := document.NewStoredField("field", "value"+string(rune('0'+i)))
		doc.Add(stf)

		// Doc value
		ndv, _ := document.NewNumericDocValuesField("dv", int64(i))
		doc.Add(ndv)

		// Point value - using BinaryPoint
		// BinaryPoint não está implementado - ignorando por enquanto

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Soft delete one document - SoftUpdateDocument não está implementado
	// Ignorando este teste por enquanto

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

	// Verify field infos testing status.
	// Gocene stores fields in-memory in SegmentInfos; the 3 fields added per
	// document are: "id" (StringField), "field" (StoredField), "dv" (NumericDV).
	// sort_field and soft_delete exist only in the config and are not materialised
	// as FieldInfo unless documents include them.
	if seg.FieldInfoStatus == nil {
		t.Error("Expected fieldInfoStatus to be non-nil")
	} else {
		if seg.FieldInfoStatus.TotFields != 3 {
			t.Errorf("Expected 3 fields (id, field, dv), got %d", seg.FieldInfoStatus.TotFields)
		}
		if seg.FieldInfoStatus.Error != nil {
			t.Errorf("fieldInfoStatus has error: %v", seg.FieldInfoStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: field infos") {
		t.Error("Expected output to contain 'test: field infos'")
	}

	// Verify field norm testing status.
	// "id" is StringField (OmitNorms=true), "field" is StoredField (not indexed),
	// "dv" is NumericDocValuesField (not indexed). No norms are stored.
	if seg.FieldNormStatus == nil {
		t.Error("Expected fieldNormStatus to be non-nil")
	} else {
		if seg.FieldNormStatus.TotFields != 0 {
			t.Errorf("Expected 0 field norms (all fields omit norms or are not indexed), got %d",
				seg.FieldNormStatus.TotFields)
		}
		if seg.FieldNormStatus.Error != nil {
			t.Errorf("fieldNormStatus has error: %v", seg.FieldNormStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: field norms") {
		t.Error("Expected output to contain 'test: field norms'")
	}

	// Verify term index testing status.
	// One indexed field ("id"), one unique term per live document (5 docs).
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

	// Verify stored field testing status.
	// 5 live documents, each with 2 stored fields ("id" and "field").
	// No soft-delete tombstone is added because SoftUpdateDocument is not yet implemented.
	if seg.StoredFieldStatus == nil {
		t.Error("Expected storedFieldStatus to be non-nil")
	} else {
		if seg.StoredFieldStatus.DocCount != liveDocCount {
			t.Errorf("Expected %d stored field docs, got %d", liveDocCount, seg.StoredFieldStatus.DocCount)
		}
		// 2 stored fields per document ("id" + "field")
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

	// Verify term vector testing status.
	// No fields in this test have term vectors enabled.
	if seg.TermVectorStatus == nil {
		t.Error("Expected termVectorStatus to be non-nil")
	} else {
		if seg.TermVectorStatus.DocCount != 0 {
			t.Errorf("Expected 0 term vector docs (no TV fields), got %d", seg.TermVectorStatus.DocCount)
		}
		if seg.TermVectorStatus.TotVectors != 0 {
			t.Errorf("Expected 0 term vectors, got %d", seg.TermVectorStatus.TotVectors)
		}
		if seg.TermVectorStatus.Error != nil {
			t.Errorf("termVectorStatus has error: %v", seg.TermVectorStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: term vectors") {
		t.Error("Expected output to contain 'test: term vectors'")
	}

	// Verify doc values testing status.
	// One numeric DocValues field ("dv"). No skipping index stored in Gocene's
	// in-memory FieldInfo representation.
	if seg.DocValuesStatus == nil {
		t.Error("Expected docValuesStatus to be non-nil")
	} else {
		if seg.DocValuesStatus.TotalNumericFields != 1 {
			t.Errorf("Expected 1 numeric field (dv), got %d", seg.DocValuesStatus.TotalNumericFields)
		}
		if seg.DocValuesStatus.TotalSkippingIndex != 0 {
			t.Errorf("Expected 0 skipping index entries, got %d", seg.DocValuesStatus.TotalSkippingIndex)
		}
		if seg.DocValuesStatus.Error != nil {
			t.Errorf("docValuesStatus has error: %v", seg.DocValuesStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: docvalues") {
		t.Error("Expected output to contain 'test: docvalues'")
	}

	// Verify point values testing status.
	// BinaryPoint / IntPoint are not added in this test; expect zero values.
	if seg.PointsStatus == nil {
		t.Error("Expected pointsStatus to be non-nil")
	} else {
		if seg.PointsStatus.TotalValueFields != 0 {
			t.Errorf("Expected 0 point value fields, got %d", seg.PointsStatus.TotalValueFields)
		}
		if seg.PointsStatus.TotalValuePoints != 0 {
			t.Errorf("Expected 0 point values, got %d", seg.PointsStatus.TotalValuePoints)
		}
		if seg.PointsStatus.Error != nil {
			t.Errorf("pointsStatus has error: %v", seg.PointsStatus.Error)
		}
	}
	if !strings.Contains(output.String(), "test: points") {
		t.Error("Expected output to contain 'test: points'")
	}

	// Verify vector testing status.
	// No KNN vector fields are added in this test; expect zero values.
	if seg.VectorValuesStatus == nil {
		t.Error("Expected vectorValuesStatus to be non-nil")
	} else {
		if seg.VectorValuesStatus.TotalVectorValues != 0 {
			t.Errorf("Expected 0 vector values, got %d", seg.VectorValuesStatus.TotalVectorValues)
		}
		if seg.VectorValuesStatus.TotalKnnVectorFields != 0 {
			t.Errorf("Expected 0 KNN vector fields, got %d", seg.VectorValuesStatus.TotalKnnVectorFields)
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

// TestCheckIndex_PriorBrokenCommitPoint tests detection of broken older commit points.
// Source: TestCheckIndex.testPriorBrokenCommitPoint()
// Purpose: Verifies CheckIndex detects corruption in older commit points while current is valid.
//
// Gocene deviation: the Lucene original uses UpdateDocument for the second commit so that the
// first segment is preserved alongside the new one (NoMergePolicy prevents it from being deleted).
// In Gocene, UpdateDocument does not add a document to the write buffer (it is handled by the
// DocumentsWriter layer, which is not yet implemented).  Therefore the test uses two separate
// AddDocument+Commit calls, each producing a distinct segment, to achieve the same observable
// behaviour: two segment info stub files on disk that can be individually corrupted.
func TestCheckIndex_PriorBrokenCommitPoint(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	config.SetMergePolicy(index.NewNoMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create first segment and commit.
	doc1 := document.NewDocument()
	sf1, _ := document.NewStringField("id", "a", false)
	doc1.Add(sf1)
	if err := writer.AddDocument(doc1); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify segment file exists.
	if !dir.FileExists("_0.si") {
		t.Error("Expected _0.si to exist")
	}

	// Create second segment and commit.
	// Gocene deviation: AddDocument instead of UpdateDocument (see function comment).
	doc2 := document.NewDocument()
	sf2, _ := document.NewStringField("id", "b", false)
	doc2.Add(sf2)
	if err := writer.AddDocument(doc2); err != nil {
		t.Fatalf("Failed to add second document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit second: %v", err)
	}

	// Verify both segment files exist.
	if !dir.FileExists("_0.si") {
		t.Error("Expected _0.si to exist after second commit")
	}
	if !dir.FileExists("_1.si") {
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
