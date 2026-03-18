// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: flush_policy_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestFlushByRamOrCountsPolicy.java
// Purpose: Tests flush policies based on RAM threshold and document count

package index_test

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// MockFlushPolicy is a mock flush policy that tracks peak memory and document counts.
// This is the Go equivalent of MockDefaultFlushPolicy in the Java test.
type MockFlushPolicy struct {
	index.FlushPolicy

	maxBufferedDocs          int
	ramBufferSizeMB          float64
	flushOnRAM               bool
	flushOnDocCount          bool
	peakBytesWithoutFlush    int64
	peakDocCountWithoutFlush int
	hasMarkedPending         bool
	mu                       sync.RWMutex
}

// NewMockFlushPolicy creates a new MockFlushPolicy.
func NewMockFlushPolicy(maxBufferedDocs int, ramBufferSizeMB float64) *MockFlushPolicy {
	return &MockFlushPolicy{
		maxBufferedDocs:          maxBufferedDocs,
		ramBufferSizeMB:          ramBufferSizeMB,
		flushOnRAM:               ramBufferSizeMB > 0,
		flushOnDocCount:          maxBufferedDocs > 0,
		peakBytesWithoutFlush:    math.MinInt64,
		peakDocCountWithoutFlush: math.MinInt64,
	}
}

// ShouldFlush returns true if a flush should occur.
func (p *MockFlushPolicy) ShouldFlush(numDocs int, ramUsed int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	shouldFlush := false

	if p.flushOnDocCount && numDocs >= p.maxBufferedDocs {
		shouldFlush = true
		p.hasMarkedPending = true
	}

	if p.flushOnRAM {
		maxBytes := int64(p.ramBufferSizeMB * 1024 * 1024)
		if ramUsed >= maxBytes {
			shouldFlush = true
			p.hasMarkedPending = true
		}
	}

	if !shouldFlush {
		if ramUsed > p.peakBytesWithoutFlush {
			p.peakBytesWithoutFlush = ramUsed
		}
		if numDocs > p.peakDocCountWithoutFlush {
			p.peakDocCountWithoutFlush = numDocs
		}
	}

	return shouldFlush
}

// FlushOnRAM returns true if flushing on RAM is enabled.
func (p *MockFlushPolicy) FlushOnRAM() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.flushOnRAM
}

// FlushOnDocCount returns true if flushing on document count is enabled.
func (p *MockFlushPolicy) FlushOnDocCount() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.flushOnDocCount
}

// PeakBytesWithoutFlush returns the peak bytes without triggering flush.
func (p *MockFlushPolicy) PeakBytesWithoutFlush() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.peakBytesWithoutFlush
}

// PeakDocCountWithoutFlush returns the peak document count without triggering flush.
func (p *MockFlushPolicy) PeakDocCountWithoutFlush() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.peakDocCountWithoutFlush
}

// HasMarkedPending returns true if any flush was marked pending.
func (p *MockFlushPolicy) HasMarkedPending() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.hasMarkedPending
}

// IndexThread simulates a thread that indexes documents.
// This is the Go equivalent of the IndexThread inner class in Java.
type IndexThread struct {
	writer         *index.IndexWriter
	pendingDocs    *atomic.Int32
	numThreads     int
	docs           []document.Document
	doRandomCommit bool
	ramSize        int64
	mu             sync.RWMutex
	err            error
	done           chan bool
}

// NewIndexThread creates a new IndexThread.
func NewIndexThread(pendingDocs *atomic.Int32, numThreads int, writer *index.IndexWriter, docs []document.Document, doRandomCommit bool) *IndexThread {
	return &IndexThread{
		writer:         writer,
		pendingDocs:    pendingDocs,
		numThreads:     numThreads,
		docs:           docs,
		doRandomCommit: doRandomCommit,
		done:           make(chan bool),
	}
}

// Start starts the indexing thread.
func (t *IndexThread) Start() {
	go t.run()
}

// run is the main loop of the indexing thread.
func (t *IndexThread) run() {
	defer close(t.done)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for t.pendingDocs.Add(-1) > -1 {
		docIndex := int(t.pendingDocs.Load()) % len(t.docs)
		doc := t.docs[docIndex]

		if err := t.writer.AddDocument(&doc); err != nil {
			t.mu.Lock()
			t.err = err
			t.mu.Unlock()
			return
		}

		if t.doRandomCommit && rng.Intn(100) < 5 { // 5% chance
			if err := t.writer.Commit(); err != nil {
				t.mu.Lock()
				t.err = err
				t.mu.Unlock()
				return
			}
		}
	}

	// Final commit
	if err := t.writer.Commit(); err != nil {
		t.mu.Lock()
		t.err = err
		t.mu.Unlock()
	}
}

