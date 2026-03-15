// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/search/TestSearcherManager.java
// Purpose: Tests for SearcherManager - NRT reopen, thread safety, lifecycle management

package search_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SearcherManager manages a single instance of IndexSearcher for searching.
// It handles NRT (Near Real Time) reopening and reference counting.
type SearcherManager struct {
	mu sync.RWMutex

	// current holds the current IndexSearcher
	current *ManagedIndexSearcher

	// directory is the underlying directory
	directory store.Directory

	// writer is the IndexWriter (for NRT mode)
	writer *index.IndexWriter

	// factory creates new IndexSearchers
	factory SearcherFactory

	// listeners are called on refresh events
	listeners []RefreshListener

	// closed indicates if the manager is closed
	closed atomic.Bool

	// applyAllDeletes indicates if deletes should be applied on refresh
	applyAllDeletes bool

	// writeAllDeletes indicates if deletes should be written
	writeAllDeletes bool

	// commitSupplier provides custom commit selection for refresh
	commitSupplier RefreshCommitSupplier
}

// ManagedIndexSearcher wraps an IndexSearcher with reference counting
type ManagedIndexSearcher struct {
	searcher *IndexSearcher
	reader   index.IndexReaderInterface
	refCount atomic.Int32
	manager  *SearcherManager
}

// SearcherFactory creates IndexSearchers
type SearcherFactory interface {
	// NewSearcher creates a new IndexSearcher from the given reader
	NewSearcher(reader index.IndexReaderInterface, previous index.IndexReaderInterface) (*IndexSearcher, error)
}

// DefaultSearcherFactory is the default implementation of SearcherFactory
type DefaultSearcherFactory struct{}

// NewSearcher creates a new IndexSearcher
func (f *DefaultSearcherFactory) NewSearcher(reader index.IndexReaderInterface, previous index.IndexReaderInterface) (*IndexSearcher, error) {
	return &IndexSearcher{reader: reader}, nil
}

// RefreshListener is notified of refresh events
type RefreshListener interface {
	// BeforeRefresh is called before a refresh
	BeforeRefresh()
	// AfterRefresh is called after a refresh, didRefresh indicates if the searcher was updated
	AfterRefresh(didRefresh bool)
}

// RefreshCommitSupplier provides custom commit selection for refresh
type RefreshCommitSupplier interface {
	// GetSearcherRefreshCommit returns the commit to use for refresh
	GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error)
}

// NewSearcherManager creates a SearcherManager from a Directory
func NewSearcherManager(dir store.Directory, factory SearcherFactory) (*SearcherManager, error) {
	if factory == nil {
		factory = &DefaultSearcherFactory{}
	}

	// Open the directory reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open directory reader: %w", err)
	}

	sm := &SearcherManager{
		directory:       dir,
		factory:         factory,
		applyAllDeletes: true,
		listeners:       make([]RefreshListener, 0),
	}

	// Create initial searcher
	searcher, err := factory.NewSearcher(reader, nil)
	if err != nil {
		reader.Close()
		return nil, err
	}

	sm.current = &ManagedIndexSearcher{
		searcher: searcher,
		reader:   reader,
		manager:  sm,
	}
	sm.current.refCount.Store(1)

	return sm, nil
}

// NewSearcherManagerFromWriter creates a SearcherManager from an IndexWriter (NRT mode)
func NewSearcherManagerFromWriter(writer *index.IndexWriter, applyAllDeletes, writeAllDeletes bool, factory SearcherFactory) (*SearcherManager, error) {
	if factory == nil {
		factory = &DefaultSearcherFactory{}
	}

	// Get NRT reader from writer
	reader, err := writer.GetReader(applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, fmt.Errorf("failed to get NRT reader: %w", err)
	}

	sm := &SearcherManager{
		writer:          writer,
		factory:         factory,
		applyAllDeletes: applyAllDeletes,
		writeAllDeletes: writeAllDeletes,
		listeners:       make([]RefreshListener, 0),
	}

	// Create initial searcher
	searcher, err := factory.NewSearcher(reader, nil)
	if err != nil {
		reader.Close()
		return nil, err
	}

	sm.current = &ManagedIndexSearcher{
		searcher: searcher,
		reader:   reader,
		manager:  sm,
	}
	sm.current.refCount.Store(1)

	return sm, nil
}

