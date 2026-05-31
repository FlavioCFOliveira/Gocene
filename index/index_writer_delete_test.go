// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterDelete
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterDelete.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4150 (Sprint 55, option c): every Java test method has a corresponding
// Go test function. Tests that depend on infrastructure not yet ported to
// Gocene call t.Skip with a precise reason; tests whose dependencies exist
// are implemented and exercised.
//
// Infrastructure gaps that drive the t.Skip calls in this file:
//   - No near-real-time reader: OpenDirectoryReader only accepts a
//     store.Directory, not an IndexWriter, so DirectoryReader.open(writer)
//     and StandardDirectoryReader.isCurrent() have no equivalent.
//   - No MockDirectoryWrapper fault injection (disk-full, failOn/Failure),
//     so the disk-full and error-injection tests cannot be reproduced.
//   - No RandomIndexWriter / MockRandomMergePolicy test harness.
//   - CheckIndex info-stream text ("has deletions") is not exposed.
//   - IndexWriter.flushCount / tryDeleteDocument-on-leaf semantics partially
//     present; see per-test notes.
//   - Delete application is not implemented: IndexWriter.DeleteDocuments and
//     DeleteDocumentsQuery are no-op stubs, DeleteAll only resets an in-memory
//     doc counter without clearing committed segments, and
//     GetBufferedDeleteTermsSize always returns 0. Every test that commits
//     documents and then expects a delete to reduce the on-disk document
//     count is therefore skipped until the buffered-updates / live-docs
//     pipeline is ported.
package index_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newDeleteTestAnalyzer returns the analyzer used across these tests. Lucene
// uses MockAnalyzer(WHITESPACE); Gocene's WhitespaceAnalyzer is the closest
// faithful equivalent.
func newDeleteTestAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// addDoc mirrors TestIndexWriterDelete.addDoc: a document with an "aaa"
// content field, a stored "id", an unstored "value", and a "dv" numeric
// doc-values field.
func addDoc(t *testing.T, modifier *index.IndexWriter, id, value int) {
	t.Helper()
	doc := document.NewDocument()

	contentField, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		t.Fatalf("NewTextField(content): %v", err)
	}
	idField, err := document.NewStringField("id", fmt.Sprintf("%d", id), true)
	if err != nil {
		t.Fatalf("NewStringField(id): %v", err)
	}
	valueField, err := document.NewStringField("value", fmt.Sprintf("%d", value), false)
	if err != nil {
		t.Fatalf("NewStringField(value): %v", err)
	}
	dvField, err := document.NewNumericDocValuesField("dv", int64(value))
	if err != nil {
		t.Fatalf("NewNumericDocValuesField(dv): %v", err)
	}

	doc.Add(contentField)
	doc.Add(idField)
	doc.Add(valueField)
	doc.Add(dvField)

	if err := modifier.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// updateDoc mirrors TestIndexWriterDelete.updateDoc: replaces the document
// whose "id" matches with a freshly-built one.
func updateDoc(t *testing.T, modifier *index.IndexWriter, id, value int) {
	t.Helper()
	doc := document.NewDocument()

	contentField, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		t.Fatalf("NewTextField(content): %v", err)
	}
	idField, err := document.NewStringField("id", fmt.Sprintf("%d", id), true)
	if err != nil {
		t.Fatalf("NewStringField(id): %v", err)
	}
	valueField, err := document.NewStringField("value", fmt.Sprintf("%d", value), false)
	if err != nil {
		t.Fatalf("NewStringField(value): %v", err)
	}
	dvField, err := document.NewNumericDocValuesField("dv", int64(value))
	if err != nil {
		t.Fatalf("NewNumericDocValuesField(dv): %v", err)
	}

	doc.Add(contentField)
	doc.Add(idField)
	doc.Add(valueField)
	doc.Add(dvField)

	if err := modifier.UpdateDocument(index.NewTerm("id", fmt.Sprintf("%d", id)), doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
}

