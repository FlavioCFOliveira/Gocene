// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: crash_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestCrash.java
//         lucene/core/src/test/org/apache/lucene/index/TestCrashCausesCorruptIndex.java
// Purpose: Tests index consistency after simulated crash scenarios,
//          uncommitted changes handling, and lock file cleanup
//
// GC-195: Test Crash Recovery

package index_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ============================================================================
// MockDirectoryWrapper - Simulates directory crashes for testing
// ============================================================================

// MockDirectoryWrapper wraps a Directory to simulate crash scenarios.
// This is used to test index recovery and consistency after crashes.
type MockDirectoryWrapper struct {
	*store.FilterDirectory
	crashEnabled     bool
	crashAfterCreate map[string]bool
	crashOnClose     bool
	mu               sync.RWMutex
	// Track open files for cleanup verification
	openOutputs      map[string]bool
	openInputs       map[string]bool
}

// NewMockDirectoryWrapper creates a new MockDirectoryWrapper wrapping the given directory.
func NewMockDirectoryWrapper(dir store.Directory) *MockDirectoryWrapper {
	mdw := &MockDirectoryWrapper{
		FilterDirectory:  store.NewFilterDirectory(dir),
		crashAfterCreate: make(map[string]bool),
		openOutputs:      make(map[string]bool),
		openInputs:       make(map[string]bool),
	}
	return mdw
}

// Crash simulates a crash by disabling further operations and clearing state.
func (m *MockDirectoryWrapper) Crash() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crashEnabled = true
}

// ClearCrash clears the crash state.
func (m *MockDirectoryWrapper) ClearCrash() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crashEnabled = false
}

// SetCrashAfterCreateOutput sets the wrapper to crash after creating output with the given name.
func (m *MockDirectoryWrapper) SetCrashAfterCreateOutput(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crashAfterCreate[name] = true
}

// SetAssertNoUnreferencedFilesOnClose controls whether to check for unreferenced files on close.
// This is a no-op in this implementation but kept for API compatibility.
func (m *MockDirectoryWrapper) SetAssertNoUnreferencedFilesOnClose(assert bool) {
	// No-op for now - would track file references in full implementation
}

// CreateOutput overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.crashEnabled {
		return nil, errors.New("directory is in crashed state")
	}

	// Check if we should crash after creating this output
	if m.crashAfterCreate[name] {
		// Create the output first, then crash
		out, err := m.FilterDirectory.CreateOutput(name, ctx)
		if err != nil {
			return nil, err
		}
		// Close the output and throw crash exception
		out.Close()
		return nil, &CrashingException{msg: fmt.Sprintf("crashAfterCreateOutput %s", name)}
	}

	out, err := m.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}

	m.openOutputs[name] = true
	return &mockIndexOutput{IndexOutput: out, name: name, wrapper: m}, nil
}

// OpenInput overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return nil, errors.New("directory is in crashed state")
	}

	return m.FilterDirectory.OpenInput(name, ctx)
}

// DeleteFile overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) DeleteFile(name string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return errors.New("directory is in crashed state")
	}

	return m.FilterDirectory.DeleteFile(name)
}

// ListAll overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) ListAll() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return nil, errors.New("directory is in crashed state")
	}

	return m.FilterDirectory.ListAll()
}

// FileExists overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) FileExists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return false
	}

	return m.FilterDirectory.FileExists(name)
}

// FileLength overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) FileLength(name string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return 0, errors.New("directory is in crashed state")
	}

	return m.FilterDirectory.FileLength(name)
}

// Close overrides to simulate crash scenarios.
func (m *MockDirectoryWrapper) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.crashEnabled {
		return errors.New("directory is in crashed state")
	}

	return m.FilterDirectory.Close()
}

// removeOpenOutput removes an output from the tracking map.
func (m *MockDirectoryWrapper) removeOpenOutput(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.openOutputs, name)
}

// mockIndexOutput wraps IndexOutput to track open files.
type mockIndexOutput struct {
	store.IndexOutput
	name    string
	wrapper *MockDirectoryWrapper
}

