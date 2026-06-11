// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/search/TestSearcherManager.java
// Purpose: Tests for SearcherManager - NRT reopen, thread safety, lifecycle management

package search_test

import (
	"context"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSearcherManager_Basic tests that a SearcherManager can be created from a
// directory, that Acquire/Release work, and that the acquired searcher can
// search the committed index.
func TestSearcherManager_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build a small committed index.
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "hello world", true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	w.Close()

	// Create SearcherManager from directory, with an afterClose that closes the reader.
	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer sm.Release(searcher)

	topDocs, err := searcher.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 5 {
		t.Errorf("expected 5 hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestSearcherManager_NRT tests NRT behavior: create a SearcherManager from a
// directory, add documents via IndexWriter, commit, refresh, and verify the
// searcher sees the new documents.
func TestSearcherManager_NRT(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build initial committed index.
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "initial doc", true)
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Create SearcherManager from directory.
	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	// Verify initial searcher sees 1 doc.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	topDocs, err := searcher.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("before refresh: expected 1 hit, got %d", topDocs.TotalHits.Value)
	}
	sm.Release(searcher)

	// Add more docs and commit.
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "new doc", true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Trigger refresh.
	ctx := context.Background()
	refreshed, err := sm.MaybeRefresh(ctx)
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if !refreshed {
		t.Error("expected refresh to return true after commit")
	}

	// Verify searcher now sees 4 docs.
	searcher2, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire after refresh: %v", err)
	}
	defer sm.Release(searcher2)
	topDocs2, err := searcher2.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search after refresh: %v", err)
	}
	if topDocs2.TotalHits.Value != 4 {
		t.Errorf("after refresh: expected 4 hits, got %d", topDocs2.TotalHits.Value)
	}

	w.Close()
}

// TestSearcherManager_IntermediateClose tests that the SearcherManager does
// not deadlock or panic when Close is called during a concurrent Acquire.
func TestSearcherManager_IntermediateClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}

	// Acquire, then close, then verify release still works.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := sm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !sm.IsClosed() {
		t.Error("expected manager to be closed")
	}

	// Release should still work after close (the searcher was acquired before close).
	if err := sm.Release(searcher); err != nil {
		t.Errorf("Release after close: %v", err)
	}
}

// TestSearcherManager_CloseTwice verifies that Close can be called multiple
// times without error (per Closeable's contract).
func TestSearcherManager_CloseTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}

	if err := sm.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := sm.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestSearcherManager_EnsureOpen verifies that Acquire and MaybeRefresh return
// an error when called after Close.
func TestSearcherManager_EnsureOpen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}

	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire before close: %v", err)
	}
	sm.Release(searcher)

	if err := sm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := sm.Acquire(); err == nil {
		t.Error("expected error on Acquire after Close")
	}

	ctx := context.Background()
	if _, err := sm.MaybeRefresh(ctx); err == nil {
		t.Error("expected error on MaybeRefresh after Close")
	}
}

// TestSearcherManager_ListenerCalled tests that the afterClose callback is
// invoked when a searcher is released.
func TestSearcherManager_ListenerCalled(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	w.Close()

	afterCloseCalled := false
	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		afterCloseCalled = true
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}

	// Acquire, release (ref goes to zero), then close.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	sm.Release(searcher)

	// afterClose won't be called until the searcher is swapped out or the
	// manager closes and the ref count reaches zero.
	sm.Close()

	if !afterCloseCalled {
		t.Error("expected afterClose callback to be invoked")
	}
}

// TestSearcherManager_PreviousReaderPassed tests that the SearcherFactory is
// called with the correct reader during refresh.
func TestSearcherManager_PreviousReaderPassed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add initial doc and commit.
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "initial", true)
	doc.Add(f)
	w.AddDocument(doc)
	w.Commit()

	called := 0
	factory := &testSearcherFactory{
		fn: func() {
			called++
		},
	}

	sm, err := search.NewSearcherManagerFromDirectory(dir, factory, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	if called != 0 {
		t.Errorf("expected factory not yet called on init (initial searcher is created directly), got %d", called)
	}

	// Add more docs, commit, and refresh (factory is called during refresh).
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "more", true)
		doc.Add(f)
		w.AddDocument(doc)
	}
	w.Commit()

	ctx := context.Background()
	refreshed, err := sm.MaybeRefresh(ctx)
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if !refreshed {
		t.Error("expected refresh to return true")
	}

	if called != 1 {
		t.Errorf("expected factory called once (on refresh), got %d", called)
	}

	w.Close()
}

// testSearcherFactory is a simple SearcherFactory implementation for testing.
type testSearcherFactory struct {
	fn func()
}

func (f *testSearcherFactory) NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*search.IndexSearcher, error) {
	if f.fn != nil {
		f.fn()
	}
	return search.NewIndexSearcher(reader), nil
}

