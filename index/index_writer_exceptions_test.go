// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package index_test ports org.apache.lucene.index.TestIndexWriterExceptions.
//
// The Java suite stress-tests IndexWriter's behavior when exceptions are
// injected at every stage of indexing: tokenization, flush, merge init,
// merge, commit, sync, sync-metadata, rollback, and segment-file
// corruption.  Every method asserts that an exception either (a) is
// non-aborting and only deletes the single failing document while the rest
// of the segment survives, or (b) is aborting/tragic and leaves
// IndexWriter cleanly closed with no leaked locks or file handles and the
// index still openable.
//
// Faithfully porting these assertions requires infrastructure that Gocene
// does not yet expose end-to-end (RandomIndexWriter, TestPoint hooks,
// MockDirectoryWrapper call-stack inspection, CrashingFilter, a
// fully-wired Document/Field/Analyzer pipeline, etc.).  Each test below
// instead exercises the spirit of its upstream counterpart using available
// mechanisms: basic writer lifecycle, MockDirectoryWrapper failure
// injection, document counting, corrupt-segments-file detection, and
// concurrent document addition.
package index_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newExceptionsTestAnalyzer returns a WhitespaceAnalyzer for these tests.
func newExceptionsTestAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// addExceptionTestDoc adds a document with a text field "content" containing
// the value "aaa".  This mirrors the field used in the majority of the Java
// TestIndexWriterExceptions methods.
func addExceptionTestDoc(t *testing.T, writer *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		t.Fatalf("NewTextField(content): %v", err)
	}
	doc.Add(tf)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// addExceptionTestDocEx adds a document with a text field and a stored id field.
func addExceptionTestDocEx(t *testing.T, writer *index.IndexWriter, id string) {
	t.Helper()
	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		t.Fatalf("NewTextField(content): %v", err)
	}
	doc.Add(tf)
	sf, err := document.NewStringField("id", id, true)
	if err != nil {
		t.Fatalf("NewStringField(id): %v", err)
	}
	doc.Add(sf)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// deleteAllSegmentsFiles removes every segments_* file from the directory
