// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnDiskFull.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestIndexWriterOnDiskFullAddDocumentOnDiskFull(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		doAbort := pass == 1
		diskFree := int64(nextInt(100, 300))
		indexExists := false
		for {
			dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
			dir.SetMaxSizeInBytes(diskFree)
			writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
			if _, ok := writer.GetConfig().GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
				// This test intentionally produces exceptions in the threads that
				// CMS launches; Java calls setSuppressExceptions() here.
				t.Error("org.apache.lucene.index.ConcurrentMergeScheduler#setSuppressExceptions() is not ported")
			}

			hitError := false
			for i := 0; i < 200; i++ {
				if err := onDiskFullAddDoc(t, writer); err != nil {
					hitError = true
					break
				}
			}
			if !hitError {
				// when calling commit(), if the writer is asynchronously closed by a
				// fatal tragedy (e.g. from disk-full-on-merge with CMS), then we may
				// receive either AlreadyClosedException OR IllegalStateException,
				// depending on when it happens.
				if _, err := writer.Commit(); err != nil {
					hitError = true
				} else {
					indexExists = true
				}
			}

			if hitError {
				if doAbort {
					if err := writer.Rollback(); err != nil {
						t.Fatalf("rollback: %v", err)
					}
				} else if err := writer.Close(); err != nil {
					dir.SetMaxSizeInBytes(0)
					if err := writer.Close(); err != nil {
						var ace *store.AlreadyClosedException
						if !errors.As(err, &ace) {
							t.Fatalf("close: %v", err)
						}
						// OK
					}
				}

				if indexExists {
					// Make sure reader can open the index:
					mustClose(t, mustOpenDirectoryReader(t, dir))
				}

				mustClose(t, dir)
				// Now try again w/ more space:
				diskFree += int64(nextInt(3000, 5000))
			} else {
				dir.SetMaxSizeInBytes(0)
				mustClose(t, writer, dir)
				break
			}
		}
	}
}

// TestIndexWriterOnDiskFullAddIndexOnDiskFull ports testAddIndexOnDiskFull,
// which uses TestUtil.addIndexesSlowly, TestUtil.syncConcurrentMerges and
// TestUtil.checkIndex.
func TestIndexWriterOnDiskFullAddIndexOnDiskFull(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#addIndexesSlowly, TestUtil#syncConcurrentMerges " +
		"and TestUtil#checkIndex are not ported")
}

// TestIndexWriterOnDiskFullCorruptionAfterDiskFullDuringMerge ports
// testCorruptionAfterDiskFullDuringMerge, which verifies the index with
// TestUtil.checkIndex.
func TestIndexWriterOnDiskFullCorruptionAfterDiskFullDuringMerge(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#checkIndex(Directory) is not ported")
}

// TestIndexWriterOnDiskFullImmediateDiskFull (LUCENE-1130) makes sure
// immediate disk full on creating an IndexWriter (hit during
// DWPT#updateDocuments()) is OK.
func TestIndexWriterOnDiskFullImmediateDiskFull(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(2)
	iwc.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	iwc.SetCommitOnClose(false)
	writer := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, writer) // empty commit, to not create confusing situation with first commit
	size, err := dir.SizeInBytes()
	if err != nil {
		t.Fatalf("sizeInBytes: %v", err)
	}
	dir.SetMaxSizeInBytes(max(1, size))
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	doc.Add(newField(t, "field", "aaa bbb ccc ddd eee fff ggg hhh iii jjj", customType))
	if _, err := writer.AddDocument(doc); err == nil {
		t.Fatal("expected IOException")
	}
	if !writer.IsClosed() {
		t.Fatal("assertTrue(writer.isClosed())")
	}
	mustClose(t, dir)
	// assertTrue(writer.isDeleterClosed())
	t.Fatal("org.apache.lucene.index.IndexWriter#isDeleterClosed() is not ported")
}

// onDiskFullAddDoc ports the private addDoc(IndexWriter).
func onDiskFullAddDoc(t *testing.T, writer *index.IndexWriter) error {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	dv, err := document.NewNumericDocValuesField("numericdv", 1)
	if err != nil {
		t.Fatalf("new NumericDocValuesField: %v", err)
	}
	doc.Add(dv)
	doc.Add(document.NewIntPoint("point", 1))
	doc.Add(document.NewIntPoint("point2d", 1, 1))
	_, err = writer.AddDocument(doc)
	return err
}
