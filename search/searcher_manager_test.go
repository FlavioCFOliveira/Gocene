// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestSearcherManager.java
// (Apache Lucene 10.5.0).
//
// @SuppressCodecs({"SimpleText", "Direct"}): the default codec is used.
//
// Gocene's SearcherManager, SearcherFactory, ReferenceManager and
// SearcherLifetimeManager do not render the Lucene API the test drives: the
// members that have no Gocene counterpart are rendered as helpers that fail
// the test naming the missing member, and the test body is kept complete.

package search_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

const (
	// threadedIndexingAndSearchingBlocker names the base class of TestSearcherManager.
	threadedIndexingAndSearchingBlocker = "requires org.apache.lucene.tests.index.ThreadedIndexingAndSearchingTestCase (not ported)"
	// searcherLifetimeManagerBlocker names the SearcherLifetimeManager members the test uses.
	searcherLifetimeManagerBlocker = "requires org.apache.lucene.search.SearcherLifetimeManager() with record(IndexSearcher), " +
		"acquire(long), prune(Pruner), close() and SearcherLifetimeManager.PruneByAge (Gocene's " +
		"SearcherLifetimeManager(manager, maxAge, maxSearchers) is not the Lucene class)"
	// searcherFactoryPreviousReaderBlocker names the overridable SearcherFactory member.
	searcherFactoryPreviousReaderBlocker = "requires org.apache.lucene.search.SearcherFactory.newSearcher(IndexReader, " +
		"IndexReader previousReader) (Gocene's SearcherFactory.NewSearcher(ctx, reader) drops previousReader)"
	// maybeRefreshBlockingBlocker names ReferenceManager.maybeRefreshBlocking().
	maybeRefreshBlockingBlocker = "requires org.apache.lucene.search.ReferenceManager.maybeRefreshBlocking() (not ported)"
	// refreshListenerBlocker names ReferenceManager.addListener(RefreshListener).
	refreshListenerBlocker = "requires org.apache.lucene.search.ReferenceManager.addListener(ReferenceManager.RefreshListener) " +
		"with afterRefresh(boolean didRefresh) (Gocene's RefreshListener.AfterRefresh(generation) is not the Lucene interface)"
	// newFSDirectoryBlocker names LuceneTestCase.newFSDirectory(Path) and createTempDir().
	newFSDirectoryBlocker = "requires LuceneTestCase.newFSDirectory(Path) and LuceneTestCase.createTempDir() (not ported)"
)

// smNewSearcherManager renders new SearcherManager(IndexWriter, SearcherFactory).
func smNewSearcherManager(t *testing.T, w *index.IndexWriter, factory search.SearcherFactory) *search.SearcherManager {
	t.Helper()
	sm, err := search.NewSearcherManager(w, factory)
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	return sm
}

// smNewSearcherManagerWithOptions renders new SearcherManager(IndexWriter,
// boolean applyAllDeletes, boolean writeAllDeletes, SearcherFactory).
func smNewSearcherManagerWithOptions(t *testing.T, w *index.IndexWriter, applyAllDeletes, writeAllDeletes bool, factory search.SearcherFactory) *search.SearcherManager {
	t.Helper()
	sm, err := search.NewSearcherManagerWithOptions(w, applyAllDeletes, writeAllDeletes, factory, nil)
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	return sm
}

// smNewSearcherManagerFromDir renders new SearcherManager(Directory, SearcherFactory).
func smNewSearcherManagerFromDir(t *testing.T, dir store.Directory, factory search.SearcherFactory) *search.SearcherManager {
	t.Helper()
	sm, err := search.NewSearcherManagerFromDir(dir, factory)
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	return sm
}

// smAcquire renders ReferenceManager.acquire().
func smAcquire(t *testing.T, sm *search.SearcherManager) *search.IndexSearcher {
	t.Helper()
	s, err := sm.Acquire()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	return s
}

