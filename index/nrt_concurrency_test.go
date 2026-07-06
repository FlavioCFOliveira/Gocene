// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Concurrent near-real-time reader tests.
//
// These tests exercise IndexWriter.GetReader, DirectoryReader.Open(writer)
// (via index.OpenDirectoryReaderFromWriter) and OpenIfChangedFromWriter under
// concurrent load. They mirror the intent of Lucene's TestNRTReaderWithThreads
// and TestNRTThreads without requiring MockDirectoryWrapper or
// RandomIndexWriter.

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// nrtWriterConfig returns a small-RAM configuration so that buffered documents
// are flushed frequently, exercising the NRT materialisation path.
func nrtWriterConfig(t *testing.T) *index.IndexWriterConfig {
	t.Helper()
	cfg := index.NewIndexWriterConfig(createTestAnalyzer())
	cfg.SetMaxBufferedDocs(2)
	return cfg
}

// nrtAddIntDoc indexes a single document whose id field is the decimal
// representation of the supplied integer. The error is returned so that
// goroutines can report failures without calling t.Fatal from a non-test
// goroutine.
func nrtAddIntDoc(w *index.IndexWriter, id int) error {
	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", strconv.Itoa(id), true)
	doc.Add(idField)
	textField, _ := document.NewTextField("content", fmt.Sprintf("value %d", id), false)
	doc.Add(textField)
	return w.AddDocument(doc)
}

// TestNRTConcurrentIndexingAndSearching verifies that reader goroutines can
// repeatedly open NRT readers while writer goroutines add documents.
func TestNRTConcurrentIndexingAndSearching(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	const (
		numIndexers    = 2
		numSearchers   = 2
		docsPerIndexer = 20
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	var maxSeen atomic.Int64
	var addFailures atomic.Int64

	for i := 0; i < numIndexers; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			<-start
			for j := 0; j < docsPerIndexer; j++ {
				if err := nrtAddIntDoc(writer, offset*docsPerIndexer+j); err != nil {
					addFailures.Add(1)
					t.Errorf("AddDocument: %v", err)
					return
				}
			}
		}(i)
	}

	var searchWg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < numSearchers; i++ {
		searchWg.Add(1)
		go func() {
			defer searchWg.Done()
			<-start
			for {
				select {
				case <-stop:
					return
				default:
				}
				reader, err := index.OpenDirectoryReaderFromWriter(writer)
				if err != nil {
					t.Errorf("OpenDirectoryReaderFromWriter: %v", err)
					return
				}
				n := int64(reader.NumDocs())
				reader.Close()
				if n < 0 {
					t.Errorf("NRT reader reported negative NumDocs: %d", n)
					return
				}
				for {
					cur := maxSeen.Load()
					if n <= cur || maxSeen.CompareAndSwap(cur, n) {
						break
					}
				}
			}
		}()
	}

	close(start)
	wg.Wait()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	searchWg.Wait()

	if addFailures.Load() > 0 {
		t.Fatalf("indexing produced %d failures", addFailures.Load())
	}

	finalReader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("final NRT reader: %v", err)
	}
	defer finalReader.Close()
	if got, want := finalReader.NumDocs(), numIndexers*docsPerIndexer; got != want {
		t.Fatalf("final NRT NumDocs = %d, want %d", got, want)
	}
	if maxSeen.Load() == 0 {
		t.Fatal("searchers never observed any documents")
	}
	if maxSeen.Load() < int64(numIndexers*docsPerIndexer) {
		t.Fatalf("searchers only observed up to %d docs, want at least %d", maxSeen.Load(), numIndexers*docsPerIndexer)
	}
}

