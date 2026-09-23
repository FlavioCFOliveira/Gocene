// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// This file pins the deadlock of IndexWriter.GetReader: it took fullFlushLock
// and then called doFlush, which took fullFlushLock again. Lucene 10.5.0 guards
// the full flush of getReader with synchronized (fullFlushLock), a reentrant
// monitor (IndexWriter.java:578); Gocene's fullFlushLock is a non-reentrant
// sync.Mutex, so every DirectoryReader.open(IndexWriter) blocked forever.

import (
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestIndexWriter_GetReaderDoesNotDeadlockRegression(t *testing.T) {
	dir := newDirectory()
	defer dir.Close()
	iwc := newIndexWriterConfig()
	// Keep the flushed segment in plain files: the NRT open of a compound
	// segment fails on a separate defect (the .cfe is not written before the
	// reader opens), which this regression does not cover.
	iwc.SetUseCompoundFile(false)
	w := mustNewIndexWriter(t, dir, iwc)
	defer w.Close()
	doc := document.NewDocument()
	f, err := document.NewStoredField("id", "1")
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f)
	mustAddDocument(t, w, doc)

	type result struct {
		r   *index.DirectoryReader
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, err := index.OpenDirectoryReaderFromWriter(w)
		done <- result{r, err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("OpenDirectoryReaderFromWriter: %v", res.err)
		}
		if err := res.r.Close(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("OpenDirectoryReaderFromWriter blocked: IndexWriter.GetReader re-acquired fullFlushLock")
	}
}
