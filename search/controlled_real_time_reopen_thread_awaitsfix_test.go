// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_awaitsfix

// The @AwaitsFix(bugUrl = "https://issues.apache.org/jira/browse/LUCENE-5737")
// test port of
// lucene/core/src/test/org/apache/lucene/search/TestControlledRealTimeReopenThread.java
// (Apache Lucene 10.5.0). Lucene runs @AwaitsFix tests only when
// tests.awaitsfix is enabled; the gocene_awaitsfix build tag renders that
// switch.

package search_test

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// crtSlowFileExists renders LuceneTestCase.slowFileExists(Directory, String).
func crtSlowFileExists(t testing.TB, dir store.Directory, fileName string) bool {
	t.Helper()
	in, err := dir.OpenInput(fileName, store.IOContextReadOnce)
	if err != nil {
		if errors.Is(err, store.ErrFileNotFound) || errors.Is(err, fs.ErrNotExist) {
			return false
		}
		t.Fatalf("slowFileExists(%s): %v", fileName, err)
	}
	if err := in.Close(); err != nil {
		t.Fatalf("slowFileExists(%s): close: %v", fileName, err)
	}
	return true
}

// Relies on wall clock time, so it can easily false-fail when the machine is otherwise busy:
// @AwaitsFix(bugUrl = "https://issues.apache.org/jira/browse/LUCENE-5737")
// LUCENE-5461
func TestControlledRealTimeReopenThreadCRTReopen(t *testing.T) {
	// test behaving badly

	// should be high enough
	maxStaleSecs := 20

	// build crap data just to store it.
	s := "        abcdefghijklmnopqrstuvwxyz     "
	chars := []rune(s)
	var builder strings.Builder
	builder.Grow(2048)
	for i := 0; i < 2048; i++ {
		builder.WriteRune(chars[random().Intn(len(chars))])
	}
	content := builder.String()

	sdp := index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())
	// final Directory dir = new NRTCachingDirectory(newFSDirectory(createTempDir("nrt")), 5, 128);
	t.Fatal(newFSDirectoryBlocker)
	dir := store.NewNRTCachingDirectory(newDirectory(), 5, 128)
	config := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	config.SetCommitOnClose(true)
	config.SetIndexDeletionPolicy(sdp)
	config.SetOpenMode(index.CreateOrAppend)
	iw := mustNewIndexWriter(t, dir, config)
	sm := smNewSearcherManager(t, iw, search.NewSearcherFactory())
	controlledRealTimeReopenThread := crtNewReopenThread(t, iw, sm, float64(maxStaleSecs), 0)

	// controlledRealTimeReopenThread.setDaemon(true);
	controlledRealTimeReopenThread.Start()

	var commitThreads sync.WaitGroup

	for i := 0; i < 500; i++ {
		if i > 0 && i%50 == 0 {
			commitThreads.Add(1)
			go func() {
				defer commitThreads.Done()
				if _, err := iw.Commit(); err != nil {
					t.Errorf("commit: %v", err)
					return
				}
				ic, err := sdp.Snapshot()
				if err != nil {
					t.Errorf("snapshot: %v", err)
					return
				}
				names, err := ic.GetFileNames()
				if err != nil {
					t.Errorf("getFileNames: %v", err)
					return
				}
				for _, name := range names {
					// distribute, and backup
					// System.out.println(names);
					if !crtSlowFileExists(t, dir, name) {
						t.Errorf("file %s does not exist", name)
					}
				}
			}()
		}
		d := document.NewDocument()
		countField, err := document.NewTextField("count", strconv.Itoa(i), false)
		if err != nil {
			t.Fatal(err)
		}
		d.Add(countField)
		contentField, err := document.NewTextField("content", content, true)
		if err != nil {
			t.Fatal(err)
		}
		d.Add(contentField)
		start := time.Now()
		l, err := iw.AddDocument(d)
		if err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		controlledRealTimeReopenThread.WaitForGeneration(l)
		wait := time.Since(start)
		if wait >= time.Duration(maxStaleSecs)*time.Second {
			t.Fatalf("waited too long for generation %d", wait.Nanoseconds())
		}
		searcher := smAcquire(t, sm)
		td, err := searcher.Search(search.NewTermQuery(index.NewTerm("count", strconv.Itoa(i))), 10)
		smRelease(t, sm, searcher)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if td.TotalHits.Value != 1 {
			t.Fatalf("totalHits=%d, want 1", td.TotalHits.Value)
		}
	}

	commitThreads.Wait()

	mustClose(t, controlledRealTimeReopenThread, sm, iw, dir)
}