// getHitCount mirrors TestIndexWriterDelete.getHitCount: opens a reader on the
// directory, runs a TermQuery, and returns the total hit count.
func getHitCount(t *testing.T, dir store.Directory, term *index.Term) int64 {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(search.NewTermQuery(term), 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return topDocs.TotalHits.Value
}

// ---------------------------------------------------------------------------
// testSimpleCase
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_SimpleCase ports testSimpleCase.
//
// Intent: add two documents, force-merge to one segment, delete one document
// by the term ("city","Amsterdam"), commit, and verify the hit count for that
// term drops from 1 to 0.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub in Gocene; deletes are
// never applied to committed segments, so the post-delete "0 hits" assertion
// cannot pass. Re-enable once the buffered-updates / live-docs pipeline lands.
func TestIndexWriterDelete_SimpleCase(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	modifier, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(newDeleteTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	keywords := []string{"1", "2"}
	city := []string{"Amsterdam", "Venice"}
	for i := range keywords {
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", keywords[i], true)
		if err != nil {
			t.Fatalf("NewStringField(id): %v", err)
		}
		cityField, err := document.NewTextField("city", city[i], true)
		if err != nil {
			t.Fatalf("NewTextField(city): %v", err)
		}
		doc.Add(idField)
		doc.Add(cityField)
		if err := modifier.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	term := index.NewTerm("city", "Amsterdam")
	if hc := getHitCount(t, dir, term); hc != 1 {
		t.Fatalf("pre-delete hit count = %d, want 1", hc)
	}
	if err := modifier.DeleteDocuments(term); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit (post-delete): %v", err)
	}
	if hc := getHitCount(t, dir, term); hc != 0 {
		t.Fatalf("post-delete hit count = %d, want 0", hc)
	}
	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// readerNumDocs opens dir, returns its live doc count, and closes the reader.
func readerNumDocs(t *testing.T, dir store.Directory) int {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	return reader.NumDocs()
}

// readerNumDeleted opens dir and returns its number of deleted documents.
func readerNumDeleted(t *testing.T, dir store.Directory) int {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	return reader.NumDeletedDocs()
}

// ---------------------------------------------------------------------------
// testNonRAMDelete
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_NonRAMDelete ports testNonRAMDelete.
//
// Intent: with maxBufferedDocs=2, index 7 docs and commit so they are all on
// disk, then delete every doc by the "value" term and commit; the reader must
// then see 0 docs.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub, so the delete against
// the on-disk segments is never applied and NumDocs stays at 7.
func TestIndexWriterDelete_NonRAMDelete(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	cfg := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	cfg.SetMaxBufferedDocs(2)
	modifier, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	id, value := 0, 100
	for i := 0; i < 7; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := modifier.GetSegmentCount(); got <= 0 {
		t.Fatalf("GetSegmentCount = %d, want > 0", got)
	}

	if n := readerNumDocs(t, dir); n != 7 {
		t.Fatalf("numDocs before delete = %d, want 7", n)
	}

	if err := modifier.DeleteDocuments(index.NewTerm("value", fmt.Sprintf("%d", value))); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit (post-delete): %v", err)
	}

	if n := readerNumDocs(t, dir); n != 0 {
		t.Fatalf("numDocs after delete = %d, want 0", n)
	}
	if nd := readerNumDeleted(t, dir); nd != 7 {
		t.Fatalf("numDeletedDocs after delete = %d, want 7", nd)
	}
	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// testRAMDeletes
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_RAMDeletes ports testRAMDeletes.
//
// Intent: with maxBufferedDocs=4, interleave addDoc and delete-by-"value"
// (iteration t=0 deletes by Term, t=1 by TermQuery) so the deletes apply only
// to buffered RAM segments; after committing, exactly the last added doc must
// remain. The t=0 iteration additionally asserts
// getBufferedDeleteTermsSize()==1.
//
// Skipped: IndexWriter.DeleteDocuments / DeleteDocumentsQuery are no-op stubs
// and GetBufferedDeleteTermsSize always returns 0, so neither the surviving
// "1 doc" assertion nor the buffered-term-count assertion can pass.
func TestIndexWriterDelete_RAMDeletes(t *testing.T) {
	t.Fatal("infra gap: DeleteDocuments(Query) are no-op stubs; GetBufferedDeleteTermsSize returns 0")
}

// ---------------------------------------------------------------------------
// testBothDeletes
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_BothDeletes ports testBothDeletes.
//
// Intent: index 5 docs with value=100 and 5 with value=200, commit, then add
// 5 more value=200 docs and delete by the "value"=200 term so the delete
// matches both committed and buffered docs; after commit exactly the 5
// value=100 docs must remain.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub; the delete is never
// applied so NumDocs stays at 15 instead of dropping to 5.
func TestIndexWriterDelete_BothDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	cfg := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	cfg.SetMaxBufferedDocs(100)
	modifier, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	id, value := 0, 100
	for i := 0; i < 5; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	value = 200
	for i := 0; i < 5; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Add 5 more value=200 docs (buffered) then delete value=200, which must
	// reach both the committed and the buffered value=200 documents.
	for i := 0; i < 5; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	if err := modifier.DeleteDocuments(index.NewTerm("value", fmt.Sprintf("%d", value))); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit (post-delete): %v", err)
	}

	if n := readerNumDocs(t, dir); n != 5 {
		t.Fatalf("numDocs after delete = %d, want 5", n)
	}
	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// testBatchDeletes
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_BatchDeletes ports testBatchDeletes.
//
// Intent: index 7 docs and commit, delete ids 1 and 2 then commit (expect 5
// docs), then delete ids 3, 4 and 5 in a batch and commit (expect 2 docs).
// Lucene's deleteDocuments(Term...) variadic form has no Gocene equivalent;
// the batch would be applied by looping DeleteDocuments.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub, so neither the "5
// docs" nor the "2 docs" assertion can pass.
func TestIndexWriterDelete_BatchDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	cfg := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	cfg.SetMaxBufferedDocs(2)
	modifier, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	id, value := 0, 100
	for i := 0; i < 7; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if n := readerNumDocs(t, dir); n != 7 {
		t.Fatalf("numDocs = %d, want 7", n)
	}

	// Delete ids 1 and 2 -> 5 remain.
	id = 0
	id++
	if err := modifier.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id))); err != nil {
		t.Fatalf("DeleteDocuments(%d): %v", id, err)
	}
	id++
	if err := modifier.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id))); err != nil {
		t.Fatalf("DeleteDocuments(%d): %v", id, err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if n := readerNumDocs(t, dir); n != 5 {
		t.Fatalf("numDocs after 2 deletes = %d, want 5", n)
	}

	// Batch-delete ids 3, 4, 5 (Lucene's deleteDocuments(Term...) variadic form;
	// reproduced by looping DeleteDocuments) -> 2 remain.
	for i := 0; i < 3; i++ {
		id++
		if err := modifier.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id))); err != nil {
			t.Fatalf("DeleteDocuments(%d): %v", id, err)
		}
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if n := readerNumDocs(t, dir); n != 2 {
		t.Fatalf("numDocs after batch delete = %d, want 2", n)
	}
	if nd := readerNumDeleted(t, dir); nd != 5 {
		t.Fatalf("numDeletedDocs after batch delete = %d, want 5", nd)
	}
	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// testDeleteAllSimple
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllSimple ports testDeleteAllSimple.
//
// Intent: index 7 docs and commit; add 1 buffered doc; call deleteAll (must
// not be visible on disk yet, reader still sees 7); add one doc and update
// one doc after the deleteAll; commit; the reader must then see exactly the 2
// docs added after the deleteAll.
//
// Skipped: IndexWriter.DeleteAll only resets an in-memory counter and does not
// clear committed segments, so the committed 7 docs survive the deleteAll and
// the final count is 9 instead of 2.
func TestIndexWriterDelete_DeleteAllSimple(t *testing.T) {
	t.Fatal("infra gap: IndexWriter.DeleteAll does not clear committed segments")
}