// TestNRTMultipleReaders verifies that each NRT reader captures the index state
// at the moment it was opened.
func TestNRTMultipleReaders(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	if err := nrtAddIntDoc(writer, 1); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	r1, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("first NRT reader: %v", err)
	}
	defer r1.Close()
	if got, want := r1.NumDocs(), 1; got != want {
		t.Fatalf("r1.NumDocs() = %d, want %d", got, want)
	}

	if err := nrtAddIntDoc(writer, 2); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	r2, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("second NRT reader: %v", err)
	}
	defer r2.Close()
	if got, want := r2.NumDocs(), 2; got != want {
		t.Fatalf("r2.NumDocs() = %d, want %d", got, want)
	}

	if err := nrtAddIntDoc(writer, 3); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	r3, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("third NRT reader: %v", err)
	}
	defer r3.Close()
	if got, want := r3.NumDocs(), 3; got != want {
		t.Fatalf("r3.NumDocs() = %d, want %d", got, want)
	}

	// Earlier readers remain stable.
	if got, want := r1.NumDocs(), 1; got != want {
		t.Fatalf("stale r1.NumDocs() = %d, want %d", got, want)
	}
	if got, want := r2.NumDocs(), 2; got != want {
		t.Fatalf("stale r2.NumDocs() = %d, want %d", got, want)
	}
}

// TestNRTRaceConditionReopen exercises concurrent reopen calls against a live
// writer.
func TestNRTRaceConditionReopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 5; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	const numReopeners = 4
	const iterations = 25
	var wg sync.WaitGroup
	var failures atomic.Int64
	var nonNilReopens atomic.Int64

	for i := 0; i < numReopeners; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				r, err := index.OpenIfChangedFromWriter(reader, writer)
				if err != nil {
					failures.Add(1)
					t.Errorf("reopener %d: OpenIfChangedFromWriter: %v", id, err)
					return
				}
				if r != nil {
					nonNilReopens.Add(1)
					r.Close()
				}
			}
		}(i)
	}

	// Writer keeps mutating the index while reopens race.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numReopeners*iterations/2; j++ {
			if err := nrtAddIntDoc(writer, 100+j); err != nil {
				t.Errorf("AddDocument: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	if failures.Load() > 0 {
		t.Fatalf("concurrent reopen produced %d failures", failures.Load())
	}

	// Because goroutine scheduling can serialize the racers on a single CPU,
	// require only that a post-race reopen sees the writer's uncommitted docs.
	finalReader, err := index.OpenIfChangedFromWriter(reader, writer)
	if err != nil {
		t.Fatalf("post-race OpenIfChangedFromWriter: %v", err)
	}
	if finalReader == nil {
		t.Fatal("post-race OpenIfChangedFromWriter returned nil despite uncommitted docs")
	}
	defer finalReader.Close()
	if got := finalReader.NumDocs(); got <= 5 {
		t.Fatalf("post-race reader.NumDocs() = %d, want > 5", got)
	}
}

// TestNRTConcurrentDeletesAndReads verifies that deletes applied through the
// writer are visible to concurrently-opened NRT readers.
func TestNRTConcurrentDeletesAndReads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	const numDocs = 50
	for i := 0; i < numDocs; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var wg sync.WaitGroup
	var minLive atomic.Int64
	minLive.Store(numDocs)

	// Deleter goroutine: delete roughly half of the documents.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numDocs/2; i++ {
			if err := writer.DeleteDocuments(index.NewTerm("id", strconv.Itoa(i))); err != nil {
				t.Errorf("DeleteDocuments(%d): %v", i, err)
				return
			}
		}
	}()

	// Reader goroutine: open NRT readers and track the smallest live count seen.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			reader, err := index.OpenDirectoryReaderFromWriter(writer)
			if err != nil {
				t.Errorf("OpenDirectoryReaderFromWriter: %v", err)
				return
			}
			n := int64(reader.NumDocs())
			reader.Close()
			if n < 0 || n > numDocs {
				t.Errorf("NRT NumDocs out of range: %d", n)
				return
			}
			for {
				cur := minLive.Load()
				if n >= cur || minLive.CompareAndSwap(cur, n) {
					break
				}
			}
		}
	}()

	wg.Wait()

	finalReader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("final NRT reader: %v", err)
	}
	defer finalReader.Close()
	if got, want := finalReader.NumDocs(), numDocs/2; got != want {
		t.Fatalf("final NRT NumDocs = %d, want %d", got, want)
	}
	if minLive.Load() == numDocs {
		t.Fatal("reader never observed any deletes")
	}
}

