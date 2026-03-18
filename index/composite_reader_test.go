// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MockIndexReader is a mock implementation of IndexReaderInterface for testing
type MockIndexReader struct {
	*IndexReader
	maxDocVal    int
	numDocsVal   int
	hasDeletions bool
}

func NewMockIndexReader(maxDoc, numDocs int, hasDeletions bool) *MockIndexReader {
	reader := &MockIndexReader{
		IndexReader:  NewIndexReader(),
		maxDocVal:    maxDoc,
		numDocsVal:   numDocs,
		hasDeletions: hasDeletions,
	}
	reader.SetMaxDoc(maxDoc)
	reader.SetNumDocs(numDocs)
	reader.SetDocCount(maxDoc)
	if hasDeletions {
		// Create a simple Bits implementation for live docs
		liveDocs, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			panic(err)
		}
		for i := 0; i < numDocs && i < maxDoc; i++ {
			liveDocs.Set(i)
		}
		reader.SetLiveDocs(liveDocs)
	}
	return reader
}

func (m *MockIndexReader) MaxDoc() int {
	return m.maxDocVal
}

func (m *MockIndexReader) NumDocs() int {
	return m.numDocsVal
}

func (m *MockIndexReader) HasDeletions() bool {
	return m.hasDeletions
}

func (m *MockIndexReader) GetContext() (IndexReaderContext, error) {
	return nil, nil
}

func (m *MockIndexReader) Leaves() ([]*LeafReaderContext, error) {
	return nil, nil
}

func (m *MockIndexReader) StoredFields() (StoredFields, error) {
	return nil, nil
}

// TestCompositeReaderCreation tests the creation of a CompositeReader
func TestCompositeReaderCreation(t *testing.T) {
	reader := NewCompositeReader()
	if reader == nil {
		t.Fatal("NewCompositeReader returned nil")
	}

	// Verify it embeds IndexReader
	if reader.IndexReader == nil {
		t.Error("CompositeReader.IndexReader is nil")
	}
}

// TestCompositeReaderGetSequentialSubReaders tests getting sub-readers
func TestCompositeReaderGetSequentialSubReaders(t *testing.T) {
	reader := NewCompositeReader()
	subReaders := reader.GetSequentialSubReaders()

	// Base implementation returns nil, subclasses should override
	if subReaders != nil {
		t.Error("GetSequentialSubReaders should return nil in base implementation")
	}
}

// TestCompositeReaderInterface tests that CompositeReader implements the interface
func TestCompositeReaderInterface(t *testing.T) {
	var _ CompositeReaderInterface = (*CompositeReader)(nil)
}

// TestBaseCompositeReaderCreation tests the creation of BaseCompositeReader
func TestBaseCompositeReaderCreation(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	if reader == nil {
		t.Fatal("NewBaseCompositeReader returned nil")
	}

	// Verify sub-readers are stored
	if len(reader.subReaders) != 2 {
		t.Errorf("Expected 2 sub-readers, got %d", len(reader.subReaders))
	}
}

// TestBaseCompositeReaderEmptySubReaders tests creation with empty sub-readers
func TestBaseCompositeReaderEmptySubReaders(t *testing.T) {
	_, err := NewBaseCompositeReader([]IndexReaderInterface{})
	if err == nil {
		t.Error("NewBaseCompositeReader should fail with empty sub-readers")
	}
}

// TestBaseCompositeReaderDocCounts tests document count calculations
func TestBaseCompositeReaderDocCounts(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 8, true),
		NewMockIndexReader(20, 18, true),
		NewMockIndexReader(30, 30, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	// MaxDoc should be sum of all sub-readers' maxDoc
	expectedMaxDoc := 10 + 20 + 30
	if reader.MaxDoc() != expectedMaxDoc {
		t.Errorf("MaxDoc: expected %d, got %d", expectedMaxDoc, reader.MaxDoc())
	}

	// NumDocs should be sum of all sub-readers' numDocs
	expectedNumDocs := 8 + 18 + 30
	if reader.NumDocs() != expectedNumDocs {
		t.Errorf("NumDocs: expected %d, got %d", expectedNumDocs, reader.NumDocs())
	}
}