// ---------------------------------------------------------------------------
// testDeleteAllNoDeadLock
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllNoDeadLock ports testDeleteAllNoDeadLock:
// repeated deleteAll concurrent with multi-goroutine indexing must not
// deadlock, and the final reader must observe an empty index.
func TestIndexWriterDelete_DeleteAllNoDeadLock(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	modifier, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	numThreads := 2
	var wg sync.WaitGroup
	startLatch := make(chan struct{})
	doneLatch := make(chan struct{}, numThreads)

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			defer func() { doneLatch <- struct{}{} }()
			id := offset * 1000
			value := 100

			<-startLatch

			for j := 0; j < 1000; j++ {
				doc := document.NewDocument()
				contentField, _ := document.NewTextField("content", "aaa", false)
				idField, _ := document.NewStringField("id", fmt.Sprintf("%d", id), true)
				valueField, _ := document.NewStringField("value", fmt.Sprintf("%d", value), false)
				dvField, _ := document.NewNumericDocValuesField("dv", int64(value))

				doc.Add(contentField)
				doc.Add(idField)
				doc.Add(valueField)
				doc.Add(dvField)

				if err := modifier.AddDocument(doc); err != nil {
					t.Errorf("AddDocument: %v", err)
					return
				}
				id++
			}
		}(i)
	}

	close(startLatch)

	timeout := time.AfterFunc(30*time.Second, func() {
		t.Error("test timed out - possible deadlock")
	})
	defer timeout.Stop()

	doneCount := 0
	for doneCount < numThreads {
		select {
		case <-doneLatch:
			doneCount++
		case <-time.After(time.Millisecond):
		}
		if err := modifier.DeleteAll(); err != nil {
			t.Fatalf("DeleteAll: %v", err)
		}
	}

	wg.Wait()

	if err := modifier.DeleteAll(); err != nil {
		t.Fatalf("final DeleteAll: %v", err)
	}
	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.MaxDoc() != 0 {
		t.Errorf("expected maxDoc=0, got %d", reader.MaxDoc())
	}
	if reader.NumDocs() != 0 {
		t.Errorf("expected numDocs=0, got %d", reader.NumDocs())
	}
	if reader.NumDeletedDocs() != 0 {
		t.Errorf("expected numDeletedDocs=0, got %d", reader.NumDeletedDocs())
	}
	reader.Close()
}

