// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterReader.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterReader.java
//
// GOC-4139: Port TestIndexWriterReader (Sprint 55, option c).
//
// All 25 test methods from the Java source are structured here. Methods that
// depend on infrastructure not yet ported are marked with t.Skip and an
// explicit reason; the remainder run against the current implementation.
//
// Missing infrastructure (drives the t.Skip calls below):
//   - IndexWriter.GetReader / DirectoryReader.open(writer): NRT reader directly
//     from the writer, exposing uncommitted changes.
//   - DirectoryReader.openIfChanged(reader[, writer|commit]): incremental reopen.
//   - IndexWriter.DeleteDocuments / UpdateDocument delete-term: currently no-op
//     stubs, so deletes and updates are not applied to the index.
//   - RandomIndexWriter, MockDirectoryWrapper, MockAnalyzer test fixtures.
//   - Merged-segment warmers (MergedSegmentWarmer, SimpleMergedSegmentWarmer).
//   - IndexWriterConfig.setLeafSorter and FilterDirectoryReader leaf ordering.
//   - DirectoryReader leaf APIs through OpenDirectoryReader (core readers nil).
package index_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// testAddCloseOpen ports testAddCloseOpen().
// Java repeatedly pulls an NRT reader from the writer mid-mutation and asserts
// isCurrent() transitions. Needs DirectoryReader.open(writer) and updateDocument
// / deleteDocuments to be applied.
func TestIndexWriterReader_AddCloseOpen(t *testing.T) {
	t.Skip("needs NRT DirectoryReader.open(writer); deleteDocuments/updateDocument are no-op stubs")
}

// testUpdateDocument ports testUpdateDocument().
// Java verifies an updated document replaces the old one and is visible via an
// NRT reader. UpdateDocument's delete-term half is currently not applied.
func TestIndexWriterReader_UpdateDocument(t *testing.T) {
	t.Skip("IndexWriter.UpdateDocument does not apply the delete term; NRT reader unavailable")
}

// testIsCurrent ports testIsCurrent().
// The committed-index portion is exercised here; the NRT portion (open(writer),
// maxDoc on an uncommitted reader) is not, as it needs NRT support.
func TestIndexWriterReader_IsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build and commit an initial single-document index.
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.AddDocument(createTestDoc(1, "test", 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A reader opened on the committed index must report itself as current.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	isCurrent, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent: %v", err)
	}
	if !isCurrent {
		t.Error("expected reader to be current on a freshly committed index")
	}

	// Reopen the writer and append a committed document: the old reader, which
	// was opened before that commit, must no longer be current.
	config2 := index.NewIndexWriterConfig(createTestAnalyzer())
	config2.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter (append): %v", err)
	}
	if err := writer2.AddDocument(createTestDoc(2, "test", 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer2.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	isCurrent, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent after commit: %v", err)
	}
	if isCurrent {
		t.Error("expected reader to not be current after a new commit")
	}
}