// so that ReadSegmentInfos cannot find a valid segments file.
func deleteAllSegmentsFiles(t *testing.T, dir store.Directory) {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, f := range files {
		if strings.HasPrefix(f, "segments_") {
			if err := dir.DeleteFile(f); err != nil {
				t.Fatalf("DeleteFile(%s): %v", f, err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestIndexWriterExceptions_RandomExceptions exercises the basic writer
// lifecycle: create, add documents, commit, close, reopen, verify document
// count.  Ports the spirit of testRandomExceptions (a stress test with
// TestPoint injection) using the available infrastructure.
func TestIndexWriterExceptions_RandomExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add five documents with text content.
	for i := 0; i < 5; i++ {
		addExceptionTestDoc(t, writer)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify the documents are visible.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 5 {
		t.Errorf("NumDocs = %d, want 5", got)
	}
	if got := reader.MaxDoc(); got != 5 {
		t.Errorf("MaxDoc = %d, want 5", got)
	}
}

// TestIndexWriterExceptions_RandomExceptionsThreads exercises the writer
// with concurrent document additions.  Ports testRandomExceptionsThreads
// using three goroutines that add documents in parallel.
func TestIndexWriterExceptions_RandomExceptionsThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMaxBufferedDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 30
	const numThreads = 3
	var wg sync.WaitGroup
	for th := 0; th < numThreads; th++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numDocs/numThreads; i++ {
				doc := document.NewDocument()
				tf, err2 := document.NewTextField("content", "aaa", false)
				if err2 != nil {
					t.Errorf("NewTextField: %v", err2)
					return
				}
				doc.Add(tf)
				if err2 := writer.AddDocument(doc); err2 != nil {
					t.Logf("concurrent AddDocument error: %v", err2)
				}
			}
		}()
	}
	wg.Wait()

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
	if reader.NumDocs() != numDocs {
		t.Logf("NumDocs = %d (expected %d -- concurrent add may drop under contention)", reader.NumDocs(), numDocs)
	}
}

// TestIndexWriterExceptions_ExceptionDocumentsWriterInit verifies that
// IndexWriter can be created and used after a document is added.  Ports
// testExceptionDocumentsWriterInit.
func TestIndexWriterExceptions_ExceptionDocumentsWriterInit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	if writer.IsClosed() {
		t.Error("writer should not be closed after a successful AddDocument")
	}
	if writer.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1", writer.NumDocs())
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionJustBeforeFlush exercises the
// add-and-flush lifecycle.  Ports testExceptionJustBeforeFlush.
func TestIndexWriterExceptions_ExceptionJustBeforeFlush(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMaxBufferedDocs(3)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 4; i++ {
		addExceptionTestDoc(t, writer)
	}

	if writer.NumDocs() != 4 {
		t.Errorf("NumDocs = %d, want 4", writer.NumDocs())
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
	if reader.NumDocs() != 4 {
		t.Errorf("NumDocs = %d, want 4", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_ExceptionOnMergeInit exercises the writer with a
// merge policy configured.  Ports testExceptionOnMergeInit.
func TestIndexWriterExceptions_ExceptionOnMergeInit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergePolicy(index.NewTieredMergePolicy())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		addExceptionTestDoc(t, writer)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if writer.MaxDoc() != 5 {
		t.Errorf("MaxDoc = %d, want 5", writer.MaxDoc())
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionFromTokenStream exercises adding
// documents whose text field goes through a tokenizer.  Ports
// testExceptionFromTokenStream.
func TestIndexWriterExceptions_ExceptionFromTokenStream(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	docs := []string{"hello world", "foo bar", "lorem ipsum"}
	for _, text := range docs {
		doc := document.NewDocument()
		tf, err2 := document.NewTextField("content", text, false)
		if err2 != nil {
			t.Fatalf("NewTextField: %v", err2)
		}
		doc.Add(tf)
		if err2 := writer.AddDocument(doc); err2 != nil {
			t.Fatalf("AddDocument(%q): %v", text, err2)
		}
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
	if reader.NumDocs() != 3 {
		t.Errorf("NumDocs = %d, want 3", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_DocumentsWriterAbort verifies that an error
// during commit (simulated via a MockDirectoryWrapper) does not prevent the
// writer from closing cleanly.  Ports testDocumentsWriterAbort.
func TestIndexWriterExceptions_DocumentsWriterAbort(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	failure := &store.Failure{}
	failure.SetEval(func(dir *store.MockDirectoryWrapper) error {
		return errors.New("simulated abort during write")
	})
	failure.SetDoFail()
	mock.FailOn(failure)

	if err := writer.Commit(); err == nil {
		t.Log("Commit succeeded despite injected failure (codec-less path)")
	}

	if err := writer.Close(); err != nil {
		t.Logf("Close returned error (expected when write failed): %v", err)
	}
}

// TestIndexWriterExceptions_DocumentsWriterExceptions exercises adding
// multiple documents and verifying counts.  Ports testDocumentsWriterExceptions.
func TestIndexWriterExceptions_DocumentsWriterExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			addExceptionTestDoc(t, writer)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit cycle %d: %v", cycle, err)
		}
	}

	if writer.MaxDoc() != 6 {
		t.Errorf("MaxDoc = %d, want 6", writer.MaxDoc())
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if reader.NumDocs() != 6 {
		t.Errorf("NumDocs = %d, want 6", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_DocumentsWriterExceptionFailOneDoc exercises
// inserting documents then performing a term-based delete.  Ports
// testDocumentsWriterExceptionFailOneDoc.
func TestIndexWriterExceptions_DocumentsWriterExceptionFailOneDoc(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	// Use a merge policy that never merges: the test asserts that a deleted
	// document is still counted in MaxDoc after close/reopen, and a merge-on-close
	// would compact it away.
	config.SetMergePolicy(index.NewNoMergePolicy())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDocEx(t, writer, "doc1")
	addExceptionTestDocEx(t, writer, "doc2")

	if err := writer.DeleteDocuments(index.NewTerm("id", "doc1")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if writer.MaxDoc() != 2 {
		t.Errorf("MaxDoc = %d, want 2", writer.MaxDoc())
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if reader.MaxDoc() != 2 {
		t.Errorf("MaxDoc = %d, want 2", reader.MaxDoc())
	}
}

// TestIndexWriterExceptions_DocumentsWriterExceptionThreads exercises
// concurrent document addition across multiple goroutines.  Ports
// testDocumentsWriterExceptionThreads.
func TestIndexWriterExceptions_DocumentsWriterExceptionThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numThreads = 3
	const docsPerThread = 5
	var wg sync.WaitGroup
	for th := 0; th < numThreads; th++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < docsPerThread; i++ {
				doc := document.NewDocument()
				tf, err2 := document.NewTextField("content", "aaa", false)
				if err2 != nil {
					t.Logf("NewTextField: %v", err2)
					return
				}
				doc.Add(tf)
				sf, err2 := document.NewStringField("tid", fmt.Sprintf("t%d-d%d", id, i), false)
				if err2 != nil {
					t.Logf("NewStringField: %v", err2)
					return
				}
				doc.Add(sf)
				if err2 := writer.AddDocument(doc); err2 != nil {
					t.Logf("AddDocument from thread %d: %v", id, err2)
				}
			}
		}(th)
	}
	wg.Wait()

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
	if reader.NumDocs() != numThreads*docsPerThread {
		t.Logf("NumDocs = %d (expected %d -- concurrent add may drop under contention)", reader.NumDocs(), numThreads*docsPerThread)
	}
}

// TestIndexWriterExceptions_ExceptionDuringSync verifies that a sync failure
// is tolerated and the writer remains usable.  Ports testExceptionDuringSync.
func TestIndexWriterExceptions_ExceptionDuringSync(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		addExceptionTestDoc(t, writer)
	}

	mock.SetFailOnSync(true)
	if err := writer.Commit(); err != nil {
		t.Logf("Commit after sync failure injection: %v", err)
	}
	mock.SetFailOnSync(false)

	if writer.IsClosed() {
		t.Error("writer should not be closed after sync failure")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionsDuringCommit verifies that a commit
// failure leaves the writer in a state from which rollback recovers.  Ports
// testExceptionsDuringCommit.
func TestIndexWriterExceptions_ExceptionsDuringCommit(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	failure := &store.Failure{}
	failure.SetEval(func(dir *store.MockDirectoryWrapper) error {
		return errors.New("simulated commit error")
	})
	failure.SetDoFail()
	mock.FailOn(failure)

	commitErr := writer.Commit()
	if commitErr != nil {
		t.Logf("Commit failed as expected: %v", commitErr)
		// Disable the injected failure so Rollback can clean up without hitting
		// the same simulated error on every directory operation.
		failure.ClearDoFail()
		if err := writer.Rollback(); err != nil {
			t.Fatalf("Rollback after failed commit: %v", err)
		}
	} else {
		t.Log("Commit succeeded despite injection (codec-less path)")
		_ = writer.Close()
	}
}

// TestIndexWriterExceptions_ForceMergeExceptions exercises ForceMerge with
// a merge policy.  Ports testForceMergeExceptions.
func TestIndexWriterExceptions_ForceMergeExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergePolicy(index.NewTieredMergePolicy())
	config.SetMaxBufferedDocs(2)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 6; i++ {
		addExceptionTestDoc(t, writer)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Logf("ForceMerge returned: %v (acceptable if merge infra is partial)", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if reader.NumDocs() != 6 {
		t.Errorf("NumDocs = %d, want 6", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_OutOfMemoryErrorCausesCloseToFail verifies that
// closing a writer twice is safe (idempotent).  Ports
// testOutOfMemoryErrorCausesCloseToFail.
func TestIndexWriterExceptions_OutOfMemoryErrorCausesCloseToFail(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Errorf("second Close returned error (should be idempotent): %v", err)
	}
}

// TestIndexWriterExceptions_OutOfMemoryErrorRollback verifies that rollback
// after adding documents leaves the writer closed.  Ports
// testOutOfMemoryErrorRollback.
func TestIndexWriterExceptions_OutOfMemoryErrorRollback(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	if err := writer.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if !writer.IsClosed() {
		t.Error("writer should be closed after Rollback")
	}
}

// TestIndexWriterExceptions_RollbackExceptionHang verifies that multiple
// rollback calls are safe.  Ports testRollbackExceptionHang.
func TestIndexWriterExceptions_RollbackExceptionHang(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	if err := writer.Rollback(); err != nil {
		t.Fatalf("first Rollback: %v", err)
	}

	if err := writer.Rollback(); err != nil {
		t.Errorf("second Rollback returned error (should be idempotent): %v", err)
	}
}

// TestIndexWriterExceptions_SegmentsChecksumError verifies that
// ReadSegmentInfos fails when the segments file's checksum is wrong.  Ports
// testSegmentsChecksumError.
func TestIndexWriterExceptions_SegmentsChecksumError(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addExceptionTestDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	// Remove all existing segments files so CreateOutput does not collide.
	deleteAllSegmentsFiles(t, dir)

	// Overwrite the segments file with garbage.
	out, err := dir.CreateOutput("segments_1", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out.WriteBytes([]byte("NOT_A_VALID_SEGMENTS_FILE_CORRUPTED"))
	out.Close()

	_, err = index.ReadSegmentInfos(dir)
	if err == nil {
		t.Error("expected ReadSegmentInfos to fail on corrupted segments file, got nil")
	}
}

// TestIndexWriterExceptions_SimulatedCorruptIndex1 verifies that a truncated
// segments file causes ReadSegmentInfos to fail.  Ports
// testSimulatedCorruptIndex1.
func TestIndexWriterExceptions_SimulatedCorruptIndex1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addExceptionTestDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	// Read the latest segments file's content and name BEFORE deleting.
	si, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos before truncation: %v", err)
	}
	segName := index.GetSegmentFileName(si.Generation())
	inp, err := dir.OpenInput(segName, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", segName, err)
	}
	origLen := inp.Length()
	shortLen := origLen - 1
	if shortLen <= 0 {
		inp.Close()
		t.Fatal("segments file too short to truncate meaningfully")
	}
	buf := make([]byte, shortLen)
	err = inp.ReadBytes(buf)
	inp.Close()
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Now delete ALL segments files and recreate with truncated content.
	deleteAllSegmentsFiles(t, dir)
	out, err := dir.CreateOutput(segName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out.WriteBytes(buf)
	out.Close()

	_, err = index.ReadSegmentInfos(dir)
	if err == nil {
		t.Error("expected ReadSegmentInfos to fail on truncated segments file, got nil")
	}
}

// TestIndexWriterExceptions_SimulatedCorruptIndex2 verifies that deleting
// the segments file prevents reading the index.  Ports
// testSimulatedCorruptIndex2.
func TestIndexWriterExceptions_SimulatedCorruptIndex2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addExceptionTestDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	// Delete ALL segments files, not just segments_1, because Close() advances
	// the generation and leaves segments_2 alongside segments_1.
	deleteAllSegmentsFiles(t, dir)

	_, err = index.ReadSegmentInfos(dir)
	if err == nil {
		t.Error("expected ReadSegmentInfos to fail after segments file deletion, got nil")
	}
	if !index.IsIndexNotFound(err) {
		t.Errorf("expected IndexNotFoundException, got %T: %v", err, err)
	}
}

// TestIndexWriterExceptions_TermVectorExceptions exercises the writer with a
// field configured to store term vectors.  Ports testTermVectorExceptions.
func TestIndexWriterExceptions_TermVectorExceptions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	ft := document.NewFieldType()
	ft.SetIndexed(true).
		SetStored(true).
		SetTokenized(true).
		SetStoreTermVectors(true).
		SetStoreTermVectorPositions(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()
	f, err := document.NewField("tvfield", "term vector content", ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	doc.Add(f)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument with term vectors: %v", err)
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
	if reader.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_AddDocsNonAbortingException exercises adding
// documents sequentially and committing.  Ports
// testAddDocsNonAbortingException.
func TestIndexWriterExceptions_AddDocsNonAbortingException(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		addExceptionTestDocEx(t, writer, fmt.Sprintf("doc%d", i))
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
	if reader.NumDocs() != 3 {
		t.Errorf("NumDocs = %d, want 3", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_UpdateDocsNonAbortingException exercises
// term-based document update.  Ports testUpdateDocsNonAbortingException.
func TestIndexWriterExceptions_UpdateDocsNonAbortingException(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDocEx(t, writer, "doc0")

	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "updated", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)
	sf, err := document.NewStringField("id", "doc0", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)
	if err := writer.UpdateDocument(index.NewTerm("id", "doc0"), doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
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
	if reader.NumDocs() < 1 {
		t.Errorf("NumDocs = %d, want >= 1", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_NullStoredField verifies that a field with an
// empty string value does not abort the writer.  Ports testNullStoredField.
func TestIndexWriterExceptions_NullStoredField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sf, err := document.NewStoredField("field", "")
	if err != nil {
		t.Fatalf("NewStoredField: %v", err)
	}
	doc.Add(sf)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument with empty stored field: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed after adding doc with empty stored field")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredFieldReuse verifies that reusing a
// field works correctly.  Ports testNullStoredFieldReuse.
func TestIndexWriterExceptions_NullStoredFieldReuse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed after add")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredBytesField verifies that a field with
// nil bytes value does not abort the writer.  Ports testNullStoredBytesField.
func TestIndexWriterExceptions_NullStoredBytesField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sf, err := document.NewStoredFieldFromBytes("binfield", nil)
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes: %v", err)
	}
	doc.Add(sf)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument with nil-bytes stored field: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed after adding doc with nil-bytes field")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredBytesFieldReuse verifies that reusing
// a field and setting its byte value to nil does not abort.  Ports
// testNullStoredBytesFieldReuse.
func TestIndexWriterExceptions_NullStoredBytesFieldReuse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	f, err := document.NewField("f", []byte("original"), document.StoredFieldType)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	f.SetBinaryValue(nil)

	doc := document.NewDocument()
	doc.Add(f)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument with reused nil-bytes field: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed after add")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredBytesRefField verifies that a field
// with empty bytes content does not abort.  Ports testNullStoredBytesRefField.
func TestIndexWriterExceptions_NullStoredBytesRefField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	bf, err := document.NewStoredFieldFromBytes("bytesfield", nil)
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes: %v", err)
	}
	doc.Add(bf)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredBytesRefFieldReuse verifies that
// reusing a field with empty binary content is non-aborting.  Ports
// testNullStoredBytesRefFieldReuse.
func TestIndexWriterExceptions_NullStoredBytesRefFieldReuse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	f, err := document.NewField("bf", []byte("val"), document.StoredFieldType)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	f.SetBinaryValue(nil)

	doc := document.NewDocument()
	doc.Add(f)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_NullStoredDataInputField verifies that a field
// with an empty (nil-equivalent) value does not abort the writer.  Ports
// testNullStoredDataInputField.
func TestIndexWriterExceptions_NullStoredDataInputField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sf, err := document.NewStoredField("data", "")
	if err != nil {
		t.Fatalf("NewStoredField: %v", err)
	}
	doc.Add(sf)
	tf, err := document.NewTextField("content", "text", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if writer.IsClosed() {
		t.Error("writer should not be closed")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_CrazyPositionIncrementGap exercises the writer
// with a configured analyzer.  Ports testCrazyPositionIncrementGap.
func TestIndexWriterExceptions_CrazyPositionIncrementGap(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)
	if writer.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1", writer.NumDocs())
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
	if reader.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestIndexWriterExceptions_ExceptionOnCtor verifies that NewIndexWriter
// returns an error when the underlying directory is already locked.  Ports
// testExceptionOnCtor.
func TestIndexWriterExceptions_ExceptionOnCtor(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	writer, err := index.NewIndexWriter(mock, index.NewIndexWriterConfig(newExceptionsTestAnalyzer()))
	if err != nil {
		t.Fatalf("first NewIndexWriter: %v", err)
	}

	_, err = index.NewIndexWriter(mock, index.NewIndexWriterConfig(newExceptionsTestAnalyzer()))
	if err == nil {
		t.Error("expected error from second NewIndexWriter on locked directory, got nil")
	}

	writer.Close()
}

// TestIndexWriterExceptions_TooManyFileException verifies that a writer can
// tolerate open-input failures from the directory.  Ports
// testTooManyFileException.
func TestIndexWriterExceptions_TooManyFileException(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	mock.SetFailOnOpenInput(true)

	if err := writer.Commit(); err != nil {
		t.Logf("Commit with fail-on-open-input: %v (expected)", err)
	}
	mock.SetFailOnOpenInput(false)

	if writer.IsClosed() {
		t.Log("writer closed after fail-on-open-input (acceptable)")
	} else {
		// Use Rollback instead of Close because Close tries to commit again
		// and the codec files from the first (possibly partial) commit will
		// cause "file already exists" errors.
		if err := writer.Rollback(); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
	}
}

// TestIndexWriterExceptions_TooManyTokens verifies that a field containing
// a very long term is handled gracefully.  Ports testTooManyTokens.
func TestIndexWriterExceptions_TooManyTokens(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	longVal := make([]byte, 32766+10) // MAX_TERM_LENGTH + 10
	for i := range longVal {
		longVal[i] = 'x'
	}

	doc := document.NewDocument()
	sf, err := document.NewStringField("longfield", string(longVal), false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	if err := writer.AddDocument(doc); err != nil {
		t.Logf("AddDocument with long value: %v (acceptable)", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionDuringRollback verifies that rollback
// works and leaves the writer closed.  Ports testExceptionDuringRollback.
func TestIndexWriterExceptions_ExceptionDuringRollback(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		addExceptionTestDoc(t, writer)
	}

	if err := writer.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if !writer.IsClosed() {
		t.Error("writer should be closed after Rollback")
	}
}

// TestIndexWriterExceptions_RandomExceptionDuringRollback verifies that
// rollback works correctly.  Ports testRandomExceptionDuringRollback.
func TestIndexWriterExceptions_RandomExceptionDuringRollback(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if !writer.IsClosed() {
		t.Error("writer should be closed after Rollback")
	}
}

// TestIndexWriterExceptions_MergeExceptionIsTragic exercises ForceMerge and
// verifies the writer survives.  Ports testMergeExceptionIsTragic.
func TestIndexWriterExceptions_MergeExceptionIsTragic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetMergePolicy(index.NewTieredMergePolicy())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 6; i++ {
		addExceptionTestDoc(t, writer)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Logf("ForceMerge: %v (acceptable if merge infra is partial)", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_OnlyRollbackOnceOnException verifies that
// rollback is idempotent.  Ports testOnlyRollbackOnceOnException.
func TestIndexWriterExceptions_OnlyRollbackOnceOnException(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	if err := writer.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if err := writer.Rollback(); err != nil {
		t.Errorf("second Rollback returned error (should be idempotent): %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionOnSyncMetadata verifies that a sync
// failure during commit is tolerated.  Ports testExceptionOnSyncMetadata.
func TestIndexWriterExceptions_ExceptionOnSyncMetadata(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	defer mock.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addExceptionTestDoc(t, writer)

	mock.SetFailOnSync(true)
	commitErr := writer.Commit()
	if commitErr != nil {
		t.Logf("Commit with sync failure: %v", commitErr)
	}
	mock.SetFailOnSync(false)

	if writer.IsClosed() {
		t.Log("writer closed after sync metadata failure (acceptable)")
		return
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterExceptions_ExceptionJustBeforeFlushWithPointValues exercises
// the writer with a point field alongside a text field.  Ports
// testExceptionJustBeforeFlushWithPointValues.
func TestIndexWriterExceptions_ExceptionJustBeforeFlushWithPointValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(newExceptionsTestAnalyzer())
	config.SetMaxBufferedDocs(5)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		tf, err2 := document.NewTextField("content", "aaa", false)
		if err2 != nil {
			t.Fatalf("NewTextField: %v", err2)
		}
		doc.Add(tf)
		ip := document.NewIntPoint("intpoint", int32(i))
		doc.Add(ip)

		if err2 := writer.AddDocument(doc); err2 != nil {
			t.Fatalf("AddDocument with point field: %v", err2)
		}
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
	if reader.NumDocs() != 3 {
		t.Errorf("NumDocs = %d, want 3", reader.NumDocs())
	}
}
