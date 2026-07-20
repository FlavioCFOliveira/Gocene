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
// The Java suite injects a fake VM error (OOM, UnknownError, LinkageError,
// IOError) via MockDirectoryWrapper.Failure and verifies the index is not
// corrupted. Gocene's Failure callback is a plain error, so each test
// exercises the same recovery path with an injected I/O exception during a
// Commit. The clean indexing roundtrip is driven first; then the failure is
// enabled, a further Commit is attempted, and Rollback / segment-info recovery
// is verified.
package index_test

import (
	"errors"
	"strconv"
	"strings"
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
// Java @Test methods. It drives the clean add/commit roundtrip, then enables a
// MockDirectoryWrapper failure during a subsequent commit, asserts the error is
// propagated, rolls back, and verifies the existing commit remains readable.
func doIndexWriterOnErrorTest(t *testing.T, failureKind string) {
	t.Helper()

	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
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

	// Add one more document so the next commit has work to do, then inject a
	// deterministic I/O failure during that commit.
	if err := writer.AddDocument(newOnErrorDoc(t, numDocs)); err != nil {
		t.Fatalf("AddDocument before fault injection: %v", err)
	}
	dir.SetFailOnCreateOutput(true)
	err = writer.Commit()
	dir.SetFailOnCreateOutput(false)
	if err == nil {
		t.Fatalf("expected Commit to fail with injected %s", failureKind)
	}
	if !errors.Is(err, store.FakeIOException{}) && !strings.Contains(err.Error(), "simulated") {
		t.Fatalf("expected simulated I/O error for %s, got %T: %v", failureKind, err, err)
	}

	// Rollback must succeed after the failure and leave a readable prior commit.
	if err := writer.Rollback(); err != nil {
		t.Fatalf("Rollback after %s: %v", failureKind, err)
	}

	if _, err := index.ReadSegmentInfos(dir); err != nil {
		t.Fatalf("ReadSegmentInfos() after %s: %v", failureKind, err)
	}
}

// TestIndexWriterOnError_OOM ports testOOM().
func TestIndexWriterOnError_OOM(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake OutOfMemoryError")
}

// TestIndexWriterOnError_UnknownError ports testUnknownError().
func TestIndexWriterOnError_UnknownError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake UnknownError")
}

// TestIndexWriterOnError_LinkageError ports testLinkageError().
func TestIndexWriterOnError_LinkageError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake LinkageError")
}

// TestIndexWriterOnError_IOError ports testIOError().
func TestIndexWriterOnError_IOError(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake IOError")
}

// TestIndexWriterOnError_Checkpoint ports testCheckpoint().
//
// The Java test is @Nightly: it injects a fake OutOfMemoryError specifically
// from IndexFileDeleter.checkpoint frames. This Go port uses the same generic
// Commit failure path.
func TestIndexWriterOnError_Checkpoint(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake OutOfMemoryError (IndexFileDeleter.checkpoint)")
}