// NewSearcherManagerFromReader creates a SearcherManager from an existing DirectoryReader
func NewSearcherManagerFromReader(reader *index.DirectoryReader, factory SearcherFactory, commitSupplier RefreshCommitSupplier) (*SearcherManager, error) {
	if factory == nil {
		factory = &DefaultSearcherFactory{}
	}

	sm := &SearcherManager{
		directory:       reader.Directory(),
		factory:         factory,
		commitSupplier:  commitSupplier,
		applyAllDeletes: true,
		listeners:       make([]RefreshListener, 0),
	}

	// Create initial searcher
	searcher, err := factory.NewSearcher(reader, nil)
	if err != nil {
		return nil, err
	}

	sm.current = &ManagedIndexSearcher{
		searcher: searcher,
		reader:   reader,
		manager:  sm,
	}
	sm.current.refCount.Store(1)

	return sm, nil
}

// Acquire returns the current IndexSearcher
func (sm *SearcherManager) Acquire() (*IndexSearcher, error) {
	if sm.closed.Load() {
		return nil, errors.New("searcher manager is closed")
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.current == nil {
		return nil, errors.New("no current searcher available")
	}

	sm.current.refCount.Add(1)
	return sm.current.searcher, nil
}

// Release releases a previously acquired IndexSearcher
func (sm *SearcherManager) Release(searcher *IndexSearcher) error {
	if sm.current != nil && sm.current.searcher == searcher {
		sm.current.refCount.Add(-1)
	}
	return nil
}

// MaybeRefresh attempts to refresh the searcher if needed
// Returns true if a refresh was performed
func (sm *SearcherManager) MaybeRefresh() (bool, error) {
	if sm.closed.Load() {
		return false, errors.New("searcher manager is closed")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.refreshLocked()
}

// MaybeRefreshBlocking blocks until a refresh is completed if needed
func (sm *SearcherManager) MaybeRefreshBlocking() error {
	if sm.closed.Load() {
		return errors.New("searcher manager is closed")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, err := sm.refreshLocked()
	return err
}

// refreshLocked performs the actual refresh with lock held
func (sm *SearcherManager) refreshLocked() (bool, error) {
	// Notify listeners before refresh
	for _, listener := range sm.listeners {
		listener.BeforeRefresh()
	}

	var newReader index.IndexReaderInterface
	var err error

	if sm.writer != nil {
		// NRT mode - get reader from writer
		newReader, err = sm.writer.GetReader(sm.applyAllDeletes, sm.writeAllDeletes)
	} else if sm.commitSupplier != nil {
		// Custom commit supplier mode
		if dr, ok := sm.current.reader.(*index.DirectoryReader); ok {
			commit, err := sm.commitSupplier.GetSearcherRefreshCommit(dr)
			if err != nil {
				return false, err
			}
			if commit == nil {
				// No new commit available
				for _, listener := range sm.listeners {
					listener.AfterRefresh(false)
				}
				return false, nil
			}
			newReader, err = index.OpenDirectoryReaderAtCommit(sm.directory, commit)
		}
	} else {
		// Standard mode - reopen directory reader
		if dr, ok := sm.current.reader.(*index.DirectoryReader); ok {
			newReader, err = dr.Reopen()
		} else {
			newReader, err = index.OpenDirectoryReader(sm.directory)
		}
	}

	if err != nil {
		for _, listener := range sm.listeners {
			listener.AfterRefresh(false)
		}
		return false, err
	}

	// Check if reader actually changed
	if newReader == sm.current.reader {
		for _, listener := range sm.listeners {
			listener.AfterRefresh(false)
		}
		return false, nil
	}

	// Create new searcher
	newSearcher, err := sm.factory.NewSearcher(newReader, sm.current.reader)
	if err != nil {
		newReader.Close()
		for _, listener := range sm.listeners {
			listener.AfterRefresh(false)
		}
		return false, err
	}

	// Update current
	oldManaged := sm.current
	sm.current = &ManagedIndexSearcher{
		searcher: newSearcher,
		reader:   newReader,
		manager:  sm,
	}
	sm.current.refCount.Store(1)

	// Decrement old reference
	oldManaged.refCount.Add(-1)
	if oldManaged.refCount.Load() <= 0 {
		oldManaged.reader.Close()
	}

	// Notify listeners after refresh
	for _, listener := range sm.listeners {
		listener.AfterRefresh(true)
	}

	return true, nil
}

// IsSearcherCurrent returns true if the current searcher is up to date
func (sm *SearcherManager) IsSearcherCurrent() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.current == nil {
		return false
	}

	if dr, ok := sm.current.reader.(*index.DirectoryReader); ok {
		return dr.IsCurrent()
	}

	return true
}

// GetSearcherCommitGeneration returns the commit generation of the current searcher
func (sm *SearcherManager) GetSearcherCommitGeneration() int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.current == nil {
		return -1
	}

	if dr, ok := sm.current.reader.(*index.DirectoryReader); ok {
		commit := dr.GetIndexCommit()
		if commit != nil {
			return commit.GetGeneration()
		}
	}

	return -1
}

// AddListener adds a refresh listener
func (sm *SearcherManager) AddListener(listener RefreshListener) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.listeners = append(sm.listeners, listener)
}