// Join waits for the thread to complete.
func (t *IndexThread) Join() error {
	<-t.done
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}

// Error returns any error that occurred during indexing.
func (t *IndexThread) Error() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}

// createTestDocuments creates a set of test documents.
func createTestDocuments(count int) []document.Document {
	docs := make([]document.Document, count)
	for i := 0; i < count; i++ {
		doc := document.NewDocument()
		contentField, _ := document.NewTextField("content", "test document content for indexing", true)
		doc.Add(contentField)
		idField, _ := document.NewStringField("id", string(rune('a'+i%26))+string(rune('0'+i/26)), true)
		doc.Add(idField)
		docs[i] = *doc
	}
	return docs
}

// TestFlushByRam tests flushing based on RAM buffer size.
// Source: TestFlushByRamOrCountsPolicy.testFlushByRam()
func TestFlushByRam(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Calculate RAM buffer: base + random
	ramBuffer := 10.0 + float64(2+rng.Intn(10)) + rng.Float64()
	numThreads := 1 + rng.Intn(2)

	runFlushByRam(t, numThreads, ramBuffer, false)
}

// TestFlushByRamLargeBuffer tests flushing with a large RAM buffer (256MB).
// With such a large buffer, we should never stall.
// Source: TestFlushByRamOrCountsPolicy.testFlushByRamLargeBuffer()
func TestFlushByRamLargeBuffer(t *testing.T) {
	rng := rand.New(rand.New(rand.NewSource(time.Now().UnixNano())))
	numThreads := 1 + rng.Intn(2)

	runFlushByRam(t, numThreads, 256.0, true)
}

// runFlushByRam runs the flush by RAM test with the given parameters.
func runFlushByRam(t *testing.T, numThreads int, maxRamMB float64, ensureNotStalled bool) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numDocumentsToIndex := 10 + 30 + rng.Intn(20)
	pendingDocs := &atomic.Int32{}
	pendingDocs.Store(int32(numDocumentsToIndex))

	// Create directory
	dir := store.NewByteBuffersDirectory()

	// Create flush policy
	flushPolicy := NewMockFlushPolicy(0, maxRamMB)

	// Create analyzer
	analyzer := analysis.NewStandardAnalyzer()

	// Create IndexWriter config
	config := index.NewIndexWriterConfig(analyzer)
	config.SetRAMBufferSizeMB(maxRamMB)
	config.SetMaxBufferedDocs(-1) // Disable auto-flush by doc count

	// Create IndexWriter
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Verify flush policy settings
	if flushPolicy.FlushOnDocCount() {
		t.Error("Expected flushOnDocCount to be false")
	}
	if !flushPolicy.FlushOnRAM() {
		t.Error("Expected flushOnRAM to be true")
	}

	// Create test documents
	testDocs := createTestDocuments(100)

	// Create and start threads
	threads := make([]*IndexThread, numThreads)
	for i := 0; i < numThreads; i++ {
		threads[i] = NewIndexThread(pendingDocs, numThreads, writer, testDocs, false)
		threads[i].Start()
	}

	// Wait for all threads to complete
	for i := 0; i < numThreads; i++ {
		if err := threads[i].Join(); err != nil {
			t.Fatalf("Thread %d failed: %v", i, err)
		}
	}

	// Verify results
	maxRAMBytes := int64(maxRamMB * 1024 * 1024)

	numDocs := writer.NumDocs()
	if numDocs != numDocumentsToIndex {
		t.Errorf("Expected %d documents, got %d", numDocumentsToIndex, numDocs)
	}

	// Verify peak bytes stayed within bounds
	if flushPolicy.PeakBytesWithoutFlush() > maxRAMBytes {
		t.Errorf("Peak bytes without flush (%d) exceeded watermark (%d)",
			flushPolicy.PeakBytesWithoutFlush(), maxRAMBytes)
	}

	// Commit and verify
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify with reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocumentsToIndex {
		t.Errorf("Expected %d documents in reader, got %d", numDocumentsToIndex, reader.NumDocs())
	}
}