// TestNRTWriterReaderConsistency checks that the NRT reader agrees with the
// index state returned by a freshly opened NRT reader after every operation.
func TestNRTWriterReaderConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("iteration %d: AddDocument: %v", i, err)
		}
		reader, err := index.OpenDirectoryReaderFromWriter(writer)
		if err != nil {
			t.Fatalf("iteration %d: OpenDirectoryReaderFromWriter: %v", i, err)
		}
		if got, want := reader.NumDocs(), i+1; got != want {
			reader.Close()
			t.Fatalf("iteration %d: reader.NumDocs()=%d, want %d", i, got, want)
		}
		reader.Close()
	}
}

// TestNRTConcurrentReopenAndCommit interleaves commits with concurrent NRT
// reopens.
func TestNRTConcurrentReopenAndCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	const iterations = 10
	var wg sync.WaitGroup
	var failures atomic.Int64

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if err := nrtAddIntDoc(writer, i); err != nil {
				t.Errorf("AddDocument(%d): %v", i, err)
				return
			}
			if err := writer.Commit(); err != nil {
				t.Errorf("Commit(%d): %v", i, err)
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations*3; i++ {
			r, err := index.OpenIfChangedFromWriter(reader, writer)
			if err != nil {
				failures.Add(1)
				t.Errorf("OpenIfChangedFromWriter: %v", err)
				return
			}
			if r != nil {
				r.Close()
			}
		}
	}()

	wg.Wait()
	if failures.Load() > 0 {
		t.Fatalf("concurrent reopen/commit produced %d failures", failures.Load())
	}

	final, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("final OpenDirectoryReader: %v", err)
	}
	defer final.Close()
	if got, want := final.NumDocs(), iterations; got != want {
		t.Fatalf("final committed NumDocs = %d, want %d", got, want)
	}
}

// TestNRTGoroutineLeak ensures that repeated GetReader / Close cycles do not
// leak goroutines.
func TestNRTGoroutineLeak(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 5; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	// Warm up and let the runtime settle.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	for i := 0; i < 30; i++ {
		if err := nrtAddIntDoc(writer, 100+i); err != nil {
			t.Fatalf("iteration %d: AddDocument: %v", i, err)
		}
		reader, err := index.OpenDirectoryReaderFromWriter(writer)
		if err != nil {
			t.Fatalf("iteration %d: OpenDirectoryReaderFromWriter: %v", i, err)
		}
		_ = reader.NumDocs()
		reader.Close()
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Tolerate small runtime fluctuations; fail only on a large persistent leak.
	if after-before > 10 {
		t.Fatalf("possible goroutine leak: before=%d after=%d delta=%d", before, after, after-before)
	}
}

// TestNRTAtomicVisibility verifies that an NRT reader never sees a partial
// batch of documents: every document added before the reader was opened is
// visible, and no document added afterwards is visible.
func TestNRTAtomicVisibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, nrtWriterConfig(t))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 5; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()
	if got, want := reader.NumDocs(), 5; got != want {
		t.Fatalf("reader.NumDocs() = %d, want %d", got, want)
	}

	for i := 5; i < 10; i++ {
		if err := nrtAddIntDoc(writer, i); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// The original reader must remain at 5 docs.
	if got, want := reader.NumDocs(), 5; got != want {
		t.Fatalf("stale reader.NumDocs() = %d, want %d", got, want)
	}

	reader2, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("second NRT reader: %v", err)
	}
	defer reader2.Close()
	if got, want := reader2.NumDocs(), 10; got != want {
		t.Fatalf("fresh reader.NumDocs() = %d, want %d", got, want)
	}
}