// smRelease renders ReferenceManager.release(G).
func smRelease(t *testing.T, sm *search.SearcherManager, s *search.IndexSearcher) {
	t.Helper()
	if err := sm.Release(s); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// smMaybeRefresh renders ReferenceManager.maybeRefresh().
func smMaybeRefresh(t *testing.T, sm *search.SearcherManager) bool {
	t.Helper()
	refreshed, err := sm.MaybeRefresh()
	if err != nil {
		t.Fatalf("maybeRefresh: %v", err)
	}
	return refreshed
}

// smMaybeRefreshBlocking renders ReferenceManager.maybeRefreshBlocking().
func smMaybeRefreshBlocking(t *testing.T, sm *search.SearcherManager) {
	t.Helper()
	t.Fatal(maybeRefreshBlockingBlocker)
}

// smIsSearcherCurrent renders SearcherManager.isSearcherCurrent().
func smIsSearcherCurrent(t *testing.T, sm *search.SearcherManager) bool {
	t.Helper()
	current, err := sm.IsSearcherCurrent()
	if err != nil {
		t.Fatalf("isSearcherCurrent: %v", err)
	}
	return current
}

// smClose renders ReferenceManager.close().
func smClose(t *testing.T, sm *search.SearcherManager) {
	t.Helper()
	if err := sm.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// expectAlreadyClosed renders expectThrows(AlreadyClosedException.class, ...).
func expectAlreadyClosed(t *testing.T, err error) {
	t.Helper()
	var ace *store.AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Fatalf("expected AlreadyClosedException, got %v", err)
	}
}

// smWarmingFactory renders the anonymous SearcherFactory of doAfterWriter.
type smWarmingFactory struct {
	t  *testing.T
	tc *searcherManagerTestCase
	es search.Executor
}

// NewSearcher renders newSearcher(IndexReader r, IndexReader previous).
func (f *smWarmingFactory) NewSearcher(ctx context.Context, r index.IndexReaderInterface) (*search.IndexSearcher, error) {
	f.t.Fatal(searcherFactoryPreviousReaderBlocker)
	s := search.NewIndexSearcherWithExecutor(r, f.es)
	f.tc.warmCalled = true
	if _, err := s.Search(search.NewTermQuery(index.NewTerm("body", "united")), 10); err != nil {
		return nil, err
	}
	return s, nil
}

// searcherManagerTestCase renders the fields and the overridden hooks of
// TestSearcherManager as a ThreadedIndexingAndSearchingTestCase subclass.
type searcherManagerTestCase struct {
	t *testing.T

	// ThreadedIndexingAndSearchingTestCase members the overrides use.
	writer                     *index.IndexWriter
	dir                        store.Directory
	failed                     atomic.Bool
	assertMergedSegmentsWarmed bool

	warmCalled    bool
	mgr           *search.SearcherManager
	pastSearchers []int64
	pastMu        sync.Mutex
	isNRT         bool
}

func TestSearcherManagerSearcherManager(t *testing.T) {
	// pruner = new SearcherLifetimeManager.PruneByAge(TEST_NIGHTLY ? TestUtil.nextInt(random(), 1, 10) : 1);
	t.Fatal(searcherLifetimeManagerBlocker)
	smRunTest(t, &searcherManagerTestCase{t: t}, "TestSearcherManager")
}

// smRunTest renders ThreadedIndexingAndSearchingTestCase.runTest(String),
// which drives the overridden hooks of tc.
func smRunTest(t *testing.T, tc *searcherManagerTestCase, testName string) {
	t.Helper()
	t.Fatal(threadedIndexingAndSearchingBlocker)
}

// getFinalSearcher renders the getFinalSearcher() override.
func (tc *searcherManagerTestCase) getFinalSearcher() *search.IndexSearcher {
	t := tc.t
	if !tc.isNRT {
		mustCommit(t, tc.writer)
	}
	if !(smMaybeRefresh(t, tc.mgr) || smIsSearcherCurrent(t, tc.mgr)) {
		t.Fatal("expected maybeRefresh() || isSearcherCurrent()")
	}
	return smAcquire(t, tc.mgr)
}

// doAfterWriter renders the doAfterWriter(ExecutorService) override.
func (tc *searcherManagerTestCase) doAfterWriter(es search.Executor) {
	t := tc.t
	factory := &smWarmingFactory{t: t, tc: tc, es: es}
	if random().Intn(2) == 0 {
		// TODO: can we randomize the applyAllDeletes?  But
		// somehow for final searcher we must apply
		// deletes...
		tc.mgr = smNewSearcherManager(t, tc.writer, factory)
		tc.isNRT = true
	} else {
		// SearcherManager needs to see empty commit:
		mustCommit(t, tc.writer)
		tc.mgr = smNewSearcherManagerFromDir(t, tc.dir, factory)
		tc.isNRT = false
		tc.assertMergedSegmentsWarmed = false
	}

	// lifetimeMGR = new SearcherLifetimeManager();
	t.Fatal(searcherLifetimeManagerBlocker)
}

// doSearching renders the doSearching(ExecutorService, int) override.
func (tc *searcherManagerTestCase) doSearching(es search.Executor, maxIterations int) {
	t := tc.t
	var wg sync.WaitGroup
	wg.Add(1)
	reopenThread := func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				tc.failed.Store(true)
				panic(r)
			}
		}()
		iterations := 0
		for iterations++; iterations < maxIterations; iterations++ {
			time.Sleep(time.Duration(nextInt(1, 100)) * time.Millisecond)
			if _, err := tc.writer.Commit(); err != nil {
				tc.failed.Store(true)
				t.Errorf("commit: %v", err)
				return
			}
			time.Sleep(time.Duration(nextInt(1, 5)) * time.Millisecond)
			block := random().Intn(2) == 0
			if block {
				smMaybeRefreshBlocking(t, tc.mgr)
				t.Error(searcherLifetimeManagerBlocker) // lifetimeMGR.prune(pruner)
			} else if smMaybeRefresh(t, tc.mgr) {
				t.Error(searcherLifetimeManagerBlocker) // lifetimeMGR.prune(pruner)
			}
		}
	}
	go reopenThread()

	t.Fatal(threadedIndexingAndSearchingBlocker) // runSearchThreads(maxIterations)

	wg.Wait()
}