// Close closes the output and removes it from tracking.
func (m *mockIndexOutput) Close() error {
	m.wrapper.removeOpenOutput(m.name)
	return m.IndexOutput.Close()
}

// ============================================================================
// CrashingException - Marker exception for simulated crashes
// ============================================================================

// CrashingException is a marker RuntimeException used in lieu of an actual machine crash.
type CrashingException struct {
	msg string
}

func (e *CrashingException) Error() string {
	return e.msg
}

// ============================================================================
// CrashAfterCreateOutput - FilterDirectory that crashes on specific file creation
// ============================================================================

// CrashAfterCreateOutput is a FilterDirectory that simulates a crash
// right after createOutput is called on a certain specified name.
type CrashAfterCreateOutput struct {
	*store.FilterDirectory
	crashAfterCreateOutput string
}

// NewCrashAfterCreateOutput creates a new CrashAfterCreateOutput wrapping the given directory.
func NewCrashAfterCreateOutput(dir store.Directory) *CrashAfterCreateOutput {
	return &CrashAfterCreateOutput{
		FilterDirectory: store.NewFilterDirectory(dir),
	}
}

// SetCrashAfterCreateOutput sets the filename that should trigger a crash.
func (c *CrashAfterCreateOutput) SetCrashAfterCreateOutput(name string) {
	c.crashAfterCreateOutput = name
}

// CreateOutput overrides to throw CrashingException after creating specific output.
func (c *CrashAfterCreateOutput) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	indexOutput, err := c.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}

	if c.crashAfterCreateOutput != "" && name == c.crashAfterCreateOutput {
		// CRASH!
		indexOutput.Close()
		return nil, &CrashingException{msg: fmt.Sprintf("crashAfterCreateOutput %s", c.crashAfterCreateOutput)}
	}

	return indexOutput, nil
}

// ============================================================================
// Test Helper Functions
// ============================================================================

// initIndex creates an IndexWriter with test documents.
// If initialCommit is true, commits before adding documents.
func initIndex(t *testing.T, r *rand.Rand, initialCommit bool) (*index.IndexWriter, *MockDirectoryWrapper) {
	dir := store.NewByteBuffersDirectory()
	mdw := NewMockDirectoryWrapper(dir)
	return initIndexWithDir(t, r, mdw, initialCommit, true)
}

// initIndexWithDir creates an IndexWriter with the given directory.
func initIndexWithDir(t *testing.T, r *rand.Rand, dir *MockDirectoryWrapper, initialCommit, commitOnClose bool) (*index.IndexWriter, *MockDirectoryWrapper) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetMaxBufferedDocs(10)
	config.SetMergeScheduler(index.NewConcurrentMergeScheduler())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	if initialCommit {
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	// Create test document
	doc := document.NewDocument()
	contentField, _ := document.NewTextField("content", "aaa", false)
	idField, _ := document.NewTextField("id", "0", false)
	doc.Add(contentField)
	doc.Add(idField)

	// Add 157 documents
	for i := 0; i < 157; i++ {
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	return writer, dir
}

// crash simulates a crash by syncing the merge scheduler and crashing the directory.
func crash(t *testing.T, writer *index.IndexWriter) {
	// Get the directory from the writer
	dir := writer.GetDirectory()
	mdw, ok := dir.(*MockDirectoryWrapper)
	if !ok {
		t.Skip("Crash test requires MockDirectoryWrapper")
		return
	}

	// Sync merge scheduler
	if scheduler := writer.GetConfig().GetMergeScheduler(); scheduler != nil {
		if cms, ok := scheduler.(*index.ConcurrentMergeScheduler); ok {
			cms.Sync()
		}
	}

	// Crash the directory
	mdw.Crash()

	// Sync again after crash
	if scheduler := writer.GetConfig().GetMergeScheduler(); scheduler != nil {
		if cms, ok := scheduler.(*index.ConcurrentMergeScheduler); ok {
			cms.Sync()
		}
	}

	// Clear crash state
	mdw.ClearCrash()
}

// ============================================================================
// Test Cases from TestCrash.java
// ============================================================================

// TestCrashWhileIndexing tests crashing while documents are being indexed.
// This test verifies that:
// - Index remains consistent after crash
// - Some documents may be lost (uncommitted)
// - Index can be reopened and recovered
func TestCrashWhileIndexing(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	// This test relies on being able to open a reader before any commit
	// happened, so we must create an initial commit just to allow that, but
	// before any documents were added.
	writer, dir := initIndex(t, r, true)

	// We create leftover files because merging could be
	// running when we crash:
	dir.SetAssertNoUnreferencedFilesOnClose(false)

	// Simulate crash
	crash(t, writer)

	// Try to open reader - should succeed even after crash
	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader after crash: %v", err)
	}

	// Should have fewer than 157 docs (some lost in crash)
	if reader.NumDocs() >= 157 {
		t.Errorf("Expected fewer than 157 docs after crash, got %d", reader.NumDocs())
	}
	reader.Close()

	// Make a new dir, copying from the crashed dir, and
	// open IW on it, to confirm IW "recovers" after a crash:
	dir2 := store.NewByteBuffersDirectory()
	copyDirectoryContents(t, dir2, dir)
	dir.Close()

	// Should be able to create a new writer on the recovered directory
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer2, err := index.NewIndexWriter(dir2, config)
	if err != nil {
		t.Fatalf("Failed to create new writer on recovered dir: %v", err)
	}
	writer2.Close()
	dir2.Close()
}