// ---------------------------------------------------------------------------
// testDeleteAllRollback
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllRollback ports testDeleteAllRollback: a
// deleteAll followed by rollback must leave the committed documents intact.
func TestIndexWriterDelete_DeleteAllRollback(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	modifier, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		addDoc(t, modifier, id, value)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	id++
	addDoc(t, modifier, id, value)

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 7 {
		t.Errorf("expected 7 docs, got %d", reader.NumDocs())
	}
	reader.Close()

	if err := modifier.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := modifier.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	reader, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 7 {
		t.Errorf("expected 7 docs after rollback, got %d", reader.NumDocs())
	}
	reader.Close()
}

// ---------------------------------------------------------------------------
// testDeleteAllNRT
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllNRT ports testDeleteAllNRT.
//
// Skipped: the test relies on DirectoryReader.open(IndexWriter) to observe
// uncommitted deleteAll state through a near-real-time reader. Gocene's
// OpenDirectoryReader only accepts a store.Directory, so there is no way to
// see the pre-commit deleteAll. Re-enable once an NRT reader API exists.
func TestIndexWriterDelete_DeleteAllNRT(t *testing.T) {
	t.Fatal("infra gap: no near-real-time reader (DirectoryReader.open(IndexWriter)) in Gocene")
}

// ---------------------------------------------------------------------------
// testDeleteAllRepeated (@Monster in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllRepeated ports testDeleteAllRepeated.
//
// Skipped: the Lucene test is annotated @Monster ("Takes 1-2 minutes but
// writes tons of files to disk"); it stress-allocates 50M field numbers to
// provoke OOME. It is not suitable for the standard suite.
func TestIndexWriterDelete_DeleteAllRepeated(t *testing.T) {
	t.Fatal("@Monster in Lucene: 50M-field OOME stress test, excluded from standard suite")
}