// testAddIndexes ports testAddIndexes().
// Builds two on-disk indexes and merges one into the other via AddIndexes,
// then verifies the document count on a committed reader.
func TestIndexWriterReader_AddIndexes(t *testing.T) {
	sourceDir := store.NewByteBuffersDirectory()
	defer sourceDir.Close()

	sourceWriter, err := index.NewIndexWriter(sourceDir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (source): %v", err)
	}
	for i := 0; i < 100; i++ {
		if err := sourceWriter.AddDocument(createTestDoc(i, "index2", 4)); err != nil {
			t.Fatalf("AddDocument (source) %d: %v", i, err)
		}
	}
	if err := sourceWriter.Commit(); err != nil {
		t.Fatalf("Commit (source): %v", err)
	}
	if err := sourceWriter.Close(); err != nil {
		t.Fatalf("Close (source): %v", err)
	}

	targetDir := store.NewByteBuffersDirectory()
	defer targetDir.Close()

	targetWriter, err := index.NewIndexWriter(targetDir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (target): %v", err)
	}
	for i := 0; i < 100; i++ {
		if err := targetWriter.AddDocument(createTestDoc(i, "index1", 4)); err != nil {
			t.Fatalf("AddDocument (target) %d: %v", i, err)
		}
	}
	if err := targetWriter.AddIndexes(sourceDir); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	if err := targetWriter.Commit(); err != nil {
		t.Fatalf("Commit (target): %v", err)
	}
	if err := targetWriter.Close(); err != nil {
		t.Fatalf("Close (target): %v", err)
	}

	reader, err := index.OpenDirectoryReader(targetDir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != 200 {
		t.Errorf("NumDocs after AddIndexes = %d, want 200", got)
	}
}

// testAddIndexes2 ports testAddIndexes2().
// Adds the same source index five times and verifies the cumulative count.
func TestIndexWriterReader_AddIndexes2(t *testing.T) {
	sourceDir := store.NewByteBuffersDirectory()
	defer sourceDir.Close()

	sourceWriter, err := index.NewIndexWriter(sourceDir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (source): %v", err)
	}
	for i := 0; i < 100; i++ {
		if err := sourceWriter.AddDocument(createTestDoc(i, "index2", 4)); err != nil {
			t.Fatalf("AddDocument (source) %d: %v", i, err)
		}
	}
	if err := sourceWriter.Commit(); err != nil {
		t.Fatalf("Commit (source): %v", err)
	}
	if err := sourceWriter.Close(); err != nil {
		t.Fatalf("Close (source): %v", err)
	}

	targetDir := store.NewByteBuffersDirectory()
	defer targetDir.Close()

	targetWriter, err := index.NewIndexWriter(targetDir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (target): %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := targetWriter.AddIndexes(sourceDir); err != nil {
			t.Fatalf("AddIndexes iteration %d: %v", i, err)
		}
	}
	if err := targetWriter.Commit(); err != nil {
		t.Fatalf("Commit (target): %v", err)
	}
	if err := targetWriter.Close(); err != nil {
		t.Fatalf("Close (target): %v", err)
	}

	reader, err := index.OpenDirectoryReader(targetDir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != 500 {
		t.Errorf("NumDocs after 5x AddIndexes = %d, want 500", got)
	}
}

// testDeleteFromIndexWriter ports testDeleteFromIndexWriter().
// Java deletes by term and by query and checks visibility through NRT readers.
func TestIndexWriterReader_DeleteFromIndexWriter(t *testing.T) {
	t.Skip("IndexWriter.DeleteDocuments is a no-op stub; deletes are not applied")
}

// testAddIndexesAndDoDeletesThreads ports testAddIndexesAndDoDeletesThreads().
// Stress test combining concurrent addIndexes and deletes; needs the
// AddDirectoriesThreads harness, applied deletes and TestUtil.checkIndex.
func TestIndexWriterReader_AddIndexesAndDoDeletesThreads(t *testing.T) {
	t.Skip("needs AddDirectoriesThreads harness and applied deletes")
}

// testIndexWriterReopenSegmentFullMerge ports testIndexWriterReopenSegmentFullMerge().
func TestIndexWriterReader_IndexWriterReopenSegmentFullMerge(t *testing.T) {
	t.Skip("needs NRT DirectoryReader.open(writer) to observe pre-commit segments")
}

// testIndexWriterReopenSegment ports testIndexWriterReopenSegment().
func TestIndexWriterReader_IndexWriterReopenSegment(t *testing.T) {
	t.Skip("needs NRT DirectoryReader.open(writer) to observe pre-commit segments")
}

// testMergeWarmer ports testMergeWarmer().
// Verifies the merged-segment warmer callback fires; warmers are not ported.
func TestIndexWriterReader_MergeWarmer(t *testing.T) {
	t.Skip("MergedSegmentWarmer is not implemented")
}

// testAfterCommit ports testAfterCommit().
// Java uses an NRT reader plus openIfChanged across commits. The committed
// portion is covered here without the NRT reopen.
func TestIndexWriterReader_AfterCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	for i := 0; i < 100; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 4)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if got := reader.NumDocs(); got != 100 {
		t.Errorf("NumDocs = %d, want 100", got)
	}
	reader.Close()

	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 4)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (reopen): %v", err)
	}
	defer reader2.Close()
	if got := reader2.NumDocs(); got != 110 {
		t.Errorf("NumDocs after second commit = %d, want 110", got)
	}
}

// testAfterClose ports testAfterClose().
// Java pulls an NRT reader, closes the writer, and confirms the reader stays
// usable. Here the reader is opened on the committed index instead.
func TestIndexWriterReader_AfterClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 100; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 4)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	// Closing the writer must not invalidate an already-open reader.
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := reader.NumDocs(); got != 100 {
		t.Errorf("NumDocs after writer close = %d, want 100", got)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// testDuringAddIndexes ports testDuringAddIndexes() (a @Nightly stress test).
func TestIndexWriterReader_DuringAddIndexes(t *testing.T) {
	t.Skip("nightly stress test; needs NRT openIfChanged and MockDirectoryWrapper")
}

// testDuringAddDelete ports testDuringAddDelete().
// Concurrent add/delete stress with NRT reopen. The reader-side reopen and
// applied deletes are unavailable; concurrent appends are covered separately
// by TestIndexWriterReader_ConcurrentAccess.
func TestIndexWriterReader_DuringAddDelete(t *testing.T) {
	t.Skip("needs NRT openIfChanged and applied deletes")
}

// testForceMergeDeletes ports testForceMergeDeletes().
// Java deletes a document then forceMergeDeletes() to physically drop it.
func TestIndexWriterReader_ForceMergeDeletes(t *testing.T) {
	t.Skip("DeleteDocuments is a no-op stub and IndexWriter.ForceMergeDeletes is not implemented")
}

// testDeletesNumDocs ports testDeletesNumDocs().
// Java checks numDocs shrinks as documents are deleted.
func TestIndexWriterReader_DeletesNumDocs(t *testing.T) {
	t.Skip("IndexWriter.DeleteDocuments is a no-op stub; numDocs cannot reflect deletes")
}

// testEmptyIndex ports testEmptyIndex().
// Ensures a reader can be opened on an empty, just-committed index.
func TestIndexWriterReader_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Java pulls an NRT reader pre-commit; without NRT support we commit first.
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader on empty index: %v", err)
	}
	if got := reader.NumDocs(); got != 0 {
		t.Errorf("NumDocs on empty index = %d, want 0", got)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}
}

