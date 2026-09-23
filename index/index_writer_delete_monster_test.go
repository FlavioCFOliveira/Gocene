// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Monster and @Nightly test ports of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterDelete.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs them only when monster/nightly tests are enabled.

package index_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Verify that we can call deleteAll repeatedly without leaking field numbers
// such that we trigger OOME on creation of FieldInfos. See
// https://issues.apache.org/jira/browse/LUCENE-9617
// @Monster("Takes 1-2 minutes but writes tons of files to disk.")
func TestIndexWriterDeleteDeleteAllRepeated(t *testing.T) {
	const breakingFieldCount = 50_000_000
	dir, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("FSDirectory.open: %v", err)
	}
	defer mustClose(t, dir)
	// Avoid flushing until the end of the test to save time.
	conf := newIndexWriterConfig()
	conf.SetMaxBufferedDocs(1000)
	conf.SetRAMBufferSizeMB(1000)
	conf.SetRAMPerThreadHardLimitMB(1000)
	conf.SetCheckPendingFlushUpdate(false)
	modifier := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, modifier)
	doc := document.NewDocument()
	const fieldsPerDoc = 1_000
	for i := 0; i < fieldsPerDoc; i++ {
		f, err := document.NewStoredField("field"+strconv.Itoa(i), "")
		if err != nil {
			t.Fatalf("StoredField: %v", err)
		}
		doc.Add(f)
	}
	var numFields atomic.Int64
	var wg sync.WaitGroup
	nThreads := atLeast(8)
	for i := 0; i < nThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for numFields.Add(fieldsPerDoc)-fieldsPerDoc < breakingFieldCount {
				if _, err := modifier.AddDocument(doc); err != nil {
					t.Errorf("addDocument: %v", err)
					return
				}
				if _, err := modifier.DeleteAll(); err != nil {
					t.Errorf("deleteAll: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	// Add one last document and flush to build FieldInfos.
	mustAddDocument(t, modifier, doc)
	mustFlush(t, modifier)
}

// TODO: can we fix MockDirectoryWrapper disk full checking to be more
// efficient (not recompute on every write)?
func TestIndexWriterDeleteDeletesOnDiskFull(t *testing.T) {
	indexWriterDeleteDoTestOperationsOnDiskFull(t, false)
}

// TODO: can we fix MockDirectoryWrapper disk full checking to be more
// efficient (not recompute on every write)?
func TestIndexWriterDeleteUpdatesOnDiskFull(t *testing.T) {
	indexWriterDeleteDoTestOperationsOnDiskFull(t, true)
}

// indexWriterDeleteDoTestOperationsOnDiskFull renders the private
// doTestOperationsOnDiskFull(boolean): make sure if modifier tries to commit
// but hits disk full that modifier remains consistent and usable. Its first
// verification reads through TestUtil.checkIndex and newSearcher.
func indexWriterDeleteDoTestOperationsOnDiskFull(t *testing.T, updates bool) {
	t.Helper()
	// First build up a starting index:
	startDir := newDirectory()

	writer := mustNewIndexWriter(t, startDir, newIndexWriterConfigWithAnalyzer(newWhitespaceMockAnalyzerLower(false)))
	for i := 0; i < 157; i++ {
		d := document.NewDocument()
		d.Add(newStringField(t, "id", strconv.Itoa(i), true))
		d.Add(newTextField(t, "content", "aaa "+strconv.Itoa(i), false))
		d.Add(numericDVField(t, "dv", int64(i)))
		mustAddDocument(t, writer, d)
	}
	mustClose(t, writer)
	defer mustClose(t, startDir)
	t.Fatal(testUtilCheckIndexMissing + "; " +
		"org.apache.lucene.tests.search.AssertingIndexSearcher (built by LuceneTestCase.newSearcher(IndexReader)) is not ported")
}

// TODO: this test can hit pathological cases (IW settings?) where it runs
// for far too long
func TestIndexWriterDeleteIndexingThenDeleting(t *testing.T) {
	t.Fatal("TestUtil.getPostingsFormat(String) and org.apache.lucene.index.IndexWriter#getFlushCount() are not ported")
}

// Make sure buffered (pushed) deletes don't use up so
// much RAM that it forces long tail of tiny segments:
func TestIndexWriterDeleteApplyDeletesOnFlush(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal("overriding org.apache.lucene.index.IndexWriter#doAfterFlush() in an IndexWriter subclass is not ported")
}