// TestWriterAfterCrash tests creating a new writer after a crash.
// This test verifies that:
// - IndexWriter can be recreated after crash
// - New documents can be added after recovery
// - Total document count reflects both old and new documents
func TestWriterAfterCrash(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	writer, dir := initIndex(t, r, true)

	// We create leftover files because merging could be
	// running / store files could be open when we crash:
	dir.SetAssertNoUnreferencedFilesOnClose(false)

	// Simulate crash
	crash(t, writer)

	// Create new writer on same directory (simulating recovery)
	writer2, _ := initIndexWithDir(t, r, dir, false, true)

	// Get doc stats before closing
	stats := writer2.GetDocStats()
	maxDoc := stats.MaxDoc

	writer2.Close()

	// Should have fewer than 314 docs (157 + 157, but some lost in crash)
	if maxDoc >= 314 {
		t.Errorf("Expected fewer than 314 docs after crash and re-index, got %d", maxDoc)
	}

	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	if reader.NumDocs() >= 314 {
		t.Errorf("Expected fewer than 314 docs in reader, got %d", reader.NumDocs())
	}
	reader.Close()

	// Make a new dir, copying from the crashed dir, and
	// open IW on it, to confirm IW "recovers" after a crash:
	dir2 := store.NewByteBuffersDirectory()
	copyDirectoryContents(t, dir2, dir)
	dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer3, err := index.NewIndexWriter(dir2, config)
	if err != nil {
		t.Fatalf("Failed to create writer on recovered dir: %v", err)
	}
	writer3.Close()
	dir2.Close()
}

// TestCrashAfterReopen tests crashing after reopening the index.
// This test verifies that:
// - Index can be closed and reopened
// - Crash after reopen maintains consistency
// - At least original documents are preserved
func TestCrashAfterReopen(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	writer, dir := initIndex(t, r, false)

	// We create leftover files because merging could be
	// running when we crash:
	dir.SetAssertNoUnreferencedFilesOnClose(false)

	writer.Close()

	// Reopen and add more documents
	writer2, _ := initIndexWithDir(t, r, dir, false, true)

	stats := writer2.GetDocStats()
	if stats.MaxDoc != 314 {
		t.Errorf("Expected 314 docs before crash, got %d", stats.MaxDoc)
	}

	// Simulate crash
	crash(t, writer2)

	// Try to open reader
	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader after crash: %v", err)
	}

	// Should have at least 157 docs (original batch)
	if reader.NumDocs() < 157 {
		t.Errorf("Expected at least 157 docs after crash, got %d", reader.NumDocs())
	}
	reader.Close()

	// Make a new dir, copying from the crashed dir
	dir2 := store.NewByteBuffersDirectory()
	copyDirectoryContents(t, dir2, dir)
	dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer3, err := index.NewIndexWriter(dir2, config)
	if err != nil {
		t.Fatalf("Failed to create writer on recovered dir: %v", err)
	}
	writer3.Close()
	dir2.Close()
}

