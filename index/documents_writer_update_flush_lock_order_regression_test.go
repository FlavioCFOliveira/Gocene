// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// This file pins a lock-order deadlock between DocumentsWriter.updateDocuments
// and DocumentsWriter.flushAllThreads. Lucene 10.5.0 releases the indexing DWPT
// under synchronized (flushControl) (DocumentsWriter.java:438); Gocene took the
// DocumentsWriter lock there instead. flushAllThreads (the full flush of an
// NRT getReader) holds the DocumentsWriter lock while markForFullFlush locks
// every DWPT, so an indexing thread that still held its DWPT and waited for the
// DocumentsWriter lock blocked forever against it. Found by the faithful port of
// TestIndexWriter.testRefreshAndRollbackConcurrently, whose NRT refresher runs
// the full flush; here commit() runs it, because a repeated NRT open currently
// trips a separate ReaderPool defect.

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestDocumentsWriter_UpdateAndFullFlushDoNotDeadlockRegression(t *testing.T) {
	dir := newDirectory()
	defer dir.Close()
	iwc := newIndexWriterConfig()
	// Keep flushed segments in plain files: reopening a compound segment
	// fails on a separate defect (the .cfe is not written before the reader
	// opens), which this regression does not cover.
	iwc.SetUseCompoundFile(false)
	// No merges: ConcurrentMergeScheduler.merge currently waits for every
	// merge from inside its own merge thread (a separate defect).
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := mustNewIndexWriter(t, dir, iwc)

	var stopped atomic.Bool
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() { // indexer
		defer wg.Done()
		for i := 0; !stopped.Load(); i++ {
			id := strconv.Itoa(i % 100)
			doc := document.NewDocument()
			f, err := document.NewStringField("id", id, true)
			if err != nil {
				errs <- err
				return
			}
			doc.Add(f)
			if _, err := w.UpdateDocument(index.NewTerm("id", id), doc); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() { // committer: every commit runs flushAllThreads
		defer wg.Done()
		for !stopped.Load() {
			if _, err := w.Commit(); err != nil {
				errs <- err
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	stopped.Store(true)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("updateDocument and flushAllThreads deadlocked: the DWPT must be released under the flush-control monitor")
	}
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatal(err)
	}
}