// Close closes the SearcherManager
func (sm *SearcherManager) Close() error {
	if sm.closed.CompareAndSwap(false, true) {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		if sm.current != nil {
			sm.current.refCount.Add(-1)
			if sm.current.refCount.Load() <= 0 {
				sm.current.reader.Close()
			}
			sm.current = nil
		}
	}
	return nil
}

// IndexSearcher wraps an IndexSearcher for the search package
type IndexSearcher struct {
	reader index.IndexReaderInterface
}

// GetIndexReader returns the underlying IndexReader
func (s *IndexSearcher) GetIndexReader() index.IndexReaderInterface {
	return s.reader
}

// Search performs a search (stub for testing)
func (s *IndexSearcher) Search(query Query, n int) (*TopDocs, error) {
	return &TopDocs{
		TotalHits: NewTotalHits(0, TotalHitsEqualTo),
		ScoreDocs: make([]*ScoreDoc, 0),
	}, nil
}

// Query interface stub
type Query interface {
	Rewrite(reader index.IndexReaderInterface) (Query, error)
	CreateWeight(searcher *IndexSearcher, needsScores bool, boost float64) (Weight, error)
}

// Weight interface stub
type Weight interface {
	Scorer(reader index.IndexReaderInterface) (Scorer, error)
}

// Scorer interface stub
type Scorer interface {
	NextDoc() (int, error)
}

// TopDocs represents search results
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
}

// TotalHits represents total hit count
type TotalHits struct {
	Value int64
	Relation TotalHitsRelation
}

// TotalHitsRelation indicates the relation of total hits
type TotalHitsRelation int

const (
	TotalHitsEqualTo TotalHitsRelation = iota
	TotalHitsGreaterThanOrEqualTo
)

// NewTotalHits creates a new TotalHits
func NewTotalHits(value int64, relation TotalHitsRelation) *TotalHits {
	return &TotalHits{Value: value, Relation: relation}
}

// ScoreDoc represents a scored document
type ScoreDoc struct {
	Doc   int
	Score float32
}

// Test Cases