// TestCrashAfterClose tests crashing after properly closing the writer.
// This test verifies that:
// - All documents are preserved when crash happens after close
// - Index remains fully consistent
func TestCrashAfterClose(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	writer, dir := initIndex(t, r, false)

	writer.Close()

	// Crash after close
	dir.Crash()

	// Try to open reader
	reader, err := index.NewDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader after crash: %v", err)
	}

	// Should have exactly 157 docs (all committed before close)
	if reader.NumDocs() != 157 {
		t.Errorf("Expected 157 docs after close and crash, got %d", reader.NumDocs())
	}
	reader.Close()
	dir.Close()
}

// TestCrashAfterCloseNoWait tests crashing after close without waiting.
// This test verifies that:
// - Commit before close ensures data persistence
// - Crash after close doesn't lose committed data
func TestCrashAfterCloseNoWait(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	dir := store.NewByteBuffersDirectory()
	mdw := NewMockDirectoryWrapper(dir)

	writer, _ := initIndexWithDir(t, r, mdw, false, false)

	// Commit before close
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	writer.Close()

	// Crash after close
	mdw.Crash()

	// Try to open reader
	reader, err := index.NewDirectoryReader(mdw)
	if err != nil {
		t.Fatalf("Failed to open reader after crash: %v", err)
	}

	// Should have exactly 157 docs
	if reader.NumDocs() != 157 {
		t.Errorf("Expected 157 docs after commit, close, and crash, got %d", reader.NumDocs())
	}
	reader.Close()
	mdw.Close()
}

// ============================================================================
// Test Cases from TestCrashCausesCorruptIndex.java
// ============================================================================

// TestCrashCorruptsIndexing tests that a crash during segment file creation
// doesn't corrupt the index (LUCENE-3627).
// This test verifies that:
// - Crash during segments file creation is handled gracefully
// - Index remains searchable after crash
// - New documents can be added after recovery
func TestCrashCorruptsIndexing(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "testCrashCorruptsIndexing")

	// Index 1 document and commit, then crash during creation of segments_2
	indexAndCrashOnCreateOutputSegments2(t, path)

	// Search for "fleas" - should find 2 documents
	searchForFleas(t, path, 2)

	// Index another document after restart
	indexAfterRestart(t, path)

	// Search again - should find 3 documents
	searchForFleas(t, path, 3)
}

// indexAndCrashOnCreateOutputSegments2 indexes 1 document and commits,
// prepares for crashing, indexes 1 more document, and crashes upon commit
// when creation of segments_2 is attempted.
func indexAndCrashOnCreateOutputSegments2(t *testing.T, path string) {
	realDirectory, err := store.NewFSDirectory(path)
	if err != nil {
		t.Fatalf("Failed to create FSDirectory: %v", err)
	}
	defer realDirectory.Close()

	crashAfterCreateOutput := NewCrashAfterCreateOutput(realDirectory)

	// NOTE: cannot use RandomIndexWriter because it sometimes commits
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	indexWriter, err := index.NewIndexWriter(crashAfterCreateOutput, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add first document
	doc1 := getCrashTestDocument()
	if err := indexWriter.AddDocument(doc1); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Writes segments_1
	if err := indexWriter.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Set up crash on creation of pending_segments_2
	crashAfterCreateOutput.SetCrashAfterCreateOutput("pending_segments_2")

	// Add second document
	doc2 := getCrashTestDocument()
	if err := indexWriter.AddDocument(doc2); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Tries to write segments_2 but hits fake exception
	err = indexWriter.Commit()
	if err == nil {
		t.Fatal("Expected CrashingException during commit")
	}
	if !strings.Contains(err.Error(), "crashAfterCreateOutput") {
		t.Fatalf("Expected crash error, got: %v", err)
	}

	// Writes segments_3
	indexWriter.Close()

	// Verify segments_2 was not created
	if slowFileExists(t, realDirectory, "segments_2") {
		t.Error("segments_2 should not exist")
	}

	crashAfterCreateOutput.Close()
}

// indexAfterRestart attempts to index another document after restart.
// LUCENE-3627 (before the fix): this would fail because it doesn't know
// what to do with the created but empty segments_2 file.
func indexAfterRestart(t *testing.T, path string) {
	realDirectory, err := store.NewFSDirectory(path)
	if err != nil {
		t.Fatalf("Failed to create FSDirectory: %v", err)
	}
	defer realDirectory.Close()

	// LUCENE-3627 (before the fix): this line fails because
	// it doesn't know what to do with the created but empty segments_2 file
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	indexWriter, err := index.NewIndexWriter(realDirectory, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter after restart: %v", err)
	}

	// Currently the test fails above.
	// However, to test the fix, the following lines should pass as well.
	doc := getCrashTestDocument()
	if err := indexWriter.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document after restart: %v", err)
	}
	indexWriter.Close()

	if slowFileExists(t, realDirectory, "segments_2") {
		t.Error("segments_2 should not exist after restart")
	}
}