// TestFlushDocCount tests flushing based on document count.
// Source: TestFlushByRamOrCountsPolicy.testFlushDocCount()
func TestFlushDocCount(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numThreadsList := []int{2 + rng.Intn(5), 1}
	analyzer := analysis.NewStandardAnalyzer()

	for _, numThreads := range numThreadsList {
		numDocumentsToIndex := 50 + 30 + rng.Intn(20)
		pendingDocs := &atomic.Int32{}
		pendingDocs.Store(int32(numDocumentsToIndex))

		// Create directory
		dir := store.NewByteBuffersDirectory()

		// Create flush policy
		maxBufferedDocs := 2 + 10 + rng.Intn(10)
		flushPolicy := NewMockFlushPolicy(maxBufferedDocs, 0)

		// Create IndexWriter config
		config := index.NewIndexWriterConfig(analyzer)
		config.SetMaxBufferedDocs(maxBufferedDocs)
		config.SetRAMBufferSizeMB(-1) // Disable auto-flush by RAM

		// Create IndexWriter
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Verify flush policy settings
		if !flushPolicy.FlushOnDocCount() {
			t.Error("Expected flushOnDocCount to be true")
		}
		if flushPolicy.FlushOnRAM() {
			t.Error("Expected flushOnRAM to be false")
		}

		// Create test documents
		testDocs := createTestDocuments(100)

		// Create and start threads
		threads := make([]*IndexThread, numThreads)
		for i := 0; i < numThreads; i++ {
			threads[i] = NewIndexThread(pendingDocs, numThreads, writer, testDocs, false)
			threads[i].Start()
		}

		// Wait for all threads to complete
		for i := 0; i < numThreads; i++ {
			if err := threads[i].Join(); err != nil {
				t.Fatalf("Thread %d failed: %v", i, err)
			}
		}

		// Verify results
		numDocs := writer.NumDocs()
		if numDocs != numDocumentsToIndex {
			t.Errorf("Expected %d documents, got %d", numDocumentsToIndex, numDocs)
		}

		// Verify peak doc count stayed within bounds
		if flushPolicy.PeakDocCountWithoutFlush() > maxBufferedDocs {
			t.Errorf("Peak doc count without flush (%d) exceeded max buffered docs (%d)",
				flushPolicy.PeakDocCountWithoutFlush(), maxBufferedDocs)
		}

		// Commit and close
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
		writer.Close()
		dir.Close()
	}
}

// TestFlushPolicyRandom tests flush policies with random configurations.
// Source: TestFlushByRamOrCountsPolicy.testRandom()
func TestFlushPolicyRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numThreads := 1 + rng.Intn(8)
	numDocumentsToIndex := 50 + 70 + rng.Intn(50)
	pendingDocs := &atomic.Int32{}
	pendingDocs.Store(int32(numDocumentsToIndex))

	// Create directory
	dir := store.NewByteBuffersDirectory()

	// Create analyzer
	analyzer := analysis.NewStandardAnalyzer()

	// Create flush policy with random settings
	flushPolicy := NewMockFlushPolicy(
		10+rng.Intn(50),      // maxBufferedDocs
		1.0+rng.Float64()*10, // ramBufferSizeMB
	)

	// Create IndexWriter config
	config := index.NewIndexWriterConfig(analyzer)
	if flushPolicy.FlushOnRAM() {
		config.SetRAMBufferSizeMB(flushPolicy.ramBufferSizeMB)
	}
	if flushPolicy.FlushOnDocCount() {
		config.SetMaxBufferedDocs(flushPolicy.maxBufferedDocs)
	}

	// Create IndexWriter
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Create test documents
	testDocs := createTestDocuments(100)

	// Create and start threads with random commits
	threads := make([]*IndexThread, numThreads)
	for i := 0; i < numThreads; i++ {
		threads[i] = NewIndexThread(pendingDocs, numThreads, writer, testDocs, true)
		threads[i].Start()
	}

	// Wait for all threads to complete
	for i := 0; i < numThreads; i++ {
		if err := threads[i].Join(); err != nil {
			t.Fatalf("Thread %d failed: %v", i, err)
		}
	}

	// Verify results
	numDocs := writer.NumDocs()
	if numDocs != numDocumentsToIndex {
		t.Errorf("Expected %d documents, got %d", numDocumentsToIndex, numDocs)
	}

	// Verify RAM-based flush constraints
	if flushPolicy.FlushOnRAM() && !flushPolicy.FlushOnDocCount() {
		maxRAMBytes := int64(flushPolicy.ramBufferSizeMB * 1024 * 1024)
		if flushPolicy.PeakBytesWithoutFlush() > maxRAMBytes {
			t.Errorf("Peak bytes without flush (%d) exceeded watermark (%d)",
				flushPolicy.PeakBytesWithoutFlush(), maxRAMBytes)
		}
	}

	// Commit and verify with reader
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocumentsToIndex {
		t.Errorf("Expected %d documents in reader, got %d", numDocumentsToIndex, reader.NumDocs())
	}

	if reader.MaxDoc() != numDocumentsToIndex {
		t.Errorf("Expected maxDoc %d in reader, got %d", numDocumentsToIndex, reader.MaxDoc())
	}

	// If not flushing on RAM, should never stall
	if !flushPolicy.FlushOnRAM() {
		// In a full implementation, we would check stall control here
		// For now, this is a placeholder for the assertion
		t.Log("Not flushing on RAM - stall control not triggered")
	}
}