// getCurrentSearcher renders the getCurrentSearcher() override.
func (tc *searcherManagerTestCase) getCurrentSearcher() *search.IndexSearcher {
	t := tc.t
	if random().Intn(10) == 7 {
		// NOTE: not best practice to call maybeRefresh
		// synchronous to your search threads, but still we
		// test as apps will presumably do this for
		// simplicity:
		if smMaybeRefresh(t, tc.mgr) {
			t.Fatal(searcherLifetimeManagerBlocker) // lifetimeMGR.prune(pruner)
		}
	}

	var s *search.IndexSearcher

	tc.pastMu.Lock()
	for len(tc.pastSearchers) != 0 && random().Float64() < 0.25 {
		// 1/4 of the time pull an old searcher, ie, simulate
		// a user doing a follow-on action on a previous
		// search (drilling down/up, clicking next/prev page,
		// etc.)
		// s = lifetimeMGR.acquire(token) — a nil result means the searcher was pruned
		tc.pastMu.Unlock()
		t.Fatal(searcherLifetimeManagerBlocker)
	}
	tc.pastMu.Unlock()

	if s == nil {
		s = smAcquire(t, tc.mgr)
		if s.GetIndexReader().NumDocs() != 0 {
			// Long token = lifetimeMGR.record(s);
			t.Fatal(searcherLifetimeManagerBlocker)
		}
	}

	return s
}

// releaseSearcher renders the releaseSearcher(IndexSearcher) override.
func (tc *searcherManagerTestCase) releaseSearcher(s *search.IndexSearcher) {
	if err := s.GetIndexReader().DecRef(); err != nil {
		tc.t.Fatalf("decRef: %v", err)
	}
}

// doClose renders the doClose() override.
func (tc *searcherManagerTestCase) doClose() {
	t := tc.t
	if !tc.warmCalled {
		t.Fatal("expected warmCalled")
	}
	if testing.Verbose() {
		t.Log("TEST: now close SearcherManager")
	}
	smClose(t, tc.mgr)
	t.Fatal(searcherLifetimeManagerBlocker) // lifetimeMGR.close()
}