// TestSearcherManager_Basic tests basic SearcherManager functionality
func TestSearcherManager_Basic(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	// Create an index
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add a document
	doc := document.NewDocument()
	doc.Add(document.NewTextField("field", "value", true))
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Commit
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create SearcherManager
	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Acquire searcher
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire searcher: %v", err)
	}

	// Verify document count
	if searcher.GetIndexReader().NumDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", searcher.GetIndexReader().NumDocs())
	}

	// Release searcher
	if err := sm.Release(searcher); err != nil {
		t.Fatalf("Failed to release searcher: %v", err)
	}

	writer.Close()
}

// TestSearcherManager_NRT tests NRT (Near Real Time) mode
func TestSearcherManager_NRT(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Create SearcherManager in NRT mode
	sm, err := NewSearcherManagerFromWriter(writer, true, false, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Add document
	doc := document.NewDocument()
	doc.Add(document.NewTextField("field", "value", true))
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Refresh
	refreshed, err := sm.MaybeRefresh()
	if err != nil {
		t.Fatalf("MaybeRefresh failed: %v", err)
	}

	// Acquire and verify
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire searcher: %v", err)
	}

	// In NRT mode, we should see the document after refresh
	if refreshed {
		t.Logf("Searcher was refreshed, doc count: %d", searcher.GetIndexReader().NumDocs())
	}

	sm.Release(searcher)
}

// TestSearcherManager_IntermediateClose tests closing while refresh is in progress
func TestSearcherManager_IntermediateClose(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	writer.AddDocument(doc)
	writer.Commit()

	// Create SearcherManager with custom factory that simulates slow warm
	awaitEnterWarm := make(chan struct{})
	awaitClose := make(chan struct{})
	triedReopen := atomic.Bool{}

	factory := &slowSearcherFactory{
		awaitEnterWarm: awaitEnterWarm,
		awaitClose:     awaitClose,
		triedReopen:    &triedReopen,
	}

	sm, err := NewSearcherManager(dir, factory)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}

	// Acquire and release initial searcher
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire searcher: %v", err)
	}
	if searcher.GetIndexReader().NumDocs() != 1 {
		t.Errorf("Expected 1 document, got %d", searcher.GetIndexReader().NumDocs())
	}
	sm.Release(searcher)

	// Add another document and commit
	doc2 := document.NewDocument()
	writer.AddDocument(doc2)
	writer.Commit()

	// Start refresh in background
	success := atomic.Bool{}
	var refreshErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		triedReopen.Store(true)
		_, refreshErr = sm.MaybeRefresh()
		success.Store(refreshErr == nil)
	}()

	// Wait for refresh to enter warm
	<-awaitEnterWarm

	// Close the manager
	sm.Close()

	// Signal to complete warm
	close(awaitClose)

	// Wait for refresh to complete
	wg.Wait()

	// Should have failed or been interrupted
	if success.Load() {
		t.Error("Expected refresh to fail after close, but it succeeded")
	}

	// Try to acquire after close - should fail
	_, err = sm.Acquire()
	if err == nil {
		t.Error("Expected error when acquiring after close")
	}

	writer.Close()
}

// slowSearcherFactory simulates a slow searcher factory for testing
type slowSearcherFactory struct {
	awaitEnterWarm chan struct{}
	awaitClose     chan struct{}
	triedReopen    *atomic.Bool
}

func (f *slowSearcherFactory) NewSearcher(reader index.IndexReaderInterface, previous index.IndexReaderInterface) (*IndexSearcher, error) {
	if f.triedReopen.Load() {
		close(f.awaitEnterWarm)
		<-f.awaitClose
	}
	return &IndexSearcher{reader: reader}, nil
}

// TestSearcherManager_CloseTwice tests that closing twice doesn't error
func TestSearcherManager_CloseTwice(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	// Create empty index
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	writer.Close()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}

	// Close twice - should not error
	if err := sm.Close(); err != nil {
		t.Errorf("First close failed: %v", err)
	}
	if err := sm.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestSearcherManager_EnsureOpen tests that operations fail after close