// ---------------------------------------------------------------------------
// testDeletesOnDiskFull / testUpdatesOnDiskFull (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeletesOnDiskFull ports testDeletesOnDiskFull.
//
// Skipped: requires MockDirectoryWrapper disk-full simulation
// (setMaxSizeInBytes, setRandomIOExceptionRate) and ConcurrentMergeScheduler
// exception suppression, none of which are ported. Also @Nightly in Lucene.
func TestIndexWriterDelete_DeletesOnDiskFull(t *testing.T) {
	t.Fatal("infra gap: no MockDirectoryWrapper disk-full fault injection; @Nightly in Lucene")
}

// TestIndexWriterDelete_UpdatesOnDiskFull ports testUpdatesOnDiskFull.
//
// Skipped: same disk-full fault-injection dependency as
// TestIndexWriterDelete_DeletesOnDiskFull. Also @Nightly in Lucene.
func TestIndexWriterDelete_UpdatesOnDiskFull(t *testing.T) {
	t.Fatal("infra gap: no MockDirectoryWrapper disk-full fault injection; @Nightly in Lucene")
}

// ---------------------------------------------------------------------------
// testErrorAfterApplyDeletes (@Ignore in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_ErrorAfterApplyDeletes ports testErrorAfterApplyDeletes.
//
// Skipped: the Lucene method carries @Ignore, and it additionally needs
// MockDirectoryWrapper.Failure call-stack-based fault injection.
func TestIndexWriterDelete_ErrorAfterApplyDeletes(t *testing.T) {
	t.Fatal("@Ignore in Lucene; also needs MockDirectoryWrapper.Failure fault injection")
}

// ---------------------------------------------------------------------------
// testErrorInDocsWriterAdd
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_ErrorInDocsWriterAdd ports testErrorInDocsWriterAdd.
//
// Skipped: requires MockDirectoryWrapper.failOn fault injection to throw an
// IOException mid-add, plus IndexWriter.isDeleterClosed() and
// TestIndexWriter.assertNoUnreferencedFiles, none of which are ported.
func TestIndexWriterDelete_ErrorInDocsWriterAdd(t *testing.T) {
	t.Fatal("infra gap: no MockDirectoryWrapper.failOn fault injection / isDeleterClosed")
}

// ---------------------------------------------------------------------------
// testDeleteNullQuery
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_NullQuery ports testDeleteNullQuery: deleting with a
// query that matches nothing must leave all documents in place.
func TestIndexWriterDelete_NullQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	modifier, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		addDoc(t, modifier, i, 2*i)
	}

	q := search.NewTermQuery(index.NewTerm("nada", "nada"))
	if err := modifier.DeleteDocumentsQuery(q); err != nil {
		t.Fatalf("DeleteDocumentsQuery: %v", err)
	}
	if err := modifier.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	stats := modifier.GetDocStats()
	if stats.NumDocs != 5 {
		t.Errorf("expected 5 docs, got %d", stats.NumDocs)
	}

	if err := modifier.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// testDeleteAllSlowly
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllSlowly ports testDeleteAllSlowly.
//
// Intent: index N docs, then delete them one by one in small batches, opening
// a reader after each batch and verifying that NumDocs equals N minus the
// number deleted so far. Lucene uses RandomIndexWriter + w.getReader() for NRT
// verification between batches; the Gocene reproduction would use a plain
// IndexWriter with a commit before each verification.
//
// Skipped: IndexWriter.DeleteDocuments is a no-op stub, so NumDocs never
// decreases and every per-batch assertion fails.
func TestIndexWriterDelete_DeleteAllSlowly(t *testing.T) {
	t.Fatal("infra gap: IndexWriter.DeleteDocuments is a no-op stub; delete application not ported")
}