// smIntermediateCloseFactory renders the anonymous SearcherFactory of
// testIntermediateClose.
type smIntermediateCloseFactory struct {
	t              *testing.T
	triedReopen    *atomic.Bool
	awaitEnterWarm *sync.WaitGroup
	awaitClose     chan struct{}
	es             search.Executor
}

// NewSearcher renders newSearcher(IndexReader r, IndexReader previous).
func (f *smIntermediateCloseFactory) NewSearcher(ctx context.Context, r index.IndexReaderInterface) (*search.IndexSearcher, error) {
	if f.triedReopen.Load() {
		f.awaitEnterWarm.Done()
		<-f.awaitClose
	}
	return search.NewIndexSearcherWithExecutor(r, f.es), nil
}

func TestSearcherManagerIntermediateClose(t *testing.T) {
	dir := newDirectory()
	// Test can deadlock if we use SMS:
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	writer := mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)
	var awaitEnterWarm sync.WaitGroup
	awaitEnterWarm.Add(1)
	awaitClose := make(chan struct{})
	var triedReopen atomic.Bool
	var es *cachedThreadPool
	var executor search.Executor
	if random().Intn(2) != 0 {
		es = newCachedThreadPool()
		executor = es
	}
	factory := &smIntermediateCloseFactory{
		t: t, triedReopen: &triedReopen, awaitEnterWarm: &awaitEnterWarm, awaitClose: awaitClose, es: executor,
	}
	var searcherManager *search.SearcherManager
	if random().Intn(2) == 0 {
		searcherManager = smNewSearcherManagerFromDir(t, dir, factory)
	} else {
		searcherManager = smNewSearcherManagerWithOptions(t, writer, random().Intn(2) == 0, false, factory)
	}
	if testing.Verbose() {
		t.Log("sm created")
	}
	searcher := smAcquire(t, searcherManager)
	func() {
		defer smRelease(t, searcherManager, searcher)
		assertIntEquals(t, 1, searcher.GetIndexReader().NumDocs())
	}()
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)
	var success atomic.Bool
	var exc error
	done := make(chan struct{})
	go func() {
		defer close(done)
		triedReopen.Store(true)
		if testing.Verbose() {
			t.Log("NOW call maybeRefresh")
		}
		_, err := searcherManager.MaybeRefresh()
		var ace *store.AlreadyClosedException
		switch {
		case err == nil:
			success.Store(true)
		case errors.As(err, &ace):
			// expected
		default:
			exc = err
			// use success as the barrier here to make sure we see the write
			success.Store(false)
		}
	}()
	if testing.Verbose() {
		t.Log("THREAD started")
	}
	awaitEnterWarm.Wait()
	if testing.Verbose() {
		t.Log("NOW call close")
	}
	smClose(t, searcherManager)
	close(awaitClose)
	<-done
	_, err := searcherManager.Acquire()
	expectAlreadyClosed(t, err)
	if success.Load() {
		t.Fatal("expected success == false")
	}
	if !triedReopen.Load() {
		t.Fatal("expected triedReopen")
	}
	if exc != nil {
		t.Fatalf("%v", exc)
	}
	mustClose(t, writer, dir)
	if es != nil {
		es.shutdownAndAwaitTermination()
	}
}

func TestSearcherManagerCloseTwice(t *testing.T) {
	// test that we can close SM twice (per Closeable's contract).
	dir := newDirectory()
	mustClose(t, mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil)))
	sm := smNewSearcherManagerFromDir(t, dir, nil)
	smClose(t, sm)
	smClose(t, sm)
	mustClose(t, dir)
}

func TestSearcherManagerReferenceDecrementIllegally(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	writer := mustNewIndexWriter(t, dir, iwc)
	sm := smNewSearcherManagerWithOptions(t, writer, false, false, search.NewDefaultSearcherFactory())
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)
	smMaybeRefreshBlocking(t, sm)

	acquire := smAcquire(t, sm)
	acquire2 := smAcquire(t, sm)
	smRelease(t, sm, acquire)
	smRelease(t, sm, acquire2)

	acquire = smAcquire(t, sm)
	if err := acquire.GetIndexReader().DecRef(); err != nil {
		t.Fatal(err)
	}
	smRelease(t, sm, acquire)
	// expectThrows(IllegalStateException.class, sm::acquire)
	if _, err := sm.Acquire(); err == nil {
		t.Fatal("expected IllegalStateException")
	}

	// sm.close(); -- already closed
	mustClose(t, writer, dir)
}