// testSegmentWarmer ports testSegmentWarmer().
func TestIndexWriterReader_SegmentWarmer(t *testing.T) {
	t.Skip("MergedSegmentWarmer and reader pooling are not implemented")
}

// testSimpleMergedSegmentWarmer ports testSimpleMergedSegmentWarmer().
func TestIndexWriterReader_SimpleMergedSegmentWarmer(t *testing.T) {
	t.Skip("SimpleMergedSegmentWarmer is not implemented")
}

// testReopenAfterNoRealChange ports testReopenAfterNoRealChange().
// Java relies on openIfChanged returning nil when nothing changed.
func TestIndexWriterReader_ReopenAfterNoRealChange(t *testing.T) {
	t.Skip("needs DirectoryReader.openIfChanged and NRT reader pooling")
}

// testNRTOpenExceptions ports testNRTOpenExceptions().
// Java injects FakeIOException via MockDirectoryWrapper while opening NRT
// readers and checks no file handles leak.
func TestIndexWriterReader_NRTOpenExceptions(t *testing.T) {
	t.Skip("needs MockDirectoryWrapper failure injection and NRT DirectoryReader.open(writer)")
}

// testTooManySegments ports testTooManySegments().
// Java opens an NRT reader after each add and asserts the merge policy keeps
// the leaf count bounded.
func TestIndexWriterReader_TooManySegments(t *testing.T) {
	t.Skip("needs NRT DirectoryReader.open(writer) and reader.leaves()")
}

// testReopenNRTReaderOnCommit ports testReopenNRTReaderOnCommit().
// Java verifies SegmentReader instances are shared when reopening an NRT
// reader against a commit point.
func TestIndexWriterReader_ReopenNRTReaderOnCommit(t *testing.T) {
	t.Skip("needs NRT openIfChanged against a commit and SegmentReader sharing")
}

// testIndexReaderWriterWithLeafSorter ports testIndexReaderWriterWithLeafSorter().
// Java configures IndexWriterConfig.setLeafSorter and checks leaf ordering.
func TestIndexWriterReader_IndexReaderWriterWithLeafSorter(t *testing.T) {
	t.Skip("IndexWriterConfig.setLeafSorter and leaf ordering are not implemented")
}

// --- Additional coverage retained from the pre-existing port ----------------
// These tests are not 1:1 with a Java method but exercise committed-index
// reader behaviour that the implementation currently supports.

// TestIndexWriterReader_BasicNRT covers committed-index reader visibility.
func TestIndexWriterReader_BasicNRT(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != 10 {
		t.Errorf("NumDocs = %d, want 10", got)
	}
}

// TestIndexWriterReader_Reopen covers re-opening a reader across commits.
func TestIndexWriterReader_Reopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if got := reader.NumDocs(); got != 10 {
		t.Errorf("NumDocs = %d, want 10", got)
	}
	reader.Close()

	for i := 10; i < 20; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (reopen): %v", err)
	}
	defer reader2.Close()
	if got := reader2.NumDocs(); got != 20 {
		t.Errorf("NumDocs after reopen = %d, want 20", got)
	}
}

// TestIndexWriterReader_ConcurrentAccess exercises concurrent appends followed
// by a single commit, then verifies the document count.
func TestIndexWriterReader_ConcurrentAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(createTestDoc(i, "test", 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	const numGoroutines = 3
	const iterations = 5
	var wg sync.WaitGroup
	var addErr atomic.Pointer[error]

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if err := writer.AddDocument(createTestDoc(1000*id+j, "concurrent", 2)); err != nil {
					addErr.Store(&err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	if errPtr := addErr.Load(); errPtr != nil {
		t.Fatalf("concurrent AddDocument: %v", *errPtr)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	want := 10 + numGoroutines*iterations
	if got := reader.NumDocs(); got != want {
		t.Errorf("NumDocs after concurrent adds = %d, want %d", got, want)
	}
}

// createTestDoc builds a test document, mirroring DocHelper.createDocument:
// an "id" field carrying the full integer, an "indexname" field, and numFields
// text fields.
func createTestDoc(id int, indexName string, numFields int) *document.Document {
	doc := &document.Document{}

	idField, _ := document.NewStringField("id", strconv.Itoa(id), true)
	doc.Add(idField)

	indexField, _ := document.NewStringField("indexname", indexName, true)
	doc.Add(indexField)

	for i := 0; i < numFields; i++ {
		fieldName := "field" + strconv.Itoa(i+1)
		fieldValue := "value" + strconv.Itoa(id) + " " + indexName
		field, _ := document.NewTextField(fieldName, fieldValue, false)
		doc.Add(field)
	}

	return doc
}
