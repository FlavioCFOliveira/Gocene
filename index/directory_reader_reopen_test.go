// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDirectoryReaderReopen.
// Source: lucene/core/src/test/org/apache/lucene/index/TestDirectoryReaderReopen.java
//
// GOC-4190: Port TestDirectoryReaderReopen (Sprint 55, option c).
//
// All 11 test methods from the Java source have a counterpart here. Methods
// that depend on infrastructure not yet ported are marked with t.Skip and an
// explicit reason; the remainder run against the current implementation.
//
// Missing infrastructure (drives the t.Skip calls below):
//   - DirectoryReader.openIfChanged(reader[, writer|commit]): the incremental
//     reopen entry point. Gocene exposes Reopen/ReopenFromCommit, which always
//     build a fresh reader and never return the "no changes" sentinel, so the
//     instance-identity assertions of the Java test cannot be reproduced.
//   - DirectoryReader.open(writer): NRT reader pulled directly from the writer.
//   - IndexWriter.DeleteDocuments / updateNumericDocValue: currently no-op
//     stubs, so every modifyIndex case and the delete/DV-update tests cannot
//     observe their effect.
//   - MockDirectoryWrapper failure injection (FakeIOException on readLiveDocs).
//   - DirectoryReader leaf APIs (Terms/StoredFields) through OpenDirectoryReader
//     fail with "core readers are nil", so index-equality comparisons that read
//     postings are unavailable.
package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// reopenDoc builds a document mirroring createDocument(n, numFields) from the
// Java source: a stored text "field1" plus numFields-1 extra stored text
// fields.
func reopenDoc(n, numFields int) *document.Document {
	doc := &document.Document{}
	base := "a" + strconv.Itoa(n)

	f1, _ := document.NewTextField("field1", base, true)
	doc.Add(f1)

	extended := base + " b" + strconv.Itoa(n)
	for i := 1; i < numFields; i++ {
		field, _ := document.NewTextField("field"+strconv.Itoa(i+1), extended, true)
		doc.Add(field)
	}
	return doc
}

// TestDirectoryReaderReopen_Reopen ports testReopen().
// Java drives performDefaultTests, which asserts openIfChanged returns the same
// instance when nothing changed and a new instance after each modifyIndex step.
// Reopen always rebuilds, and modifyIndex relies on no-op DeleteDocuments.
func TestDirectoryReaderReopen_Reopen(t *testing.T) {
	t.Fatal("needs DirectoryReader.openIfChanged identity semantics; modifyIndex relies on no-op DeleteDocuments")
}

// TestDirectoryReaderReopen_CommitReopen ports testCommitReopen().
// Java commits in iterations and reopens via openIfChanged, reading stored
// fields from the previous iteration through the live reader.
func TestDirectoryReaderReopen_CommitReopen(t *testing.T) {
	t.Fatal("needs DirectoryReader.openIfChanged and leaf StoredFields access (core readers nil)")
}

// TestDirectoryReaderReopen_CommitRecreate ports testCommitRecreate().
// The recreate path closes and re-opens the reader on each commit rather than
// using openIfChanged. The doc-count progression is verified here; the
// per-iteration stored-field readback of the Java test needs leaf access.
func TestDirectoryReaderReopen_CommitRecreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	const m = 3
	for i := 0; i < 4; i++ {
		for j := 0; j < m; j++ {
			if err := writer.AddDocument(reopenDoc(i*m+j, 4)); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit iteration %d: %v", i, err)
		}

		// Recreate: close the stale reader and open the committed index.
		if err := reader.Close(); err != nil {
			t.Fatalf("reader.Close: %v", err)
		}
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader iteration %d: %v", i, err)
		}

		if got, want := reader.NumDocs(), (i+1)*m; got != want {
			t.Fatalf("iteration %d: NumDocs = %d, want %d", i, got, want)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("final reader.Close: %v", err)
	}
}

// TestDirectoryReaderReopen_ThreadSafety ports testThreadSafety().
// Java spins 20-40 threads that concurrently openIfChanged, search, and compare
// reader couples while a writer mutates the index.
func TestDirectoryReaderReopen_ThreadSafety(t *testing.T) {
	t.Fatal("needs DirectoryReader.openIfChanged and leaf search APIs (core readers nil)")
}

// TestDirectoryReaderReopen_ReopenOnCommit ports testReopenOnCommit().
// Java reopens onto each listed IndexCommit via openIfChanged(r, commit) and
// asserts numDocs derived from the per-commit user data.
func TestDirectoryReaderReopen_ReopenOnCommit(t *testing.T) {
	t.Fatal("needs openIfChanged(reader, commit); deleteDocuments is a no-op stub")
}

// TestDirectoryReaderReopen_OpenIfChangedNRTToCommit ports
// testOpenIfChangedNRTToCommit(). Java opens an NRT reader from the writer and
// reopens it backwards onto an older commit.
func TestDirectoryReaderReopen_OpenIfChangedNRTToCommit(t *testing.T) {
	t.Fatal("needs DirectoryReader.open(writer) and openIfChanged(reader, commit)")
}

// TestDirectoryReaderReopen_OverDecRefDuringReopen ports
// testOverDecRefDuringReopen(). Java injects a FakeIOException on readLiveDocs
// during reopen and asserts the original reader survives the failed reopen.
func TestDirectoryReaderReopen_OverDecRefDuringReopen(t *testing.T) {
	t.Fatal("needs MockDirectoryWrapper failure injection and openIfChanged")
}

// TestDirectoryReaderReopen_NPEAfterInvalidReindex1 ports
// testNPEAfterInvalidReindex1(). Java blows away the index under an open
// reader, reindexes incompatibly, and asserts openIfChanged fails cleanly.
func TestDirectoryReaderReopen_NPEAfterInvalidReindex1(t *testing.T) {
	t.Fatal("needs DirectoryReader.openIfChanged and updateNumericDocValue")
}

// TestDirectoryReaderReopen_NPEAfterInvalidReindex2 ports
// testNPEAfterInvalidReindex2(): same scenario as variant 1 without the
// doc-values update.
func TestDirectoryReaderReopen_NPEAfterInvalidReindex2(t *testing.T) {
	t.Fatal("needs DirectoryReader.openIfChanged on a recreated index")
}

// TestDirectoryReaderReopen_NRTMdeletes ports testNRTMdeletes().
// Java reopens a non-NRT reader backwards across snapshotted commits while
// documents are deleted, verifying numDocs per commit.
func TestDirectoryReaderReopen_NRTMdeletes(t *testing.T) {
	t.Fatal("needs SnapshotDeletionPolicy, openIfChanged(reader, commit), and applied deletes")
}

// TestDirectoryReaderReopen_ListCommits exercises the commit-listing plumbing
// that testReopenOnCommit depends on. This is not a 1:1 port of a Java @Test
// method but isolates the verifiable subset reachable today: that ListCommits
// enumerates the index and a reopened reader reflects every committed document.
//
// The per-commit user-data assertions of testReopenOnCommit are not reproduced
// here: ListCommits is currently a stub that returns only the latest commit
// rebuilt from ReadSegmentInfos, and the setLiveCommitData payload is not
// round-tripped through the segments file.
func TestDirectoryReaderReopen_ListCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 4; i++ {
		if err := writer.AddDocument(reopenDoc(i, 4)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("ListCommits returned no commits")
	}

	// Reopening the writer-built index reflects every committed document.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got, want := reader.NumDocs(), 4; got != want {
		t.Errorf("NumDocs = %d, want %d", got, want)
	}
}