func TestSearcherManagerEnsureOpen(t *testing.T) {
	dir := newDirectory()
	mustClose(t, mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil)))
	sm := smNewSearcherManagerFromDir(t, dir, nil)
	s := smAcquire(t, sm)
	smClose(t, sm)

	// this should succeed;
	smRelease(t, sm, s)

	// this should fail
	_, err := sm.Acquire()
	expectAlreadyClosed(t, err)

	// this should fail
	_, err = sm.MaybeRefresh()
	expectAlreadyClosed(t, err)

	mustClose(t, dir)
}

func TestSearcherManagerListenerCalled(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	var afterRefreshCalled atomic.Bool
	sm := smNewSearcherManagerWithOptions(t, iw, false, false, search.NewDefaultSearcherFactory())
	// sm.addListener(new ReferenceManager.RefreshListener() {
	//   beforeRefresh() {}
	//   afterRefresh(boolean didRefresh) { if (didRefresh) afterRefreshCalled.set(true); }
	// });
	t.Fatal(refreshListenerBlocker)
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

// smEvilSearcherFactory renders theEvilOne of testEvilSearcherFactory.
type smEvilSearcherFactory struct {
	t     *testing.T
	other index.IndexReaderInterface
}

// NewSearcher renders newSearcher(IndexReader ignored, IndexReader previous).
func (f *smEvilSearcherFactory) NewSearcher(ctx context.Context, ignored index.IndexReaderInterface) (*search.IndexSearcher, error) {
	return newSearcher(f.t, f.other), nil
}

func TestSearcherManagerEvilSearcherFactory(t *testing.T) {
	r := random()
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	mustCommit(t, w)

	other := mustOpenDirectoryReader(t, dir)

	theEvilOne := &smEvilSearcherFactory{t: t, other: other}

	// expectThrows(IllegalStateException.class, () -> new SearcherManager(dir, theEvilOne));
	if _, err := search.NewSearcherManagerFromDir(dir, theEvilOne); err == nil {
		t.Fatal("expected IllegalStateException")
	}
	// expectThrows(IllegalStateException.class,
	//     () -> new SearcherManager(w.w, random.nextBoolean(), false, theEvilOne));
	if _, err := search.NewSearcherManagerWithOptions(w.W, r.Intn(2) == 0, false, theEvilOne, nil); err == nil {
		t.Fatal("expected IllegalStateException")
	}
	mustClose(t, w, other, dir)
}

func TestSearcherManagerMaybeRefreshBlockingLock(t *testing.T) {
	// make sure that maybeRefreshBlocking releases the lock, otherwise other
	// threads cannot obtain it.
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	mustClose(t, w)

	sm := smNewSearcherManagerFromDir(t, dir, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// this used to not release the lock, preventing other threads from obtaining it.
		t.Error(maybeRefreshBlockingBlocker) // sm.maybeRefreshBlocking()
	}()
	<-done
	if t.Failed() {
		t.FailNow()
	}

	// if maybeRefreshBlocking didn't release the lock, this will fail.
	if !smMaybeRefresh(t, sm) {
		t.Fatal("failde to obtain the refreshLock!")
	}

	smClose(t, sm)
	mustClose(t, dir)
}

// LUCENE-6087
func TestSearcherManagerCustomDirectoryReader(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	nrtReader := mustGetReader(t, w)

	// FilterDirectoryReader reader = new MyFilterDirectoryReader(nrtReader);
	t.Fatal(filterDirectoryReaderSubReaderWrapperBlocker)

	mustClose(t, nrtReader, w, dir)
}

// smPreviousReaderFactory renders the local class MySearcherFactory of
// testPreviousReaderIsPassed.
type smPreviousReaderFactory struct {
	t                  *testing.T
	lastReader         index.IndexReaderInterface
	lastPreviousReader index.IndexReaderInterface
	called             int
}

// NewSearcher renders newSearcher(IndexReader reader, IndexReader previousReader).
func (f *smPreviousReaderFactory) NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*search.IndexSearcher, error) {
	f.t.Fatal(searcherFactoryPreviousReaderBlocker)
	f.called++
	f.lastReader = reader
	return search.NewDefaultSearcherFactory().NewSearcher(ctx, reader)
}

