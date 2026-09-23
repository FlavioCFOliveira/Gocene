// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_awaitsfix

// The @AwaitsFix(bugUrl = "https://github.com/apache/lucene/issues/14483")
// test ports of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMergePolicy.java
// (Apache Lucene 10.5.0). Lucene runs @AwaitsFix tests only when
// tests.awaitsfix is enabled; the gocene_awaitsfix build tag renders that
// switch.

package index_test

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TODO: tests using stressUpdateSameDocumentWithMergeOnX have resource issues
func TestIndexWriterMergePolicyStressUpdateSameDocumentWithMergeOnGetReader(t *testing.T) {
	stressUpdateSameDocumentWithMergeOnX(t, true)
}

// TODO: tests using stressUpdateSameDocumentWithMergeOnX have resource issues
func TestIndexWriterMergePolicyStressUpdateSameDocumentWithMergeOnCommit(t *testing.T) {
	stressUpdateSameDocumentWithMergeOnX(t, false)
}

func assertTermHits(t *testing.T, r index.IndexReaderInterface, expected int64) {
	t.Helper()
	topDocs, err := search.NewIndexSearcher(r).Search(search.NewTermQuery(index.NewTerm("id", "1")), 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if topDocs.TotalHits.Value != expected {
		t.Fatalf("totalHits.value(): expected %d, got %d", expected, topDocs.TotalHits.Value)
	}
}

func stressUpdateSameDocumentWithMergeOnX(t *testing.T, useGetReader bool) {
	directory := newDirectory()
	defer mustClose(t, directory)
	trigger := index.MergeTriggerCommit
	if useGetReader {
		trigger = index.MergeTriggerGetReader
	}
	conf := newIndexWriterConfig()
	conf.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), trigger))
	conf.SetMaxFullFlushMergeWaitMillis(int64(10 + rand.Intn(2000)))
	conf.SetSoftDeletesField("soft_delete")
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	writer := newRandomIndexWriterWithConfig(t, directory, conf)
	defer mustClose(t, writer)

	d1 := document.NewDocument()
	d1.Add(newStringField(t, "id", "1", false))
	if _, err := writer.UpdateDocument(index.NewTerm("id", "1"), d1); err != nil {
		t.Fatalf("updateDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	itersBound, flushesBound := 1000, 100
	if testNightly {
		itersBound, flushesBound = 5000, 500
	}
	var iters atomic.Int32
	iters.Store(int32(100 + rand.Intn(itersBound)))
	var numFullFlushes atomic.Int32
	numFullFlushes.Store(int32(10 + rand.Intn(flushesBound)))
	var done atomic.Bool
	var wg sync.WaitGroup
	numThreads := 1 + rand.Intn(4)
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer done.Store(true)
			for iters.Add(-1) > 0 || numFullFlushes.Load() > 0 {
				if _, err := writer.UpdateDocument(index.NewTerm("id", "1"), d1); err != nil {
					t.Errorf("updateDocument: %v", err)
					return
				}
				if rand.Intn(2) == 0 {
					if _, err := writer.AddDocument(document.NewDocument()); err != nil {
						t.Errorf("addDocument: %v", err)
						return
					}
				}
			}
		}()
	}
	defer func() {
		numFullFlushes.Store(0)
		wg.Wait()
	}()
	for !done.Load() {
		if useGetReader {
			reader, err := writer.GetReader()
			if err != nil {
				t.Fatalf("getReader: %v", err)
			}
			assertTermHits(t, reader, 1)
			mustClose(t, reader)
		} else {
			if rand.Intn(2) == 0 {
				if _, err := writer.Commit(); err != nil {
					t.Fatalf("commit: %v", err)
				}
			}
			delegate := mustOpenDirectoryReader(t, directory)
			open, err := index.NewSoftDeletesDirectoryReaderWrapper(delegate, "___soft_deletes")
			if err != nil {
				t.Fatalf("SoftDeletesDirectoryReaderWrapper: %v", err)
			}
			assertTermHits(t, open, 1)
			mustClose(t, open, delegate)
		}
		numFullFlushes.Add(-1)
	}
}