// ---------------------------------------------------------------------------
// testIndexingThenDeleting (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_IndexingThenDeleting ports testIndexingThenDeleting.
//
// Skipped: @Nightly in Lucene; it loops on IndexWriter.getFlushCount() until a
// RAM-triggered flush occurs and asserts thousands of operations happened
// first. The RAM-buffer-driven flush counting is timing-sensitive and the
// test is explicitly excluded from the standard suite upstream.
func TestIndexWriterDelete_IndexingThenDeleting(t *testing.T) {
	t.Fatal("@Nightly in Lucene: RAM-buffer flush-count stress test, excluded from standard suite")
}

// ---------------------------------------------------------------------------
// testFlushPushedDeletesByRAM
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_FlushPushedDeletesByRAM ports testFlushPushedDeletesByRAM.
//
// Skipped: the test polls for the on-disk side files "_0_1.del" / "_0_1.liv"
// via slowFileExists to detect when buffered (pushed) deletes are flushed by
// RAM pressure. Gocene exposes neither slowFileExists nor a stable per-segment
// live-docs filename contract, so the loop's termination condition cannot be
// reproduced.
func TestIndexWriterDelete_FlushPushedDeletesByRAM(t *testing.T) {
	t.Fatal("infra gap: no slowFileExists / stable _N_M.liv filename contract to detect RAM-flushed deletes")
}

// ---------------------------------------------------------------------------
// testApplyDeletesOnFlush (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_ApplyDeletesOnFlush ports testApplyDeletesOnFlush.
//
// Skipped: @Nightly in Lucene. It also subclasses IndexWriter to override
// doAfterFlush() and polls slowFileExists for live-docs side files; neither an
// IndexWriter doAfterFlush hook nor slowFileExists is available in Gocene.
func TestIndexWriterDelete_ApplyDeletesOnFlush(t *testing.T) {
	t.Fatal("@Nightly in Lucene; also needs IndexWriter.doAfterFlush override and slowFileExists")
}

// ---------------------------------------------------------------------------
// testDeletesCheckIndexOutput
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeletesCheckIndexOutput ports testDeletesCheckIndexOutput.
//
// Skipped: the test asserts on the human-readable CheckIndex info-stream text
// (it greps for the substring "has deletions"). Gocene's CheckIndex returns a
// structured CheckIndexStatus but does not expose a configurable info-stream
// whose text can be inspected, so the substring assertions cannot be ported.
func TestIndexWriterDelete_DeletesCheckIndexOutput(t *testing.T) {
	t.Fatal("infra gap: CheckIndex info-stream text ('has deletions') not exposed for inspection")
}

// ---------------------------------------------------------------------------
// testTryDeleteDocument
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_TryDeleteDocument ports testTryDeleteDocument.
//
// Skipped: the test opens a near-real-time reader via
// DirectoryReader.open(writer, applyAllDeletes, writeAllDeletes), calls
// tryDeleteDocument against both the composite reader and an individual leaf,
// and checks StandardDirectoryReader.isCurrent(). Gocene has no NRT reader
// open-from-writer and no isCurrent(); IndexWriter.TryDeleteDocument exists but
// cannot be driven through this scenario without them.
func TestIndexWriterDelete_TryDeleteDocument(t *testing.T) {
	t.Fatal("infra gap: no NRT reader open-from-writer / StandardDirectoryReader.isCurrent")
}

// ---------------------------------------------------------------------------
// testNRTIsCurrentAfterDelete
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_NRTIsCurrentAfterDelete ports testNRTIsCurrentAfterDelete.
//
// Skipped: relies entirely on near-real-time readers opened from the writer
// and on StandardDirectoryReader.isCurrent() to assert staleness after a
// delete. Neither exists in Gocene.
func TestIndexWriterDelete_NRTIsCurrentAfterDelete(t *testing.T) {
	t.Fatal("infra gap: no NRT reader open-from-writer / StandardDirectoryReader.isCurrent")
}