func TestSearcherManagerPreviousReaderIsPassed(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())

	factory := &smPreviousReaderFactory{t: t}
	sm := smNewSearcherManagerWithOptions(t, w, random().Intn(2) == 0, false, factory)
	assertIntEquals(t, 1, factory.called)
	if factory.lastPreviousReader != nil {
		t.Fatal("expected no previous reader")
	}
	if factory.lastReader == nil {
		t.Fatal("expected a last reader")
	}
	acquire := smAcquire(t, sm)
	if factory.lastReader != acquire.GetIndexReader() {
		t.Fatal("expected same reader")
	}
	smRelease(t, sm, acquire)

	lastReader := factory.lastReader
	// refresh
	mustAddDocument(t, w, document.NewDocument())
	if !smMaybeRefresh(t, sm) {
		t.Fatal("expected a refresh")
	}

	acquire = smAcquire(t, sm)
	if factory.lastReader != acquire.GetIndexReader() {
		t.Fatal("expected same reader")
	}
	smRelease(t, sm, acquire)
	if factory.lastPreviousReader == nil {
		t.Fatal("expected a previous reader")
	}
	if lastReader != factory.lastPreviousReader {
		t.Fatal("expected previous reader to be the last reader")
	}
	if factory.lastReader == lastReader {
		t.Fatal("expected a new reader")
	}
	assertIntEquals(t, 2, factory.called)
	mustClose(t, w)
	smClose(t, sm)
	mustClose(t, dir)
}

func TestSearcherManagerConcurrentIndexCloseSearchAndRefresh(t *testing.T) {
	// final Directory dir = newFSDirectory(createTempDir());
	t.Fatal(newFSDirectoryBlocker)
	dir := newDirectory()
	var writerRef atomic.Pointer[index.IndexWriter]
	analyzer := testanalysis.NewMockAnalyzerRandom(random())
	analyzer.SetMaxTokenLength(index.MAX_TERM_LENGTH)
	writerRef.Store(mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer)))

	var mgrRef atomic.Pointer[search.SearcherManager]
	mgrRef.Store(smNewSearcherManager(t, writerRef.Load(), nil))
	var stop atomic.Bool

	var wg sync.WaitGroup
	wg.Add(4)

	indexThread := func() {
		defer wg.Done()
		defer stop.Store(true)
		numDocs := atLeast(200)
		for i := 0; i < numDocs; i++ {
			w := writerRef.Load()
			doc := document.NewDocument()
			doc.Add(newTextField(t, "field", randomAnalysisString(t, 256, false), true))
			if _, err := w.AddDocument(doc); err != nil {
				t.Errorf("addDocument: %v", err)
				return
			}
			if random().Intn(1000) == 17 {
				var err error
				if random().Intn(2) == 0 {
					err = w.Close()
				} else {
					err = w.Rollback()
				}
				if err != nil {
					t.Errorf("close/rollback: %v", err)
					return
				}
				nw, err := index.NewIndexWriter(dir, newIndexWriterConfigWithAnalyzer(analyzer))
				if err != nil {
					t.Errorf("new IndexWriter: %v", err)
					return
				}
				writerRef.Store(nw)
			}
		}
		if testing.Verbose() {
			stats, _ := writerRef.Load().GetDocStats()
			t.Logf("TEST: index count=%d", stats.MaxDoc)
		}
	}

	searchThread := func() {
		defer wg.Done()
		totCount := 0
		for !stop.Load() {
			mgr := mgrRef.Load()
			if mgr != nil {
				searcher, err := mgr.Acquire()
				if err != nil {
					var ace *store.AlreadyClosedException
					if errors.As(err, &ace) {
						// ok
						continue
					}
					t.Errorf("acquire: %v", err)
					return
				}
				totCount += searcher.GetIndexReader().MaxDoc()
				if err := mgr.Release(searcher); err != nil {
					t.Errorf("release: %v", err)
					return
				}
			}
		}
		if testing.Verbose() {
			t.Logf("TEST: search totCount=%d", totCount)
		}
	}

	refreshThread := func() {
		defer wg.Done()
		refreshCount := 0
		aceCount := 0
		for !stop.Load() {
			mgr := mgrRef.Load()
			if mgr != nil {
				refreshCount++
				t.Error(maybeRefreshBlockingBlocker) // mgr.maybeRefreshBlocking()
				return
			}
		}
		if testing.Verbose() {
			t.Logf("TEST: refresh count=%d aceCount=%d", refreshCount, aceCount)
		}
	}

	closeThread := func() {
		defer wg.Done()
		closeCount := 0
		aceCount := 0
		for !stop.Load() {
			mgr := mgrRef.Load()
			if mgr == nil {
				panic("assert mgr != null")
			}
			if err := mgr.Close(); err != nil {
				t.Errorf("close: %v", err)
				return
			}
			closeCount++
			for !stop.Load() {
				nm, err := search.NewSearcherManager(writerRef.Load(), nil)
				if err == nil {
					mgrRef.Store(nm)
					break
				}
				var ace *store.AlreadyClosedException
				if !errors.As(err, &ace) {
					t.Errorf("new SearcherManager: %v", err)
					return
				}
				// ok
				aceCount++
			}
		}
		if testing.Verbose() {
			t.Logf("TEST: close count=%d aceCount=%d", closeCount, aceCount)
		}
	}

	go indexThread()
	go searchThread()
	go refreshThread()
	go closeThread()

	wg.Wait()

	smClose(t, mgrRef.Load())
	mustClose(t, writerRef.Load(), dir)
}