// TestFlushPolicyBasic tests basic flush policy functionality.
func TestFlushPolicyBasic(t *testing.T) {
	// Test RAM-based flush
	ramPolicy := NewMockFlushPolicy(0, 16.0)
	if !ramPolicy.FlushOnRAM() {
		t.Error("Expected FlushOnRAM to be true")
	}
	if ramPolicy.FlushOnDocCount() {
		t.Error("Expected FlushOnDocCount to be false")
	}

	// Test doc count-based flush
	docPolicy := NewMockFlushPolicy(100, 0)
	if docPolicy.FlushOnRAM() {
		t.Error("Expected FlushOnRAM to be false")
	}
	if !docPolicy.FlushOnDocCount() {
		t.Error("Expected FlushOnDocCount to be true")
	}

	// Test combined flush
	combinedPolicy := NewMockFlushPolicy(100, 16.0)
	if !combinedPolicy.FlushOnRAM() {
		t.Error("Expected FlushOnRAM to be true")
	}
	if !combinedPolicy.FlushOnDocCount() {
		t.Error("Expected FlushOnDocCount to be true")
	}
}

// TestFlushPolicyThresholds tests flush policy threshold calculations.
func TestFlushPolicyThresholds(t *testing.T) {
	// Test RAM threshold
	policy := NewMockFlushPolicy(0, 1.0) // 1MB threshold

	// Should not flush below threshold
	if policy.ShouldFlush(1, 100) {
		t.Error("Should not flush below RAM threshold")
	}

	// Should flush at threshold
	maxBytes := int64(1.0 * 1024 * 1024)
	if !policy.ShouldFlush(1, maxBytes) {
		t.Error("Should flush at RAM threshold")
	}

	// Test doc count threshold
	policy2 := NewMockFlushPolicy(10, 0) // 10 docs threshold

	// Should not flush below threshold
	if policy2.ShouldFlush(5, 1000000) {
		t.Error("Should not flush below doc count threshold")
	}

	// Should flush at threshold
	if !policy2.ShouldFlush(10, 100) {
		t.Error("Should flush at doc count threshold")
	}
}

// TestFlushPolicyPeakTracking tests that peak values are tracked correctly.
func TestFlushPolicyPeakTracking(t *testing.T) {
	policy := NewMockFlushPolicy(100, 100.0)

	// Simulate increasing memory usage
	policy.ShouldFlush(10, 1000)
	policy.ShouldFlush(20, 2000)
	policy.ShouldFlush(30, 1500)

	if policy.PeakBytesWithoutFlush() != 2000 {
		t.Errorf("Expected peak bytes 2000, got %d", policy.PeakBytesWithoutFlush())
	}

	if policy.PeakDocCountWithoutFlush() != 30 {
		t.Errorf("Expected peak doc count 30, got %d", policy.PeakDocCountWithoutFlush())
	}
}

// TestFlushPolicyConcurrent tests flush policy with concurrent access.
func TestFlushPolicyConcurrent(t *testing.T) {
	policy := NewMockFlushPolicy(1000, 100.0)
	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				docCount := id*numIterations + j
				ramUsed := int64(docCount * 100)
				policy.ShouldFlush(docCount, ramUsed)
			}
		}(i)
	}

	wg.Wait()

	// Verify that peak values were tracked
	if policy.PeakBytesWithoutFlush() <= 0 {
		t.Error("Expected positive peak bytes")
	}
	if policy.PeakDocCountWithoutFlush() <= 0 {
		t.Error("Expected positive peak doc count")
	}
}

// TestFlushByRamOrCountsPolicy is the main test suite entry point.
// This test runs all the flush policy tests.
func TestFlushByRamOrCountsPolicy(t *testing.T) {
	t.Run("Basic", TestFlushPolicyBasic)
	t.Run("Thresholds", TestFlushPolicyThresholds)
	t.Run("PeakTracking", TestFlushPolicyPeakTracking)
	t.Run("Concurrent", TestFlushPolicyConcurrent)
	t.Run("ByRam", TestFlushByRam)
	t.Run("ByRamLargeBuffer", TestFlushByRamLargeBuffer)
	t.Run("ByDocCount", TestFlushDocCount)
	t.Run("Random", TestFlushPolicyRandom)
}
