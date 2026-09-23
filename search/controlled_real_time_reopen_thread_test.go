// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestControlledRealTimeReopenThread.java
// (Apache Lucene 10.5.0).
//
// @SuppressCodecs({"SimpleText", "Direct"}): the default codec is used.
// testCRTReopen is @AwaitsFix(LUCENE-5737) and lives in
// controlled_real_time_reopen_thread_awaitsfix_test.go behind the
// gocene_awaitsfix build tag.
//
// Thread.setName / setPriority / setDaemon have no Go counterpart and no
// observable effect on the test; they are kept as comments.

package search_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// crtNewReopenThread renders new ControlledRealTimeReopenThread<>(writer,
// manager, targetMaxStaleSec, targetMinStaleSec).
func crtNewReopenThread(t *testing.T, w *index.IndexWriter, mgr *search.SearcherManager, targetMaxStaleSec, targetMinStaleSec float64) *search.ControlledRealTimeReopenThread[*search.IndexSearcher] {
	t.Helper()
	thread, err := search.NewControlledRealTimeReopenThread(w, mgr.ReferenceManager, targetMaxStaleSec, targetMinStaleSec)
	if err != nil {
		t.Fatalf("new ControlledRealTimeReopenThread: %v", err)
	}
	return thread
}

// crtTestCase renders the fields and the overridden hooks of
// TestControlledRealTimeReopenThread as a ThreadedIndexingAndSearchingTestCase
// subclass.
type crtTestCase struct {
	t *testing.T

	// ThreadedIndexingAndSearchingTestCase members the overrides use.
	writer *index.IndexWriter

	// Not guaranteed to reflect deletes:
	nrtNoDeletes *search.SearcherManager

	// Is guaranteed to reflect deletes:
	nrtDeletes *search.SearcherManager

	genWriter *index.IndexWriter

	nrtDeletesThread   *search.ControlledRealTimeReopenThread[*search.IndexSearcher]
	nrtNoDeletesThread *search.ControlledRealTimeReopenThread[*search.IndexSearcher]

	// lastGens renders ThreadLocal<Long> lastGens: Go has no thread-local
	// storage, so each indexing goroutine owns one slot, keyed by the
	// goroutine's explicit identity.
	lastGensMu sync.Mutex
	lastGens   map[int]int64
	warmCalled atomic.Bool

	maxGenMu sync.Mutex
	maxGen   int64
}

func TestControlledRealTimeReopenThreadControlledRealTimeReopenThread(t *testing.T) {
	tc := &crtTestCase{t: t, lastGens: map[int]int64{}, maxGen: -1}
	crtRunTest(t, tc, "TestControlledRealTimeReopenThread")
}

// crtRunTest renders ThreadedIndexingAndSearchingTestCase.runTest(String),
// which drives the overridden hooks of tc.
func crtRunTest(t *testing.T, tc *crtTestCase, testName string) {
	t.Helper()
	t.Fatal(threadedIndexingAndSearchingBlocker)
}

// getFinalSearcher renders the getFinalSearcher() override.
func (tc *crtTestCase) getFinalSearcher() *search.IndexSearcher {
	if testing.Verbose() {
		tc.t.Logf("TEST: finalSearcher maxGen=%d", tc.maxGen)
	}
	tc.nrtDeletesThread.WaitForGeneration(tc.maxGen)
	return smAcquire(tc.t, tc.nrtDeletes)
}

// getDirectory renders the getDirectory(Directory) override.
func (tc *crtTestCase) getDirectory(in store.Directory) store.Directory {
	// Randomly swap in NRTCachingDir
	if random().Intn(2) == 0 {
		if testing.Verbose() {
			tc.t.Log("TEST: wrap NRTCachingDir")
		}

		return store.NewNRTCachingDirectory(in, 5.0, 60.0)
	}
	return in
}