// nextCommitSelector renders the public static NextCommitSelector: it
// returns the first commit with generation higher than current reader commit.
type nextCommitSelector struct{}

// GetSearcherRefreshCommit renders getSearcherRefreshCommit(DirectoryReader).
func (nextCommitSelector) GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error) {
	commits, err := index.ListCommits(reader.GetDirectory())
	if err != nil {
		return nil, err
	}
	current := reader.GetIndexCommit()
	for i := 0; i < len(commits); i++ {
		commit := commits[i]
		if commit.GetGeneration() > current.GetGeneration() {
			return commit, nil
		}
	}
	// we're already on latest commit
	return nil, nil
}

func TestSearcherManagerStepWiseCommitRefresh(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexDeletionPolicy(index.NoDeletionPolicyInstance)
	w := mustNewIndexWriter(t, dir, iwc)
	docID := 0
	// create initial commit
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "docId", "doc-"+strconv.Itoa(docID), true))
		docID++
		mustAddDocument(t, w, doc)
	}
	mustCommit(t, w)
	sm, err := search.NewSearcherManagerFromReader(mustOpenDirectoryReader(t, dir), nil, nextCommitSelector{})
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	const numCommits = 5
	for i := 0; i < numCommits; i++ {
		for j := 0; j < 20; j++ {
			doc := document.NewDocument()
			doc.Add(newStringField(t, "docId", "doc-"+strconv.Itoa(docID), true))
			docID++
			mustAddDocument(t, w, doc)
		}
		mustCommit(t, w)
	}

	// maybeRefresh only refreshes on the next incremental commit
	// so it takes us numCommits to get to latest
	stepsToCurrent := 0
	for !smIsSearcherCurrent(t, sm) {
		oldGen, err := sm.GetSearcherCommitGeneration()
		if err != nil {
			t.Fatal(err)
		}
		smMaybeRefreshBlocking(t, sm)
		newGen, err := sm.GetSearcherCommitGeneration()
		if err != nil {
			t.Fatal(err)
		}
		if newGen != oldGen+1 {
			t.Fatalf("newGen = %d, want %d", newGen, oldGen+1)
		}
		stepsToCurrent++
	}
	assertIntEquals(t, numCommits, stepsToCurrent)
	smClose(t, sm)
	mustClose(t, w, dir)
}
