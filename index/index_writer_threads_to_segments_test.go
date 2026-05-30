// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Ported from org.apache.lucene.index.TestIndexWriterThreadsToSegments.
//
// Sprint 55 option c: full roundtrip where the infrastructure exists, t.Skip
// where it does not. Gocene has no NRT DirectoryReader.open(IndexWriter) path,
// so segment counts are observed after Commit() via OpenDirectoryReader(dir).

// TestIndexWriterThreadsToSegments_SegmentCountOnFlushBasic ports
// testSegmentCountOnFlushBasic. LUCENE-5644: two threads each index one doc
// (likely concurrently) for the first segment; for the second, the threads
// index at different times and should share a single thread state / segment.
func TestIndexWriterThreadsToSegments_SegmentCountOnFlushBasic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	startingGun := make(chan struct{})
	startDone := make(chan struct{}, 2)
	middleGun := make(chan struct{})
	finalGun := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			<-startingGun
			doc := &testDocument{fields: []interface{}{}}
			if err := w.AddDocument(doc); err != nil {
				t.Errorf("thread %d: AddDocument() error = %v", threadID, err)
				return
			}
			startDone <- struct{}{}

			<-middleGun
			if threadID == 0 {
				if err := w.AddDocument(doc); err != nil {
					t.Errorf("thread %d: AddDocument() error = %v", threadID, err)
				}
			} else {
				<-finalGun
				if err := w.AddDocument(doc); err != nil {
					t.Errorf("thread %d: AddDocument() error = %v", threadID, err)
				}
			}
		}(i)
	}

	close(startingGun)
	<-startDone
	<-startDone

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader() error = %v", err)
	}
	if got := r.NumDocs(); got != 2 {
		t.Errorf("NumDocs() = %d, want 2", got)
	}
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("Leaves() error = %v", err)
	}
	numSegments := len(leaves)
	// 1 segment if the threads ran sequentially, else 2:
	if numSegments > 2 {
		t.Errorf("numSegments = %d, want <= 2", numSegments)
	}
	r.Close()

	close(middleGun)
	close(finalGun)
	wg.Wait()

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	r, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader() error = %v", err)
	}
	if got := r.NumDocs(); got != 4 {
		t.Errorf("NumDocs() = %d, want 4", got)
	}
	r.Close()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// maxThreadsAtOnce is the maximum number of simultaneous threads used per
// iteration in testSegmentCountOnFlushRandom.
const maxThreadsAtOnce = 10

// TestIndexWriterThreadsToSegments_SegmentCountOnFlushRandom ports
// testSegmentCountOnFlushRandom. LUCENE-5644: index docs with multiple threads
// but, between flushes, limit how many threads may index concurrently in the
// next iteration, then verify that no more segments were flushed than threads.
func TestIndexWriterThreadsToSegments_SegmentCountOnFlushRandom(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// Never trigger flushes (so we only flush on commit):
	config.SetMaxBufferedDocs(100000000)
	config.SetRAMBufferSizeMB(-1)
	// Never trigger merges (so we can simplistically count flushed segments):
	config.SetMergePolicy(index.NewNoMergePolicy())

	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	// How many threads are indexing in the current cycle:
	var indexingCount atomic.Int32
	// How many threads we will use on each cycle:
	var maxThreadCount atomic.Int32

	oldSegmentCount := 0
	rng := rand.New(rand.NewSource(42))
	setNextIterThreadCount := func() {
		indexingCount.Store(0)
		maxThreadCount.Store(int32(rng.Intn(maxThreadsAtOnce) + 1))
	}
	setNextIterThreadCount()

	const iters = 10

	// Barrier modelling Java's CyclicBarrier with an after-barrier action: once
	// all threads reach it, the last to arrive commits, pulls a fresh reader and
	// verifies the segment count, then arms the next iteration.
	barrier := newCyclicBarrier(maxThreadsAtOnce, func() {
		if err := w.Commit(); err != nil {
			t.Errorf("Commit() error = %v", err)
			return
		}
		r, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Errorf("OpenDirectoryReader() error = %v", err)
			return
		}
		defer r.Close()
		leaves, err := r.Leaves()
		if err != nil {
			t.Errorf("Leaves() error = %v", err)
			return
		}
		// NOTE: not necessarily ==, since some threads may never have conflicted.
		maxExpectedSegments := oldSegmentCount + int(maxThreadCount.Load())
		if len(leaves) > maxExpectedSegments {
			t.Errorf("segments = %d, want <= %d", len(leaves), maxExpectedSegments)
		}
		oldSegmentCount = len(leaves)
		setNextIterThreadCount()
	})

	var wg sync.WaitGroup
	for i := 0; i < maxThreadsAtOnce; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iter := 0; iter < iters; iter++ {
				if indexingCount.Add(1) <= maxThreadCount.Load() {
					// We get to index on this cycle.
					for j := 0; j < 200; j++ {
						doc := &testDocument{fields: []interface{}{}}
						if err := w.AddDocument(doc); err != nil {
							t.Errorf("AddDocument() error = %v", err)
							return
						}
					}
				}
				barrier.await()
			}
		}()
	}

	wg.Wait()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestIndexWriterThreadsToSegments_ManyThreadsClose ports testManyThreadsClose.
// Skipped: requires RandomIndexWriter, MockAnalyzer, TestUtil.reduceOpenFiles
// and IndexWriterConfig.setCommitOnClose, none of which exist in Gocene yet.
func TestIndexWriterThreadsToSegments_ManyThreadsClose(t *testing.T) {
	t.Fatal("requires RandomIndexWriter / setCommitOnClose infrastructure (Sprint 55 option c)")
}

// TestIndexWriterThreadsToSegments_DocsStuckInRAMForever ports
// testDocsStuckInRAMForever (a @Nightly test). Skipped: requires
// SegmentInfoFormat.read, SegmentReader.docFreq and core readers, which are
// not yet wired (see SegmentReader core-readers gap).
func TestIndexWriterThreadsToSegments_DocsStuckInRAMForever(t *testing.T) {
	t.Fatal("nightly; requires SegmentInfoFormat.read + SegmentReader.docFreq (Sprint 55 option c)")
}

// cyclicBarrier is a minimal port of java.util.concurrent.CyclicBarrier with a
// barrier action: it blocks parties goroutines until all have called await(),
// runs action once on the final arrival, then releases all and rearms.
type cyclicBarrier struct {
	mu      sync.Mutex
	cond    *sync.Cond
	parties int
	count   int
	gen     uint64
	action  func()
}

func newCyclicBarrier(parties int, action func()) *cyclicBarrier {
	b := &cyclicBarrier{parties: parties, action: action}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *cyclicBarrier) await() {
	b.mu.Lock()
	defer b.mu.Unlock()
	gen := b.gen
	b.count++
	if b.count == b.parties {
		if b.action != nil {
			b.action()
		}
		b.count = 0
		b.gen++
		b.cond.Broadcast()
		return
	}
	for gen == b.gen {
		b.cond.Wait()
	}
}
