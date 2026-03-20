package index

import (
	"context"
	"testing"
	"time"
)

func TestNewNRTManager(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	if manager == nil {
		t.Fatal("expected NRTManager to not be nil")
	}

	if !manager.IsOpen() {
		t.Error("expected manager to be open")
	}

	if manager.GetGeneration() != 1 {
		t.Errorf("expected generation 1, got %d", manager.GetGeneration())
	}

	if manager.GetWriter() != writer {
		t.Error("expected writer to be set")
	}
}

func TestNewNRTManager_Nil(t *testing.T) {
	_, err := NewNRTManager(nil)
	if err == nil {
		t.Error("expected error for nil writer")
	}
}

func TestNRTManager_GetReader(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	reader, err := manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	if reader == nil {
		t.Error("expected reader to not be nil")
	}

	// Getting reader again should return same instance
	reader2, err := manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	if reader != reader2 {
		t.Error("expected same reader instance")
	}
}

func TestNRTManager_GetReader_Closed(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	manager.Close()

	_, err = manager.GetReader()
	if err == nil {
		t.Error("expected error when manager is closed")
	}
}

func TestNRTManager_MaybeRefresh(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Try refresh
	refreshed, err := manager.MaybeRefresh(context.Background())
	if err != nil {
		t.Fatalf("maybe refresh failed: %v", err)
	}

	// Without actual changes, should not refresh
	if refreshed {
		t.Error("expected no refresh without changes")
	}
}

func TestNRTManager_MaybeRefresh_Closed(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	manager.Close()

	_, err = manager.MaybeRefresh(context.Background())
	if err == nil {
		t.Error("expected error when manager is closed")
	}
}

func TestNRTManager_Close(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	err = manager.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if manager.IsOpen() {
		t.Error("expected manager to be closed")
	}

	// Close again should not error
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestNRTManager_SetMaxRefreshSec(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Default should be 60 seconds
	if manager.GetMaxRefreshSec() != 60*time.Second {
		t.Errorf("expected default 60s, got %v", manager.GetMaxRefreshSec())
	}

	// Set to 30 seconds
	manager.SetMaxRefreshSec(30 * time.Second)
	if manager.GetMaxRefreshSec() != 30*time.Second {
		t.Errorf("expected 30s, got %v", manager.GetMaxRefreshSec())
	}
}

func TestNRTManager_ApplyAllDeletes(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Default should be true
	if !manager.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be true by default")
	}

	// Set to false
	manager.SetApplyAllDeletes(false)
	if manager.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be false")
	}
}

// mockNRTReopenListener is a mock implementation of NRTReopenListener for testing
type mockNRTReopenListener struct {
	beforeCalled bool
	afterCalled  bool
	generation   int64
}

func (m *mockNRTReopenListener) BeforeReopen() {
	m.beforeCalled = true
}

func (m *mockNRTReopenListener) AfterReopen(generation int64) {
	m.afterCalled = true
	m.generation = generation
}

func TestNRTManager_ReopenListener(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	listener := &mockNRTReopenListener{}
	manager.AddReopenListener(listener)

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Trigger refresh
	manager.MaybeRefresh(context.Background())

	// Note: Listeners are only called when an actual refresh happens
	// Without actual changes, they won't be called
}

func TestNRTManager_RemoveReopenListener(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	listener := &mockNRTReopenListener{}
	manager.AddReopenListener(listener)
	manager.RemoveReopenListener(listener)

	// Should not panic
}

func TestNRTManager_WaitForGeneration(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Wait for current generation should return immediately
	reader, err := manager.WaitForGeneration(context.Background(), 1)
	if err != nil {
		t.Fatalf("wait for generation failed: %v", err)
	}

	if reader == nil {
		t.Error("expected reader")
	}
}

func TestNRTManager_WaitForGeneration_Timeout(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Set short timeout
	manager.SetMaxRefreshSec(100 * time.Millisecond)

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Wait for future generation should timeout
	_, err = manager.WaitForGeneration(context.Background(), 100)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestNRTManager_Refresh(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Refresh
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = manager.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
}