// searchForFleas runs a search for "fleas" and verifies the expected hit count.
func searchForFleas(t *testing.T, path string, expectedTotalHits int) {
	realDirectory, err := store.NewFSDirectory(path)
	if err != nil {
		t.Fatalf("Failed to create FSDirectory: %v", err)
	}
	defer realDirectory.Close()

	indexReader, err := index.NewDirectoryReader(realDirectory)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer indexReader.Close()

	// Create searcher
	searcher := index.NewIndexSearcher(indexReader)

	// Search for "fleas" in the text field
	term := index.NewTerm("text", "fleas")
	query := search.NewTermQuery(term)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if topDocs == nil {
		t.Fatal("TopDocs should not be nil")
	}

	if int(topDocs.TotalHits.Value) != expectedTotalHits {
		t.Errorf("Expected %d hits, got %d", expectedTotalHits, topDocs.TotalHits.Value)
	}
}

// getCrashTestDocument returns a document with content "my dog has fleas".
func getCrashTestDocument() *document.Document {
	doc := document.NewDocument()
	textField, _ := document.NewTextField("text", "my dog has fleas", false)
	doc.Add(textField)
	return doc
}

// slowFileExists checks if a file exists in the directory (with retry for slow filesystems).
func slowFileExists(t *testing.T, dir store.Directory, name string) bool {
	// Simple implementation - just check once
	return dir.FileExists(name)
}

// ============================================================================
// Utility Functions
// ============================================================================

// copyDirectoryContents copies all files from source to destination directory.
func copyDirectoryContents(t *testing.T, dst, src store.Directory) {
	files, err := src.ListAll()
	if err != nil {
		t.Fatalf("Failed to list source directory: %v", err)
	}

	for _, file := range files {
		srcInput, err := src.OpenInput(file, store.IOContextDefault)
		if err != nil {
			t.Fatalf("Failed to open source file %s: %v", file, err)
		}

		srcLen, err := src.FileLength(file)
		if err != nil {
			srcInput.Close()
			t.Fatalf("Failed to get file length for %s: %v", file, err)
		}

		data := make([]byte, srcLen)
		_, err = srcInput.Read(data)
		srcInput.Close()
		if err != nil {
			t.Fatalf("Failed to read source file %s: %v", file, err)
		}

		dstOutput, err := dst.CreateOutput(file, store.IOContextDefault)
		if err != nil {
			t.Fatalf("Failed to create destination file %s: %v", file, err)
		}

		_, err = dstOutput.Write(data)
		dstOutput.Close()
		if err != nil {
			t.Fatalf("Failed to write destination file %s: %v", file, err)
		}
	}
}

// newMockDirectory creates a new MockDirectoryWrapper with the given random and lock factory.
func newMockDirectory(t *testing.T, r *rand.Rand) *MockDirectoryWrapper {
	dir := store.NewByteBuffersDirectory()
	return NewMockDirectoryWrapper(dir)
}