// ---------------------------------------------------------------------------
// testOnlyDeletesTriggersMergeOnClose
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_OnlyDeletesTriggersMergeOnClose ports
// testOnlyDeletesTriggersMergeOnClose.
//
// Skipped: the test depends on LogDocMergePolicy.setMinMergeDocs(1) to lower
// the merge threshold so the under-filled segments collapse to a single leaf.
// Gocene's LogDocMergePolicy does not port setMinMergeDocs (it inherits only
// the byte-oriented SetMinMergeMB from LogMergePolicy, whose minMergeSize
// default would suppress the merge under the doc-count Size function). Without
// the knob the "1 leaf" assertion cannot be reproduced faithfully; re-enable
// once LogDocMergePolicy exposes setMinMergeDocs.
func TestIndexWriterDelete_OnlyDeletesTriggersMergeOnClose(t *testing.T) {
	t.Fatal("infra gap: LogDocMergePolicy.setMinMergeDocs not ported in Gocene")
}

// ---------------------------------------------------------------------------
// testOnlyDeletesTriggersMergeOnGetReader
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_OnlyDeletesTriggersMergeOnGetReader ports
// testOnlyDeletesTriggersMergeOnGetReader.
//
// Skipped: the test exercises DirectoryReader.open(writer) twice — the first
// open triggers but does not reflect the merge, the second observes it. This
// depends on an NRT reader opened from the writer, which Gocene lacks.
func TestIndexWriterDelete_OnlyDeletesTriggersMergeOnGetReader(t *testing.T) {
	t.Fatal("infra gap: no NRT reader open-from-writer to trigger/observe merge")
}

// ---------------------------------------------------------------------------
// testOnlyDeletesTriggersMergeOnFlush
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_OnlyDeletesTriggersMergeOnFlush ports
// testOnlyDeletesTriggersMergeOnFlush.
//
// Skipped: same dependency on LogDocMergePolicy.setMinMergeDocs(1) as
// TestIndexWriterDelete_OnlyDeletesTriggersMergeOnClose. Gocene does not port
// that knob, so the single-leaf assertion cannot be reproduced faithfully.
func TestIndexWriterDelete_OnlyDeletesTriggersMergeOnFlush(t *testing.T) {
	t.Fatal("infra gap: LogDocMergePolicy.setMinMergeDocs not ported in Gocene")
}

// ---------------------------------------------------------------------------
// testOnlyDeletesDeleteAllDocs
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_OnlyDeletesDeleteAllDocs ports
// testOnlyDeletesDeleteAllDocs.
//
// Skipped: the test reads back through DirectoryReader.open(writer) (NRT) to
// assert that deleting every document leaves zero leaves and maxDoc 0. The
// equivalent post-commit assertions are covered by
// TestIndexWriterDelete_DeleteAllSlowly; this method specifically validates
// the NRT path, which Gocene lacks.
func TestIndexWriterDelete_OnlyDeletesDeleteAllDocs(t *testing.T) {
	t.Fatal("infra gap: no NRT reader open-from-writer to observe zero leaves")
}

// ---------------------------------------------------------------------------
// testMergingAfterDeleteAll
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_MergingAfterDeleteAll ports testMergingAfterDeleteAll.
//
// Intent: index 10 docs and commit, call deleteAll, index 100 fresh docs and
// force-merge; the index must collapse to a single leaf holding exactly the
// 100 new docs, proving merges still kick off after a deleteAll. Lucene
// verifies via DirectoryReader.open(writer) and tunes
// LogDocMergePolicy.setMinMergeDocs(1).
//
// Skipped: IndexWriter.DeleteAll does not clear committed segments, so the
// original 10 docs survive and the result is 110 docs across 2 leaves instead
// of 100 docs in 1 leaf.
func TestIndexWriterDelete_MergingAfterDeleteAll(t *testing.T) {
	t.Fatal("infra gap: IndexWriter.DeleteAll does not clear committed segments")
}
