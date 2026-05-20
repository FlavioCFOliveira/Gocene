// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter exception handling.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterExceptions2
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterExceptions2.java
//
// GOC-4197: Port test org.apache.lucene.index.TestIndexWriterExceptions2 (Sprint 55).
//
// Port strategy (Sprint 55 option c): the single Java @Test (testBasics) is a
// fault-injection harness. It deliberately provokes non-aborting exceptions
// (CrankyTokenFilter) and aborting exceptions (CrankyCodec), then asserts that
// the writer either recovers or leaves a non-corrupt index. The Go counterpart
// keeps a 1:1 mapping: the document build and add/commit roundtrip run for real
// wherever the Gocene API supports it; the fault-injection assertions are gated
// with t.Skip so the divergence is explicit rather than silently absent.
//
// Known API gaps that force a skip in this file:
//   - CrankyTokenFilter (random "Fake IOException" from a TokenStream) does not
//     exist, so non-aborting analyzer exceptions cannot be provoked.
//   - CrankyCodec / AssertingCodec (random "Fake IOException" from codec writes)
//     do not exist, so aborting codec exceptions cannot be provoked.
//   - IndexWriter.IsDeleterClosed is not implemented, so the post-abort
//     "deleter closed" assertion cannot be checked.
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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// newExceptions2Doc builds the per-iteration document mirroring the `doc` local
// of the Java testBasics loop: an id, the five doc-values flavors, text fields
// (including payload and term-vector variants), stored fields, and points.
func newExceptions2Doc(t *testing.T, id int) *document.Document {
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

	_ = util.NewBytesRef // keep util import meaningful if helpers shift
	return doc
}

// TestIndexWriterExceptions2_Basics ports testBasics().
//
// The Java test installs a CrankyTokenFilter (non-aborting "Fake IOException")
// and a CrankyCodec (aborting "Fake IOException"), drives ~100 docs with
// per-iteration single/block adds, deletes, doc-values updates, flushes and
// checkIndex passes, and verifies that no provoked exception ever corrupts the
// index. None of the cranky fault-injection components exist in Gocene, so the
// add/commit roundtrip runs for real (proving the non-faulted path is sound)
// and the fault-injection assertions are skipped.
func TestIndexWriterExceptions2_Basics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Java pins SerialMergeScheduler to keep the test reproducible.
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	// numDocs is atLeast(100) in Java; a fixed count keeps this deterministic.
	const numDocs = 100
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(newExceptions2Doc(t, i)); err != nil {
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

	t.Skip("CrankyCodec/CrankyTokenFilter fault injection unavailable; aborting/non-aborting exception assertions deferred")
}