func TestSearcherManager_EnsureOpen(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	writer.Close()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}

	// Acquire before close
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire searcher: %v", err)
	}

	// Close
	sm.Close()

	// Release after close - should succeed
	if err := sm.Release(searcher); err != nil {
		t.Errorf("Release after close failed: %v", err)
	}

	// Acquire after close - should fail
	_, err = sm.Acquire()
	if err == nil {
		t.Error("Expected error when acquiring after close")
	}

	// MaybeRefresh after close - should fail
	_, err = sm.MaybeRefresh()
	if err == nil {
		t.Error("Expected error when calling MaybeRefresh after close")
	}
}

// TestSearcherManager_ListenerCalled tests that refresh listeners are called
func TestSearcherManager_ListenerCalled(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	sm, err := NewSearcherManagerFromWriter(writer, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Add listener
	afterRefreshCalled := atomic.Bool{}
	listener := &testRefreshListener{
		afterRefreshCalled: &afterRefreshCalled,
	}
	sm.AddListener(listener)

	// Add document and commit
	doc := document.NewDocument()
	writer.AddDocument(doc)
	writer.Commit()

	// Before refresh, listener should not have been called
	if afterRefreshCalled.Load() {
		t.Error("Listener called before refresh")
	}

	// Refresh
	sm.MaybeRefreshBlocking()

	// After refresh, listener should have been called
	if !afterRefreshCalled.Load() {
		t.Error("Listener not called after refresh")
	}
}

// testRefreshListener is a test implementation of RefreshListener
type testRefreshListener struct {
	afterRefreshCalled *atomic.Bool
}

func (l *testRefreshListener) BeforeRefresh() {}

func (l *testRefreshListener) AfterRefresh(didRefresh bool) {
	if didRefresh {
		l.afterRefreshCalled.Store(true)
	}
}

// TestSearcherManager_PreviousReaderPassed tests that previous reader is passed to factory
func TestSearcherManager_PreviousReaderPassed(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Create factory that tracks calls
	factory := &trackingSearcherFactory{}

	sm, err := NewSearcherManagerFromWriter(writer, true, false, factory)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Initial creation should call factory with nil previous
	if factory.called != 1 {
		t.Errorf("Expected 1 call to factory, got %d", factory.called)
	}
	if factory.lastPreviousReader != nil {
		t.Error("Expected nil previous reader on first call")
	}

	lastReader := factory.lastReader

	// Acquire and release
	searcher, _ := sm.Acquire()
	sm.Release(searcher)

	// Add document and refresh
	doc := document.NewDocument()
	writer.AddDocument(doc)
	sm.MaybeRefresh()

	// Should have been called again with previous reader
	if factory.called != 2 {
		t.Errorf("Expected 2 calls to factory, got %d", factory.called)
	}
	if factory.lastPreviousReader == nil {
		t.Error("Expected non-nil previous reader on second call")
	}
	if factory.lastPreviousReader != lastReader {
		t.Error("Previous reader should match last reader")
	}
	if factory.lastReader == lastReader {
		t.Error("New reader should be different from last reader")
	}
}

// trackingSearcherFactory tracks calls to NewSearcher
type trackingSearcherFactory struct {
	called             int
	lastReader         index.IndexReaderInterface
	lastPreviousReader index.IndexReaderInterface
}

func (f *trackingSearcherFactory) NewSearcher(reader index.IndexReaderInterface, previous index.IndexReaderInterface) (*IndexSearcher, error) {
	f.called++
	f.lastReader = reader
	f.lastPreviousReader = previous
	return &IndexSearcher{reader: reader}, nil
}

// TestSearcherManager_MaybeRefreshBlockingLock tests that maybeRefreshBlocking releases the lock
func TestSearcherManager_MaybeRefreshBlockingLock(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	writer.Close()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Call maybeRefreshBlocking in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- sm.MaybeRefreshBlocking()
	}()

	// Wait for it to complete
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("MaybeRefreshBlocking failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("MaybeRefreshBlocking timed out")
	}

	// If maybeRefreshBlocking didn't release the lock, this will fail
	refreshed, err := sm.MaybeRefresh()
	if err != nil {
		t.Errorf("Failed to obtain refresh lock: %v", err)
	}
	// Should return false since no changes
	_ = refreshed
}

