// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter exception handling.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterOnError
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnError.java
//
// GOC-4228: Port test org.apache.lucene.index.TestIndexWriterOnError (Sprint 55).
//
// Port strategy (Sprint 55 option c): the Java suite has five @Test methods
// (testOOM, testUnknownError, testLinkageError, testIOError, testCheckpoint),
// each of which only differs in the fake VM error it injects. All five delegate
// to the doTest(failOn) harness, which installs a MockDirectoryWrapper.Failure,
// drives ~2000 docs through single/block adds, deletes, doc-values updates,
// flushes and checkIndex passes, and asserts that the injected error never
// produces index corruption and is never replaced by a different exception.
//
// The Go counterpart keeps a 1:1 test-method mapping. The shared harness
// (doIndexWriterOnErrorTest) runs the non-faulted add/commit roundtrip for real,
// proving the clean path is sound; the fault-injection assertion is gated with
// t.Skip so each missing capability is explicit rather than silently absent.
//
// Known API gaps that force a skip in this file:
//   - MockDirectoryWrapper and its Failure callback do not exist, so fake VM
//     errors (OutOfMemoryError, UnknownError, LinkageError, IOError) cannot be
//     injected into store operations.
//   - callStackContains (scoping injection to IndexWriter / IndexFileDeleter
//     frames) has no Go equivalent.
//   - IndexWriter.Rollback / GetTragedy (tragic-exception recovery) are not
//     wired here, so the post-error rollback assertion cannot be checked.
//   - IndexWriter.UpdateNumericDocValue / UpdateBinaryDocValue are not
//     implemented, so the random doc-values update branch cannot run.
//   - DeleteDocuments accepts a single Term only; the multi-Term overload used
//     for block-document deletes is unavailable.
//   - TestUtil.checkIndex / checkReader fault-tolerant verification is not
//     wired here.
package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newOnErrorDoc builds the per-iteration document mirroring the `doc` local of
// the Java doTest loop: an id, the five doc-values flavors, text fields
// (including payload and term-vector variants), stored fields, and points.
func newOnErrorDoc(t *testing.T, id int) *document.Document {
	t.Helper()
	s := strconv.Itoa(id)
	doc := document.NewDocument()

	idField, err := document.NewStringField("id", s, false)
	if err != nil {
		t.Fatalf("NewStringField(id) error = %v", err)
	}
	doc.Add(idField)

	dv, err := document.NewNumericDocValuesField("dv", int64(id))
	if err != nil {
		t.Fatalf("NewNumericDocValuesField(dv) error = %v", err)
	}
	doc.Add(dv)

	dv2, err := document.NewBinaryDocValuesField("dv2", []byte(s))
	if err != nil {
		t.Fatalf("NewBinaryDocValuesField(dv2) error = %v", err)
	}
	doc.Add(dv2)

	dv3, err := document.NewSortedDocValuesField("dv3", []byte(s))
	if err != nil {
		t.Fatalf("NewSortedDocValuesField(dv3) error = %v", err)
	}
	doc.Add(dv3)

	dv4, err := document.NewSortedSetDocValuesField("dv4", [][]byte{
		[]byte(s), []byte(strconv.Itoa(id - 1)),
	})
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesField(dv4) error = %v", err)
	}
	doc.Add(dv4)

	dv5, err := document.NewSortedNumericDocValuesField("dv5", []int64{
		int64(id), int64(id - 1),
	})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesField(dv5) error = %v", err)
	}
	doc.Add(dv5)

	text1, err := document.NewTextField("text1", "the quick brown fox "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text1) error = %v", err)
	}
	doc.Add(text1)

	stored1a, err := document.NewStoredField("stored1", "foo")
	if err != nil {
		t.Fatalf("NewStoredField(stored1=foo) error = %v", err)
	}
	doc.Add(stored1a)
	stored1b, err := document.NewStoredField("stored1", "bar")
	if err != nil {
		t.Fatalf("NewStoredField(stored1=bar) error = %v", err)
	}
	doc.Add(stored1b)

	payloads, err := document.NewTextField("text_payloads", "lorem ipsum "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text_payloads) error = %v", err)
	}
	doc.Add(payloads)

	vectors, err := document.NewTextField("text_vectors", "dolor sit "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text_vectors) error = %v", err)
	}
	doc.Add(vectors)

	doc.Add(document.NewIntPoint("point", int32(id)))
	doc.Add(document.NewIntPoints("point2d", int32(id), int32(-id)))

	return doc
}

// doIndexWriterOnErrorTest ports the doTest(failOn) harness shared by all five
// Java @Test methods. The Java harness installs a MockDirectoryWrapper.Failure
// and asserts the injected fake VM error never corrupts the index. Gocene has
// no fault-injection directory, so this harness drives the clean add/commit
// roundtrip (single + deterministic delete + flush branches) to prove the
// non-faulted path is sound, then skips the fault-injection assertion.
//
// failureKind names the Java Failure variant the caller stands in for, so the
// skip message attributes the gap precisely.
func doIndexWriterOnErrorTest(t *testing.T, failureKind string) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Java pins SerialMergeScheduler to keep the test reproducible.
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	// Java commits once up front to ensure there is always a commit.
	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit() error = %v", err)
	}

	// numDocs is atLeast(2000) in Java; a fixed, smaller count keeps this
	// deterministic and fast since no fault injection forces STARTOVER.
	const numDocs = 200
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(newOnErrorDoc(t, i)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
		// Deterministic stand-in for the random single-doc delete branch.
		if i%4 == 0 {
			if err := writer.DeleteDocuments(index.NewTerm("id", strconv.Itoa(i))); err != nil {
				t.Fatalf("DeleteDocuments(%d) error = %v", i, err)
			}
		}
		// Deterministic stand-in for the random commit/flush branch.
		if i%10 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit() at doc %d error = %v", i, err)
			}
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("final Commit() error = %v", err)
	}
	writer.Close()

	if _, err := index.ReadSegmentInfos(dir); err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skipf("MockDirectoryWrapper.Failure fault injection unavailable; %s injection and rollback/checkIndex assertions deferred", failureKind)
}

// TestIndexWriterOnError_OOM ports testOOM().
//
// The Java test injects a fake OutOfMemoryError from store operations whenever
// the call stack contains IndexWriter, and verifies no index corruption ensues.
func TestIndexWriterOnError_OOM(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake OutOfMemoryError")
}

// TestIndexWriterOnError_UnknownError ports testUnknownError().
//
// The Java test injects a fake UnknownError from store operations whenever the
// call stack contains IndexWriter, and verifies no index corruption ensues.
func TestIndexWriterOnError_UnknownError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake UnknownError")
}

// TestIndexWriterOnError_LinkageError ports testLinkageError().
//
// The Java test injects a fake LinkageError from store operations whenever the
// call stack contains IndexWriter, and verifies no index corruption ensues.
func TestIndexWriterOnError_LinkageError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake LinkageError")
}

// TestIndexWriterOnError_IOError ports testIOError().
//
// The Java test injects a fake IOError from store operations whenever the call
// stack contains IndexWriter, and verifies no index corruption ensues.
func TestIndexWriterOnError_IOError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake IOError")
}

// TestIndexWriterOnError_Checkpoint ports testCheckpoint().
//
// The Java test is @Nightly: it injects a fake OutOfMemoryError specifically
// from IndexFileDeleter.checkpoint frames and verifies no index corruption.
func TestIndexWriterOnError_Checkpoint(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake OutOfMemoryError (IndexFileDeleter.checkpoint)")
}
