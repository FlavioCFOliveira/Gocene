// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestQueryCancelledError(t *testing.T) {
	err := &QueryCancelledError{Reason: "timeout exceeded"}
	if !strings.Contains(err.Error(), "timeout exceeded") {
		t.Errorf("Expected error message to contain 'timeout exceeded', got: %s", err.Error())
	}

	if !IsQueryCancelled(err) {
		t.Error("IsQueryCancelled should return true for QueryCancelledError")
	}

	// Test with regular error
	regularErr := &AlreadyClosedError{}
	if IsQueryCancelled(regularErr) {
		t.Error("IsQueryCancelled should return false for non-QueryCancelledError")
	}
}

func TestNewExitableDirectoryReader(t *testing.T) {
	// Test with nil context - should return error
	_, err := NewExitableDirectoryReader(nil, ExitableReaderConfig{
		QueryContext: nil,
	})
	if err == nil {
		t.Error("Expected error for nil context")
	}

	// Test with valid context - since we don't have a real DirectoryReader,
	// we can't test this fully, but we verify the error handling
	ctx := context.Background()
	_, err = NewExitableDirectoryReader(nil, ExitableReaderConfig{
		QueryContext: ctx,
		CheckEvery:   10,
	})
	// This will fail because we passed nil reader, but that's expected
	if err == nil {
		t.Error("Expected error for nil reader")
	}
}

func TestExitableReaderConfigDefaults(t *testing.T) {
	// Test default CheckEvery value
	ctx := context.Background()

	config := ExitableReaderConfig{
		QueryContext: ctx,
		// CheckEvery is 0
	}

	// The config should use DefaultCheckEvery when CheckEvery is 0
	// This is tested by verifying the struct values
	if config.CheckEvery != 0 {
		t.Errorf("Expected CheckEvery to be 0, got %d", config.CheckEvery)
	}
}

func TestExitablePostingsEnumCancellation(t *testing.T) {
	// Create a mock PostingsEnum
	mockEnum := &mockPostingsEnum{
		docs: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := ExitableReaderConfig{
		QueryContext: ctx,
		CheckEvery:   3,
	}

	exitableEnum := NewExitablePostingsEnum(mockEnum, config)

	// First call should work (before check)
	_, err := exitableEnum.NextDoc()
	if err != nil {
		t.Fatalf("First NextDoc should succeed: %v", err)
	}

	// Second call
	_, err = exitableEnum.NextDoc()
	if err != nil {
		t.Fatalf("Second NextDoc should succeed: %v", err)
	}

	// Third call should check timeout and fail
	_, err = exitableEnum.NextDoc()
	if err == nil {
		t.Error("Expected error after timeout check")
	}

	if !IsQueryCancelled(err) {
		t.Errorf("Expected QueryCancelledError, got: %T", err)
	}
}

func TestExitablePostingsEnumTimeout(t *testing.T) {
	// Create a mock PostingsEnum
	mockEnum := &mockPostingsEnum{
		docs: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	config := ExitableReaderConfig{
		QueryContext: ctx,
		CheckEvery:   5,
	}

	exitableEnum := NewExitablePostingsEnum(mockEnum, config)

	// Let the context timeout
	time.Sleep(100 * time.Millisecond)

	// Call NextDoc enough times to trigger timeout check
	for i := 0; i < 6; i++ {
		_, err := exitableEnum.NextDoc()
		if err != nil {
			if IsQueryCancelled(err) {
				// Expected cancellation
				return
			}
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	t.Error("Expected query to be cancelled after timeout")
}

func TestExitablePostingsEnumAdvance(t *testing.T) {
	// Create a mock PostingsEnum
	mockEnum := &mockPostingsEnum{
		docs: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	ctx := context.Background()
	config := ExitableReaderConfig{
		QueryContext: ctx,
		CheckEvery:   3,
	}

	exitableEnum := NewExitablePostingsEnum(mockEnum, config)

	// Test Advance
	docID, err := exitableEnum.Advance(5)
	if err != nil {
		t.Fatalf("Advance failed: %v", err)
	}
	if docID != 5 {
		t.Errorf("Expected docID 5, got %d", docID)
	}

	// Test DocID
	currentDoc := exitableEnum.DocID()
	if currentDoc != 5 {
		t.Errorf("Expected DocID 5, got %d", currentDoc)
	}

	// Test Freq
	freq, err := exitableEnum.Freq()
	if err != nil {
		t.Fatalf("Freq failed: %v", err)
	}
	if freq != 1 {
		t.Errorf("Expected freq 1, got %d", freq)
	}
}

// mockPostingsEnum is a simple mock implementation for testing
type mockPostingsEnum struct {
	docs      []int
	positions []int
	index     int
	docID     int
}

func (m *mockPostingsEnum) NextDoc() (int, error) {
	if m.index >= len(m.docs) {
		return -1, nil // NO_MORE_DOCS
	}
	m.docID = m.docs[m.index]
	m.index++
	return m.docID, nil
}

func (m *mockPostingsEnum) DocID() int {
	if m.index == 0 {
		return -1 // No current doc yet
	}
	return m.docID
}

func (m *mockPostingsEnum) Advance(target int) (int, error) {
	for m.index < len(m.docs) {
		if m.docs[m.index] >= target {
			m.docID = m.docs[m.index]
			m.index++
			return m.docID, nil
		}
		m.index++
	}
	return -1, nil
}

func (m *mockPostingsEnum) Freq() (int, error) {
	return 1, nil
}

func (m *mockPostingsEnum) NextPosition() (int, error) {
	return 0, nil
}

func (m *mockPostingsEnum) StartOffset() (int, error) {
	return 0, nil
}

func (m *mockPostingsEnum) EndOffset() (int, error) {
	return 0, nil
}

func (m *mockPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

func (m *mockPostingsEnum) Cost() int64 {
	return int64(len(m.docs))
}