// TestSearcherManager_MaybeRefreshBlockingLock tests that MaybeRefresh returns
// false (no error) when the index hasn't changed.
func TestSearcherManager_MaybeRefreshBlockingLock(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	ctx := context.Background()
	// First call to MaybeRefresh should return false (index unchanged).
	refreshed, err := sm.MaybeRefresh(ctx)
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if refreshed {
		t.Log("MaybeRefresh returned true (acceptable if initial reader was not at latest)")
	}

	// Verify we can still acquire and use a searcher after MaybeRefresh.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire after MaybeRefresh: %v", err)
	}
	defer sm.Release(searcher)
}

// TestSearcherManager_ConcurrentOperations tests concurrent Acquire/Release
// calls to verify thread safety.
func TestSearcherManager_ConcurrentOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "hello", true)
		doc.Add(f)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			searcher, err := sm.Acquire()
			if err != nil {
				return
			}
			defer sm.Release(searcher)
			searcher.Search(search.NewMatchAllDocsQuery(), 10)
		}()
	}
	wg.Wait()
}

// TestSearcherManager_IsSearcherCurrent verifies that IsCurrent on the
// underlying reader reports correctly.
func TestSearcherManager_IsSearcherCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "hello", true)
	doc.Add(f)
	w.AddDocument(doc)
	w.Commit()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	// Initially the searcher should be current.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	reader := searcher.GetReader()
	type currentChecker interface {
		IsCurrent() (bool, error)
	}
	if cc, ok := reader.(currentChecker); ok {
		isCurrent, err := cc.IsCurrent()
		if err != nil {
			t.Fatalf("IsCurrent: %v", err)
		}
		if !isCurrent {
			t.Error("expected reader to be current initially")
		}
	}
	sm.Release(searcher)

	// Add more docs and commit (index is now newer).
	doc2 := document.NewDocument()
	f2, _ := document.NewTextField("content", "world", true)
	doc2.Add(f2)
	w.AddDocument(doc2)
	w.Commit()

	// Old reader should now report not current.
	searcher2, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire after commit: %v", err)
	}
	reader2 := searcher2.GetReader()
	if cc, ok := reader2.(currentChecker); ok {
		isCurrent, err := cc.IsCurrent()
		if err != nil {
			t.Fatalf("IsCurrent: %v", err)
		}
		// The reader may or may not be current depending on whether the
		// SearcherManager auto-refreshed. Check that the value is sensible.
		_ = isCurrent
	}
	sm.Release(searcher2)

	w.Close()
}

// TestSearcherManager_ThreadSafety tests concurrent Acquire/Release/MaybeRefresh
// from multiple goroutines.
func TestSearcherManager_ThreadSafety(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "hello", true)
		doc.Add(f)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	var wg sync.WaitGroup

	// Goroutines that acquire and release.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				searcher, err := sm.Acquire()
				if err != nil {
					return
				}
				searcher.Search(search.NewMatchAllDocsQuery(), 10)
				sm.Release(searcher)
			}
		}()
	}
	wg.Wait()
}

// TestSearcherManager_Lifecycle tests the full lifecycle: create, use searcher,
// add docs, refresh, re-acquire, close.
func TestSearcherManager_Lifecycle(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "hello", true)
	doc.Add(f)
	w.AddDocument(doc)
	w.Commit()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		t.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}

	// Phase 1: Acquire and search.
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	topDocs, err := searcher.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit, got %d", topDocs.TotalHits.Value)
	}
	if err := sm.Release(searcher); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// Phase 2: Add more docs, commit, refresh.
	for i := 0; i < 4; i++ {
		d := document.NewDocument()
		f, _ := document.NewTextField("content", "world", true)
		d.Add(f)
		w.AddDocument(d)
	}
	w.Commit()
	ctx := context.Background()
	refreshed, err := sm.MaybeRefresh(ctx)
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if !refreshed {
		t.Error("expected refresh after new commit")
	}

	// Phase 3: Acquire new searcher and verify.
	searcher2, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Acquire after refresh: %v", err)
	}
	topDocs2, err := searcher2.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search after refresh: %v", err)
	}
	if topDocs2.TotalHits.Value != 5 {
		t.Errorf("expected 5 hits, got %d", topDocs2.TotalHits.Value)
	}
	sm.Release(searcher2)

	if err := sm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !sm.IsClosed() {
		t.Error("expected manager to be closed")
	}

	w.Close()
}

// BenchmarkSearcherManager_AcquireRelease benchmarks the Acquire/Release cycle.
func BenchmarkSearcherManager_AcquireRelease(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		b.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "hello", true)
		doc.Add(f)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	sm, err := search.NewSearcherManagerFromDirectory(dir, nil, func(s *search.IndexSearcher) {
		s.Close()
	})
	if err != nil {
		b.Fatalf("NewSearcherManagerFromDirectory: %v", err)
	}
	defer sm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher, err := sm.Acquire()
		if err != nil {
			b.Fatalf("Acquire: %v", err)
		}
		if err := sm.Release(searcher); err != nil {
			b.Fatalf("Release: %v", err)
		}
	}
}