// TestBaseCompositeReaderStarts tests the starts array calculation
func TestBaseCompositeReaderStarts(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
		NewMockIndexReader(30, 30, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	// starts[0] should be 0
	if reader.starts[0] != 0 {
		t.Errorf("starts[0]: expected 0, got %d", reader.starts[0])
	}

	// starts[1] should be 10
	if reader.starts[1] != 10 {
		t.Errorf("starts[1]: expected 10, got %d", reader.starts[1])
	}

	// starts[2] should be 30
	if reader.starts[2] != 30 {
		t.Errorf("starts[2]: expected 30, got %d", reader.starts[2])
	}

	// starts[3] should be 60 (total maxDoc)
	if reader.starts[3] != 60 {
		t.Errorf("starts[3]: expected 60, got %d", reader.starts[3])
	}
}

// TestBaseCompositeReaderReaderIndex tests the ReaderIndex method
func TestBaseCompositeReaderReaderIndex(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
		NewMockIndexReader(30, 30, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	tests := []struct {
		docID    int
		expected int
	}{
		{0, 0},    // First reader
		{5, 0},    // First reader
		{9, 0},    // First reader (last doc)
		{10, 1},   // Second reader (first doc)
		{15, 1},   // Second reader
		{29, 1},   // Second reader (last doc)
		{30, 2},   // Third reader (first doc)
		{45, 2},   // Third reader
		{59, 2},   // Third reader (last doc)
		{-1, -1},  // Invalid
		{60, -1},  // Invalid (equal to maxDoc)
		{100, -1}, // Invalid
	}

	for _, test := range tests {
		result := reader.ReaderIndex(test.docID)
		if result != test.expected {
			t.Errorf("ReaderIndex(%d): expected %d, got %d", test.docID, test.expected, result)
		}
	}
}

// TestBaseCompositeReaderReaderBase tests the ReaderBase method
func TestBaseCompositeReaderReaderBase(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
		NewMockIndexReader(30, 30, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	tests := []struct {
		readerIndex int
		expected    int
	}{
		{0, 0},
		{1, 10},
		{2, 30},
		{3, 60}, // This is the sentinel value (total maxDoc)
		{-1, -1},
		{4, -1},
	}

	for _, test := range tests {
		result := reader.ReaderBase(test.readerIndex)
		if result != test.expected {
			t.Errorf("ReaderBase(%d): expected %d, got %d", test.readerIndex, test.expected, result)
		}
	}
}

// TestBaseCompositeReaderGetSubReader tests the GetSubReader method
func TestBaseCompositeReaderGetSubReader(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
		NewMockIndexReader(30, 30, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	// Get sub-reader for various doc IDs
	tests := []struct {
		docID         int
		expectedIndex int
	}{
		{0, 0},
		{5, 0},
		{10, 1},
		{25, 1},
		{30, 2},
		{50, 2},
	}

	for _, test := range tests {
		subReader := reader.GetSubReader(test.docID)
		if subReader == nil {
			t.Errorf("GetSubReader(%d): returned nil", test.docID)
			continue
		}

		// Verify it's the correct sub-reader by checking maxDoc
		expectedMaxDoc := subReaders[test.expectedIndex].MaxDoc()
		if subReader.MaxDoc() != expectedMaxDoc {
			t.Errorf("GetSubReader(%d): expected sub-reader with MaxDoc=%d, got %d",
				test.docID, expectedMaxDoc, subReader.MaxDoc())
		}
	}

	// Test invalid doc ID
	if reader.GetSubReader(-1) != nil {
		t.Error("GetSubReader(-1): should return nil for invalid doc ID")
	}

	if reader.GetSubReader(100) != nil {
		t.Error("GetSubReader(100): should return nil for invalid doc ID")
	}
}

// TestBaseCompositeReaderGetSequentialSubReaders tests getting sub-readers
func TestBaseCompositeReaderGetSequentialSubReaders(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	result := reader.GetSequentialSubReaders()
	if len(result) != 2 {
		t.Errorf("GetSequentialSubReaders: expected 2 sub-readers, got %d", len(result))
	}

	// Verify the sub-readers are the same
	for i, sr := range result {
		if sr != subReaders[i] {
			t.Errorf("sub-reader %d: not the same instance", i)
		}
	}
}

// TestBaseCompositeReaderHasDeletions tests the HasDeletions method
func TestBaseCompositeReaderHasDeletions(t *testing.T) {
	// Test with no deletions
	reader1, _ := NewBaseCompositeReader([]IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
	})
	if reader1.HasDeletions() {
		t.Error("HasDeletions: expected false when no sub-readers have deletions")
	}

	// Test with deletions in one sub-reader
	reader2, _ := NewBaseCompositeReader([]IndexReaderInterface{
		NewMockIndexReader(10, 8, true),
		NewMockIndexReader(20, 20, false),
	})
	if !reader2.HasDeletions() {
		t.Error("HasDeletions: expected true when one sub-reader has deletions")
	}

	// Test with deletions in all sub-readers
	reader3, _ := NewBaseCompositeReader([]IndexReaderInterface{
		NewMockIndexReader(10, 8, true),
		NewMockIndexReader(20, 18, true),
	})
	if !reader3.HasDeletions() {
		t.Error("HasDeletions: expected true when all sub-readers have deletions")
	}
}

// TestBaseCompositeReaderInterface tests that BaseCompositeReader implements the interface
func TestBaseCompositeReaderInterface(t *testing.T) {
	var _ BaseCompositeReaderInterface = (*BaseCompositeReader)(nil)
}

// TestBaseCompositeReaderClose tests closing the reader
func TestBaseCompositeReaderClose(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
		NewMockIndexReader(20, 20, false),
	}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	// Close should not error
	if err := reader.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After closing, EnsureOpen should fail
	if err := reader.EnsureOpen(); err == nil {
		t.Error("EnsureOpen should fail after Close")
	}
}

// TestBaseCompositeReaderEnsureOpen tests the EnsureOpen method
func TestBaseCompositeReaderEnsureOpen(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
	}

	reader, _ := NewBaseCompositeReader(subReaders)

	// Should not error when open
	if err := reader.EnsureOpen(); err != nil {
		t.Errorf("EnsureOpen failed on open reader: %v", err)
	}

	// Close and verify it fails
	reader.Close()
	if err := reader.EnsureOpen(); err == nil {
		t.Error("EnsureOpen should fail on closed reader")
	}
}

// TestBaseCompositeReaderRefCount tests reference counting
func TestBaseCompositeReaderRefCount(t *testing.T) {
	subReaders := []IndexReaderInterface{
		NewMockIndexReader(10, 10, false),
	}

	reader, _ := NewBaseCompositeReader(subReaders)

	// Initial ref count should be 1
	if reader.GetRefCount() != 1 {
		t.Errorf("Initial ref count: expected 1, got %d", reader.GetRefCount())
	}

	// IncRef should increment
	if err := reader.IncRef(); err != nil {
		t.Errorf("IncRef failed: %v", err)
	}
	if reader.GetRefCount() != 2 {
		t.Errorf("After IncRef: expected 2, got %d", reader.GetRefCount())
	}

	// DecRef should decrement
	if err := reader.DecRef(); err != nil {
		t.Errorf("DecRef failed: %v", err)
	}
	if reader.GetRefCount() != 1 {
		t.Errorf("After DecRef: expected 1, got %d", reader.GetRefCount())
	}

	// TryIncRef should succeed on open reader
	if !reader.TryIncRef() {
		t.Error("TryIncRef should succeed on open reader")
	}
	if reader.GetRefCount() != 2 {
		t.Errorf("After TryIncRef: expected 2, got %d", reader.GetRefCount())
	}

	// Close both references
	reader.DecRef()
	reader.Close()

	// After close, TryIncRef should fail
	if reader.TryIncRef() {
		t.Error("TryIncRef should fail on closed reader")
	}
}

// TestCompositeReaderContextIntegration tests GetContext and Leaves methods
func TestCompositeReaderContextIntegration(t *testing.T) {
	// This test requires actual LeafReader implementations
	// For now, we just verify the methods exist and can be called

	// Create a mock IndexReader that acts like a LeafReader
	mockReader := NewMockIndexReader(10, 10, false)

	subReaders := []IndexReaderInterface{mockReader}

	reader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		t.Fatalf("NewBaseCompositeReader failed: %v", err)
	}

	// GetContext will fail because mockReader is not a *LeafReader
	// This is expected behavior - BaseCompositeReader requires LeafReader sub-readers
	_, err = reader.GetContext()
	if err == nil {
		t.Error("GetContext should fail when sub-readers are not LeafReader instances")
	}
}
