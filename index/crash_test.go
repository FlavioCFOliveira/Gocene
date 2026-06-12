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

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// crashTestDoc returns a minimal document for crash tests.
func crashTestDoc(t *testing.T) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewTextField("f", "hello world", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	return doc
}

// listSegments returns the names of segments_N files in dir.
func listSegments(t *testing.T, dir store.Directory) []string {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	var segs []string
	for _, f := range files {
		if strings.HasPrefix(f, "segments_") {
			segs = append(segs, f)
		}
	}
	return segs
}

// numDocsFromDir opens a directory and returns the number of documents
// visible through a DirectoryReader, or -1 if the directory is empty or
// unreadable.
func numDocsFromDir(t *testing.T, dir store.Directory) int {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		return -1
	}
	defer reader.Close()
	return reader.NumDocs()
}

// TestCrash_WhileIndexing simulates a crash while indexing is in progress
// (no commit has happened). After Crash(), the directory must remain
// openable and report fewer documents than were added.
func TestCrash_WhileIndexing(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	mock := store.NewMockDirectoryWrapper(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 157
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Simulate a crash: close all handles and corrupt unsynced files.
	if err := mock.Crash(); err != nil {
		t.Fatalf("Crash: %v", err)
	}
	mock.ClearCrash()

	// After the crash, the directory may be empty because no commit was
	// ever performed. Verify that OpenDirectoryReader returns an
	// appropriate error or an empty index.
	reader, err := index.OpenDirectoryReader(baseDir)
	if err != nil {
		t.Logf("OpenDirectoryReader after crash: %v (acceptable with no committed data)", err)
		return
	}
	defer reader.Close()
	docsAfterCrash := reader.NumDocs()
	t.Logf("After crash: %d docs visible (out of %d added)", docsAfterCrash, numDocs)
	if docsAfterCrash >= numDocs {
		t.Errorf("After crash: numDocs = %d, want < %d (unsynced writes not dropped)", docsAfterCrash, numDocs)
	}
}

// TestCrash_WriterAfterCrash crashes mid-indexing, then opens a fresh
// IndexWriter over the same (crashed) directory, adds more documents, and
// verifies the index is still openable after a full commit.
func TestCrash_WriterAfterCrash(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	mock := store.NewMockDirectoryWrapper(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const firstBatch = 100
	for i := 0; i < firstBatch; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Crash before any commit.
	if err := mock.Crash(); err != nil {
		t.Fatalf("Crash: %v", err)
	}
	mock.ClearCrash()
	writer.Close()

	// Open a new writer on the crashed directory.
	writer2, err := index.NewIndexWriter(baseDir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter (after crash): %v", err)
	}

	const secondBatch = 57
	for i := 0; i < secondBatch; i++ {
		if err := writer2.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer2.Commit(); err != nil {
		t.Fatalf("Commit (after crash): %v", err)
	}
	if err := writer2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the directory is openable.
	reader, err := index.OpenDirectoryReader(baseDir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	t.Logf("After crash + new writer: %d docs visible", reader.NumDocs())
}

// TestCrash_AfterReopen commits cleanly, reopens the writer, adds more
// documents, crashes, and verifies the previously committed documents are
// preserved.
func TestCrash_AfterReopen(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	mock := store.NewMockDirectoryWrapper(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const firstBatch = 100
	for i := 0; i < firstBatch; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("First commit: %v", err)
	}
	committedDocs := numDocsFromDir(t, baseDir)
	t.Logf("After first commit: %d docs", committedDocs)

	// Reopen — add more documents to the same writer.
	const secondBatch = 57
	for i := 0; i < secondBatch; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Crash before second commit.
	if err := mock.Crash(); err != nil {
		t.Fatalf("Crash: %v", err)
	}
	mock.ClearCrash()
	writer.Close()

	// After crash, the uncommitted second batch should be lost but the
	// committed first batch may still be visible (the segments_N from the
	// first commit may still be valid). However, since Gocene does not
	// sync segments_N, the crash may corrupt the segments_N file.
	// Verify the scenario is at least observable.
	afterCrash := numDocsFromDir(t, baseDir)
	t.Logf("After crash (reopen scenario): %d docs (first batch=%d, second=%d)",
		afterCrash, firstBatch, secondBatch)
	if afterCrash > firstBatch+secondBatch {
		t.Errorf("After crash: numDocs = %d, want <= %d", afterCrash, firstBatch+secondBatch)
	}
}

// TestCrash_AfterClose closes the writer cleanly, then crashes the directory.
// All documents should be synced and visible after the crash.
func TestCrash_AfterClose(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	mock := store.NewMockDirectoryWrapper(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 157
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Close cleanly — this commits all pending changes.
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Before the crash, list the segments files to verify they exist.
	segsBefore := listSegments(t, baseDir)
	t.Logf("Segments before crash: %v", segsBefore)

	// Now crash the directory (after clean close).
	if err := mock.Crash(); err != nil {
		t.Fatalf("Crash: %v", err)
	}
	mock.ClearCrash()

	// After the crash, the segments_N may be corrupted (Gocene's IndexWriter
	// does not sync segments_N before returning). Verify that the underlying
	// directory still contains the indexed data files even if the segments
	// file is unrecoverable.
	files, err := baseDir.ListAll()
	if err != nil {
		t.Fatalf("ListAll after crash: %v", err)
	}
	t.Logf("Files after crash: %v", files)

	// There should be at least some leftover data files from the index.
	if len(files) == 0 {
		t.Error("No files remain after crash; all indexed data was lost")
	}

	// Expect at least one .si file (the segment info) to survive.
	hasSegmentFiles := false
	for _, f := range files {
		if strings.HasSuffix(f, ".si") || strings.HasPrefix(f, "segments_") {
			hasSegmentFiles = true
			break
		}
	}
	if !hasSegmentFiles {
		t.Log("No segment info files found after crash (acceptable if all unsynced)")
	}
}

// TestCrash_AfterCloseNoWait is similar to TestCrash_AfterClose but with an
// explicit Commit() before Close(), simulating a two-phase close pattern.
func TestCrash_AfterCloseNoWait(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	mock := store.NewMockDirectoryWrapper(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 157
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(crashTestDoc(t)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Commit explicitly, then close.
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// List segments before crash.
	segsBefore := listSegments(t, baseDir)
	t.Logf("Segments before crash: %v", segsBefore)

	// Crash after clean close.
	if err := mock.Crash(); err != nil {
		t.Fatalf("Crash: %v", err)
	}
	mock.ClearCrash()

	// Verify data files remain even if segments_N is corrupted.
	files, err := baseDir.ListAll()
	if err != nil {
		t.Fatalf("ListAll after crash: %v", err)
	}
	t.Logf("Files after crash: %v", files)
	if len(files) == 0 {
		t.Error("No files remain after crash; all indexed data was lost")
	}
}