// crtVerify renders the shared "randomly verify the change took" block of the
// update/add/delete overrides: wait for gen on thread, then assert that the
// query for id hits want documents on mgr.
func (tc *crtTestCase) crtVerify(thread *search.ControlledRealTimeReopenThread[*search.IndexSearcher], mgr *search.SearcherManager, id *index.Term, gen int64, want int64, label string) {
	t := tc.t
	thread.WaitForGeneration(gen)
	if gen > thread.GetSearchingGen() {
		t.Fatalf("gen %d > searchingGen %d", gen, thread.GetSearchingGen())
	}
	s := smAcquire(t, mgr)
	if testing.Verbose() {
		t.Logf("nrt: got %s searcher=%v", label, s)
	}
	defer smRelease(t, mgr, s)
	td, err := s.Search(search.NewTermQuery(id), 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if td.TotalHits.Value != want {
		t.Fatalf("generation: %d: totalHits=%d, want %d", gen, td.TotalHits.Value, want)
	}
}

// updateDocuments renders the updateDocuments(Term, List) override.
func (tc *crtTestCase) updateDocuments(goroutine int, id *index.Term, docs []*document.Document) {
	gen, err := tc.genWriter.UpdateDocuments(id, docs)
	if err != nil {
		tc.t.Fatalf("updateDocuments: %v", err)
	}

	// Randomly verify the update "took":
	if random().Intn(20) == 2 {
		if testing.Verbose() {
			tc.t.Logf("nrt: verify updateDocuments %v gen=%d", id, gen)
		}
		tc.crtVerify(tc.nrtDeletesThread, tc.nrtDeletes, id, gen, int64(len(docs)), "deletes")
	}

	tc.setLastGen(goroutine, gen)
}

// addDocuments renders the addDocuments(Term, List) override.
func (tc *crtTestCase) addDocuments(goroutine int, id *index.Term, docs []*document.Document) {
	gen, err := tc.genWriter.AddDocuments(docs)
	if err != nil {
		tc.t.Fatalf("addDocuments: %v", err)
	}
	// Randomly verify the add "took":
	if random().Intn(20) == 2 {
		if testing.Verbose() {
			tc.t.Logf("nrt: verify addDocuments %v gen=%d", id, gen)
		}
		tc.crtVerify(tc.nrtNoDeletesThread, tc.nrtNoDeletes, id, gen, int64(len(docs)), "noDeletes")
	}
	tc.setLastGen(goroutine, gen)
}

// addDocument renders the addDocument(Term, Iterable) override.
func (tc *crtTestCase) addDocument(goroutine int, id *index.Term, doc *document.Document) {
	gen, err := tc.genWriter.AddDocument(doc)
	if err != nil {
		tc.t.Fatalf("addDocument: %v", err)
	}

	// Randomly verify the add "took":
	if random().Intn(20) == 2 {
		if testing.Verbose() {
			tc.t.Logf("nrt: verify addDocument %v gen=%d", id, gen)
		}
		tc.crtVerify(tc.nrtNoDeletesThread, tc.nrtNoDeletes, id, gen, 1, "noDeletes")
	}
	tc.setLastGen(goroutine, gen)
}

// updateDocument renders the updateDocument(Term, Iterable) override.
func (tc *crtTestCase) updateDocument(goroutine int, id *index.Term, doc *document.Document) {
	gen, err := tc.genWriter.UpdateDocument(id, doc)
	if err != nil {
		tc.t.Fatalf("updateDocument: %v", err)
	}
	// Randomly verify the udpate "took":
	if random().Intn(20) == 2 {
		if testing.Verbose() {
			tc.t.Logf("nrt: verify updateDocument %v gen=%d", id, gen)
		}
		tc.crtVerify(tc.nrtDeletesThread, tc.nrtDeletes, id, gen, 1, "deletes")
	}
	tc.setLastGen(goroutine, gen)
}

// deleteDocuments renders the deleteDocuments(Term) override.
func (tc *crtTestCase) deleteDocuments(goroutine int, id *index.Term) {
	gen, err := tc.genWriter.DeleteDocuments([]index.Term{*id})
	if err != nil {
		tc.t.Fatalf("deleteDocuments: %v", err)
	}
	// randomly verify the delete "took":
	if random().Intn(20) == 7 {
		if testing.Verbose() {
			tc.t.Logf("nrt: verify deleteDocuments %v gen=%d", id, gen)
		}
		tc.crtVerify(tc.nrtDeletesThread, tc.nrtDeletes, id, gen, 0, "deletes")
	}
	tc.setLastGen(goroutine, gen)
}

// setLastGen renders lastGens.set(gen) for the calling indexing goroutine.
func (tc *crtTestCase) setLastGen(goroutine int, gen int64) {
	tc.lastGensMu.Lock()
	defer tc.lastGensMu.Unlock()
	tc.lastGens[goroutine] = gen
}

// crtWarmingFactory renders the anonymous SearcherFactory of doAfterWriter.
type crtWarmingFactory struct {
	tc *crtTestCase
	es search.Executor
}

// NewSearcher renders newSearcher(IndexReader r, IndexReader previous).
func (f *crtWarmingFactory) NewSearcher(r, previous index.IndexReaderInterface) (*search.IndexSearcher, error) {
	f.tc.warmCalled.Store(true)
	s := search.NewIndexSearcherWithExecutor(r, f.es)
	if _, err := s.Search(search.NewTermQuery(index.NewTerm("body", "united")), 10); err != nil {
		return nil, err
	}
	return s, nil
}

// doAfterWriter renders the doAfterWriter(ExecutorService) override.
func (tc *crtTestCase) doAfterWriter(es search.Executor) {
	t := tc.t
	minReopenSec := 0.01 + 0.05*random().Float64()
	maxReopenSec := minReopenSec * (1.0 + 10*random().Float64())

	if testing.Verbose() {
		t.Logf("TEST: make SearcherManager maxReopenSec=%v minReopenSec=%v", maxReopenSec, minReopenSec)
	}

	tc.genWriter = tc.writer

	sf := &crtWarmingFactory{tc: tc, es: es}

	tc.nrtNoDeletes = smNewSearcherManagerWithDeletes(t, tc.writer, false, false, sf)
	tc.nrtDeletes = smNewSearcherManager(t, tc.writer, sf)

	tc.nrtDeletesThread = crtNewReopenThread(t, tc.genWriter, tc.nrtDeletes, maxReopenSec, minReopenSec)
	// nrtDeletesThread.setName("NRTDeletes Reopen Thread");
	// nrtDeletesThread.setPriority(Math.min(Thread.currentThread().getPriority() + 2, Thread.MAX_PRIORITY));
	// nrtDeletesThread.setDaemon(true);
	tc.nrtDeletesThread.Start()

	tc.nrtNoDeletesThread = crtNewReopenThread(t, tc.genWriter, tc.nrtNoDeletes, maxReopenSec, minReopenSec)
	// nrtNoDeletesThread.setName("NRTNoDeletes Reopen Thread");
	// nrtNoDeletesThread.setPriority(Math.min(Thread.currentThread().getPriority() + 2, Thread.MAX_PRIORITY));
	// nrtNoDeletesThread.setDaemon(true);
	tc.nrtNoDeletesThread.Start()
}

// doAfterIndexingThreadDone renders the doAfterIndexingThreadDone() override.
func (tc *crtTestCase) doAfterIndexingThreadDone(goroutine int) {
	tc.lastGensMu.Lock()
	gen, ok := tc.lastGens[goroutine]
	tc.lastGensMu.Unlock()
	if ok {
		tc.addMaxGen(gen)
	}
}

// addMaxGen renders the synchronized addMaxGen(long).
func (tc *crtTestCase) addMaxGen(gen int64) {
	tc.maxGenMu.Lock()
	defer tc.maxGenMu.Unlock()
	tc.maxGen = max(gen, tc.maxGen)
}

// doSearching renders the doSearching(ExecutorService, int) override.
func (tc *crtTestCase) doSearching(es search.Executor, maxIterations int) {
	tc.t.Fatal(threadedIndexingAndSearchingBlocker) // runSearchThreads(maxIterations)
}

// getCurrentSearcher renders the getCurrentSearcher() override.
func (tc *crtTestCase) getCurrentSearcher() *search.IndexSearcher {
	// Test doesn't assert deletions until the end, so we
	// can randomize whether dels must be applied
	var nrt *search.SearcherManager
	if random().Intn(2) == 0 {
		nrt = tc.nrtDeletes
	} else {
		nrt = tc.nrtNoDeletes
	}

	return smAcquire(tc.t, nrt)
}

// releaseSearcher renders the releaseSearcher(IndexSearcher) override.
func (tc *crtTestCase) releaseSearcher(s *search.IndexSearcher) {
	// NOTE: a bit iffy... technically you should release
	// against the same SearcherManager you acquired from... but
	// both impls just decRef the underlying reader so we
	// can get away w/ cheating:
	smRelease(tc.t, tc.nrtNoDeletes, s)
}

// doClose renders the doClose() override.
func (tc *crtTestCase) doClose() {
	t := tc.t
	if !tc.warmCalled.Load() {
		t.Fatal("expected warmCalled")
	}
	if testing.Verbose() {
		t.Log("TEST: now close SearcherManagers")
	}
	mustClose(t, tc.nrtDeletesThread, tc.nrtDeletes, tc.nrtNoDeletesThread, tc.nrtNoDeletes)
}

// LUCENE-3528 - NRTManager hangs in certain situations
func TestControlledRealTimeReopenThreadThreadStarvationNoDeleteNRTReader(t *testing.T) {
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	d := newDirectory()
	latch := make(chan struct{})
	signal := make(chan struct{})

	writer := newLatchedIndexWriter(t, d, conf, latch, signal)
	manager := smNewSearcherManagerWithDeletes(t, writer.IndexWriter, false, false, nil)
	doc := document.NewDocument()
	doc.Add(newTextField(t, "test", "test", true))
	mustAddDocument(t, writer.IndexWriter, doc)
	smMaybeRefresh(t, manager)
	tDone := make(chan struct{})
	go func() {
		defer close(tDone)
		defer close(latch) // let the add below finish
		<-signal
		if _, err := manager.MaybeRefresh(); err != nil {
			t.Logf("%v", err) // e.printStackTrace()
			return
		}
		if _, err := writer.DeleteDocumentsQuery([]index.Query{search.NewTermQuery(index.NewTerm("foo", "barista"))}); err != nil {
			t.Logf("%v", err) // e.printStackTrace()
			return
		}
		if _, err := manager.MaybeRefresh(); err != nil { // kick off another reopen so we inc. the internal gen
			t.Logf("%v", err) // e.printStackTrace()
		}
	}()
	writer.waitAfterUpdate = true // wait in addDocument to let some reopens go through

	lastGen, err := writer.UpdateDocument(index.NewTerm("foo", "bar"), doc) // once this returns the doc is already reflected in the last reopen
	if err != nil {
		t.Fatalf("updateDocument: %v", err)
	}

	// We now eagerly resolve deletes so the manager should see it after update:
	if !smIsSearcherCurrent(t, manager) {
		t.Fatal("expected manager.isSearcherCurrent()")
	}

	searcher := smAcquire(t, manager)
	func() {
		defer smRelease(t, manager, searcher)
		assertIntEquals(t, 2, searcher.GetIndexReader().NumDocs())
	}()
	thread := crtNewReopenThread(t, writer.IndexWriter, manager, 0.01, 0.01)
	thread.Start() // start reopening
	if testing.Verbose() {
		t.Logf("waiting now for generation %d", lastGen)
	}

	var finished atomic.Bool
	waiterDone := make(chan struct{})
	go func() {
		defer close(waiterDone)
		thread.WaitForGeneration(lastGen)
		finished.Store(true)
	}()
	smMaybeRefresh(t, manager)
	select { // waiter.join(1000)
	case <-waiterDone:
	case <-time.After(1000 * time.Millisecond):
	}
	if !finished.Load() {
		// waiter.interrupt(): Go goroutines cannot be interrupted.
		t.Fatal("thread deadlocked on waitForGeneration")
	}
	mustClose(t, thread)
	thread.Join()
	<-tDone
	mustClose(t, writer.IndexWriter, manager, d)
}

// latchedIndexWriter renders the public static class LatchedIndexWriter.
type latchedIndexWriter struct {
	*index.IndexWriter

	latch           chan struct{}
	waitAfterUpdate bool
	signal          chan struct{}
	signalOnce      sync.Once
}

// newLatchedIndexWriter renders LatchedIndexWriter(Directory, IndexWriterConfig,
// CountDownLatch latch, CountDownLatch signal); a CountDownLatch(1) is a
// channel closed by countDown().
func newLatchedIndexWriter(t *testing.T, d store.Directory, conf *index.IndexWriterConfig, latch, signal chan struct{}) *latchedIndexWriter {
	t.Helper()
	return &latchedIndexWriter{IndexWriter: mustNewIndexWriter(t, d, conf), latch: latch, signal: signal}
}

// UpdateDocument renders the updateDocument(Term, Iterable) override.
func (w *latchedIndexWriter) UpdateDocument(term *index.Term, doc *document.Document) (int64, error) {
	result, err := w.IndexWriter.UpdateDocument(term, doc)
	if err != nil {
		return 0, err
	}
	if w.waitAfterUpdate {
		w.signalOnce.Do(func() { close(w.signal) })
		<-w.latch
	}
	return result, nil
}

func TestControlledRealTimeReopenThreadEvilSearcherFactory(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	mustCommit(t, w)

	other := mustOpenDirectoryReader(t, dir)

	theEvilOne := &smEvilSearcherFactory{t: t, other: other}

	// expectThrows(IllegalStateException.class,
	//     () -> new SearcherManager(w.w, false, false, theEvilOne));
	if _, err := search.NewSearcherManagerWithDeletes(w.W, false, false, theEvilOne); err == nil {
		t.Fatal("expected IllegalStateException")
	}

	mustClose(t, w, other, dir)
}

func TestControlledRealTimeReopenThreadListenerCalled(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	var afterRefreshCalled atomic.Bool
	sm := smNewSearcherManager(t, iw, search.NewSearcherFactory())
	sm.AddListener(&smAfterRefreshListener{afterRefreshCalled: &afterRefreshCalled})
	mustAddDocument(t, iw, document.NewDocument())
	mustCommit(t, iw)
	if afterRefreshCalled.Load() {
		t.Fatal("afterRefresh called before refresh")
	}
	smMaybeRefreshBlocking(t, sm)
	if !afterRefreshCalled.Load() {
		t.Fatal("afterRefresh not called")
	}
	smClose(t, sm)
	mustClose(t, iw, dir)
}

func TestControlledRealTimeReopenThreadDeleteAll(t *testing.T) {
	tc := &crtTestCase{t: t}
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mgr := smNewSearcherManager(t, w, search.NewSearcherFactory())
	tc.nrtDeletesThread = crtNewReopenThread(t, w, mgr, 0.1, 0.01)
	// nrtDeletesThread.setName("NRTDeletes Reopen Thread");
	// nrtDeletesThread.setDaemon(true);
	tc.nrtDeletesThread.Start()

	mustAddDocument(t, w, document.NewDocument())
	gen2, err := w.DeleteAll()
	if err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	tc.nrtDeletesThread.WaitForGeneration(gen2)
	// IOUtils.close(nrtDeletesThread, nrtDeletes, w, dir): nrtDeletes is null
	// in this test, and IOUtils.close skips null arguments.
	if err := crtIOUtilsClose(tc.nrtDeletesThread, tc.nrtDeletes, w, dir); err != nil {
		t.Fatal(err)
	}
}

// crtIOUtilsClose renders IOUtils.close(Closeable...): it closes every
// non-null argument and rethrows the first exception hit.
func crtIOUtilsClose(closeables ...interface{ Close() error }) error {
	var firstErr error
	for _, c := range closeables {
		if c == nil {
			continue
		}
		if sm, ok := c.(*search.SearcherManager); ok && sm == nil {
			continue
		}
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