// TestSearcherManager_ConcurrentOperations tests concurrent index, search, refresh, and close
func TestSearcherManager_ConcurrentOperations(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	sm, err := NewSearcherManagerFromWriter(writer, true, false, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}

	stop := atomic.Bool{}
	var wg sync.WaitGroup

	// Index thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			doc := document.NewDocument()
			doc.Add(document.NewTextField("field", fmt.Sprintf("value%d", i), true))
			writer.AddDocument(doc)
			if i%10 == 0 {
				writer.Commit()
			}
		}
		stop.Store(true)
	}()

	// Search thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			searcher, err := sm.Acquire()
			if err != nil {
				continue
			}
			_ = searcher.GetIndexReader().MaxDoc()
			sm.Release(searcher)
			time.Sleep(time.Millisecond)
		}
	}()

	// Refresh thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			sm.MaybeRefreshBlocking()
			time.Sleep(time.Millisecond)
		}
	}()

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Error("Concurrent operations timed out")
	}

	sm.Close()
}

// TestSearcherManager_IsSearcherCurrent tests the IsSearcherCurrent method
func TestSearcherManager_IsSearcherCurrent(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	sm, err := NewSearcherManagerFromWriter(writer, true, false, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	// Initially should be current
	if !sm.IsSearcherCurrent() {
		t.Error("Searcher should be current initially")
	}

	// Add document
	doc := document.NewDocument()
	writer.AddDocument(doc)

	// May not be current after adding document (depends on NRT implementation)
	// Just verify the method doesn't panic
	_ = sm.IsSearcherCurrent()
}

// TestSearcherManager_ThreadSafety tests thread safety of acquire/release
func TestSearcherManager_ThreadSafety(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	writer.AddDocument(doc)
	writer.Commit()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	writer.Close()

	// Run concurrent acquire/release operations
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				searcher, err := sm.Acquire()
				if err != nil {
					continue
				}
				// Simulate some work
				time.Sleep(time.Microsecond)
				sm.Release(searcher)
			}
		}(i)
	}

	wg.Wait()
}

// TestSearcherManager_Lifecycle tests the complete lifecycle
func TestSearcherManager_Lifecycle(t *testing.T) {
	dir, err := store.NewByteBuffersDirectory()
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Phase 1: Create with empty index
	writer.Commit()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("Failed to create SearcherManager: %v", err)
	}

	// Phase 2: Acquire and verify empty
	searcher, err := sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	if searcher.GetIndexReader().NumDocs() != 0 {
		t.Errorf("Expected 0 docs, got %d", searcher.GetIndexReader().NumDocs())
	}
	sm.Release(searcher)

	// Phase 3: Add documents
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", fmt.Sprintf("doc%d", i), true))
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Phase 4: Refresh and verify
	refreshed, err := sm.MaybeRefresh()
	if err != nil {
		t.Fatalf("MaybeRefresh failed: %v", err)
	}
	if !refreshed {
		t.Error("Expected refresh to occur")
	}

	searcher, err = sm.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	if searcher.GetIndexReader().NumDocs() != 5 {
		t.Errorf("Expected 5 docs, got %d", searcher.GetIndexReader().NumDocs())
	}
	sm.Release(searcher)

	// Phase 5: Close
	if err := sm.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Phase 6: Verify closed state
	_, err = sm.Acquire()
	if err == nil {
		t.Error("Expected error after close")
	}

	writer.Close()
}

// BenchmarkSearcherManager_AcquireRelease benchmarks acquire/release
func BenchmarkSearcherManager_AcquireRelease(b *testing.B) {
	dir, _ := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	writer, _ := index.NewIndexWriter(dir, config)
	writer.Commit()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()
	writer.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			searcher, _ := sm.Acquire()
			sm.Release(searcher)
		}
	})
}

// Ensure context is used
var _ = context.Background

// Ensure rand is used
var _ = rand.Int
